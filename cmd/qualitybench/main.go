// Command qualitybench measures the detection quality of a rule set on the
// labelled PaySim CSV (https://www.kaggle.com/datasets/ealaxi/paysim1).
//
// It runs each row through Osprey's own rule engine, typology engine and
// decision processor in-process, and prints precision, recall, the result of
// each rule alone and a threshold sweep as JSON.
//
// By default velocity variables are 0 and rows run in parallel. With -velocity,
// rows are replayed in file order into a temporary SQLite store with the event
// clock on, so velocity_* variables see the transactions of the hour before
// each row. PaySim steps are hours: a row's timestamp is 2017-01-01 plus step hours.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/repository"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
	"github.com/opensource-finance/osprey/internal/velocity"
)

var epoch = time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)

type row struct {
	n          int
	step       int
	typ        string
	amount     float64
	ob, nb     float64
	orig, dest string
	fraud      bool
}

type counts struct{ TP, FP, TN, FN int64 }

func (c *counts) add(alert, fraud bool) {
	switch {
	case alert && fraud:
		c.TP++
	case alert && !fraud:
		c.FP++
	case !alert && fraud:
		c.FN++
	default:
		c.TN++
	}
}

func (c counts) summary() map[string]any {
	p := float64(c.TP) / max(float64(c.TP+c.FP), 1)
	r := float64(c.TP) / max(float64(c.TP+c.FN), 1)
	f1 := 0.0
	if p+r > 0 {
		f1 = 2 * p * r / (p + r)
	}
	return map[string]any{"tp": c.TP, "fp": c.FP, "tn": c.TN, "fn": c.FN,
		"alerts": c.TP + c.FP, "precision": p, "recall": r, "f1": f1,
		"fpr": float64(c.FP) / max(float64(c.FP+c.TN), 1)}
}

func loadJSON[T any](path, key string) ([]T, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrapped map[string]json.RawMessage
	if json.Unmarshal(b, &wrapped) == nil && wrapped[key] != nil {
		b = wrapped[key]
	}
	var out []T
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return out, nil
}

type result struct {
	r      row
	alert  bool
	score  float64
	fired  []string
	failed bool
}

type bench struct {
	engine    *rules.Engine
	typEngine *rules.TypologyEngine
	proc      *tadp.Processor
	repo      domain.Repository // nil unless -velocity
}

func (b *bench) evaluate(ctx context.Context, x row) (result, error) {
	in := &rules.EvaluateInput{
		TenantID: "bench", TxID: strconv.Itoa(x.n), Type: x.typ, DebtorID: x.orig, CreditorID: x.dest,
		Amount: x.amount, Currency: "USD",
		AdditionalData: map[string]any{"old_balance": x.ob, "new_balance": x.nb},
	}
	if b.repo != nil {
		ts := epoch.Add(time.Duration(x.step) * time.Hour)
		if err := b.repo.SaveTransaction(ctx, "bench", &domain.Transaction{
			ID: in.TxID, TenantID: "bench", Type: x.typ, DebtorID: x.orig, CreditorID: x.dest,
			Amount: x.amount, Currency: "USD", Timestamp: ts, CreatedAt: time.Now().UTC(),
		}); err != nil {
			return result{}, err
		}
		in.VelocityWindow = 3600
		in.EventTime = ts
	}
	rr, err := b.engine.EvaluateAll(ctx, in)
	if err != nil {
		return result{}, err
	}
	var tr []domain.TypologyResult
	if b.typEngine != nil {
		tr = b.typEngine.EvaluateTypologies(rr)
	}
	ev := b.proc.Process(ctx, &tadp.DecisionInput{RuleResults: rr, TypologyResults: tr})
	res := result{r: x, alert: ev.Status == domain.StatusAlert, score: ev.Score}
	for _, q := range rr {
		if q.Score > 0 {
			res.fired = append(res.fired, q.RuleID)
		}
		if q.SubRuleRef == domain.RuleOutcomeFail {
			res.failed = true
		}
	}
	return res, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "qualitybench:", err)
		os.Exit(1)
	}
}

