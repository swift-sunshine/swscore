package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/kiali/kiali/config/dashboards"
	"github.com/kiali/kiali/config/security"
	"github.com/kiali/kiali/log"
)

// Environment variables that can override the ConfigMap yaml values
const (
	// External services auth
	EnvGrafanaPassword    = "GRAFANA_PASSWORD"
	EnvGrafanaToken       = "GRAFANA_TOKEN"
	EnvPrometheusPassword = "PROMETHEUS_PASSWORD"
	EnvPrometheusToken    = "PROMETHEUS_TOKEN"
	EnvTracingPassword    = "TRACING_PASSWORD"
	EnvTracingToken       = "TRACING_TOKEN"

	// Login Token signing key used to prepare the token for user login
	EnvLoginTokenSigningKey = "LOGIN_TOKEN_SIGNING_KEY"
)

// The versions that Kiali requires
const (
	IstioVersionSupported   = ">= 1.0"
	MaistraVersionSupported = ">= 0.7.0"
	OSSMVersionSupported    = ">= 1.0"
	Iter8VersionSupported   = ">= 0.2"
)

// The valid auth strategies and values for cookie handling
const (
	AuthStrategyOpenshift = "openshift"
	AuthStrategyAnonymous = "anonymous"
	AuthStrategyToken     = "token"
	AuthStrategyOpenId    = "openid"
	AuthStrategyHeader    = "header"

	TokenCookieName             = "kiali-token"
	AuthStrategyOpenshiftIssuer = "kiali-openshift"
	AuthStrategyTokenIssuer     = "kiali-token"
	AuthStrategyOpenIdIssuer    = "kiali-open-id"
	AuthStrategyHeaderIssuer    = "kiali-header"

	// These constants are used for external services auth (Prometheus, Grafana ...) ; not for Kiali auth
	AuthTypeBasic  = "basic"
	AuthTypeBearer = "bearer"
	AuthTypeNone   = "none"
)

const (
	IstioMultiClusterHostSuffix = "global"
	OidcClientSecretFile        = "/kiali-secret/oidc-secret"
)

const (
	DashboardsDiscoveryEnabled = "true"
	DashboardsDiscoveryAuto    = "auto"
)

// Global configuration for the application.
var configuration Config
var rwMutex sync.RWMutex

// Server configuration
type Server struct {
	Address                    string `yaml:",omitempty"`
	AuditLog                   bool   `yaml:"audit_log,omitempty"` // When true, allows additional audit logging on Write operations
	CORSAllowAll               bool   `yaml:"cors_allow_all,omitempty"`
	GzipEnabled                bool   `yaml:"gzip_enabled,omitempty"`
	MetricsEnabled             bool   `yaml:"metrics_enabled,omitempty"`
	MetricsPort                int    `yaml:"metrics_port,omitempty"`
	Port                       int    `yaml:",omitempty"`
	StaticContentRootDirectory string `yaml:"static_content_root_directory,omitempty"`
	WebFQDN                    string `yaml:"web_fqdn,omitempty"`
	WebPort                    string `yaml:"web_port,omitempty"`
	WebRoot                    string `yaml:"web_root,omitempty"`
	WebHistoryMode             string `yaml:"web_history_mode,omitempty"`
	WebSchema                  string `yaml:"web_schema,omitempty"`
}

