package domain

import "testing"

func TestComputeAmountCentsFlat(t *testing.T) {
	rule := RatingRuleVersion{Mode: PricingModeFlat, FlatAmountCents: 500}

	amount, err := ComputeAmountCents(rule, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if amount != 500 {
		t.Fatalf("expected 500, got %d", amount)
	}
}

func TestComputeAmountCentsGraduated(t *testing.T) {
	rule := RatingRuleVersion{
		Mode: PricingModeGraduated,
		GraduatedTiers: []RatingTier{
			{UpTo: 100, UnitAmountCents: 2},
			{UpTo: 200, UnitAmountCents: 1},
			{UpTo: 0, UnitAmountCents: 1},
		},
	}

	amount, err := ComputeAmountCents(rule, 150)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if amount != 250 {
		t.Fatalf("expected 250, got %d", amount)
	}
}

func TestComputeAmountCentsPackage(t *testing.T) {
	rule := RatingRuleVersion{
		Mode:                   PricingModePackage,
		PackageSize:            100,
		PackageAmountCents:     1000,
		OverageUnitAmountCents: 5,
	}

	amount, err := ComputeAmountCents(rule, 230)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if amount != 2150 {
		t.Fatalf("expected 2150, got %d", amount)
	}
}
