package appender

import (
	"fmt"
	"math"
	"time"

	"github.com/prometheus/common/model"

	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/graph/telemetry/istio/util"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/prometheus"
)

const (
	// ResponseTimeAppenderName uniquely identifies the appender: responseTime
	ResponseTimeAppenderName = "responseTime"
)

// ResponseTimeAppender is responsible for adding responseTime information to the graph. ResponseTime
// is represented as a percentile value. The default is 95th percentile, which means that
// 95% of requests executed in no more than the resulting milliseconds. ResponeTime values are
// reported in milliseconds.
// Response Times are reported using destination proxy telemetry, when available, which should remove
// network latency fluctuations.
// TODO: Should we report both source and destination when possible (with and without latency)?
// Name: responseTime
type ResponseTimeAppender struct {
	GraphType          string
	InjectServiceNodes bool
	Namespaces         graph.NamespaceInfoMap
	Quantile           float64
	QueryTime          int64 // unix time in seconds
	Rates              graph.RequestedRates
}

// Name implements Appender
func (a ResponseTimeAppender) Name() string {
	return ResponseTimeAppenderName
}

// AppendGraph implements Appender
func (a ResponseTimeAppender) AppendGraph(trafficMap graph.TrafficMap, globalInfo *graph.AppenderGlobalInfo, namespaceInfo *graph.AppenderNamespaceInfo) {
	if len(trafficMap) == 0 {
		return
	}

	// Response times only apply to request traffic (not TCP or gRPC-message traffic)
	if a.Rates.Grpc != graph.RateRequests && a.Rates.Http != graph.RateRequests {
		return
	}

	if globalInfo.PromClient == nil {
		var err error
		globalInfo.PromClient, err = prometheus.NewClient()
		graph.CheckError(err)
	}

	a.appendGraph(trafficMap, namespaceInfo.Namespace, globalInfo.PromClient)
}

