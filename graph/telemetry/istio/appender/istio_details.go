package appender

import (
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kiali/kiali/business"
	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/models"
)

const IstioAppenderName = "istio"

// IstioAppender is responsible for badging nodes with special Istio significance:
// - CircuitBreaker: n.Metadata[HasCB] = true
// - VirtualService: n.Metadata[HasVS] = true
// - Ingress Gateways: Graph nodes representing Istio ingress gateways are decorated with configured hostnames
// Name: istio
type IstioAppender struct{}

// Name implements Appender
func (a IstioAppender) Name() string {
	return IstioAppenderName
}

// AppendGraph implements Appender
func (a IstioAppender) AppendGraph(trafficMap graph.TrafficMap, globalInfo *graph.AppenderGlobalInfo, namespaceInfo *graph.AppenderNamespaceInfo) {
	if len(trafficMap) == 0 {
		return
	}

	sdl := getServiceDefinitionList(namespaceInfo.Namespace, globalInfo)

	addBadging(trafficMap, globalInfo, namespaceInfo)
	addLabels(trafficMap, globalInfo, sdl)
	decorateGateways(trafficMap, globalInfo, namespaceInfo)
}

func addBadging(trafficMap graph.TrafficMap, globalInfo *graph.AppenderGlobalInfo, namespaceInfo *graph.AppenderNamespaceInfo) {
	// Currently no other appenders use DestinationRules or VirtualServices, so they are not cached in AppenderNamespaceInfo
	istioCfg, err := globalInfo.Business.IstioConfig.GetIstioConfigList(business.IstioConfigCriteria{
		IncludeDestinationRules: true,
		IncludeVirtualServices:  true,
		Namespace:               namespaceInfo.Namespace,
	})
	graph.CheckError(err)

	applyCircuitBreakers(trafficMap, namespaceInfo.Namespace, istioCfg)
	applyVirtualServices(trafficMap, namespaceInfo.Namespace, istioCfg)
}

func applyCircuitBreakers(trafficMap graph.TrafficMap, namespace string, istioCfg models.IstioConfigList) {
NODES:
	for _, n := range trafficMap {
		// Skip the check if this node is outside the requested namespace, we limit badging to the requested namespaces
		if n.Namespace != namespace {
			continue
		}

		// Note, Because DestinationRules are applied to services we limit CB badges to service nodes and app nodes.
		// Whether we should add to workload nodes is debatable, we could add it later if needed.
		versionOk := graph.IsOK(n.Version)
		switch {
		case n.NodeType == graph.NodeTypeService:
			for _, destinationRule := range istioCfg.DestinationRules.Items {
				if destinationRule.HasCircuitBreaker(namespace, n.Service, "") {
					n.Metadata[graph.HasCB] = true
					continue NODES
				}
			}
		case !versionOk && (n.NodeType == graph.NodeTypeApp):
			if destServices, ok := n.Metadata[graph.DestServices]; ok {
				for _, ds := range destServices.(graph.DestServicesMetadata) {
					for _, destinationRule := range istioCfg.DestinationRules.Items {
						if destinationRule.HasCircuitBreaker(ds.Namespace, ds.Name, "") {
							n.Metadata[graph.HasCB] = true
							continue NODES
						}
					}
				}
			}
		case versionOk:
			if destServices, ok := n.Metadata[graph.DestServices]; ok {
				for _, ds := range destServices.(graph.DestServicesMetadata) {
					for _, destinationRule := range istioCfg.DestinationRules.Items {
						if destinationRule.HasCircuitBreaker(ds.Namespace, ds.Name, n.Version) {
							n.Metadata[graph.HasCB] = true
							continue NODES
						}
					}
				}
			}
		default:
			continue
		}
	}
}

func applyVirtualServices(trafficMap graph.TrafficMap, namespace string, istioCfg models.IstioConfigList) {
NODES:
	for _, n := range trafficMap {
		if n.NodeType != graph.NodeTypeService {
			continue
		}
		if n.Namespace != namespace {
			continue
		}
		for _, virtualService := range istioCfg.VirtualServices.Items {
			if virtualService.IsValidHost(namespace, n.Service) {
				n.Metadata[graph.HasVS] = true
				if len(virtualService.Spec.Hosts) != 0 {
					var vsMetadata graph.VirtualServicesMetadata
					var vsOk bool
					if vsMetadata, vsOk = n.Metadata[graph.VirtualServices].(graph.VirtualServicesMetadata); !vsOk {
						vsMetadata = make(graph.VirtualServicesMetadata)
						n.Metadata[graph.VirtualServices] = vsMetadata
					}

					vsMetadata[virtualService.Metadata.Name] = virtualService.Spec.Hosts
				}

				// Check if the VS also has one of the Kiali scenarios created by the wizard.
				// TODO: Get this info from the VS spec in case the Istio object was not created by a wizard.
				switch virtualService.Metadata.Labels[graph.WizardLabelKey] {
				case graph.WizardRequestRoutingLabel:
					n.Metadata[graph.HasRequestRouting] = true
				case graph.WizardFaultInjectionLabel:
					n.Metadata[graph.HasFaultInjection] = true
				case graph.WizardTrafficShiftingLabel:
					n.Metadata[graph.HasTrafficShifting] = true
				case graph.WizardTCPTrafficShiftingLabel:
					n.Metadata[graph.HasTCPTrafficShifting] = true
				case graph.WizardRequestTimeoutsLabel:
					n.Metadata[graph.HasRequestTimeout] = true
				}

				continue NODES
			}
		}
	}
}

