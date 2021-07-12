package dashboards

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/kiali/kiali/log"
)

const (
	// DashboardTemplateAnnotation is the key name of the namespace annotation for dashboard templates
	DashboardTemplateAnnotation string = "dashboards.kiali.io/templates"
	// Raw constant for DataType
	Raw = "raw"
	// Rate constant for DataType
	Rate = "rate"
	// Histogram constant for DataType
	Histogram = "histogram"
)

type MonitoringDashboardsList []MonitoringDashboard

type MonitoringDashboard struct {
	Name          string                            `yaml:"name"`
	Title         string                            `yaml:"title"`
	Runtime       string                            `yaml:"runtime"`
	DiscoverOn    string                            `yaml:"discoverOn"`
	Items         []MonitoringDashboardItem         `yaml:"items"`
	ExternalLinks []MonitoringDashboardExternalLink `yaml:"externalLinks"`
}

type MonitoringDashboardItem struct {
	// Items are exclusive: either Include or Chart must be set (if both are set, Chart will be ignored)
	// Include is a reference to another dashboard and/or chart
	// Ex: "microprofile-1.0" will include the whole dashboard named "microprofile-1.0" at this position
	//		 "microprofile-1.0$Thread count" will include only the chart named "Thread count" from that dashboard at this position
	Include string                   `yaml:"include"`
	Chart   MonitoringDashboardChart `yaml:"chart"`
}

type MonitoringDashboardChart struct {
	Name             string                           `yaml:"name"`
	Unit             string                           `yaml:"unit"`      // Stands for the base unit (regardless its scale in datasource)
	UnitScale        float64                          `yaml:"unitScale"` // Stands for the scale of the values in datasource, related to the base unit provided. E.g. unit: "seconds" and unitScale: 0.001 means that values in datasource are actually in milliseconds.
	Spans            int                              `yaml:"spans"`
	StartCollapsed   bool                             `yaml:"startCollapsed"`
	ChartType        *string                          `yaml:"chartType"`
	Min              *int                             `yaml:"min"`
	Max              *int                             `yaml:"max"`
	MetricName       string                           `yaml:"metricName"` // Deprecated; use Metrics instead
	Metrics          []MonitoringDashboardMetric      `yaml:"metrics"`
	DataType         string                           `yaml:"dataType"`   // DataType is either "raw", "rate" or "histogram"
	Aggregator       string                           `yaml:"aggregator"` // Aggregator can be set for raw data. Ex: "sum", "avg". See https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators
	Aggregations     []MonitoringDashboardAggregation `yaml:"aggregations"`
	XAxis            *string                          `yaml:"xAxis"`            // "time" (default) or "series"
	GroupLabels      []string                         `yaml:"groupLabels"`      // Prometheus label to be used for grouping; Similar to Aggregations, except this grouping will be always turned on
	SortLabel        string                           `yaml:"sortLabel"`        // Prometheus label to be used for sorting
	SortLabelParseAs string                           `yaml:"sortLabelParseAs"` // Set "int" if the SortLabel needs to be parsed and compared as an integer
}

type MonitoringDashboardMetric struct {
	MetricName  string `yaml:"metricName"`
	DisplayName string `yaml:"displayName"`
}

type MonitoringDashboardAggregation struct {
	Label           string `yaml:"label"`
	DisplayName     string `yaml:"displayName"`
	SingleSelection bool   `yaml:"singleSelection"`
}

type MonitoringDashboardExternalLink struct {
	Type      string                                   `yaml:"type"`
	Name      string                                   `yaml:"name"`
	Variables MonitoringDashboardExternalLinkVariables `yaml:"variables"`
}

type MonitoringDashboardExternalLinkVariables struct {
	Namespace string `yaml:"namespace,omitempty"`
	App       string `yaml:"app,omitempty"`
	Service   string `yaml:"service,omitempty"`
	Version   string `yaml:"version,omitempty"`
	Workload  string `yaml:"workload,omitempty"`
}

// GetMetrics provides consistent MonitoringDashboardMetric slice in a backward-compatible way, if deprecated field MetricName is used instead of Metrics in Spec.
func (in *MonitoringDashboardChart) GetMetrics() []MonitoringDashboardMetric {
	if len(in.Metrics) == 0 {
		return []MonitoringDashboardMetric{
			{
				MetricName:  in.MetricName,
				DisplayName: in.Name,
			},
		}
	}
	return in.Metrics
}

