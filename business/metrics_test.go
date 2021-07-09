package business

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/prometheus"
	"github.com/kiali/kiali/prometheus/prometheustest"
)

func setupMocked() (*MetricsService, *prometheustest.PromAPIMock, error) {
	config.Set(config.NewConfig())
	api := new(prometheustest.PromAPIMock)
	client, err := prometheus.NewClient()
	if err != nil {
		return nil, nil, err
	}
	client.Inject(api)
	return NewMetricsService(client), api, nil
}

func TestGetServiceMetrics(t *testing.T) {
	assert := assert.New(t)
	srv, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}

	q := models.IstioMetricsQuery{
		Namespace: "bookinfo",
		Service:   "productpage",
	}
	q.FillDefaults()
	q.Direction = "inbound"
	q.RateInterval = "5m"
	q.Quantiles = []string{"0.99"}

	labels := `reporter="source",destination_service_name="productpage",destination_service_namespace="bookinfo"`
	api.MockRangeRounded("sum(rate(istio_requests_total{"+labels+"}[5m]))", 2.5)
	api.MockRangeErrRounded("sum(rate(istio_requests_total{"+labels+`,response_code=~"^0$|^[4-5]\\d\\d$"}[5m])) OR sum(rate(istio_requests_total{`+labels+`,grpc_response_status=~"^[1-9]$|^1[0-6]$",response_code!~"^0$|^[4-5]\\d\\d$"}[5m]))`, 4.5)
	api.MockRangeRounded("sum(rate(istio_request_bytes_sum{"+labels+"}[5m]))", 1000)
	api.MockRangeRounded("sum(rate(istio_response_bytes_sum{"+labels+"}[5m]))", 1001)
	api.MockRangeRounded("sum(rate(istio_request_messages_total{"+labels+"}[5m]))", 10)
	api.MockRangeRounded("sum(rate(istio_response_messages_total{"+labels+"}[5m]))", 20)
	api.MockRangeRounded("sum(rate(istio_tcp_received_bytes_total{"+labels+"}[5m]))", 11)
	api.MockRangeRounded("sum(rate(istio_tcp_sent_bytes_total{"+labels+"}[5m]))", 13)
	api.MockHistoRange("istio_request_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.7)
	api.MockHistoRange("istio_request_duration_seconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.8)
	api.MockHistoRange("istio_request_duration_milliseconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.8)
	api.MockHistoRange("istio_response_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.9)

	// Test that range and rate interval are changed when needed (namespace bounds)
	metrics, err := srv.GetMetrics(q, nil)

	assert.Nil(err)
	assert.Equal(11, len(metrics))
	grpcRecIn := metrics["grpc_received"]
	assert.NotNil(grpcRecIn)
	grpcSentIn := metrics["grpc_sent"]
	assert.NotNil(grpcSentIn)
	rqCountIn := metrics["request_count"]
	assert.NotNil(rqCountIn)
	rqErrorCountIn := metrics["request_error_count"]
	assert.NotNil(rqErrorCountIn)
	rqThroughput := metrics["request_throughput"]
	assert.NotNil(rqThroughput)
	rsThroughput := metrics["response_throughput"]
	assert.NotNil(rsThroughput)
	rqSizeIn := metrics["request_size"]
	assert.NotNil(rqSizeIn)
	rqDurationMillisIn := metrics["request_duration_millis"]
	assert.NotNil(rqDurationMillisIn)
	rsSizeIn := metrics["response_size"]
	assert.NotNil(rsSizeIn)
	tcpRecIn := metrics["tcp_received"]
	assert.NotNil(tcpRecIn)
	tcpSentIn := metrics["tcp_sent"]
	assert.NotNil(tcpSentIn)

	assert.Equal(20.0, float64(grpcRecIn[0].Datapoints[0].Value))
	assert.Equal(10.0, float64(grpcSentIn[0].Datapoints[0].Value))
	assert.Equal(2.5, float64(rqCountIn[0].Datapoints[0].Value))
	assert.Equal(4.5, float64(rqErrorCountIn[0].Datapoints[0].Value))
	assert.Equal(1000.0, float64(rqThroughput[0].Datapoints[0].Value))
	assert.Equal(1001.0, float64(rsThroughput[0].Datapoints[0].Value))
	assertHisto(assert, rqSizeIn, "0.99", 0.7)
	assertHisto(assert, rqDurationMillisIn, "0.99", 0.8)
	assertHisto(assert, rsSizeIn, "0.99", 0.9)
	assert.Equal(11.0, float64(tcpRecIn[0].Datapoints[0].Value))
	assert.Equal(13.0, float64(tcpSentIn[0].Datapoints[0].Value))
}

func assertHisto(assert *assert.Assertions, metrics []models.Metric, stat string, expected float64) {
	for _, m := range metrics {
		if m.Stat == stat {
			assert.Equal(expected, m.Datapoints[0].Value)
			return
		}
	}
	assert.Fail(fmt.Sprintf("Stat %s not found in %v", stat, metrics))
}

func assertEmptyHisto(assert *assert.Assertions, metrics []models.Metric, stat string) {
	for _, m := range metrics {
		if m.Stat == stat {
			assert.Empty(m.Datapoints, fmt.Sprintf("Expected stat %s to be empty", stat))
			return
		}
	}
	assert.Fail(fmt.Sprintf("Stat %s not found in %v", stat, metrics))
}

func TestGetAppMetrics(t *testing.T) {
	assert := assert.New(t)
	srv, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	labels := `reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage"`
	api.MockRangeRounded("sum(rate(istio_requests_total{"+labels+"}[5m]))", 1.5)
	api.MockRangeErrRounded("sum(rate(istio_requests_total{"+labels+`,response_code=~"^0$|^[4-5]\\d\\d$"}[5m])) OR sum(rate(istio_requests_total{`+labels+`,grpc_response_status=~"^[1-9]$|^1[0-6]$",response_code!~"^0$|^[4-5]\\d\\d$"}[5m]))`, 3.5)
	api.MockRangeRounded("sum(rate(istio_request_bytes_sum{"+labels+"}[5m]))", 1000)
	api.MockRangeRounded("sum(rate(istio_response_bytes_sum{"+labels+"}[5m]))", 1001)
	api.MockRangeRounded("sum(rate(istio_request_messages_total{"+labels+"}[5m]))", 10)
	api.MockRangeRounded("sum(rate(istio_response_messages_total{"+labels+"}[5m]))", 20)
	api.MockRangeRounded("sum(rate(istio_tcp_received_bytes_total{"+labels+"}[5m]))", 10)
	api.MockRangeRounded("sum(rate(istio_tcp_sent_bytes_total{"+labels+"}[5m]))", 12)
	api.MockHistoRange("istio_request_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.4)
	api.MockHistoRange("istio_request_duration_seconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.5)
	api.MockHistoRange("istio_request_duration_milliseconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.5)
	api.MockHistoRange("istio_response_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.6)

	q := models.IstioMetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
	}
	q.FillDefaults()
	q.RateInterval = "5m"
	q.Quantiles = []string{"0.5", "0.95", "0.99"}
	metrics, err := srv.GetMetrics(q, nil)

	assert.Nil(err)
	assert.Equal(11, len(metrics))
	grpcRecIn := metrics["grpc_received"]
	assert.NotNil(grpcRecIn)
	grpcSentIn := metrics["grpc_sent"]
	assert.NotNil(grpcSentIn)
	rqCountIn := metrics["request_count"]
	assert.NotNil(rqCountIn)
	rqErrorCountIn := metrics["request_error_count"]
	assert.NotNil(rqErrorCountIn)
	rqThroughput := metrics["request_throughput"]
	assert.NotNil(rqThroughput)
	rsThroughput := metrics["response_throughput"]
	assert.NotNil(rsThroughput)
	rqSizeIn := metrics["request_size"]
	assert.NotNil(rqSizeIn)
	rqDurationMillisIn := metrics["request_duration_millis"]
	assert.NotNil(rqDurationMillisIn)
	rsSizeIn := metrics["response_size"]
	assert.NotNil(rsSizeIn)
	tcpRecIn := metrics["tcp_received"]
	assert.NotNil(tcpRecIn)
	tcpSentIn := metrics["tcp_sent"]
	assert.NotNil(tcpSentIn)

	assert.Equal(20.0, float64(grpcRecIn[0].Datapoints[0].Value))
	assert.Equal(10.0, float64(grpcSentIn[0].Datapoints[0].Value))
	assert.Equal(1.5, float64(rqCountIn[0].Datapoints[0].Value))
	assert.Equal(3.5, float64(rqErrorCountIn[0].Datapoints[0].Value))
	assert.Equal(1000.0, float64(rqThroughput[0].Datapoints[0].Value))
	assert.Equal(1001.0, float64(rsThroughput[0].Datapoints[0].Value))
	assertHisto(assert, rqSizeIn, "avg", 0.35)
	assertHisto(assert, rqSizeIn, "0.5", 0.2)
	assertHisto(assert, rqSizeIn, "0.95", 0.3)
	assertHisto(assert, rqSizeIn, "0.99", 0.4)
	assertHisto(assert, rqDurationMillisIn, "0.99", 0.5)
	assertHisto(assert, rsSizeIn, "0.99", 0.6)
	assert.Equal(10.0, float64(tcpRecIn[0].Datapoints[0].Value))
	assert.Equal(12.0, float64(tcpSentIn[0].Datapoints[0].Value))
}

func TestGetFilteredAppMetrics(t *testing.T) {
	assert := assert.New(t)
	srv, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	api.MockRangeRounded(`sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage"}[5m]))`, 1.5)
	api.MockHistoRange("istio_request_bytes", `{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage"}[5m]`, 0.35, 0.2, 0.3, 0.4)
	q := models.IstioMetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
	}
	q.FillDefaults()
	q.RateInterval = "5m"
	q.Filters = []string{"request_count", "request_size"}
	metrics, err := srv.GetMetrics(q, nil)

	assert.Nil(err)
	assert.Equal(2, len(metrics))
	rqCountOut := metrics["request_count"]
	assert.NotNil(rqCountOut)
	rqSizeOut := metrics["request_size"]
	assert.NotNil(rqSizeOut)
}

func TestGetAppMetricsInstantRates(t *testing.T) {
	assert := assert.New(t)
	srv, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	api.MockRangeRounded(`sum(irate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage"}[1m]))`, 1.5)
	q := models.IstioMetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
	}
	q.FillDefaults()
	q.RateFunc = "irate"
	q.Filters = []string{"request_count"}
	metrics, err := srv.GetMetrics(q, nil)

	assert.Nil(err)
	assert.Equal(1, len(metrics))
	rqCountOut := metrics["request_count"]
	assert.NotNil(rqCountOut)
}

func TestGetAppMetricsUnavailable(t *testing.T) {
	assert := assert.New(t)
	srv, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	// Mock everything to return empty data
	api.MockEmptyRangeRounded(`sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage"}[5m]))`)
	api.MockEmptyHistoRange("istio_request_bytes", `{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage"}[5m]`)
	q := models.IstioMetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
	}
	q.FillDefaults()
	q.RateInterval = "5m"
	q.Quantiles = []string{"0.5", "0.95", "0.99"}
	q.Filters = []string{"request_count", "request_size"}
	metrics, err := srv.GetMetrics(q, nil)

	assert.Nil(err)
	assert.Equal(2, len(metrics))
	// Simple metric & histogram are empty
	rqCountIn := metrics["request_count"]
	assert.NotNil(rqCountIn)
	assert.Empty(rqCountIn[0].Datapoints)

	rqSizeIn := metrics["request_size"]
	assert.NotNil(rqSizeIn)
	assertEmptyHisto(assert, rqSizeIn, "avg")
	assertEmptyHisto(assert, rqSizeIn, "0.5")
	assertEmptyHisto(assert, rqSizeIn, "0.95")
	assertEmptyHisto(assert, rqSizeIn, "0.99")
}

func TestGetNamespaceMetrics(t *testing.T) {
	assert := assert.New(t)
	srv, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	labels := `reporter="source",source_workload_namespace="bookinfo"`
	api.MockRangeRounded("sum(rate(istio_requests_total{"+labels+"}[5m]))", 1.5)
	api.MockRangeErrRounded("sum(rate(istio_requests_total{"+labels+`,response_code=~"^0$|^[4-5]\\d\\d$"}[5m])) OR sum(rate(istio_requests_total{`+labels+`,grpc_response_status=~"^[1-9]$|^1[0-6]$",response_code!~"^0$|^[4-5]\\d\\d$"}[5m]))`, 3.5)
	api.MockRangeRounded("sum(rate(istio_request_bytes_sum{"+labels+"}[5m]))", 1000)
	api.MockRangeRounded("sum(rate(istio_response_bytes_sum{"+labels+"}[5m]))", 1001)
	api.MockRangeRounded("sum(rate(istio_request_messages_total{"+labels+"}[5m]))", 10)
	api.MockRangeRounded("sum(rate(istio_response_messages_total{"+labels+"}[5m]))", 20)
	api.MockRangeRounded("sum(rate(istio_tcp_received_bytes_total{"+labels+"}[5m]))", 10)
	api.MockRangeRounded("sum(rate(istio_tcp_sent_bytes_total{"+labels+"}[5m]))", 12)
	api.MockHistoRange("istio_request_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.4)
	api.MockHistoRange("istio_request_duration_seconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.5)
	api.MockHistoRange("istio_request_duration_milliseconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.5)
	api.MockHistoRange("istio_response_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.6)

	q := models.IstioMetricsQuery{
		Namespace: "bookinfo",
	}
	q.FillDefaults()
	q.RateInterval = "5m"
	q.Quantiles = []string{"0.5", "0.95", "0.99"}
	metrics, err := srv.GetMetrics(q, nil)

	assert.Nil(err)
	assert.Equal(11, len(metrics))
	grpcRecOut := metrics["grpc_received"]
	assert.NotNil(grpcRecOut)
	grpcSentOut := metrics["grpc_sent"]
	assert.NotNil(grpcSentOut)
	rqCountOut := metrics["request_count"]
	assert.NotNil(rqCountOut)
	rqErrorCountOut := metrics["request_error_count"]
	assert.NotNil(rqErrorCountOut)
	rqThroughput := metrics["request_throughput"]
	assert.NotNil(rqThroughput)
	rsThroughput := metrics["response_throughput"]
	assert.NotNil(rsThroughput)
	rqSizeOut := metrics["request_size"]
	assert.NotNil(rqSizeOut)
	rqDurationMillisOut := metrics["request_duration_millis"]
	assert.NotNil(rqDurationMillisOut)
	rsSizeOut := metrics["response_size"]
	assert.NotNil(rsSizeOut)
	tcpRecOut := metrics["tcp_received"]
	assert.NotNil(tcpRecOut)
	tcpSentOut := metrics["tcp_sent"]
	assert.NotNil(tcpSentOut)

	assert.Equal(20.0, float64(grpcRecOut[0].Datapoints[0].Value))
	assert.Equal(10.0, float64(grpcSentOut[0].Datapoints[0].Value))
	assert.Equal(1.5, float64(rqCountOut[0].Datapoints[0].Value))
	assert.Equal(3.5, float64(rqErrorCountOut[0].Datapoints[0].Value))
	assert.Equal(1000.0, float64(rqThroughput[0].Datapoints[0].Value))
	assert.Equal(1001.0, float64(rsThroughput[0].Datapoints[0].Value))
	assertHisto(assert, rqSizeOut, "avg", 0.35)
	assertHisto(assert, rqSizeOut, "0.5", 0.2)
	assertHisto(assert, rqSizeOut, "0.95", 0.3)
	assertHisto(assert, rqSizeOut, "0.99", 0.4)
	assertHisto(assert, rqDurationMillisOut, "0.99", 0.5)
	assertHisto(assert, rsSizeOut, "0.99", 0.6)
	assert.Equal(10.0, float64(tcpRecOut[0].Datapoints[0].Value))
	assert.Equal(12.0, float64(tcpSentOut[0].Datapoints[0].Value))
}

func TestCreateMetricsLabelsBuilder(t *testing.T) {
	assert := assert.New(t)
	q := models.IstioMetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
	}
	q.FillDefaults()
	q.Reporter = "source"
	lb := createMetricsLabelsBuilder(&q)
	assert.Equal(`{reporter="source",source_workload_namespace="bookinfo",source_canonical_service="productpage"}`, lb.Build())
}

func TestCreateStatsMetricsLabelsBuilder(t *testing.T) {
	assert := assert.New(t)
	q := models.MetricsStatsQuery{
		Target: models.Target{
			Namespace: "ns3",
			Name:      "foo",
			Kind:      "app",
		},
		Direction: "inbound",
		Interval:  "3h",
		Avg:       true,
		Quantiles: []string{"0.90", "0.5"},
		QueryTime: time.Now(),
	}
	lb := createStatsMetricsLabelsBuilder(&q)
	assert.Equal(`{reporter="destination",destination_workload_namespace="ns3",destination_canonical_service="foo"}`, lb.Build())
}

func TestCreateStatsMetricsLabelsBuilderWithPeer(t *testing.T) {
	assert := assert.New(t)
	q := models.MetricsStatsQuery{
		Target: models.Target{
			Namespace: "ns3",
			Name:      "foo",
			Kind:      "app",
		},
		PeerTarget: &models.Target{
			Namespace: "ns4",
			Name:      "bar",
			Kind:      "app",
		},
		Direction: "inbound",
		Interval:  "3h",
		Avg:       true,
		Quantiles: []string{"0.90", "0.5"},
		QueryTime: time.Now(),
	}
	lb := createStatsMetricsLabelsBuilder(&q)
	assert.Equal(`{reporter="destination",destination_workload_namespace="ns3",destination_canonical_service="foo",source_workload_namespace="ns4",source_canonical_service="bar"}`, lb.Build())
}

func TestGetMetricsStats(t *testing.T) {
	assert := assert.New(t)
	srv, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}

	queryTime := time.Now()
	queries := []models.MetricsStatsQuery{{
		Target: models.Target{
			Namespace: "ns1",
			Name:      "foo",
			Kind:      "app",
		},
		Direction:    "outbound",
		Interval:     "30m",
		RawInterval:  "30m",
		Avg:          true,
		Quantiles:    []string{"0.95"},
		RawQueryTime: queryTime.Unix(),
	}, {
		Target: models.Target{
			Namespace: "ns2",
			Name:      "bar",
			Kind:      "service",
		},
		PeerTarget: &models.Target{
			Namespace: "ns3",
			Name:      "w1",
			Kind:      "workload",
		},
		Direction:    "inbound",
		Interval:     "3h",
		RawInterval:  "3h",
		Avg:          false,
		Quantiles:    []string{"0.5", "0.95"},
		RawQueryTime: queryTime.Unix(),
	}}

	// Setup mocks
	v0 := model.Vector{createSample(0)}
	q1Avg := model.Vector{createSample(5)}
	q1P95 := model.Vector{createSample(8)}
	q2P50 := model.Vector{createSample(6.3)}
	q2P95 := model.Vector{createSample(9.3)}
	q1Labels := `reporter="source",source_workload_namespace="ns1",source_canonical_service="foo"`
	q2Labels := `reporter="destination",destination_service_name="bar",destination_service_namespace="ns2",source_workload_namespace="ns3",source_workload="w1"`
	api.MockHistoValue("istio_request_duration_milliseconds", "{"+q1Labels+"}[30m]", q1Avg, v0, q1P95, v0)
	api.MockHistoValue("istio_request_duration_milliseconds", "{"+q2Labels+"}[3h]", v0, q2P50, q2P95, v0)

	stats, err := srv.GetStats(queries)

	assert.Nil(err)
	assert.Len(stats, 2)
	fmt.Printf("%v\n", stats)
	assert.Equal([]models.Stat{{Name: "0.95", Value: 8.0}, {Name: "avg", Value: 5.0}}, stats["ns1:app:foo::outbound:30m"].ResponseTimes)
	assert.Equal([]models.Stat{{Name: "0.5", Value: 6.3}, {Name: "0.95", Value: 9.3}}, stats["ns2:service:bar:ns3:workload:w1:inbound:3h"].ResponseTimes)
}

func createSample(value float64) *model.Sample {
	return &model.Sample{
		Timestamp: model.Now(),
		Value:     model.SampleValue(value),
		Metric:    model.Metric{},
	}
}