// Auth provides authentication data for external services
type Auth struct {
	CAFile             string `yaml:"ca_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	Password           string `yaml:"password"`
	Token              string `yaml:"token"`
	Type               string `yaml:"type"`
	UseKialiToken      bool   `yaml:"use_kiali_token"`
	Username           string `yaml:"username"`
}

func (a *Auth) Obfuscate() {
	a.Token = "xxx"
	a.Password = "xxx"
	a.Username = "xxx"
	a.CAFile = "xxx"
}

// PrometheusConfig describes configuration of the Prometheus component
type PrometheusConfig struct {
	Auth Auth `yaml:"auth,omitempty"`
	// Cache duration per query expressed in seconds
	CacheDuration int `yaml:"cache_duration,omitempty"`
	// Enable cache for Prometheus queries
	CacheEnabled bool `yaml:"cache_enabled,omitempty"`
	// Global cache expiration expressed in seconds
	CacheExpiration int    `yaml:"cache_expiration,omitempty"`
	HealthCheckUrl  string `yaml:"health_check_url,omitempty"`
	IsCore          bool   `yaml:"is_core,omitempty"`
	URL             string `yaml:"url,omitempty"`
}

// CustomDashboardsConfig describes configuration specific to Custom Dashboards
type CustomDashboardsConfig struct {
	DiscoveryEnabled       string           `yaml:"discovery_enabled,omitempty"`
	DiscoveryAutoThreshold int              `yaml:"discovery_auto_threshold,omitempty"`
	Enabled                bool             `yaml:"enabled,omitempty"`
	IsCore                 bool             `yaml:"is_core,omitempty"`
	NamespaceLabel         string           `yaml:"namespace_label,omitempty"`
	Prometheus             PrometheusConfig `yaml:"prometheus,omitempty"`
}

// GrafanaConfig describes configuration used for Grafana links
type GrafanaConfig struct {
	Auth           Auth                     `yaml:"auth"`
	Dashboards     []GrafanaDashboardConfig `yaml:"dashboards"`
	Enabled        bool                     `yaml:"enabled"` // Enable or disable Grafana support in Kiali
	HealthCheckUrl string                   `yaml:"health_check_url,omitempty"`
	InClusterURL   string                   `yaml:"in_cluster_url"`
	IsCore         bool                     `yaml:"is_core,omitempty"`
	URL            string                   `yaml:"url"`
}

type GrafanaDashboardConfig struct {
	Name      string                 `yaml:"name"`
	Variables GrafanaVariablesConfig `yaml:"variables"`
}

type GrafanaVariablesConfig struct {
	App       string `yaml:"app" json:"app,omitempty"`
	Namespace string `yaml:"namespace" json:"namespace,omitempty"`
	Service   string `yaml:"service" json:"service,omitempty"`
	Version   string `yaml:"version" json:"version,omitempty"`
	Workload  string `yaml:"workload" json:"workload,omitempty"`
}

// TracingConfig describes configuration used for tracing links
type TracingConfig struct {
	Auth                 Auth     `yaml:"auth"`
	Enabled              bool     `yaml:"enabled"` // Enable Jaeger in Kiali
	HealthCheckUrl       string   `yaml:"health_check_url,omitempty"`
	InClusterURL         string   `yaml:"in_cluster_url"`
	IsCore               bool     `yaml:"is_core,omitempty"`
	NamespaceSelector    bool     `yaml:"namespace_selector"`
	URL                  string   `yaml:"url"`
	UseGRPC              bool     `yaml:"use_grpc"`
	WhiteListIstioSystem []string `yaml:"whitelist_istio_system"`
}

// IstioConfig describes configuration used for istio links
type IstioConfig struct {
	ComponentStatuses                 ComponentStatuses `yaml:"component_status,omitempty"`
	ConfigMapName                     string            `yaml:"config_map_name,omitempty"`
	EnvoyAdminLocalPort               int               `yaml:"envoy_admin_local_port,omitempty"`
	IstioIdentityDomain               string            `yaml:"istio_identity_domain,omitempty"`
	IstioInjectionAnnotation          string            `yaml:"istio_injection_annotation,omitempty"`
	IstioSidecarInjectorConfigMapName string            `yaml:"istio_sidecar_injector_config_map_name,omitempty"`
	IstioSidecarAnnotation            string            `yaml:"istio_sidecar_annotation,omitempty"`
	IstiodDeploymentName              string            `yaml:"istiod_deployment_name,omitempty"`
	UrlServiceVersion                 string            `yaml:"url_service_version"`
}

type ComponentStatuses struct {
	Enabled    bool              `yaml:"enabled,omitempty"`
	Components []ComponentStatus `yaml:"components,omitempty"`
}

type ComponentStatus struct {
	AppLabel  string `yaml:"app_label,omitempty"`
	IsCore    bool   `yaml:"is_core,omitempty"`
	IsProxy   bool   `yaml:"is_proxy,omitempty"`
	Namespace string `yaml:"namespace,omitempty"`
}

type Iter8Config struct {
	Enabled bool `yaml:"enabled"`
	// Define which namespace Iter8 is installed on, default to iter8
	Namespace string `yaml:"namespace"`
}

// Extensions struct describes configuration for Kiali add-ons (extensions)
// New add-on/extension configuration should create a specific config and be located under this
type Extensions struct {
	Iter8 Iter8Config `yaml:"iter_8,omitempty"`
}

// ExternalServices holds configurations for other systems that Kiali depends on
type ExternalServices struct {
	Grafana          GrafanaConfig          `yaml:"grafana,omitempty"`
	Istio            IstioConfig            `yaml:"istio,omitempty"`
	Prometheus       PrometheusConfig       `yaml:"prometheus,omitempty"`
	CustomDashboards CustomDashboardsConfig `yaml:"custom_dashboards,omitempty"`
	Tracing          TracingConfig          `yaml:"tracing,omitempty"`
}

// LoginToken holds config used for generating the Kiali session tokens.
type LoginToken struct {
	ExpirationSeconds int64  `yaml:"expiration_seconds,omitempty"`
	SigningKey        string `yaml:"signing_key,omitempty"`
}

func (lt *LoginToken) Obfuscate() {
	lt.SigningKey = "xxx"
}

// IstioLabels holds configuration about the labels required by Istio
type IstioLabels struct {
	AppLabelName       string `yaml:"app_label_name,omitempty" json:"appLabelName"`
	InjectionLabelName string `yaml:"injection_label,omitempty" json:"injectionLabelName"`
	VersionLabelName   string `yaml:"version_label_name,omitempty" json:"versionLabelName"`
}

// AdditionalDisplayItem holds some display-related configuration, like which annotations are to be displayed
type AdditionalDisplayItem struct {
	Annotation     string `yaml:"annotation"`
	IconAnnotation string `yaml:"icon_annotation"`
	Title          string `yaml:"title"`
}

// KubernetesConfig holds the k8s client, caching and performance configuration
type KubernetesConfig struct {
	Burst int `yaml:"burst,omitempty"`
	// Cache duration expressed in seconds
	// Cache uses watchers to sync with the backend, after a CacheDuration watchers are closed and re-opened
	CacheDuration int `yaml:"cache_duration,omitempty"`
	// Enable cache for kubernetes and istio resources
	CacheEnabled bool `yaml:"cache_enabled,omitempty"`
	// Kiali can cache VirtualService,DestinationRule,Gateway and ServiceEntry Istio resources if they are present
	// on this list of Istio types. Other Istio types are not yet supported.
	CacheIstioTypes []string `yaml:"cache_istio_types,omitempty"`
	// List of namespaces or regex defining namespaces to include in a cache
	CacheNamespaces []string `yaml:"cache_namespaces,omitempty"`
	// Cache duration expressed in seconds
	// Kiali cache list of namespaces per user, this is typically short lived cache compared with the duration of the
	// namespace cache defined by previous CacheDuration parameter
	CacheTokenNamespaceDuration int `yaml:"cache_token_namespace_duration,omitempty"`
	// List of controllers that won't be used for Workload calculation
	// Kiali queries Deployment,ReplicaSet,ReplicationController,DeploymentConfig,StatefulSet,Job and CronJob controllers
	// Deployment and ReplicaSet will be always queried, but ReplicationController,DeploymentConfig,StatefulSet,Job and CronJobs
	// can be skipped from Kiali workloads query if they are present in this list
	ExcludeWorkloads []string `yaml:"excluded_workloads,omitempty"`
	QPS              float32  `yaml:"qps,omitempty"`
}

// ApiConfig contains API specific configuration.
type ApiConfig struct {
	Namespaces ApiNamespacesConfig
}

// ApiNamespacesConfig provides a list of regex strings defining namespaces to blacklist.
type ApiNamespacesConfig struct {
	Exclude       []string
	LabelSelector string `yaml:"label_selector,omitempty" json:"labelSelector"`
}

// AuthConfig provides details on how users are to authenticate
type AuthConfig struct {
	OpenId    OpenIdConfig    `yaml:"openid,omitempty"`
	OpenShift OpenShiftConfig `yaml:"openshift,omitempty"`
	Strategy  string          `yaml:"strategy,omitempty"`
}

// OpenShiftConfig contains specific configuration for authentication when on OpenShift
type OpenShiftConfig struct {
	ClientIdPrefix string `yaml:"client_id_prefix,omitempty"`
}

// OpenIdConfig contains specific configuration for authentication using an OpenID provider
type OpenIdConfig struct {
	AdditionalRequestParams map[string]string `yaml:"additional_request_params,omitempty"`
	ApiProxy                string            `yaml:"api_proxy,omitempty"`
	ApiProxyCAData          string            `yaml:"api_proxy_ca_data,omitempty"`
	AuthenticationTimeout   int               `yaml:"authentication_timeout,omitempty"`
	ApiToken                string            `yaml:"api_token,omitempty"`
	AuthorizationEndpoint   string            `yaml:"authorization_endpoint,omitempty"`
	ClientId                string            `yaml:"client_id,omitempty"`
	ClientSecret            string            `yaml:"client_secret,omitempty"`
	DisableRBAC             bool              `yaml:"disable_rbac,omitempty"`
	HTTPProxy               string            `yaml:"http_proxy,omitempty"`
	HTTPSProxy              string            `yaml:"https_proxy,omitempty"`
	InsecureSkipVerifyTLS   bool              `yaml:"insecure_skip_verify_tls,omitempty"`
	IssuerUri               string            `yaml:"issuer_uri,omitempty"`
	Scopes                  []string          `yaml:"scopes,omitempty"`
	UsernameClaim           string            `yaml:"username_claim,omitempty"`
}

// DeploymentConfig provides details on how Kiali was deployed.
type DeploymentConfig struct {
	AccessibleNamespaces []string `yaml:"accessible_namespaces"`
	Namespace            string   `yaml:"namespace,omitempty"` // Kiali deployment namespace
}

// GraphFindOption defines a single Graph Find/Hide Option
type GraphFindOption struct {
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Expression  string `yaml:"expression,omitempty" json:"expression,omitempty"`
}

// GraphTraffic defines the protocol-specific rates used to determine traffic for graph generation.
// grpc options : none | sent (messages) | received (messages) | requests (default) | total (messages)
// http options : none | requests (default)
// tcp options  : none | sent (bytes, default) | received (bytes) | total (bytes)
type GraphTraffic struct {
	Grpc string `yaml:"grpc,omitempty" json:"grpc,omitempty"`
	Http string `yaml:"http,omitempty" json:"http,omitempty"`
	Tcp  string `yaml:"tcp,omitempty" json:"tcp,omitempty"`
}

// GraphUIDefaults defines UI Defaults specific to the UI Graph
type GraphUIDefaults struct {
	FindOptions []GraphFindOption `yaml:"find_options,omitempty" json:"findOptions,omitempty"`
	HideOptions []GraphFindOption `yaml:"hide_options,omitempty" json:"hideOptions,omitempty"`
	Traffic     GraphTraffic      `yaml:"traffic,omitempty" json:"traffic,omitempty"`
}

// UIDefaults defines default settings configured for the UI
type UIDefaults struct {
	Graph             GraphUIDefaults `yaml:"graph,omitempty" json:"graph,omitempty"`
	MetricsPerRefresh string          `yaml:"metrics_per_refresh,omitempty" json:"metricsPerRefresh,omitempty"`
	Namespaces        []string        `yaml:"namespaces,omitempty" json:"namespaces,omitempty"`
	RefreshInterval   string          `yaml:"refresh_interval,omitempty" json:"refreshInterval,omitempty"`
}

// KialiFeatureFlags available from the CR
type KialiFeatureFlags struct {
	IstioInjectionAction bool       `yaml:"istio_injection_action,omitempty" json:"istioInjectionAction"`
	UIDefaults           UIDefaults `yaml:"ui_defaults,omitempty" json:"uiDefaults,omitempty"`
}

// Tolerance config
type Tolerance struct {
	Code      string  `yaml:"code,omitempty" json:"code"`
	Degraded  float32 `yaml:"degraded,omitempty" json:"degraded"`
	Failure   float32 `yaml:"failure,omitempty" json:"failure"`
	Protocol  string  `yaml:"protocol,omitempty" json:"protocol"`
	Direction string  `yaml:"direction,omitempty" json:"direction"`
}

// Rate config
type Rate struct {
	Namespace string      `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Kind      string      `yaml:"kind,omitempty" json:"kind,omitempty"`
	Name      string      `yaml:"name,omitempty" json:"name,omitempty"`
	Tolerance []Tolerance `yaml:"tolerance,omitempty" json:"tolerance"`
}

