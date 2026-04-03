package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreateCustomer(input domain.Customer) (domain.Customer, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = normalizeOptionalText(input.Email)
	input.Status = normalizeCustomerStatus(input.Status)
	if input.ExternalID == "" {
		return domain.Customer{}, fmt.Errorf("validation failed: external_id is required")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.ExternalID
	}
	if input.ID == "" {
		input.ID = newID("cust")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Customer{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO customers (id, tenant_id, external_id, display_name, email, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)
		RETURNING id, tenant_id, external_id, display_name, email, status, created_at, updated_at`,
		input.ID, input.TenantID, input.ExternalID, input.DisplayName, input.Email, string(input.Status), input.CreatedAt, input.UpdatedAt)
	out, err := scanCustomer(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Customer{}, ErrAlreadyExists
		}
		return domain.Customer{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Customer{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetCustomer(tenantID, id string) (domain.Customer, error) {
	tenantID = normalizeTenantID(tenantID)
	id = strings.TrimSpace(id)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Customer{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, external_id, display_name, email, status, created_at, updated_at FROM customers WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	out, err := scanCustomer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Customer{}, ErrNotFound
		}
		return domain.Customer{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Customer{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetCustomerByExternalID(tenantID, externalID string) (domain.Customer, error) {
	tenantID = normalizeTenantID(tenantID)
	externalID = strings.TrimSpace(externalID)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Customer{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, external_id, display_name, email, status, created_at, updated_at FROM customers WHERE tenant_id = $1 AND external_id = $2`, tenantID, externalID)
	out, err := scanCustomer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Customer{}, ErrNotFound
		}
		return domain.Customer{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Customer{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListCustomers(filter CustomerListFilter) ([]domain.Customer, error) {
	filter.TenantID = normalizeTenantID(filter.TenantID)
	filter.Status = strings.TrimSpace(strings.ToLower(filter.Status))
	filter.ExternalID = strings.TrimSpace(filter.ExternalID)
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, filter.TenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"tenant_id = $1"}
	args := []any{filter.TenantID}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.ExternalID != "" {
		args = append(args, filter.ExternalID)
		clauses = append(clauses, fmt.Sprintf("external_id = $%d", len(args)))
	}
	args = append(args, filter.Limit, filter.Offset)
	query := `SELECT id, tenant_id, external_id, display_name, email, status, created_at, updated_at FROM customers WHERE ` + strings.Join(clauses, " AND ") + fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Customer, 0)
	for rows.Next() {
		item, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) UpdateCustomer(input domain.Customer) (domain.Customer, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.ID = strings.TrimSpace(input.ID)
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = normalizeOptionalText(input.Email)
	input.Status = normalizeCustomerStatus(input.Status)
	if input.ID == "" {
		return domain.Customer{}, fmt.Errorf("validation failed: customer id is required")
	}
	if input.ExternalID == "" {
		return domain.Customer{}, fmt.Errorf("validation failed: external_id is required")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.ExternalID
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Customer{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `UPDATE customers SET external_id = $1, display_name = $2, email = NULLIF($3,''), status = $4, updated_at = $5 WHERE tenant_id = $6 AND id = $7 RETURNING id, tenant_id, external_id, display_name, email, status, created_at, updated_at`,
		input.ExternalID, input.DisplayName, input.Email, string(input.Status), input.UpdatedAt, input.TenantID, input.ID)
	out, err := scanCustomer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Customer{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.Customer{}, ErrAlreadyExists
		}
		return domain.Customer{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Customer{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertCustomerBillingProfile(input domain.CustomerBillingProfile) (domain.CustomerBillingProfile, error) {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.LegalName = normalizeOptionalText(input.LegalName)
	input.Email = normalizeOptionalText(input.Email)
	input.Phone = normalizeOptionalText(input.Phone)
	input.AddressLine1 = normalizeOptionalText(input.AddressLine1)
	input.AddressLine2 = normalizeOptionalText(input.AddressLine2)
	input.City = normalizeOptionalText(input.City)
	input.State = normalizeOptionalText(input.State)
	input.PostalCode = normalizeOptionalText(input.PostalCode)
	input.Country = normalizeOptionalText(input.Country)
	input.Currency = normalizeOptionalText(input.Currency)
	input.TaxIdentifier = normalizeOptionalText(input.TaxIdentifier)
	input.TaxCodes = normalizeStringList(input.TaxCodes)
	input.ProviderCode = normalizeOptionalText(input.ProviderCode)
	input.ProfileStatus = normalizeBillingProfileStatus(input.ProfileStatus)
	input.LastSyncError = normalizeOptionalText(input.LastSyncError)
	if input.CustomerID == "" {
		return domain.CustomerBillingProfile{}, fmt.Errorf("validation failed: customer_id is required")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `INSERT INTO customer_billing_profiles (customer_id, tenant_id, legal_name, email, phone, billing_address_line1, billing_address_line2, billing_city, billing_state, billing_postal_code, billing_country, currency, tax_identifier, tax_codes, provider_code, profile_status, last_synced_at, last_sync_error, created_at, updated_at)
	VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),$14,NULLIF($15,''),$16,$17,NULLIF($18,''),$19,$20)
	ON CONFLICT (customer_id) DO UPDATE SET
	 legal_name = EXCLUDED.legal_name,
	 email = EXCLUDED.email,
	 phone = EXCLUDED.phone,
	 billing_address_line1 = EXCLUDED.billing_address_line1,
	 billing_address_line2 = EXCLUDED.billing_address_line2,
	 billing_city = EXCLUDED.billing_city,
	 billing_state = EXCLUDED.billing_state,
	 billing_postal_code = EXCLUDED.billing_postal_code,
	 billing_country = EXCLUDED.billing_country,
	 currency = EXCLUDED.currency,
	 tax_identifier = EXCLUDED.tax_identifier,
	 tax_codes = EXCLUDED.tax_codes,
	 provider_code = EXCLUDED.provider_code,
	 profile_status = EXCLUDED.profile_status,
	 last_synced_at = EXCLUDED.last_synced_at,
	 last_sync_error = EXCLUDED.last_sync_error,
	 updated_at = EXCLUDED.updated_at
	RETURNING customer_id, tenant_id, legal_name, email, phone, billing_address_line1, billing_address_line2, billing_city, billing_state, billing_postal_code, billing_country, currency, tax_identifier, tax_codes, provider_code, profile_status, last_synced_at, last_sync_error, created_at, updated_at`,
		input.CustomerID, input.TenantID, input.LegalName, input.Email, input.Phone, input.AddressLine1, input.AddressLine2, input.City, input.State, input.PostalCode, input.Country, input.Currency, input.TaxIdentifier, pq.Array(input.TaxCodes), input.ProviderCode, string(input.ProfileStatus), input.LastSyncedAt, input.LastSyncError, input.CreatedAt, input.UpdatedAt)
	out, err := scanCustomerBillingProfile(row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.CustomerBillingProfile{}, ErrNotFound
		}
		return domain.CustomerBillingProfile{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetCustomerBillingProfile(tenantID, customerID string) (domain.CustomerBillingProfile, error) {
	tenantID = normalizeTenantID(tenantID)
	customerID = strings.TrimSpace(customerID)
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `SELECT customer_id, tenant_id, legal_name, email, phone, billing_address_line1, billing_address_line2, billing_city, billing_state, billing_postal_code, billing_country, currency, tax_identifier, tax_codes, provider_code, profile_status, last_synced_at, last_sync_error, created_at, updated_at FROM customer_billing_profiles WHERE tenant_id = $1 AND customer_id = $2`, tenantID, customerID)
	out, err := scanCustomerBillingProfile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.CustomerBillingProfile{}, ErrNotFound
		}
		return domain.CustomerBillingProfile{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	return out, nil
}


func (s *PostgresStore) UpsertCustomerPaymentSetup(input domain.CustomerPaymentSetup) (domain.CustomerPaymentSetup, error) {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.SetupStatus = normalizePaymentSetupStatus(input.SetupStatus)
	input.PaymentMethodType = normalizeOptionalText(input.PaymentMethodType)
	input.ProviderCustomerReference = normalizeOptionalText(input.ProviderCustomerReference)
	input.ProviderPaymentMethodReference = normalizeOptionalText(input.ProviderPaymentMethodReference)
	input.LastVerificationResult = normalizeOptionalText(input.LastVerificationResult)
	input.LastVerificationError = normalizeOptionalText(input.LastVerificationError)
	input.LastRequestStatus = normalizePaymentSetupRequestStatus(input.LastRequestStatus)
	input.LastRequestKind = normalizeOptionalText(input.LastRequestKind)
	input.LastRequestToEmail = strings.ToLower(normalizeOptionalText(input.LastRequestToEmail))
	input.LastRequestError = normalizeOptionalText(input.LastRequestError)
	if input.CustomerID == "" {
		return domain.CustomerPaymentSetup{}, fmt.Errorf("validation failed: customer_id is required")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `INSERT INTO customer_payment_setup (customer_id, tenant_id, setup_status, default_payment_method_present, payment_method_type, provider_customer_reference, provider_payment_method_reference, last_verified_at, last_verification_result, last_verification_error, last_request_status, last_request_kind, last_request_to_email, last_request_sent_at, last_request_error, created_at, updated_at)
	VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),$8,NULLIF($9,''),NULLIF($10,''),$11,NULLIF($12,''),NULLIF($13,''),$14,NULLIF($15,''),$16,$17)
	ON CONFLICT (customer_id) DO UPDATE SET
	 setup_status = EXCLUDED.setup_status,
	 default_payment_method_present = EXCLUDED.default_payment_method_present,
	 payment_method_type = EXCLUDED.payment_method_type,
	 provider_customer_reference = EXCLUDED.provider_customer_reference,
	 provider_payment_method_reference = EXCLUDED.provider_payment_method_reference,
	 last_verified_at = EXCLUDED.last_verified_at,
	 last_verification_result = EXCLUDED.last_verification_result,
	 last_verification_error = EXCLUDED.last_verification_error,
	 last_request_status = EXCLUDED.last_request_status,
	 last_request_kind = EXCLUDED.last_request_kind,
	 last_request_to_email = EXCLUDED.last_request_to_email,
	 last_request_sent_at = EXCLUDED.last_request_sent_at,
	 last_request_error = EXCLUDED.last_request_error,
	 updated_at = EXCLUDED.updated_at
	RETURNING customer_id, tenant_id, setup_status, default_payment_method_present, payment_method_type, provider_customer_reference, provider_payment_method_reference, last_verified_at, last_verification_result, last_verification_error, last_request_status, last_request_kind, last_request_to_email, last_request_sent_at, last_request_error, created_at, updated_at`,
		input.CustomerID, input.TenantID, string(input.SetupStatus), input.DefaultPaymentMethodPresent, input.PaymentMethodType, input.ProviderCustomerReference, input.ProviderPaymentMethodReference, input.LastVerifiedAt, input.LastVerificationResult, input.LastVerificationError, string(input.LastRequestStatus), input.LastRequestKind, input.LastRequestToEmail, input.LastRequestSentAt, input.LastRequestError, input.CreatedAt, input.UpdatedAt)
	out, err := scanCustomerPaymentSetup(row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.CustomerPaymentSetup{}, ErrNotFound
		}
		return domain.CustomerPaymentSetup{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetCustomerPaymentSetup(tenantID, customerID string) (domain.CustomerPaymentSetup, error) {
	tenantID = normalizeTenantID(tenantID)
	customerID = strings.TrimSpace(customerID)
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `SELECT customer_id, tenant_id, setup_status, default_payment_method_present, payment_method_type, provider_customer_reference, provider_payment_method_reference, last_verified_at, last_verification_result, last_verification_error, last_request_status, last_request_kind, last_request_to_email, last_request_sent_at, last_request_error, created_at, updated_at FROM customer_payment_setup WHERE tenant_id = $1 AND customer_id = $2`, tenantID, customerID)
	out, err := scanCustomerPaymentSetup(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.CustomerPaymentSetup{}, ErrNotFound
		}
		return domain.CustomerPaymentSetup{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	return out, nil
}


func scanCustomer(s rowScanner) (domain.Customer, error) {
	var out domain.Customer
	var email sql.NullString
	var status string
	if err := s.Scan(&out.ID, &out.TenantID, &out.ExternalID, &out.DisplayName, &email, &status, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.Customer{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.ExternalID = strings.TrimSpace(out.ExternalID)
	out.DisplayName = strings.TrimSpace(out.DisplayName)
	if email.Valid {
		out.Email = normalizeOptionalText(email.String)
	}
	out.Status = normalizeCustomerStatus(domain.CustomerStatus(status))
	return out, nil
}


func scanCustomerBillingProfile(s rowScanner) (domain.CustomerBillingProfile, error) {
	var out domain.CustomerBillingProfile
	var legalName, email, phone, line1, line2, city, state, postal, country, currency, taxID, providerCode, lastSyncError sql.NullString
	var taxCodes pq.StringArray
	var status string
	var lastSyncedAt sql.NullTime
	if err := s.Scan(&out.CustomerID, &out.TenantID, &legalName, &email, &phone, &line1, &line2, &city, &state, &postal, &country, &currency, &taxID, &taxCodes, &providerCode, &status, &lastSyncedAt, &lastSyncError, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.CustomerID = strings.TrimSpace(out.CustomerID)
	out.TaxCodes = normalizeStringList([]string(taxCodes))
	return finalizeCustomerBillingProfile(out, legalName, email, phone, line1, line2, city, state, postal, country, currency, taxID, providerCode, status, lastSyncedAt, lastSyncError), nil
}

func finalizeCustomerBillingProfile(out domain.CustomerBillingProfile, legalName, email, phone, line1, line2, city, state, postal, country, currency, taxID, providerCode sql.NullString, status string, lastSyncedAt sql.NullTime, lastSyncError sql.NullString) domain.CustomerBillingProfile {
	if legalName.Valid {
		out.LegalName = normalizeOptionalText(legalName.String)
	}
	if email.Valid {
		out.Email = normalizeOptionalText(email.String)
	}
	if phone.Valid {
		out.Phone = normalizeOptionalText(phone.String)
	}
	if line1.Valid {
		out.AddressLine1 = normalizeOptionalText(line1.String)
	}
	if line2.Valid {
		out.AddressLine2 = normalizeOptionalText(line2.String)
	}
	if city.Valid {
		out.City = normalizeOptionalText(city.String)
	}
	if state.Valid {
		out.State = normalizeOptionalText(state.String)
	}
	if postal.Valid {
		out.PostalCode = normalizeOptionalText(postal.String)
	}
	if country.Valid {
		out.Country = normalizeOptionalText(country.String)
	}
	if currency.Valid {
		out.Currency = normalizeOptionalText(currency.String)
	}
	if taxID.Valid {
		out.TaxIdentifier = normalizeOptionalText(taxID.String)
	}
	if providerCode.Valid {
		out.ProviderCode = normalizeOptionalText(providerCode.String)
	}
	if lastSyncError.Valid {
		out.LastSyncError = normalizeOptionalText(lastSyncError.String)
	}
	if lastSyncedAt.Valid {
		t := lastSyncedAt.Time.UTC()
		out.LastSyncedAt = &t
	}
	out.ProfileStatus = normalizeBillingProfileStatus(domain.BillingProfileStatus(status))
	return out
}


func scanCustomerPaymentSetup(s rowScanner) (domain.CustomerPaymentSetup, error) {
	var out domain.CustomerPaymentSetup
	var status string
	var paymentMethodType, providerCustomerRef, providerPaymentMethodRef, lastVerificationResult, lastVerificationError, lastRequestStatus, lastRequestKind, lastRequestToEmail, lastRequestError sql.NullString
	var lastVerifiedAt, lastRequestSentAt sql.NullTime
	if err := s.Scan(&out.CustomerID, &out.TenantID, &status, &out.DefaultPaymentMethodPresent, &paymentMethodType, &providerCustomerRef, &providerPaymentMethodRef, &lastVerifiedAt, &lastVerificationResult, &lastVerificationError, &lastRequestStatus, &lastRequestKind, &lastRequestToEmail, &lastRequestSentAt, &lastRequestError, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.CustomerID = strings.TrimSpace(out.CustomerID)
	if paymentMethodType.Valid {
		out.PaymentMethodType = normalizeOptionalText(paymentMethodType.String)
	}
	if providerCustomerRef.Valid {
		out.ProviderCustomerReference = normalizeOptionalText(providerCustomerRef.String)
	}
	if providerPaymentMethodRef.Valid {
		out.ProviderPaymentMethodReference = normalizeOptionalText(providerPaymentMethodRef.String)
	}
	if lastVerificationResult.Valid {
		out.LastVerificationResult = normalizeOptionalText(lastVerificationResult.String)
	}
	if lastVerificationError.Valid {
		out.LastVerificationError = normalizeOptionalText(lastVerificationError.String)
	}
	if lastVerifiedAt.Valid {
		t := lastVerifiedAt.Time.UTC()
		out.LastVerifiedAt = &t
	}
	if lastRequestStatus.Valid {
		out.LastRequestStatus = normalizePaymentSetupRequestStatus(domain.PaymentSetupRequestStatus(lastRequestStatus.String))
	} else {
		out.LastRequestStatus = domain.PaymentSetupRequestStatusNotRequested
	}
	if lastRequestKind.Valid {
		out.LastRequestKind = normalizeOptionalText(lastRequestKind.String)
	}
	if lastRequestToEmail.Valid {
		out.LastRequestToEmail = strings.ToLower(normalizeOptionalText(lastRequestToEmail.String))
	}
	if lastRequestSentAt.Valid {
		t := lastRequestSentAt.Time.UTC()
		out.LastRequestSentAt = &t
	}
	if lastRequestError.Valid {
		out.LastRequestError = normalizeOptionalText(lastRequestError.String)
	}
	out.SetupStatus = normalizePaymentSetupStatus(domain.PaymentSetupStatus(status))
	return out, nil
}


func normalizeCustomerStatus(v domain.CustomerStatus) domain.CustomerStatus {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.CustomerStatusSuspended):
		return domain.CustomerStatusSuspended
	case string(domain.CustomerStatusArchived):
		return domain.CustomerStatusArchived
	default:
		return domain.CustomerStatusActive
	}
}

func normalizeBillingProfileStatus(v domain.BillingProfileStatus) domain.BillingProfileStatus {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.BillingProfileStatusIncomplete):
		return domain.BillingProfileStatusIncomplete
	case string(domain.BillingProfileStatusReady):
		return domain.BillingProfileStatusReady
	case string(domain.BillingProfileStatusSyncError):
		return domain.BillingProfileStatusSyncError
	default:
		return domain.BillingProfileStatusMissing
	}
}

func normalizePaymentSetupStatus(v domain.PaymentSetupStatus) domain.PaymentSetupStatus {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.PaymentSetupStatusPending):
		return domain.PaymentSetupStatusPending
	case string(domain.PaymentSetupStatusReady):
		return domain.PaymentSetupStatusReady
	case string(domain.PaymentSetupStatusError):
		return domain.PaymentSetupStatusError
	default:
		return domain.PaymentSetupStatusMissing
	}
}

func normalizePaymentSetupRequestStatus(v domain.PaymentSetupRequestStatus) domain.PaymentSetupRequestStatus {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.PaymentSetupRequestStatusSent):
		return domain.PaymentSetupRequestStatusSent
	case string(domain.PaymentSetupRequestStatusFailed):
		return domain.PaymentSetupRequestStatusFailed
	default:
		return domain.PaymentSetupRequestStatusNotRequested
	}
}

