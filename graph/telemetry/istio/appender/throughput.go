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
	// ThroughputAppenderName uniquely identifies the appender: throughput
	ThroughputAppenderName = "throughput"
)

// ThroughputAppender is responsible for adding throughput information to the graph. Throughput
// is represented as bytes/sec.  Throughput may be for request bytes or response bytes depending
// on the options.  Request throughput will be reported using source telemetry, response throughput
// using destination telemetry.
// Name: throughput
type ThroughputAppender struct {
	GraphType          string
	InjectServiceNodes bool
	Namespaces         graph.NamespaceInfoMap
	QueryTime          int64 // unix time in seconds
	Rates              graph.RequestedRates
	ThroughputType     string
}

// Name implements Appender
func (a ThroughputAppender) Name() string {
	return ThroughputAppenderName
}

// AppendGraph implements Appender
func (a ThroughputAppender) AppendGraph(trafficMap graph.TrafficMap, globalInfo *graph.AppenderGlobalInfo, namespaceInfo *graph.AppenderNamespaceInfo) {
	if len(trafficMap) == 0 {
		return
	}

	// HTTP Throughput only apply to HTTP request traffic
	if a.Rates.Http != graph.RateRequests {
		return
	}

	if globalInfo.PromClient == nil {
		var err error
		globalInfo.PromClient, err = prometheus.NewClient()
		graph.CheckError(err)
	}

	a.appendGraph(trafficMap, namespaceInfo.Namespace, globalInfo.PromClient)
}

func (a ThroughputAppender) appendGraph(trafficMap graph.TrafficMap, namespace string, client *prometheus.Client) {
	log.Tracef("Generating [%s] throughput; namespace = %v", a.ThroughputType, namespace)

	// create map to quickly look up throughput
	throughputMap := make(map[string]float64)
	duration := a.Namespaces[namespace].Duration

	// query prometheus for throughput info in two queries:
	groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision"
	metric := fmt.Sprintf("istio_%s_bytes_sum", a.ThroughputType)
	reporter := "destination"
	if a.ThroughputType == "request" {
		reporter = "source"
	}

	// query prometheus for throughput rates in two queries:
	// 1) query for requests originating from a workload outside the namespace.
	query := fmt.Sprintf(`sum(rate(%s{reporter="%s",source_workload_namespace!="%s",destination_service_namespace="%s"}[%vs])) by (%s) > 0`,
		metric,
		reporter,
		namespace,
		namespace,
		int(duration.Seconds()), // range duration for the query
		groupBy)
	vector := promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)
	a.populateThroughputMap(throughputMap, &vector)

	// 2) query for requests originating from a workload inside of the namespace
	query = fmt.Sprintf(`sum(rate(%s{reporter="%s",source_workload_namespace="%s"}[%vs])) by (%s) > 0`,
		metric,
		reporter,
		namespace,
		int(duration.Seconds()), // range duration for the query
		groupBy)
	vector = promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)
	a.populateThroughputMap(throughputMap, &vector)

	applyThroughput(trafficMap, throughputMap)
}

func applyThroughput(trafficMap graph.TrafficMap, throughputMap map[string]float64) {
	for _, n := range trafficMap {
		for _, e := range n.Edges {
			key := fmt.Sprintf("%s %s %s", e.Source.ID, e.Dest.ID, e.Metadata[graph.ProtocolKey].(string))
			if val, ok := throughputMap[key]; ok {
				e.Metadata[graph.Throughput] = val
			}
		}
	}
}

func (a ThroughputAppender) populateThroughputMap(throughputMap map[string]float64, vector *model.Vector) {
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

		if !sourceWlNsOk || !sourceWlOk || !sourceAppOk || !sourceVerOk || !destSvcNsOk || !destSvcNameOk || !destSvcOk || !destWlNsOk || !destWlOk || !destAppOk || !destVerOk {
			log.Warningf("populateThroughputMap: Skipping %s, missing expected labels", m.String())
			continue
		}

		sourceWlNs := string(lSourceWlNs)
		sourceWl := string(lSourceWl)
		sourceApp := string(lSourceApp)
		sourceVer := string(lSourceVer)
		destSvc := string(lDestSvc)

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
			// Only set throughput on the outgoing edge. On the incoming edge, we can't validly aggregate thoughputs of the outgoing edges
			// - analogous to https://issues.redhat.com/browse/KIALI-2297, we can't assume even distribution
			a.addThroughput(throughputMap, val, destCluster, destSvcNs, destSvcName, "", "", "", destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer)
		} else {
			a.addThroughput(throughputMap, val, sourceCluster, sourceWlNs, "", sourceWl, sourceApp, sourceVer, destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer)
		}
	}
}

func (a ThroughputAppender) addThroughput(throughputMap map[string]float64, val float64, sourceCluster, sourceNs, sourceSvc, sourceWl, sourceApp, sourceVer, destCluster, destSvcNs, destSvc, destWlNs, destWl, destApp, destVer string) {
	sourceID, _ := graph.Id(sourceCluster, sourceNs, sourceSvc, sourceNs, sourceWl, sourceApp, sourceVer, a.GraphType)
	destID, _ := graph.Id(destCluster, destSvcNs, destSvc, destWlNs, destWl, destApp, destVer, a.GraphType)
	key := fmt.Sprintf("%s %s http", sourceID, destID)

	throughputMap[key] += val
}
