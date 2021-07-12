package business

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/config/dashboards"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/prometheus"
)

const defaultNamespaceLabel = "namespace"

// DashboardsService deals with fetching dashboards from config
type DashboardsService struct {
	promClient      prometheus.ClientInterface
	dashboards      map[string]dashboards.MonitoringDashboard
	promConfig      config.PrometheusConfig
	globalNamespace string
	namespaceLabel  string
	CustomEnabled   bool
}

// NewDashboardsService initializes this business service
func NewDashboardsService(namespace *models.Namespace, workload *models.Workload) *DashboardsService {
	cfg := config.Get()
	customEnabled := cfg.ExternalServices.CustomDashboards.Enabled
	prom := cfg.ExternalServices.Prometheus
	if customEnabled && cfg.ExternalServices.CustomDashboards.Prometheus.URL != "" {
		prom = cfg.ExternalServices.CustomDashboards.Prometheus
	}
	nsLabel := cfg.ExternalServices.CustomDashboards.NamespaceLabel
	if nsLabel == "" {
		nsLabel = "kubernetes_namespace"
	}

	// Overwrite Custom dashboards defined at Namespace level
	builtInDashboards := cfg.CustomDashboards
	if namespace != nil {
		nsDashboards := dashboards.GetNamespaceMonitoringDashboards(namespace.Name, namespace.Annotations)
		builtInDashboards = dashboards.AddMonitoringDashboards(builtInDashboards, nsDashboards)
	}
	if workload != nil {
		wkDashboards := dashboards.GetWorkloadMonitoringDashboards(namespace.Name, workload.Name, workload.DashboardAnnotations)
		builtInDashboards = dashboards.AddMonitoringDashboards(builtInDashboards, wkDashboards)
	}

	return &DashboardsService{
		CustomEnabled:   customEnabled,
		promConfig:      prom,
		globalNamespace: cfg.Deployment.Namespace,
		namespaceLabel:  nsLabel,
		dashboards:      builtInDashboards.OrganizeByName(),
	}
}

func (in *DashboardsService) prom() (prometheus.ClientInterface, error) {
	// Lazy init
	if in.promClient == nil {
		client, err := prometheus.NewClientForConfig(in.promConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize Prometheus Client: %v", err)
		}
		in.promClient = client
	}
	return in.promClient, nil
}

func (in *DashboardsService) loadRawDashboardResource(template string) (*dashboards.MonitoringDashboard, error) {
	dashboard, ok := in.dashboards[template]
	if !ok {
		return nil, fmt.Errorf("Dashboard [%v] does not exist or is disabled", template)
	}

	return &dashboard, nil
}

func (in *DashboardsService) loadAndResolveDashboardResource(template string, loaded map[string]bool) (*dashboards.MonitoringDashboard, error) {
	// Circular dependency check
	if _, ok := loaded[template]; ok {
		return nil, fmt.Errorf("cannot load dashboard %s due to circular dependency detected. Already loaded dependencies: %v", template, loaded)
	}
	loaded[template] = true
	dashboard, err := in.loadRawDashboardResource(template)
	if err != nil {
		return nil, err
	}
	err = in.resolveReferences(dashboard, loaded)
	return dashboard, err
}

// resolveReferences resolves the composition mechanism that allows to reference a dashboard from another one
func (in *DashboardsService) resolveReferences(dashboard *dashboards.MonitoringDashboard, loaded map[string]bool) error {
	resolved := []dashboards.MonitoringDashboardItem{}
	for _, item := range dashboard.Items {
		reference := strings.TrimSpace(item.Include)
		if reference != "" {
			// reference can point to a whole dashboard (ex: microprofile-1.0) or a chart within a dashboard (ex: microprofile-1.0$Thread count)
			parts := strings.Split(reference, "$")
			dashboardRefName := parts[0]
			composedDashboard, err := in.loadAndResolveDashboardResource(dashboardRefName, loaded)
			if err != nil {
				return err
			}
			for _, item2 := range composedDashboard.Items {
				if len(parts) > 1 {
					// Reference a specific chart
					if item2.Chart.Name == parts[1] {
						resolved = append(resolved, item2)
						break
					}
				} else {
					// Reference the whole dashboard
					resolved = append(resolved, item2)
				}
			}
		} else {
			resolved = append(resolved, item)
		}
	}
	dashboard.Items = resolved
	return nil
}