// addLabels is a chance to add any missing label info to nodes when the telemetry does not provide enough information.
// For example, service injection has this problem.
func addLabels(trafficMap graph.TrafficMap, globalInfo *graph.AppenderGlobalInfo, sdl *models.ServiceDefinitionList) {
	// build map for quick lookup
	svcMap := map[string]*models.Service{}
	for _, sd := range sdl.ServiceDefinitions {
		s := sd.Service
		svcMap[sd.Service.Name] = &s
	}

	appLabelName := config.Get().IstioLabels.AppLabelName
	for _, n := range trafficMap {
		// make sure service nodes have the defined app label so it can be used for app grouping in the UI.
		if n.NodeType == graph.NodeTypeService && n.Namespace == sdl.Namespace.Name && n.App == "" {
			// A service node that is a service entry will not have a service definition
			if _, ok := n.Metadata[graph.IsServiceEntry]; ok {
				continue
			}
			// A service node that is an Istio egress cluster will not have a service definition
			if _, ok := n.Metadata[graph.IsEgressCluster]; ok {
				continue
			}

			if svc, found := svcMap[n.Service]; !found {
				log.Debugf("Service not found, may not apply app label correctly for [%s:%s]", n.Namespace, n.Service)
				continue
			} else if app, ok := svc.Labels[appLabelName]; ok {
				n.App = app
			}
		}
	}
}

func decorateGateways(trafficMap graph.TrafficMap, globalInfo *graph.AppenderGlobalInfo, namespaceInfo *graph.AppenderNamespaceInfo) {
	// Get ingress-gateways deployments in the namespace. Then, find if the graph is showing any of them. If so, flag the GW nodes.
	ingressWorkloads := getIngressGatewaysWorkloads(globalInfo)
	istioAppLabelName := config.Get().IstioLabels.AppLabelName

	ingressNodeMapping := make(map[*models.WorkloadListItem][]*graph.Node)
	if ingressWorkloadsList, ok := ingressWorkloads[namespaceInfo.Namespace]; ok {
		for _, gw := range ingressWorkloadsList {
			for _, node := range trafficMap {
				if node.App == gw.Labels[istioAppLabelName] && node.Namespace == namespaceInfo.Namespace {
					node.Metadata[graph.IsIngressGw] = true
					ingressNodeMapping[&gw] = append(ingressNodeMapping[&gw], node)
				}
			}
		}
	}

	// If there is any ingress gateway node in the processing namespace, find Gateway CRDs and
	// match them against gateways in the graph.
	if len(ingressNodeMapping) != 0 {
		gatewaysCrds := getIstioGatewayResources(globalInfo)

		for _, gwCrd := range gatewaysCrds {
			gwSelector := labels.Set(gwCrd.Spec.Selector).AsSelector()
			for gw, nodes := range ingressNodeMapping {
				if gwSelector.Matches(labels.Set(gw.Labels)) {

					// If we are here, the GatewayCrd selects the Gateway workload.
					// So, all node graphs associated with the GW workload should be listening
					// requests for the hostnames listed in the GatewayCRD.

					// Let's extract the hostnames and add them to the node metadata.
					for _, node := range nodes {
						gwServers := gwCrd.Spec.Servers.([]interface{})
						var hostnames []string

						for _, gwServer := range gwServers {
							gwServerMap := gwServer.(map[string]interface{})
							gwHosts := gwServerMap["hosts"].([]interface{})
							for _, gwHost := range gwHosts {
								hostnames = append(hostnames, gwHost.(string))
							}
						}

						// Metadata format: { gatewayName => array of hostnames }
						node.Metadata[graph.Gateways] = graph.GatewaysMetadata{
							gwCrd.Metadata.Name: hostnames,
						}
					}
				}
			}
		}
	}
}

func getIngressGatewaysWorkloads(globalInfo *graph.AppenderGlobalInfo) map[string][]models.WorkloadListItem {
	nsList, nsErr := globalInfo.Business.Namespace.GetNamespaces()
	graph.CheckError(nsErr)

	ingressWorkloads := make(map[string][]models.WorkloadListItem)
	for _, namespace := range nsList {
		wList, err := globalInfo.Business.Workload.GetWorkloadList(namespace.Name)
		graph.CheckError(err)

		// Find Ingress Gateway deployments
		for _, workload := range wList.Workloads {
			if workload.Type == "Deployment" {
				if labelValue, ok := workload.Labels["operator.istio.io/component"]; ok && labelValue == "IngressGateways" {
					ingressWorkloads[namespace.Name] = append(ingressWorkloads[namespace.Name], workload)
				}
			}
		}
	}

	return ingressWorkloads
}

func getIstioGatewayResources(globalInfo *graph.AppenderGlobalInfo) models.Gateways {
	nsList, nsErr := globalInfo.Business.Namespace.GetNamespaces()
	graph.CheckError(nsErr)

	retVal := models.Gateways{}
	for _, namespace := range nsList {
		istioCfg, err := globalInfo.Business.IstioConfig.GetIstioConfigList(business.IstioConfigCriteria{
			IncludeGateways: true,
			Namespace:       namespace.Name,
		})
		graph.CheckError(err)

		retVal = append(retVal, istioCfg.Gateways...)
	}

	return retVal
}