func run() error {
	csvPath := flag.String("csv", "", "PaySim CSV")
	rulesPath := flag.String("rules", "", "rules JSON")
	typPath := flag.String("typologies", "", "typologies JSON (compliance mode)")
	limit := flag.Int("limit", 0, "max rows (0 = all)")
	scoring := flag.String("scoring", tadp.ScoringSum, "detection scoring: sum or average")
	threshold := flag.Float64("threshold", 0.7, "detection alert threshold")
	withVelocity := flag.Bool("velocity", false, "replay rows in order into a temporary store so velocity_* variables work (slower)")
	dbPath := flag.String("db", "", "with -velocity: SQLite file to keep (default: a temporary file)")
	cpuProfile := flag.String("cpuprofile", "", "write a CPU profile to this file")
	flag.Parse()
	if *cpuProfile != "" {
		pf, err := os.Create(*cpuProfile)
		if err != nil {
			return err
		}
		if err := pprof.StartCPUProfile(pf); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}
	if *csvPath == "" || *rulesPath == "" {
		flag.Usage()
		return fmt.Errorf("-csv and -rules are required")
	}

	ctx := context.Background()
	b := &bench{}
	var err error
	if *withVelocity {
		path := *dbPath
		if path == "" {
			dir, err := os.MkdirTemp("", "qualitybench")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			path = filepath.Join(dir, "bench.db")
		}
		b.repo, err = repository.New(domain.RepositoryConfig{Driver: "sqlite", SQLitePath: path})
		if err != nil {
			return err
		}
		defer func() { _ = b.repo.Close() }()
		svc := velocity.NewService(b.repo, nil)
		b.engine, err = rules.NewEngine(nil, 1)
		if err != nil {
			return err
		}
		b.engine.SetAggregatesGetter(func(ctx context.Context, tenantID, entityID string, windowSecs int, at time.Time) (rules.VelocityAggregates, error) {
			a, err := svc.GetAggregatesAt(ctx, tenantID, entityID, windowSecs, at)
			return rules.VelocityAggregates{
				Count: a.Count, AmountSum: a.AmountSum, DistinctCreditors: a.DistinctCreditors,
				InboundCount: a.InboundCount, InboundAmountSum: a.InboundAmountSum,
			}, err
		})
		b.engine.SetEventClock(true)
	} else if b.engine, err = rules.NewEngine(nil, 1); err != nil {
		return err
	}

	cfgs, err := loadJSON[*domain.RuleConfig](*rulesPath, "rules")
	if err != nil {
		return err
	}
	for _, c := range cfgs {
		c.Enabled = true
	}
	if err := b.engine.LoadRules(cfgs); err != nil {
		return err
	}
	b.proc = tadp.NewProcessor()
	if *typPath != "" {
		b.proc = tadp.NewComplianceProcessor()
		b.typEngine = rules.NewTypologyEngine()
		ts, err := loadJSON[*domain.Typology](*typPath, "typologies")
		if err != nil {
			return err
		}
		for _, t := range ts {
			t.Enabled = true
		}
		b.typEngine.LoadTypologies(ts)
	}
	b.proc.Scoring = *scoring
	b.proc.AlertThreshold = *threshold

	f, err := os.Open(*csvPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	if _, err := r.Read(); err != nil {
		return err
	}

	workers := runtime.NumCPU()
	if *withVelocity {
		workers = 1 // order matters: each row must see the rows before it
	}
	in := make(chan row, 4096)
	out := make(chan result, 4096)
	errCh := make(chan error, workers+1)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for x := range in {
				res, err := b.evaluate(ctx, x)
				if err != nil {
					errCh <- err
					return
				}
				out <- res
			}
		})
	}
	go func() {
		defer func() {
			close(in)
			wg.Wait()
			close(out)
		}()
		for n := 0; *limit == 0 || n < *limit; n++ {
			rec, err := r.Read()
			if err == io.EOF {
				return
			}
			if err != nil {
				errCh <- err
				return
			}
			p := func(i int) float64 { v, _ := strconv.ParseFloat(rec[i], 64); return v }
			step, _ := strconv.Atoi(rec[0])
			in <- row{n: n, step: step, typ: rec[1], amount: p(2), orig: rec[3], ob: p(4), nb: p(5), dest: rec[6], fraud: rec[9] == "1"}
		}
	}()

	var all counts
	byType := map[string]*counts{}
	ruleFire := map[string]*counts{} // rule fired = "alert"
	thresholds := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	sweep := make([]counts, len(thresholds))
	start := time.Now()
	for res := range out {
		all.add(res.alert, res.r.fraud)
		if byType[res.r.typ] == nil {
			byType[res.r.typ] = &counts{}
		}
		byType[res.r.typ].add(res.alert, res.r.fraud)
		fired := map[string]bool{}
		for _, id := range res.fired {
			fired[id] = true
		}
		for _, c := range cfgs {
			if ruleFire[c.ID] == nil {
				ruleFire[c.ID] = &counts{}
			}
			ruleFire[c.ID].add(fired[c.ID], res.r.fraud)
		}
		for i, t := range thresholds {
			sweep[i].add(res.failed || res.score >= t, res.r.fraud)
		}
	}
	select {
	case err := <-errCh:
		return err
	default:
	}

	report := map[string]any{"overall": all.summary(), "seconds": time.Since(start).Seconds(),
		"scoring": *scoring, "threshold": *threshold, "velocity": *withVelocity}
	bt := map[string]any{}
	for k, v := range byType {
		bt[k] = v.summary()
	}
	report["by_type"] = bt
	rf := map[string]any{}
	ids := make([]string, 0, len(ruleFire))
	for k := range ruleFire {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	for _, k := range ids {
		rf[k] = ruleFire[k].summary()
	}
	report["rule_alone"] = rf
	sw := []any{}
	for i, t := range thresholds {
		s := sweep[i].summary()
		s["threshold"] = t
		sw = append(sw, s)
	}
	report["threshold_sweep"] = sw
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	return enc.Encode(report)
}