// HealthConfig rates
type HealthConfig struct {
	Rate []Rate `yaml:"rate,omitempty" json:"rate,omitempty"`
}

// Config defines full YAML configuration.
type Config struct {
	AdditionalDisplayDetails []AdditionalDisplayItem             `yaml:"additional_display_details,omitempty"`
	API                      ApiConfig                           `yaml:"api,omitempty"`
	Auth                     AuthConfig                          `yaml:"auth,omitempty"`
	CustomDashboards         dashboards.MonitoringDashboardsList `yaml:"custom_dashboards,omitempty"`
	Deployment               DeploymentConfig                    `yaml:"deployment,omitempty"`
	Extensions               Extensions                          `yaml:"extensions,omitempty"`
	ExternalServices         ExternalServices                    `yaml:"external_services,omitempty"`
	HealthConfig             HealthConfig                        `yaml:"health_config,omitempty" json:"healthConfig,omitempty"`
	Identity                 security.Identity                   `yaml:",omitempty"`
	InCluster                bool                                `yaml:"in_cluster,omitempty"`
	InstallationTag          string                              `yaml:"installation_tag,omitempty"`
	IstioLabels              IstioLabels                         `yaml:"istio_labels,omitempty"`
	IstioNamespace           string                              `yaml:"istio_namespace,omitempty"` // default component namespace
	KialiFeatureFlags        KialiFeatureFlags                   `yaml:"kiali_feature_flags,omitempty"`
	KubernetesConfig         KubernetesConfig                    `yaml:"kubernetes_config,omitempty"`
	LoginToken               LoginToken                          `yaml:"login_token,omitempty"`
	Server                   Server                              `yaml:",omitempty"`
}

