package business

import (
	"sort"
	"sync"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/models"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Workload deals with fetching istio/kubernetes workloads related content and convert to kiali model
type WorkloadService struct {
	k8s kubernetes.IstioClientInterface
}

// GetWorkloadList is the API handler to fetch the list of workloads in a given namespace.
func (in *WorkloadService) GetWorkloadList(namespace string) (models.WorkloadList, error) {

	workloadList := &models.WorkloadList{
		Namespace: models.Namespace{namespace},
		Workloads: []models.WorkloadListItem{},
	}
	ws, err := fetchWorkloads(in.k8s, namespace, "")
	if err != nil {
		return *workloadList, err
	}

	for _, w := range ws {
		wItem := &models.WorkloadListItem{}
		wItem.ParseWorkload(w)
		workloadList.Workloads = append(workloadList.Workloads, *wItem)
	}

	return *workloadList, nil
}

// GetWorkload is the API handler to fetch details of a specific workload.
// If includeServices is set true, the Workload will fetch all services related
func (in *WorkloadService) GetWorkload(namespace string, workloadName string, includeServices bool) (*models.Workload, error) {
	workload, err := fetchWorkload(in.k8s, namespace, workloadName)
	if err != nil {
		return nil, err
	}

	if includeServices {
		services, err := in.k8s.GetServices(namespace, workload.Labels)
		if err != nil {
			return nil, err
		}
		workload.SetServices(services)
	}

	return workload, nil
}

func (in *WorkloadService) GetPods(namespace string, labelSelector string) (models.Pods, error) {
	ps, err := in.k8s.GetPods(namespace, labelSelector)
	if err != nil {
		return nil, err
	}
	pods := models.Pods{}
	pods.Parse(ps)
	return pods, nil
}

func fetchWorkloads(k8s kubernetes.IstioClientInterface, namespace string, labelSelector string) (models.Workloads, error) {
	var pods []v1.Pod
	var dep []v1beta1.Deployment

	ws := models.Workloads{}

	wg := sync.WaitGroup{}
	wg.Add(2)
	errChan := make(chan error, 2)

	go func() {
		defer wg.Done()
		var err error
		pods, err = k8s.GetPods(namespace, labelSelector)
		if err != nil {
			log.Errorf("Error fetching Pods per namespace %s: %s", namespace, err)
			errChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		dep, err = k8s.GetDeployments(namespace)
		if err != nil {
			log.Errorf("Error fetching Deployments per namespace %s: %s", namespace, err)
			errChan <- err
		}
	}()

	wg.Wait()
	if len(errChan) != 0 {
		err := <-errChan
		return ws, err
	}

	// Key: name of controller; Value: type of controller
	controllers := map[string]string{}

	// Find controllers from pods
	for _, pod := range pods {
		if len(pod.OwnerReferences) == 0 {
			if _, exist := controllers[pod.Name]; !exist {
				// Pod without controller
				controllers[pod.Name] = "Pod"
			}
		}
	}

	// Cornercase, check for controllers without pods, to show them as a workload
	var selector labels.Selector
	var selErr error
	if labelSelector != "" {
		selector, selErr = labels.Parse(labelSelector)
		if selErr != nil {
			log.Errorf("%s can not be processed as selector: %v", labelSelector, selErr)
		}
	}
	for _, d := range dep {
		selectorCheck := true
		if selector != nil {
			selectorCheck = selector.Matches(labels.Set(d.Spec.Template.Labels))
		}
		if _, exist := controllers[d.Name]; !exist && selectorCheck {
			controllers[d.Name] = "Deployment"
		}
	}

	// Build workloads from controllers
	var cnames []string
	for k := range controllers {
		cnames = append(cnames, k)
	}
	sort.Strings(cnames)
	for _, cname := range cnames {
		w := &models.Workload{
			Pods:     models.Pods{},
			Services: models.Services{},
		}
		ctype := controllers[cname]
		// Flag to add a controller if it is found
		cnFound := true
		switch ctype {
		case "Deployment":
			found := false
			iFound := -1
			for i, dp := range dep {
				if dp.Name == cname {
					found = true
					iFound = i
					break
				}
			}
			if found {
				selector := labels.Set(dep[iFound].Spec.Template.Labels).AsSelector()
				w.SetPods(kubernetes.FilterPodsForSelector(selector, pods))
				w.ParseDeployment(&dep[iFound])
			} else {
				log.Errorf("Workload %s is not found as Deployment", cname)
				cnFound = false
			}
		case "Pod":
			found := false
			iFound := -1
			for i, pod := range pods {
				if pod.Name == cname {
					found = true
					iFound = i
					break
				}
			}
			if found {
				w.SetPods([]v1.Pod{pods[iFound]})
				w.ParsePod(&pods[iFound])
			} else {
				log.Errorf("Workload %s is not found as Pod", cname)
				cnFound = false
			}
		default:
			cPods := kubernetes.FilterPodsForController(cname, ctype, pods)
			w.SetPods(cPods)
			w.ParsePods(cname, ctype, cPods)
		}
		if cnFound {
			ws = append(ws, w)
		}
	}
	return ws, nil
}

func fetchWorkload(k8s kubernetes.IstioClientInterface, namespace string, workloadName string) (*models.Workload, error) {
	wl := &models.Workload{
		Pods:     models.Pods{},
		Services: models.Services{},
	}

	var pods []v1.Pod
	var dp *v1beta1.Deployment

	wg := sync.WaitGroup{}
	wg.Add(2)
	errChan := make(chan error, 1)

	go func() {
		defer wg.Done()
		var err error
		pods, err = k8s.GetPods(namespace, "")
		if err != nil {
			log.Errorf("Error fetching Deployments per namespace %s: %s", namespace, err)
			errChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		dp, err = k8s.GetDeployment(namespace, workloadName)
		if err != nil {
			dp = nil
		}
	}()

	wg.Wait()
	if len(errChan) != 0 {
		err := <-errChan
		return wl, err
	}

	if dp != nil {
		selector := labels.Set(dp.Spec.Template.Labels).AsSelector()
		wl.SetPods(kubernetes.FilterPodsForSelector(selector, pods))
		wl.ParseDeployment(dp)
		return wl, nil
	}

	// Workload is a controller not listed or a pod
	ctype := ""
	found := false
	iFound := -1
	for i, pod := range pods {
		if pod.Name == workloadName {
			ctype = "Pod"
			found = true
			iFound = i
			break
		}
		for _, ref := range pod.OwnerReferences {
			if ref.Controller != nil && *ref.Controller && ref.Name == workloadName {
				ctype = ref.Kind
				break
			}
		}
		if ctype != "" {
			break
		}
	}

	if found && ctype == "Pod" {
		wl.ParsePod(&pods[iFound])
		wl.SetPods([]v1.Pod{pods[iFound]})
		return wl, nil
	}

	if ctype != "" {
		cPods := kubernetes.FilterPodsForController(workloadName, ctype, pods)
		wl.ParsePods(workloadName, ctype, cPods)
		wl.SetPods(cPods)
		return wl, nil
	} else {
		// Workload Not found
		return wl, kubernetes.NewNotFound("kiali", "workload", workloadName)
	}

	return wl, nil
}

// KIALI-1730
// This method is used to decide the priority of the controller in the cornercase when two controllers have same labels
// on the selector. Note that this is a situation that user should control as it is described in the documentation:
// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
// But Istio only identifies one controller as workload (it doesn't note which one).
// Kiali can select one on the list of workloads and other in the details and this should be consistent.
var controllerOrder = map[string]int{
	"Deployment":            6,
	"DeploymentConfig":      5,
	"ReplicaSet":            4,
	"ReplicationController": 3,
	"StatefulSet":           2,
	"Job":                   1,
	"DaemonSet":             0,
	"Pod":                   -1,
}

func controllerPriority(type1, type2 string) string {
	w1, e1 := controllerOrder[type1]
	if !e1 {
		log.Errorf("This controller %s is assigned in a Pod and it's not properly managed", type1)
	}
	w2, e2 := controllerOrder[type2]
	if !e2 {
		log.Errorf("This controller %s is assigned in a Pod and it's not properly managed", type2)
	}
	if w1 >= w2 {
		return type1
	} else {
		return type2
	}
}
