package business

import (
	"fmt"
	"testing"
	"time"

	prom_v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kiali/kiali/config"
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

func round(q string) string {
	return fmt.Sprintf("round(%s, 0.001000) > 0.001000 or %s", q, q)
}

func roundErrs(q string) string {
	return fmt.Sprintf("round((%s), 0.001000) > 0.001000 or (%s)", q, q)
}

func TestGetServiceMetrics(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}

	q := IstioMetricsQuery{
		Namespace: "bookinfo",
		Service:   "productpage",
	}
	q.FillDefaults()
	q.Direction = "inbound"
	q.RateInterval = "5m"
	q.Quantiles = []string{"0.99"}
	expectedRange := prom_v1.Range{
		Start: q.Start,
		End:   q.End,
		Step:  q.Step,
	}

	labels := `reporter="source",destination_service_name="productpage",destination_service_namespace="bookinfo"`
	mockWithRange(api, expectedRange, round("sum(rate(istio_requests_total{"+labels+"}[5m]))"), 2.5)
	mockWithRange(api, expectedRange, roundErrs("sum(rate(istio_requests_total{"+labels+`,response_code=~"^0$|^[4-5]\\d\\d$"}[5m])) OR sum(rate(istio_requests_total{`+labels+`,grpc_response_status=~"^[1-9]$|^1[0-6]$",response_code!~"^0$|^[4-5]\\d\\d$"}[5m]))`), 4.5)
	mockWithRange(api, expectedRange, round("sum(rate(istio_request_bytes_sum{"+labels+"}[5m]))"), 1000)
	mockWithRange(api, expectedRange, round("sum(rate(istio_response_bytes_sum{"+labels+"}[5m]))"), 1001)
	mockWithRange(api, expectedRange, round("sum(rate(istio_tcp_received_bytes_total{"+labels+"}[5m]))"), 11)
	mockWithRange(api, expectedRange, round("sum(rate(istio_tcp_sent_bytes_total{"+labels+"}[5m]))"), 13)
	mockHistogram(api, "istio_request_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.7)
	mockHistogram(api, "istio_request_duration_seconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.8)
	mockHistogram(api, "istio_request_duration_milliseconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.8)
	mockHistogram(api, "istio_response_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.9)

	// Test that range and rate interval are changed when needed (namespace bounds)
	metrics := client.GetMetrics(q)

	assert.Equal(t, 6, len(metrics.Metrics), "Should have 6 simple metrics")
	assert.Equal(t, 4, len(metrics.Histograms), "Should have 4 histograms")
	rqCountIn := metrics.Metrics["request_count"]
	assert.NotNil(t, rqCountIn)
	rqErrorCountIn := metrics.Metrics["request_error_count"]
	assert.NotNil(t, rqErrorCountIn)
	rqThroughput := metrics.Metrics["request_throughput"]
	assert.NotNil(t, rqThroughput)
	rsThroughput := metrics.Metrics["response_throughput"]
	assert.NotNil(t, rsThroughput)
	rqSizeIn := metrics.Histograms["request_size"]
	assert.NotNil(t, rqSizeIn)
	rqDurationIn := metrics.Histograms["request_duration"]
	assert.NotNil(t, rqDurationIn)
	rqDurationMillisIn := metrics.Histograms["request_duration_millis"]
	assert.NotNil(t, rqDurationMillisIn)
	rsSizeIn := metrics.Histograms["response_size"]
	assert.NotNil(t, rsSizeIn)
	tcpRecIn := metrics.Metrics["tcp_received"]
	assert.NotNil(t, tcpRecIn)
	tcpSentIn := metrics.Metrics["tcp_sent"]
	assert.NotNil(t, tcpSentIn)

	assert.Equal(t, 2.5, float64(rqCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 4.5, float64(rqErrorCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 1000.0, float64(rqThroughput.Matrix[0].Values[0].Value))
	assert.Equal(t, 1001.0, float64(rsThroughput.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.7, float64(rqSizeIn["0.99"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.8, float64(rqDurationIn["0.99"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.9, float64(rsSizeIn["0.99"].Matrix[0].Values[0].Value))
	assert.Equal(t, 11.0, float64(tcpRecIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 13.0, float64(tcpSentIn.Matrix[0].Values[0].Value))
}

func TestGetAppMetrics(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	labels := `reporter="source",source_workload_namespace="bookinfo",source_app="productpage"`
	mockRange(api, round("sum(rate(istio_requests_total{"+labels+"}[5m]))"), 1.5)
	mockRange(api, roundErrs("sum(rate(istio_requests_total{"+labels+`,response_code=~"^0$|^[4-5]\\d\\d$"}[5m])) OR sum(rate(istio_requests_total{`+labels+`,grpc_response_status=~"^[1-9]$|^1[0-6]$",response_code!~"^0$|^[4-5]\\d\\d$"}[5m]))`), 3.5)
	mockRange(api, round("sum(rate(istio_request_bytes_sum{"+labels+"}[5m]))"), 1000)
	mockRange(api, round("sum(rate(istio_response_bytes_sum{"+labels+"}[5m]))"), 1001)
	mockRange(api, round("sum(rate(istio_tcp_received_bytes_total{"+labels+"}[5m]))"), 10)
	mockRange(api, round("sum(rate(istio_tcp_sent_bytes_total{"+labels+"}[5m]))"), 12)
	mockHistogram(api, "istio_request_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.4)
	mockHistogram(api, "istio_request_duration_seconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.5)
	mockHistogram(api, "istio_request_duration_milliseconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.5)
	mockHistogram(api, "istio_response_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.6)

	q := IstioMetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
	}
	q.FillDefaults()
	q.RateInterval = "5m"
	q.Quantiles = []string{"0.5", "0.95", "0.99"}
	metrics := client.GetMetrics(q)

	assert.Equal(t, 6, len(metrics.Metrics), "Should have 6 simple metrics")
	assert.Equal(t, 4, len(metrics.Histograms), "Should have 4 histograms")
	rqCountIn := metrics.Metrics["request_count"]
	assert.NotNil(t, rqCountIn)
	rqErrorCountIn := metrics.Metrics["request_error_count"]
	assert.NotNil(t, rqErrorCountIn)
	rqThroughput := metrics.Metrics["request_throughput"]
	assert.NotNil(t, rqThroughput)
	rsThroughput := metrics.Metrics["response_throughput"]
	assert.NotNil(t, rsThroughput)
	rqSizeIn := metrics.Histograms["request_size"]
	assert.NotNil(t, rqSizeIn)
	rqDurationIn := metrics.Histograms["request_duration"]
	assert.NotNil(t, rqDurationIn)
	rqDurationMillisIn := metrics.Histograms["request_duration_millis"]
	assert.NotNil(t, rqDurationMillisIn)
	rsSizeIn := metrics.Histograms["response_size"]
	assert.NotNil(t, rsSizeIn)
	tcpRecIn := metrics.Metrics["tcp_received"]
	assert.NotNil(t, tcpRecIn)
	tcpSentIn := metrics.Metrics["tcp_sent"]
	assert.NotNil(t, tcpSentIn)

	assert.Equal(t, 1.5, float64(rqCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 3.5, float64(rqErrorCountIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 1000.0, float64(rqThroughput.Matrix[0].Values[0].Value))
	assert.Equal(t, 1001.0, float64(rsThroughput.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.35, float64(rqSizeIn["avg"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.2, float64(rqSizeIn["0.5"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.3, float64(rqSizeIn["0.95"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.4, float64(rqSizeIn["0.99"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.5, float64(rqDurationIn["0.99"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.6, float64(rsSizeIn["0.99"].Matrix[0].Values[0].Value))
	assert.Equal(t, 10.0, float64(tcpRecIn.Matrix[0].Values[0].Value))
	assert.Equal(t, 12.0, float64(tcpSentIn.Matrix[0].Values[0].Value))
}

func TestGetFilteredAppMetrics(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockRange(api, round(`sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_app="productpage"}[5m]))`), 1.5)
	mockHistogram(api, "istio_request_bytes", `{reporter="source",source_workload_namespace="bookinfo",source_app="productpage"}[5m]`, 0.35, 0.2, 0.3, 0.4)
	q := IstioMetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
	}
	q.FillDefaults()
	q.RateInterval = "5m"
	q.Filters = []string{"request_count", "request_size"}
	metrics := client.GetMetrics(q)

	assert.Equal(t, 1, len(metrics.Metrics), "Should have 1 simple metric")
	assert.Equal(t, 1, len(metrics.Histograms), "Should have 1 histogram")
	rqCountOut := metrics.Metrics["request_count"]
	assert.NotNil(t, rqCountOut)
	rqSizeOut := metrics.Histograms["request_size"]
	assert.NotNil(t, rqSizeOut)
}

func TestGetAppMetricsInstantRates(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	mockRange(api, round(`sum(irate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_app="productpage"}[1m]))`), 1.5)
	q := IstioMetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
	}
	q.FillDefaults()
	q.RateFunc = "irate"
	q.Filters = []string{"request_count"}
	metrics := client.GetMetrics(q)

	assert.Equal(t, 1, len(metrics.Metrics), "Should have 1 simple metric")
	assert.Equal(t, 0, len(metrics.Histograms), "Should have no histogram")
	rqCountOut := metrics.Metrics["request_count"]
	assert.NotNil(t, rqCountOut)
}

func TestGetAppMetricsUnavailable(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	// Mock everything to return empty data
	mockEmptyRange(api, round(`sum(rate(istio_requests_total{reporter="source",source_workload_namespace="bookinfo",source_app="productpage"}[5m]))`))
	mockEmptyHistogram(api, "istio_request_bytes", `{reporter="source",source_workload_namespace="bookinfo",source_app="productpage"}[5m]`)
	q := IstioMetricsQuery{
		Namespace: "bookinfo",
		App:       "productpage",
	}
	q.FillDefaults()
	q.RateInterval = "5m"
	q.Quantiles = []string{"0.5", "0.95", "0.99"}
	q.Filters = []string{"request_count", "request_size"}
	metrics := client.GetMetrics(q)

	assert.Equal(t, 1, len(metrics.Metrics), "Should have 1 simple metric")
	assert.Equal(t, 1, len(metrics.Histograms), "Should have 1 histogram")
	rqCountIn := metrics.Metrics["request_count"]
	assert.NotNil(t, rqCountIn)
	rqSizeIn := metrics.Histograms["request_size"]
	assert.NotNil(t, rqSizeIn)

	// Simple metric & histogram are empty
	assert.Empty(t, rqCountIn.Matrix[0].Values)
	assert.Empty(t, rqSizeIn["avg"].Matrix[0].Values)
	assert.Empty(t, rqSizeIn["0.5"].Matrix[0].Values)
	assert.Empty(t, rqSizeIn["0.95"].Matrix[0].Values)
	assert.Empty(t, rqSizeIn["0.99"].Matrix[0].Values)
}

func TestGetNamespaceMetrics(t *testing.T) {
	client, api, err := setupMocked()
	if err != nil {
		t.Error(err)
		return
	}
	labels := `reporter="source",source_workload_namespace="bookinfo"`
	mockRange(api, round("sum(rate(istio_requests_total{"+labels+"}[5m]))"), 1.5)
	mockRange(api, roundErrs("sum(rate(istio_requests_total{"+labels+`,response_code=~"^0$|^[4-5]\\d\\d$"}[5m])) OR sum(rate(istio_requests_total{`+labels+`,grpc_response_status=~"^[1-9]$|^1[0-6]$",response_code!~"^0$|^[4-5]\\d\\d$"}[5m]))`), 3.5)
	mockRange(api, round("sum(rate(istio_request_bytes_sum{"+labels+"}[5m]))"), 1000)
	mockRange(api, round("sum(rate(istio_response_bytes_sum{"+labels+"}[5m]))"), 1001)
	mockRange(api, round("sum(rate(istio_tcp_received_bytes_total{"+labels+"}[5m]))"), 10)
	mockRange(api, round("sum(rate(istio_tcp_sent_bytes_total{"+labels+"}[5m]))"), 12)
	mockHistogram(api, "istio_request_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.4)
	mockHistogram(api, "istio_request_duration_seconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.5)
	mockHistogram(api, "istio_request_duration_milliseconds", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.5)
	mockHistogram(api, "istio_response_bytes", "{"+labels+"}[5m]", 0.35, 0.2, 0.3, 0.6)

	q := IstioMetricsQuery{
		Namespace: "bookinfo",
	}
	q.FillDefaults()
	q.RateInterval = "5m"
	q.Quantiles = []string{"0.5", "0.95", "0.99"}
	metrics := client.GetMetrics(q)

	assert.Equal(t, 6, len(metrics.Metrics), "Should have 6 simple metrics")
	assert.Equal(t, 4, len(metrics.Histograms), "Should have 4 histograms")
	rqCountOut := metrics.Metrics["request_count"]
	assert.NotNil(t, rqCountOut)
	rqErrorCountOut := metrics.Metrics["request_error_count"]
	assert.NotNil(t, rqErrorCountOut)
	rqThroughput := metrics.Metrics["request_throughput"]
	assert.NotNil(t, rqThroughput)
	rsThroughput := metrics.Metrics["response_throughput"]
	assert.NotNil(t, rsThroughput)
	rqSizeOut := metrics.Histograms["request_size"]
	assert.NotNil(t, rqSizeOut)
	rqDurationOut := metrics.Histograms["request_duration"]
	assert.NotNil(t, rqDurationOut)
	rqDurationMillisOut := metrics.Histograms["request_duration_millis"]
	assert.NotNil(t, rqDurationMillisOut)
	rsSizeOut := metrics.Histograms["response_size"]
	assert.NotNil(t, rsSizeOut)
	tcpRecOut := metrics.Metrics["tcp_received"]
	assert.NotNil(t, tcpRecOut)
	tcpSentOut := metrics.Metrics["tcp_sent"]
	assert.NotNil(t, tcpSentOut)

	assert.Equal(t, 1.5, float64(rqCountOut.Matrix[0].Values[0].Value))
	assert.Equal(t, 3.5, float64(rqErrorCountOut.Matrix[0].Values[0].Value))
	assert.Equal(t, 1000.0, float64(rqThroughput.Matrix[0].Values[0].Value))
	assert.Equal(t, 1001.0, float64(rsThroughput.Matrix[0].Values[0].Value))
	assert.Equal(t, 0.35, float64(rqSizeOut["avg"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.2, float64(rqSizeOut["0.5"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.3, float64(rqSizeOut["0.95"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.4, float64(rqSizeOut["0.99"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.5, float64(rqDurationOut["0.99"].Matrix[0].Values[0].Value))
	assert.Equal(t, 0.6, float64(rsSizeOut["0.99"].Matrix[0].Values[0].Value))
	assert.Equal(t, 10.0, float64(tcpRecOut.Matrix[0].Values[0].Value))
	assert.Equal(t, 12.0, float64(tcpSentOut.Matrix[0].Values[0].Value))
}

func mockQueryWithTime(api *prometheustest.PromAPIMock, query string, queryTime time.Time, ret *model.Vector) {
	api.On(
		"Query",
		mock.AnythingOfType("*context.emptyCtx"),
		query,
		queryTime).
		Return(*ret, nil)
}

func mockQueryRange(api *prometheustest.PromAPIMock, query string, ret *model.Matrix) {
	api.On(
		"QueryRange",
		mock.AnythingOfType("*context.emptyCtx"),
		query,
		mock.AnythingOfType(`v1.Range`)).
		Return(*ret, nil)
}

func mockRange(api *prometheustest.PromAPIMock, query string, ret model.SampleValue) {
	metric := model.Metric{
		"reporter": "destination",
		"__name__": "whatever",
		"instance": "whatever",
		"job":      "whatever"}
	matrix := model.Matrix{
		&model.SampleStream{
			Metric: metric,
			Values: []model.SamplePair{{Timestamp: 0, Value: ret}}}}
	mockQueryRange(api, query, &matrix)
}

func mockWithRange(api *prometheustest.PromAPIMock, qRange prom_v1.Range, query string, ret model.SampleValue) {
	metric := model.Metric{
		"reporter": "destination",
		"__name__": "whatever",
		"instance": "whatever",
		"job":      "whatever"}
	matrix := model.Matrix{
		&model.SampleStream{
			Metric: metric,
			Values: []model.SamplePair{{Timestamp: 0, Value: ret}}}}
	api.On(
		"QueryRange",
		mock.AnythingOfType("*context.emptyCtx"),
		query,
		qRange).
		Return(matrix, nil)
}

func mockEmptyRange(api *prometheustest.PromAPIMock, query string) {
	metric := model.Metric{
		"reporter": "destination",
		"__name__": "whatever",
		"instance": "whatever",
		"job":      "whatever"}
	matrix := model.Matrix{
		&model.SampleStream{
			Metric: metric,
			Values: []model.SamplePair{}}}
	mockQueryRange(api, query, &matrix)
}

func mockHistogram(api *prometheustest.PromAPIMock, baseName string, suffix string, retAvg model.SampleValue, retMed model.SampleValue, ret95 model.SampleValue, ret99 model.SampleValue) {
	histMetric := "sum(rate(" + baseName + "_bucket" + suffix + ")) by (le))"
	mockRange(api, round("histogram_quantile(0.5, "+histMetric), retMed)
	mockRange(api, round("histogram_quantile(0.95, "+histMetric), ret95)
	mockRange(api, round("histogram_quantile(0.99, "+histMetric), ret99)
	mockRange(api, round("histogram_quantile(0.999, "+histMetric), ret99)
	mockRange(api, round("sum(rate("+baseName+"_sum"+suffix+")) / sum(rate("+baseName+"_count"+suffix+"))"), retAvg)
}

func mockEmptyHistogram(api *prometheustest.PromAPIMock, baseName string, suffix string) {
	histMetric := "sum(rate(" + baseName + "_bucket" + suffix + ")) by (le))"
	mockEmptyRange(api, round("histogram_quantile(0.5, "+histMetric))
	mockEmptyRange(api, round("histogram_quantile(0.95, "+histMetric))
	mockEmptyRange(api, round("histogram_quantile(0.99, "+histMetric))
	mockEmptyRange(api, round("histogram_quantile(0.999, "+histMetric))
	mockEmptyRange(api, round("sum(rate("+baseName+"_sum"+suffix+")) / sum(rate("+baseName+"_count"+suffix+"))"))
}