// NewConfig creates a default Config struct
func NewConfig() (c *Config) {
	c = &Config{
		InCluster:      true,
		IstioNamespace: "istio-system",
		API: ApiConfig{
			Namespaces: ApiNamespacesConfig{
				Exclude: []string{
					"istio-operator",
					"kube.*",
					"openshift.*",
					"ibm.*",
					"kial-operator",
				},
			},
		},
		Auth: AuthConfig{
			Strategy: "token",
			OpenId: OpenIdConfig{
				AdditionalRequestParams: map[string]string{},
				ApiProxy:                "",
				ApiProxyCAData:          "",
				AuthenticationTimeout:   300,
				ApiToken:                "id_token",
				AuthorizationEndpoint:   "",
				ClientId:                "",
				ClientSecret:            "",
				DisableRBAC:             false,
				InsecureSkipVerifyTLS:   false,
				IssuerUri:               "",
				Scopes:                  []string{"openid", "profile", "email"},
				UsernameClaim:           "sub",
			},
			OpenShift: OpenShiftConfig{
				ClientIdPrefix: "kiali",
			},
		},
		CustomDashboards: dashboards.GetBuiltInMonitoringDashboards(),
		Deployment: DeploymentConfig{
			AccessibleNamespaces: []string{"**"},
			Namespace:            "istio-system",
		},
		Extensions: Extensions{
			Iter8: Iter8Config{
				Enabled:   false,
				Namespace: "iter8",
			},
		},
		ExternalServices: ExternalServices{
			CustomDashboards: CustomDashboardsConfig{
				DiscoveryEnabled:       DashboardsDiscoveryAuto,
				DiscoveryAutoThreshold: 10,
				Enabled:                true,
				IsCore:                 false,
				NamespaceLabel:         "kubernetes_namespace",
			},
			Grafana: GrafanaConfig{
				Auth: Auth{
					Type: AuthTypeNone,
				},
				Enabled: true,
				IsCore:  false,
			},
			Istio: IstioConfig{
				ComponentStatuses: ComponentStatuses{
					Enabled: true,
					Components: []ComponentStatus{
						{
							AppLabel: "istio-egressgateway",
							IsCore:   false,
							IsProxy:  true,
						},
						{
							AppLabel: "istio-ingressgateway",
							IsCore:   true,
							IsProxy:  true,
						},
						{
							AppLabel: "istiod",
							IsCore:   true,
							IsProxy:  false,
						},
					},
				},
				ConfigMapName:                     "istio",
				EnvoyAdminLocalPort:               15000,
				IstioIdentityDomain:               "svc.cluster.local",
				IstioInjectionAnnotation:          "sidecar.istio.io/inject",
				IstioSidecarInjectorConfigMapName: "istio-sidecar-injector",
				IstioSidecarAnnotation:            "sidecar.istio.io/status",
				IstiodDeploymentName:              "istiod",
				UrlServiceVersion:                 "http://istiod:15014/version",
			},
			Prometheus: PrometheusConfig{
				Auth: Auth{
					Type: AuthTypeNone,
				},
				CacheEnabled: true,
				// 1/2 Prom Scrape Interval
				CacheDuration: 7,
				// Prom Cache expires and it forces to repopulate cache
				CacheExpiration: 300,
				URL:             "http://prometheus.istio-system:9090",
			},
			Tracing: TracingConfig{
				Auth: Auth{
					Type: AuthTypeNone,
				},
				Enabled:              true,
				NamespaceSelector:    true,
				InClusterURL:         "http://tracing.istio-system/jaeger",
				IsCore:               false,
				URL:                  "",
				UseGRPC:              false,
				WhiteListIstioSystem: []string{"jaeger-query", "istio-ingressgateway"},
			},
		},
		IstioLabels: IstioLabels{
			AppLabelName:       "app",
			InjectionLabelName: "istio-injection",
			VersionLabelName:   "version",
		},
		KialiFeatureFlags: KialiFeatureFlags{
			IstioInjectionAction: true,
			UIDefaults: UIDefaults{
				Graph: GraphUIDefaults{
					FindOptions: []GraphFindOption{
						{
							Description: "Find: slow edges (> 1s)",
							Expression:  "rt > 1000",
						},
						{
							Description: "Find: unhealthy nodes",
							Expression:  "! healthy",
						},
						{
							Description: "Find: unknown nodes",
							Expression:  "name = unknown",
						},
					},
					HideOptions: []GraphFindOption{
						{
							Description: "Hide: healthy nodes",
							Expression:  "healthy",
						},
						{
							Description: "Hide: unknown nodes",
							Expression:  "name = unknown",
						},
					},
					Traffic: GraphTraffic{
						Grpc: "requests",
						Http: "requests",
						Tcp:  "sent",
					},
				},
				MetricsPerRefresh: "1m",
				Namespaces:        make([]string, 0),
				RefreshInterval:   "15s",
			},
		},
		KubernetesConfig: KubernetesConfig{
			Burst:                       200,
			CacheDuration:               5 * 60,
			CacheEnabled:                true,
			CacheIstioTypes:             []string{"AuthorizationPolicy", "DestinationRule", "EnvoyFilter", "Gateway", "PeerAuthentication", "RequestAuthentication", "ServiceEntry", "Sidecar", "VirtualService", "WorkloadEntry", "WorkloadGroup"},
			CacheNamespaces:             []string{".*"},
			CacheTokenNamespaceDuration: 10,
			ExcludeWorkloads:            []string{"CronJob", "DeploymentConfig", "Job", "ReplicationController"},
			QPS:                         175,
		},
		LoginToken: LoginToken{
			ExpirationSeconds: 24 * 3600,
			SigningKey:        "kiali",
		},
		Server: Server{
			AuditLog:                   true,
			GzipEnabled:                true,
			MetricsEnabled:             true,
			MetricsPort:                9090,
			Port:                       20001,
			StaticContentRootDirectory: "/opt/kiali/console",
			WebFQDN:                    "",
			WebRoot:                    "/",
			WebHistoryMode:             "browser",
			WebSchema:                  "",
		},
	}

	return
}

