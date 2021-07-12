// Package cytoscape provides conversion from our graph to the CystoscapeJS
// configuration json model.
//
// The following links are useful for understanding CytoscapeJS and it's configuration:
//
// Main page:   http://js.cytoscape.org/
// JSON config: http://js.cytoscape.org/#notation/elements-json
// Demos:       http://js.cytoscape.org/#demos
//
// Algorithm: Process the graph structure adding nodes and edges, decorating each
//            with information provided.  An optional second pass generates compound
//            nodes for requested boxing.
//
// The package provides the Cytoscape implementation of graph/ConfigVendor.
package cytoscape

import (
	"crypto/md5"
	"fmt"
	"sort"
	"strings"

	"github.com/kiali/kiali/graph"
)

// ResponseFlags is a map of maps. Each response code is broken down by responseFlags:percentageOfTraffic, e.g.:
// "200" : {
//    "-"     : "80.0",
//    "DC"    : "10.0",
//    "FI,FD" : "10.0"
// }, ...
type ResponseFlags map[string]string

// ResponseHosts is a map of maps. Each response host is broken down by responseFlags:percentageOfTraffic, e.g.:
// "200" : {
//    "www.google.com" : "80.0",
//    "www.yahoo.com"  : "20.0"
// }, ...
type ResponseHosts map[string]string

// ResponseDetail holds information broken down by response code.
type ResponseDetail struct {
	Flags ResponseFlags `json:"flags,omitempty"`
	Hosts ResponseHosts `json:"hosts,omitempty"`
}

// Responses maps responseCodes to detailed information for that code
type Responses map[string]*ResponseDetail

// ProtocolTraffic supplies all of the traffic information for a single protocol
type ProtocolTraffic struct {
	Protocol  string            `json:"protocol,omitempty"`  // protocol
	Rates     map[string]string `json:"rates,omitempty"`     // map[rate]value
	Responses Responses         `json:"responses,omitempty"` // see comment above
}

// GWInfo contains the resolved gateway configuration if the node represents an Istio gateway
type GWInfo struct {
	// IngressInfo contains the resolved gateway configuration if the node represents an Istio ingress gateway
	IngressInfo GWInfoIngress `json:"ingressInfo,omitempty"`
	//EgressInfo struct {
	//
	//}
}

// GWInfoIngress contains the resolved gateway configuration if the node represents an Istio ingress gateway
type GWInfoIngress struct {
	// Hostnames is the list of hosts being served by the associated Istio gateways.
	Hostnames []string `json:"hostnames,omitempty"`
}

// VSInfo contains the resolved VS configuration if the node has a VS attached.
type VSInfo struct {
	// Hostnames is the list of hostnames configured in the associated VSs
	Hostnames []string `json:"hostnames,omitempty"`
}

// HealthConfig maps annotations information for health
type HealthConfig map[string]string

