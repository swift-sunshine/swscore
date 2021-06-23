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
	AggregateNodeAppenderName = "aggregateNode"
)

// AggregateNodeAppender is responsible for injecting aggregate nodes into the graph to gain
// visibility into traffic aggregations for a user-specfied metric attribute.
type AggregateNodeAppender struct {
	Aggregate          string
	AggregateValue     string
	GraphType          string
	InjectServiceNodes bool
	Namespaces         map[string]graph.NamespaceInfo
	QueryTime          int64 // unix time in seconds
	Service            string
}

// Name implements Appender
func (a AggregateNodeAppender) Name() string {
	return AggregateNodeAppenderName
}

// AppendGraph implements Appender
func (a AggregateNodeAppender) AppendGraph(trafficMap graph.TrafficMap, globalInfo *graph.AppenderGlobalInfo, namespaceInfo *graph.AppenderNamespaceInfo) {
	if len(trafficMap) == 0 {
		return
	}

	if globalInfo.PromClient == nil {
		var err error
		globalInfo.PromClient, err = prometheus.NewClient()
		graph.CheckError(err)
	}

	if a.AggregateValue == "" {
		a.appendGraph(trafficMap, namespaceInfo.Namespace, globalInfo.PromClient)
	} else {
		a.appendNodeGraph(trafficMap, namespaceInfo.Namespace, globalInfo.PromClient)
	}
}

func (a AggregateNodeAppender) appendGraph(trafficMap graph.TrafficMap, namespace string, client *prometheus.Client) {
	log.Tracef("Resolving request aggregates for namespace=[%s], aggregate=[%s]", namespace, a.Aggregate)
	duration := a.Namespaces[namespace].Duration

	// query prometheus for aggregate info in two queries (assume aggregation is typically request classification, so use dest telemetry):
	//   note1: we want to only match the aggregate when it is set and not "unknown".  But in Prometheus a negative test on an unset label
	//      matches everything, so using %s!=unknown means we still have to filter out unset time-series below...
	//   note2: for now we will filter out aggregates with no traffic on the assumption that users probably don't want to
	//      see them and it will just increase the graph density.  To change that behavior remove the "> 0" conditions.
	// 1) query for requests originating from a workload outside the namespace.
	groupBy := fmt.Sprintf("source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,%s", a.Aggregate)
	httpQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace!="%s",destination_service_namespace="%v",%s!="unknown"}[%vs])) by (%s) > 0`,
		"istio_requests_total",
		namespace,
		namespace,
		a.Aggregate,
		int(duration.Seconds()), // range duration for the query
		groupBy)
	/* It's not clear that request classification makes sense for TCP metrics. Because it costs us queries I'm
	   removing the support for now, we can add it back if someone presents a valid use case. (same for gRPC messages)
	tcpQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace!="%s",destination_service_namespace="%v",%s!="unknown"}[%vs])) by (%s) > 0`,
		"istio_tcp_sent_bytes_total",
		namespace,
		namespace,
		a.Aggregate,
		int(duration.Seconds()), // range duration for the query
		groupBy)
	query := fmt.Sprintf(`(%s) OR (%s)`, httpQuery, tcpQuery)
	*/
	query := httpQuery
	vector := promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)
	a.injectAggregates(trafficMap, &vector)

	// 2) query for requests originating from a workload inside of the namespace
	httpQuery = fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace="%s",%s!="unknown"}[%vs])) by (%s) > 0`,
		"istio_requests_total",
		namespace,
		a.Aggregate,
		int(duration.Seconds()), // range duration for the query
		groupBy)
	/* See comment above...
	tcpQuery = fmt.Sprintf(`sum(rate(%s{reporter="destination",source_workload_namespace="%s",%s!="unknown"}[%vs])) by (%s) > 0`,
		"istio_tcp_sent_bytes_total",
		namespace,
		a.Aggregate,
		int(duration.Seconds()), // range duration for the query
		groupBy)
	query = fmt.Sprintf(`(%s) OR (%s)`, httpQuery, tcpQuery)
	*/
	query = httpQuery
	vector = promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)
	a.injectAggregates(trafficMap, &vector)
}

func (a AggregateNodeAppender) appendNodeGraph(trafficMap graph.TrafficMap, namespace string, client *prometheus.Client) {
	log.Tracef("Resolving node request aggregates for namespace=[%s], aggregate=[%s=%s]", namespace, a.Aggregate, a.AggregateValue)
	duration := a.Namespaces[namespace].Duration

	// query prometheus for aggregate info in a single query (assume aggregation is typically request classification, so use dest telemetry):
	//   note1: for now we will filter out aggregates with no traffic on the assumption that users probably don't want to
	//      see them and it will just increase the graph density.  To change that behavior remove the "> 0" conditions.
	serviceFragment := ""
	if a.Service != "" {
		serviceFragment = fmt.Sprintf(`,destination_service_name="%s"`, a.Service)
	}
	groupBy := fmt.Sprintf("source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,%s", a.Aggregate)
	httpQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_service_namespace="%s",%s="%s"%s}[%vs])) by (%s) > 0`,
		"istio_requests_total",
		namespace,
		a.Aggregate,
		a.AggregateValue,
		serviceFragment,
		int(duration.Seconds()), // range duration for the query
		groupBy)
	/* See comment above...
	tcpQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_service_namespace="%s",%s="%s"%s}[%vs])) by (%s) > 0`,
		"istio_tcp_sent_bytes_total",
		namespace,
		a.Aggregate,
		a.AggregateValue,
		serviceFragment,
		int(duration.Seconds()), // range duration for the query
		groupBy)
	query := fmt.Sprintf(`(%s) OR (%s)`, httpQuery, tcpQuery)
	*/
	query := httpQuery
	vector := promQuery(query, time.Unix(a.QueryTime, 0), client.GetContext(), client.API(), a)
	a.injectAggregates(trafficMap, &vector)
}

