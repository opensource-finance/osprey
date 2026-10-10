package rules

import (
	"testing"

	"github.com/opensource-finance/osprey/internal/domain"
)

func TestMatchBand(t *testing.T) {
	zero, half, one, eightTenths := 0.0, 0.5, 1.0, 0.8

	tests := []struct {
		name       string
		score      float64
		bands      []domain.RuleBand
		wantRef    string
		wantReason string
	}{
		{
			name:  "closed last band score equals upper returns last band outcome (the bug)",
			score: 1.0,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
				{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "High amount fraud"},
			},
			wantRef:    domain.RuleOutcomeFail,
			wantReason: "High amount fraud",
		},
		{
			name:  "score strictly below last band upper matches inside last band",
			score: 0.7,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
				{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "High amount fraud"},
			},
			wantRef:    domain.RuleOutcomeFail,
			wantReason: "High amount fraud",
		},
		{
			name:  "score inside first band matches first band",
			score: 0.25,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
				{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "High amount fraud"},
			},
			wantRef:    domain.RuleOutcomePass,
			wantReason: "Normal",
		},
		{
			name:  "score at boundary between two bands defers to next band (non-last continue)",
			score: 0.5,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
				{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "High amount fraud"},
			},
			wantRef:    domain.RuleOutcomeFail,
			wantReason: "High amount fraud",
		},
		{
			name:  "open-ended last band matches score above all finite uppers",
			score: 1.0,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
				{LowerLimit: &one, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeFail, Reason: "High amount"},
			},
			wantRef:    domain.RuleOutcomeFail,
			wantReason: "High amount",
		},
		{
			name:  "single closed band score equals upper matches the last (and only) band",
			score: 1.0,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "Fraud"},
			},
			wantRef:    domain.RuleOutcomeFail,
			wantReason: "Fraud",
		},
		{
			name:  "single open band matches any score at or above lower",
			score: 0.9,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: nil, SubRuleRef: domain.RuleOutcomeFail, Reason: "Fraud"},
			},
			wantRef:    domain.RuleOutcomeFail,
			wantReason: "Fraud",
		},
		{
			name:  "score below all band lowers falls through to default pass",
			score: -0.1,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
				{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "Fraud"},
			},
			wantRef:    domain.RuleOutcomePass,
			wantReason: "no matching band",
		},
		{
			name:  "score above last band upper with a gap defaults to pass (no coverage)",
			score: 2.0,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
				{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "Fraud"},
			},
			wantRef:    domain.RuleOutcomePass,
			wantReason: "no matching band",
		},
		{
			name:       "empty bands defaults to pass",
			score:      1.0,
			bands:      []domain.RuleBand{},
			wantRef:    domain.RuleOutcomePass,
			wantReason: "no matching band",
		},
		{
			name:  "three bands score at second band upper defers to third band",
			score: 0.8,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
				{LowerLimit: &half, UpperLimit: &eightTenths, SubRuleRef: domain.RuleOutcomeReview, Reason: "Elevated"},
				{LowerLimit: &eightTenths, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "Fraud"},
			},
			wantRef:    domain.RuleOutcomeFail,
			wantReason: "Fraud",
		},
		{
			name:  "closed last band with review outcome still returns review (not just fail)",
			score: 1.0,
			bands: []domain.RuleBand{
				{LowerLimit: &zero, UpperLimit: &half, SubRuleRef: domain.RuleOutcomePass, Reason: "Normal"},
				{LowerLimit: &half, UpperLimit: &one, SubRuleRef: domain.RuleOutcomeReview, Reason: "Review me"},
			},
			wantRef:    domain.RuleOutcomeReview,
			wantReason: "Review me",
		},
		{
			name:  "no lower limit defaults to zero lower bound closed last band",
			score: 1.0,
			bands: []domain.RuleBand{
				{UpperLimit: &one, SubRuleRef: domain.RuleOutcomeFail, Reason: "Fraud"},
			},
			wantRef:    domain.RuleOutcomeFail,
			wantReason: "Fraud",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRef, gotReason := matchBand(tt.score, tt.bands)
			if gotRef != tt.wantRef {
				t.Errorf("matchBand ref = %q, want %q", gotRef, tt.wantRef)
			}
			if gotReason != tt.wantReason {
				t.Errorf("matchBand reason = %q, want %q", gotReason, tt.wantReason)
			}
		})
	}
}
