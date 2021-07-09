package appender

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"github.com/kiali/kiali/graph"
)

func TestNamespacesGraphWithServiceInjection(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_requests_total{reporter="destination",source_workload_namespace!="bookinfo",destination_service_namespace="bookinfo",request_operation!="unknown"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,request_operation) > 0,0.001)`
	v0 := model.Vector{}

	q1 := `round(sum(rate(istio_requests_total{reporter="destination",source_workload_namespace="bookinfo",request_operation!="unknown"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,request_operation) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"response_code":                  "200",
		"response_flags":                 "",
		"request_protocol":               "http",
		"request_operation":              "Top"}
	q1m1 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"response_code":                  "200",
		"response_flags":                 "",
		"request_protocol":               "http",
		"request_operation":              "All"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  70},
		&model.Sample{
			Metric: q1m1,
			Value:  30}}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)

	trafficMap := aggregateNodeTestTraffic(true)
	ppID, _ := graph.Id(graph.Unknown, "bookinfo", "productpage", "bookinfo", "productpage-v1", "productpage", "v1", graph.GraphTypeVersionedApp)
	pp, ok := trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeService, pp.Edges[0].Dest.NodeType)

	duration, _ := time.ParseDuration("60s")
	appender := AggregateNodeAppender{
		Aggregate:          "request_operation",
		GraphType:          graph.GraphTypeVersionedApp,
		InjectServiceNodes: true,
		Namespaces: map[string]graph.NamespaceInfo{
			"bookinfo": {
				Name:     "bookinfo",
				Duration: duration,
			},
		},
		QueryTime: time.Now().Unix(),
		Rates: graph.RequestedRates{
			Grpc: graph.RateRequests,
			Http: graph.RateRequests,
			Tcp:  graph.RateTotal,
		},
	}

	appender.appendGraph(trafficMap, "bookinfo", client)

	pp, ok = trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(2, len(pp.Edges))
	assert.Equal(graph.NodeTypeAggregate, pp.Edges[0].Dest.NodeType)
	assert.Equal(graph.NodeTypeAggregate, pp.Edges[1].Dest.NodeType)

	topReviews := pp.Edges[0].Dest
	if "Top" != topReviews.Metadata[graph.AggregateValue] {
		topReviews = pp.Edges[1].Dest
	}
	assert.Equal("request_operation", topReviews.Metadata[graph.Aggregate])
	assert.Equal("Top", topReviews.Metadata[graph.AggregateValue])
	assert.Equal("reviews", topReviews.App)
	assert.Equal("reviews", topReviews.Service)
	assert.Equal(1, len(topReviews.Edges))
	assert.Equal(graph.NodeTypeService, topReviews.Edges[0].Dest.NodeType)

	allReviews := pp.Edges[1].Dest
	if "All" != allReviews.Metadata[graph.AggregateValue] {
		allReviews = pp.Edges[0].Dest
	}
	assert.Equal("request_operation", allReviews.Metadata[graph.Aggregate])
	assert.Equal("All", allReviews.Metadata[graph.AggregateValue])
	assert.Equal("reviews", allReviews.App)
	assert.Equal("reviews", allReviews.Service)
	assert.Equal(1, len(allReviews.Edges))
	assert.Equal(graph.NodeTypeService, allReviews.Edges[0].Dest.NodeType)

	assert.Equal(topReviews.Edges[0].Dest.ID, allReviews.Edges[0].Dest.ID)

	reviewsService := topReviews.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(1, len(reviewsService.Edges))

	reviews := reviewsService.Edges[0].Dest
	assert.Equal("reviews", reviews.App)
	assert.Equal("v1", reviews.Version)
}

