package conf_test

import (
	"context"
	"testing"

	"github.com/kanzifucius/xp-clean/internal/conf"
	"github.com/kanzifucius/xp-clean/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLoadFromConfigMapsMultiTenant(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "team-b-policy",
				Namespace: conf.DefaultPolicyNamespace,
				Labels:    map[string]string{conf.PolicyLabelKey: conf.PolicyLabelValue},
			},
			Data: map[string]string{
				conf.PolicyDataKey: `
name: team-b
selectors:
  globs:
    namespaces: ["team-b-*"]
    gvks: ["*.dev.example.org/*"]
deletion:
  scope: instances-only
`,
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "team-a-policy",
				Namespace: conf.DefaultPolicyNamespace,
				Labels:    map[string]string{conf.PolicyLabelKey: conf.PolicyLabelValue},
			},
			Data: map[string]string{
				conf.PolicyDataKey: `
name: team-a
selectors:
  requireLabels:
    xp-clean.io/eligible: "true"
  globs:
    namespaces: ["team-a-*"]
deletion:
  scope: instances-only
`,
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unrelated",
				Namespace: conf.DefaultPolicyNamespace,
			},
			Data: map[string]string{"other": "x"},
		},
	)

	got, err := conf.LoadFromConfigMaps(context.Background(), conf.NewClientsetLister(client), conf.DefaultPolicyNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 policies, got %d", len(got))
	}
	// Sorted by ConfigMap name: team-a-policy then team-b-policy.
	if got[0].Name != "team-a" || got[1].Name != "team-b" {
		t.Fatalf("names: %q %q", got[0].Name, got[1].Name)
	}
	if got[0].Deletion.Scope != cleanup.ScopeInstancesOnly {
		t.Fatalf("scope %q", got[0].Deletion.Scope)
	}
}

func TestLoadFromConfigMapsEmpty(t *testing.T) {
	client := fake.NewSimpleClientset()
	_, err := conf.LoadFromConfigMaps(context.Background(), conf.NewClientsetLister(client), "xp-clean")
	if err == nil {
		t.Fatal("expected error when no labelled ConfigMaps exist")
	}
}

func TestLoadFromConfigMapsMissingKey(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad",
			Namespace: "xp-clean",
			Labels:    map[string]string{conf.PolicyLabelKey: conf.PolicyLabelValue},
		},
		Data: map[string]string{"wrong.yaml": "x"},
	})
	_, err := conf.LoadFromConfigMaps(context.Background(), conf.NewClientsetLister(client), "xp-clean")
	if err == nil {
		t.Fatal("expected missing key error")
	}
}
