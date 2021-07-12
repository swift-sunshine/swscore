package models

import (
	"time"

	osproject_v1 "github.com/openshift/api/project/v1"
	core_v1 "k8s.io/api/core/v1"

	"github.com/kiali/kiali/config/dashboards"
)

// A Namespace provide a scope for names
// This type is used to describe a set of objects.
//
// swagger:model namespace
type Namespace struct {
	// The id of the namespace.
	//
	// example:  istio-system
	// required: true
	Name string `json:"name"`

	// Creation date of the namespace.
	// There is no need to export this through the API. So, this is
	// set to be ignored by JSON package.
	//
	// required: true
	CreationTimestamp time.Time `json:"-"`

	// Labels for Namespace
	Labels map[string]string `json:"labels"`

	// Specific annotations used in Kiali
	Annotations map[string]string `json:"annotations"`
}

type Namespaces []Namespace
type NamespaceNames []string

func CastNamespaceCollection(ns []core_v1.Namespace) []Namespace {
	namespaces := make([]Namespace, len(ns))
	for i, item := range ns {
		namespaces[i] = CastNamespace(item)
	}

	return namespaces
}

func CastNamespace(ns core_v1.Namespace) Namespace {
	namespace := Namespace{}
	namespace.Name = ns.Name
	namespace.CreationTimestamp = ns.CreationTimestamp.Time
	namespace.Labels = ns.Labels
	namespace.Annotations = make(map[string]string)
	// Parse only annotations used by Kiali
	if da, ok := ns.Annotations[dashboards.DashboardTemplateAnnotation]; ok {
		namespace.Annotations[dashboards.DashboardTemplateAnnotation] = da
	}
	return namespace
}

func CastProjectCollection(ps []osproject_v1.Project) []Namespace {
	namespaces := make([]Namespace, len(ps))
	for i, project := range ps {
		namespaces[i] = CastProject(project)
	}

	return namespaces
}

func CastProject(p osproject_v1.Project) Namespace {
	namespace := Namespace{}
	namespace.Name = p.Name
	namespace.CreationTimestamp = p.CreationTimestamp.Time
	namespace.Labels = p.Labels

	return namespace
}

func (nss Namespaces) Includes(namespace string) bool {
	for _, ns := range nss {
		if ns.Name == namespace {
			return true
		}
	}
	return false
}

func (nss Namespaces) GetNames() []string {
	names := make([]string, len(nss))
	for _, ns := range nss {
		names = append(names, ns.Name)
	}
	return names
}

func (nsn NamespaceNames) Includes(namespace string) bool {
	for _, ns := range nsn {
		if ns == namespace {
			return true
		}
	}
	return false
}

func (a NamespaceNames) IsSubsetOf(b NamespaceNames) bool {
	isSubset := true
	for _, n := range b {
		isSubset = isSubset && b.Includes(n)
	}
	return isSubset
}
