// Package sanctions screens party names against a locally loaded OFAC SDN list.
//
// The list is read from the public OFAC CSV exports (SDN.CSV, optional ALT.CSV)
// fetched by scripts/fetch-ofac.sh. Nothing is downloaded at runtime.
package sanctions

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// Match is the best list entry for a screened name.
type Match struct {
	EntryID string  // OFAC ent_num
	Name    string  // primary SDN name of the matched entry
	Program string  // sanctions program(s), e.g. "PROGRAM-A] [PROGRAM-B"
	Score   float64 // 0..1, token-level name similarity (see score)
}

type entry struct {
	id, name, program string
	tokens            [][]rune // normalized name tokens
	full              int      // tokens longer than an initial
}

// List is an immutable in-memory sanctions list, safe for concurrent use.
type List struct {
	entries []entry
	index   map[string][]int // indexKey -> entry positions
}

// noise tokens carry no identity and would flood the candidate index.
var noise = map[string]bool{
	"the": true, "of": true, "and": true, "co": true, "ltd": true, "llc": true,
	"inc": true, "sa": true, "de": true, "al": true, "el": true, "bin": true, "ibn": true,
}

// tokenize lowercases, splits on punctuation and drops noise tokens, so
// "SAMPLE, Jonathan" and "jonathan sample" carry the same tokens in any order.
// Limit: no diacritic folding (needs x/text); add when non-Latin lists matter.
func tokenize(s string) [][]rune {
	var toks [][]rune
	for _, t := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if !noise[t] {
			toks = append(toks, []rune(t))
		}
	}
	return toks
}

// indexKey buckets a token by its first three letters (the whole token when
// shorter), so spellings that differ later ("maximilian"/"maximillian",
// "jonathon"/"jonathan") still meet as candidates. Initials never retrieve.
// Limit: first-letter variants (Katrin/Catrin) rely on ALT.CSV aliases; add
// phonetic keys if that misses real hits.
func indexKey(t []rune) (string, bool) {
	if len(t) < 2 {
		return "", false
	}
	return string(t[:min(3, len(t))]), true
}

// add indexes matchOn (a primary name or alias) under the listed entry's identity.
func (l *List) add(id, name, program, matchOn string) {
	toks := tokenize(matchOn)
	if len(toks) == 0 {
		return
	}
	pos := len(l.entries)
	e := entry{id: id, name: name, program: program, tokens: toks}
	for _, t := range toks {
		if len(t) > 1 {
			e.full++
		}
		if k, ok := indexKey(t); ok {
			if b := l.index[k]; len(b) == 0 || b[len(b)-1] != pos {
				l.index[k] = append(b, pos)
			}
		}
	}
	l.entries = append(l.entries, e)
}

// LoadOFACDir loads SDN.CSV (required) and ALT.CSV (aliases, optional) from dir.
func LoadOFACDir(dir string) (*List, error) {
	l := &List{index: map[string][]int{}}
	primary := map[string]entry{} // ent_num -> primary entry, for alias attribution

	if err := readCSV(filepath.Join(dir, "SDN.CSV"), func(rec []string) {
		if len(rec) < 4 {
			return
		}
		id, name, program := field(rec[0]), field(rec[1]), field(rec[3])
		if _, err := strconv.Atoi(id); err != nil {
			return // not an OFAC row (ent_num is numeric): trailing EOF marker or wrong file
		}
		l.add(id, name, program, name)
		primary[id] = entry{id: id, name: name, program: program}
	}); err != nil {
		return nil, err
	}
	if len(l.entries) == 0 {
		return nil, fmt.Errorf("sanctions: SDN.CSV in %s has no OFAC rows", dir)
	}

	err := readCSV(filepath.Join(dir, "ALT.CSV"), func(rec []string) {
		if len(rec) < 4 {
			return
		}
		if p, ok := primary[field(rec[0])]; ok {
			// Index the alias, report the primary name so analysts see the listed party.
			l.add(p.id, p.name, p.program, field(rec[3]))
		}
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return l, nil
}

func readCSV(path string, fn func([]string)) error {
	f, err := os.Open(path) // #nosec G304 -- operator-configured list directory
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("sanctions: read %s: %w", path, err)
		}
		fn(rec)
	}
}

// field trims padding and OFAC's "-0-" null marker.
func field(s string) string {
	s = strings.TrimSpace(strings.Trim(s, "\x1a"))
	if s == "-0-" {
		return ""
	}
	return s
}

// Len reports the number of indexed names (primary names plus aliases).
func (l *List) Len() int { return len(l.entries) }

