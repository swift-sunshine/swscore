package appender

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"

	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/graph/telemetry/istio/util"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/prometheus"
)

const (
	SecurityPolicyAppenderName = "securityPolicy"
	policyMTLS                 = "mutual_tls"
)

// SecurityPolicyAppender is responsible for adding securityPolicy information to the graph.
// The appender currently reports only mutual_tls security although is written in a generic way.
// Name: securityPolicy
type SecurityPolicyAppender struct {
	GraphType          string
	InjectServiceNodes bool
	Namespaces         map[string]graph.NamespaceInfo
	QueryTime          int64 // unix time in seconds
	Rates              graph.RequestedRates
}

type PolicyRates map[string]float64

// Name implements Appender
func (a SecurityPolicyAppender) Name() string {
	return SecurityPolicyAppenderName
}

// AppendGraph implements Appender
func (a SecurityPolicyAppender) AppendGraph(trafficMap graph.TrafficMap, globalInfo *graph.AppenderGlobalInfo, namespaceInfo *graph.AppenderNamespaceInfo) {
	if len(trafficMap) == 0 {
		return
	}

	if globalInfo.PromClient == nil {
		var err error
		globalInfo.PromClient, err = prometheus.NewClient()
		graph.CheckError(err)
	}

	a.appendGraph(trafficMap, namespaceInfo.Namespace, globalInfo.PromClient)
}

