package store

import (
	"errors"
	"time"

	"usage-billing-control-plane/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrDuplicateKey  = errors.New("duplicate key")
	ErrInvalidState  = errors.New("invalid state")
)

type Filter struct {
	TenantID          string
	From              *time.Time
	To                *time.Time
	CustomerID        string
	MeterID           string
	BilledSource      domain.BilledEntrySource
	BilledReplayJobID string
	SortDesc          bool
	Limit             int
	Offset            int
	CursorID          string
	CursorOccurredAt  *time.Time
}

type RatingRuleListFilter struct {
	TenantID       string
	RuleKey        string
	LifecycleState string
	LatestOnly     bool
}

type CustomerListFilter struct {
	TenantID   string
	Status     string
	ExternalID string
	Limit      int
	Offset     int
}

type BillingProviderConnectionListFilter struct {
	ProviderType  string
	Environment   string
	Status        string
	Scope         string
	OwnerTenantID string
	Limit         int
	Offset        int
}

type APIKeyListFilter struct {
	TenantID      string
	Role          string
	State         string
	NameContains  string
	Limit         int
	Offset        int
	CursorID      string
	CursorCreated *time.Time
	ReferenceTime time.Time
}

type APIKeyListResult struct {
	Items             []domain.APIKey
	Total             int
	Limit             int
	Offset            int
	NextCursorID      string
	NextCursorCreated *time.Time
}

type APIKeyAuditFilter struct {
	TenantID      string
	APIKeyID      string
	ActorAPIKeyID string
	Action        string
	Limit         int
	Offset        int
	CursorID      string
	CursorCreated *time.Time
}

type APIKeyAuditResult struct {
	Items             []domain.APIKeyAuditEvent
	Total             int
	Limit             int
	Offset            int
	NextCursorID      string
	NextCursorCreated *time.Time
}

type TenantAuditFilter struct {
	TenantID      string
	ActorAPIKeyID string
	Action        string
	Limit         int
	Offset        int
}

type TenantAuditResult struct {
	Items  []domain.TenantAuditEvent
	Total  int
	Limit  int
	Offset int
}

type APIKeyAuditExportFilter struct {
	TenantID            string
	Status              string
	RequestedByAPIKeyID string
	Limit               int
	Offset              int
	CursorID            string
	CursorCreated       *time.Time
}

type APIKeyAuditExportResult struct {
	Items             []domain.APIKeyAuditExportJob
	Total             int
	Limit             int
	Offset            int
	NextCursorID      string
	NextCursorCreated *time.Time
}

type ReplayJobListFilter struct {
	TenantID      string
	CustomerID    string
	MeterID       string
	Status        string
	Limit         int
	Offset        int
	CursorID      string
	CursorCreated *time.Time
}

type ReplayJobListResult struct {
	Items             []domain.ReplayJob
	Total             int
	Limit             int
	Offset            int
	NextCursorID      string
	NextCursorCreated *time.Time
}

type InvoicePaymentStatusListFilter struct {
	TenantID       string
	OrganizationID string
	PaymentStatus  string
	InvoiceStatus  string
	PaymentOverdue *bool
	SortBy         string
	SortDesc       bool
	Limit          int
	Offset         int
}

type InvoicePaymentStatusSummaryFilter struct {
	TenantID       string
	OrganizationID string
	StaleBefore    *time.Time
}

type InvoicePaymentStatusSummary struct {
	TotalInvoices          int64
	OverdueCount           int64
	AttentionRequiredCount int64
	StaleAttentionRequired int64
	LatestEventAt          *time.Time
	PaymentStatusCounts    map[string]int64
	InvoiceStatusCounts    map[string]int64
}

type LagoWebhookEventListFilter struct {
	TenantID       string
	OrganizationID string
	InvoiceID      string
	WebhookType    string
	SortBy         string
	SortDesc       bool
	Limit          int
	Offset         int
}

type InvoicePaymentSyncCandidateFilter struct {
	StaleBefore time.Time
	Limit       int
}

type InvoicePaymentSyncCandidate struct {
	TenantID       string
	OrganizationID string
	InvoiceID      string
	PaymentStatus  string
	PaymentOverdue *bool
	LastEventAt    time.Time
	UpdatedAt      time.Time
}

