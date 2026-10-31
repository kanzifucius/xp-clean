package conf

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultPolicyNamespace is the dedicated install namespace for policy ConfigMaps.
	DefaultPolicyNamespace = "xp-clean"

	// PolicyLabelKey marks ConfigMaps that carry CleanupPolicy documents.
	PolicyLabelKey = "xp-clean.io/policy"
	// PolicyLabelValue is the expected value for PolicyLabelKey.
	PolicyLabelValue = "true"

	// PolicyDataKey is the ConfigMap data key that holds policy YAML.
	PolicyDataKey = "policy.yaml"
)

// ConfigMapLister lists ConfigMaps (real client or fake for tests).
type ConfigMapLister interface {
	List(ctx context.Context, namespace string, opts metav1.ListOptions) (*corev1.ConfigMapList, error)
}

type clientsetLister struct {
	client kubernetes.Interface
}

func (c clientsetLister) List(ctx context.Context, namespace string, opts metav1.ListOptions) (*corev1.ConfigMapList, error) {
	return c.client.CoreV1().ConfigMaps(namespace).List(ctx, opts)
}

// NewClientsetLister wraps a kubernetes for ConfigMap listing.
func NewClientsetLister(client kubernetes.Interface) ConfigMapLister {
	return clientsetLister{client: client}
}

// LoadFromConfigMaps loads every labelled policy ConfigMap in namespace.
// Multiple ConfigMaps are supported for per-team multi-tenancy.
// Results are sorted by ConfigMap name for stable ordering.
func LoadFromConfigMaps(ctx context.Context, lister ConfigMapLister, namespace string) ([]Resolved, error) {
	if lister == nil {
		return nil, fmt.Errorf("configmap lister is nil")
	}
	if namespace == "" {
		namespace = DefaultPolicyNamespace
	}

	list, err := lister.List(ctx, namespace, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", PolicyLabelKey, PolicyLabelValue),
	})
	if err != nil {
		return nil, fmt.Errorf("list policy ConfigMaps in %s: %w", namespace, err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("no policy ConfigMaps found in namespace %q with label %s=%s (pass --config for a file override)",
			namespace, PolicyLabelKey, PolicyLabelValue)
	}

	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].Name < list.Items[j].Name
	})

	out := make([]Resolved, 0, len(list.Items))
	for i := range list.Items {
		cm := &list.Items[i]
		raw, ok := cm.Data[PolicyDataKey]
		if !ok || raw == "" {
			return nil, fmt.Errorf("ConfigMap %s/%s missing data key %q", cm.Namespace, cm.Name, PolicyDataKey)
		}
		resolved, err := Parse([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf("ConfigMap %s/%s: %w", cm.Namespace, cm.Name, err)
		}
		if resolved.Name == "" || resolved.Name == "default" {
			resolved.Name = cm.Name
		}
		out = append(out, resolved)
	}
	return out, nil
}
