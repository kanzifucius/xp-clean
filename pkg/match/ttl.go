package match

import (
	"fmt"
	"time"
)

// ExpiryResult describes whether an object has expired for cleanup.
type ExpiryResult struct {
	Expired   bool
	ExpiresAt *time.Time
	Reason    string
}

// EvaluateExpiry checks xp-clean.io/expires-at, then xp-clean.io/ttl, then
// optional default TTL from policy. When RespectResourceTTL is false, expiry
// is treated as already elapsed (gate deferred to other selectors).
func EvaluateExpiry(meta ObjectMeta, cfg TTLConfig, now time.Time) (ExpiryResult, error) {
	if !cfg.RespectResourceTTL {
		return ExpiryResult{Expired: true, Reason: "ttl_not_required"}, nil
	}

	if meta.Annotations != nil {
		if raw, ok := meta.Annotations[AnnotExpires]; ok && raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return ExpiryResult{}, fmt.Errorf("parse %s: %w", AnnotExpires, err)
			}
			return ExpiryResult{
				Expired:   !now.Before(t),
				ExpiresAt: &t,
				Reason:    "expires_at",
			}, nil
		}
		if raw, ok := meta.Annotations[AnnotTTL]; ok && raw != "" {
			d, err := time.ParseDuration(raw)
			if err != nil {
				return ExpiryResult{}, fmt.Errorf("parse %s: %w", AnnotTTL, err)
			}
			exp := meta.CreationTimestamp.Add(d)
			return ExpiryResult{
				Expired:   !now.Before(exp),
				ExpiresAt: &exp,
				Reason:    "ttl",
			}, nil
		}
	}

	if cfg.ApplyDefaultTTLIfMissing && cfg.DefaultTTL > 0 {
		exp := meta.CreationTimestamp.Add(cfg.DefaultTTL)
		return ExpiryResult{
			Expired:   !now.Before(exp),
			ExpiresAt: &exp,
			Reason:    "default_ttl",
		}, nil
	}

	// No TTL annotations and defaults not applied: eligible on selectors alone.
	return ExpiryResult{Expired: true, Reason: "no_ttl"}, nil
}
