package service

import "strings"

const defaultTenantID = "default"

func normalizeTenantID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return defaultTenantID
	}
	return v
}