type NodeData struct {
	// Cytoscape Fields
	ID     string `json:"id"`               // unique internal node ID (n0, n1...)
	Parent string `json:"parent,omitempty"` // Compound Node parent ID

	// App Fields (not required by Cytoscape)
	NodeType              string              `json:"nodeType"`
	Cluster               string              `json:"cluster"`
	Namespace             string              `json:"namespace"`
	Workload              string              `json:"workload,omitempty"`
	App                   string              `json:"app,omitempty"`
	Version               string              `json:"version,omitempty"`
	Service               string              `json:"service,omitempty"`               // requested service for NodeTypeService
	Aggregate             string              `json:"aggregate,omitempty"`             // set like "<aggregate>=<aggregateVal>"
	DestServices          []graph.ServiceName `json:"destServices,omitempty"`          // requested services for [dest] node
	Traffic               []ProtocolTraffic   `json:"traffic,omitempty"`               // traffic rates for all detected protocols
	HasCB                 bool                `json:"hasCB,omitempty"`                 // true (has circuit breaker) | false
	HasFaultInjection     bool                `json:"hasFaultInjection,omitempty"`     // true (vs has fault injection) | false
	HasHealthConfig       HealthConfig        `json:"hasHealthConfig,omitempty"`       // set to the health config override
	HasMissingSC          bool                `json:"hasMissingSC,omitempty"`          // true (has missing sidecar) | false
	HasRequestRouting     bool                `json:"hasRequestRouting,omitempty"`     // true (vs has request routing) | false
	HasRequestTimeout     bool                `json:"hasRequestTimeout,omitempty"`     // true (vs has request timeout) | false
	HasTCPTrafficShifting bool                `json:"hasTCPTrafficShifting,omitempty"` // true (vs has tcp traffic shifting) | false
	HasTrafficShifting    bool                `json:"hasTrafficShifting,omitempty"`    // true (vs has traffic shifting) | false
	HasVS                 *VSInfo             `json:"hasVS,omitempty"`                 // it can be empty if there is a VS without hostnames
	IsBox                 string              `json:"isBox,omitempty"`                 // set for NodeTypeBox, current values: [ 'app', 'cluster', 'namespace' ]
	IsDead                bool                `json:"isDead,omitempty"`                // true (has no pods) | false
	IsGateway             *GWInfo             `json:"isGateway,omitempty"`             // Istio ingress/egress gateway information
	IsIdle                bool                `json:"isIdle,omitempty"`                // true | false
	IsInaccessible        bool                `json:"isInaccessible,omitempty"`        // true if the node exists in an inaccessible namespace
	IsOutside             bool                `json:"isOutside,omitempty"`             // true | false
	IsRoot                bool                `json:"isRoot,omitempty"`                // true | false
	IsServiceEntry        *graph.SEInfo       `json:"isServiceEntry,omitempty"`        // set static service entry information
}

type EdgeData struct {
	// Cytoscape Fields
	ID     string `json:"id"`     // unique internal edge ID (e0, e1...)
	Source string `json:"source"` // parent node ID
	Target string `json:"target"` // child node ID

	// App Fields (not required by Cytoscape)
	DestPrincipal   string          `json:"destPrincipal,omitempty"`   // principal used for the edge destination
	IsMTLS          string          `json:"isMTLS,omitempty"`          // set to the percentage of traffic using a mutual TLS connection
	ResponseTime    string          `json:"responseTime,omitempty"`    // in millis
	SourcePrincipal string          `json:"sourcePrincipal,omitempty"` // principal used for the edge source
	Throughput      string          `json:"throughput,omitempty"`      // in bytes/sec (request or response, depends on client request)
	Traffic         ProtocolTraffic `json:"traffic,omitempty"`         // traffic rates for the edge protocol
}

type NodeWrapper struct {
	Data *NodeData `json:"data"`
}

type EdgeWrapper struct {
	Data *EdgeData `json:"data"`
}

type Elements struct {
	Nodes []*NodeWrapper `json:"nodes"`
	Edges []*EdgeWrapper `json:"edges"`
}

type Config struct {
	Timestamp int64    `json:"timestamp"`
	Duration  int64    `json:"duration"`
	GraphType string   `json:"graphType"`
	Elements  Elements `json:"elements"`
}

func nodeHash(id string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(id)))
}

func edgeHash(from, to, protocol string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s.%s.%s", from, to, protocol))))
}

