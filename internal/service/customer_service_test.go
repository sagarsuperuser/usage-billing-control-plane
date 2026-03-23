package service

import (
	"encoding/json"
	"testing"

	"usage-billing-control-plane/internal/domain"
)

func TestInferPaymentProviderFromCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		want    string
		wantErr bool
	}{
		{name: "plain stripe", code: "stripe_test", want: "stripe"},
		{name: "namespaced stripe", code: "alpha_stripe_test_bpc_53564373212e3a6d", want: "stripe"},
		{name: "unsupported", code: "unknown_gateway", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := inferPaymentProviderFromCode(tc.code)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for code %q", tc.code)
				}
				return
			}
			if err != nil {
				t.Fatalf("infer payment provider from code %q: %v", tc.code, err)
			}
			if got != tc.want {
				t.Fatalf("infer payment provider from code %q = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

func TestBuildLagoCustomerPayloadIncludesCommercialExecutionFields(t *testing.T) {
	payload, err := buildLagoCustomerPayload(
		"alpha_stripe_test_bpc_demo",
		domain.Customer{ExternalID: "cust_123", DisplayName: "Customer 123"},
		domain.CustomerBillingProfile{
			Email:         "billing@example.com",
			Currency:      "USD",
			Country:       "US",
			TaxIdentifier: "US123456",
			TaxCodes:      []string{"us_sales"},
		},
		domain.CustomerPaymentSetup{PaymentMethodType: "card"},
		domain.WorkspaceBillingSettings{
			WorkspaceID:       "tenant_demo",
			BillingEntityCode: "be_us_primary",
			NetPaymentTermDays: func() *int {
				v := 14
				return &v
			}(),
		},
	)
	if err != nil {
		t.Fatalf("build payload: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	customer, ok := decoded["customer"].(map[string]any)
	if !ok {
		t.Fatalf("expected customer object")
	}
	if got, _ := customer["billing_entity_code"].(string); got != "be_us_primary" {
		t.Fatalf("expected billing_entity_code be_us_primary, got %q", got)
	}
	if got, ok := customer["net_payment_term"].(float64); !ok || int(got) != 14 {
		t.Fatalf("expected net_payment_term 14, got %#v", customer["net_payment_term"])
	}
	if got, _ := customer["tax_identification_number"].(string); got != "US123456" {
		t.Fatalf("expected tax_identification_number US123456, got %q", got)
	}
	taxCodes, ok := customer["tax_codes"].([]any)
	if !ok || len(taxCodes) != 1 || taxCodes[0] != "US_SALES" {
		t.Fatalf("expected tax_codes [US_SALES], got %#v", customer["tax_codes"])
	}
}