// AddHealthDefault Configuration
func (conf *Config) AddHealthDefault() {
	// Health default configuration
	healthConfig := HealthConfig{
		Rate: []Rate{
			{
				Tolerance: []Tolerance{
					{
						Code:      "5XX",
						Protocol:  "http",
						Direction: ".*",
						Failure:   10,
					},
					{
						Code:      "4XX",
						Protocol:  "http",
						Direction: ".*",
						Degraded:  10,
						Failure:   20,
					},
					{
						Code:      "^[1-9]$|^1[0-6]$",
						Protocol:  "grpc",
						Direction: ".*",
						Failure:   10,
					},
					{
						Code:      "^-$", // no response is indicated with a "-" code
						Protocol:  "http|grpc",
						Direction: ".*",
						Failure:   10,
					},
				},
			},
		},
	}
	conf.HealthConfig.Rate = append(conf.HealthConfig.Rate, healthConfig.Rate...)
}

// Get the global Config
func Get() (conf *Config) {
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	copy := configuration
	return &copy
}

// Set the global Config
// This function should not be called outside of main or tests.
// If possible keep config unmutated and use globals and/or appstate package for mutable states to avoid concurrent writes risk.
func Set(conf *Config) {
	rwMutex.Lock()
	defer rwMutex.Unlock()
	conf.AddHealthDefault()
	configuration = *conf
}

