package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/gorilla/mux"
	osproject_v1 "github.com/openshift/api/project/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/kiali/kiali/business"
	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/kubernetes/kubetest"
	"github.com/kiali/kiali/prometheus"
	"github.com/kiali/kiali/prometheus/prometheustest"
)

// Setup mock

func setupMocked() (*prometheus.Client, *prometheustest.PromAPIMock, *kubetest.K8SClientMock, error) {
	conf := config.NewConfig()
	conf.KubernetesConfig.CacheEnabled = false
	config.Set(conf)

	k8s := new(kubetest.K8SClientMock)

	k8s.On("GetNamespaces", mock.AnythingOfType("string")).Return(
		&core_v1.NamespaceList{
			Items: []core_v1.Namespace{
				{
					ObjectMeta: meta_v1.ObjectMeta{
						Name: "bookinfo",
					},
				},
				{
					ObjectMeta: meta_v1.ObjectMeta{
						Name: "tutorial",
					},
				},
			},
		}, nil)

	fmt.Println("!!! Set up standard mock")
	k8s.On("GetProjects", mock.AnythingOfType("string")).Return(
		[]osproject_v1.Project{
			{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "bookinfo",
				},
			},
			{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "tutorial",
				},
			},
		}, nil)

	k8s.On("IsOpenShift").Return(true)

	api := new(prometheustest.PromAPIMock)
	client, err := prometheus.NewClient()
	if err != nil {
		return nil, nil, nil, err
	}
	client.Inject(api)

	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	business.SetWithBackends(mockClientFactory, nil)

	return client, api, k8s, nil
}

func setupMockedWithIstioComponentNamespaces() (*prometheus.Client, *prometheustest.PromAPIMock, *kubetest.K8SClientMock, error) {
	testConfig := config.NewConfig()
	testConfig.KubernetesConfig.CacheEnabled = false
	config.Set(testConfig)
	k8s := new(kubetest.K8SClientMock)

	fmt.Println("!!! Set up complex mock")
	k8s.On("GetNamespaces", mock.AnythingOfType("string")).Return(
		&core_v1.NamespaceList{
			Items: []core_v1.Namespace{
				{
					ObjectMeta: meta_v1.ObjectMeta{
						Name: "bookinfo",
					},
				},
				{
					ObjectMeta: meta_v1.ObjectMeta{
						Name: "tutorial",
					},
				},
				{
					ObjectMeta: meta_v1.ObjectMeta{
						Name: "istio-system",
					},
				},
				{
					ObjectMeta: meta_v1.ObjectMeta{
						Name: "istio-telemetry",
					},
				},
			},
		}, nil)

	k8s.On("GetProjects", mock.AnythingOfType("string")).Return(
		[]osproject_v1.Project{
			{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "bookinfo",
				},
			},
			{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "tutorial",
				},
			},
			{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "istio-system",
				},
			},
			{
				ObjectMeta: meta_v1.ObjectMeta{
					Name: "istio-telemetry",
				},
			},
		}, nil)

	k8s.On("IsOpenShift").Return(true)

	api := new(prometheustest.PromAPIMock)
	client, err := prometheus.NewClient()
	if err != nil {
		return nil, nil, nil, err
	}
	client.Inject(api)

	mockClientFactory := kubetest.NewK8SClientFactoryMock(k8s)
	business.SetWithBackends(mockClientFactory, nil)

	return client, api, k8s, nil
}

func mockQuery(api *prometheustest.PromAPIMock, query string, ret *model.Vector) {
	api.On(
		"Query",
		mock.AnythingOfType("*context.emptyCtx"),
		query,
		mock.AnythingOfType("time.Time"),
	).Return(*ret, nil)
	api.On(
		"Query",
		mock.AnythingOfType("*context.cancelCtx"),
		query,
		mock.AnythingOfType("time.Time"),
	).Return(*ret, nil)
}

