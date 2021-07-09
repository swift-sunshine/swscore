package appender

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"github.com/kiali/kiali/graph"
)

func TestResponseThroughput(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_response_bytes_sum{reporter="destination",source_workload_namespace!="bookinfo",destination_service_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      model.LabelValue(graph.Unknown),
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage.bookinfo.svc.cluster.local",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  1000.0},
	}

	q1 := `round(sum(rate(istio_response_bytes_sum{reporter="destination",source_workload_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
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
		"destination_canonical_revision": "v1"}
	q1m1 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v2"}
	q1m2 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v1",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings.bookinfo.svc.cluster.local",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1"}
	q1m3 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v2",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v2",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings.bookinfo.svc.cluster.local",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  1000.0},
		&model.Sample{
			Metric: q1m1,
			Value:  2000.0},
		&model.Sample{
			Metric: q1m2,
			Value:  1000.0},
		&model.Sample{
			Metric: q1m3,
			Value:  2000.0}}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)

	trafficMap := responseThroughputTestTraffic()
	ingressID, _ := graph.Id(graph.Unknown, "istio-system", "", "istio-system", "ingressgateway-unknown", "ingressgateway", graph.Unknown, graph.GraphTypeVersionedApp)
	ingress, ok := trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	assert.Equal(nil, ingress.Edges[0].Metadata[graph.Throughput])

	duration, _ := time.ParseDuration("60s")
	appender := ThroughputAppender{
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
		ThroughputType: "response",
	}

	appender.appendGraph(trafficMap, "bookinfo", client)

	ingress, ok = trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	_, ok = ingress.Edges[0].Metadata[graph.Throughput]
	assert.Equal(false, ok)

	productpageService := ingress.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, productpageService.NodeType)
	assert.Equal("productpage", productpageService.Service)
	assert.Equal(nil, productpageService.Metadata[graph.Throughput])
	assert.Equal(1, len(productpageService.Edges))
	assert.Equal(1000.0, productpageService.Edges[0].Metadata[graph.Throughput])

	productpage := productpageService.Edges[0].Dest
	assert.Equal("productpage", productpage.App)
	assert.Equal("v1", productpage.Version)
	assert.Equal(nil, productpage.Metadata[graph.Throughput])
	assert.Equal(1, len(productpage.Edges))
	_, ok = productpage.Edges[0].Metadata[graph.Throughput]
	assert.Equal(false, ok)

	reviewsService := productpage.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(nil, reviewsService.Metadata[graph.Throughput])
	assert.Equal(2, len(reviewsService.Edges))
	assert.Equal(1000.0, reviewsService.Edges[0].Metadata[graph.Throughput])
	assert.Equal(2000.0, reviewsService.Edges[1].Metadata[graph.Throughput])

	reviews1 := reviewsService.Edges[0].Dest
	assert.Equal("reviews", reviews1.App)
	assert.Equal("v1", reviews1.Version)
	assert.Equal(nil, reviews1.Metadata[graph.Throughput])
	assert.Equal(1, len(reviews1.Edges))
	_, ok = reviews1.Edges[0].Metadata[graph.Throughput]
	assert.Equal(false, ok)

	reviews2 := reviewsService.Edges[1].Dest
	assert.Equal("reviews", reviews2.App)
	assert.Equal("v2", reviews2.Version)
	assert.Equal(nil, reviews2.Metadata[graph.Throughput])
	assert.Equal(1, len(reviews2.Edges))
	_, ok = reviews2.Edges[0].Metadata[graph.Throughput]
	assert.False(ok)

	ratingsService := reviews1.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, ratingsService.NodeType)
	assert.Equal("ratings", ratingsService.Service)
	assert.Equal(nil, ratingsService.Metadata[graph.Throughput])
	assert.Equal(1, len(ratingsService.Edges))
	assert.Equal(3000.0, ratingsService.Edges[0].Metadata[graph.Throughput])

	assert.Equal(ratingsService, reviews2.Edges[0].Dest)

	ratings := ratingsService.Edges[0].Dest
	assert.Equal("ratings", ratings.App)
	assert.Equal("v1", ratings.Version)
	assert.Equal(nil, ratings.Metadata[graph.Throughput])
	assert.Equal(0, len(ratings.Edges))
}

func responseThroughputTestTraffic() graph.TrafficMap {
	ingress := graph.NewNode(graph.Unknown, "istio-system", "", "istio-system", "ingressgateway-unknown", "ingressgateway", graph.Unknown, graph.GraphTypeVersionedApp)
	productpageService := graph.NewNode(graph.Unknown, "bookinfo", "productpage", "", "", "", "", graph.GraphTypeVersionedApp)
	productpage := graph.NewNode(graph.Unknown, "bookinfo", "productpage", "bookinfo", "productpage-v1", "productpage", "v1", graph.GraphTypeVersionedApp)
	reviewsService := graph.NewNode(graph.Unknown, "bookinfo", "reviews", "", "", "", "", graph.GraphTypeVersionedApp)
	reviewsV1 := graph.NewNode(graph.Unknown, "bookinfo", "reviews", "bookinfo", "reviews-v1", "reviews", "v1", graph.GraphTypeVersionedApp)
	reviewsV2 := graph.NewNode(graph.Unknown, "bookinfo", "reviews", "bookinfo", "reviews-v2", "reviews", "v2", graph.GraphTypeVersionedApp)
	ratingsService := graph.NewNode(graph.Unknown, "bookinfo", "ratings", "", "", "", "", graph.GraphTypeVersionedApp)
	ratings := graph.NewNode(graph.Unknown, "bookinfo", "ratings", "bookinfo", "ratings-v1", "ratings", "v1", graph.GraphTypeVersionedApp)
	trafficMap := graph.NewTrafficMap()

	trafficMap[ingress.ID] = &ingress
	trafficMap[productpageService.ID] = &productpageService
	trafficMap[productpage.ID] = &productpage
	trafficMap[reviewsService.ID] = &reviewsService
	trafficMap[reviewsV1.ID] = &reviewsV1
	trafficMap[reviewsV2.ID] = &reviewsV2
	trafficMap[ratingsService.ID] = &ratingsService
	trafficMap[ratings.ID] = &ratings

	ingress.AddEdge(&productpageService).Metadata[graph.ProtocolKey] = "http"
	productpageService.AddEdge(&productpage).Metadata[graph.ProtocolKey] = "http"
	productpage.AddEdge(&reviewsService).Metadata[graph.ProtocolKey] = "http"
	reviewsService.AddEdge(&reviewsV1).Metadata[graph.ProtocolKey] = "http"
	reviewsService.AddEdge(&reviewsV2).Metadata[graph.ProtocolKey] = "http"
	reviewsV1.AddEdge(&ratingsService).Metadata[graph.ProtocolKey] = "http"
	reviewsV2.AddEdge(&ratingsService).Metadata[graph.ProtocolKey] = "http"
	ratingsService.AddEdge(&ratings).Metadata[graph.ProtocolKey] = "http"

	return trafficMap
}

func TestRequestThroughput(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_request_bytes_sum{reporter="source",source_workload_namespace!="bookinfo",destination_service_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      model.LabelValue(graph.Unknown),
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage.bookinfo.svc.cluster.local",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  1000.0},
	}

	q1 := `round(sum(rate(istio_request_bytes_sum{reporter="source",source_workload_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "unknown", // simulate failed requests to reviews service
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  1000.0}}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)

	trafficMap := requestThroughputTestTraffic()
	ingressID, _ := graph.Id(graph.Unknown, "istio-system", "", "istio-system", "ingressgateway-unknown", "ingressgateway", graph.Unknown, graph.GraphTypeVersionedApp)
	ingress, ok := trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	assert.Equal(nil, ingress.Edges[0].Metadata[graph.Throughput])

	duration, _ := time.ParseDuration("60s")
	appender := ThroughputAppender{
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
		ThroughputType: "request",
	}

	appender.appendGraph(trafficMap, "bookinfo", client)

	ingress, ok = trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	_, ok = ingress.Edges[0].Metadata[graph.Throughput]
	assert.Equal(false, ok)

	productpageService := ingress.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, productpageService.NodeType)
	assert.Equal("productpage", productpageService.Service)
	assert.Equal(nil, productpageService.Metadata[graph.Throughput])
	assert.Equal(1, len(productpageService.Edges))
	assert.Equal(1000.0, productpageService.Edges[0].Metadata[graph.Throughput])

	productpage := productpageService.Edges[0].Dest
	assert.Equal("productpage", productpage.App)
	assert.Equal("v1", productpage.Version)
	assert.Equal(nil, productpage.Metadata[graph.Throughput])
	assert.Equal(1, len(productpage.Edges))
	assert.Equal(1000.0, productpage.Edges[0].Metadata[graph.Throughput])

	reviewsService := productpage.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(nil, reviewsService.Metadata[graph.Throughput])
	assert.Equal(0, len(reviewsService.Edges))
}

