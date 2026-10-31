package match

import "time"

// Selectors describe label and/or glob eligibility constraints.
// Empty sides impose no constraint. Non-empty sides are AND-combined.
type Selectors struct {
	RequireLabels map[string]string `json:"requireLabels,omitempty" yaml:"requireLabels,omitempty"`
	Globs         GlobPatterns      `json:"globs,omitempty" yaml:"globs,omitempty"`
	NamespaceDeny []string          `json:"namespaceDeny,omitempty" yaml:"namespaceDeny,omitempty"`
}

// GlobPatterns match string fields used during discovery and eligibility.
// Patterns use filepath.Match semantics: * does not cross '/' ;
// for GVK strings use "group/version" or "group/*".
type GlobPatterns struct {
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	XRDNames   []string `json:"xrdNames,omitempty" yaml:"xrdNames,omitempty"`
	XRNames    []string `json:"xrNames,omitempty" yaml:"xrNames,omitempty"`
	GVKs       []string `json:"gvks,omitempty" yaml:"gvks,omitempty"`
}

// TTLConfig controls expiry evaluation for XR instances.
type TTLConfig struct {
	RespectResourceTTL       bool          `json:"respectResourceTTL" yaml:"respectResourceTTL"`
	DefaultTTL               time.Duration `json:"defaultTTL,omitempty" yaml:"defaultTTL,omitempty"`
	ApplyDefaultTTLIfMissing bool          `json:"applyDefaultTTLIfMissing" yaml:"applyDefaultTTLIfMissing"`
}

// ObjectMeta is the minimal metadata used for matching.
type ObjectMeta struct {
	Name              string
	Namespace         string
	Labels            map[string]string
	Annotations       map[string]string
	CreationTimestamp time.Time
}

// Label keys recognised by Xp-clean.
const (
	LabelEligible = "xp-clean.io/eligible"
	LabelKeep     = "xp-clean.io/keep"
	LabelOwner    = "xp-clean.io/owner"
	AnnotTTL      = "xp-clean.io/ttl"
	AnnotExpires  = "xp-clean.io/expires-at"
	AnnotPaused   = "crossplane.io/paused"
)
