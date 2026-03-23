package service

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

func TestNewLagoHTTPTransportRequiresConfig(t *testing.T) {
	t.Parallel()

	if _, err := NewLagoHTTPTransport(LagoClientConfig{}); err == nil {
		t.Fatalf("expected constructor error for missing config")
	}
}

func TestLagoInvoiceAdapterGetInvoice(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/invoices/inv_123" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoice":{"lago_id":"inv_123"}}`))
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	status, body, err := NewLagoInvoiceAdapter(transport).GetInvoice(context.Background(), "inv_123")
	if err != nil {
		t.Fatalf("proxy invoice by id: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "inv_123") {
		t.Fatalf("expected invoice body to contain invoice id, got %s", string(body))
	}
}

func TestLagoInvoiceAdapterListInvoices(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/invoices" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("customer_external_id"); got != "cust_123" {
			t.Fatalf("expected customer_external_id filter, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoices":[{"lago_id":"inv_123","number":"INV-123"}],"meta":{"current_page":1}}`))
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	query := url.Values{}
	query.Set("customer_external_id", "cust_123")
	status, body, err := NewLagoInvoiceAdapter(transport).ListInvoices(context.Background(), query)
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "INV-123") {
		t.Fatalf("expected invoice body to contain invoice number, got %s", string(body))
	}
}