// mockNamespaceGraph provides the same single-namespace mocks to be used for different graph types
func mockNamespaceGraph(t *testing.T) (*prometheus.Client, *prometheustest.PromAPIMock, error) {
	q0 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace!="bookinfo",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.bookinfo\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	v0 := model.Vector{}

	q1 := `round(sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      "latest",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m1 := model.Metric{
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m2 := model.Metric{
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "",
		"destination_service_name":       "",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "kiali-2412", // test case when there is no destination_service_name
		"destination_canonical_service":  "",
		"destination_canonical_revision": "",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m3 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m4 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v2",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m5 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v3",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v3",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m6 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "300",
		"response_flags":                 "-"}
	q1m7 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "400",
		"response_flags":                 "-"}
	q1m8 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q1m9 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m10 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m11 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v2",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v2",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings:9080",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m12 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v2",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v2",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings:9080",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q1m13 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v2",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v2",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v2",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m14 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings:9080",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m15 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings:9080",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q1m16 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v3",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v3",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  100},
		&model.Sample{
			Metric: q1m1,
			Value:  50},
		&model.Sample{
			Metric: q1m2,
			Value:  50},
		&model.Sample{
			Metric: q1m3,
			Value:  20},
		&model.Sample{
			Metric: q1m4,
			Value:  20},
		&model.Sample{
			Metric: q1m5,
			Value:  20},
		&model.Sample{
			Metric: q1m6,
			Value:  20},
		&model.Sample{
			Metric: q1m7,
			Value:  20},
		&model.Sample{
			Metric: q1m8,
			Value:  20},
		&model.Sample{
			Metric: q1m9,
			Value:  20},
		&model.Sample{
			Metric: q1m10,
			Value:  20},
		&model.Sample{
			Metric: q1m11,
			Value:  20},
		&model.Sample{
			Metric: q1m12,
			Value:  10},
		&model.Sample{
			Metric: q1m13,
			Value:  20},
		&model.Sample{
			Metric: q1m14,
			Value:  20},
		&model.Sample{
			Metric: q1m15,
			Value:  10},
		&model.Sample{
			Metric: q1m16,
			Value:  20}}

	q2 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q2m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m1 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v2",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m2 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v3",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v3",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m3 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "300",
		"response_flags":                 "-"}
	q2m4 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "400",
		"response_flags":                 "-"}
	q2m5 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q2m6 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m7 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m8 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v2",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v2",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings:9080",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m9 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v2",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v2",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings:9080",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q2m10 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v2",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v2",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v2",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m11 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings:9080",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m12 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "ratings:9080",
		"destination_service_name":       "ratings",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_canonical_service":  "ratings",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q2m13 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v3",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v3",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m14 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bankapp",
		"destination_service":            "pricing:9080",
		"destination_service_name":       "pricing",
		"destination_workload_namespace": "bankapp",
		"destination_workload":           "pricing-v1",
		"destination_canonical_service":  "pricing",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q2m15 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "unknown",
		"destination_service":            "unknown",
		"destination_service_name":       "unknown",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "404",
		"response_flags":                 "NR"}
	q2m16 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bankapp",
		"destination_service":            "deposit:9080",
		"destination_service_name":       "deposit",
		"destination_workload_namespace": "bankapp",
		"destination_workload":           "deposit-v1",
		"destination_canonical_service":  "deposit",
		"destination_canonical_revision": "v1",
		"request_protocol":               "grpc",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	v2 := model.Vector{
		&model.Sample{
			Metric: q2m0,
			Value:  20},
		&model.Sample{
			Metric: q2m1,
			Value:  20},
		&model.Sample{
			Metric: q2m2,
			Value:  20},
		&model.Sample{
			Metric: q2m3,
			Value:  20},
		&model.Sample{
			Metric: q2m4,
			Value:  20},
		&model.Sample{
			Metric: q2m5,
			Value:  20},
		&model.Sample{
			Metric: q2m6,
			Value:  20},
		&model.Sample{
			Metric: q2m7,
			Value:  20},
		&model.Sample{
			Metric: q2m8,
			Value:  20},
		&model.Sample{
			Metric: q2m9,
			Value:  10},
		&model.Sample{
			Metric: q2m10,
			Value:  20},
		&model.Sample{
			Metric: q2m11,
			Value:  20},
		&model.Sample{
			Metric: q2m12,
			Value:  10},
		&model.Sample{
			Metric: q2m13,
			Value:  20},
		&model.Sample{
			Metric: q2m14,
			Value:  20},
		&model.Sample{
			Metric: q2m15,
			Value:  4},
		&model.Sample{
			Metric: q2m16,
			Value:  50}}

	q3 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace!="bookinfo",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.bookinfo\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v3 := model.Vector{}

	q4 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q4m0 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      "latest",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	q4m1 := model.Metric{
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	q4m2 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v4 := model.Vector{
		&model.Sample{
			Metric: q4m0,
			Value:  150},
		&model.Sample{
			Metric: q4m1,
			Value:  400},
		&model.Sample{
			Metric: q4m2,
			Value:  31}}

	q5 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q5m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v5 := model.Vector{
		&model.Sample{
			Metric: q5m0,
			Value:  31}}

	client, api, _, err := setupMocked()
	if err != nil {
		return client, api, err
	}

	mockQuery(api, q0, &v0)
	mockQuery(api, q1, &v1)
	mockQuery(api, q2, &v2)
	mockQuery(api, q3, &v3)
	mockQuery(api, q4, &v4)
	mockQuery(api, q5, &v5)

	return client, api, nil
}

// mockNamespaceRatesGraph adds additional queries to mockNamespaceGraph to test non-default rates for graph-gen. Basic approach
// is for "sent" to use the same traffic/rates as is done for the default traffic.  This produces the same rates (and nearly the
// same graph as for the defaults). Use double the rates for "received".  And so "total" should be triple the "sent" rates.
func mockNamespaceRatesGraph(t *testing.T) (*prometheus.Client, *prometheustest.PromAPIMock, error) {
	client, api, err := mockNamespaceGraph(t)
	if err != nil {
		return client, api, err
	}

	q6 := `round(sum(rate(istio_tcp_received_bytes_total{reporter="source",source_workload_namespace!="bookinfo",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.bookinfo\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v6 := model.Vector{}

	q7 := `round(sum(rate(istio_tcp_received_bytes_total{reporter="destination",destination_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q7m0 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      "latest",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	q7m1 := model.Metric{
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	q7m2 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v7 := model.Vector{
		&model.Sample{
			Metric: q7m0,
			Value:  300},
		&model.Sample{
			Metric: q7m1,
			Value:  800},
		&model.Sample{
			Metric: q7m2,
			Value:  62}}

	q8 := `round(sum(rate(istio_tcp_received_bytes_total{reporter="source",source_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q8m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v8 := model.Vector{
		&model.Sample{
			Metric: q8m0,
			Value:  62}}

	q9 := `round(sum(rate(istio_request_messages_total{reporter="source",source_workload_namespace!="bookinfo",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.bookinfo\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	v9 := model.Vector{}

	q10 := `round(sum(rate(istio_request_messages_total{reporter="destination",destination_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	v10 := model.Vector{}

	q11 := `round(sum(rate(istio_request_messages_total{reporter="source",source_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	q11m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bankapp",
		"destination_service":            "deposit:9080",
		"destination_service_name":       "deposit",
		"destination_workload_namespace": "bankapp",
		"destination_workload":           "deposit-v1",
		"destination_canonical_service":  "deposit",
		"destination_canonical_revision": "v1"}
	v11 := model.Vector{
		&model.Sample{
			Metric: q11m0,
			Value:  50}}

	q12 := `round(sum(rate(istio_response_messages_total{reporter="source",source_workload_namespace!="bookinfo",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.bookinfo\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	v12 := model.Vector{}

	q13 := `round(sum(rate(istio_response_messages_total{reporter="destination",destination_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	v13 := model.Vector{}

	q14 := `round(sum(rate(istio_response_messages_total{reporter="source",source_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	q14m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "reviews-v3",
		"source_canonical_service":       "reviews",
		"source_canonical_revision":      "v3",
		"destination_service_namespace":  "bankapp",
		"destination_service":            "deposit:9080",
		"destination_service_name":       "deposit",
		"destination_workload_namespace": "bankapp",
		"destination_workload":           "deposit-v1",
		"destination_canonical_service":  "deposit",
		"destination_canonical_revision": "v1"}
	v14 := model.Vector{
		&model.Sample{
			Metric: q14m0,
			Value:  100}}

	mockQuery(api, q6, &v6)
	mockQuery(api, q7, &v7)
	mockQuery(api, q8, &v8)
	mockQuery(api, q9, &v9)
	mockQuery(api, q10, &v10)
	mockQuery(api, q11, &v11)
	mockQuery(api, q12, &v12)
	mockQuery(api, q13, &v13)
	mockQuery(api, q14, &v14)

	return client, api, nil
}

func respond(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		response = []byte(err.Error())
		code = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

func TestAppGraph(t *testing.T) {
	client, _, err := mockNamespaceGraph(t)
	if err != nil {
		t.Error(err)
		return
	}

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?namespaces=bookinfo&graphType=app&appenders&queryTime=1523364075"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_app_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestVersionedAppGraph(t *testing.T) {
	client, _, err := mockNamespaceGraph(t)
	if err != nil {
		t.Error(err)
		return
	}

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?namespaces=bookinfo&graphType=versionedApp&appenders&queryTime=1523364075"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_versioned_app_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestServiceGraph(t *testing.T) {
	client, _, err := mockNamespaceGraph(t)
	if err != nil {
		t.Error(err)
		return
	}

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?namespaces=bookinfo&graphType=service&appenders&queryTime=1523364075"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_service_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestWorkloadGraph(t *testing.T) {
	client, _, err := mockNamespaceGraph(t)
	if err != nil {
		t.Error(err)
		return
	}

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?namespaces=bookinfo&graphType=workload&appenders&queryTime=1523364075"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_workload_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestRatesGraphSent(t *testing.T) {
	client, _, err := mockNamespaceRatesGraph(t)
	if err != nil {
		t.Error(err)
		return
	}

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?namespaces=bookinfo&graphType=workload&appenders&queryTime=1523364075&rateGrpc=sent&rateHttp=requests&rateTcp=sent"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_rates_sent_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestRatesGraphReceived(t *testing.T) {
	client, _, err := mockNamespaceRatesGraph(t)
	if err != nil {
		t.Error(err)
		return
	}

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?namespaces=bookinfo&graphType=workload&appenders&queryTime=1523364075&rateGrpc=received&rateHttp=requests&rateTcp=received"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_rates_received_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestRatesGraphTotal(t *testing.T) {
	client, _, err := mockNamespaceRatesGraph(t)
	if err != nil {
		t.Error(err)
		return
	}

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?namespaces=bookinfo&graphType=workload&appenders&queryTime=1523364075&rateGrpc=total&rateHttp=requests&rateTcp=total"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_rates_total_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestRatesGraphNone(t *testing.T) {
	client, _, err := mockNamespaceRatesGraph(t)
	if err != nil {
		t.Error(err)
		return
	}

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?namespaces=bookinfo&graphType=workload&appenders&queryTime=1523364075&rateGrpc=total&rateHttp=none&rateTcp=total"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_rates_none_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestWorkloadNodeGraph(t *testing.T) {
	q0 := `round(sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="bookinfo",destination_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q0m1 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      "latest",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  50},
		&model.Sample{
			Metric: q0m1,
			Value:  100}}

	q1 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m1 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v2",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m2 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v3",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v3",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m3 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "300",
		"response_flags":                 "-"}
	q1m4 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "400",
		"response_flags":                 "-"}
	q1m5 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q1m6 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m7 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m8 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "unknown",
		"destination_service":            "unknown",
		"destination_service_name":       "unknown",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "404",
		"response_flags":                 "NR"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  20},
		&model.Sample{
			Metric: q1m1,
			Value:  20},
		&model.Sample{
			Metric: q1m2,
			Value:  20},
		&model.Sample{
			Metric: q1m3,
			Value:  20},
		&model.Sample{
			Metric: q1m4,
			Value:  20},
		&model.Sample{
			Metric: q1m5,
			Value:  20},
		&model.Sample{
			Metric: q1m6,
			Value:  20},
		&model.Sample{
			Metric: q1m7,
			Value:  20},
		&model.Sample{
			Metric: q1m8,
			Value:  4}}

	q2 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_workload_namespace="bookinfo",destination_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v2 := model.Vector{}

	q3 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace="bookinfo",source_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q3m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v3 := model.Vector{
		&model.Sample{
			Metric: q3m0,
			Value:  31}}

	client, xapi, _, err := setupMocked()
	if err != nil {
		return
	}

	mockQuery(xapi, q0, &v0)
	mockQuery(xapi, q1, &v1)
	mockQuery(xapi, q2, &v2)
	mockQuery(xapi, q3, &v3)

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/{namespace}/workloads/{workload}/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNodeIstio
	url := ts.URL + "/api/namespaces/bookinfo/workloads/productpage-v1/graph?graphType=workload&appenders&queryTime=1523364075"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_workload_node_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestAppNodeGraph(t *testing.T) {
	q0 := `round(sum(rate(istio_requests_total{reporter="destination",destination_service_namespace="bookinfo",destination_canonical_service="productpage"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q0m1 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      "latest",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  50},
		&model.Sample{
			Metric: q0m1,
			Value:  100}}

	q1 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m1 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v2",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m2 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v3",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v3",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m3 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "300",
		"response_flags":                 "-"}
	q1m4 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "400",
		"response_flags":                 "-"}
	q1m5 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q1m6 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m7 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m8 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "unknown",
		"destination_service":            "unknown",
		"destination_service_name":       "unknown",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "404",
		"response_flags":                 "NR"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  20},
		&model.Sample{
			Metric: q1m1,
			Value:  20},
		&model.Sample{
			Metric: q1m2,
			Value:  20},
		&model.Sample{
			Metric: q1m3,
			Value:  20},
		&model.Sample{
			Metric: q1m4,
			Value:  20},
		&model.Sample{
			Metric: q1m5,
			Value:  20},
		&model.Sample{
			Metric: q1m6,
			Value:  20},
		&model.Sample{
			Metric: q1m7,
			Value:  20},
		&model.Sample{
			Metric: q1m8,
			Value:  4}}

	q2 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_service_namespace="bookinfo",destination_canonical_service="productpage"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v2 := model.Vector{}

	q3 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q3m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v3 := model.Vector{
		&model.Sample{
			Metric: q3m0,
			Value:  31}}

	client, xapi, _, err := setupMocked()
	if err != nil {
		return
	}

	mockQuery(xapi, q0, &v0)
	mockQuery(xapi, q1, &v1)
	mockQuery(xapi, q2, &v2)
	mockQuery(xapi, q3, &v3)

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/{namespace}/applications/{app}/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNodeIstio
	url := ts.URL + "/api/namespaces/bookinfo/applications/productpage/graph?graphType=versionedApp&appenders&queryTime=1523364075"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_app_node_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestVersionedAppNodeGraph(t *testing.T) {
	q0 := `round(sum(rate(istio_requests_total{reporter="destination",destination_service_namespace="bookinfo",destination_canonical_service="productpage",destination_canonical_revision="v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q0m1 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      "latest",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  50},
		&model.Sample{
			Metric: q0m1,
			Value:  100}}

	q1 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage",source_canonical_revision="v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m1 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v2",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m2 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v3",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v3",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m3 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "300",
		"response_flags":                 "-"}
	q1m4 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "400",
		"response_flags":                 "-"}
	q1m5 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q1m6 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m7 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m8 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "unknown",
		"destination_service":            "unknown",
		"destination_service_name":       "unknown",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "404",
		"response_flags":                 "NR"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  20},
		&model.Sample{
			Metric: q1m1,
			Value:  20},
		&model.Sample{
			Metric: q1m2,
			Value:  20},
		&model.Sample{
			Metric: q1m3,
			Value:  20},
		&model.Sample{
			Metric: q1m4,
			Value:  20},
		&model.Sample{
			Metric: q1m5,
			Value:  20},
		&model.Sample{
			Metric: q1m6,
			Value:  20},
		&model.Sample{
			Metric: q1m7,
			Value:  20},
		&model.Sample{
			Metric: q1m8,
			Value:  4}}

	q2 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_service_namespace="bookinfo",destination_canonical_service="productpage",destination_canonical_revision="v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v2 := model.Vector{}

	q3 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage",source_canonical_revision="v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q3m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v3 := model.Vector{
		&model.Sample{
			Metric: q3m0,
			Value:  31}}

	client, xapi, _, err := setupMocked()
	if err != nil {
		return
	}

	mockQuery(xapi, q0, &v0)
	mockQuery(xapi, q1, &v1)
	mockQuery(xapi, q2, &v2)
	mockQuery(xapi, q3, &v3)

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/{namespace}/applications/{app}/versions/{version}/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNodeIstio
	url := ts.URL + "/api/namespaces/bookinfo/applications/productpage/versions/v1/graph?graphType=versionedApp&appenders&queryTime=1523364075"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_versioned_app_node_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestServiceNodeGraph(t *testing.T) {
	q0 := `round(sum(rate(istio_requests_total{reporter="source",destination_workload="unknown",destination_service=~"^productpage\\.bookinfo\\..*$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	v0 := model.Vector{}

	q1 := `round(sum(rate(istio_requests_total{reporter="destination",destination_service_namespace="bookinfo",destination_service=~"^productpage\\.bookinfo\\..*$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      "latest",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  100}}

	q2 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_service_namespace="bookinfo",destination_service=~"^productpage\\.bookinfo\\..*$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q2m0 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      "latest",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v2 := model.Vector{
		&model.Sample{
			Metric: q2m0,
			Value:  31}}

	client, xapi, _, err := setupMocked()
	if err != nil {
		return
	}

	mockQuery(xapi, q0, &v0)
	mockQuery(xapi, q1, &v1)
	mockQuery(xapi, q2, &v2)

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/{namespace}/services/{service}/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNodeIstio
	url := ts.URL + "/api/namespaces/bookinfo/services/productpage/graph?graphType=workload&appenders&queryTime=1523364075"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_service_node_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestRatesNodeGraphTotal(t *testing.T) {
	q0 := `round(sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="bookinfo",destination_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q0m0 := model.Metric{
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q0m1 := model.Metric{
		"source_workload_namespace":      "istio-system",
		"source_workload":                "ingressgateway-unknown",
		"source_canonical_service":       "ingressgateway",
		"source_canonical_revision":      "latest",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  50},
		&model.Sample{
			Metric: q0m1,
			Value:  100}}

	q1 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q1m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m1 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v2",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m2 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews:9080",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v3",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v3",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m3 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "300",
		"response_flags":                 "-"}
	q1m4 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "400",
		"response_flags":                 "-"}
	q1m5 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "500",
		"response_flags":                 "-"}
	q1m6 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "details:9080",
		"destination_service_name":       "details",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "details-v1",
		"destination_canonical_service":  "details",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m7 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m8 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "unknown",
		"destination_service":            "unknown",
		"destination_service_name":       "unknown",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "404",
		"response_flags":                 "NR"}
	q1m9 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bankapp",
		"destination_service":            "deposit:9080",
		"destination_service_name":       "deposit",
		"destination_workload_namespace": "bankapp",
		"destination_workload":           "deposit-v1",
		"destination_canonical_service":  "deposit",
		"destination_canonical_revision": "v1",
		"request_protocol":               "grpc",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  20},
		&model.Sample{
			Metric: q1m1,
			Value:  20},
		&model.Sample{
			Metric: q1m2,
			Value:  20},
		&model.Sample{
			Metric: q1m3,
			Value:  20},
		&model.Sample{
			Metric: q1m4,
			Value:  20},
		&model.Sample{
			Metric: q1m5,
			Value:  20},
		&model.Sample{
			Metric: q1m6,
			Value:  20},
		&model.Sample{
			Metric: q1m7,
			Value:  20},
		&model.Sample{
			Metric: q1m8,
			Value:  4},
		&model.Sample{
			Metric: q1m9,
			Value:  50}}

	q2 := `round(sum(rate(istio_request_messages_total{reporter="destination",destination_workload_namespace="bookinfo",destination_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	v2 := model.Vector{}

	q3 := `round(sum(rate(istio_request_messages_total{reporter="source",source_workload_namespace="bookinfo",source_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	q3m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v3 := model.Vector{
		&model.Sample{
			Metric: q3m0,
			Value:  31}}

	q4 := `round(sum(rate(istio_response_messages_total{reporter="destination",destination_workload_namespace="bookinfo",destination_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	v4 := model.Vector{}

	q5 := `round(sum(rate(istio_response_messages_total{reporter="source",source_workload_namespace="bookinfo",source_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
	q5m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v5 := model.Vector{
		&model.Sample{
			Metric: q5m0,
			Value:  62}}

	q6 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_workload_namespace="bookinfo",destination_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v6 := model.Vector{}

	q7 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace="bookinfo",source_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q7m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v7 := model.Vector{
		&model.Sample{
			Metric: q7m0,
			Value:  31}}

	q8 := `round(sum(rate(istio_tcp_received_bytes_total{reporter="destination",destination_workload_namespace="bookinfo",destination_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v8 := model.Vector{}

	q9 := `round(sum(rate(istio_tcp_received_bytes_total{reporter="source",source_workload_namespace="bookinfo",source_workload="productpage-v1"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	q9m0 := model.Metric{
		"source_workload_namespace":      "bookinfo",
		"source_workload":                "productpage-v1",
		"source_canonical_service":       "productpage",
		"source_canonical_revision":      "v1",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "tcp:9080",
		"destination_service_name":       "tcp",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "tcp-v1",
		"destination_canonical_service":  "tcp",
		"destination_canonical_revision": "v1",
		"response_flags":                 "-"}
	v9 := model.Vector{
		&model.Sample{
			Metric: q9m0,
			Value:  62}}

	client, xapi, _, err := setupMocked()
	if err != nil {
		return
	}

	mockQuery(xapi, q0, &v0)
	mockQuery(xapi, q1, &v1)
	mockQuery(xapi, q2, &v2)
	mockQuery(xapi, q3, &v3)
	mockQuery(xapi, q4, &v4)
	mockQuery(xapi, q5, &v5)
	mockQuery(xapi, q6, &v6)
	mockQuery(xapi, q7, &v7)
	mockQuery(xapi, q8, &v8)
	mockQuery(xapi, q9, &v9)

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/{namespace}/workloads/{workload}/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNodeIstio
	url := ts.URL + "/api/namespaces/bookinfo/workloads/productpage-v1/graph?rateGrpc=total&rateHttp=requests&rateTcp=total&graphType=workload&appenders&queryTime=1523364075"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_rates_node_graph_total.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)

	/*
		q0 := `round(sum(rate(istio_requests_total{reporter="source",destination_workload="unknown",destination_service=~"^productpage\\.bookinfo\\..*$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
		q0m0 := model.Metric{
			"source_workload_namespace":      "bookinfo",
			"source_workload":                "productpage-v1",
			"source_canonical_service":       "productpage",
			"source_canonical_revision":      "v1",
			"destination_service_namespace":  "bankapp",
			"destination_service":            "deposit:9080",
			"destination_service_name":       "deposit",
			"destination_workload_namespace": "bankapp",
			"destination_workload":           "deposit-v1",
			"destination_canonical_service":  "deposit",
			"destination_canonical_revision": "v1",
			"request_protocol":               "grpc",
			"response_code":                  "200",
			"grpc_response_status":           "0",
			"response_flags":                 "-"}
		v0 := model.Vector{&model.Sample{
			Metric: q0m0,
			Value:  100}}

		q1 := `round(sum(rate(istio_requests_total{reporter="destination",destination_service_namespace="bookinfo",destination_service=~"^productpage\\.bookinfo\\..*$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
		q1m0 := model.Metric{
			"source_workload_namespace":      "istio-system",
			"source_workload":                "ingressgateway-unknown",
			"source_canonical_service":       "ingressgateway",
			"source_canonical_revision":      "latest",
			"destination_service_namespace":  "bookinfo",
			"destination_service":            "productpage:9080",
			"destination_service_name":       "productpage",
			"destination_workload_namespace": "bookinfo",
			"destination_workload":           "productpage-v1",
			"destination_canonical_service":  "productpage",
			"destination_canonical_revision": "v1",
			"request_protocol":               "http",
			"response_code":                  "200",
			"grpc_response_status":           "0",
			"response_flags":                 "-"}

		v1 := model.Vector{
			&model.Sample{
				Metric: q1m0,
				Value:  100}}

		q2 := `round(sum(rate(istio_request_messages_total{reporter="destination",destination_service_namespace="bookinfo",destination_service=~"^productpage\\.bookinfo\\..*$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
		q2m0 := model.Metric{
			"source_workload_namespace":      "istio-system",
			"source_workload":                "ingressgateway-unknown",
			"source_canonical_service":       "ingressgateway",
			"source_canonical_revision":      "latest",
			"destination_service_namespace":  "bookinfo",
			"destination_service":            "productpage:9080",
			"destination_service_name":       "productpage",
			"destination_workload_namespace": "bookinfo",
			"destination_workload":           "productpage-v1",
			"destination_canonical_service":  "productpage",
			"destination_canonical_revision": "v1"}
		v2 := model.Vector{
			&model.Sample{
				Metric: q2m0,
				Value:  31}}

		q3 := `round(sum(rate(istio_request_messages_total{reporter="source",destination_service_namespace="bookinfo",destination_service=~"^productpage\\.bookinfo\\..*$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision) > 0,0.001)`
		q3m0 := model.Metric{
			"source_workload_namespace":      "istio-system",
			"source_workload":                "ingressgateway-unknown",
			"source_canonical_service":       "ingressgateway",
			"source_canonical_revision":      "latest",
			"destination_service_namespace":  "bookinfo",
			"destination_service":            "productpage:9080",
			"destination_service_name":       "productpage",
			"destination_workload_namespace": "bookinfo",
			"destination_workload":           "productpage-v1",
			"destination_canonical_service":  "productpage",
			"destination_canonical_revision": "v1",
			"response_flags":                 "-"}
		v3 := model.Vector{
			&model.Sample{
				Metric: q3m0,
				Value:  62}}

		q2 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",destination_service_namespace="bookinfo",destination_service=~"^productpage\\.bookinfo\\..*$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
		q2m0 := model.Metric{
			"source_workload_namespace":      "istio-system",
			"source_workload":                "ingressgateway-unknown",
			"source_canonical_service":       "ingressgateway",
			"source_canonical_revision":      "latest",
			"destination_service_namespace":  "bookinfo",
			"destination_service":            "productpage:9080",
			"destination_service_name":       "productpage",
			"destination_workload_namespace": "bookinfo",
			"destination_workload":           "productpage-v1",
			"destination_canonical_service":  "productpage",
			"destination_canonical_revision": "v1",
			"response_flags":                 "-"}
		v2 := model.Vector{
			&model.Sample{
				Metric: q2m0,
				Value:  31}}

		q3 := `round(sum(rate(istio_tcp_received_bytes_total{reporter="source",destination_service_namespace="bookinfo",destination_service=~"^productpage\\.bookinfo\\..*$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
		q3m0 := model.Metric{
			"source_workload_namespace":      "istio-system",
			"source_workload":                "ingressgateway-unknown",
			"source_canonical_service":       "ingressgateway",
			"source_canonical_revision":      "latest",
			"destination_service_namespace":  "bookinfo",
			"destination_service":            "productpage:9080",
			"destination_service_name":       "productpage",
			"destination_workload_namespace": "bookinfo",
			"destination_workload":           "productpage-v1",
			"destination_canonical_service":  "productpage",
			"destination_canonical_revision": "v1",
			"response_flags":                 "-"}
		v3 := model.Vector{
			&model.Sample{
				Metric: q3m0,
				Value:  62}}

		client, xapi, _, err := setupMocked()
		if err != nil {
			return
		}

		mockQuery(xapi, q0, &v0)
		mockQuery(xapi, q1, &v1)
		mockQuery(xapi, q2, &v2)
		mockQuery(xapi, q3, &v3)

		var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

		mr := mux.NewRouter()
		mr.HandleFunc("/api/namespaces/{namespace}/services/{service}/graph", http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
				code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
				respond(w, code, config)
			}))

		ts := httptest.NewServer(mr)
		defer ts.Close()

		fut = graphNodeIstio
		url := ts.URL + "/api/namespaces/bookinfo/services/productpage/graph?graphType=workload&appenders&queryTime=1523364075"
		resp, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		actual, _ := ioutil.ReadAll(resp.Body)
		expected, _ := ioutil.ReadFile("testdata/test_service_node_graph.expected")
		if runtime.GOOS == "windows" {
			expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
		}
		expected = expected[:len(expected)-1] // remove EOF byte

		if !assert.Equal(t, expected, actual) {
			fmt.Printf("\nActual:\n%v", string(actual))
		}
		assert.Equal(t, 200, resp.StatusCode)
	*/
}

// TestComplexGraph aims to provide test coverage for a more robust graph and specific corner cases. Listed below are coverage cases
// - multi-cluster graph
// - multi-namespace graph
// - istio namespace
// - a "shared" node (internal in ns-1, outsider in ns-2)
// - request.host
// - bad dest telemetry filtering
// - bad source telemetry filtering
// - workload -> egress -> service-entry traffic
// - 0 response code (no response)
// note: appenders still tested in separate unit tests given that they create their own new business/kube clients
func TestComplexGraph(t *testing.T) {
	// bookinfo
	q0 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace!="bookinfo",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.bookinfo\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q0m0 := model.Metric{ // outsider request that fails to reach workload
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "outsider",
		"source_workload":                "outsider-ingress",
		"source_canonical_service":       "outsider-ingress",
		"source_canonical_revision":      "latest",
		"destination_cluster":            "unknown", // this reflects real ingress reporting at this time,
		"destination_service_namespace":  "unknown", // although it seems like a possible bug to me
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "latest",
		"request_protocol":               "http",
		"response_code":                  "503",
		"response_flags":                 "-"}
	v0 := model.Vector{
		&model.Sample{
			Metric: q0m0,
			Value:  50},
	}

	q1 := `round(sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q1m0 := model.Metric{
		"source_cluster":                 "unknown",
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_cluster":            "cluster-bookinfo",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m1 := model.Metric{
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "cluster-bookinfo",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q1m2 := model.Metric{ // bad dest telem (variant 1.0)
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "cluster-bookinfo",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "10.20.30.40:9080",
		"destination_service_name":       "10.20.30.40:9080",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	q1m3 := model.Metric{ // bad dest telem (variant 1.2)
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "cluster-bookinfo",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "10.20.30.40",
		"destination_service_name":       "10.20.30.40",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	q1m4 := model.Metric{ // good telem (mock service entry)
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "cluster-bookinfo",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "app.example.com",
		"destination_service_name":       "app.example.com",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	// TODO: This is bad telemetry that should normally be filtered.  But due to https://github.com/istio/istio/issues/29373
	// we are currently not filtering but rather setting dest_cluster = source_cluster.  When 29373 is fixed for all
	// supported versions and we remove the workaround, results of the test will change.
	q1m5 := model.Metric{ // bad dest telem (variant 2.1)
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  50},
		&model.Sample{
			Metric: q1m1,
			Value:  50},
		&model.Sample{
			Metric: q1m2,
			Value:  100},
		&model.Sample{
			Metric: q1m3,
			Value:  200},
		&model.Sample{
			Metric: q1m4,
			Value:  300},
		// see above
		&model.Sample{
			Metric: q1m5,
			Value:  700},
	}

	q2 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	v2 := model.Vector{}

	q3 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace!="bookinfo",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.bookinfo\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v3 := model.Vector{}

	q4 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v4 := model.Vector{}

	q5 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v5 := model.Vector{}

	// tutorial
	q6 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace!="tutorial",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.tutorial\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	v6 := model.Vector{}

	q7 := `round(sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="tutorial"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q7m0 := model.Metric{
		"source_cluster":                 "unknown",
		"source_workload_namespace":      "unknown",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_cluster":            "cluster-tutorial",
		"destination_service_namespace":  "tutorial",
		"destination_service":            "customer:9080",
		"destination_service_name":       "customer",
		"destination_workload_namespace": "tutorial",
		"destination_workload":           "customer-v1",
		"destination_canonical_service":  "customer",
		"destination_canonical_revision": "v1",
		"request_protocol":               "grpc",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q7m1 := model.Metric{
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "bad-source-telemetry-case-1",
		"source_workload":                "unknown",
		"source_canonical_service":       "unknown",
		"source_canonical_revision":      "unknown",
		"destination_cluster":            "cluster-tutorial",
		"destination_service_namespace":  "tutorial",
		"destination_service":            "customer:9080",
		"destination_service_name":       "customer",
		"destination_workload_namespace": "tutorial",
		"destination_workload":           "customer-v1",
		"destination_canonical_service":  "customer",
		"destination_canonical_revision": "v1",
		"request_protocol":               "grpc",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	v7 := model.Vector{
		&model.Sample{
			Metric: q7m0,
			Value:  50},
		&model.Sample{
			Metric: q7m1,
			Value:  50},
	}

	q8 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace="tutorial"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q8m0 := model.Metric{
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "cluster-bookinfo",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "productpage:9080",
		"destination_service_name":       "productpage",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_canonical_service":  "productpage",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"grpc_response_status":           "0",
		"response_flags":                 "-"}
	q8m1 := model.Metric{ // bad dest telem (variant 1.0)
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "cluster-bookinfo",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "10.20.30.40:9080",
		"destination_service_name":       "10.20.30.40:9080",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	q8m2 := model.Metric{ // bad dest telem (variant 1.2)
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "cluster-bookinfo",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "10.20.30.40",
		"destination_service_name":       "10.20.30.40",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	q8m3 := model.Metric{ // good telem (mock service entry)
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "cluster-bookinfo",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "app.example.com",
		"destination_service_name":       "app.example.com",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	q8m4 := model.Metric{ // good telem (service entry via egressgateway, see the second hop below)
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "unknown",
		"destination_service_namespace":  "unknown",
		"destination_service":            "istio-egressgateway.istio-system.svc.cluster.local",
		"destination_service_name":       "istio-egressgateway.istio-system.svc.cluster.local",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	q8m5 := model.Metric{ // no response http
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "unknown",
		"destination_service_namespace":  "unknown",
		"destination_service":            "istio-egressgateway.istio-system.svc.cluster.local",
		"destination_service_name":       "istio-egressgateway.istio-system.svc.cluster.local",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "0",
		"response_flags":                 "DC"}
	q8m6 := model.Metric{ // no response grpc
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "unknown",
		"destination_service_namespace":  "unknown",
		"destination_service":            "istio-egressgateway.istio-system.svc.cluster.local",
		"destination_service_name":       "istio-egressgateway.istio-system.svc.cluster.local",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "grpc",
		"response_code":                  "0", // note, grpc_response_status is not reported for grpc with no response
		"response_flags":                 "DC"}
	// TODO: This is bad telemetry that should normally be filtered.  But due to https://github.com/istio/istio/issues/29373
	// we are currently not filtering but rather setting dest_cluster = source_cluster.  When 29373 is fixed for all
	// supported versions and we remove the workaround, results of the test will change.
	q8m7 := model.Metric{ // bad dest telem (variant 2.1)
		"source_cluster":                 "cluster-tutorial",
		"source_workload_namespace":      "tutorial",
		"source_workload":                "customer-v1",
		"source_canonical_service":       "customer",
		"source_canonical_revision":      "v1",
		"destination_cluster":            "unknown",
		"destination_service_namespace":  "bookinfo",
		"destination_service":            "reviews",
		"destination_service_name":       "reviews",
		"destination_workload_namespace": "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_canonical_service":  "reviews",
		"destination_canonical_revision": "v1",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	v8 := model.Vector{
		&model.Sample{
			Metric: q8m0,
			Value:  50},
		&model.Sample{
			Metric: q8m1,
			Value:  100},
		&model.Sample{
			Metric: q8m2,
			Value:  200},
		&model.Sample{
			Metric: q8m3,
			Value:  300},
		&model.Sample{
			Metric: q8m4,
			Value:  400},
		&model.Sample{
			Metric: q8m5,
			Value:  500},
		&model.Sample{
			Metric: q8m6,
			Value:  600},
		// see above
		&model.Sample{
			Metric: q8m7,
			Value:  700},
	}

	q9 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace!="tutorial",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.tutorial\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v9 := model.Vector{}

	q10 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_workload_namespace="tutorial"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v10 := model.Vector{}

	q11 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace="tutorial"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v11 := model.Vector{}

	// istio-system
	q12 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace!="istio-system",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.istio-system\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	v12 := model.Vector{}

	q13 := `round(sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="istio-system"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	v13 := model.Vector{}

	q14 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace="istio-system"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) > 0,0.001)`
	q14m0 := model.Metric{ // good telem (service entry via egressgateway, the second hop)
		"source_cluster":                 "cluster-cp",
		"source_workload_namespace":      "istio-system",
		"source_workload":                "istio-egressgateway",
		"source_canonical_service":       "istio-egressgateway",
		"source_canonical_revision":      "latest",
		"destination_cluster":            "unknown",
		"destination_service_namespace":  "unknown",
		"destination_service":            "app.example-2.com",
		"destination_service_name":       "app.example-2.com",
		"destination_workload_namespace": "unknown",
		"destination_workload":           "unknown",
		"destination_canonical_service":  "unknown",
		"destination_canonical_revision": "unknown",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-"}
	v14 := model.Vector{
		&model.Sample{
			Metric: q14m0,
			Value:  400}}

	q15 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace!="istio-system",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.istio-system\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v15 := model.Vector{}

	q16 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_workload_namespace="istio-system"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v16 := model.Vector{}

	q17 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace="istio-system"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) > 0,0.001)`
	v17 := model.Vector{}

	client, xapi, _, err := setupMockedWithIstioComponentNamespaces()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(xapi, q0, &v0)
	mockQuery(xapi, q1, &v1)
	mockQuery(xapi, q2, &v2)
	mockQuery(xapi, q3, &v3)
	mockQuery(xapi, q4, &v4)
	mockQuery(xapi, q5, &v5)
	mockQuery(xapi, q6, &v6)
	mockQuery(xapi, q7, &v7)
	mockQuery(xapi, q8, &v8)
	mockQuery(xapi, q9, &v9)
	mockQuery(xapi, q10, &v10)
	mockQuery(xapi, q11, &v11)
	mockQuery(xapi, q12, &v12)
	mockQuery(xapi, q13, &v13)
	mockQuery(xapi, q14, &v14)
	mockQuery(xapi, q15, &v15)
	mockQuery(xapi, q16, &v16)
	mockQuery(xapi, q17, &v17)

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?graphType=versionedApp&appenders=&queryTime=1523364075&namespaces=bookinfo,tutorial,istio-system"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_complex_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

func TestMultiClusterSourceGraph(t *testing.T) {
	// bookinfo
	q0 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace!="bookinfo",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.bookinfo\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) ,0.001)`
	v0 := model.Vector{}

	q1 := `round(sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) ,0.001)`
	q1m0 := model.Metric{
		"destination_canonical_revision": "v2",
		"destination_canonical_service":  "reviews",
		"destination_cluster":            "kukulcan",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "v1",
		"source_canonical_service":       "productpage",
		"source_cluster":                 "kukulcan",
		"source_workload":                "productpage-v1",
		"source_workload_namespace":      "bookinfo"}
	q1m1 := model.Metric{
		"destination_canonical_revision": "v1",
		"destination_canonical_service":  "details",
		"destination_cluster":            "kukulcan",
		"destination_service":            "details.bookinfo.svc.cluster.local",
		"destination_service_name":       "details",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "details-v1",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "v1",
		"source_canonical_service":       "productpage",
		"source_cluster":                 "kukulcan",
		"source_workload":                "productpage-v1",
		"source_workload_namespace":      "bookinfo"}
	q1m2 := model.Metric{
		"destination_canonical_revision": "v1",
		"destination_canonical_service":  "productpage",
		"destination_cluster":            "kukulcan",
		"destination_service":            "productpage.bookinfo.svc.cluster.local",
		"destination_service_name":       "productpage",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "productpage-v1",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "latest",
		"source_canonical_service":       "istio-ingressgateway",
		"source_cluster":                 "kukulcan",
		"source_workload":                "istio-ingressgateway",
		"source_workload_namespace":      "istio-system"}
	q1m3 := model.Metric{
		"destination_canonical_revision": "v1",
		"destination_canonical_service":  "reviews",
		"destination_cluster":            "kukulcan",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "v1",
		"source_canonical_service":       "productpage",
		"source_cluster":                 "kukulcan",
		"source_workload":                "productpage-v1",
		"source_workload_namespace":      "bookinfo"}
	v1 := model.Vector{
		&model.Sample{
			Metric: q1m0,
			Value:  100},
		&model.Sample{
			Metric: q1m1,
			Value:  100},
		&model.Sample{
			Metric: q1m2,
			Value:  100},
		&model.Sample{
			Metric: q1m3,
			Value:  100},
	}

	q2 := `round(sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,request_protocol,response_code,grpc_response_status,response_flags) ,0.001)`
	q2m0 := model.Metric{
		"destination_canonical_revision": "v3",
		"destination_canonical_service":  "reviews",
		"destination_cluster":            "tzotz",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "reviews-v3",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "v1",
		"source_canonical_service":       "productpage",
		"source_cluster":                 "kukulcan",
		"source_workload":                "productpage-v1",
		"source_workload_namespace":      "bookinfo"}
	q2m1 := model.Metric{
		"destination_canonical_revision": "v1",
		"destination_canonical_service":  "ratings",
		"destination_cluster":            "tzotz",
		"destination_service":            "ratings.bookinfo.svc.cluster.local",
		"destination_service_name":       "ratings",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "ratings-v1",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "v2",
		"source_canonical_service":       "reviews",
		"source_cluster":                 "kukulcan",
		"source_workload":                "reviews-v2",
		"source_workload_namespace":      "bookinfo"}
	q2m2 := model.Metric{
		"destination_canonical_revision": "v1",
		"destination_canonical_service":  "details",
		"destination_cluster":            "kukulcan",
		"destination_service":            "details.bookinfo.svc.cluster.local",
		"destination_service_name":       "details",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "details-v1",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "v1",
		"source_canonical_service":       "productpage",
		"source_cluster":                 "kukulcan",
		"source_workload":                "productpage-v1",
		"source_workload_namespace":      "bookinfo"}
	q2m3 := model.Metric{
		"destination_canonical_revision": "v1",
		"destination_canonical_service":  "reviews",
		"destination_cluster":            "kukulcan",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "reviews-v1",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "v1",
		"source_canonical_service":       "productpage",
		"source_cluster":                 "kukulcan",
		"source_workload":                "productpage-v1",
		"source_workload_namespace":      "bookinfo"}
	q2m4 := model.Metric{
		"destination_canonical_revision": "v2",
		"destination_canonical_service":  "reviews",
		"destination_cluster":            "kukulcan",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "v1",
		"source_canonical_service":       "productpage",
		"source_cluster":                 "kukulcan",
		"source_workload":                "productpage-v1",
		"source_workload_namespace":      "bookinfo"}
	q2m5 := model.Metric{
		"destination_canonical_revision": "v2",
		"destination_canonical_service":  "reviews",
		"destination_cluster":            "tzotz",
		"destination_service":            "reviews.bookinfo.svc.cluster.local",
		"destination_service_name":       "reviews",
		"destination_service_namespace":  "bookinfo",
		"destination_workload":           "reviews-v2",
		"destination_workload_namespace": "bookinfo",
		"request_protocol":               "http",
		"response_code":                  "200",
		"response_flags":                 "-",
		"source_canonical_revision":      "v1",
		"source_canonical_service":       "productpage",
		"source_cluster":                 "kukulcan",
		"source_workload":                "productpage-v1",
		"source_workload_namespace":      "bookinfo"}
	v2 := model.Vector{
		&model.Sample{
			Metric: q2m0,
			Value:  100},
		&model.Sample{
			Metric: q2m1,
			Value:  100},
		&model.Sample{
			Metric: q2m2,
			Value:  100},
		&model.Sample{
			Metric: q2m3,
			Value:  100},
		&model.Sample{
			Metric: q2m4,
			Value:  100},
		&model.Sample{
			Metric: q2m5,
			Value:  100},
	}

	q3 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace!="bookinfo",destination_workload_namespace="unknown",destination_workload="unknown",destination_service=~"^.+\\.bookinfo\\..+$"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) ,0.001)`
	v3 := model.Vector{}

	q4 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="destination",destination_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) ,0.001)`
	v4 := model.Vector{}

	q5 := `round(sum(rate(istio_tcp_sent_bytes_total{reporter="source",source_workload_namespace="bookinfo"} [600s])) by (source_cluster,source_workload_namespace,source_workload,source_canonical_service,source_canonical_revision,destination_cluster,destination_service_namespace,destination_service,destination_service_name,destination_workload_namespace,destination_workload,destination_canonical_service,destination_canonical_revision,response_flags) ,0.001)`
	v5 := model.Vector{}

	client, xapi, _, err := setupMockedWithIstioComponentNamespaces()
	if err != nil {
		t.Error(err)
		return
	}
	mockQuery(xapi, q0, &v0)
	mockQuery(xapi, q1, &v1)
	mockQuery(xapi, q2, &v2)
	mockQuery(xapi, q3, &v3)
	mockQuery(xapi, q4, &v4)
	mockQuery(xapi, q5, &v5)

	var fut func(b *business.Layer, p *prometheus.Client, o graph.Options) (int, interface{})

	mr := mux.NewRouter()
	mr.HandleFunc("/api/namespaces/graph", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			context := context.WithValue(r.Context(), "authInfo", &api.AuthInfo{Token: "test"})
			code, config := fut(nil, client, graph.NewOptions(r.WithContext(context)))
			respond(w, code, config)
		}))

	ts := httptest.NewServer(mr)
	defer ts.Close()

	fut = graphNamespacesIstio
	url := ts.URL + "/api/namespaces/graph?graphType=versionedApp&injectServiceNodes=true&includeIdleEdges=true&appenders=&queryTime=1523364075&namespaces=bookinfo"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := ioutil.ReadAll(resp.Body)
	expected, _ := ioutil.ReadFile("testdata/test_mc_source_graph.expected")
	if runtime.GOOS == "windows" {
		expected = bytes.Replace(expected, []byte("\r\n"), []byte("\n"), -1)
	}
	expected = expected[:len(expected)-1] // remove EOF byte

	if !assert.Equal(t, expected, actual) {
		fmt.Printf("\nActual:\n%v", string(actual))
	}
	assert.Equal(t, 200, resp.StatusCode)
}
