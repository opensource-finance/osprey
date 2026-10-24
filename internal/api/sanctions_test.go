package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
	"github.com/opensource-finance/osprey/internal/rules"
	"github.com/opensource-finance/osprey/internal/tadp"
)

func TestEvaluateScreensPartyNames(t *testing.T) {
	engine, _ := rules.NewEngine(nil, 2)
	zero, hit := 0.0, 0.88
	if err := engine.LoadRule(&domain.RuleConfig{
		ID: "sanctions-001", Expression: "sanctions_score", Weight: 0.1, Enabled: true,
		Bands: []domain.RuleBand{
			{LowerLimit: &zero, UpperLimit: &hit, SubRuleRef: domain.RuleOutcomePass},
			{LowerLimit: &hit, SubRuleRef: domain.RuleOutcomeFail, Reason: "Probable OFAC SDN match"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	var screened []string
	engine.SetSanctionsScreener(func(name string) (float64, string) {
		screened = append(screened, name)
		if name == "Bad Actor" {
			return 0.97, "ACTOR, Bad"
		}
		return 0, ""
	})
	server := NewServer(domain.ServerConfig{Host: "localhost", Port: 8080}, nil, nil, nil,
		engine, rules.NewTypologyEngine(), tadp.NewProcessor(), "test-v1", domain.ModeDetection)

	body, _ := json.Marshal(TransactionRequest{
		Type:     "transfer",
		Debtor:   PartyInfo{ID: "d1", AccountID: "a1", Name: "Alice Clean"},
		Creditor: PartyInfo{ID: "c1", AccountID: "a2", Name: "Bad Actor"},
		Amount:   AmountInfo{Value: 10, Currency: "USD"},
	})
	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-001")
	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	var resp EvaluateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if len(screened) != 2 || screened[0] != "Alice Clean" || screened[1] != "Bad Actor" {
		t.Errorf("screened names = %q, want debtor and creditor", screened)
	}
	if resp.Status != domain.StatusAlert {
		t.Errorf("status = %s, want ALRT for a creditor SDN hit", resp.Status)
	}
	if len(resp.Reasons) != 1 || resp.Reasons[0] != "Probable OFAC SDN match: ACTOR, Bad" {
		t.Errorf("reasons = %q, want the matched entry named", resp.Reasons)
	}
}
