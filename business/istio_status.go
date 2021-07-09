package business

import (
	"fmt"
	"sync"
	"time"

	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/util/httputil"
)

// SvcService deals with fetching istio/kubernetes services related content and convert to kiali model
type IstioStatusService struct {
	k8s           kubernetes.ClientInterface
	businessLayer *Layer
}

type ComponentStatus struct {
	// The app label value of the Istio component
	//
	// example: istio-ingressgateway
	// required: true
	Name string `json:"name"`

	// The status of a Istio component
	//
	// example:  Not Found
	// required: true
	Status string `json:"status"`

	// When true, the component is necessary for Istio to function. Otherwise, it is an addon
	//
	// example:  true
	// required: true
	IsCore bool `json:"is_core"`
}

type IstioComponentStatus []ComponentStatus

func (ics *IstioComponentStatus) merge(cs IstioComponentStatus) IstioComponentStatus {
	*ics = append(*ics, cs...)
	return *ics
}

const (
	Healthy     string = "Healthy"
	NotFound    string = "NotFound"
	NotReady    string = "NotReady"
	Unhealthy   string = "Unhealthy"
	Unreachable string = "Unreachable"
)

func (iss *IstioStatusService) GetStatus() (IstioComponentStatus, error) {
	if !config.Get().ExternalServices.Istio.ComponentStatuses.Enabled {
		return IstioComponentStatus{}, nil
	}

	ics, err := iss.getIstioComponentStatus()
	if err != nil {
		return nil, err
	}

	return ics.merge(iss.getAddonComponentStatus()), nil
}

func (iss *IstioStatusService) getIstioComponentStatus() (IstioComponentStatus, error) {
	// Fetching workloads from component namespaces
	workloads, err := iss.getComponentNamespacesWorkloads()
	if err != nil {
		return IstioComponentStatus{}, err
	}

	deploymentStatus, err := iss.getStatusOf(workloads)
	if err != nil {
		return IstioComponentStatus{}, err
	}

	istiodStatus, err := iss.getIstiodReachingCheck()
	if err != nil {
		return IstioComponentStatus{}, err
	}

	return deploymentStatus.merge(istiodStatus), nil
}

func (iss *IstioStatusService) getComponentNamespacesWorkloads() ([]*models.Workload, error) {
	var wg sync.WaitGroup

	nss := map[string]bool{}
	wls := make([]*models.Workload, 0)

	comNs := getComponentNamespaces()

	wlChan := make(chan []*models.Workload, len(comNs))
	errChan := make(chan error, len(comNs))

	for _, n := range comNs {
		if !nss[n] {
			wg.Add(1)
			nss[n] = true

			go func(n string, wliChan chan []*models.Workload, errChan chan error) {
				defer wg.Done()
				var wls models.Workloads
				var err error
				wls, err = fetchWorkloads(iss.businessLayer, n, "")
				wliChan <- wls
				errChan <- err
			}(n, wlChan, errChan)
		}
	}

	wg.Wait()

	close(wlChan)
	close(errChan)
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	for wl := range wlChan {
		if wl != nil {
			wls = append(wls, wl...)
		}
	}

	return wls, nil
}

func getComponentNamespaces() []string {
	nss := make([]string, 0)

	// By default, add the istio control plane namespace
	nss = append(nss, config.Get().IstioNamespace)

	// Adding Istio Components namespaces
	externalServices := config.Get().ExternalServices
	for _, cmp := range externalServices.Istio.ComponentStatuses.Components {
		if cmp.Namespace != "" {
			nss = append(nss, cmp.Namespace)
		}
	}

	return nss
}

func istioCoreComponents() map[string]bool {
	components := map[string]bool{}
	cs := config.Get().ExternalServices.Istio.ComponentStatuses
	for _, c := range cs.Components {
		components[c.AppLabel] = c.IsCore
	}
	return components
}

func (iss *IstioStatusService) getStatusOf(workloads []*models.Workload) (IstioComponentStatus, error) {
	statusComponents := istioCoreComponents()
	isc := IstioComponentStatus{}
	cf := map[string]bool{}

	// Map workloads there by app name
	for _, workload := range workloads {
		appLabel := labels.Set(workload.Labels).Get("app")
		if appLabel == "" {
			continue
		}

		isCore, found := statusComponents[appLabel]
		if !found {
			continue
		}

		// Component found
		cf[appLabel] = true

		if status := GetWorkloadStatus(*workload); status != Healthy {
			// Check status
			isc = append(isc, ComponentStatus{
				Name:   workload.Name,
				Status: status,
				IsCore: isCore,
			},
			)
		}
	}

	// Add missing deployments
	componentNotFound := 0
	for comp, isCore := range statusComponents {
		if _, found := cf[comp]; !found {
			componentNotFound += 1
			isc = append(isc, ComponentStatus{
				Name:   comp,
				Status: NotFound,
				IsCore: isCore,
			})
		}
	}

	// When all the deployments are missing,
	// Warn users that their kiali config might be wrong
	if componentNotFound == len(statusComponents) {
		return isc, fmt.Errorf(
			"Kiali is unable to find any Istio deployment in namespace %s. Are you sure the Istio namespace is configured correctly in Kiali?",
			config.Get().IstioNamespace)
	}

	return isc, nil
}