// String marshals the given Config into a YAML string
// WARNING: do NOT use the result of this function to retrieve any configuration: some fields are obfuscated for security reasons.
func (conf Config) String() (str string) {
	obf := conf
	obf.ExternalServices.Grafana.Auth.Obfuscate()
	obf.ExternalServices.Prometheus.Auth.Obfuscate()
	obf.ExternalServices.Tracing.Auth.Obfuscate()
	obf.Identity.Obfuscate()
	obf.LoginToken.Obfuscate()
	obf.Auth.OpenId.ClientSecret = "xxx"
	str, err := Marshal(&obf)
	if err != nil {
		str = fmt.Sprintf("Failed to marshal config to string. err=%v", err)
		log.Debugf(str)
	}

	return
}

// prepareDashboards will ensure conf.CustomDashboards contains only the dashboards that are enabled
func (conf *Config) prepareDashboards() {
	if conf.ExternalServices.CustomDashboards.Enabled {
		// If the user defined their own dashboards, we still want the built-in dashboards as a fallback.
		// But the user-defined dashboards take precedence - if they gave us dashboards with the same name
		// as one of the built-in dashboards, the user-defined dashboard "wins".
		conf.CustomDashboards = dashboards.AddMonitoringDashboards(dashboards.GetBuiltInMonitoringDashboards(), conf.CustomDashboards)
	} else {
		// The user has disabled the custom dashboards, empty out the list completely
		conf.CustomDashboards = dashboards.MonitoringDashboardsList(make([]dashboards.MonitoringDashboard, 0))
	}

	// to assist in debugging problems, log the number of dashboards and their names
	if log.IsDebug() {
		dashboardNames := make([]string, 0, len(conf.CustomDashboards))
		for _, d := range conf.CustomDashboards {
			dashboardNames = append(dashboardNames, d.Name)
		}
		sort.Strings(dashboardNames)
		log.Debugf("Custom dashboards [count=%v, enabled=%v]: %v", len(dashboardNames), conf.ExternalServices.CustomDashboards.Enabled, strings.Join(dashboardNames, ","))
	}
}