// GetDashboard returns a dashboard filled-in with target data
func (in *DashboardsService) GetDashboard(authInfo *api.AuthInfo, params models.DashboardQuery, template string) (*models.MonitoringDashboard, error) {
	promClient, err := in.prom()
	if err != nil {
		return nil, err
	}

	dashboard, err := in.loadAndResolveDashboardResource(template, map[string]bool{})
	if err != nil {
		return nil, err
	}

	filters := in.buildLabels(params.Namespace, params.LabelsFilters)
	aggLabels := append(params.AdditionalLabels, models.ConvertAggregations(*dashboard)...)
	if len(aggLabels) == 0 {
		// Prevent null in json
		aggLabels = []models.Aggregation{}
	}

	wg := sync.WaitGroup{}
	wg.Add(len(dashboard.Items) + 1)
	filledCharts := make([]models.Chart, len(dashboard.Items))

	for i, item := range dashboard.Items {
		go func(idx int, chart dashboards.MonitoringDashboardChart) {
			defer wg.Done()
			conversionParams := models.ConversionParams{Scale: 1.0, SortLabel: chart.SortLabel, SortLabelParseAs: chart.SortLabelParseAs}
			if chart.UnitScale != 0.0 {
				conversionParams.Scale = chart.UnitScale
			}
			// Group by labels is concat of what is defined in CR + what is passed as parameters
			byLabels := append(chart.GroupLabels, params.ByLabels...)
			if len(chart.SortLabel) > 0 {
				// We also need to group by the label used for sorting, if not explicitly present
				present := false
				for _, lbl := range byLabels {
					if lbl == chart.SortLabel {
						present = true
						break
					}
				}
				if !present {
					byLabels = append(byLabels, chart.SortLabel)
					// Mark the sort label to not be kept during conversion
					conversionParams.RemoveSortLabel = true
				}
			}
			grouping := strings.Join(byLabels, ",")

			filledCharts[idx] = models.ConvertChart(chart)
			metrics := chart.GetMetrics()
			for _, ref := range metrics {
				var converted []models.Metric
				var err error
				if chart.DataType == dashboards.Raw {
					aggregator := params.RawDataAggregator
					if chart.Aggregator != "" {
						aggregator = chart.Aggregator
					}
					metric := promClient.FetchRange(ref.MetricName, filters, grouping, aggregator, &params.RangeQuery)
					converted, err = models.ConvertMetric(ref.DisplayName, metric, conversionParams)
				} else if chart.DataType == dashboards.Rate {
					metric := promClient.FetchRateRange(ref.MetricName, []string{filters}, grouping, &params.RangeQuery)
					converted, err = models.ConvertMetric(ref.DisplayName, metric, conversionParams)
				} else {
					histo := promClient.FetchHistogramRange(ref.MetricName, filters, grouping, &params.RangeQuery)
					converted, err = models.ConvertHistogram(ref.DisplayName, histo, conversionParams)
				}

				// Fill in chart
				if err != nil {
					filledCharts[idx].Error = err.Error()
				} else {
					filledCharts[idx].Metrics = append(filledCharts[idx].Metrics, converted...)
				}
			}
		}(i, item.Chart)
	}

	var externalLinks []models.ExternalLink
	go func() {
		defer wg.Done()
		links, _, err := GetGrafanaLinks(authInfo, dashboard.ExternalLinks)
		if err != nil {
			log.Errorf("Error while getting Grafana links: %v", err)
		}
		if links != nil {
			externalLinks = links
		} else {
			externalLinks = []models.ExternalLink{}
		}
	}()

	wg.Wait()
	return &models.MonitoringDashboard{
		Name:          dashboard.Name,
		Title:         dashboard.Title,
		Charts:        filledCharts,
		Aggregations:  aggLabels,
		ExternalLinks: externalLinks,
	}, nil
}

// SearchExplicitDashboards will check annotations of all supplied pods to extract a unique list of dashboards
//	Accepted annotations are "kiali.io/runtimes" and "kiali.io/dashboards"
func (in *DashboardsService) SearchExplicitDashboards(pods []models.Pod) []models.Runtime {
	uniqueRefsList := extractUniqueDashboards(pods)
	if len(uniqueRefsList) > 0 {
		log.Tracef("getting dashboards from refs list: %v", uniqueRefsList)
		return in.buildRuntimesList(uniqueRefsList)
	}
	return []models.Runtime{}
}

func (in *DashboardsService) buildRuntimesList(templatesNames []string) []models.Runtime {
	dashboards := make([]*dashboards.MonitoringDashboard, len(templatesNames))
	wg := sync.WaitGroup{}
	wg.Add(len(templatesNames))
	for idx, template := range templatesNames {
		go func(i int, tpl string) {
			defer wg.Done()
			dashboard, err := in.loadRawDashboardResource(tpl)
			if err != nil {
				log.Errorf("cannot get dashboard [%s]: %v", tpl, err)
			} else {
				dashboards[i] = dashboard
			}
		}(idx, template)
	}

	wg.Wait()

	runtimes := []models.Runtime{}
	for _, dashboard := range dashboards {
		if dashboard == nil {
			continue
		}
		runtimes = addDashboardToRuntimes(dashboard, runtimes)
	}
	return runtimes
}