func TestLagoCustomerBillingAdapter(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_123","external_id":"cust_123","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_123","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_123","external_id":"cust_123","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_123","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_123/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_123","is_default":true,"provider_method_id":"pm_123"}]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_123/checkout_url":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"checkout_url":"https://checkout.example.test/cust_123"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	adapter := NewLagoCustomerBillingAdapter(transport)
	status, body, err := adapter.UpsertCustomer(context.Background(), []byte(`{"customer":{"external_id":"cust_123"}}`))
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "lago_cust_123") {
		t.Fatalf("expected lago customer id in response, got %s", string(body))
	}

	status, body, err = adapter.GetCustomer(context.Background(), "cust_123")
	if err != nil {
		t.Fatalf("get customer: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "pcus_123") {
		t.Fatalf("expected provider customer id in response, got %s", string(body))
	}

	status, body, err = adapter.ListCustomerPaymentMethods(context.Background(), "cust_123")
	if err != nil {
		t.Fatalf("list customer payment methods: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "pm_123") {
		t.Fatalf("expected provider payment method id in response, got %s", string(body))
	}

	status, body, err = adapter.GenerateCustomerCheckoutURL(context.Background(), "cust_123")
	if err != nil {
		t.Fatalf("generate customer checkout url: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "checkout.example.test/cust_123") {
		t.Fatalf("expected checkout url in response, got %s", string(body))
	}
}

func TestLagoCustomerBillingAdapterSyncBillingEntitySettings(t *testing.T) {
	t.Parallel()

	var sawUpdate bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/billing_entities/be_us_primary":
			sawUpdate = true
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"net_payment_term":21`) {
				t.Fatalf("expected net_payment_term in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"tax_codes":["GST_IN"]`) {
				t.Fatalf("expected tax_codes in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"invoice_footer":"Wire details available on request."`) {
				t.Fatalf("expected invoice_footer in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"document_locale":"fr"`) {
				t.Fatalf("expected document_locale in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"invoice_grace_period":5`) {
				t.Fatalf("expected invoice_grace_period in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"document_numbering":"per_billing_entity"`) {
				t.Fatalf("expected document_numbering in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"document_number_prefix":"ALPHA-"`) {
				t.Fatalf("expected document_number_prefix in payload, got %s", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"billing_entity":{"code":"be_us_primary"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	netTerms := 21
	invoiceGracePeriodDays := 5
	err = NewLagoCustomerBillingAdapter(transport).SyncBillingEntitySettings(context.Background(), domain.WorkspaceBillingSettings{
		WorkspaceID:            "tenant_demo",
		BillingEntityCode:      "be_us_primary",
		NetPaymentTermDays:     &netTerms,
		TaxCodes:               []string{"gst_in"},
		InvoiceFooter:          "Wire details available on request.",
		DocumentLocale:         "fr",
		InvoiceGracePeriodDays: &invoiceGracePeriodDays,
		DocumentNumbering:      "per_billing_entity",
		DocumentNumberPrefix:   "ALPHA-",
	})
	if err != nil {
		t.Fatalf("sync billing entity settings: %v", err)
	}
	if !sawUpdate {
		t.Fatalf("expected update request to lago billing entities endpoint")
	}
}

func TestLagoTaxSyncAdapter(t *testing.T) {
	t.Parallel()

	var sawCreate bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/taxes":
			sawCreate = true
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"code":"gst_in"`) || !strings.Contains(payload, `"rate":18`) {
				t.Fatalf("unexpected tax payload: %s", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tax":{"code":"gst_in"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoTaxSyncAdapter(transport).SyncTax(context.Background(), domain.Tax{
		Code:        "gst_in",
		Name:        "GST India",
		Description: "India GST",
		Rate:        18,
	})
	if err != nil {
		t.Fatalf("sync tax: %v", err)
	}
	if !sawCreate {
		t.Fatalf("expected create request to lago taxes endpoint")
	}
}

func TestLagoPlanSyncAdapter(t *testing.T) {
	t.Parallel()

	var sawCreate bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/billable_metrics/api_calls":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"billable_metric":{"lago_id":"bm_123","code":"api_calls"}}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/plans":
			sawCreate = true
			w.Header().Set("Content-Type", "application/json")
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"code":"growth"`) {
				t.Fatalf("expected plan code in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"pay_in_advance":false`) {
				t.Fatalf("expected pay_in_advance false in plan payload, got %s", payload)
			}
			if !strings.Contains(payload, `"billable_metric_id":"bm_123"`) {
				t.Fatalf("expected billable metric id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"charge_model":"standard"`) {
				t.Fatalf("expected standard charge model in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"amount":"0.22"`) {
				t.Fatalf("expected decimal amount in payload, got %s", payload)
			}
			_, _ = w.Write([]byte(`{"plan":{"code":"growth"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoPlanSyncAdapter(transport, nil).SyncPlan(context.Background(), domain.Plan{
		Code:            "growth",
		Name:            "Growth",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		BaseAmountCents: 4900,
	}, []PlanSyncComponent{{
		Meter: domain.Meter{
			Key:  "api_calls",
			Name: "API Calls",
		},
		RatingRuleVersion: domain.RatingRuleVersion{
			Mode:            domain.PricingModeFlat,
			Currency:        "USD",
			FlatAmountCents: 22,
		},
	}})
	if err != nil {
		t.Fatalf("sync plan: %v", err)
	}
	if !sawCreate {
		t.Fatalf("expected create request to lago plans endpoint")
	}
}

func TestLagoSubscriptionSyncAdapter(t *testing.T) {
	t.Parallel()

	var sawCreate bool
	startedAt := time.Date(2026, time.January, 1, 12, 30, 0, 0, time.UTC)
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/subscriptions":
			sawCreate = true
			w.Header().Set("Content-Type", "application/json")
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"external_customer_id":"cust_123"`) {
				t.Fatalf("expected external customer id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"plan_code":"growth"`) {
				t.Fatalf("expected plan code in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"billing_time":"anniversary"`) {
				t.Fatalf("expected billing_time in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"external_id":"cust_123_growth"`) {
				t.Fatalf("expected external subscription id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"subscription_at":"2026-01-01T12:30:00Z"`) {
				t.Fatalf("expected subscription_at in payload, got %s", payload)
			}
			_, _ = w.Write([]byte(`{"subscription":{"external_id":"cust_123_growth"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoSubscriptionSyncAdapter(transport, nil).SyncSubscription(context.Background(),
		domain.Subscription{Code: "cust_123_growth", DisplayName: "Customer Growth", BillingTime: domain.SubscriptionBillingTimeAnniversary, StartedAt: &startedAt},
		domain.Customer{ExternalID: "cust_123"},
		domain.Plan{Code: "growth"},
	)
	if err != nil {
		t.Fatalf("sync subscription: %v", err)
	}
	if !sawCreate {
		t.Fatalf("expected create request to lago subscriptions endpoint")
	}
}

func TestLagoSubscriptionSyncAdapterFallsBackToUpdateForRename(t *testing.T) {
	t.Parallel()

	var sawUpdate bool
	startedAt := time.Date(2026, time.February, 15, 8, 0, 0, 0, time.UTC)
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/subscriptions":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"status":422,"error":"already_exists"}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/subscriptions/cust_123_growth":
			sawUpdate = true
			w.Header().Set("Content-Type", "application/json")
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if strings.Contains(payload, `"plan_code"`) {
				t.Fatalf("expected update payload to omit plan_code, got %s", payload)
			}
			if !strings.Contains(payload, `"name":"Customer Growth Renamed"`) {
				t.Fatalf("expected renamed subscription in update payload, got %s", payload)
			}
			if strings.Contains(payload, `"billing_time"`) {
				t.Fatalf("expected update payload to omit billing_time, got %s", payload)
			}
			if !strings.Contains(payload, `"subscription_at":"2026-02-15T08:00:00Z"`) {
				t.Fatalf("expected subscription_at in update payload, got %s", payload)
			}
			_, _ = w.Write([]byte(`{"subscription":{"external_id":"cust_123_growth"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoSubscriptionSyncAdapter(transport, nil).SyncSubscription(context.Background(),
		domain.Subscription{Code: "cust_123_growth", DisplayName: "Customer Growth Renamed", BillingTime: domain.SubscriptionBillingTimeAnniversary, StartedAt: &startedAt},
		domain.Customer{ExternalID: "cust_123"},
		domain.Plan{Code: "growth_v2"},
	)
	if err != nil {
		t.Fatalf("sync subscription rename: %v", err)
	}
	if !sawUpdate {
		t.Fatalf("expected update request to lago subscriptions endpoint")
	}
}

func TestLagoSubscriptionSyncAdapterArchivesSubscription(t *testing.T) {
	t.Parallel()

	var sawDelete bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/subscriptions/cust_123_growth":
			sawDelete = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"subscription":{"external_id":"cust_123_growth","status":"terminated"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoSubscriptionSyncAdapter(transport, nil).SyncSubscription(context.Background(),
		domain.Subscription{Code: "cust_123_growth", DisplayName: "Customer Growth", Status: domain.SubscriptionStatusArchived},
		domain.Customer{ExternalID: "cust_123"},
		domain.Plan{Code: "growth"},
	)
	if err != nil {
		t.Fatalf("archive subscription: %v", err)
	}
	if !sawDelete {
		t.Fatalf("expected delete request to lago terminate endpoint")
	}
}

func TestLagoPlanSyncAdapterSyncsCommercialArtifacts(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	tenantID := "tenant_lago_plan_sync_commercials"
	rule, err := repo.CreateRatingRuleVersion(domain.RatingRuleVersion{
		TenantID:        tenantID,
		RuleKey:         "rule_growth_api_calls",
		Name:            "Growth API Calls",
		Version:         1,
		LifecycleState:  domain.RatingRuleLifecycleActive,
		Mode:            domain.PricingModeFlat,
		Currency:        "USD",
		FlatAmountCents: 22,
	})
	if err != nil {
		t.Fatalf("create rule version: %v", err)
	}
	meter, err := repo.CreateMeter(domain.Meter{
		TenantID:            tenantID,
		Key:                 "api_calls_growth",
		Name:                "API Calls",
		Unit:                "request",
		Aggregation:         "sum",
		RatingRuleVersionID: rule.ID,
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}
	addOn, err := repo.CreateAddOn(domain.AddOn{
		TenantID:        tenantID,
		Code:            "priority_support",
		Name:            "Priority support",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		Status:          domain.AddOnStatusActive,
		AmountCents:     1500,
	})
	if err != nil {
		t.Fatalf("create add-on: %v", err)
	}
	coupon, err := repo.CreateCoupon(domain.Coupon{
		TenantID:          tenantID,
		Code:              "launch_20",
		Name:              "Launch 20",
		Status:            domain.CouponStatusActive,
		DiscountType:      domain.CouponDiscountTypePercentOff,
		PercentOff:        20,
		Frequency:         domain.CouponFrequencyRecurring,
		FrequencyDuration: 3,
	})
	if err != nil {
		t.Fatalf("create coupon: %v", err)
	}
	plan, err := repo.CreatePlan(domain.Plan{
		TenantID:        tenantID,
		Code:            "growth",
		Name:            "Growth",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		Status:          domain.PlanStatusActive,
		BaseAmountCents: 4900,
		MeterIDs:        []string{meter.ID},
		AddOnIDs:        []string{addOn.ID},
		CouponIDs:       []string{coupon.ID},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	var sawAddOn, sawFixedCharge, sawCoupon bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/billable_metrics/api_calls_growth":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"billable_metric":{"lago_id":"bm_123","code":"api_calls_growth"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/plans":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"plan":{"code":"growth"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/add_ons":
			sawAddOn = true
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"code":"priority_support"`) || !strings.Contains(payload, `"amount_cents":1500`) {
				t.Fatalf("unexpected add-on payload: %s", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"add_on":{"code":"priority_support"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/plans/growth/fixed_charges":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"fixed_charges":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/plans/growth/fixed_charges":
			sawFixedCharge = true
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"add_on_code":"priority_support"`) || !strings.Contains(payload, `"amount":"15.00"`) {
				t.Fatalf("unexpected fixed charge payload: %s", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"fixed_charge":{"code":"alpha_fc_growth_priority_support"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/coupons":
			sawCoupon = true
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"code":"launch_20"`) || !strings.Contains(payload, `"coupon_type":"percentage"`) || !strings.Contains(payload, `"plan_codes":["growth"]`) || !strings.Contains(payload, `"frequency":"recurring"`) || !strings.Contains(payload, `"frequency_duration":3`) {
				t.Fatalf("unexpected coupon payload: %s", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"coupon":{"code":"launch_20"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{BaseURL: lago.URL, APIKey: "test", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoPlanSyncAdapter(transport, repo).SyncPlan(context.Background(), plan, []PlanSyncComponent{{
		Meter: meter,
		RatingRuleVersion: domain.RatingRuleVersion{
			Mode:            domain.PricingModeFlat,
			Currency:        "USD",
			FlatAmountCents: 22,
		},
	}})
	if err != nil {
		t.Fatalf("sync plan commercials: %v", err)
	}
	if !sawAddOn || !sawFixedCharge || !sawCoupon {
		t.Fatalf("expected add-on, fixed charge, and coupon sync to run")
	}
}

func TestLagoSubscriptionSyncAdapterReconcilesAppliedCoupons(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	tenantID := "tenant_lago_subscription_coupon_reconcile"
	customer, err := repo.CreateCustomer(domain.Customer{
		TenantID:    tenantID,
		ExternalID:  "cust_reconcile_123",
		DisplayName: "Coupon Customer",
		Status:      domain.CustomerStatusActive,
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	rule, err := repo.CreateRatingRuleVersion(domain.RatingRuleVersion{
		TenantID:        tenantID,
		RuleKey:         "rule_growth_coupon",
		Name:            "Growth Rule",
		Version:         1,
		LifecycleState:  domain.RatingRuleLifecycleActive,
		Mode:            domain.PricingModeFlat,
		Currency:        "USD",
		FlatAmountCents: 100,
	})
	if err != nil {
		t.Fatalf("create rule version: %v", err)
	}
	meter, err := repo.CreateMeter(domain.Meter{
		TenantID:            tenantID,
		Key:                 "coupon_meter",
		Name:                "Coupon Meter",
		Unit:                "event",
		Aggregation:         "sum",
		RatingRuleVersionID: rule.ID,
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}
	desiredCoupon, err := repo.CreateCoupon(domain.Coupon{
		TenantID:          tenantID,
		Code:              "launch_20_runtime",
		Name:              "Launch 20 Runtime",
		Status:            domain.CouponStatusActive,
		DiscountType:      domain.CouponDiscountTypePercentOff,
		PercentOff:        20,
		Frequency:         domain.CouponFrequencyRecurring,
		FrequencyDuration: 3,
	})
	if err != nil {
		t.Fatalf("create desired coupon: %v", err)
	}
	staleCoupon, err := repo.CreateCoupon(domain.Coupon{
		TenantID:       tenantID,
		Code:           "stale_10_runtime",
		Name:           "Stale 10 Runtime",
		Status:         domain.CouponStatusActive,
		DiscountType:   domain.CouponDiscountTypeAmountOff,
		Currency:       "USD",
		AmountOffCents: 1000,
	})
	if err != nil {
		t.Fatalf("create stale coupon: %v", err)
	}
	plan, err := repo.CreatePlan(domain.Plan{
		TenantID:        tenantID,
		Code:            "growth_coupon_runtime",
		Name:            "Growth Coupon Runtime",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		Status:          domain.PlanStatusActive,
		BaseAmountCents: 4900,
		MeterIDs:        []string{meter.ID},
		CouponIDs:       []string{desiredCoupon.ID},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	subscription, err := repo.CreateSubscription(domain.Subscription{
		TenantID:    tenantID,
		Code:        "cust_reconcile_123_growth",
		DisplayName: "Coupon Subscription",
		CustomerID:  customer.ID,
		PlanID:      plan.ID,
		Status:      domain.SubscriptionStatusActive,
		BillingTime: domain.SubscriptionBillingTimeAnniversary,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	var sawCouponCreate, sawAppliedCouponCreate, sawAppliedCouponDelete bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/subscriptions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"subscription":{"external_id":"cust_reconcile_123_growth"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/coupons":
			sawCouponCreate = true
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"code":"launch_20_runtime"`) || !strings.Contains(payload, `"frequency":"recurring"`) {
				t.Fatalf("unexpected coupon sync payload: %s", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"coupon":{"code":"launch_20_runtime"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/applied_coupons":
			if got := r.URL.Query().Get("external_customer_id"); got != "cust_reconcile_123" {
				t.Fatalf("expected external_customer_id filter, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"applied_coupons":[{"lago_id":"ac_stale","coupon_code":"stale_10_runtime","external_customer_id":"cust_reconcile_123","status":"active"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/applied_coupons":
			sawAppliedCouponCreate = true
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"coupon_code":"launch_20_runtime"`) {
				t.Fatalf("unexpected applied coupon payload: %s", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"applied_coupon":{"lago_id":"ac_launch"}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/customers/cust_reconcile_123/applied_coupons/ac_stale":
			sawAppliedCouponDelete = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"applied_coupon":{"lago_id":"ac_stale","status":"terminated"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{BaseURL: lago.URL, APIKey: "test", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoSubscriptionSyncAdapter(transport, repo).SyncSubscription(context.Background(), subscription, customer, plan)
	if err != nil {
		t.Fatalf("sync subscription coupons: %v", err)
	}
	if !sawCouponCreate || !sawAppliedCouponCreate || !sawAppliedCouponDelete {
		t.Fatalf("expected coupon create=%t apply=%t delete=%t", sawCouponCreate, sawAppliedCouponCreate, sawAppliedCouponDelete)
	}
	if staleCoupon.Code != "stale_10_runtime" {
		t.Fatalf("expected stale coupon fixture to exist")
	}
}

func TestLagoSubscriptionSyncAdapterDoesNotReapplyRecurringCouponAfterTermination(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	tenantID := "tenant_lago_subscription_coupon_no_reapply"
	customer, err := repo.CreateCustomer(domain.Customer{
		TenantID:    tenantID,
		ExternalID:  "cust_coupon_terminated_123",
		DisplayName: "Coupon Customer",
		Status:      domain.CustomerStatusActive,
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	rule, err := repo.CreateRatingRuleVersion(domain.RatingRuleVersion{
		TenantID:        tenantID,
		RuleKey:         "rule_growth_coupon_no_reapply",
		Name:            "Growth Rule",
		Version:         1,
		LifecycleState:  domain.RatingRuleLifecycleActive,
		Mode:            domain.PricingModeFlat,
		Currency:        "USD",
		FlatAmountCents: 100,
	})
	if err != nil {
		t.Fatalf("create rule version: %v", err)
	}
	meter, err := repo.CreateMeter(domain.Meter{
		TenantID:            tenantID,
		Key:                 "coupon_meter_no_reapply",
		Name:                "Coupon Meter",
		Unit:                "event",
		Aggregation:         "sum",
		RatingRuleVersionID: rule.ID,
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}
	desiredCoupon, err := repo.CreateCoupon(domain.Coupon{
		TenantID:          tenantID,
		Code:              "launch_20_no_reapply",
		Name:              "Launch 20 Runtime",
		Status:            domain.CouponStatusActive,
		DiscountType:      domain.CouponDiscountTypePercentOff,
		PercentOff:        20,
		Frequency:         domain.CouponFrequencyRecurring,
		FrequencyDuration: 3,
	})
	if err != nil {
		t.Fatalf("create desired coupon: %v", err)
	}
	plan, err := repo.CreatePlan(domain.Plan{
		TenantID:        tenantID,
		Code:            "growth_coupon_no_reapply",
		Name:            "Growth Coupon Runtime",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		Status:          domain.PlanStatusActive,
		BaseAmountCents: 4900,
		MeterIDs:        []string{meter.ID},
		CouponIDs:       []string{desiredCoupon.ID},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	subscription, err := repo.CreateSubscription(domain.Subscription{
		TenantID:    tenantID,
		Code:        "cust_coupon_terminated_123_growth",
		DisplayName: "Coupon Subscription",
		CustomerID:  customer.ID,
		PlanID:      plan.ID,
		Status:      domain.SubscriptionStatusActive,
		BillingTime: domain.SubscriptionBillingTimeAnniversary,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	var sawAppliedCouponCreate bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/subscriptions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"subscription":{"external_id":"cust_coupon_terminated_123_growth"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/coupons":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"coupon":{"code":"launch_20_no_reapply"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/applied_coupons":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"applied_coupons":[{"lago_id":"ac_old","coupon_code":"launch_20_no_reapply","external_customer_id":"cust_coupon_terminated_123","status":"terminated"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/applied_coupons":
			sawAppliedCouponCreate = true
			http.Error(w, "should not reapply recurring coupon", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{BaseURL: lago.URL, APIKey: "test", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoSubscriptionSyncAdapter(transport, repo).SyncSubscription(context.Background(), subscription, customer, plan)
	if err != nil {
		t.Fatalf("sync subscription coupons: %v", err)
	}
	if sawAppliedCouponCreate {
		t.Fatalf("expected terminated recurring coupon not to be reapplied")
	}
}

func TestLagoUsageSyncAdapter(t *testing.T) {
	t.Parallel()

	var sawCreate bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/events":
			sawCreate = true
			w.Header().Set("Content-Type", "application/json")
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"code":"api_calls"`) {
				t.Fatalf("expected billable metric code in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"external_subscription_id":"cust_123_growth"`) {
				t.Fatalf("expected external subscription id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"transaction_id":"evt_sync_123"`) {
				t.Fatalf("expected transaction id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"value":"12"`) {
				t.Fatalf("expected quantity value in payload, got %s", payload)
			}
			_, _ = w.Write([]byte(`{"event":{"transaction_id":"evt_sync_123"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoUsageSyncAdapter(transport).SyncUsageEvent(context.Background(),
		domain.UsageEvent{ID: "evt_sync_123", Quantity: 12, Timestamp: time.Unix(1_700_000_000, 0).UTC()},
		domain.Meter{Key: "api_calls", Aggregation: "sum"},
		domain.Subscription{Code: "cust_123_growth"},
	)
	if err != nil {
		t.Fatalf("sync usage event: %v", err)
	}
	if !sawCreate {
		t.Fatalf("expected create request to lago events endpoint")
	}
}

func TestLagoAdaptersWithRealLago(t *testing.T) {
	baseURL := strings.TrimSpace(os.Getenv("TEST_LAGO_API_URL"))
	apiKey := strings.TrimSpace(os.Getenv("TEST_LAGO_API_KEY"))
	if baseURL == "" || apiKey == "" {
		t.Skip("TEST_LAGO_API_URL and TEST_LAGO_API_KEY are required for real Lago tests")
	}
	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoMeterSyncAdapter(transport).SyncMeter(context.Background(), domain.Meter{
		Key:                 "alpha_test_meter",
		Name:                "Alpha Test Meter",
		Aggregation:         "count",
		RatingRuleVersionID: "rrv_test",
	})
	if err != nil {
		t.Fatalf("sync meter with real lago: %v", err)
	}

	status, body, err := NewLagoInvoiceAdapter(transport).PreviewInvoice(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("proxy invoice preview with real lago: %v", err)
	}
	if status == 0 {
		t.Fatalf("expected non-zero status from lago preview proxy")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
		t.Fatalf("expected json response body, got %q", string(body))
	}
}