// AddMonitoringDashboards adds additional dashboards to the original list.
// If a name of a new dashboard already exists in orig, it replaces the original.
// Note that the returns list is also stripped of dashboards that should be disabled.
// A dashboard is disabled if "DiscoverOn" is an empty string and "Items" is an empty list.
func AddMonitoringDashboards(orig MonitoringDashboardsList, additional MonitoringDashboardsList) MonitoringDashboardsList {
	if additional == nil || orig == nil {
		return nil
	}
	allDashboardsMap := orig.OrganizeByName()
	for _, a := range additional {
		allDashboardsMap[a.Name] = *((&a).deepCopy())
	}

	newList := make([]MonitoringDashboard, 0, len(allDashboardsMap))
	for _, val := range allDashboardsMap {
		if strings.TrimSpace(val.DiscoverOn) != "" || len(val.Items) > 0 {
			newList = append(newList, val)
		}
	}
	return MonitoringDashboardsList(newList)
}

func GetBuiltInMonitoringDashboards() MonitoringDashboardsList {
	if v, err := unmarshal(DEFAULT_DASHBOARDS_YAML); err == nil {
		return MonitoringDashboardsList(v)
	} else {
		log.Errorf("Failed to unmarshal built-in dashboard yaml - this is a bug, please report it. err=%v", err)
		empty := make([]MonitoringDashboard, 0)
		return MonitoringDashboardsList(empty)
	}
}

// GetNamespaceMonitoringDashboards will examine the given namespace annotations and return any dashboard yaml found.
func GetNamespaceMonitoringDashboards(namespace string, annotations map[string]string) MonitoringDashboardsList {
	if dashboardYaml, ok := annotations[DashboardTemplateAnnotation]; ok {
		if v, err := unmarshal(dashboardYaml); err == nil {
			return MonitoringDashboardsList(v)
		} else {
			log.Errorf("Failed to unmarshall yaml dashboard in namespace %s. err=%v", namespace, err)
		}
	}
	empty := make([]MonitoringDashboard, 0)
	return MonitoringDashboardsList(empty)
}

// GetWorkloadMonitoringDashboards will examine the given namespace annotations and return any dashboard yaml found.
func GetWorkloadMonitoringDashboards(namespace string, workload string, annotations map[string]string) MonitoringDashboardsList {
	if dashboardYaml, ok := annotations[DashboardTemplateAnnotation]; ok {
		if v, err := unmarshal(dashboardYaml); err == nil {
			return MonitoringDashboardsList(v)
		} else {
			log.Errorf("Failed to unmarshall yaml dashboard in namespace %s and workload %s. err=%v", namespace, workload, err)
		}
	}
	empty := make([]MonitoringDashboard, 0)
	return MonitoringDashboardsList(empty)
}

// OrganizeByName returns a map with the key being the names of the dashboards; values are the dashboards themselves
func (in *MonitoringDashboardsList) OrganizeByName() map[string]MonitoringDashboard {
	out := make(map[string]MonitoringDashboard, len(*in))
	for _, i := range *in {
		out[i.Name] = i
	}
	return out
}

// Unmarshal parses the given YAML string and returns its MonitoringDashboardsList object representation.
func unmarshal(yamlString string) (out MonitoringDashboardsList, err error) {
	list := new(MonitoringDashboardsList)
	err = yaml.Unmarshal([]byte(yamlString), &list)
	if err != nil {
		return nil, fmt.Errorf("failed to parse monitoring dashboard yaml data. error=%v", err)
	}
	return *list, nil
}

func (in *MonitoringDashboard) deepCopyInto(out *MonitoringDashboard) {
	*out = *in
}

func (in *MonitoringDashboard) deepCopy() *MonitoringDashboard {
	if in == nil {
		return nil
	}
	out := new(MonitoringDashboard)
	in.deepCopyInto(out)
	return out
}

func (in *MonitoringDashboardsList) deepCopyInto(out MonitoringDashboardsList) {
	for i := range *in {
		(*in)[i].deepCopyInto(&out[i])
	}
}

func (in *MonitoringDashboardsList) DeepCopy() *MonitoringDashboardsList {
	if in == nil {
		return nil
	}
	out := make([]MonitoringDashboard, len(*in))
	in.deepCopyInto(out)
	return (*MonitoringDashboardsList)(&out)
}
