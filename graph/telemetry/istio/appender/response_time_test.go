package appender

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"github.com/kiali/kiali/graph"
)

func TestResponseTimeP95(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(histogram_quantile(0.95, sum(rate(istio_request_duration_milliseconds_bucket{reporter="destination",destination_service_namespace="bookinfo"}[60s])) by (le,source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol)) > 0,0.001)`
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
	q0m1 := model.Metric{
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
		"request_protocol":               "http"}
	q0m2 := model.Metric{
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
		"destination_canonical_revision": "v2",
		"request_protocol":               "http"}
	q0m3 := model.Metric{
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
	q0m4 := model.Metric{
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  0.010},
		&model.Sample{
			Metric: q0m1,
			Value:  0.020},
		&model.Sample{
			Metric: q0m2,
			Value:  0.020},
		&model.Sample{
			Metric: q0m3,
			Value:  0.030}, // same edge reported by outgoing (q1), this > value should be preferred
		&model.Sample{
			Metric: q0m4,
			Value:  0.030}, // same edge reported by outgoing (q1), this > value should be preferred
	}

	q1 := `round(histogram_quantile(0.95, sum(rate(istio_request_duration_milliseconds_bucket{reporter="source",source_workload_namespace="bookinfo"}[60s])) by (le,source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol)) > 0,0.001)`
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
		"request_protocol":               "http"}
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
		"destination_canonical_revision": "v2",
		"request_protocol":               "http"}
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}

	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  0.020},
		&model.Sample{
			Metric: q1m1,
			Value:  0.020},
		&model.Sample{
			Metric: q1m2,
			Value:  0.040}, // same edge reported by incoming (q0), this > value should get ignored
		&model.Sample{
			Metric: q1m3,
			Value:  0.040}, // same edge reported by incoming (q0), this > value should get ignored
	}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)

	trafficMap := responseTimeTestTraffic()
	ingressID, _ := graph.Id(graph.Unknown, "istio-system", "", "istio-system", "ingressgateway-unknown", "ingressgateway", graph.Unknown, graph.GraphTypeVersionedApp)
	ingress, ok := trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	assert.Equal(nil, ingress.Edges[0].Metadata[graph.ResponseTime])

	duration, _ := time.ParseDuration("60s")
	appender := ResponseTimeAppender{
		GraphType:          graph.GraphTypeVersionedApp,
		InjectServiceNodes: true,
		Namespaces: map[string]graph.NamespaceInfo{
			"bookinfo": {
				Name:     "bookinfo",
				Duration: duration,
			},
		},
		Quantile:  0.95,
		QueryTime: time.Now().Unix(),
		Rates: graph.RequestedRates{
			Grpc: graph.RateRequests,
			Http: graph.RateRequests,
			Tcp:  graph.RateTotal,
		},
	}

	appender.appendGraph(trafficMap, "bookinfo", client)

	ingress, ok = trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	_, ok = ingress.Edges[0].Metadata[graph.ResponseTime]
	assert.Equal(false, ok)

	productpageService := ingress.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, productpageService.NodeType)
	assert.Equal("productpage", productpageService.Service)
	assert.Equal(nil, productpageService.Metadata[graph.ResponseTime])
	assert.Equal(1, len(productpageService.Edges))
	assert.Equal(0.01, productpageService.Edges[0].Metadata[graph.ResponseTime])

	productpage := productpageService.Edges[0].Dest
	assert.Equal("productpage", productpage.App)
	assert.Equal("v1", productpage.Version)
	assert.Equal(nil, productpage.Metadata[graph.ResponseTime])
	assert.Equal(1, len(productpage.Edges))
	_, ok = productpage.Edges[0].Metadata[graph.ResponseTime]
	assert.Equal(false, ok)

	reviewsService := productpage.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(nil, reviewsService.Metadata[graph.ResponseTime])
	assert.Equal(2, len(reviewsService.Edges))
	assert.Equal(0.02, reviewsService.Edges[0].Metadata[graph.ResponseTime])
	assert.Equal(0.02, reviewsService.Edges[1].Metadata[graph.ResponseTime])

	reviews1 := reviewsService.Edges[0].Dest
	assert.Equal("reviews", reviews1.App)
	assert.Equal("v1", reviews1.Version)
	assert.Equal(nil, reviews1.Metadata[graph.ResponseTime])
	assert.Equal(1, len(reviews1.Edges))
	_, ok = reviews1.Edges[0].Metadata[graph.ResponseTime]
	assert.Equal(false, ok)

	ratingsService := reviews1.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, ratingsService.NodeType)
	assert.Equal("ratings", ratingsService.Service)
	assert.Equal(nil, ratingsService.Metadata[graph.ResponseTime])
	assert.Equal(1, len(ratingsService.Edges))
	assert.Equal(0.03, ratingsService.Edges[0].Metadata[graph.ResponseTime])

	reviews2 := reviewsService.Edges[1].Dest
	assert.Equal("reviews", reviews2.App)
	assert.Equal("v2", reviews2.Version)
	assert.Equal(nil, reviews2.Metadata[graph.ResponseTime])
	assert.Equal(1, len(reviews2.Edges))
	_, ok = reviews2.Edges[0].Metadata[graph.ResponseTime]
	assert.False(ok)

	assert.Equal(ratingsService, reviews2.Edges[0].Dest)

	ratings := ratingsService.Edges[0].Dest
	assert.Equal("ratings", ratings.App)
	assert.Equal("v1", ratings.Version)
	assert.Equal(nil, ratings.Metadata[graph.ResponseTime])
	assert.Equal(0, len(ratings.Edges))
}

func TestResponseTimeAvgSkipRates(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_request_duration_milliseconds_sum{reporter="destination",destination_service_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol) / sum(rate(istio_request_duration_milliseconds_count{reporter="destination",destination_service_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol) > 0,0.001)`
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
	q0m1 := model.Metric{
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
		"request_protocol":               "http"}
	q0m2 := model.Metric{
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
		"destination_canonical_revision": "v2",
		"request_protocol":               "http"}
	q0m3 := model.Metric{
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
	q0m4 := model.Metric{
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  0.010},
		&model.Sample{
			Metric: q0m1,
			Value:  0.020},
		&model.Sample{
			Metric: q0m2,
			Value:  0.020},
		&model.Sample{
			Metric: q0m3,
			Value:  0.030}, // same edge reported by outgoing (q1), this > value should be preferred
		&model.Sample{
			Metric: q0m4,
			Value:  0.030}, // same edge reported by outgoing (q1), this > value should be preferred
	}

	q1 := `round(sum(rate(istio_request_duration_milliseconds_sum{reporter="source",source_workload_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol) / sum(rate(istio_request_duration_milliseconds_count{reporter="source",source_workload_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol) > 0,0.001)`
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
		"request_protocol":               "http"}
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
		"destination_canonical_revision": "v2",
		"request_protocol":               "http"}
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}

	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  0.020},
		&model.Sample{
			Metric: q1m1,
			Value:  0.020},
		&model.Sample{
			Metric: q1m2,
			Value:  0.040}, // same edge reported by incoming (q0), this > value should get ignored
		&model.Sample{
			Metric: q1m3,
			Value:  0.040}, // same edge reported by incoming (q0), this > value should get ignored
	}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)

	trafficMap := responseTimeTestTraffic()
	ingressID, _ := graph.Id(graph.Unknown, "istio-system", "", "istio-system", "ingressgateway-unknown", "ingressgateway", graph.Unknown, graph.GraphTypeVersionedApp)
	ingress, ok := trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	assert.Equal(nil, ingress.Edges[0].Metadata[graph.ResponseTime])

	duration, _ := time.ParseDuration("60s")
	appender := ResponseTimeAppender{
		GraphType:          graph.GraphTypeVersionedApp,
		InjectServiceNodes: true,
		Namespaces: map[string]graph.NamespaceInfo{
			"bookinfo": {
				Name:     "bookinfo",
				Duration: duration,
			},
		},
		Quantile:  0.0,
		QueryTime: time.Now().Unix(),
		Rates: graph.RequestedRates{
			Grpc: graph.RateRequests,
			Http: graph.RateNone,
			Tcp:  graph.RateTotal,
		},
	}

	appender.appendGraph(trafficMap, "bookinfo", client)

	ingress, ok = trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	_, ok = ingress.Edges[0].Metadata[graph.ResponseTime]
	assert.Equal(false, ok)

	productpageService := ingress.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, productpageService.NodeType)
	assert.Equal("productpage", productpageService.Service)
	assert.Equal(nil, productpageService.Metadata[graph.ResponseTime])
	assert.Equal(1, len(productpageService.Edges))
	assert.Equal(nil, productpageService.Edges[0].Metadata[graph.ResponseTime])

	productpage := productpageService.Edges[0].Dest
	assert.Equal("productpage", productpage.App)
	assert.Equal("v1", productpage.Version)
	assert.Equal(nil, productpage.Metadata[graph.ResponseTime])
	assert.Equal(1, len(productpage.Edges))
	_, ok = productpage.Edges[0].Metadata[graph.ResponseTime]
	assert.Equal(false, ok)

	reviewsService := productpage.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(nil, reviewsService.Metadata[graph.ResponseTime])
	assert.Equal(2, len(reviewsService.Edges))
	assert.Equal(nil, reviewsService.Edges[0].Metadata[graph.ResponseTime])
	assert.Equal(nil, reviewsService.Edges[1].Metadata[graph.ResponseTime])

	reviews1 := reviewsService.Edges[0].Dest
	assert.Equal("reviews", reviews1.App)
	assert.Equal("v1", reviews1.Version)
	assert.Equal(nil, reviews1.Metadata[graph.ResponseTime])
	assert.Equal(1, len(reviews1.Edges))
	_, ok = reviews1.Edges[0].Metadata[graph.ResponseTime]
	assert.Equal(false, ok)

	ratingsService := reviews1.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, ratingsService.NodeType)
	assert.Equal("ratings", ratingsService.Service)
	assert.Equal(nil, ratingsService.Metadata[graph.ResponseTime])
	assert.Equal(1, len(ratingsService.Edges))
	assert.Equal(nil, ratingsService.Edges[0].Metadata[graph.ResponseTime])

	reviews2 := reviewsService.Edges[1].Dest
	assert.Equal("reviews", reviews2.App)
	assert.Equal("v2", reviews2.Version)
	assert.Equal(nil, reviews2.Metadata[graph.ResponseTime])
	assert.Equal(1, len(reviews2.Edges))
	_, ok = reviews2.Edges[0].Metadata[graph.ResponseTime]
	assert.False(ok)

	assert.Equal(ratingsService, reviews2.Edges[0].Dest)

	ratings := ratingsService.Edges[0].Dest
	assert.Equal("ratings", ratings.App)
	assert.Equal("v1", ratings.Version)
	assert.Equal(nil, ratings.Metadata[graph.ResponseTime])
	assert.Equal(0, len(ratings.Edges))
}

