package business

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/config/dashboards"
	"github.com/kiali/kiali/models"
	pmock "github.com/kiali/kiali/prometheus/prometheustest"
)

func setupService(namespace string, dashboards []dashboards.MonitoringDashboard) (*DashboardsService, *pmock.PromClientMock) {
	cfg := config.NewConfig()
	for _, d := range dashboards {
		cfg.CustomDashboards = append(cfg.CustomDashboards, d)
	}
	config.Set(cfg)
	prom := new(pmock.PromClientMock)
	ns := models.Namespace{Name: namespace}
	service := NewDashboardsService(&ns, nil)
	service.promClient = prom
	return service, prom
}

func TestGetDashboard(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	service, prom := setupService("my-namespace", []dashboards.MonitoringDashboard{*fakeDashboard("1")})

	expectedLabels := "{kubernetes_namespace=\"my-namespace\",APP=\"my-app\"}"
	namespace := models.Namespace{
		Name: "my-namespace",
	}
	query := models.DashboardQuery{
		Namespace: namespace.Name,
		LabelsFilters: map[string]string{
			"APP": "my-app",
		},
		AdditionalLabels: []models.Aggregation{
			{
				Label:       "version",
				DisplayName: "Version",
			},
		},
	}
	query.FillDefaults()
	prom.MockMetric("my_metric_1_1", expectedLabels, &query.RangeQuery, 10)
	prom.MockHistogram("my_metric_1_2", expectedLabels, &query.RangeQuery, 11, 12)

	dashboard, err := service.GetDashboard(&api.AuthInfo{Token: ""}, query, "dashboard1")

	assert.Nil(err)
	assert.Equal("Dashboard 1", dashboard.Title)
	assert.Len(dashboard.Aggregations, 3)
	assert.Len(dashboard.Charts, 2)
	assert.Equal("My chart 1_1", dashboard.Charts[0].Name)
	assert.Equal("My chart 1_2", dashboard.Charts[1].Name)
	assert.Len(dashboard.Charts[0].Metrics, 1)
	// Note: fake dashboard has scale=10 for every chart
	assert.Equal(float64(100), dashboard.Charts[0].Metrics[0].Datapoints[0].Value)
	assert.Len(dashboard.Charts[1].Metrics, 2)
	assertHisto(assert, dashboard.Charts[1].Metrics, "avg", 110)
	assertHisto(assert, dashboard.Charts[1].Metrics, "0.99", 120)
}

func TestGetDashboardFromKialiNamespace(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	service, prom := setupService("my-namespace", []dashboards.MonitoringDashboard{*fakeDashboard("1")})

	expectedLabels := "{kubernetes_namespace=\"my-namespace\",APP=\"my-app\"}"
	namespace := models.Namespace{
		Name: "my-namespace",
	}
	query := models.DashboardQuery{
		Namespace: namespace.Name,
		LabelsFilters: map[string]string{
			"APP": "my-app",
		},
	}
	query.FillDefaults()
	prom.MockMetric("my_metric_1_1", expectedLabels, &query.RangeQuery, 10)
	prom.MockHistogram("my_metric_1_2", expectedLabels, &query.RangeQuery, 11, 12)

	dashboard, err := service.GetDashboard(&api.AuthInfo{Token: ""}, query, "dashboard1")

	assert.Nil(err)
	assert.Equal("Dashboard 1", dashboard.Title)
}

func TestGetComposedDashboard(t *testing.T) {
	assert := assert.New(t)

	composed := fakeDashboard("2")
	composed.Items = append(composed.Items, dashboards.MonitoringDashboardItem{Include: "dashboard1"})

	// Setup mocks
	service, _ := setupService("my-namespace", []dashboards.MonitoringDashboard{*fakeDashboard("1"), *composed})

	d, err := service.loadAndResolveDashboardResource("dashboard2", map[string]bool{})
	assert.Nil(err)
	assert.Equal("Dashboard 2", d.Title)
	assert.Len(d.Items, 4)
	assert.Equal("My chart 2_1", d.Items[0].Chart.Name)
	assert.Equal("My chart 2_2", d.Items[1].Chart.Name)
	assert.Equal("My chart 1_1", d.Items[2].Chart.Name)
	assert.Equal("My chart 1_2", d.Items[3].Chart.Name)
}