// NewConfig is required by the graph/ConfigVendor interface
func NewConfig(trafficMap graph.TrafficMap, o graph.ConfigOptions) (result Config) {
	nodes := []*NodeWrapper{}
	edges := []*EdgeWrapper{}

	buildConfig(trafficMap, &nodes, &edges, o)

	// Add compound nodes as needed, inner boxes first
	if strings.Contains(o.BoxBy, graph.BoxByApp) || o.GraphType == graph.GraphTypeApp || o.GraphType == graph.GraphTypeVersionedApp {
		boxByApp(&nodes)
	}
	if strings.Contains(o.BoxBy, graph.BoxByNamespace) {
		boxByNamespace(&nodes)
	}
	if strings.Contains(o.BoxBy, graph.BoxByCluster) {
		boxByCluster(&nodes)
	}

	// sort nodes and edges for better json presentation (and predictable testing)
	// kiali-1258 parent nodes must come before the child references
	sort.Slice(nodes, func(i, j int) bool {
		switch {
		case nodes[i].Data.IsBox != nodes[j].Data.IsBox:
			rank := func(boxBy string) int {
				switch boxBy {
				case graph.BoxByCluster:
					return 0
				case graph.BoxByNamespace:
					return 1
				case graph.BoxByApp:
					return 2
				default:
					return 3
				}
			}
			return rank(nodes[i].Data.IsBox) < rank(nodes[j].Data.IsBox)
		case nodes[i].Data.Cluster != nodes[j].Data.Cluster:
			return nodes[i].Data.Cluster < nodes[j].Data.Cluster
		case nodes[i].Data.Namespace != nodes[j].Data.Namespace:
			return nodes[i].Data.Namespace < nodes[j].Data.Namespace
		case nodes[i].Data.App != nodes[j].Data.App:
			return nodes[i].Data.App < nodes[j].Data.App
		case nodes[i].Data.Version != nodes[j].Data.Version:
			return nodes[i].Data.Version < nodes[j].Data.Version
		case nodes[i].Data.Service != nodes[j].Data.Service:
			return nodes[i].Data.Service < nodes[j].Data.Service
		default:
			return nodes[i].Data.Workload < nodes[j].Data.Workload
		}
	})
	sort.Slice(edges, func(i, j int) bool {
		switch {
		case edges[i].Data.Source != edges[j].Data.Source:
			return edges[i].Data.Source < edges[j].Data.Source
		case edges[i].Data.Target != edges[j].Data.Target:
			return edges[i].Data.Target < edges[j].Data.Target
		default:
			// source and target are the same, it must differ on protocol
			return edges[i].Data.Traffic.Protocol < edges[j].Data.Traffic.Protocol
		}
	})

	elements := Elements{nodes, edges}
	result = Config{
		Duration:  int64(o.Duration.Seconds()),
		Timestamp: o.QueryTime,
		GraphType: o.GraphType,
		Elements:  elements,
	}
	return result
}

