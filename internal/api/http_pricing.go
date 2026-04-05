package api

import (
	"net/http"
	"strings"
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

func (s *Server) handlePricingMetrics(w http.ResponseWriter, r *http.Request) {
	if s.pricingMetricService == nil {
		writeError(w, http.StatusServiceUnavailable, "pricing metric service is required")
		return
	}
	if s.meterSyncAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "Pricing updates are unavailable right now.")
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
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleTaxes(w http.ResponseWriter, r *http.Request) {
	if s.taxService == nil {
		writeError(w, http.StatusServiceUnavailable, "tax service is required")
		return
	}
	tenantID := requestTenantID(r)
	switch r.Method {
	case http.MethodGet:
		items, err := s.taxService.ListTaxes(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
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
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleTaxByID(w http.ResponseWriter, r *http.Request) {
	if s.taxService == nil {
		writeError(w, http.StatusServiceUnavailable, "tax service is required")
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/taxes/")
	if strings.TrimSpace(id) == "" {
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

func (s *Server) handleCoupons(w http.ResponseWriter, r *http.Request) {
	if s.couponService == nil {
		writeError(w, http.StatusServiceUnavailable, "coupon service is required")
		return
	}
	tenantID := requestTenantID(r)
	switch r.Method {
	case http.MethodGet:
		items, err := s.couponService.ListCoupons(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
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
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleCouponByID(w http.ResponseWriter, r *http.Request) {
	if s.couponService == nil {
		writeError(w, http.StatusServiceUnavailable, "coupon service is required")
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/coupons/")
	if strings.TrimSpace(id) == "" {
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
