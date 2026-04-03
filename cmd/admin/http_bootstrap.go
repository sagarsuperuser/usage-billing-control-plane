package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type customerSpec struct {
	ExternalID  string `json:"external_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type customerSpecValue []customerSpec

func (v *customerSpecValue) String() string {
	parts := make([]string, 0, len(*v))
	for _, spec := range *v {
		parts = append(parts, fmt.Sprintf("%s|%s|%s", spec.ExternalID, spec.DisplayName, spec.Email))
	}
	return strings.Join(parts, ",")
}

func (v *customerSpecValue) Set(value string) error {
	spec, err := parseCustomerSpec(value)
	if err != nil {
		return err
	}
	*v = append(*v, spec)
	return nil
}

func parseCustomerSpec(value string) (customerSpec, error) {
	parts := strings.Split(value, "|")
	if len(parts) != 3 {
		return customerSpec{}, fmt.Errorf("customer spec must be external_id|display_name|email")
	}
	spec := customerSpec{
		ExternalID:  strings.TrimSpace(parts[0]),
		DisplayName: strings.TrimSpace(parts[1]),
		Email:       strings.TrimSpace(parts[2]),
	}
	if spec.ExternalID == "" || spec.DisplayName == "" || spec.Email == "" {
		return customerSpec{}, fmt.Errorf("customer spec fields must be non-empty")
	}
	return spec, nil
}

type tenantMappingResult struct {
	Command                 string    `json:"command"`
	AppliedAt               time.Time `json:"applied_at"`
	TenantID                string    `json:"tenant_id"`
	StatusCode              int       `json:"status_code"`
}

type workspaceBillingBindingResult struct {
	Command                     string    `json:"command"`
	AppliedAt                   time.Time `json:"applied_at"`
	TenantID                    string    `json:"tenant_id"`
	BillingProviderConnectionID string    `json:"billing_provider_connection_id"`
	StatusCode                  int       `json:"status_code"`
}

type customerEnsureResult struct {
	Command         string                 `json:"command"`
	ConflictIsError bool                   `json:"conflict_is_error"`
	AppliedAt       time.Time              `json:"applied_at"`
	Results         []customerEnsureRecord `json:"results"`
}

type customerEnsureRecord struct {
	ExternalID  string `json:"external_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Outcome     string `json:"outcome"`
	StatusCode  int    `json:"status_code"`
}

type billingProfileResult struct {
	Command            string          `json:"command"`
	AppliedAt          time.Time       `json:"applied_at"`
	CustomerExternalID string          `json:"customer_external_id"`
	StatusCode         int             `json:"status_code"`
	Response           json.RawMessage `json:"response"`
}