func buildConfig(trafficMap graph.TrafficMap, nodes *[]*NodeWrapper, edges *[]*EdgeWrapper, o graph.ConfigOptions) {
	for id, n := range trafficMap {
		nodeID := nodeHash(id)

		nd := &NodeData{
			ID:        nodeID,
			NodeType:  n.NodeType,
			Cluster:   n.Cluster,
			Namespace: n.Namespace,
			Workload:  n.Workload,
			App:       n.App,
			Version:   n.Version,
			Service:   n.Service,
		}

		addNodeTelemetry(n, nd)

		// set annotations, if available
		if val, ok := n.Metadata[graph.HasHealthConfig]; ok {
			nd.HasHealthConfig = val.(map[string]string)
		}

		// node may have deployment but no pods running)
		if val, ok := n.Metadata[graph.IsDead]; ok {
			nd.IsDead = val.(bool)
		}

		// node may be idle
		if val, ok := n.Metadata[graph.IsIdle]; ok {
			nd.IsIdle = val.(bool)
		}

		// node may be a root
		if val, ok := n.Metadata[graph.IsRoot]; ok {
			nd.IsRoot = val.(bool)
		}

		// node is not accessible to the current user
		if val, ok := n.Metadata[graph.IsInaccessible]; ok {
			nd.IsInaccessible = val.(bool)
		}

		// node may represent an Istio Ingress Gateway
		if val, ok := n.Metadata[graph.IsIngressGw]; ok && val.(bool) {
			var configuredHostnames []string
			if gwData, gwOk := n.Metadata[graph.Gateways]; gwOk {
				for _, hosts := range gwData.(graph.GatewaysMetadata) {
					configuredHostnames = append(configuredHostnames, hosts...)
				}
			}

			nd.IsGateway = &GWInfo{
				IngressInfo: GWInfoIngress{Hostnames: configuredHostnames},
			}
		}

		// node may have a circuit breaker
		if val, ok := n.Metadata[graph.HasCB]; ok {
			nd.HasCB = val.(bool)
		}

		// node may have a virtual service
		if val, ok := n.Metadata[graph.HasVS]; ok && val.(bool) {
			nd.HasVS = &VSInfo{}

			var configuredHostnames []string
			if vsData, vsOk := n.Metadata[graph.VirtualServices]; vsOk {
				for _, hosts := range vsData.(graph.VirtualServicesMetadata) {
					configuredHostnames = append(configuredHostnames, hosts...)
				}
			}

			nd.HasVS.Hostnames = configuredHostnames
		}

		// set sidecars checks, if available
		if val, ok := n.Metadata[graph.HasMissingSC]; ok {
			nd.HasMissingSC = val.(bool)
		}

		// check if node is on another namespace
		if val, ok := n.Metadata[graph.IsOutside]; ok {
			nd.IsOutside = val.(bool)
		}

		if val, ok := n.Metadata[graph.HasRequestRouting]; ok {
			nd.HasRequestRouting = val.(bool)
		}

		if val, ok := n.Metadata[graph.HasFaultInjection]; ok {
			nd.HasFaultInjection = val.(bool)
		}

		if val, ok := n.Metadata[graph.HasTrafficShifting]; ok {
			nd.HasTrafficShifting = val.(bool)
		}

		if val, ok := n.Metadata[graph.HasTCPTrafficShifting]; ok {
			nd.HasTCPTrafficShifting = val.(bool)
		}

		if val, ok := n.Metadata[graph.HasRequestTimeout]; ok {
			nd.HasRequestTimeout = val.(bool)
		}

		// node may have destination service info
		if val, ok := n.Metadata[graph.DestServices]; ok {
			nd.DestServices = []graph.ServiceName{}
			for _, val := range val.(graph.DestServicesMetadata) {
				nd.DestServices = append(nd.DestServices, val)
			}
		}

		// node may have service entry static info
		if val, ok := n.Metadata[graph.IsServiceEntry]; ok {
			nd.IsServiceEntry = val.(*graph.SEInfo)
		}

		// node may be an aggregate
		if n.NodeType == graph.NodeTypeAggregate {
			nd.Aggregate = fmt.Sprintf("%s=%s", n.Metadata[graph.Aggregate].(string), n.Metadata[graph.AggregateValue].(string))
		}

		nw := NodeWrapper{
			Data: nd,
		}

		*nodes = append(*nodes, &nw)

		for _, e := range n.Edges {
			sourceIDHash := nodeHash(n.ID)
			destIDHash := nodeHash(e.Dest.ID)
			protocol := ""
			if e.Metadata[graph.ProtocolKey] != nil {
				protocol = e.Metadata[graph.ProtocolKey].(string)
			}
			edgeID := edgeHash(sourceIDHash, destIDHash, protocol)
			ed := EdgeData{
				ID:     edgeID,
				Source: sourceIDHash,
				Target: destIDHash,
				Traffic: ProtocolTraffic{
					Protocol: protocol,
				},
			}
			if e.Metadata[graph.DestPrincipal] != nil {
				ed.DestPrincipal = e.Metadata[graph.DestPrincipal].(string)
			}
			if e.Metadata[graph.SourcePrincipal] != nil {
				ed.SourcePrincipal = e.Metadata[graph.SourcePrincipal].(string)
			}
			addEdgeTelemetry(e, &ed)

			ew := EdgeWrapper{
				Data: &ed,
			}
			*edges = append(*edges, &ew)
		}
	}
}

