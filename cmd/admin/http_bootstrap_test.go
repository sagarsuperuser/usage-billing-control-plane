package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCustomerSpec(t *testing.T) {
	t.Parallel()

	spec, err := parseCustomerSpec("cust_123|Acme Corp|billing@acme.test")
	if err != nil {
		t.Fatalf("parseCustomerSpec returned error: %v", err)
	}
	if spec.ExternalID != "cust_123" {
		t.Fatalf("unexpected external id: %q", spec.ExternalID)
	}
	if spec.DisplayName != "Acme Corp" {
		t.Fatalf("unexpected display name: %q", spec.DisplayName)
	}
	if spec.Email != "billing@acme.test" {
		t.Fatalf("unexpected email: %q", spec.Email)
	}
}

func TestParseCustomerSpecRejectsBadFormat(t *testing.T) {
	t.Parallel()

	cases := []string{
		"cust_123|Acme Corp",
		"cust_123|Acme Corp|",
		"cust_123||billing@acme.test",
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			t.Parallel()
			if _, err := parseCustomerSpec(tc); err == nil {
				t.Fatalf("expected error for %q", tc)
			}
		})
	}
}

func TestHTTPJSONSetsHeadersAndBody(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name string `json:"name"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("unexpected Accept header: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected Content-Type header: %q", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "writer-key" {
			t.Fatalf("unexpected X-API-Key header: %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		defer r.Body.Close()

		var got payload
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		if got.Name != "alpha" {
			t.Fatalf("unexpected request payload: %+v", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	statusCode, body, err := httpJSON(context.Background(), server.Client(), http.MethodPost, server.URL+"/v1/test", payload{Name: "alpha"}, "writer-key")
	if err != nil {
		t.Fatalf("httpJSON returned error: %v", err)
	}
	if statusCode != http.StatusCreated {
		t.Fatalf("unexpected status code: %d", statusCode)
	}
	if strings.TrimSpace(string(body)) != `{"ok":true}` {
		t.Fatalf("unexpected response body: %s", string(body))
	}
}

func TestJoinURL(t *testing.T) {
	t.Parallel()

	got := joinURL("https://api.example.com///", "/v1/customers")
	if got != "https://api.example.com/v1/customers" {
		t.Fatalf("unexpected joined url: %q", got)
	}
}