type customerPayload struct {
	ExternalID  string `json:"external_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type billingProfilePayload struct {
	LegalName           string `json:"legal_name"`
	Email               string `json:"email"`
	BillingAddressLine1 string `json:"billing_address_line1"`
	BillingCity         string `json:"billing_city"`
	BillingState        string `json:"billing_state,omitempty"`
	BillingPostalCode   string `json:"billing_postal_code"`
	BillingCountry      string `json:"billing_country"`
	Currency            string `json:"currency"`
}

func runEnsureTenantWorkspaceBilling(logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("ensure-tenant-workspace-billing", flag.ExitOnError)
	var (
		baseURL      string
		platformKey  string
		tenantID     string
		connectionID string
		output       string
	)
	fs.StringVar(&baseURL, "alpha-api-base-url", firstNonEmpty(os.Getenv("ALPHA_API_BASE_URL"), os.Getenv("PLAYWRIGHT_LIVE_API_BASE_URL"), "http://127.0.0.1:8080"), "alpha api base url")
	fs.StringVar(&platformKey, "platform-api-key", firstNonEmpty(os.Getenv("PLATFORM_ADMIN_API_KEY"), os.Getenv("PLAYWRIGHT_LIVE_PLATFORM_API_KEY")), "platform admin api key")
	fs.StringVar(&tenantID, "tenant-id", firstNonEmpty(os.Getenv("TARGET_TENANT_ID"), "default"), "tenant id")
	fs.StringVar(&connectionID, "billing-provider-connection-id", "", "billing provider connection id")
	fs.StringVar(&output, "output", "json", "output format: json or text")
	_ = fs.Parse(args)

	if strings.TrimSpace(connectionID) == "" {
		fatal(logger, "billing-provider-connection-id is required")
	}
	if strings.TrimSpace(platformKey) == "" {
		fatal(logger, "platform-api-key is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	statusCode, _, err := httpJSON(ctx, newAdminHTTPClient(), http.MethodPatch,
		joinURL(baseURL, "/internal/tenants/"+url.PathEscape(strings.TrimSpace(tenantID))+"/workspace-billing"),
		map[string]string{
			"billing_provider_connection_id": strings.TrimSpace(connectionID),
		},
		platformKey,
	)
	if err != nil {
		fatal(logger, "patch tenant workspace billing", "error", err)
	}
	if statusCode != http.StatusOK {
		fatal(logger, "patch tenant workspace billing failed", "status_code", statusCode)
	}

	res := workspaceBillingBindingResult{
		Command:                     "ensure-tenant-workspace-billing",
		AppliedAt:                   time.Now().UTC(),
		TenantID:                    strings.TrimSpace(tenantID),
		BillingProviderConnectionID: strings.TrimSpace(connectionID),
		StatusCode:                  statusCode,
	}
	writeStructuredOutput(output, res)
}

func runEnsureAlphaCustomers(logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("ensure-alpha-customers", flag.ExitOnError)
	var (
		baseURL         string
		writerKey       string
		output          string
		conflictIsError bool
		customers       customerSpecValue
	)
	fs.StringVar(&baseURL, "alpha-api-base-url", firstNonEmpty(os.Getenv("ALPHA_API_BASE_URL"), os.Getenv("PLAYWRIGHT_LIVE_API_BASE_URL"), "http://127.0.0.1:8080"), "alpha api base url")
	fs.StringVar(&writerKey, "writer-api-key", firstNonEmpty(os.Getenv("ALPHA_WRITER_API_KEY"), os.Getenv("PLAYWRIGHT_LIVE_WRITER_API_KEY")), "writer api key")
	fs.StringVar(&output, "output", "json", "output format: json or text")
	fs.BoolVar(&conflictIsError, "conflict-is-error", false, "treat existing customers as an error")
	fs.Var(&customers, "customer", "customer spec external_id|display_name|email (repeatable)")
	_ = fs.Parse(args)

	if len(customers) == 0 {
		fatal(logger, "at least one -customer is required")
	}
	if strings.TrimSpace(writerKey) == "" {
		fatal(logger, "writer-api-key is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAdminHTTPClient()
	res := customerEnsureResult{
		Command:         "ensure-alpha-customers",
		ConflictIsError: conflictIsError,
		AppliedAt:       time.Now().UTC(),
		Results:         make([]customerEnsureRecord, 0, len(customers)),
	}
	for _, spec := range customers {
		statusCode, _, err := httpJSON(ctx, client, http.MethodPost, joinURL(baseURL, "/v1/customers"), customerPayload(spec), writerKey)
		if err != nil {
			fatal(logger, "create customer", "external_id", spec.ExternalID, "error", err)
		}
		record := customerEnsureRecord{
			ExternalID:  spec.ExternalID,
			DisplayName: spec.DisplayName,
			Email:       spec.Email,
			StatusCode:  statusCode,
		}
		switch statusCode {
		case http.StatusCreated:
			record.Outcome = "created"
		case http.StatusConflict:
			if conflictIsError {
				fatal(logger, "customer already exists", "external_id", spec.ExternalID, "status_code", statusCode)
			}
			record.Outcome = "existing"
		default:
			fatal(logger, "create customer failed", "external_id", spec.ExternalID, "status_code", statusCode)
		}
		res.Results = append(res.Results, record)
	}
	writeStructuredOutput(output, res)
}

func runUpsertCustomerBillingProfile(logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("upsert-customer-billing-profile", flag.ExitOnError)
	var (
		baseURL            string
		writerKey          string
		customerExternalID string
		legalName          string
		email              string
		line1              string
		city               string
		state              string
		postalCode         string
		country            string
		currency           string
		output             string
	)
	fs.StringVar(&baseURL, "alpha-api-base-url", firstNonEmpty(os.Getenv("ALPHA_API_BASE_URL"), os.Getenv("PLAYWRIGHT_LIVE_API_BASE_URL"), "http://127.0.0.1:8080"), "alpha api base url")
	fs.StringVar(&writerKey, "writer-api-key", firstNonEmpty(os.Getenv("ALPHA_WRITER_API_KEY"), os.Getenv("PLAYWRIGHT_LIVE_WRITER_API_KEY")), "writer api key")
	fs.StringVar(&customerExternalID, "customer-external-id", "", "customer external id")
	fs.StringVar(&legalName, "legal-name", "", "billing legal name")
	fs.StringVar(&email, "email", "", "billing email")
	fs.StringVar(&line1, "billing-address-line1", "", "billing address line 1")
	fs.StringVar(&city, "billing-city", "", "billing city")
	fs.StringVar(&state, "billing-state", "", "billing state")
	fs.StringVar(&postalCode, "billing-postal-code", "", "billing postal code")
	fs.StringVar(&country, "billing-country", "", "billing country")
	fs.StringVar(&currency, "currency", "USD", "billing currency")
	fs.StringVar(&output, "output", "json", "output format: json or text")
	_ = fs.Parse(args)

	if strings.TrimSpace(writerKey) == "" {
		fatal(logger, "writer-api-key is required")
	}
	for _, pair := range []struct{ key, value string }{
		{"customer-external-id", customerExternalID},
		{"legal-name", legalName},
		{"email", email},
		{"billing-address-line1", line1},
		{"billing-city", city},
		{"billing-postal-code", postalCode},
		{"billing-country", country},
	} {
		if strings.TrimSpace(pair.value) == "" {
			fatal(logger, pair.key+" is required")
		}
	}

	payload := billingProfilePayload{
		LegalName:           strings.TrimSpace(legalName),
		Email:               strings.TrimSpace(email),
		BillingAddressLine1: strings.TrimSpace(line1),
		BillingCity:         strings.TrimSpace(city),
		BillingState:        strings.TrimSpace(state),
		BillingPostalCode:   strings.TrimSpace(postalCode),
		BillingCountry:      strings.TrimSpace(country),
		Currency:            strings.TrimSpace(currency),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	statusCode, body, err := httpJSON(ctx, newAdminHTTPClient(), http.MethodPut, joinURL(baseURL, "/v1/customers/"+url.PathEscape(strings.TrimSpace(customerExternalID))+"/billing-profile"), payload, writerKey)
	if err != nil {
		fatal(logger, "upsert billing profile", "customer_external_id", customerExternalID, "error", err)
	}
	if statusCode != http.StatusOK {
		fatal(logger, "upsert billing profile failed", "customer_external_id", customerExternalID, "status_code", statusCode)
	}
	res := billingProfileResult{
		Command:            "upsert-customer-billing-profile",
		AppliedAt:          time.Now().UTC(),
		CustomerExternalID: strings.TrimSpace(customerExternalID),
		StatusCode:         statusCode,
		Response:           json.RawMessage(body),
	}
	writeStructuredOutput(output, res)
}

func newAdminHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func httpJSON(ctx context.Context, client *http.Client, method, target string, body any, apiKey string) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", strings.TrimSpace(apiKey))
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, respBody, nil
}

func writeStructuredOutput(format string, v any) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "text":
		fmt.Fprintf(os.Stdout, "%+v\n", v)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			fmt.Fprintf(os.Stderr, "encode output: %v\n", err)
			os.Exit(1)
		}
	}
}

func joinURL(baseURL, path string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return trimmed + path
}