func addNodeTelemetry(n *graph.Node, nd *NodeData) {
	for _, p := range graph.Protocols {
		protocolTraffic := ProtocolTraffic{Protocol: p.Name}
		for _, r := range p.NodeRates {
			if rateVal := getRate(n.Metadata, r.Name); rateVal > 0.0 {
				if protocolTraffic.Rates == nil {
					protocolTraffic.Rates = make(map[string]string)
				}
				protocolTraffic.Rates[string(r.Name)] = rateToString(r.Precision, rateVal)
			}
		}
		if protocolTraffic.Rates != nil {
			if nd.Traffic == nil {
				nd.Traffic = []ProtocolTraffic{}
			}
			nd.Traffic = append(nd.Traffic, protocolTraffic)
		}
	}
}

func addEdgeTelemetry(e *graph.Edge, ed *EdgeData) {
	if val, ok := e.Metadata[graph.IsMTLS]; ok {
		ed.IsMTLS = fmt.Sprintf("%.0f", val.(float64))
	}
	if val, ok := e.Metadata[graph.ResponseTime]; ok {
		responseTime := val.(float64)
		ed.ResponseTime = fmt.Sprintf("%.0f", responseTime)
	}
	if val, ok := e.Metadata[graph.Throughput]; ok {
		throughput := val.(float64)
		ed.Throughput = fmt.Sprintf("%.0f", throughput)
	}

	// an edge represents traffic for at most one protocol
	for _, p := range graph.Protocols {
		protocolTraffic := ProtocolTraffic{Protocol: p.Name}
		total := 0.0
		err := 0.0
		var percentErr, percentReq graph.Rate
		for _, r := range p.EdgeRates {
			rateVal := getRate(e.Metadata, r.Name)
			switch {
			case r.IsTotal:
				// there is one field holding the total traffic
				total = rateVal
			case r.IsErr:
				// error rates can be reported for several error status codes, so sum up all
				// of the error traffic to be used in the percentErr calculation below.
				err += rateVal
			case r.IsPercentErr:
				// hold onto the percentErr field so we know how to report it below
				percentErr = r
			case r.IsPercentReq:
				// hold onto the percentReq field so we know how to report it below
				percentReq = r
			}
			if rateVal > 0.0 {
				if protocolTraffic.Rates == nil {
					protocolTraffic.Rates = make(map[string]string)
				}
				protocolTraffic.Rates[string(r.Name)] = rateToString(r.Precision, rateVal)
			}
		}
		if protocolTraffic.Rates != nil {
			if total > 0 {
				if percentErr.Name != "" {
					rateVal := err / total * 100
					if rateVal > 0.0 {
						protocolTraffic.Rates[string(percentErr.Name)] = fmt.Sprintf("%.*f", percentErr.Precision, rateVal)
					}
				}
				if percentReq.Name != "" {
					rateVal := 0.0
					for _, r := range p.NodeRates {
						if !r.IsOut {
							continue
						}
						rateVal = total / getRate(e.Source.Metadata, r.Name) * 100.0
						break
					}
					if rateVal > 0.0 {
						protocolTraffic.Rates[string(percentReq.Name)] = fmt.Sprintf("%.*f", percentReq.Precision, rateVal)
					}
				}
				mdResponses := e.Metadata[p.EdgeResponses].(graph.Responses)
				for code, detail := range mdResponses {
					responseFlags := make(ResponseFlags)
					responseHosts := make(ResponseHosts)
					for flags, value := range detail.Flags {
						responseFlags[flags] = fmt.Sprintf("%.*f", 1, value/total*100.0)
					}
					for host, value := range detail.Hosts {
						responseHosts[host] = fmt.Sprintf("%.*f", 1, value/total*100.0)
					}
					responseDetail := &ResponseDetail{Flags: responseFlags, Hosts: responseHosts}
					if protocolTraffic.Responses == nil {
						protocolTraffic.Responses = Responses{code: responseDetail}
					} else {
						protocolTraffic.Responses[code] = responseDetail
					}
				}
				ed.Traffic = protocolTraffic
			}
			break
		}
	}
}