func (a SecurityPolicyAppender) appendGraph(trafficMap graph.TrafficMap, namespace string, client *prometheus.Client) {
	log.Tracef("Resolving security policy for namespace [%v], rates [%+v]", namespace, a.Rates)
	duration := a.Namespaces[namespace].Duration

	// query prometheus for mutual_tls info in two queries (use dest telemetry because it reports the security policy):
	// 1) query for requests originating from a workload outside the namespace.
	groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,source_principal,destination_cluster,destination_service_namespace,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,destination_principal,connection_security_policy"
	var query string
	if a.Rates.Grpc == graph.RateRequests || a.Rates.Http == graph.RateRequests {
		requestsQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace!="%v",destination_service_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_requests_total",
			namespace,
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		query = fmt.Sprintf(`(%s)`, requestsQuery)
	}
	if a.Rates.Grpc == graph.RateSent || a.Rates.Grpc == graph.RateTotal {
		grpcSentQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace!="%v",destination_service_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_request_messages_total",
			namespace,
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		if query == "" {
			query = fmt.Sprintf(`(%s)`, grpcSentQuery)
		} else {
			query = fmt.Sprintf(`%s OR (%s)`, query, grpcSentQuery)
		}
	}
	if a.Rates.Grpc == graph.RateReceived || a.Rates.Grpc == graph.RateTotal {
		grpcReceivedQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace!="%v",destination_service_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_response_messages_total",
			namespace,
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		if query == "" {
			query = fmt.Sprintf(`(%s)`, grpcReceivedQuery)
		} else {
			query = fmt.Sprintf(`%s OR (%s)`, query, grpcReceivedQuery)
		}
	}
	if a.Rates.Tcp == graph.RateSent || a.Rates.Tcp == graph.RateTotal {
		tcpSentQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace!="%v",destination_service_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_tcp_sent_bytes_total",
			namespace,
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		if query == "" {
			query = fmt.Sprintf(`(%s)`, tcpSentQuery)
		} else {
			query = fmt.Sprintf(`%s OR (%s)`, query, tcpSentQuery)
		}
	}
	if a.Rates.Tcp == graph.RateReceived || a.Rates.Tcp == graph.RateTotal {
		tcpReceivedQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace!="%v",destination_service_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_tcp_received_bytes_total",
			namespace,
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		if query == "" {
			query = fmt.Sprintf(`(%s)`, tcpReceivedQuery)
		} else {
			query = fmt.Sprintf(`%s OR (%s)`, query, tcpReceivedQuery)
		}
	}
	outVector := promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)

	// 2) query for requests originating from a workload inside of the namespace
	query = ""
	if a.Rates.Grpc == graph.RateRequests || a.Rates.Http == graph.RateRequests {
		requestsQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_requests_total",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		query = fmt.Sprintf(`(%s)`, requestsQuery)
	}
	if a.Rates.Grpc == graph.RateSent || a.Rates.Grpc == graph.RateTotal {
		grpcSentQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_request_messages_total",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		if query == "" {
			query = fmt.Sprintf(`(%s)`, grpcSentQuery)
		} else {
			query = fmt.Sprintf(`%s OR (%s)`, query, grpcSentQuery)
		}
	}
	if a.Rates.Grpc == graph.RateReceived || a.Rates.Grpc == graph.RateTotal {
		grpcReceivedQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_response_messages_total",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		if query == "" {
			query = fmt.Sprintf(`(%s)`, grpcReceivedQuery)
		} else {
			query = fmt.Sprintf(`%s OR (%s)`, query, grpcReceivedQuery)
		}
	}
	if a.Rates.Tcp == graph.RateSent || a.Rates.Tcp == graph.RateTotal {
		tcpSentQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_tcp_sent_bytes_total",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		if query == "" {
			query = fmt.Sprintf(`(%s)`, tcpSentQuery)
		} else {
			query = fmt.Sprintf(`%s OR (%s)`, query, tcpSentQuery)
		}
	}
	if a.Rates.Tcp == graph.RateReceived || a.Rates.Tcp == graph.RateTotal {
		tcpReceivedQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace="%v"}[%vs])) by (%s) > 0`,
			"istio_tcp_received_bytes_total",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		if query == "" {
			query = fmt.Sprintf(`(%s)`, tcpReceivedQuery)
		} else {
			query = fmt.Sprintf(`%s OR (%s)`, query, tcpReceivedQuery)
		}
	}
	inVector := promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)

	// create map to quickly look up securityPolicy
	securityPolicyMap := make(map[string]PolicyRates)
	principalMap := make(map[string]map[graph.MetadataKey]string)
	a.populateSecurityPolicyMap(securityPolicyMap, principalMap, &outVector)
	a.populateSecurityPolicyMap(securityPolicyMap, principalMap, &inVector)

	applySecurityPolicy(trafficMap, securityPolicyMap, principalMap)
}

func (a SecurityPolicyAppender) populateSecurityPolicyMap(securityPolicyMap map[string]PolicyRates, principalMap map[string]map[graph.MetadataKey]string, vector *model.Vector) {
	for _, s := range *vector {
		m := s.Metric
		lSourceCluster, sourceClusterOk := m["source_cluster"]
		lSourceWlNs, sourceWlNsOk := m["source_workload_namespace"]
		lSourceWl, sourceWlOk := m["source_workload"]
		lSourceApp, sourceAppOk := m["source_canonical_service"]
		lSourceVer, sourceVerOk := m["source_canonical_revision"]
		lSourcePrincipal, sourcePrincipalOk := m["source_principal"]
		lDestCluster, destClusterOk := m["destination_cluster"]
		lDestSvcNs, destSvcNsOk := m["destination_service_namespace"]
		lDestSvcName, destSvcNameOk := m["destination_service_name"]
		lDestWlNs, destWlNsOk := m["destination_workload_namespace"]
		lDestWl, destWlOk := m["destination_workload"]
		lDestApp, destAppOk := m["destination_canonical_service"]
		lDestVer, destVerOk := m["destination_canonical_revision"]
		lDestPrincipal, destPrincipalOk := m["destination_principal"]
		lCsp, cspOk := m["connection_security_policy"]

		if !sourceWlNsOk || !sourceWlOk || !sourceAppOk || !sourceVerOk || !destSvcNsOk || !destSvcNameOk || !destWlNsOk || !destWlOk || !destAppOk || !destVerOk || !sourcePrincipalOk || !destPrincipalOk {
			log.Warningf("populateSecurityPolicyMap: Skipping %s, missing expected labels", m.String())
			continue
		}

		sourceWlNs := string(lSourceWlNs)
		sourceWl := string(lSourceWl)
		sourceApp := string(lSourceApp)
		sourceVer := string(lSourceVer)
		sourcePrincipal := string(lSourcePrincipal)
		destSvcNs := string(lDestSvcNs)
		destSvcName := string(lDestSvcName)
		destWlNs := string(lDestWlNs)
		destWl := string(lDestWl)
		destApp := string(lDestApp)
		destVer := string(lDestVer)
		destPrincipal := string(lDestPrincipal)
		// connection_security_policy is not set on gRPC message metrics
		csp := graph.Unknown
		if cspOk {
			csp = string(lCsp)
		}

		val := float64(s.Value)

		// handle clusters
		sourceCluster, destCluster := util.HandleClusters(lSourceCluster, sourceClusterOk, lDestCluster, destClusterOk)

		// don't inject a service node if destSvcName is not set or the dest node is already a service node.
		inject := false
		if a.InjectServiceNodes && graph.IsOK(destSvcName) {
			_, destNodeType := graph.Id(destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, a.GraphType)
			inject = (graph.NodeTypeService != destNodeType)
		}
		if inject {
			a.addSecurityPolicy(securityPolicyMap, csp, val, sourceCluster, sourceWlNs, "", sourceWl, sourceApp, sourceVer, destCluster, destSvcNs, destSvcName, "", "", "", "")
			a.addSecurityPolicy(securityPolicyMap, csp, val, destCluster, destSvcNs, destSvcName, "", "", "", destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer)
			a.addPrincipal(principalMap, sourceCluster, sourceWlNs, "", sourceWl, sourceApp, sourceVer, sourcePrincipal, destCluster, destSvcNs, destSvcName, "", "", "", "", destPrincipal)
			a.addPrincipal(principalMap, destCluster, destSvcNs, destSvcName, "", "", "", sourcePrincipal, destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, destPrincipal)
		} else {
			a.addSecurityPolicy(securityPolicyMap, csp, val, sourceCluster, sourceWlNs, "", sourceWl, sourceApp, sourceVer, destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer)
			a.addPrincipal(principalMap, sourceCluster, sourceWlNs, "", sourceWl, sourceApp, sourceVer, sourcePrincipal, destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, destPrincipal)
		}
	}
}

func (a SecurityPolicyAppender) addSecurityPolicy(securityPolicyMap map[string]PolicyRates, csp string, val float64, sourceCluster, sourceNs, sourceSvc, sourceWl, sourceApp, sourceVer, destCluster, destSvcNs, destSvc, destWlNs, destWl, destApp, destVer string) {
	sourceId, _ := graph.Id(sourceCluster, sourceNs, sourceSvc, sourceNs, sourceWl, sourceApp, sourceVer, a.GraphType)
	destId, _ := graph.Id(destCluster, destSvcNs, destSvc, destWlNs, destWl, destApp, destVer, a.GraphType)
	key := fmt.Sprintf("%s %s", sourceId, destId)
	var policyRates PolicyRates
	var ok bool
	if policyRates, ok = securityPolicyMap[key]; !ok {
		policyRates = make(PolicyRates)
		securityPolicyMap[key] = policyRates
	}
	policyRates[csp] = val
}

func applySecurityPolicy(trafficMap graph.TrafficMap, securityPolicyMap map[string]PolicyRates, principalMap map[string]map[graph.MetadataKey]string) {
	for _, s := range trafficMap {
		for _, e := range s.Edges {
			key := fmt.Sprintf("%s %s", e.Source.ID, e.Dest.ID)
			if policyRates, ok := securityPolicyMap[key]; ok {
				mtls := 0.0
				other := 0.0
				for policy, rate := range policyRates {
					if policy == policyMTLS {
						mtls = rate
					} else {
						other += rate
					}
				}
				if mtls > 0 {
					e.Metadata[graph.IsMTLS] = mtls / (mtls + other) * 100
				}
			}
			if kPrincipalMap, ok := principalMap[key]; ok {
				e.Metadata[graph.SourcePrincipal] = kPrincipalMap[graph.SourcePrincipal]
				e.Metadata[graph.DestPrincipal] = kPrincipalMap[graph.DestPrincipal]
			}
		}
	}
}

func (a SecurityPolicyAppender) addPrincipal(principalMap map[string]map[graph.MetadataKey]string, sourceCluster, sourceNs, sourceSvc, sourceWl, sourceApp, sourceVer, sourcePrincipal, destCluster, destSvcNs, destSvc, destWlNs, destWl, destApp, destVer, destPrincipal string) {
	sourceID, _ := graph.Id(sourceCluster, sourceNs, sourceSvc, sourceNs, sourceWl, sourceApp, sourceVer, a.GraphType)
	destID, _ := graph.Id(destCluster, destSvcNs, destSvc, destWlNs, destWl, destApp, destVer, a.GraphType)
	key := fmt.Sprintf("%s %s", sourceID, destID)
	var ok bool
	if _, ok = principalMap[key]; !ok {
		kPrincipalMap := make(map[graph.MetadataKey]string)
		kPrincipalMap[graph.SourcePrincipal] = sourcePrincipal
		kPrincipalMap[graph.DestPrincipal] = destPrincipal
		principalMap[key] = kPrincipalMap
	}
}
