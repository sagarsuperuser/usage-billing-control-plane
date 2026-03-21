package service

import "testing"

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
