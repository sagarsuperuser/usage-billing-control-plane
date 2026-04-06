package api

import (
	"net/http"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
)

type createPricingMetricRequest struct {
	Key         string `json:"key" validate:"required,min=1,max=100"`
	Name        string `json:"name" validate:"required,min=1,max=200"`
	Unit        string `json:"unit" validate:"required,min=1,max=50"`
	Aggregation string `json:"aggregation" validate:"required,oneof=sum count max unique_count"`
	Currency    string `json:"currency" validate:"max=3"`
}

type createPlanRequest struct {
	Code            string   `json:"code" validate:"required,min=1,max=100"`
	Name            string   `json:"name" validate:"required,min=1,max=200"`
	Description     string   `json:"description" validate:"max=1000"`
	Currency        string   `json:"currency" validate:"required,len=3"`
	BillingInterval string   `json:"billing_interval" validate:"required,oneof=monthly yearly"`
	Status          string   `json:"status" validate:"oneof=draft active archived"`
	BaseAmountCents int64    `json:"base_amount_cents" validate:"gte=0"`
	MeterIDs        []string `json:"meter_ids"`
	AddOnIDs        []string `json:"add_on_ids"`
	CouponIDs       []string `json:"coupon_ids"`
}

type createTaxRequest struct {
	Code        string  `json:"code" validate:"required,min=1,max=100"`
	Name        string  `json:"name" validate:"required,min=1,max=200"`
	Description string  `json:"description" validate:"max=1000"`
	Status      string  `json:"status" validate:"oneof=active inactive"`
	Rate        float64 `json:"rate" validate:"gte=0,lte=1"`
}

type createAddOnRequest struct {
	Code            string `json:"code" validate:"required,min=1,max=100"`
	Name            string `json:"name" validate:"required,min=1,max=200"`
	Description     string `json:"description" validate:"max=1000"`
	Currency        string `json:"currency" validate:"required,len=3"`
	BillingInterval string `json:"billing_interval" validate:"required,oneof=monthly yearly"`
	Status          string `json:"status" validate:"oneof=draft active archived"`
	AmountCents     int64  `json:"amount_cents" validate:"gte=0"`
}

type createCouponRequest struct {
	Code              string     `json:"code" validate:"required,min=1,max=100"`
	Name              string     `json:"name" validate:"required,min=1,max=200"`
	Description       string     `json:"description" validate:"max=1000"`
	Status            string     `json:"status" validate:"oneof=draft active archived"`
	DiscountType      string     `json:"discount_type" validate:"required,oneof=percent_off amount_off"`
	Currency          string     `json:"currency" validate:"max=3"`
	AmountOffCents    int64      `json:"amount_off_cents" validate:"gte=0"`
	PercentOff        int        `json:"percent_off" validate:"gte=0,lte=100"`
	Frequency         string     `json:"frequency" validate:"required,oneof=forever once recurring"`
	FrequencyDuration int        `json:"frequency_duration" validate:"gte=0"`
	ExpirationAt      *time.Time `json:"expiration_at"`
}

// ── Metrics ─────────────────────────────────────────────────────────────

func (s *Server) listPricingMetrics(w http.ResponseWriter, r *http.Request) {
	if s.pricingMetricService == nil {
		writeError(w, http.StatusServiceUnavailable, "pricing metric service is required")
		return
	}
	metrics, err := s.pricingMetricService.ListMetrics(requestTenantID(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) createPricingMetric(w http.ResponseWriter, r *http.Request) {
	if s.pricingMetricService == nil {
		writeError(w, http.StatusServiceUnavailable, "pricing metric service is required")
		return
	}
	if s.meterSyncAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "Pricing updates are unavailable right now.")
		return
	}
	tenantID := requestTenantID(r)
	var req createPricingMetricRequest
	if err := decodeAndValidate(r, &req); err != nil {
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
		writeError(w, http.StatusBadGateway, "Pricing metric changes could not be applied right now.")
		return
	}
	writeJSON(w, http.StatusCreated, metric)
}