func (a ResponseTimeAppender) appendGraph(trafficMap graph.TrafficMap, namespace string, client *prometheus.Client) {
	// create map to quickly look up responseTime
	responseTimeMap := make(map[string]float64)
	duration := a.Namespaces[namespace].Duration

	quantile := a.Quantile
	if a.Quantile == 0.0 {
		log.Tracef("Generating average responseTime; namespace = %v", namespace)

		// query prometheus for the responseTime info in two queries:
		groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol"

		// 1) Incoming: query destination telemetry to capture namespace services' incoming traffic
		// note - the query order is important as both queries may have overlapping results for edges within
		//        the namespace.  This query uses destination proxy and so must come first.
		query := fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_service_namespace="%s"}[%vs])) by (%s) / sum(rate(%s{reporter="destination",destination_service_namespace="%s"}[%vs])) by (%s) > 0`,
			"istio_request_duration_milliseconds_sum",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy,
			"istio_request_duration_milliseconds_count",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		incomingVector := promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)
		a.populateResponseTimeMap(responseTimeMap, &incomingVector)

		// 2) Outgoing: query source telemetry to capture namespace workloads' outgoing traffic
		query = fmt.Sprintf(`sum(rate(%s{reporter="source",source_workload_namespace="%s"}[%vs])) by (%s) / sum(rate(%s{reporter="source",source_workload_namespace="%s"}[%vs])) by (%s) > 0`,
			"istio_request_duration_milliseconds_sum",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy,
			"istio_request_duration_milliseconds_count",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		outgoingVector := promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)
		a.populateResponseTimeMap(responseTimeMap, &outgoingVector)

	} else {
		log.Tracef("Generating responseTime for quantile [%.2f]; namespace = %v", quantile, namespace)

		// query prometheus for the responseTime info in two queries:
		groupBy := "le,source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol"

		// 1) Incoming: query destination telemetry to capture namespace services' incoming traffic
		// note - the query order is important as both queries may have overlapping results for edges within
		//        the namespace.  This query uses destination proxy and so must come first.
		query := fmt.Sprintf(`histogram_quantile(%.2f, sum(rate(%s{reporter="destination",destination_service_namespace="%s"}[%vs])) by (%s)) > 0`,
			quantile,
			"istio_request_duration_milliseconds_bucket",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		incomingVector := promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)
		a.populateResponseTimeMap(responseTimeMap, &incomingVector)

		// 2) Outgoing: query source telemetry to capture namespace workloads' outgoing traffic
		query = fmt.Sprintf(`histogram_quantile(%.2f, sum(rate(%s{reporter="source",source_workload_namespace="%s"}[%vs])) by (%s)) > 0`,
			quantile,
			"istio_request_duration_milliseconds_bucket",
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy)
		outgoingVector := promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)
		a.populateResponseTimeMap(responseTimeMap, &outgoingVector)
	}

	applyResponseTime(trafficMap, responseTimeMap)
}

func applyResponseTime(trafficMap graph.TrafficMap, responseTimeMap map[string]float64) {
	for _, n := range trafficMap {
		for _, e := range n.Edges {
			key := fmt.Sprintf("%s %s %s", e.Source.ID, e.Dest.ID, e.Metadata[graph.ProtocolKey].(string))
			if val, ok := responseTimeMap[key]; ok {
				e.Metadata[graph.ResponseTime] = val
			}
		}
	}
}

func (a ResponseTimeAppender) populateResponseTimeMap(responseTimeMap map[string]float64, vector *model.Vector) {
	skipRequestsGrpc := a.Rates.Grpc != graph.RateRequests
	skipRequestsHttp := a.Rates.Http != graph.RateRequests

	for _, s := range *vector {
		m := s.Metric
		lSourceCluster, sourceClusterOk := m["source_cluster"]
		lSourceWlNs, sourceWlNsOk := m["source_workload_namespace"]
		lSourceWl, sourceWlOk := m["source_workload"]
		lSourceApp, sourceAppOk := m["source_canonical_service"]
		lSourceVer, sourceVerOk := m["source_canonical_revision"]
		lDestCluster, destClusterOk := m["destination_cluster"]
		lDestSvcNs, destSvcNsOk := m["destination_service_namespace"]
		lDestSvc, destSvcOk := m["destination_service"]
		lDestSvcName, destSvcNameOk := m["destination_service_name"]
		lDestWlNs, destWlNsOk := m["destination_workload_namespace"]
		lDestWl, destWlOk := m["destination_workload"]
		lDestApp, destAppOk := m["destination_canonical_service"]
		lDestVer, destVerOk := m["destination_canonical_revision"]
		lProtocol, protocolOk := m["request_protocol"]

		if !sourceWlNsOk || !sourceWlOk || !sourceAppOk || !sourceVerOk || !destSvcNsOk || !destSvcNameOk || !destSvcOk || !destWlNsOk || !destWlOk || !destAppOk || !destVerOk || !protocolOk {
			log.Warningf("populateResponseTimeMap: Skipping %s, missing expected labels", m.String())
			continue
		}

		sourceWlNs := string(lSourceWlNs)
		sourceWl := string(lSourceWl)
		sourceApp := string(lSourceApp)
		sourceVer := string(lSourceVer)
		destSvc := string(lDestSvc)
		protocol := string(lProtocol)

		if (skipRequestsHttp && protocol == graph.HTTP.Name) || (skipRequestsGrpc && protocol == graph.GRPC.Name) {
			continue
		}

		// handle clusters
		sourceCluster, destCluster := util.HandleClusters(lSourceCluster, sourceClusterOk, lDestCluster, destClusterOk)

		if util.IsBadSourceTelemetry(sourceCluster, sourceClusterOk, sourceWlNs, sourceWl, sourceApp) {
			continue
		}

		val := float64(s.Value)

		// handle unusual destinations
		destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, _ := util.HandleDestination(sourceCluster, sourceWlNs, sourceWl, destCluster, string(lDestSvcNs), string(lDestSvc), string(lDestSvcName), string(lDestWlNs), string(lDestWl), string(lDestApp), string(lDestVer))

		if util.IsBadDestTelemetry(destCluster, destClusterOk, destSvcNs, destSvc, destSvcName, destWl) {
			continue
		}

		// Should not happen but if NaN for any reason, Just skip it
		if math.IsNaN(val) {
			continue
		}

		// don't inject a service node if destSvcName is not set or the dest node is already a service node.
		inject := false
		if a.InjectServiceNodes && graph.IsOK(destSvcName) {
			_, destNodeType := graph.Id(destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, a.GraphType)
			inject = (graph.NodeTypeService != destNodeType)
		}

		if inject {
			// Only set response time on the outgoing edge. On the incoming edge, we can't validly aggregate response times of the outgoing edges (kiali-2297)
			a.addResponseTime(responseTimeMap, val, protocol, destCluster, destSvcNs, destSvcName, "", "", "", destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer)
		} else {
			a.addResponseTime(responseTimeMap, val, protocol, sourceCluster, sourceWlNs, "", sourceWl, sourceApp, sourceVer, destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer)
		}
	}
}

func (a ResponseTimeAppender) addResponseTime(responseTimeMap map[string]float64, val float64, protocol, sourceCluster, sourceNs, sourceSvc, sourceWl, sourceApp, sourceVer, destCluster, destSvcNs, destSvc, destWlNs, destWl, destApp, destVer string) {
	sourceID, _ := graph.Id(sourceCluster, sourceNs, sourceSvc, sourceNs, sourceWl, sourceApp, sourceVer, a.GraphType)
	destID, _ := graph.Id(destCluster, destSvcNs, destSvc, destWlNs, destWl, destApp, destVer, a.GraphType)
	key := fmt.Sprintf("%s %s %s", sourceID, destID, protocol)

	// For edges within the namespace we may get a responseTime reported from both the incoming and outgoing
	// traffic queries.  We assume here the first reported value is preferred (i.e. defer to query order)
	if _, found := responseTimeMap[key]; !found {
		responseTimeMap[key] = val
	}
}
