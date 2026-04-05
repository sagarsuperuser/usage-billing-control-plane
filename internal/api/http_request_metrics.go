package api

import (
	"fmt"
	"strings"
	"sync"
)

type requestMetricsCollector struct {
	mu                   sync.Mutex
	counts               map[string]int64
	tenantCounts         map[string]int64
	authDeniedCounts     map[string]map[string]int64
	rateLimitedCounts    map[string]map[string]int64
	rateLimitErrorCounts map[string]map[string]int64
}

func newRequestMetricsCollector() *requestMetricsCollector {
	return &requestMetricsCollector{
		counts:               make(map[string]int64),
		tenantCounts:         make(map[string]int64),
		authDeniedCounts:     make(map[string]map[string]int64),
		rateLimitedCounts:    make(map[string]map[string]int64),
		rateLimitErrorCounts: make(map[string]map[string]int64),
	}
}

func (c *requestMetricsCollector) Inc(method, route string, statusCode int) {
	if c == nil {
		return
	}
	key := fmt.Sprintf("%s %s %d", strings.ToUpper(strings.TrimSpace(method)), strings.TrimSpace(route), statusCode)
	c.mu.Lock()
	c.counts[key]++
	c.mu.Unlock()
}

func (c *requestMetricsCollector) Snapshot() map[string]int64 {
	if c == nil {
		return map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.counts))
	for k, v := range c.counts {
		out[k] = v
	}
	return out
}

func (c *requestMetricsCollector) TenantSnapshot() map[string]int64 {
	if c == nil {
		return map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.tenantCounts))
	for k, v := range c.tenantCounts {
		out[k] = v
	}
	return out
}

func (c *requestMetricsCollector) AuthDeniedSnapshot() map[string]map[string]int64 {
	if c == nil {
		return map[string]map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneNestedCounterMap(c.authDeniedCounts)
}

func (c *requestMetricsCollector) RateLimitedSnapshot() map[string]map[string]int64 {
	if c == nil {
		return map[string]map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneNestedCounterMap(c.rateLimitedCounts)
}

func (c *requestMetricsCollector) RateLimitErrorSnapshot() map[string]map[string]int64 {
	if c == nil {
		return map[string]map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneNestedCounterMap(c.rateLimitErrorCounts)
}

func (c *requestMetricsCollector) IncTenant(tenantID string) {
	if c == nil {
		return
	}
	tenantID = normalizeTenantID(strings.TrimSpace(tenantID))
	c.mu.Lock()
	c.tenantCounts[tenantID]++
	c.mu.Unlock()
}

func (c *requestMetricsCollector) IncAuthDenied(tenantID, reason string) {
	if c == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "unknown"
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "unknown"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.authDeniedCounts[tenantID]; !ok {
		c.authDeniedCounts[tenantID] = make(map[string]int64)
	}
	c.authDeniedCounts[tenantID][reason]++
}

func (c *requestMetricsCollector) IncRateLimited(tenantID, policy string) {
	if c == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "unknown"
	}
	policy = strings.TrimSpace(policy)
	if policy == "" {
		policy = "unknown"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.rateLimitedCounts[tenantID]; !ok {
		c.rateLimitedCounts[tenantID] = make(map[string]int64)
	}
	c.rateLimitedCounts[tenantID][policy]++
}

func (c *requestMetricsCollector) IncRateLimitError(tenantID, policy string) {
	if c == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "unknown"
	}
	policy = strings.TrimSpace(policy)
	if policy == "" {
		policy = "unknown"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.rateLimitErrorCounts[tenantID]; !ok {
		c.rateLimitErrorCounts[tenantID] = make(map[string]int64)
	}
	c.rateLimitErrorCounts[tenantID][policy]++
}

func cloneNestedCounterMap(src map[string]map[string]int64) map[string]map[string]int64 {
	out := make(map[string]map[string]int64, len(src))
	for key, inner := range src {
		innerCopy := make(map[string]int64, len(inner))
		for innerKey, value := range inner {
			innerCopy[innerKey] = value
		}
		out[key] = innerCopy
	}
	return out
}
