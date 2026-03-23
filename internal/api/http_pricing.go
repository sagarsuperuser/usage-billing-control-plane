package api

import (
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
)

type createPricingMetricRequest struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Unit        string `json:"unit"`
	Aggregation string `json:"aggregation"`
	Currency    string `json:"currency"`
}

type createPlanRequest struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Currency        string   `json:"currency"`
	BillingInterval string   `json:"billing_interval"`
	Status          string   `json:"status"`
	BaseAmountCents int64    `json:"base_amount_cents"`
	MeterIDs        []string `json:"meter_ids"`
	AddOnIDs        []string `json:"add_on_ids"`
}

type createAddOnRequest struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Currency        string `json:"currency"`
	BillingInterval string `json:"billing_interval"`
	Status          string `json:"status"`
	AmountCents     int64  `json:"amount_cents"`
}

func (s *Server) handlePricingMetrics(w http.ResponseWriter, r *http.Request) {
	if s.pricingMetricService == nil {
		writeError(w, http.StatusServiceUnavailable, "pricing metric service is required")
		return
	}
	if s.meterSyncAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "meter sync adapter is required")
		return
	}

	tenantID := requestTenantID(r)
	switch r.Method {
	case http.MethodGet:
		metrics, err := s.pricingMetricService.ListMetrics(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, metrics)
	case http.MethodPost:
		var req createPricingMetricRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		metric, err := s.pricingMetricService.CreateMetric(service.CreatePricingMetricInput{
			TenantID:    tenantID,
			Key:         req.Key,
			Name:        req.Name,
			Unit:        req.Unit,
			Aggregation: req.Aggregation,
			Currency:    req.Currency,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if err := s.meterSyncAdapter.SyncMeter(r.Context(), metric); err != nil {
			writeError(w, http.StatusBadGateway, "metric created but lago sync failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, metric)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handlePricingMetricByID(w http.ResponseWriter, r *http.Request) {
	if s.pricingMetricService == nil {
		writeError(w, http.StatusServiceUnavailable, "pricing metric service is required")
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/pricing/metrics/")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	metric, err := s.pricingMetricService.GetMetric(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, metric)
}

func (s *Server) handlePlans(w http.ResponseWriter, r *http.Request) {
	if s.planService == nil {
		writeError(w, http.StatusServiceUnavailable, "plan service is required")
		return
	}
	tenantID := requestTenantID(r)
	switch r.Method {
	case http.MethodGet:
		plans, err := s.planService.ListPlans(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, plans)
	case http.MethodPost:
		var req createPlanRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		plan, err := s.planService.CreatePlan(r.Context(), domain.Plan{
			TenantID:        tenantID,
			Code:            req.Code,
			Name:            req.Name,
			Description:     req.Description,
			Currency:        req.Currency,
			BillingInterval: domain.BillingInterval(req.BillingInterval),
			Status:          domain.PlanStatus(req.Status),
			BaseAmountCents: req.BaseAmountCents,
			MeterIDs:        req.MeterIDs,
			AddOnIDs:        req.AddOnIDs,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, plan)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handlePlanByID(w http.ResponseWriter, r *http.Request) {
	if s.planService == nil {
		writeError(w, http.StatusServiceUnavailable, "plan service is required")
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/plans/")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	plan, err := s.planService.GetPlan(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) handleAddOns(w http.ResponseWriter, r *http.Request) {
	if s.addOnService == nil {
		writeError(w, http.StatusServiceUnavailable, "add-on service is required")
		return
	}
	tenantID := requestTenantID(r)
	switch r.Method {
	case http.MethodGet:
		items, err := s.addOnService.ListAddOns(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var req createAddOnRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := s.addOnService.CreateAddOn(domain.AddOn{
			TenantID:        tenantID,
			Code:            req.Code,
			Name:            req.Name,
			Description:     req.Description,
			Currency:        req.Currency,
			BillingInterval: domain.BillingInterval(req.BillingInterval),
			Status:          domain.AddOnStatus(req.Status),
			AmountCents:     req.AmountCents,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleAddOnByID(w http.ResponseWriter, r *http.Request) {
	if s.addOnService == nil {
		writeError(w, http.StatusServiceUnavailable, "add-on service is required")
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/add-ons/")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	item, err := s.addOnService.GetAddOn(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