func (in *DashboardsService) fetchMetricNames(namespace string, labelsFilters map[string]string) []string {
	promClient, err := in.prom()
	if err != nil {
		return []string{}
	}

	labels := in.buildLabels(namespace, labelsFilters)
	metrics, err := promClient.GetMetricsForLabels([]string{labels})
	if err != nil {
		log.Errorf("custom dashboard discovery failed, cannot load metrics for labels [%s]: %v", labels, err)
	}
	return metrics
}

// discoverDashboards tries to discover dashboards based on existing metrics
func (in *DashboardsService) discoverDashboards(namespace string, labelsFilters map[string]string) []models.Runtime {
	log.Tracef("starting custom dashboard discovery on namespace [%s] with filters [%v]", namespace, labelsFilters)

	var metrics []string
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		metrics = in.fetchMetricNames(namespace, labelsFilters)
	}()

	wg.Wait()
	return runDiscoveryMatcher(metrics, in.dashboards)
}

func runDiscoveryMatcher(metrics []string, allDashboards map[string]dashboards.MonitoringDashboard) []models.Runtime {
	// In all dashboards, finds the ones that match the metrics set
	// We must exclude from the results included dashboards when both the including and the included dashboards are matching
	runtimesMap := make(map[string]*dashboards.MonitoringDashboard)
	for _, d := range allDashboards {
		dashboard := d // sticky reference
		matchReference := strings.TrimSpace(dashboard.DiscoverOn)
		if matchReference != "" {
			for _, metric := range metrics {
				if matchReference == strings.TrimSpace(metric) {
					if _, exists := runtimesMap[dashboard.Name]; !exists {
						runtimesMap[dashboard.Name] = &dashboard
					}
					// Mark included dashboards as already found
					// and set them "nil" to not show them as standalone dashboards even if they match
					for _, item := range dashboard.Items {
						if item.Include != "" {
							runtimesMap[item.Include] = nil
						}
					}
					break
				}
			}
		}
	}
	runtimes := []models.Runtime{}
	for _, dashboard := range runtimesMap {
		if dashboard != nil {
			runtimes = addDashboardToRuntimes(dashboard, runtimes)
		}
	}
	sort.Slice(runtimes, func(i, j int) bool { return runtimes[i].Name < runtimes[j].Name })
	return runtimes
}

func addDashboardToRuntimes(dashboard *dashboards.MonitoringDashboard, runtimes []models.Runtime) []models.Runtime {
	runtime := dashboard.Runtime
	ref := models.DashboardRef{
		Template: dashboard.Name,
		Title:    dashboard.Title,
	}
	found := false
	for i := range runtimes {
		rtObj := &runtimes[i]
		if rtObj.Name == runtime {
			rtObj.DashboardRefs = append(rtObj.DashboardRefs, ref)
			found = true
			break
		}
	}
	if !found {
		runtimes = append(runtimes, models.Runtime{
			Name:          runtime,
			DashboardRefs: []models.DashboardRef{ref},
		})
	}
	return runtimes
}

func (in *DashboardsService) buildLabels(namespace string, labelsFilters map[string]string) string {
	namespaceLabel := in.namespaceLabel
	if namespaceLabel == "" {
		namespaceLabel = defaultNamespaceLabel
	}
	labels := fmt.Sprintf(`{%s="%s"`, namespaceLabel, namespace)
	for k, v := range labelsFilters {
		labels += fmt.Sprintf(`,%s="%s"`, prometheus.SanitizeLabelName(k), v)
	}
	labels += "}"
	return labels
}

type istioChart struct {
	models.Chart
	refName string
	scale   float64
}