// Screen returns the best match for name, or a zero Match when no listed name
// shares a token prefix with it.
// Limit: scores every entry in the query's prefix buckets; common prefixes
// (the most frequent name starts) mean thousands of comparisons per name. Add a tighter
// index if p99 suffers.
func (l *List) Screen(name string) Match {
	q := tokenize(name)
	if l == nil || len(q) == 0 {
		return Match{}
	}
	keys := map[string]bool{}
	for _, t := range q {
		if k, ok := indexKey(t); ok {
			keys[k] = true
		}
	}
	// Count shared keys per entry first: an entry sharing one key with a
	// multi-key query can only score at partialCap, so scoring it is wasted
	// work unless the listed name is a single token.
	hits := map[int]int{}
	for k := range keys {
		for _, pos := range l.index[k] {
			hits[pos]++
		}
	}
	need := min(2, len(keys))
	var best Match
	bestPos := -1 // lowest position wins ties, so map order never picks the match
	for pos, n := range hits {
		e := l.entries[pos]
		if n < need && e.full > 1 {
			continue
		}
		if s := score(q, e); s > 0 && (s > best.Score || (s == best.Score && pos < bestPos)) {
			best, bestPos = Match{EntryID: e.id, Name: e.name, Program: e.program, Score: s}, pos
		}
	}
	return best
}

// Token pairs below tokenFloor count as no match, so "kestrel" never earns
// partial credit against "brannock". Tokens of four letters or fewer need
// shortFloor: one added letter in "ora" ("lora") is a different name.
const (
	tokenFloor = 0.85
	shortFloor = 0.93
	// minLenRatio rejects pairs of very different length, where Jaro-Winkler
	// over-rewards a shared prefix ("brant" vs "brannock" is 0.86).
	minLenRatio = 0.75
)

// partialCap holds a name to review level (below the starter kit's 0.88 hit
// band) when fewer than two of its listed tokens matched, so "Ali" or
// "J. Sample" alone never reads as a probable hit against a longer name.
const partialCap = 0.85

// score weights how fully the query is covered by the listed name (0.7) over
// how fully the listed name is covered by the query (0.3), so an omitted
// middle name or patronymic costs less than an extra unmatched query token.
func score(q [][]rune, e entry) float64 {
	qe, matched := coverage(q, e.tokens)
	eq, _ := coverage(e.tokens, q)
	s := 0.7*qe + 0.3*eq
	if matched < min(2, e.full) {
		s = min(s, partialCap)
	}
	return s
}

// coverage returns the mean, over tokens in a, of the best token similarity
// in b (0 below the floor), and how many distinct full (non-initial) tokens of
// b were matched.
func coverage(a, b [][]rune) (float64, int) {
	var sum float64
	var hit uint64 // bitset over b positions; tokens past 64 are not counted
	for _, x := range a {
		best, at := 0.0, 0
		for j, y := range b {
			if s := tokenSim(x, y); s > best {
				best, at = s, j
			}
		}
		if best > 0 {
			sum += best
			if len(x) > 1 && len(b[at]) > 1 && at < 64 {
				hit |= 1 << at
			}
		}
	}
	return sum / float64(len(a)), bits.OnesCount64(hit)
}

// initialSim is what an initial ("v") earns against a name starting with it:
// enough to keep "Jonathan Q. Sample" a hit, never a full-name match.
const initialSim = 0.9

// tokenSim scores two name tokens, returning 0 for anything below the floor.
func tokenSim(x, y []rune) float64 {
	if len(x) == 1 || len(y) == 1 {
		if x[0] == y[0] {
			return initialSim
		}
		return 0
	}
	short, long := min(len(x), len(y)), max(len(x), len(y))
	if float64(short)/float64(long) < minLenRatio {
		return 0
	}
	floor := tokenFloor
	if short <= 4 {
		floor = shortFloor
	}
	if s := jaroWinkler(x, y); s >= floor {
		return s
	}
	return 0
}

// jaroWinkler returns the Jaro-Winkler similarity of a and b in [0,1], with
// Winkler's prefix bonus applied only above the 0.7 boost threshold.
func jaroWinkler(a, b []rune) float64 {
	la, lb := len(a), len(b)
	if la == 0 || lb == 0 {
		return 0
	}
	if string(a) == string(b) {
		return 1
	}
	window := max(max(la, lb)/2-1, 0)
	var bufA, bufB [32]bool // names rarely exceed 32 letters; avoids a heap alloc per pair
	ma, mb := bufA[:], bufB[:]
	if la > len(ma) {
		ma = make([]bool, la)
	}
	if lb > len(mb) {
		mb = make([]bool, lb)
	}
	ma, mb = ma[:la], mb[:lb]
	matches := 0
	for i := range a {
		lo, hi := max(0, i-window), min(lb, i+window+1)
		for j := lo; j < hi; j++ {
			if !mb[j] && a[i] == b[j] {
				ma[i], mb[j] = true, true
				matches++
				break
			}
		}
	}
	if matches == 0 {
		return 0
	}
	transpositions, j := 0, 0
	for i := range a {
		if !ma[i] {
			continue
		}
		for !mb[j] {
			j++
		}
		if a[i] != b[j] {
			transpositions++
		}
		j++
	}
	m := float64(matches)
	jaro := (m/float64(la) + m/float64(lb) + (m-float64(transpositions)/2)/m) / 3
	if jaro <= 0.7 {
		return jaro
	}
	prefix := 0
	for prefix < min(4, la, lb) && a[prefix] == b[prefix] {
		prefix++
	}
	return jaro + float64(prefix)*0.1*(1-jaro)
}