// Unmarshal parses the given YAML string and returns its Config object representation.
func Unmarshal(yamlString string) (conf *Config, err error) {
	conf = NewConfig()
	err = yaml.Unmarshal([]byte(yamlString), &conf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse yaml data. error=%v", err)
	}

	conf.prepareDashboards()

	// Some config settings (such as sensitive settings like passwords) are overrideable
	// via environment variables. This allows a user to store sensitive values in secrets
	// and mount those secrets to environment variables rather than storing them directly
	// in the config map itself.
	type overridesType struct {
		configValue *string
		envVarName  string
	}

	overrides := []overridesType{
		{
			configValue: &conf.ExternalServices.Grafana.Auth.Password,
			envVarName:  EnvGrafanaPassword,
		},
		{
			configValue: &conf.ExternalServices.Grafana.Auth.Token,
			envVarName:  EnvGrafanaToken,
		},
		{
			configValue: &conf.ExternalServices.Prometheus.Auth.Password,
			envVarName:  EnvPrometheusPassword,
		},
		{
			configValue: &conf.ExternalServices.Prometheus.Auth.Token,
			envVarName:  EnvPrometheusToken,
		},
		{
			configValue: &conf.ExternalServices.Tracing.Auth.Password,
			envVarName:  EnvTracingPassword,
		},
		{
			configValue: &conf.ExternalServices.Tracing.Auth.Token,
			envVarName:  EnvTracingToken,
		},
		{
			configValue: &conf.LoginToken.SigningKey,
			envVarName:  EnvLoginTokenSigningKey,
		},
	}

	for _, override := range overrides {
		envVarValue := os.Getenv(override.envVarName)
		if envVarValue != "" {
			*override.configValue = envVarValue
		}
	}

	return
}