type Repository interface {
	Migrate() error

	GetTenantByLagoOrganizationID(organizationID string) (domain.Tenant, error)
	CreateTenant(input domain.Tenant) (domain.Tenant, error)
	GetTenant(id string) (domain.Tenant, error)
	UpdateTenant(input domain.Tenant) (domain.Tenant, error)
	ListTenants(status string) ([]domain.Tenant, error)
	UpdateTenantStatus(id string, status domain.TenantStatus, updatedAt time.Time) (domain.Tenant, error)
	CreateTenantAuditEvent(input domain.TenantAuditEvent) (domain.TenantAuditEvent, error)
	ListTenantAuditEvents(filter TenantAuditFilter) (TenantAuditResult, error)
	CreateBillingProviderConnection(input domain.BillingProviderConnection) (domain.BillingProviderConnection, error)
	GetBillingProviderConnection(id string) (domain.BillingProviderConnection, error)
	ListBillingProviderConnections(filter BillingProviderConnectionListFilter) ([]domain.BillingProviderConnection, error)
	CountTenantsByBillingProviderConnections(connectionIDs []string) (map[string]int, error)
	UpdateBillingProviderConnection(input domain.BillingProviderConnection) (domain.BillingProviderConnection, error)
	CreateUser(input domain.User) (domain.User, error)
	GetUser(id string) (domain.User, error)
	GetUserByEmail(email string) (domain.User, error)
	UpdateUser(input domain.User) (domain.User, error)
	UpsertUserPasswordCredential(input domain.UserPasswordCredential) (domain.UserPasswordCredential, error)
	GetUserPasswordCredential(userID string) (domain.UserPasswordCredential, error)
	UpsertUserTenantMembership(input domain.UserTenantMembership) (domain.UserTenantMembership, error)
	ListUserTenantMemberships(userID string) ([]domain.UserTenantMembership, error)
	GetUserFederatedIdentity(providerKey, subject string) (domain.UserFederatedIdentity, error)
	UpsertUserFederatedIdentity(input domain.UserFederatedIdentity) (domain.UserFederatedIdentity, error)
	CreateCustomer(input domain.Customer) (domain.Customer, error)
	GetCustomer(tenantID, id string) (domain.Customer, error)
	GetCustomerByExternalID(tenantID, externalID string) (domain.Customer, error)
	ListCustomers(filter CustomerListFilter) ([]domain.Customer, error)
	UpdateCustomer(input domain.Customer) (domain.Customer, error)
	UpsertCustomerBillingProfile(input domain.CustomerBillingProfile) (domain.CustomerBillingProfile, error)
	GetCustomerBillingProfile(tenantID, customerID string) (domain.CustomerBillingProfile, error)
	UpsertCustomerPaymentSetup(input domain.CustomerPaymentSetup) (domain.CustomerPaymentSetup, error)
	GetCustomerPaymentSetup(tenantID, customerID string) (domain.CustomerPaymentSetup, error)

	CreateRatingRuleVersion(input domain.RatingRuleVersion) (domain.RatingRuleVersion, error)
	ListRatingRuleVersions(filter RatingRuleListFilter) ([]domain.RatingRuleVersion, error)
	GetRatingRuleVersion(tenantID, id string) (domain.RatingRuleVersion, error)

	CreateMeter(input domain.Meter) (domain.Meter, error)
	ListMeters(tenantID string) ([]domain.Meter, error)
	GetMeter(tenantID, id string) (domain.Meter, error)
	UpdateMeter(input domain.Meter) (domain.Meter, error)
	CreatePlan(input domain.Plan) (domain.Plan, error)
	ListPlans(tenantID string) ([]domain.Plan, error)
	GetPlan(tenantID, id string) (domain.Plan, error)
	CreateSubscription(input domain.Subscription) (domain.Subscription, error)
	ListSubscriptions(tenantID string) ([]domain.Subscription, error)
	GetSubscription(tenantID, id string) (domain.Subscription, error)
	UpdateSubscription(input domain.Subscription) (domain.Subscription, error)

	CreateUsageEvent(input domain.UsageEvent) (domain.UsageEvent, error)
	GetUsageEventByIdempotencyKey(tenantID, idempotencyKey string) (domain.UsageEvent, error)
	ListUsageEvents(filter Filter) ([]domain.UsageEvent, error)

	CreateBilledEntry(input domain.BilledEntry) (domain.BilledEntry, error)
	GetBilledEntryByIdempotencyKey(tenantID, idempotencyKey string) (domain.BilledEntry, error)
	ListBilledEntries(filter Filter) ([]domain.BilledEntry, error)

	CreateReplayJob(input domain.ReplayJob) (domain.ReplayJob, error)
	GetReplayJob(tenantID, id string) (domain.ReplayJob, error)
	GetReplayJobByIdempotencyKey(tenantID, key string) (domain.ReplayJob, error)
	ListReplayJobs(filter ReplayJobListFilter) (ReplayJobListResult, error)
	RetryReplayJob(tenantID, id string) (domain.ReplayJob, error)
	StartReplayJob(tenantID, id string) (domain.ReplayJob, error)
	ListQueuedReplayJobs(limit int) ([]domain.ReplayJob, error)
	CompleteReplayJob(id string, processedRecords int64, completedAt time.Time) (domain.ReplayJob, error)
	FailReplayJob(id string, errMessage string, completedAt time.Time) (domain.ReplayJob, error)
	IngestLagoWebhookEvent(input domain.LagoWebhookEvent) (domain.LagoWebhookEvent, bool, error)
	ListInvoicePaymentStatusViews(filter InvoicePaymentStatusListFilter) ([]domain.InvoicePaymentStatusView, error)
	GetInvoicePaymentStatusView(tenantID, invoiceID string) (domain.InvoicePaymentStatusView, error)
	GetInvoicePaymentStatusSummary(filter InvoicePaymentStatusSummaryFilter) (InvoicePaymentStatusSummary, error)
	ListLagoWebhookEvents(filter LagoWebhookEventListFilter) ([]domain.LagoWebhookEvent, error)
	ListInvoicePaymentSyncCandidates(filter InvoicePaymentSyncCandidateFilter) ([]InvoicePaymentSyncCandidate, error)

	CreateAPIKey(input domain.APIKey) (domain.APIKey, error)
	GetAPIKeyByID(tenantID, id string) (domain.APIKey, error)
	ListAPIKeys(filter APIKeyListFilter) (APIKeyListResult, error)
	RevokeAPIKey(tenantID, id string, revokedAt time.Time) (domain.APIKey, error)
	CreateAPIKeyAuditEvent(input domain.APIKeyAuditEvent) (domain.APIKeyAuditEvent, error)
	ListAPIKeyAuditEvents(filter APIKeyAuditFilter) (APIKeyAuditResult, error)
	CreateAPIKeyAuditExportJob(input domain.APIKeyAuditExportJob) (domain.APIKeyAuditExportJob, error)
	GetAPIKeyAuditExportJob(tenantID, id string) (domain.APIKeyAuditExportJob, error)
	GetAPIKeyAuditExportJobByIdempotencyKey(tenantID, idempotencyKey string) (domain.APIKeyAuditExportJob, error)
	ListAPIKeyAuditExportJobs(filter APIKeyAuditExportFilter) (APIKeyAuditExportResult, error)
	DequeueAPIKeyAuditExportJob() (domain.APIKeyAuditExportJob, error)
	CompleteAPIKeyAuditExportJob(id, objectKey string, rowCount int64, completedAt, expiresAt time.Time) (domain.APIKeyAuditExportJob, error)
	FailAPIKeyAuditExportJob(id, errMessage string, completedAt time.Time) (domain.APIKeyAuditExportJob, error)
	GetAPIKeyByPrefix(prefix string) (domain.APIKey, error)
	GetActiveAPIKeyByPrefix(prefix string, at time.Time) (domain.APIKey, error)
	TouchAPIKeyLastUsed(id string, usedAt time.Time) error
	CreatePlatformAPIKey(input domain.PlatformAPIKey) (domain.PlatformAPIKey, error)
	GetPlatformAPIKeyByPrefix(prefix string) (domain.PlatformAPIKey, error)
	GetActivePlatformAPIKeyByPrefix(prefix string, at time.Time) (domain.PlatformAPIKey, error)
	TouchPlatformAPIKeyLastUsed(id string, usedAt time.Time) error
	CountActivePlatformAPIKeys(at time.Time) (int, error)
	RevokeActivePlatformAPIKeysByName(name string, revokedAt time.Time) (int, error)
}