func TestGetComposedDashboardSingleChart(t *testing.T) {
	assert := assert.New(t)

	composed := fakeDashboard("2")
	composed.Items = append(composed.Items, dashboards.MonitoringDashboardItem{Include: "dashboard1$My chart 1_2"})

	// Setup mocks
	service, _ := setupService("my-namespace", []dashboards.MonitoringDashboard{*fakeDashboard("1"), *composed})

	d, err := service.loadAndResolveDashboardResource("dashboard2", map[string]bool{})
	assert.Nil(err)
	assert.Equal("Dashboard 2", d.Title)
	assert.Len(d.Items, 3)
	assert.Equal("My chart 2_1", d.Items[0].Chart.Name)
	assert.Equal("My chart 2_2", d.Items[1].Chart.Name)
	assert.Equal("My chart 1_2", d.Items[2].Chart.Name)
}

func TestCircularDependency(t *testing.T) {
	assert := assert.New(t)

	composed := fakeDashboard("2")
	composed.Items = append(composed.Items, dashboards.MonitoringDashboardItem{Include: "dashboard2"})

	// Setup mocks
	service, _ := setupService("my-namespace", []dashboards.MonitoringDashboard{*fakeDashboard("2"), *composed})

	_, err := service.loadAndResolveDashboardResource("dashboard2", map[string]bool{})
	assert.Contains(err.Error(), "circular dependency detected")
}

func TestDiscoveryMatcher(t *testing.T) {
	assert := assert.New(t)

	d1 := fakeDashboard("1")
	d2 := fakeDashboard("2")
	d3 := fakeDashboard("3")

	dashboards := make(map[string]dashboards.MonitoringDashboard)
	dashboards[d1.Name] = *d1
	dashboards[d2.Name] = *d2
	dashboards[d3.Name] = *d3

	metrics := []string{
		"my_metric_1_1",
		"my_metric_1_2",
		"my_metric_1_3",
		"my_metric_2_1",
	}

	runtimes := runDiscoveryMatcher(metrics, dashboards)

	assert.Len(runtimes, 2)
	assert.Equal("Runtime 1", runtimes[0].Name)
	assert.Len(runtimes[0].DashboardRefs, 1)
	assert.Equal("dashboard1", runtimes[0].DashboardRefs[0].Template)
	assert.Equal("Runtime 2", runtimes[1].Name)
	assert.Len(runtimes[1].DashboardRefs, 1)
	assert.Equal("dashboard2", runtimes[1].DashboardRefs[0].Template)
}

func TestDiscoveryMatcherWithComposition(t *testing.T) {
	assert := assert.New(t)

	d1 := fakeDashboard("1")
	d2 := fakeDashboard("2")
	d2.Items = append(d2.Items, dashboards.MonitoringDashboardItem{Include: d1.Name})
	d3 := fakeDashboard("3")

	dashboards := make(map[string]dashboards.MonitoringDashboard)
	dashboards[d1.Name] = *d1
	dashboards[d2.Name] = *d2
	dashboards[d3.Name] = *d3

	metrics := []string{
		"my_metric_1_1",
		"my_metric_1_2",
		"my_metric_1_3",
		"my_metric_2_1",
	}

	runtimes := runDiscoveryMatcher(metrics, dashboards)

	// Only top-level runtime must appear
	assert.Len(runtimes, 1)
	assert.Equal("Runtime 2", runtimes[0].Name)
	assert.Len(runtimes[0].DashboardRefs, 1)
	assert.Equal("dashboard2", runtimes[0].DashboardRefs[0].Template)
}

func TestGetCustomDashboardRefs(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	service, prom := setupService("my-namespace", []dashboards.MonitoringDashboard{*fakeDashboard("1"), *fakeDashboard("2")})

	prom.MockMetricsForLabels([]string{"my_metric_1_1", "request_count", "tcp_received", "tcp_sent"})
	pods := []*models.Pod{}

	runtimes := service.GetCustomDashboardRefs("my-namespace", "app", "", pods)

	prom.AssertNumberOfCalls(t, "GetMetricsForLabels", 1)
	assert.Len(runtimes, 1)
	assert.Equal("Runtime 1", runtimes[0].Name)
	assert.Len(runtimes[0].DashboardRefs, 1)
	assert.Equal("dashboard1", runtimes[0].DashboardRefs[0].Template)
	assert.Equal("Dashboard 1", runtimes[0].DashboardRefs[0].Title)
}