func TestNamespacesGraphNoServiceInjection(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_requests_total{reporter="destination",source_workload_namespace!="bookinfo",destination_service_namespace="bookinfo",request_operation!="unknown"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,request_operation) > 0,0.001)`
	v0 := model.Vector{}

	q1 := `round(sum(rate(istio_requests_total{reporter="destination",source_workload_namespace="bookinfo",request_operation!="unknown"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,request_operation) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"response_code":                  "200",
		"response_flags":                 "",
		"request_protocol":               "http",
		"request_operation":              "Top"}
	q1m1 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"response_code":                  "200",
		"response_flags":                 "",
		"request_protocol":               "http",
		"request_operation":              "All"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  70},
		&model.Sample{
			Metric: q1m1,
			Value:  30}}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)

	trafficMap := aggregateNodeTestTraffic(false)
	ppID, _ := graph.Id(graph.Unknown, "bookinfo", "productpage", "bookinfo", "productpage-v1", "productpage", "v1", graph.GraphTypeVersionedApp)
	pp, ok := trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeApp, pp.Edges[0].Dest.NodeType)

	duration, _ := time.ParseDuration("60s")
	appender := AggregateNodeAppender{
		Aggregate:          "request_operation",
		GraphType:          graph.GraphTypeVersionedApp,
		InjectServiceNodes: false,
		Namespaces: map[string]graph.NamespaceInfo{
			"bookinfo": {
				Name:     "bookinfo",
				Duration: duration,
			},
		},
		QueryTime: time.Now().Unix(),
		Rates: graph.RequestedRates{
			Grpc: graph.RateRequests,
			Http: graph.RateRequests,
			Tcp:  graph.RateTotal,
		}}

	appender.appendGraph(trafficMap, "bookinfo", client)

	pp, ok = trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(2, len(pp.Edges))
	assert.Equal(graph.NodeTypeAggregate, pp.Edges[0].Dest.NodeType)
	assert.Equal(graph.NodeTypeAggregate, pp.Edges[1].Dest.NodeType)

	topReviews := pp.Edges[0].Dest
	if "Top" != topReviews.Metadata[graph.AggregateValue] {
		topReviews = pp.Edges[1].Dest
	}
	assert.Equal("request_operation", topReviews.Metadata[graph.Aggregate])
	assert.Equal("Top", topReviews.Metadata[graph.AggregateValue])
	assert.Equal("", topReviews.App)
	assert.Equal(1, len(topReviews.Edges))
	assert.Equal(graph.NodeTypeApp, topReviews.Edges[0].Dest.NodeType)

	allReviews := pp.Edges[1].Dest
	if "All" != allReviews.Metadata[graph.AggregateValue] {
		allReviews = pp.Edges[0].Dest
	}
	assert.Equal("request_operation", allReviews.Metadata[graph.Aggregate])
	assert.Equal("All", allReviews.Metadata[graph.AggregateValue])
	assert.Equal("", allReviews.App)
	assert.Equal(1, len(allReviews.Edges))
	assert.Equal(graph.NodeTypeApp, allReviews.Edges[0].Dest.NodeType)

	assert.Equal(topReviews.Edges[0].Dest.ID, allReviews.Edges[0].Dest.ID)

	reviews := topReviews.Edges[0].Dest
	assert.Equal(graph.NodeTypeApp, reviews.NodeType)
	assert.Equal("reviews", reviews.App)
	assert.Equal(0, len(reviews.Edges))
}

