package discover

import (
	"context"
	"fmt"

	"github.com/kanzifucius/xp-clean/pkg/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// DynamicClient wraps a Kubernetes dynamic interface for discovery helpers.
type DynamicClient struct {
	Client dynamic.Interface
}

// List lists objects for a GVR, optionally namespaced.
func (d *DynamicClient) List(ctx context.Context, gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if d == nil || d.Client == nil {
		return nil, fmt.Errorf("dynamic client not configured")
	}
	if namespace == "" {
		return d.Client.Resource(gvr).List(ctx, opts)
	}
	return d.Client.Resource(gvr).Namespace(namespace).List(ctx, opts)
}

// Get fetches one object.
func (d *DynamicClient) Get(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	if d == nil || d.Client == nil {
		return nil, fmt.Errorf("dynamic client not configured")
	}
	if namespace == "" {
		return d.Client.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	return d.Client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// Delete deletes one object.
func (d *DynamicClient) Delete(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	if d == nil || d.Client == nil {
		return fmt.Errorf("dynamic client not configured")
	}
	if namespace == "" {
		return d.Client.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
	}
	return d.Client.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// Fetcher adapts DynamicClient to trace.ObjectFetcher using GVK→resource mapping.
type Fetcher struct {
	Dyn      *DynamicClient
	Resource func(group, version, kind string) (schema.GroupVersionResource, error)
}

// Get implements trace.ObjectFetcher.
func (f *Fetcher) Get(ctx context.Context, ref trace.ObjectRef) (*unstructured.Unstructured, error) {
	if f.Resource == nil {
		return nil, fmt.Errorf("resource mapper not configured")
	}
	gvr, err := f.Resource(ref.Group, ref.Version, ref.Kind)
	if err != nil {
		return nil, err
	}
	return f.Dyn.Get(ctx, gvr, ref.Namespace, ref.Name)
}
