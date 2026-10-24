package sanctions

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestJaroWinkler(t *testing.T) {
	// Reference values from Winkler (1990).
	for _, c := range []struct {
		a, b string
		want float64
	}{
		{"martha", "marhta", 0.9611},
		{"dwayne", "duane", 0.84},
		{"abc", "xyz", 0},
		{"same", "same", 1},
	} {
		if got := jaroWinkler([]rune(c.a), []rune(c.b)); math.Abs(got-c.want) > 0.001 {
			t.Errorf("jaroWinkler(%q,%q) = %.4f, want %.4f", c.a, c.b, got, c.want)
		}
	}
}

func TestScreen(t *testing.T) {
	l, err := LoadOFACDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() != 8 {
		t.Fatalf("Len = %d, want 8 (7 primary + 1 alias)", l.Len())
	}

	// All fixture names are invented.
	// Reordered, recased, punctuation dropped: exact after normalization.
	if m := l.Screen("quill jonathan sample"); m.Score != 1 || m.EntryID != "1002" || m.Program != "TEST-A" {
		t.Errorf("reordered name: %+v", m)
	}
	// Alias hits report the primary listed name.
	if m := l.Screen("Northern Fictional Bank"); m.Score != 1 || m.Name != "BANCO FICTICIO DEL NORTE" {
		t.Errorf("alias: %+v", m)
	}
	// Typo still scores high.
	if m := l.Screen("Jonathon Quill Sample"); m.Score < 0.9 || m.EntryID != "1002" {
		t.Errorf("typo: %+v", m)
	}
	// Shares only a surname: well below a hit threshold.
	if m := l.Screen("Alice Sample"); m.Score >= 0.85 {
		t.Errorf("partial overlap scored too high: %+v", m)
	}
	// Two of three tokens match but "kestrel" vs "brannock" earns no partial credit.
	if m := l.Screen("Tobias Ellwood Kestrel"); m.Score >= 0.85 {
		t.Errorf("token floor not applied: %+v", m)
	}
	// Omitted middle name is a review, not a miss.
	if m := l.Screen("Tobias Brannock"); m.Score < 0.85 || m.Score >= 1 {
		t.Errorf("omitted middle name: %+v", m)
	}
	// An initial matches the name it abbreviates.
	if m := l.Screen("Jonathan Q. Sample"); m.Score < 0.92 || m.EntryID != "1002" {
		t.Errorf("initial: %+v", m)
	}
	// Spelling variants that share no exact token still meet through the prefix index.
	if m := l.Screen("Maximillian Delacourt"); m.Score < 0.88 || m.EntryID != "1004" {
		t.Errorf("spelling variant: %+v", m)
	}
	// Two-letter tokens are indexed.
	if m := l.Screen("Lo Qi"); m.Score != 1 || m.EntryID != "1005" {
		t.Errorf("short tokens: %+v", m)
	}
	// One matched token against a longer listed name stays at review level.
	if m := l.Screen("Orin"); m.Score > partialCap {
		t.Errorf("single token not capped: %+v", m)
	}
	if m := l.Screen("J. Sample"); m.Score > partialCap {
		t.Errorf("initial + surname not capped: %+v", m)
	}
	// A one-token listed name still hits on its own.
	if m := l.Screen("Greyfenworks"); m.Score != 1 || m.EntryID != "1007" {
		t.Errorf("one-token entity: %+v", m)
	}
	// No shared token: no candidate at all.
	if m := l.Screen("Jane Doe"); m != (Match{}) {
		t.Errorf("unrelated name: %+v", m)
	}
}

func TestLoadRejectsNonOFACFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SDN.CSV"), []byte("<html>,error,page,body\nname,type,program,title\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOFACDir(dir); err == nil {
		t.Fatal("want error for a file with no numeric ent_num rows")
	}
}

func TestLoadMissingSDN(t *testing.T) {
	if _, err := LoadOFACDir(t.TempDir()); err == nil {
		t.Fatal("want error when SDN.CSV is missing")
	}
}