func getIstioCharts() []istioChart {
	istioCharts := []istioChart{
		{
			Chart: models.Chart{
				Name:  "Request volume",
				Unit:  "ops",
				Spans: 3,
			},
			refName: "request_count",
		},
		{
			Chart: models.Chart{
				Name:  "Request duration",
				Unit:  "seconds",
				Spans: 3,
			},
			refName: "request_duration_millis",
			scale:   0.001,
		},
		{
			Chart: models.Chart{
				Name:  "Request throughput",
				Unit:  "bitrate",
				Spans: 3,
			},
			refName: "request_throughput",
			scale:   8, // Bps to bps
		},
		{
			Chart: models.Chart{
				Name:  "Request size",
				Unit:  "bytes",
				Spans: 3,
			},
			refName: "request_size",
		},
		{
			Chart: models.Chart{
				Name:  "Response throughput",
				Unit:  "bitrate",
				Spans: 3,
			},
			refName: "response_throughput",
			scale:   8, // Bps to bps
		},
		{
			Chart: models.Chart{
				Name:  "Response size",
				Unit:  "bytes",
				Spans: 3,
			},
			refName: "response_size",
		},
		{
			Chart: models.Chart{
				Name:  "TCP received",
				Unit:  "bitrate",
				Spans: 3,
			},
			refName: "tcp_received",
		},
		{
			Chart: models.Chart{
				Name:  "TCP sent",
				Unit:  "bitrate",
				Spans: 3,
			},
			refName: "tcp_sent",
		},
	}
	return istioCharts
}

func GetIstioScaler() func(name string) float64 {
	charts := getIstioCharts()
	return func(name string) float64 {
		for _, c := range charts {
			if c.refName == name {
				return c.scale
			}
		}
		return 1.0
	}
}

// GetIstioDashboard returns Istio dashboard (currently hard-coded) filled-in with metrics
func (in *DashboardsService) BuildIstioDashboard(metrics models.MetricsMap, direction string) *models.MonitoringDashboard {
	var dashboard models.MonitoringDashboard
	// Copy dashboard
	if direction == "inbound" {
		dashboard = models.PrepareIstioDashboard("Inbound", "destination", "source")
	} else {
		dashboard = models.PrepareIstioDashboard("Outbound", "source", "destination")
	}

	istioCharts := getIstioCharts()

	for _, chartTpl := range istioCharts {
		newChart := chartTpl.Chart
		conversionParams := models.ConversionParams{Scale: 1.0}
		if chartTpl.scale != 0.0 {
			conversionParams.Scale = chartTpl.scale
		}
		if metrics := metrics[chartTpl.refName]; metrics != nil {
			newChart.Metrics = metrics
		} else {
			newChart.Metrics = []models.Metric{}
		}
		dashboard.Charts = append(dashboard.Charts, newChart)
	}
	return &dashboard
}

// GetCustomDashboardRefs finds all dashboard IDs and Titles associated to this app and add them to the model
func (in *DashboardsService) GetCustomDashboardRefs(namespace, app, version string, pods []*models.Pod) []models.Runtime {
	if !in.CustomEnabled || app == "" {
		// Custom dashboards are disabled or the app label is not configured
		return []models.Runtime{}
	}

	// A better way to do?
	var podsCast []models.Pod
	for _, p := range pods {
		podsCast = append(podsCast, *p)
	}
	runtimes := in.SearchExplicitDashboards(podsCast)

	if len(runtimes) == 0 {
		cfg := config.Get()
		discoveryEnabled := cfg.ExternalServices.CustomDashboards.DiscoveryEnabled
		if discoveryEnabled == config.DashboardsDiscoveryEnabled ||
			(discoveryEnabled == config.DashboardsDiscoveryAuto &&
				len(pods) <= cfg.ExternalServices.CustomDashboards.DiscoveryAutoThreshold) {
			filters := make(map[string]string)
			filters[cfg.IstioLabels.AppLabelName] = app
			if version != "" {
				filters[cfg.IstioLabels.VersionLabelName] = version
			}
			runtimes = in.discoverDashboards(namespace, filters)
		}
	}
	return runtimes
}

func extractUniqueDashboards(pods []models.Pod) []string {
	// Get uniqueness from plain list rather than map to preserve ordering; anyway, very low amount of objects is expected
	uniqueRefs := []string{}
	for _, pod := range pods {
		// Check for custom dashboards annotation
		dashboards := extractDashboardsFromAnnotation(pod, "kiali.io/runtimes")
		dashboards = append(dashboards, extractDashboardsFromAnnotation(pod, "kiali.io/dashboards")...)
		for _, ref := range dashboards {
			if ref != "" {
				exists := false
				for _, existingRef := range uniqueRefs {
					if ref == existingRef {
						exists = true
						break
					}
				}
				if !exists {
					uniqueRefs = append(uniqueRefs, ref)
				}
			}
		}
	}
	return uniqueRefs
}

func extractDashboardsFromAnnotation(pod models.Pod, annotation string) []string {
	dashboards := []string{}
	if rawDashboards, ok := pod.Annotations[annotation]; ok {
		rawDashboardsSlice := strings.Split(rawDashboards, ",")
		for _, dashboard := range rawDashboardsSlice {
			dashboards = append(dashboards, strings.TrimSpace(dashboard))
		}
	}
	return dashboards
}