func getRate(md graph.Metadata, k graph.MetadataKey) float64 {
	if rate, ok := md[k]; ok {
		return rate.(float64)
	}
	return 0.0
}

// boxByApp adds compound nodes to box nodes for the same app
func boxByApp(nodes *[]*NodeWrapper) {
	box := make(map[string][]*NodeData)

	for _, nw := range *nodes {
		if nw.Data.App != "unknown" && nw.Data.App != "" {
			k := fmt.Sprintf("box_%s_%s_%s", nw.Data.Cluster, nw.Data.Namespace, nw.Data.App)
			box[k] = append(box[k], nw.Data)
		}
	}

	generateBoxCompoundNodes(box, nodes, graph.BoxByApp)
}

// boxByNamespace adds compound nodes to box nodes in the same namespace
func boxByNamespace(nodes *[]*NodeWrapper) {
	box := make(map[string][]*NodeData)

	for _, nw := range *nodes {
		if nw.Data.Parent == "" {
			k := fmt.Sprintf("box_%s_%s", nw.Data.Cluster, nw.Data.Namespace)
			box[k] = append(box[k], nw.Data)
		}
	}

	generateBoxCompoundNodes(box, nodes, graph.BoxByNamespace)
}

// boxByCluster adds compound nodes to box nodes in the same cluster
func boxByCluster(nodes *[]*NodeWrapper) {
	box := make(map[string][]*NodeData)

	for _, nw := range *nodes {
		if nw.Data.Parent == "" {
			k := fmt.Sprintf("box_%s", nw.Data.Cluster)
			box[k] = append(box[k], nw.Data)
		}
	}

	generateBoxCompoundNodes(box, nodes, graph.BoxByCluster)
}

func generateBoxCompoundNodes(box map[string][]*NodeData, nodes *[]*NodeWrapper, boxBy string) {
	for k, members := range box {
		if boxBy != graph.BoxByApp || len(members) > 1 {
			// create the compound (parent) node for the member nodes
			nodeID := nodeHash(k)
			namespace := ""
			app := ""
			switch boxBy {
			case graph.BoxByNamespace:
				namespace = members[0].Namespace
			case graph.BoxByApp:
				namespace = members[0].Namespace
				app = members[0].App
			}
			nd := NodeData{
				ID:        nodeID,
				NodeType:  graph.NodeTypeBox,
				Cluster:   members[0].Cluster,
				Namespace: namespace,
				App:       app,
				Version:   "",
				IsBox:     boxBy,
			}

			nw := NodeWrapper{
				Data: &nd,
			}

			// assign each member node to the compound parent
			nd.HasMissingSC = false // TODO: this is probably unecessarily noisy
			nd.IsInaccessible = false
			nd.IsOutside = false

			for _, n := range members {
				n.Parent = nodeID

				// For logical boxing (app), copy some member attributes to to the box node
				if boxBy == graph.BoxByApp {
					nd.HasMissingSC = nd.HasMissingSC || n.HasMissingSC
					nd.IsInaccessible = nd.IsInaccessible || n.IsInaccessible
					nd.IsOutside = nd.IsOutside || n.IsOutside
				}
			}

			// add the compound node to the list of nodes
			*nodes = append(*nodes, &nw)
		}
	}
}

func rateToString(minPrecision int, rateVal float64) string {
	precision := minPrecision
	if requiredPrecision := calcPrecision(rateVal, 5); requiredPrecision > minPrecision {
		precision = requiredPrecision
	}

	return fmt.Sprintf("%.*f", precision, rateVal)
}

// calcPrecision returns the precision necessary to see at least one significant digit (up to max)
func calcPrecision(val float64, max int) int {
	if val <= 0 {
		return 0
	}

	precision := 0
	for precision < max {
		if val >= 1 {
			break
		}
		val *= 10
		precision++
	}
	return precision
}