func TestResponseTimeAvg(t *testing.T) {
	assert := assert.New(t)

	q0 := `round(sum(rate(istio_request_duration_milliseconds_sum{reporter="destination",destination_service_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol) / sum(rate(istio_request_duration_milliseconds_count{reporter="destination",destination_service_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol) > 0,0.001)`
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
	q0m1 := model.Metric{
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
		"request_protocol":               "http"}
	q0m2 := model.Metric{
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
		"destination_canonical_revision": "v2",
		"request_protocol":               "http"}
	q0m3 := model.Metric{
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
	q0m4 := model.Metric{
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  0.010},
		&model.Sample{
			Metric: q0m1,
			Value:  0.020},
		&model.Sample{
			Metric: q0m2,
			Value:  0.020},
		&model.Sample{
			Metric: q0m3,
			Value:  0.030}, // same edge reported by outgoing (q1), this > value should be preferred
		&model.Sample{
			Metric: q0m4,
			Value:  0.030}, // same edge reported by outgoing (q1), this > value should be preferred
	}

	q1 := `round(sum(rate(istio_request_duration_milliseconds_sum{reporter="source",source_workload_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol) / sum(rate(istio_request_duration_milliseconds_count{reporter="source",source_workload_namespace="bookinfo"}[60s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol) > 0,0.001)`
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
		"request_protocol":               "http"}
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
		"destination_canonical_revision": "v2",
		"request_protocol":               "http"}
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}
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
		"destination_canonical_revision": "v1",
		"request_protocol":               "http"}

	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  0.020},
		&model.Sample{
			Metric: q1m1,
			Value:  0.020},
		&model.Sample{
			Metric: q1m2,
			Value:  0.040}, // same edge reported by incoming (q0), this > value should get ignored
		&model.Sample{
			Metric: q1m3,
			Value:  0.040}, // same edge reported by incoming (q0), this > value should get ignored
	}

	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)

	trafficMap := responseTimeTestTraffic()
	ingressID, _ := graph.Id(graph.Unknown, "istio-system", "", "istio-system", "ingressgateway-unknown", "ingressgateway", graph.Unknown, graph.GraphTypeVersionedApp)
	ingress, ok := trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	assert.Equal(nil, ingress.Edges[0].Metadata[graph.ResponseTime])

	duration, _ := time.ParseDuration("60s")
	appender := ResponseTimeAppender{
		GraphType:          graph.GraphTypeVersionedApp,
		InjectServiceNodes: true,
		Namespaces: map[string]graph.NamespaceInfo{
			"bookinfo": {
				Name:     "bookinfo",
				Duration: duration,
			},
		},
		Quantile:  0.0,
		QueryTime: time.Now().Unix(),
		Rates: graph.RequestedRates{
			Grpc: graph.RateRequests,
			Http: graph.RateRequests,
			Tcp:  graph.RateTotal,
		},
	}

	appender.appendGraph(trafficMap, "bookinfo", client)

	ingress, ok = trafficMap[ingressID]
	assert.Equal(true, ok)
	assert.Equal("ingressgateway", ingress.App)
	assert.Equal(1, len(ingress.Edges))
	_, ok = ingress.Edges[0].Metadata[graph.ResponseTime]
	assert.Equal(false, ok)

	productpageService := ingress.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, productpageService.NodeType)
	assert.Equal("productpage", productpageService.Service)
	assert.Equal(nil, productpageService.Metadata[graph.ResponseTime])
	assert.Equal(1, len(productpageService.Edges))
	assert.Equal(0.01, productpageService.Edges[0].Metadata[graph.ResponseTime])

	productpage := productpageService.Edges[0].Dest
	assert.Equal("productpage", productpage.App)
	assert.Equal("v1", productpage.Version)
	assert.Equal(nil, productpage.Metadata[graph.ResponseTime])
	assert.Equal(1, len(productpage.Edges))
	_, ok = productpage.Edges[0].Metadata[graph.ResponseTime]
	assert.Equal(false, ok)

	reviewsService := productpage.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, reviewsService.NodeType)
	assert.Equal("reviews", reviewsService.Service)
	assert.Equal(nil, reviewsService.Metadata[graph.ResponseTime])
	assert.Equal(2, len(reviewsService.Edges))
	assert.Equal(0.02, reviewsService.Edges[0].Metadata[graph.ResponseTime])
	assert.Equal(0.02, reviewsService.Edges[1].Metadata[graph.ResponseTime])

	reviews1 := reviewsService.Edges[0].Dest
	assert.Equal("reviews", reviews1.App)
	assert.Equal("v1", reviews1.Version)
	assert.Equal(nil, reviews1.Metadata[graph.ResponseTime])
	assert.Equal(1, len(reviews1.Edges))
	_, ok = reviews1.Edges[0].Metadata[graph.ResponseTime]
	assert.Equal(false, ok)

	ratingsService := reviews1.Edges[0].Dest
	assert.Equal(graph.NodeTypeService, ratingsService.NodeType)
	assert.Equal("ratings", ratingsService.Service)
	assert.Equal(nil, ratingsService.Metadata[graph.ResponseTime])
	assert.Equal(1, len(ratingsService.Edges))
	assert.Equal(0.03, ratingsService.Edges[0].Metadata[graph.ResponseTime])

	reviews2 := reviewsService.Edges[1].Dest
	assert.Equal("reviews", reviews2.App)
	assert.Equal("v2", reviews2.Version)
	assert.Equal(nil, reviews2.Metadata[graph.ResponseTime])
	assert.Equal(1, len(reviews2.Edges))
	_, ok = reviews2.Edges[0].Metadata[graph.ResponseTime]
	assert.False(ok)

	assert.Equal(ratingsService, reviews2.Edges[0].Dest)

	ratings := ratingsService.Edges[0].Dest
	assert.Equal("ratings", ratings.App)
	assert.Equal("v1", ratings.Version)
	assert.Equal(nil, ratings.Metadata[graph.ResponseTime])
	assert.Equal(0, len(ratings.Edges))
}

func responseTimeTestTraffic() graph.TrafficMap {
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
