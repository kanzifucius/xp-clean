package discover_test

import (
	"time"

	"github.com/kanzifucius/xp-clean/pkg/discover"
	"github.com/kanzifucius/xp-clean/pkg/match"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func sampleXRD(name string, labels map[string]string, scope string) *unstructured.Unstructured {
	return sampleXRDCustom(name, labels, scope, "db.dev.example.org", "XDB", "xdbs", "v1alpha1")
}

func sampleXRDCustom(name string, labels map[string]string, scope, group, kind, plural, version string) *unstructured.Unstructured {
	if scope == "" {
		scope = "Namespaced"
	}
	return discover.MustUnstructured("apiextensions.crossplane.io/v2", "CompositeResourceDefinition", "", name, labels,
		map[string]interface{}{
			"group": group,
			"scope": scope,
			"names": map[string]interface{}{"kind": kind, "plural": plural},
			"versions": []interface{}{
				map[string]interface{}{"name": version, "served": true, "storage": true},
			},
		},
	)
}

func sampleXRCustom(apiVersion, kind, ns, name string, labels map[string]string, ann map[string]string, created time.Time) *unstructured.Unstructured {
	xr := discover.MustUnstructured(apiVersion, kind, ns, name, labels,
		map[string]interface{}{
			"crossplane": map[string]interface{}{
				"compositionRef": map[string]interface{}{"name": kind + "-comp"},
			},
		},
	)
	if ann != nil {
		xr.SetAnnotations(ann)
	}
	xr.SetCreationTimestamp(metav1.NewTime(created))
	xr.SetUID(types.UID("uid-" + name))
	return xr
}

func sampleXRDV1(name string) *unstructured.Unstructured {
	return discover.MustUnstructured("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "", name,
		map[string]string{match.LabelEligible: "true"},
		map[string]interface{}{
			"group": "legacy.dev.example.org",
			"scope": "LegacyCluster",
			"names": map[string]interface{}{"kind": "XLegacy", "plural": "xlegacies"},
			"versions": []interface{}{
				map[string]interface{}{"name": "v1alpha1", "served": true, "storage": true},
			},
		},
	)
}

func sampleXR(ns, name string, labels map[string]string, ann map[string]string, created time.Time) *unstructured.Unstructured {
	xr := discover.MustUnstructured("db.dev.example.org/v1alpha1", "XDB", ns, name, labels,
		map[string]interface{}{
			"crossplane": map[string]interface{}{
				"compositionRef": map[string]interface{}{"name": "xdb-comp"},
				"resourceRefs": []interface{}{
					map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"name":       name + "-cm",
						"namespace":  ns,
					},
				},
			},
		},
	)
	if ann != nil {
		xr.SetAnnotations(ann)
	}
	xr.SetCreationTimestamp(metav1.NewTime(created))
	xr.SetUID(types.UID("uid-" + name))
	return xr
}

func eligibleSelectors() match.Selectors {
	return match.Selectors{
		RequireLabels: map[string]string{match.LabelEligible: "true"},
	}
}

func fakeDyn(objs ...runtime.Object) dynamic.Interface {
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "apiextensions.crossplane.io", Version: "v2", Resource: "compositeresourcedefinitions"}: "CompositeResourceDefinitionList",
		{Group: "apiextensions.crossplane.io", Version: "v1", Resource: "compositeresourcedefinitions"}: "CompositeResourceDefinitionList",
		{Group: "apiextensions.crossplane.io", Version: "v1", Resource: "compositions"}:                 "CompositionList",
		{Group: "apiextensions.crossplane.io", Version: "v1", Resource: "compositionrevisions"}:        "CompositionRevisionList",
		{Group: "db.dev.example.org", Version: "v1alpha1", Resource: "xdbs"}:                           "XDBList",
		{Group: "datalab.dev.example.org", Version: "v1alpha1", Resource: "datalabtenants"}:            "DataLabTenantList",
		{Group: "dbx.dev.example.org", Version: "v1alpha1", Resource: "dbxworkspaces"}:                 "DBXWorkspaceList",
		{Group: "dbx.dev.example.org", Version: "v1beta1", Resource: "dbxworkspaces"}:                  "DBXWorkspaceList",
		{Group: "", Version: "v1", Resource: "configmaps"}:                                             "ConfigMapList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
}

func discoverClient(objs ...runtime.Object) *discover.DynamicClient {
	return &discover.DynamicClient{Client: fakeDyn(objs...)}
}
