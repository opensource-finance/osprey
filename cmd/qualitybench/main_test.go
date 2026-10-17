package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const testCSV = `step,type,amount,nameOrig,oldbalanceOrg,newbalanceOrig,nameDest,oldbalanceDest,newbalanceDest,isFraud,isFlaggedFraud
1,PAYMENT,100,C1,1000,900,M1,0,0,0,0
1,TRANSFER,500,C2,500,0,C9,0,0,1,0
1,CASH_OUT,300,C9,0,0,C8,0,0,0,0
2,TRANSFER,800,C3,800,0,C7,0,0,0,0
`

const testRules = `{"rules": [
  {"id": "drain", "expression": "old_balance > 0.0 && new_balance == 0.0", "weight": 0.5,
   "bands": [{"lowerLimit": 1.0, "subRuleRef": ".review", "reason": "drain"},
             {"lowerLimit": 0.0, "upperLimit": 1.0, "subRuleRef": ".pass", "reason": "ok"}]},
  {"id": "inbound", "expression": "velocity_inbound_count >= 1", "weight": 0.5,
   "bands": [{"lowerLimit": 1.0, "subRuleRef": ".review", "reason": "inbound"},
             {"lowerLimit": 0.0, "upperLimit": 1.0, "subRuleRef": ".pass", "reason": "ok"}]}
]}`

func writeFixtures(t *testing.T) options {
	t.Helper()
	dir := t.TempDir()
	o := options{csvPath: filepath.Join(dir, "paysim.csv"), rulesPath: filepath.Join(dir, "rules.json"), scoring: "sum", threshold: 0.5}
	if err := os.WriteFile(o.csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(o.rulesPath, []byte(testRules), 0o600); err != nil {
		t.Fatal(err)
	}
	return o
}

func overall(t *testing.T, report map[string]any) map[string]any {
	t.Helper()
	o, ok := report["overall"].(map[string]any)
	if !ok {
		t.Fatalf("report has no overall: %v", report)
	}
	return o
}

func TestBenchmarkWithoutVelocity(t *testing.T) {
	report, err := benchmark(context.Background(), writeFixtures(t))
	if err != nil {
		t.Fatal(err)
	}
	// Drain alerts on rows 2 and 4. Velocity is 0, so row 3 does not alert.
	o := overall(t, report)
	if o["tp"] != int64(1) || o["fp"] != int64(1) || o["tn"] != int64(2) || o["fn"] != int64(0) {
		t.Fatalf("overall = %v, want tp 1, fp 1, tn 2, fn 0", o)
	}
}

func TestBenchmarkWithVelocityReplaysInOrder(t *testing.T) {
	o := writeFixtures(t)
	o.velocity = true
	report, err := benchmark(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	// C9 received 500 from C2 in step 1, so its CASH_OUT in the same hour
	// matches the inbound rule and alerts too.
	got := overall(t, report)
	if got["alerts"] != int64(3) {
		t.Fatalf("overall = %v, want 3 alerts", got)
	}
	rule, ok := report["rule_alone"].(map[string]any)["inbound"].(map[string]any)
	if !ok || rule["alerts"] != int64(1) {
		t.Fatalf("inbound rule alone = %v, want 1 alert", rule)
	}
}

func TestLoadJSONAcceptsWrappedAndBareArrays(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, "bare.json")
	if err := os.WriteFile(bare, []byte(`[{"id": "a"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadJSON[map[string]any](bare, "rules")
	if err != nil || len(got) != 1 {
		t.Fatalf("bare array: %v, %v", got, err)
	}
	if _, err := loadJSON[map[string]any](filepath.Join(dir, "missing.json"), "rules"); err == nil {
		t.Fatal("missing file: want an error")
	}
}