func (a AggregateNodeAppender) injectAggregates(trafficMap graph.TrafficMap, vector *model.Vector) {
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
		lCode := m["response_code"]                // will be missing for TCP
		lGrpc, grpcOk := m["grpc_response_status"] // will be missing for non-GRPC
		lFlags, flagsOk := m["response_flags"]
		lProtocol, protocolOk := m["request_protocol"]
		lAggregate, aggregateOk := m[model.LabelName(a.Aggregate)] // may be unset, see note above

		if !aggregateOk {
			continue
		}

		if !sourceWlNsOk || !sourceWlOk || !sourceAppOk || !sourceVerOk || !destSvcNsOk || !destSvcOk || !destSvcNameOk || !destWlNsOk || !destWlOk || !destAppOk || !destVerOk || !flagsOk {
			log.Warningf("Skipping %v, missing expected labels", m.String())
			continue
		}

		sourceWlNs := string(lSourceWlNs)
		sourceWl := string(lSourceWl)
		sourceApp := string(lSourceApp)
		sourceVer := string(lSourceVer)
		destSvc := string(lDestSvc)
		code := string(lCode)
		protocol := string(lProtocol)
		flags := string(lFlags)
		aggregate := string(lAggregate)

		// handle clusters
		sourceCluster, destCluster := util.HandleClusters(lSourceCluster, sourceClusterOk, lDestCluster, destClusterOk)

		if util.IsBadSourceTelemetry(sourceCluster, sourceClusterOk, sourceWlNs, sourceWl, sourceApp) {
			continue
		}

		if protocolOk {
			// set response code in a backward compatible way
			code = util.HandleResponseCode(protocol, code, grpcOk, string(lGrpc))
		} else {
			// because we are not currently supporting TCP requests, the protocol should be set
			log.Warningf("Skipping %v, missing expected protocol label", m.String())
			continue
			// protocol = "tcp"
		}

		// handle unusual destinations
		destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, _ := util.HandleDestination(sourceCluster, sourceWlNs, sourceWl, destCluster, string(lDestSvcNs), string(lDestSvc), string(lDestSvcName), string(lDestWlNs), string(lDestWl), string(lDestApp), string(lDestVer))

		if util.IsBadDestTelemetry(destCluster, destClusterOk, destSvcNs, destSvc, destSvcName, destWl) {
			continue
		}

		// make code more readable by setting "host" because "destSvc" holds destination.service.host | request.host | "unknown"
		host := destSvc

		val := float64(s.Value)

		// inject aggregate node between source and destination
		sourceID, _ := graph.Id(sourceCluster, sourceWlNs, "", sourceWlNs, sourceWl, sourceApp, sourceVer, a.GraphType)
		sourceNode, sourceFound := trafficMap[sourceID]
		if !sourceFound {
			log.Debugf("Expected source [%s] node not found in traffic map. Skipping aggregate injection [%s]", sourceID, aggregate)
			continue
		}

		// if service nodes are injected show the service-related aggregation:
		//   - use the service node as the dest
		//   - associate aggregate node with the destSvcName and, if set, destApp
		// else show the independent aggregation by using the workload/app node as the dest
		destID := ""
		if a.InjectServiceNodes {
			destID, _ = graph.Id(destCluster, destSvcNs, destSvcName, "", "", "", "", a.GraphType) // service
		} else {
			destID, _ = graph.Id(destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, a.GraphType) // wl/app
		}
		destNode, destFound := trafficMap[destID]
		if !destFound {
			log.Debugf("Expected dest [%s] node not found in traffic map. Skipping aggregate injection [%s]", destID, aggregate)
			continue
		}

		var aggrNode *graph.Node
		if a.InjectServiceNodes {
			aggrNode, _ = addNode(trafficMap, destCluster, destSvcNs, a.Aggregate, aggregate, destSvcName, destApp)
		} else {
			aggrNode, _ = addNode(trafficMap, destCluster, destWlNs, a.Aggregate, aggregate, "", "")
		}

		// replace the non-classified edge (from source to dest) with the classified edges
		// - note that if not every request has a classification match the traffic may be lower than actual, I
		//   think this this OK, and if the user cares they should define a "catch-all" classification match
		safeEdges := []*graph.Edge{}
		for _, e := range sourceNode.Edges {
			if e.Dest.ID != destID {
				safeEdges = append(safeEdges, e)
			}
		}
		sourceNode.Edges = safeEdges

		addTraffic(val, protocol, code, flags, host, sourceNode, aggrNode)
		addTraffic(val, protocol, code, flags, host, aggrNode, destNode)
	}
}