func TestNodeGraphWithServiceInjection(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_requests_total{reporter="destination",destination_service_namespace="bookinfo",request_operation="Top",destination_service_name="reviews"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,request_operation) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"response_code":                  "200",
		"response_flags":                 "",
		"request_protocol":               "http",
		"request_operation":              "Top"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  70}}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)

	trafficMap := aggregateNodeTestTraffic(true)
	ppID, _ := graph.Id(graph.Unknown, "bookinfo", "productpage", "bookinfo", "productpage-v1", "productpage", "v1", graph.GraphTypeVersionedApp)
	pp, ok := trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeService, pp.Edges[0].Dest.NodeType)

	duration, _ := time.ParseDuration("60s")
	appender := AggregateNodeAppender{
		Aggregate:          "request_operation",
		AggregateValue:     "Top",
		GraphType:          graph.GraphTypeVersionedApp,
		InjectServiceNodes: true,
		Namespaces: map[string]graph.NamespaceInfo{
			"bookinfo": {
				Name:     "bookinfo",
				Duration: duration,
			},
		},
		QueryTime: time.Now().Unix(),
		Rates: graph.RequestedRates{
			Grpc: graph.RateRequests,
			Http: graph.RateRequests,
			Tcp:  graph.RateTotal,
		},
		Service: "reviews",
	}

	appender.appendNodeGraph(trafficMap, "bookinfo", client)

	pp, ok = trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeAggregate, pp.Edges[0].Dest.NodeType)

	topReviews := pp.Edges[0].Dest
	assert.Equal("Top", topReviews.Metadata[graph.AggregateValue])
	assert.Equal("request_operation", topReviews.Metadata[graph.Aggregate])
	assert.Equal("Top", topReviews.Metadata[graph.AggregateValue])
	assert.Equal("reviews", topReviews.App)
	assert.Equal(1, len(topReviews.Edges))
	assert.Equal(graph.NodeTypeService, topReviews.Edges[0].Dest.NodeType)

	reviewsService := topReviews.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(1, len(reviewsService.Edges))

	reviews := reviewsService.Edges[0].Dest
	assert.Equal("reviews", reviews.App)
	assert.Equal("v1", reviews.Version)
}

func TestNamespacesGraphWithServiceInjectionSkipRates(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_requests_total{reporter="destination",source_workload_namespace!="bookinfo",destination_service_namespace="bookinfo",request_operation!="unknown"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,request_operation) > 0,0.001)`
	v0 := model.Vector{}

	q1 := `round(sum(rate(istio_requests_total{reporter="destination",source_workload_namespace="bookinfo",request_operation!="unknown"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,request_operation) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"response_code":                  "200",
		"response_flags":                 "",
		"request_protocol":               "http",
		"request_operation":              "Top"}
	q1m1 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"response_code":                  "200",
		"response_flags":                 "",
		"request_protocol":               "http",
		"request_operation":              "All"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  70},
		&model.Sample{
			Metric: q1m1,
			Value:  30}}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)

	trafficMap := aggregateNodeTestTraffic(true)
	ppID, _ := graph.Id(graph.Unknown, "bookinfo", "productpage", "bookinfo", "productpage-v1", "productpage", "v1", graph.GraphTypeVersionedApp)
	pp, ok := trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeService, pp.Edges[0].Dest.NodeType)

	duration, _ := time.ParseDuration("60s")
	appender := AggregateNodeAppender{
		Aggregate:          "request_operation",
		GraphType:          graph.GraphTypeVersionedApp,
		InjectServiceNodes: true,
		Namespaces: map[string]graph.NamespaceInfo{
			"bookinfo": {
				Name:     "bookinfo",
				Duration: duration,
			},
		},
		QueryTime: time.Now().Unix(),
		Rates: graph.RequestedRates{
			Grpc: graph.RateRequests,
			Http: graph.RateNone,
			Tcp:  graph.RateTotal,
		},
	}

	appender.appendGraph(trafficMap, "bookinfo", client)

	pp, ok = trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeService, pp.Edges[0].Dest.NodeType)

	reviewsService := pp.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(1, len(reviewsService.Edges))

	reviews := reviewsService.Edges[0].Dest
	assert.Equal("reviews", reviews.App)
	assert.Equal("v1", reviews.Version)
}

