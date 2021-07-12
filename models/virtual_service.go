package models

import (
	"github.com/kiali/kiali/kubernetes"
)

// VirtualServices virtualServices
//
// This type is used for returning an array of VirtualServices with some permission flags
//
// swagger:model virtualServices
// An array of virtualService
// swagger:allOf
type VirtualServices struct {
	Permissions ResourcePermissions `json:"permissions"`
	Items       []VirtualService    `json:"items"`
}

// VirtualService virtualService
//
// This type is used for returning a VirtualService
//
// swagger:model virtualService
type VirtualService struct {
	IstioBase
	Spec struct {
		Hosts    []string    `json:"hosts,omitempty"`
		Gateways interface{} `json:"gateways,omitempty"`
		Http     interface{} `json:"http,omitempty"`
		Tcp      interface{} `json:"tcp,omitempty"`
		Tls      interface{} `json:"tls,omitempty"`
		ExportTo interface{} `json:"exportTo,omitempty"`
	} `json:"spec"`
}

func (vServices *VirtualServices) Parse(virtualServices []kubernetes.IstioObject) {
	vServices.Items = []VirtualService{}
	for _, vs := range virtualServices {
		virtualService := VirtualService{}
		virtualService.Parse(vs)
		vServices.Items = append(vServices.Items, virtualService)
	}
}

func (vService *VirtualService) Parse(virtualService kubernetes.IstioObject) {
	vService.IstioBase.Parse(virtualService)
	vService.Spec.Gateways = virtualService.GetSpec()["gateways"]
	vService.Spec.Http = virtualService.GetSpec()["http"]
	vService.Spec.Tls = virtualService.GetSpec()["tls"]
	vService.Spec.Tcp = virtualService.GetSpec()["tcp"]
	vService.Spec.ExportTo = virtualService.GetSpec()["exportTo"]

	if virtualService.GetSpec()["hosts"] != nil {
		hosts := virtualService.GetSpec()["hosts"].([]interface{})
		parsedHosts := make([]string, 0, len(hosts))
		for _, host := range virtualService.GetSpec()["hosts"].([]interface{}) {
			parsedHosts = append(parsedHosts, host.(string))
		}
		vService.Spec.Hosts = parsedHosts
	}
}

// IsValidHost returns true if VirtualService hosts applies to the service
func (vService *VirtualService) IsValidHost(namespace string, serviceName string) bool {
	if serviceName == "" {
		return false
	}

	protocolNames := []string{"http", "tls", "tcp"} // ordered by matching preference
	protocols := map[string]interface{}{
		"http": vService.Spec.Http,
		"tls":  vService.Spec.Tls,
		"tcp":  vService.Spec.Tcp,
	}

	return kubernetes.FilterByRoute(protocols, protocolNames, serviceName, namespace, nil)
}
