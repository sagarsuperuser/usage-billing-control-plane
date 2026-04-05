package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const defaultTenantID = "default"

func normalizeTenantID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return defaultTenantID
	}
	return v
}

// generateTenantID creates a unique tenant ID like "tenant_a1b2c3d4e5f6".
func GenerateTenantID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("tenant_%d", time.Now().UnixNano())
	}
	return "tenant_" + hex.EncodeToString(b)
}
