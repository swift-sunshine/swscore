// Package istio provides the Istio implementation of graph/TelemetryProvider.
package istio

// Istio.go is responsible for generating TrafficMaps using Istio telemetry.  It implements the
// TelemetryVendor interface.
//
// The algorithm is two-pass:
//   First Pass: Query Prometheus (istio-requests-total metric) to retrieve the source-destination
//               dependencies. Build a traffic map to provide a full representation of nodes and edges.
//
//   Second Pass: Apply any requested appenders to alter or append to the graph.
//
// Supports three vendor-specific query parameters:
//   aggregate: Must be a valid metric attribute (default: request_operation)
//   responseTime: Must be one of: avg | 50 | 95 | 99
//   throughputType: request | response (default: response)
//
import (
	"context"
	"crypto/md5"
	"fmt"
	"regexp"
	"strings"
	"time"

	prom_v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/graph/telemetry"
	"github.com/kiali/kiali/graph/telemetry/istio/appender"
	"github.com/kiali/kiali/graph/telemetry/istio/util"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/prometheus"
	"github.com/kiali/kiali/prometheus/internalmetrics"
)

const (
	tsHash    graph.MetadataKey = "tsHash"
	tsHashMap graph.MetadataKey = "tsHashMap"
)

var (
	grpcMetric = regexp.MustCompile(`istio_.*_messages`)
)

// BuildNamespacesTrafficMap is required by the graph/TelemtryVendor interface
func BuildNamespacesTrafficMap(o graph.TelemetryOptions, client *prometheus.Client, globalInfo *graph.AppenderGlobalInfo) graph.TrafficMap {
	log.Tracef("Build [%s] graph for [%d] namespaces [%v]", o.GraphType, len(o.Namespaces), o.Namespaces)

	appenders := appender.ParseAppenders(o)
	trafficMap := graph.NewTrafficMap()

	for _, namespace := range o.Namespaces {
		log.Tracef("Build traffic map for namespace [%v]", namespace)
		namespaceTrafficMap := buildNamespaceTrafficMap(namespace.Name, o, client)
		namespaceInfo := graph.NewAppenderNamespaceInfo(namespace.Name)
		for _, a := range appenders {
			appenderTimer := internalmetrics.GetGraphAppenderTimePrometheusTimer(a.Name())
			a.AppendGraph(namespaceTrafficMap, globalInfo, namespaceInfo)
			appenderTimer.ObserveDuration()
		}
		telemetry.MergeTrafficMaps(trafficMap, namespace.Name, namespaceTrafficMap)
	}

	// The appenders can add/remove/alter nodes. After the manipulations are complete
	// we can make some final adjustments:
	// - mark the outsiders (i.e. nodes not in the requested namespaces)
	// - mark the insider traffic generators (i.e. inside the namespace and only outgoing edges)
	telemetry.MarkOutsideOrInaccessible(trafficMap, o)
	telemetry.MarkTrafficGenerators(trafficMap)

	if graph.GraphTypeService == o.GraphType {
		trafficMap = telemetry.ReduceToServiceGraph(trafficMap)
	}

	return trafficMap
}