func TestNodeGraphNoServiceInjection(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_requests_total{reporter="destination",destination_service_namespace="bookinfo",request_operation="Top"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,request_operation) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"response_code":                  "200",
		"response_flags":                 "",
		"request_protocol":               "http",
		"request_operation":              "Top"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  70}}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)

	trafficMap := aggregateNodeTestTraffic(false)
	ppID, _ := graph.Id(graph.Unknown, "bookinfo", "productpage", "bookinfo", "productpage-v1", "productpage", "v1", graph.GraphTypeVersionedApp)
	pp, ok := trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeApp, pp.Edges[0].Dest.NodeType)

	duration, _ := time.ParseDuration("60s")
	appender := AggregateNodeAppender{
		Aggregate:          "request_operation",
		AggregateValue:     "Top",
		GraphType:          graph.GraphTypeVersionedApp,
		InjectServiceNodes: false,
		Namespaces: map[string]graph.NamespaceInfo{
			"bookinfo": {
				Name:     "bookinfo",
				Duration: duration,
			},
		},
		QueryTime: time.Now().Unix(),
		Rates: graph.RequestedRates{
			Grpc: graph.RateRequests,
			Http: graph.RateRequests,
			Tcp:  graph.RateTotal,
		},
	}

	appender.appendNodeGraph(trafficMap, "bookinfo", client)

	pp, ok = trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeAggregate, pp.Edges[0].Dest.NodeType)

	topReviews := pp.Edges[0].Dest
	assert.Equal("Top", topReviews.Metadata[graph.AggregateValue])
	assert.Equal("request_operation", topReviews.Metadata[graph.Aggregate])
	assert.Equal("Top", topReviews.Metadata[graph.AggregateValue])
	assert.Equal("", topReviews.App)
	assert.Equal(1, len(topReviews.Edges))
	assert.Equal(graph.NodeTypeApp, topReviews.Edges[0].Dest.NodeType)

	reviews := topReviews.Edges[0].Dest
	assert.Equal("reviews", reviews.App)
	assert.Equal("v1", reviews.Version)
}

func TestNodeGraphWithServiceInjectionSkipRates(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_requests_total{reporter="destination",destination_service_namespace="bookinfo",request_operation="Top",destination_service_name="reviews"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags,request_operation) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"response_code":                  "200",
		"response_flags":                 "",
		"request_protocol":               "http",
		"request_operation":              "Top"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  70}}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)

	trafficMap := aggregateNodeTestTraffic(true)
	ppID, _ := graph.Id(graph.Unknown, "bookinfo", "productpage", "bookinfo", "productpage-v1", "productpage", "v1", graph.GraphTypeVersionedApp)
	pp, ok := trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeService, pp.Edges[0].Dest.NodeType)

	duration, _ := time.ParseDuration("60s")
	appender := AggregateNodeAppender{
		Aggregate:          "request_operation",
		AggregateValue:     "Top",
		GraphType:          graph.GraphTypeVersionedApp,
		InjectServiceNodes: true,
		Namespaces: map[string]graph.NamespaceInfo{
			"bookinfo": {
				Name:     "bookinfo",
				Duration: duration,
			},
		},
		QueryTime: time.Now().Unix(),
		Rates: graph.RequestedRates{
			Grpc: graph.RateRequests,
			Http: graph.RateNone,
			Tcp:  graph.RateTotal,
		},
		Service: "reviews",
	}

	appender.appendNodeGraph(trafficMap, "bookinfo", client)

	pp, ok = trafficMap[ppID]
	assert.Equal(true, ok)
	assert.Equal(1, len(pp.Edges))
	assert.Equal(graph.NodeTypeService, pp.Edges[0].Dest.NodeType)

	reviewsService := pp.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(1, len(reviewsService.Edges))

	reviews := reviewsService.Edges[0].Dest
	assert.Equal("reviews", reviews.App)
	assert.Equal("v1", reviews.Version)
}

func aggregateNodeTestTraffic(injectServices bool) graph.TrafficMap {
	productpage := graph.NewNode(graph.Unknown, "bookinfo", "productpage", "bookinfo", "productpage-v1", "productpage", "v1", graph.GraphTypeVersionedApp)
	reviews := graph.NewNode(graph.Unknown, "bookinfo", "reviews", "bookinfo", "reviews-v1", "reviews", "v1", graph.GraphTypeVersionedApp)
	reviewsService := graph.NewNode(graph.Unknown, "bookinfo", "reviews", "", "", "", "", graph.GraphTypeVersionedApp)

	trafficMap := graph.NewTrafficMap()
	trafficMap[productpage.ID] = &productpage
	trafficMap[reviews.ID] = &reviews
	if injectServices {
		trafficMap[reviewsService.ID] = &reviewsService
		productpage.AddEdge(&reviewsService)
		reviewsService.AddEdge(&reviews)
	} else {
		productpage.AddEdge(&reviews)
	}

	return trafficMap
}