func GetWorkloadStatus(wl models.Workload) string {
	status := Unhealthy

	if wl.DesiredReplicas == 0 {
		status = NotReady
	} else if wl.DesiredReplicas == wl.AvailableReplicas && wl.DesiredReplicas == wl.CurrentReplicas {
		status = Healthy
	}
	return status
}

func (iss *IstioStatusService) getAddonComponentStatus() IstioComponentStatus {
	var wg sync.WaitGroup
	wg.Add(4)

	staChan := make(chan IstioComponentStatus, 4)
	extServices := config.Get().ExternalServices
	ics := IstioComponentStatus{}

	go getAddonStatus("prometheus", true, extServices.Prometheus.IsCore, &extServices.Prometheus.Auth, extServices.Prometheus.URL, extServices.Prometheus.HealthCheckUrl, staChan, &wg)
	go getAddonStatus("grafana", extServices.Grafana.Enabled, extServices.Grafana.IsCore, &extServices.Grafana.Auth, extServices.Grafana.InClusterURL, extServices.Grafana.HealthCheckUrl, staChan, &wg)
	go getAddonStatus("jaeger", extServices.Tracing.Enabled, extServices.Tracing.IsCore, &extServices.Tracing.Auth, extServices.Tracing.InClusterURL, extServices.Tracing.HealthCheckUrl, staChan, &wg)

	// Custom dashboards may use the main Prometheus config
	customProm := extServices.CustomDashboards.Prometheus
	if customProm.URL == "" {
		customProm = extServices.Prometheus
	}
	go getAddonStatus("custom dashboards", extServices.CustomDashboards.Enabled, extServices.CustomDashboards.IsCore, &customProm.Auth, customProm.URL, customProm.HealthCheckUrl, staChan, &wg)

	wg.Wait()

	close(staChan)
	for stat := range staChan {
		ics.merge(stat)
	}

	return ics
}

func (iss *IstioStatusService) getIstiodReachingCheck() (IstioComponentStatus, error) {
	cfg := config.Get()

	istiods, err := iss.k8s.GetPods(cfg.IstioNamespace, labels.Set(map[string]string{"app": "istiod"}).String())
	if err != nil {
		return nil, err
	}

	healthyIstiods := make([]*core_v1.Pod, 0, len(istiods))
	for i, istiod := range istiods {
		if istiod.Status.Phase == core_v1.PodRunning {
			healthyIstiods = append(healthyIstiods, &istiods[i])
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(len(healthyIstiods))
	syncChan := make(chan ComponentStatus, len(healthyIstiods))

	for _, istiod := range healthyIstiods {
		go func(name, namespace string) {
			defer wg.Done()
			// Using the proxy method to make sure that K8s API has access to the Istio Control Plane namespace.
			// By proxying one Istiod, we ensure that the following connection is allowed:
			// Kiali -> K8s API (proxy) -> istiod
			// This scenario is no obvious for private clusters (like GKE private cluster)
			_, err := iss.k8s.ForwardGetRequest(namespace, name, httputil.Pool.GetFreePort(), 8080, "/ready")
			if err != nil {
				syncChan <- ComponentStatus{
					Name:   name,
					Status: Unreachable,
					IsCore: true,
				}
			}
		}(istiod.Name, istiod.Namespace)
	}

	wg.Wait()
	close(syncChan)
	ics := IstioComponentStatus{}
	for componentStatus := range syncChan {
		ics.merge(IstioComponentStatus{componentStatus})
	}

	return ics, nil
}

func getAddonStatus(name string, enabled bool, isCore bool, auth *config.Auth, url string, healthCheckUrl string, staChan chan<- IstioComponentStatus, wg *sync.WaitGroup) {
	defer wg.Done()

	// When the addOn is disabled, don't perform any check
	if !enabled {
		return
	}

	// Take the alternative health check url if present
	if healthCheckUrl != "" {
		url = healthCheckUrl
	}

	if auth.UseKialiToken {
		token, err := kubernetes.GetKialiToken()
		if err != nil {
			log.Errorf("Could not read the Kiali Service Account token: %v", err)
		}
		auth.Token = token
	}

	// Call the addOn service endpoint to find out whether is reachable or not
	_, statusCode, err := httputil.HttpGet(url, auth, 10*time.Second)
	if err != nil || statusCode > 399 {
		staChan <- IstioComponentStatus{
			ComponentStatus{
				Name:   name,
				Status: Unreachable,
				IsCore: isCore,
			},
		}
	}
}