// buildNamespaceTrafficMap returns a map of all namespace nodes (key=id).  All
// nodes either directly send and/or receive requests from a node in the namespace.
func buildNamespaceTrafficMap(namespace string, o graph.TelemetryOptions, client *prometheus.Client) graph.TrafficMap {
	// create map to aggregate traffic by protocol and response code
	trafficMap := graph.NewTrafficMap()
	duration := o.Namespaces[namespace].Duration
	idleCondition := "> 0"
	if o.IncludeIdleEdges {
		idleCondition = ""
	}

	// HTTP/GRPC request traffic
	if o.Rates.Http == graph.RateRequests || o.Rates.Grpc == graph.RateRequests {
		metric := "istio_requests_total"
		groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags"

		// 0) Incoming: query source telemetry to capture unserviced namespace services' incoming traffic
		query := fmt.Sprintf(`sum(rate(%s{reporter="source",source_workload_namespace!="%s",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.%s\\..+$"} [%vs])) by (%s) %s`,
			metric,
			namespace,
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy,
			idleCondition)
		incomingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
		populateTrafficMap(trafficMap, &incomingVector, metric, o)

		// 1) Incoming: query destination telemetry to capture namespace services' incoming traffic
		query = fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_workload_namespace="%s"} [%vs])) by (%s) %s`,
			metric,
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy,
			idleCondition)
		incomingVector = promQuery(query, time.Unix(o.QueryTime, 0), client.API())
		populateTrafficMap(trafficMap, &incomingVector, metric, o)

		// 2) Outgoing: query source telemetry to capture namespace workloads' outgoing traffic
		query = fmt.Sprintf(`sum(rate(%s{reporter="source",source_workload_namespace="%s"} [%vs])) by (%s) %s`,
			metric,
			namespace,
			int(duration.Seconds()), // range duration for the query
			groupBy,
			idleCondition)
		outgoingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
		populateTrafficMap(trafficMap, &outgoingVector, metric, o)
	}

	// GRPC Message traffic
	if o.Rates.Grpc != graph.RateNone && o.Rates.Grpc != graph.RateRequests {
		var metrics []string
		groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision"

		switch o.Rates.Grpc {
		case graph.RateReceived:
			metrics = []string{"istio_response_messages_total"}
		case graph.RateSent:
			metrics = []string{"istio_request_messages_total"}
		case graph.RateTotal:
			metrics = []string{"istio_request_messages_total", "istio_response_messages_total"}
		default:
			metrics = []string{}
		}

		for _, metric := range metrics {
			// 0) Incoming: query source telemetry to capture unserviced namespace services' incoming traffic
			query := fmt.Sprintf(`sum(rate(%s{reporter="source",source_workload_namespace!="%s",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.%s\\..+$"} [%vs])) by (%s) %s`,
				metric,
				namespace,
				namespace,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
			incomingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &incomingVector, metric, o)

			// 1) Incoming: query destination telemetry to capture namespace services' incoming traffic	query = fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_service_namespace="%s"} [%vs])) by (%s) %s`,
			query = fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_workload_namespace="%s"} [%vs])) by (%s) %s`,
				metric,
				namespace,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
			incomingVector = promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &incomingVector, metric, o)

			// 2) Outgoing: query source telemetry to capture namespace workloads' outgoing traffic
			query = fmt.Sprintf(`sum(rate(%s{reporter="source",source_workload_namespace="%s"} [%vs])) by (%s) %s`,
				metric,
				namespace,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
			outgoingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &outgoingVector, metric, o)
		}
	}

	// TCP Byte traffic
	if o.Rates.Tcp != graph.RateNone {
		var metrics []string
		groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags"

		switch o.Rates.Tcp {
		case graph.RateReceived:
			metrics = []string{"istio_tcp_received_bytes_total"}
		case graph.RateSent:
			metrics = []string{"istio_tcp_sent_bytes_total"}
		case graph.RateTotal:
			metrics = []string{"istio_tcp_sent_bytes_total", "istio_tcp_received_bytes_total"}
		default:
			metrics = []string{}
		}

		for _, metric := range metrics {
			// 0) Incoming: query source telemetry to capture unserviced namespace services' incoming traffic
			query := fmt.Sprintf(`sum(rate(%s{reporter="source",source_workload_namespace!="%s",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.%s\\..+$"} [%vs])) by (%s) %s`,
				metric,
				namespace,
				namespace,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
			incomingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &incomingVector, metric, o)

			// 1) Incoming: query destination telemetry to capture namespace services' incoming traffic	query = fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_service_namespace="%s"} [%vs])) by (%s) %s`,
			query = fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_workload_namespace="%s"} [%vs])) by (%s) %s`,
				metric,
				namespace,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
			incomingVector = promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &incomingVector, metric, o)

			// 2) Outgoing: query source telemetry to capture namespace workloads' outgoing traffic
			query = fmt.Sprintf(`sum(rate(%s{reporter="source",source_workload_namespace="%s"} [%vs])) by (%s) %s`,
				metric,
				namespace,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
			outgoingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &outgoingVector, metric, o)
		}
	}

	return trafficMap
}

func populateTrafficMap(trafficMap graph.TrafficMap, vector *model.Vector, metric string, o graph.TelemetryOptions) {
	isRequests := true
	protocol := ""
	switch {
	case grpcMetric.MatchString(metric):
		isRequests = false
		protocol = graph.GRPC.Name
	case strings.HasPrefix(metric, "istio_tcp"):
		isRequests = false
		protocol = graph.TCP.Name
	}
	skipRequestsGrpc := isRequests && o.Rates.Grpc != graph.RateRequests
	skipRequestsHttp := isRequests && o.Rates.Http != graph.RateRequests

	for _, s := range *vector {
		val := float64(s.Value)

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

		if !sourceWlNsOk || !sourceWlOk || !sourceAppOk || !sourceVerOk || !destSvcNsOk || !destSvcOk || !destSvcNameOk || !destWlNsOk || !destWlOk || !destAppOk || !destVerOk {
			log.Warningf("Skipping %s, missing expected TS labels", m.String())
			continue
		}

		sourceWlNs := string(lSourceWlNs)
		sourceWl := string(lSourceWl)
		sourceApp := string(lSourceApp)
		sourceVer := string(lSourceVer)
		destSvc := string(lDestSvc)

		flags := ""
		if isRequests || protocol == graph.TCP.Name {
			lFlags, flagsOk := m["response_flags"]
			if !flagsOk {
				log.Warningf("Skipping %s, missing expected TS labels", m.String())
				continue
			}
			flags = string(lFlags)
		}

		// handle clusters
		sourceCluster, destCluster := util.HandleClusters(lSourceCluster, sourceClusterOk, lDestCluster, destClusterOk)

		if util.IsBadSourceTelemetry(sourceCluster, sourceClusterOk, sourceWlNs, sourceWl, sourceApp) {
			continue
		}

		// handle unusual destinations
		destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, _ := util.HandleDestination(sourceCluster, sourceWlNs, sourceWl, destCluster, string(lDestSvcNs), string(lDestSvc), string(lDestSvcName), string(lDestWlNs), string(lDestWl), string(lDestApp), string(lDestVer))

		if util.IsBadDestTelemetry(destCluster, destClusterOk, destSvcNs, destSvc, destSvcName, destWl) {
			continue
		}

		var code string
		if isRequests {
			lProtocol, protocolOk := m["request_protocol"]
			lCode, codeOk := m["response_code"]
			lGrpc, grpcOk := m["grpc_response_status"]

			if !protocolOk || !codeOk {
				log.Warningf("Skipping %s, missing expected HTTP/GRPC TS labels", m.String())
				continue
			}

			protocol = string(lProtocol)
			if skipRequestsGrpc && protocol == graph.GRPC.Name || skipRequestsHttp && protocol == graph.HTTP.Name {
				continue
			}

			// set response code in a backward compatible way
			code = util.HandleResponseCode(protocol, string(lCode), grpcOk, string(lGrpc))
		}

		// make code more readable by setting "host" because "destSvc" holds destination.service.host | request.host | "unknown"
		host := destSvc

		// don't inject a service node if destSvcName is not set or the dest node is already a service node.
		inject := false
		if o.InjectServiceNodes && graph.IsOK(destSvcName) {
			_, destNodeType := graph.Id(destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, o.GraphType)
			inject = (graph.NodeTypeService != destNodeType)
		}
		addTraffic(trafficMap, metric, inject, val, protocol, code, flags, host, sourceCluster, sourceWlNs, "", sourceWl, sourceApp, sourceVer, destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, o)
	}
}

func addTraffic(trafficMap graph.TrafficMap, metric string, inject bool, val float64, protocol, code, flags, host, sourceCluster, sourceNs, sourceSvc, sourceWl, sourceApp, sourceVer, destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer string, o graph.TelemetryOptions) {
	source, _ := addNode(trafficMap, sourceCluster, sourceNs, sourceSvc, sourceNs, sourceWl, sourceApp, sourceVer, o)
	dest, _ := addNode(trafficMap, destCluster, destSvcNs, destSvcName, destWlNs, destWl, destApp, destVer, o)

	// Istio can generate duplicate metrics by reporting from both the source and destination proxies. To avoid
	// processing the same information twice we keep track of the time series applied to a particular edge. The
	// edgeTSHash incorporates information about the time series' source, destination and metric information,
	// and uses that unique TS has to protect against applying the same intomation twice.
	edgeTSHash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s:%s:%s:%s", metric, source.Metadata[tsHash], dest.Metadata[tsHash], code, flags, host))))

	if inject {
		injectedService, _ := addNode(trafficMap, destCluster, destSvcNs, destSvcName, "", "", "", "", o)
		if addEdgeTraffic(trafficMap, val, protocol, code, flags, host, source, injectedService, edgeTSHash, o) {
			addToDestServices(injectedService.Metadata, destCluster, destSvcNs, destSvcName)

			addEdgeTraffic(trafficMap, val, protocol, code, flags, host, injectedService, dest, edgeTSHash, o)
			addToDestServices(dest.Metadata, destCluster, destSvcNs, destSvcName)
		}
	} else {
		if addEdgeTraffic(trafficMap, val, protocol, code, flags, host, source, dest, edgeTSHash, o) {
			addToDestServices(dest.Metadata, destCluster, destSvcNs, destSvcName)
		}
	}
}

// addEdgeTraffic uses edgeTSHash that the metric information has not been applied to the edge. Returns true
// if the the metric information is applied, false if it determined to be a duplicate.
func addEdgeTraffic(trafficMap graph.TrafficMap, val float64, protocol, code, flags, host string, source, dest *graph.Node, edgeTSHash string, o graph.TelemetryOptions) bool {

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
		edge.Metadata[tsHashMap] = make(map[string]bool)
	}

	if _, ok := edge.Metadata[tsHashMap].(map[string]bool)[edgeTSHash]; !ok {
		edge.Metadata[tsHashMap].(map[string]bool)[edgeTSHash] = true
		graph.AddToMetadata(protocol, val, code, flags, host, source.Metadata, dest.Metadata, edge.Metadata)
		return true
	}

	return false
}

func addToDestServices(md graph.Metadata, cluster, namespace, service string) {
	if !graph.IsOK(service) {
		return
	}
	destServices, ok := md[graph.DestServices]
	if !ok {
		destServices = graph.NewDestServicesMetadata()
		md[graph.DestServices] = destServices
	}
	destService := graph.ServiceName{Cluster: cluster, Namespace: namespace, Name: service}
	destServices.(graph.DestServicesMetadata)[destService.Key()] = destService
}

func addNode(trafficMap graph.TrafficMap, cluster, serviceNs, service, workloadNs, workload, app, version string, o graph.TelemetryOptions) (*graph.Node, bool) {
	id, nodeType := graph.Id(cluster, serviceNs, service, workloadNs, workload, app, version, o.GraphType)
	node, found := trafficMap[id]
	if !found {
		namespace := workloadNs
		if !graph.IsOK(namespace) {
			namespace = serviceNs
		}
		newNode := graph.NewNodeExplicit(id, cluster, namespace, workload, app, version, service, nodeType, o.GraphType)
		node = &newNode
		trafficMap[id] = node
	}
	node.Metadata["tsHash"] = timeSeriesHash(cluster, serviceNs, service, workloadNs, workload, app, version)
	return node, found
}

func timeSeriesHash(cluster, serviceNs, service, workloadNs, workload, app, version string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s", cluster, serviceNs, service, workloadNs, workload, app, version))))
}

// BuildNodeTrafficMap is required by the graph/TelemtryVendor interface
func BuildNodeTrafficMap(o graph.TelemetryOptions, client *prometheus.Client, globalInfo *graph.AppenderGlobalInfo) graph.TrafficMap {
	if o.NodeOptions.Aggregate != "" {
		return handleAggregateNodeTrafficMap(o, client, globalInfo)
	}

	n := graph.NewNode(o.NodeOptions.Cluster, o.NodeOptions.Namespace, o.NodeOptions.Service, o.NodeOptions.Namespace, o.NodeOptions.Workload, o.NodeOptions.App, o.NodeOptions.Version, o.GraphType)

	log.Tracef("Build graph for node [%+v]", n)

	appenders := appender.ParseAppenders(o)
	trafficMap := buildNodeTrafficMap(o.Cluster, o.NodeOptions.Namespace, n, o, client)

	namespaceInfo := graph.NewAppenderNamespaceInfo(o.NodeOptions.Namespace)

	for _, a := range appenders {
		appenderTimer := internalmetrics.GetGraphAppenderTimePrometheusTimer(a.Name())
		a.AppendGraph(trafficMap, globalInfo, namespaceInfo)
		appenderTimer.ObserveDuration()
	}

	// The appenders can add/remove/alter nodes. After the manipulations are complete
	// we can make some final adjustments:
	// - mark the outsiders (i.e. nodes not in the requested namespaces)
	// - mark the traffic generators
	telemetry.MarkOutsideOrInaccessible(trafficMap, o)
	telemetry.MarkTrafficGenerators(trafficMap)

	// Note that this is where we would call reduceToServiceGraph for graphTypeService but
	// the current decision is to not reduce the node graph to provide more detail.  This may be
	// confusing to users, we'll see...

	return trafficMap
}

// buildNodeTrafficMap returns a map of all nodes requesting or requested by the target node (key=id). Node graphs
// are from the perspective of the node, as such we use destination telemetry for incoming traffic and source telemetry
// for outgoing traffic.
func buildNodeTrafficMap(cluster, namespace string, n graph.Node, o graph.TelemetryOptions, client *prometheus.Client) graph.TrafficMap {
	// create map to aggregate traffic by protocol and response code
	trafficMap := graph.NewTrafficMap()
	duration := o.Namespaces[namespace].Duration
	idleCondition := "> 0"
	if o.IncludeIdleEdges {
		idleCondition = ""
	}

	// only narrow by cluster if it is set on the target node
	var sourceCluster, destCluster string
	if cluster != graph.Unknown {
		sourceCluster = fmt.Sprintf(",source_cluster=%s", cluster)
		destCluster = fmt.Sprintf(",destination_cluster=%s", cluster)
	}

	// HTTP/GRPC Traffic
	if o.Rates.Http == graph.RateRequests || o.Rates.Grpc == graph.RateRequests {
		metric := "istio_requests_total"
		groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags"

		// query prometheus for request traffic in two queries:
		// 1) query for incoming traffic
		var query string
		switch n.NodeType {
		case graph.NodeTypeWorkload:
			query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_workload_namespace="%s",destination_workload="%s"} [%vs])) by (%s) %s`,
				metric,
				destCluster,
				namespace,
				n.Workload,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
		case graph.NodeTypeApp:
			if graph.IsOK(n.Version) {
				query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_service_namespace="%s",destination_canonical_service="%s",destination_canonical_revision="%s"} [%vs])) by (%s) %s`,
					metric,
					destCluster,
					namespace,
					n.App,
					n.Version,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			} else {
				query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_service_namespace="%s",destination_canonical_service="%s"} [%vs])) by (%s) %s`,
					metric,
					destCluster,
					namespace,
					n.App,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			}
		case graph.NodeTypeService:
			// Service nodes require two queries for incoming
			// 1.a) query source telemetry for requests to the service that could not be serviced
			query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,destination_workload="unknown",destination_service=~"^%s\\.%s\\..*$"} [%vs])) by (%s) %s`,
				metric,
				destCluster,
				n.Service,
				namespace,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
			vector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &vector, metric, o)

			// 1.b) query dest telemetry for requests to the service, serviced by service workloads
			query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_service_namespace="%s",destination_service=~"^%s\\.%s\\..*$"} [%vs])) by (%s) %s`,
				metric,
				destCluster,
				namespace,
				n.Service,
				namespace,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
		default:
			graph.Error(fmt.Sprintf("NodeType [%s] not supported", n.NodeType))
		}
		inVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
		populateTrafficMap(trafficMap, &inVector, metric, o)

		// 2) query for outbound traffic
		switch n.NodeType {
		case graph.NodeTypeWorkload:
			query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,source_workload_namespace="%s",source_workload="%s"} [%vs])) by (%s) %s`,
				metric,
				sourceCluster,
				namespace,
				n.Workload,
				int(duration.Seconds()), // range duration for the query
				groupBy,
				idleCondition)
		case graph.NodeTypeApp:
			if graph.IsOK(n.Version) {
				query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,source_workload_namespace="%s",source_canonical_service="%s",source_canonical_revision="%s"} [%vs])) by (%s) %s`,
					metric,
					sourceCluster,
					namespace,
					n.App,
					n.Version,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			} else {
				query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,source_workload_namespace="%s",source_canonical_service="%s"} [%vs])) by (%s) %s`,
					metric,
					sourceCluster,
					namespace,
					n.App,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			}
		case graph.NodeTypeService:
			query = ""
		default:
			graph.Error(fmt.Sprintf("NodeType [%s] not supported", n.NodeType))
		}
		outVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
		populateTrafficMap(trafficMap, &outVector, metric, o)
	}

	// gRPC message traffic
	if o.Rates.Grpc != graph.RateNone && o.Rates.Grpc != graph.RateRequests {
		var metrics []string
		groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision"

		switch o.Rates.Grpc {
		case graph.RateReceived:
			metrics = []string{"istio_response_messages_total"}
		case graph.RateSent:
			metrics = []string{"istio_request_messages_total"}
		case graph.RateTotal:
			metrics = []string{"istio_request_messages_total", "istio_response_messages_total"}
		default:
			metrics = []string{}
		}

		for _, metric := range metrics {
			var query string

			switch n.NodeType {
			case graph.NodeTypeWorkload:
				query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_workload_namespace="%s",destination_workload="%s"} [%vs])) by (%s) %s`,
					metric,
					destCluster,
					namespace,
					n.Workload,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			case graph.NodeTypeApp:
				if graph.IsOK(n.Version) {
					query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_service_namespace="%s",destination_canonical_service="%s",destination_canonical_revision="%s"} [%vs])) by (%s) %s`,
						metric,
						destCluster,
						namespace,
						n.App,
						n.Version,
						int(duration.Seconds()), // range duration for the query
						groupBy,
						idleCondition)
				} else {
					query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_service_namespace="%s",destination_canonical_service="%s"} [%vs])) by (%s) %s`,
						metric,
						destCluster,
						namespace,
						n.App,
						int(duration.Seconds()), // range duration for the query
						groupBy,
						idleCondition)
				}
			case graph.NodeTypeService:
				// TODO: Do we need to handle requests from unknown in a special way (like in HTTP above)? Not sure how gRPC-messages is reported from unknown.
				query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_service_namespace="%s",destination_service=~"^%s\\.%s\\..*$"} [%vs])) by (%s) %s`,
					metric,
					destCluster,
					namespace,
					n.Service,
					namespace,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			default:
				graph.Error(fmt.Sprintf("NodeType [%s] not supported", n.NodeType))
			}
			incomingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &incomingVector, metric, o)

			// 2) query for outbound traffic
			switch n.NodeType {
			case graph.NodeTypeWorkload:
				query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,source_workload_namespace="%s",source_workload="%s"} [%vs])) by (%s) %s`,
					metric,
					sourceCluster,
					namespace,
					n.Workload,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			case graph.NodeTypeApp:
				if graph.IsOK(n.Version) {
					query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,source_workload_namespace="%s",source_canonical_service="%s",source_canonical_revision="%s"} [%vs])) by (%s) %s`,
						metric,
						sourceCluster,
						namespace,
						n.App,
						n.Version,
						int(duration.Seconds()), // range duration for the query
						groupBy,
						idleCondition)
				} else {
					query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,source_workload_namespace="%s",source_canonical_service="%s"} [%vs])) by (%s) %s`,
						metric,
						sourceCluster,
						namespace,
						n.App,
						int(duration.Seconds()), // range duration for the query
						groupBy,
						idleCondition)
				}
			case graph.NodeTypeService:
				query = ""
			default:
				graph.Error(fmt.Sprintf("NodeType [%s] not supported", n.NodeType))
			}
			outgoingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &outgoingVector, metric, o)
		}
	}

	// TCP byte traffic
	if o.Rates.Tcp != graph.RateNone {
		var metrics []string
		groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags"

		switch o.Rates.Tcp {
		case graph.RateReceived:
			metrics = []string{"istio_tcp_received_bytes_total"}
		case graph.RateSent:
			metrics = []string{"istio_tcp_sent_bytes_total"}
		case graph.RateTotal:
			metrics = []string{"istio_tcp_sent_bytes_total", "istio_tcp_received_bytes_total"}
		default:
			metrics = []string{}
		}

		for _, metric := range metrics {
			var query string

			switch n.NodeType {
			case graph.NodeTypeWorkload:
				query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_workload_namespace="%s",destination_workload="%s"} [%vs])) by (%s) %s`,
					metric,
					destCluster,
					namespace,
					n.Workload,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			case graph.NodeTypeApp:
				if graph.IsOK(n.Version) {
					query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_service_namespace="%s",destination_canonical_service="%s",destination_canonical_revision="%s"} [%vs])) by (%s) %s`,
						metric,
						destCluster,
						namespace,
						n.App,
						n.Version,
						int(duration.Seconds()), // range duration for the query
						groupBy,
						idleCondition)
				} else {
					query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_service_namespace="%s",destination_canonical_service="%s"} [%vs])) by (%s) %s`,
						metric,
						destCluster,
						namespace,
						n.App,
						int(duration.Seconds()), // range duration for the query
						groupBy,
						idleCondition)
				}
			case graph.NodeTypeService:
				// TODO: Do we need to handle requests from unknown in a special way (like in HTTP above)? Not sure how tcp is reported from unknown.
				query = fmt.Sprintf(`sum(rate(%s{reporter="destination"%s,destination_service_namespace="%s",destination_service=~"^%s\\.%s\\..*$"} [%vs])) by (%s) %s`,
					metric,
					destCluster,
					namespace,
					n.Service,
					namespace,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			default:
				graph.Error(fmt.Sprintf("NodeType [%s] not supported", n.NodeType))
			}
			incomingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &incomingVector, metric, o)

			// 2) query for outbound traffic
			switch n.NodeType {
			case graph.NodeTypeWorkload:
				query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,source_workload_namespace="%s",source_workload="%s"} [%vs])) by (%s) %s`,
					metric,
					sourceCluster,
					namespace,
					n.Workload,
					int(duration.Seconds()), // range duration for the query
					groupBy,
					idleCondition)
			case graph.NodeTypeApp:
				if graph.IsOK(n.Version) {
					query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,source_workload_namespace="%s",source_canonical_service="%s",source_canonical_revision="%s"} [%vs])) by (%s) %s`,
						metric,
						sourceCluster,
						namespace,
						n.App,
						n.Version,
						int(duration.Seconds()), // range duration for the query
						groupBy,
						idleCondition)
				} else {
					query = fmt.Sprintf(`sum(rate(%s{reporter="source"%s,source_workload_namespace="%s",source_canonical_service="%s"} [%vs])) by (%s) %s`,
						metric,
						sourceCluster,
						namespace,
						n.App,
						int(duration.Seconds()), // range duration for the query
						groupBy,
						idleCondition)
				}
			case graph.NodeTypeService:
				query = ""
			default:
				graph.Error(fmt.Sprintf("NodeType [%s] not supported", n.NodeType))
			}
			outgoingVector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
			populateTrafficMap(trafficMap, &outgoingVector, metric, o)
		}
	}

	return trafficMap
}

func handleAggregateNodeTrafficMap(o graph.TelemetryOptions, client *prometheus.Client, globalInfo *graph.AppenderGlobalInfo) graph.TrafficMap {
	n := graph.NewAggregateNode(o.NodeOptions.Cluster, o.NodeOptions.Namespace, o.NodeOptions.Aggregate, o.NodeOptions.AggregateValue, o.NodeOptions.Service, o.NodeOptions.App)

	log.Tracef("Build graph for aggregate node [%+v]", n)

	if !o.Appenders.All {
		o.Appenders.AppenderNames = append(o.Appenders.AppenderNames, appender.AggregateNodeAppenderName)
	}
	appenders := appender.ParseAppenders(o)
	trafficMap := buildAggregateNodeTrafficMap(o.NodeOptions.Namespace, n, o, client)

	namespaceInfo := graph.NewAppenderNamespaceInfo(o.NodeOptions.Namespace)

	for _, a := range appenders {
		appenderTimer := internalmetrics.GetGraphAppenderTimePrometheusTimer(a.Name())
		a.AppendGraph(trafficMap, globalInfo, namespaceInfo)
		appenderTimer.ObserveDuration()
	}

	// The appenders can add/remove/alter nodes. After the manipulations are complete
	// we can make some final adjustments:
	// - mark the outsiders (i.e. nodes not in the requested namespaces)
	// - mark the traffic generators
	telemetry.MarkOutsideOrInaccessible(trafficMap, o)
	telemetry.MarkTrafficGenerators(trafficMap)

	return trafficMap
}

// buildAggregateNodeTrafficMap returns a map of all incoming and outgoing traffic from the perspective of the aggregate. Aggregates
// are always generated for serviced requests and therefore via destination telemetry.
func buildAggregateNodeTrafficMap(namespace string, n graph.Node, o graph.TelemetryOptions, client *prometheus.Client) graph.TrafficMap {
	interval := o.Namespaces[namespace].Duration

	// create map to aggregate traffic by response code
	trafficMap := graph.NewTrafficMap()

	// It takes only one prometheus query to get everything involving the target operation
	serviceFragment := ""
	if n.Service != "" {
		serviceFragment = fmt.Sprintf(`,destination_service_name="%s"`, n.Service)
	}
	metric := "istio_requests_total"
	groupBy := "source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags"
	httpQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_service_namespace="%s",%s="%s"%s}[%vs])) by (%s) > 0`,
		metric,
		namespace,
		n.Metadata[graph.Aggregate],
		n.Metadata[graph.AggregateValue],
		serviceFragment,
		int(interval.Seconds()), // range duration for the query
		groupBy)
	/* It's not clear that request classification makes sense for TCP metrics. Because it costs us queries I'm
	   removing the support for now, we can add it back if someone presents a valid use case. (same for gRCP message metrics)
	tcpQuery := fmt.Sprintf(`sum(rate(%s{reporter="destination",destination_service_namespace="%s",%s="%s"}[%vs])) by (%s) > 0`,
		"istio_tcp_sent_bytes_total",
		namespace,
		n.Metadata[graph.Aggregate],
		n.Metadata[graph.AggregateValue],
		int(interval.Seconds()), // range duration for the query
		groupBy)
	query := fmt.Sprintf(`(%s) OR (%s)`, httpQuery, tcpQuery)
	*/
	query := httpQuery
	vector := promQuery(query, time.Unix(o.QueryTime, 0), client.API())
	populateTrafficMap(trafficMap, &vector, metric, o)

	return trafficMap
}

func promQuery(query string, queryTime time.Time, api prom_v1.API) model.Vector {
	if query == "" {
		return model.Vector{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// wrap with a round() to be in line with metrics api
	query = fmt.Sprintf("round(%s,0.001)", query)
	log.Tracef("Graph query:\n%s@time=%v (now=%v, %v)\n", query, queryTime.Format(graph.TF), time.Now().Format(graph.TF), queryTime.Unix())

	promtimer := internalmetrics.GetPrometheusProcessingTimePrometheusTimer("Graph-Generation")
	value, warnings, err := api.Query(ctx, query, queryTime)
	if warnings != nil && len(warnings) > 0 {
		log.Warningf("promQuery. Prometheus Warnings: [%s]", strings.Join(warnings, ","))
	}
	graph.CheckUnavailable(err)
	promtimer.ObserveDuration() // notice we only collect metrics for successful prom queries

	switch t := value.Type(); t {
	case model.ValVector: // Instant Vector
		return value.(model.Vector)
	default:
		graph.Error(fmt.Sprintf("No handling for type %v!\n", t))
	}

	return nil
}
