package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	redislib "github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	redisstore "github.com/ulule/limiter/v3/drivers/store/redis"
)

const (
	RateLimitPolicyPreAuthLogin     = "preauth_login"
	RateLimitPolicyPreAuthProtected = "preauth_protected"
	RateLimitPolicyWebhook          = "webhook"
	RateLimitPolicyAuthRead         = "auth_read"
	RateLimitPolicyAuthWrite        = "auth_write"
	RateLimitPolicyAuthAdmin        = "auth_admin"
	RateLimitPolicyAuthInternal     = "auth_internal"
)

type RateLimitRequest struct {
	Policy     string
	Identifier string
}

type RateLimitDecision struct {
	Allowed   bool
	Limit     int64
	Remaining int64
	ResetAt   time.Time
}

type RateLimiter interface {
	Allow(ctx context.Context, req RateLimitRequest) (RateLimitDecision, error)
}

type RedisRateLimiterConfig struct {
	RedisURL    string
	KeyPrefix   string
	PolicyRates map[string]string
}

type RedisRateLimiter struct {
	client   *redislib.Client
	limiters map[string]*limiter.Limiter
}

func DefaultRateLimitPolicyRates() map[string]string {
	return map[string]string{
		RateLimitPolicyPreAuthLogin:     "20-M",
		RateLimitPolicyPreAuthProtected: "600-M",
		RateLimitPolicyWebhook:          "2400-M",
		RateLimitPolicyAuthRead:         "1200-M",
		RateLimitPolicyAuthWrite:        "300-M",
		RateLimitPolicyAuthAdmin:        "120-M",
		RateLimitPolicyAuthInternal:     "120-M",
	}
}

func NewRedisRateLimiter(cfg RedisRateLimiterConfig) (*RedisRateLimiter, error) {
	redisURL := strings.TrimSpace(cfg.RedisURL)
	if redisURL == "" {
		return nil, fmt.Errorf("rate limit redis url is required")
	}

	policyRates := DefaultRateLimitPolicyRates()
	for policy, rate := range cfg.PolicyRates {
		policy = strings.TrimSpace(policy)
		rate = strings.TrimSpace(rate)
		if policy == "" || rate == "" {
			continue
		}
		policyRates[policy] = rate
	}

	for policy, rate := range policyRates {
		if _, err := limiter.NewRateFromFormatted(rate); err != nil {
			return nil, fmt.Errorf("invalid rate for policy=%s value=%q: %w", policy, rate, err)
		}
	}

	redisOpts, err := redislib.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse rate limit redis url: %w", err)
	}
	client := redislib.NewClient(redisOpts)

	keyPrefix := strings.TrimSpace(cfg.KeyPrefix)
	if keyPrefix == "" {
		keyPrefix = "alpha:ratelimit"
	}
	store, err := redisstore.NewStoreWithOptions(client, limiter.StoreOptions{
		Prefix: keyPrefix,
	})
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("initialize redis rate limit store: %w", err)
	}

	out := &RedisRateLimiter{
		client:   client,
		limiters: make(map[string]*limiter.Limiter, len(policyRates)),
	}
	for policy, rateFormatted := range policyRates {
		rate, _ := limiter.NewRateFromFormatted(rateFormatted)
		out.limiters[policy] = limiter.New(store, rate)
	}

	return out, nil
}

func (l *RedisRateLimiter) Allow(ctx context.Context, req RateLimitRequest) (RateLimitDecision, error) {
	if l == nil {
		return RateLimitDecision{}, fmt.Errorf("rate limiter is not configured")
	}
	policy := strings.TrimSpace(req.Policy)
	identifier := strings.TrimSpace(req.Identifier)
	if policy == "" || identifier == "" {
		return RateLimitDecision{}, fmt.Errorf("rate limit policy and identifier are required")
	}

	instance, ok := l.limiters[policy]
	if !ok {
		return RateLimitDecision{}, fmt.Errorf("rate limit policy %q is not configured", policy)
	}

	result, err := instance.Get(ctx, identifier)
	if err != nil {
		return RateLimitDecision{}, err
	}

	return RateLimitDecision{
		Allowed:   !result.Reached,
		Limit:     result.Limit,
		Remaining: result.Remaining,
		ResetAt:   time.Unix(result.Reset, 0).UTC(),
	}, nil
}

func (l *RedisRateLimiter) Close() error {
	if l == nil || l.client == nil {
		return nil
	}
	return l.client.Close()
}