// Marshal converts the Config object and returns its YAML string.
func Marshal(conf *Config) (yamlString string, err error) {
	yamlBytes, err := yaml.Marshal(&conf)
	if err != nil {
		return "", fmt.Errorf("failed to produce yaml. error=%v", err)
	}

	yamlString = string(yamlBytes)
	return
}

// LoadFromFile reads the YAML from the given file, parses the content, and returns its Config object representation.
func LoadFromFile(filename string) (conf *Config, err error) {
	log.Debugf("Reading YAML config from [%s]", filename)
	fileContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file [%v]. error=%v", filename, err)
	}

	conf, err = Unmarshal(string(fileContent))
	if err != nil {
		return
	}

	// Read OIDC secret, if present
	if oidcSecret, oidcErr := ioutil.ReadFile(OidcClientSecretFile); oidcErr == nil {
		conf.Auth.OpenId.ClientSecret = string(oidcSecret)
	} else {
		if !os.IsNotExist(oidcErr) {
			err = fmt.Errorf("failed to OIDC client secret file [%v]. error=%v", OidcClientSecretFile, oidcErr)
		}

		// ...else, if error indicates that secret does not exist, then ignore because the secret is optional
	}

	return
}

// SaveToFile converts the Config object and stores its YAML string into the given file, overwriting any data that is in the file.
func SaveToFile(filename string, conf *Config) (err error) {
	fileContent, err := Marshal(conf)
	if err != nil {
		return fmt.Errorf("failed to save config file [%v]. error=%v", filename, err)
	}

	log.Debugf("Writing YAML config to [%s]", filename)
	err = ioutil.WriteFile(filename, []byte(fileContent), 0640)
	return
}

// IsIstioNamespace returns true if the namespace is the default istio namespace
func IsIstioNamespace(namespace string) bool {
	return namespace == configuration.IstioNamespace
}