func fakeDashboard(id string) *dashboards.MonitoringDashboard {
	return &dashboards.MonitoringDashboard{
		Name:       "dashboard" + id,
		Title:      "Dashboard " + id,
		Runtime:    "Runtime " + id,
		DiscoverOn: "my_metric_" + id + "_1",
		Items: []dashboards.MonitoringDashboardItem{
			{
				Chart: fakeChart(id+"_1", dashboards.Rate),
			},
			{
				Chart: fakeChart(id+"_2", dashboards.Histogram),
			},
		},
	}
}

func fakeChart(id string, dataType string) dashboards.MonitoringDashboardChart {
	return dashboards.MonitoringDashboardChart{
		Name:      "My chart " + id,
		Unit:      "s",
		UnitScale: 10.0,
		Spans:     6,
		Metrics:   []dashboards.MonitoringDashboardMetric{{DisplayName: "My chart " + id, MetricName: "my_metric_" + id}},
		DataType:  dataType,
		Aggregations: []dashboards.MonitoringDashboardAggregation{
			{
				DisplayName: "Agg " + id,
				Label:       "agg_" + id,
			},
		},
	}
}

func TestBuildIstioDashboard(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	conf := config.NewConfig()
	config.Set(conf)
	ns := models.Namespace{Name: "my-namespace"}
	service := NewDashboardsService(&ns, nil)

	dashboard := service.BuildIstioDashboard(fakeMetrics(), "inbound")

	assert.Equal("Inbound Metrics", dashboard.Title)
	assert.Len(dashboard.Aggregations, 7)
	assert.Equal("Local version", dashboard.Aggregations[0].DisplayName)
	assert.Equal("destination_canonical_revision", dashboard.Aggregations[0].Label)
	assert.Equal("Remote namespace", dashboard.Aggregations[1].DisplayName)
	assert.Equal("source_workload_namespace", dashboard.Aggregations[1].Label)
	assert.Equal("Remote app", dashboard.Aggregations[2].DisplayName)
	assert.Equal("source_canonical_service", dashboard.Aggregations[2].Label)
	assert.Len(dashboard.Charts, 8)
	assert.Equal("Request volume", dashboard.Charts[0].Name)
	assert.Len(dashboard.Charts[0].Metrics, 1)
	assert.Equal("Request duration", dashboard.Charts[1].Name)
	assert.Len(dashboard.Charts[1].Metrics, 2)
	assert.Equal("TCP sent", dashboard.Charts[7].Name)
	assert.Equal(float64(10), dashboard.Charts[0].Metrics[0].Datapoints[0].Value) // Request volume (request_count)
	assert.Equal(float64(20), dashboard.Charts[1].Metrics[0].Datapoints[0].Value) // Request duration (request_duration_millis)
	assert.Equal(float64(13), dashboard.Charts[7].Metrics[0].Datapoints[0].Value) // TCP sent (tcp_sent)
	// Absent metrics are not nil
	assert.Equal("Request throughput", dashboard.Charts[2].Name)
	assert.NotNil(dashboard.Charts[2].Metrics)
	assert.Len(dashboard.Charts[2].Metrics, 0)
}

func fakeCounter(value float64) []models.Metric {
	return []models.Metric{{
		Labels:     map[string]string{},
		Datapoints: []models.Datapoint{{Timestamp: 0, Value: value}},
	}}
}

func fakeHistogram(avg, p99 float64) []models.Metric {
	return []models.Metric{{
		Stat:       "0.99",
		Datapoints: []models.Datapoint{{Timestamp: 0, Value: p99}},
	}, {
		Stat:       "avg",
		Datapoints: []models.Datapoint{{Timestamp: 0, Value: avg}},
	}}
}

func fakeMetrics() models.MetricsMap {
	return models.MetricsMap{
		"request_count":           fakeCounter(10),
		"request_error_count":     fakeCounter(11),
		"tcp_received":            fakeCounter(12),
		"tcp_sent":                fakeCounter(13),
		"request_duration_millis": fakeHistogram(20, 20),
		"request_size":            fakeHistogram(21, 21),
		"response_size":           fakeHistogram(22, 22),
	}
}