func addTraffic(val float64, protocol, code, flags, host string, source, dest *graph.Node) {
	var edge *graph.Edge
	for _, e := range source.Edges {
		if dest.ID == e.Dest.ID && e.Metadata[graph.ProtocolKey] == protocol {
			edge = e
			break
		}
	}
	if nil == edge {
		edge = source.AddEdge(dest)
		edge.Metadata[graph.ProtocolKey] = protocol
	}

	// Only update traffic on the aggregate node and associated edges.  Remember that this is an appender and the
	// in/out traffic is already set for the non-aggregate nodes.
	var sourceMetadata graph.Metadata
	var destMetadata graph.Metadata
	if source.NodeType == graph.NodeTypeAggregate {
		sourceMetadata = source.Metadata
	} else {
		destMetadata = dest.Metadata
	}
	graph.AddToMetadata(protocol, val, code, flags, host, sourceMetadata, destMetadata, edge.Metadata)
}

func addNode(trafficMap graph.TrafficMap, cluster, namespace, aggregate, aggregateVal, svcName, app string) (*graph.Node, bool) {
	id := graph.AggregateID(cluster, namespace, aggregate, aggregateVal, svcName)
	node, found := trafficMap[id]
	if !found {
		newNode := graph.NewAggregateNodeExplicit(id, cluster, namespace, aggregate, aggregateVal, svcName, app)
		node = &newNode
		trafficMap[id] = node
	}
	return node, found
}