func TestRequestThroughputSkipRates(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_request_bytes_sum{reporter="source",source_workload_namespace!="bookinfo",destination_service_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      model.LabelValue(graph.Unknown),
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage.bookinfo.svc.cluster.local",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  1000.0},
	}

	q1 := `round(sum(rate(istio_request_bytes_sum{reporter="source",source_workload_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "unknown", // simulate failed requests to reviews service
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  1000.0}}

	_, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)

	trafficMap := requestThroughputTestTraffic()
	ingressID, _ := graph.Id(graph.Unknown, "istio-system", "", "istio-system", "ingressgateway-unknown", "ingressgateway", graph.Unknown, graph.GraphTypeVersionedApp)
	ingress, ok := trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	assert.Equal(nil, ingress.Edges[0].Metadata[graph.Throughput])

	duration, _ := time.ParseDuration("60s")
	appender := ThroughputAppender{
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
		ThroughputType: "request",
	}

	appender.AppendGraph(trafficMap, nil, nil)

	ingress, ok = trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	_, ok = ingress.Edges[0].Metadata[graph.Throughput]
	assert.Equal(false, ok)

	productpageService := ingress.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, productpageService.NodeType)
	assert.Equal("productpage", productpageService.Service)
	assert.Equal(nil, productpageService.Metadata[graph.Throughput])
	assert.Equal(1, len(productpageService.Edges))
	assert.Equal(nil, productpageService.Edges[0].Metadata[graph.Throughput])

	productpage := productpageService.Edges[0].Dest
	assert.Equal("productpage", productpage.App)
	assert.Equal("v1", productpage.Version)
	assert.Equal(nil, productpage.Metadata[graph.Throughput])
	assert.Equal(1, len(productpage.Edges))
	assert.Equal(nil, productpage.Edges[0].Metadata[graph.Throughput])

	reviewsService := productpage.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(nil, reviewsService.Metadata[graph.Throughput])
	assert.Equal(0, len(reviewsService.Edges))
}

func requestThroughputTestTraffic() graph.TrafficMap {
	ingress := graph.NewNode(graph.Unknown, "istio-system", "", "istio-system", "ingressgateway-unknown", "ingressgateway", graph.Unknown, graph.GraphTypeVersionedApp)
	productpageService := graph.NewNode(graph.Unknown, "bookinfo", "productpage", "", "", "", "", graph.GraphTypeVersionedApp)
	productpage := graph.NewNode(graph.Unknown, "bookinfo", "productpage", "bookinfo", "productpage-v1", "productpage", "v1", graph.GraphTypeVersionedApp)
	reviewsService := graph.NewNode(graph.Unknown, "bookinfo", "reviews", "", "", "", "", graph.GraphTypeVersionedApp)
	trafficMap := graph.NewTrafficMap()

	trafficMap[ingress.ID] = &ingress
	trafficMap[productpageService.ID] = &productpageService
	trafficMap[productpage.ID] = &productpage
	trafficMap[reviewsService.ID] = &reviewsService

	ingress.AddEdge(&productpageService).Metadata[graph.ProtocolKey] = "http"
	productpageService.AddEdge(&productpage).Metadata[graph.ProtocolKey] = "http"
	productpage.AddEdge(&reviewsService).Metadata[graph.ProtocolKey] = "http"

	return trafficMap
}