func (s *Server) getPricingMetric(w http.ResponseWriter, r *http.Request) {
	if s.pricingMetricService == nil {
		writeError(w, http.StatusServiceUnavailable, "pricing metric service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
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

// ── Plans ───────────────────────────────────────────────────────────────

func (s *Server) listPlans(w http.ResponseWriter, r *http.Request) {
	if s.planService == nil {
		writeError(w, http.StatusServiceUnavailable, "plan service is required")
		return
	}
	plans, err := s.planService.ListPlans(requestTenantID(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plans)
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	if s.planService == nil {
		writeError(w, http.StatusServiceUnavailable, "plan service is required")
		return
	}
	tenantID := requestTenantID(r)
	var req createPlanRequest
	if err := decodeAndValidate(r, &req); err != nil {
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
		CouponIDs:       req.CouponIDs,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request) {
	if s.planService == nil {
		writeError(w, http.StatusServiceUnavailable, "plan service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	var req service.UpdatePlanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plan, err := s.planService.UpdatePlan(r.Context(), requestTenantID(r), id, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) activatePlan(w http.ResponseWriter, r *http.Request) {
	if s.planService == nil {
		writeError(w, http.StatusServiceUnavailable, "plan service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	plan, err := s.planService.ActivatePlan(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) archivePlan(w http.ResponseWriter, r *http.Request) {
	if s.planService == nil {
		writeError(w, http.StatusServiceUnavailable, "plan service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	plan, err := s.planService.ArchivePlan(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) getPlan(w http.ResponseWriter, r *http.Request) {
	if s.planService == nil {
		writeError(w, http.StatusServiceUnavailable, "plan service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
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

// ── Taxes ───────────────────────────────────────────────────────────────

func (s *Server) listTaxes(w http.ResponseWriter, r *http.Request) {
	if s.taxService == nil {
		writeError(w, http.StatusServiceUnavailable, "tax service is required")
		return
	}
	items, err := s.taxService.ListTaxes(requestTenantID(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createTax(w http.ResponseWriter, r *http.Request) {
	if s.taxService == nil {
		writeError(w, http.StatusServiceUnavailable, "tax service is required")
		return
	}
	tenantID := requestTenantID(r)
	var req createTaxRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.taxService.CreateTax(r.Context(), domain.Tax{
		TenantID:    tenantID,
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Status:      domain.TaxStatus(req.Status),
		Rate:        req.Rate,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getTax(w http.ResponseWriter, r *http.Request) {
	if s.taxService == nil {
		writeError(w, http.StatusServiceUnavailable, "tax service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	item, err := s.taxService.GetTax(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// ── Add-Ons ─────────────────────────────────────────────────────────────

func (s *Server) listAddOns(w http.ResponseWriter, r *http.Request) {
	if s.addOnService == nil {
		writeError(w, http.StatusServiceUnavailable, "add-on service is required")
		return
	}
	items, err := s.addOnService.ListAddOns(requestTenantID(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createAddOn(w http.ResponseWriter, r *http.Request) {
	if s.addOnService == nil {
		writeError(w, http.StatusServiceUnavailable, "add-on service is required")
		return
	}
	tenantID := requestTenantID(r)
	var req createAddOnRequest
	if err := decodeAndValidate(r, &req); err != nil {
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
}

func (s *Server) getAddOn(w http.ResponseWriter, r *http.Request) {
	if s.addOnService == nil {
		writeError(w, http.StatusServiceUnavailable, "add-on service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
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

// ── Coupons ─────────────────────────────────────────────────────────────

func (s *Server) listCoupons(w http.ResponseWriter, r *http.Request) {
	if s.couponService == nil {
		writeError(w, http.StatusServiceUnavailable, "coupon service is required")
		return
	}
	items, err := s.couponService.ListCoupons(requestTenantID(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createCoupon(w http.ResponseWriter, r *http.Request) {
	if s.couponService == nil {
		writeError(w, http.StatusServiceUnavailable, "coupon service is required")
		return
	}
	tenantID := requestTenantID(r)
	var req createCouponRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.couponService.CreateCoupon(domain.Coupon{
		TenantID:          tenantID,
		Code:              req.Code,
		Name:              req.Name,
		Description:       req.Description,
		Status:            domain.CouponStatus(req.Status),
		DiscountType:      domain.CouponDiscountType(req.DiscountType),
		Currency:          req.Currency,
		AmountOffCents:    req.AmountOffCents,
		PercentOff:        req.PercentOff,
		Frequency:         domain.CouponFrequency(req.Frequency),
		FrequencyDuration: req.FrequencyDuration,
		ExpirationAt:      req.ExpirationAt,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getCoupon(w http.ResponseWriter, r *http.Request) {
	if s.couponService == nil {
		writeError(w, http.StatusServiceUnavailable, "coupon service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	item, err := s.couponService.GetCoupon(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
