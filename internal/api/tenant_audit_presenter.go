package api

import (
	"fmt"
	"strings"

	"usage-billing-control-plane/internal/domain"
)

type tenantAuditEventPresentation struct {
	Code     string
	Category string
	Title    string
	Summary  string
}

func presentTenantAuditEvent(event domain.TenantAuditEvent) tenantAuditEventPresentation {
	action := strings.ToLower(strings.TrimSpace(event.Action))
	metadata := event.Metadata

	switch action {
	case "workspace.created", "created":
		return tenantAuditEventPresentation{
			Code:     "workspace.created",
			Category: "Workspace",
			Title:    "Workspace created",
			Summary:  "This workspace was created.",
		}
	case "workspace.status_changed", "status_changed":
		return tenantAuditEventPresentation{
			Code:     "workspace.status_changed",
			Category: "Workspace",
			Title:    "Workspace status changed",
			Summary:  describeBeforeAfterSummary(metadata, "status", "Workspace status was updated."),
		}
	case "workspace.renamed":
		return tenantAuditEventPresentation{
			Code:     "workspace.renamed",
			Category: "Workspace",
			Title:    "Workspace renamed",
			Summary:  describeBeforeAfterSummary(metadata, "name", "Workspace name was updated."),
		}
	case "workspace.billing_connection_changed", "workspace_billing_binding_updated":
		return tenantAuditEventPresentation{
			Code:     "workspace.billing_connection_changed",
			Category: "Billing",
			Title:    "Billing connection changed",
			Summary:  describeBeforeAfterSummary(metadata, "billing_provider_connection_id", "Active billing connection changed for this workspace."),
		}
	case "workspace.billing_configuration_updated":
		return tenantAuditEventPresentation{
			Code:     "workspace.billing_configuration_updated",
			Category: "Billing",
			Title:    "Billing configuration updated",
			Summary:  "Workspace billing configuration was updated.",
		}
	case "workspace.updated", "updated":
		if metadataHasAny(metadata, "previous_billing_provider_connection_id", "new_billing_provider_connection_id") {
			return tenantAuditEventPresentation{
				Code:     "workspace.billing_connection_changed",
				Category: "Billing",
				Title:    "Billing connection changed",
				Summary:  describeBeforeAfterSummary(metadata, "billing_provider_connection_id", "Active billing connection changed for this workspace."),
			}
		}
		if metadataHasAny(metadata, "previous_name", "new_name") {
			return tenantAuditEventPresentation{
				Code:     "workspace.renamed",
				Category: "Workspace",
				Title:    "Workspace renamed",
				Summary:  describeBeforeAfterSummary(metadata, "name", "Workspace name was updated."),
			}
		}
		if metadataHasAny(metadata, "previous_status", "new_status") {
			return tenantAuditEventPresentation{
				Code:     "workspace.status_changed",
				Category: "Workspace",
				Title:    "Workspace status changed",
				Summary:  describeBeforeAfterSummary(metadata, "status", "Workspace status was updated."),
			}
		}
		return tenantAuditEventPresentation{
			Code:     "workspace.updated",
			Category: "Workspace",
			Title:    "Workspace updated",
			Summary:  "Workspace settings were updated.",
		}
	case "customer.payment_setup_requested", "payment_setup_requested":
		return tenantAuditEventPresentation{
			Code:     "customer.payment_setup_requested",
			Category: "Billing",
			Title:    "Payment setup requested",
			Summary:  "A payment setup request was sent to the customer.",
		}
	case "customer.payment_setup_resent", "payment_setup_resent":
		return tenantAuditEventPresentation{
			Code:     "customer.payment_setup_resent",
			Category: "Billing",
			Title:    "Payment setup resent",
			Summary:  "A payment setup request was resent to the customer.",
		}
	case "workspace.member_role_changed", "workspace_member_role_changed":
		return tenantAuditEventPresentation{
			Code:     "workspace.member_role_changed",
			Category: "Access",
			Title:    "Member role changed",
			Summary:  "A workspace member role was updated.",
		}
	case "workspace.member_disabled", "workspace_member_disabled":
		return tenantAuditEventPresentation{
			Code:     "workspace.member_disabled",
			Category: "Access",
			Title:    "Member disabled",
			Summary:  "A workspace member was disabled.",
		}
	case "workspace.member_reactivated", "workspace_member_reactivated":
		return tenantAuditEventPresentation{
			Code:     "workspace.member_reactivated",
			Category: "Access",
			Title:    "Member reactivated",
			Summary:  "A workspace member was reactivated.",
		}
	case "workspace.invitation_revoked", "workspace_invitation_revoked":
		return tenantAuditEventPresentation{
			Code:     "workspace.invitation_revoked",
			Category: "Access",
			Title:    "Invitation revoked",
			Summary:  "A workspace invitation was revoked.",
		}
	default:
		return tenantAuditEventPresentation{
			Code:     action,
			Category: "Activity",
			Title:    humanizeAuditCode(action),
			Summary:  "Activity was recorded for this workspace.",
		}
	}
}

func metadataHasAny(metadata map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := metadata[key]; ok && strings.TrimSpace(stringifyAuditValue(value)) != "" {
			return true
		}
	}
	return false
}

func describeBeforeAfterSummary(metadata map[string]any, field, fallback string) string {
	previous := strings.TrimSpace(stringifyAuditValue(metadata["previous_"+field]))
	next := strings.TrimSpace(stringifyAuditValue(metadata["new_"+field]))
	switch {
	case previous != "" && next != "":
		return humanizeAuditField(field) + " changed from " + previous + " to " + next + "."
	case next != "":
		return humanizeAuditField(field) + " set to " + next + "."
	default:
		return fallback
	}
}

func humanizeAuditCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "Activity"
	}
	code = strings.ReplaceAll(code, ".", " ")
	code = strings.ReplaceAll(code, "_", " ")
	parts := strings.Fields(code)
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func humanizeAuditField(field string) string {
	switch strings.TrimSpace(field) {
	case "billing_provider_connection_id":
		return "Billing connection"
	case "name":
		return "Workspace name"
	case "status":
		return "Workspace status"
	default:
		return humanizeAuditCode(field)
	}
}

func stringifyAuditValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
