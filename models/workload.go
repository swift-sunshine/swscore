package models

import (
	"strconv"

	osapps_v1 "github.com/openshift/api/apps/v1"
	apps_v1 "k8s.io/api/apps/v1"
	batch_v1 "k8s.io/api/batch/v1"
	batch_v1beta1 "k8s.io/api/batch/v1beta1"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kiali/kiali/config"
)

type WorkloadList struct {
	// Namespace where the workloads live in
	// required: true
	// example: bookinfo
	Namespace Namespace `json:"namespace"`

	// Workloads for a given namespace
	// required: true
	Workloads []WorkloadListItem `json:"workloads"`
}

// WorkloadListItem has the necessary information to display the console workload list
type WorkloadListItem struct {
	// Name of the workload
	// required: true
	// example: reviews-v1
	Name string `json:"name"`

	// Type of the workload
	// required: true
	// example: deployment
	Type string `json:"type"`

	// Creation timestamp (in RFC3339 format)
	// required: true
	// example: 2018-07-31T12:24:17Z
	CreatedAt string `json:"createdAt"`

	// Kubernetes ResourceVersion
	// required: true
	// example: 192892127
	ResourceVersion string `json:"resourceVersion"`

	// Define if Workload has an explicit Istio policy annotation
	// It's mapped as a pointer to show three values nil, true, false
	IstioInjectionAnnotation *bool `json:"istioInjectionAnnotation,omitempty"`

	// Define if Pods related to this Workload has an IstioSidecar deployed
	// required: true
	// example: true
	IstioSidecar bool `json:"istioSidecar"`

	// Additional item sample, such as type of api being served (graphql, grpc, rest)
	// example: rest
	// required: false
	AdditionalDetailSample *AdditionalItem `json:"additionalDetailSample"`

	// Workload labels
	Labels map[string]string `json:"labels"`

	// Define if Pods related to this Workload has the label App
	// required: true
	// example: true
	AppLabel bool `json:"appLabel"`

	// Define if Pods related to this Workload has the label Version
	// required: true
	// example: true
	VersionLabel bool `json:"versionLabel"`

	// Number of current workload pods
	// required: true
	// example: 1
	PodCount int `json:"podCount"`

	// HealthAnnotations
	// required: false
	HealthAnnotations map[string]string `json:"healthAnnotations"`

	// Istio References
	IstioReferences []*IstioValidationKey `json:"istioReferences"`

	// Dashboard annotations
	// required: false
	DashboardAnnotations map[string]string `json:"dashboardAnnotations"`
}

type WorkloadOverviews []*WorkloadListItem

// Workload has the details of a workload
type Workload struct {
	WorkloadListItem

	// Number of desired replicas defined by the user in the controller Spec
	// required: true
	// example: 2
	DesiredReplicas int32 `json:"desiredReplicas"`

	// Number of current replicas pods that matches controller selector labels
	// required: true
	// example: 2
	CurrentReplicas int32 `json:"currentReplicas"`

	// Number of available replicas
	// required: true
	// example: 1
	AvailableReplicas int32 `json:"availableReplicas"`

	// Pods bound to the workload
	Pods Pods `json:"pods"`

	// Services that match workload selector
	Services Services `json:"services"`

	// Runtimes and associated dashboards
	Runtimes []Runtime `json:"runtimes"`

	// Additional details to display, such as configured annotations
	AdditionalDetails []AdditionalItem `json:"additionalDetails"`
}

type Workloads []*Workload

func (workload *WorkloadListItem) ParseWorkload(w *Workload) {
	conf := config.Get()
	workload.Name = w.Name
	workload.Type = w.Type
	workload.CreatedAt = w.CreatedAt
	workload.ResourceVersion = w.ResourceVersion
	workload.IstioSidecar = w.HasIstioSidecar()
	workload.Labels = w.Labels
	workload.PodCount = len(w.Pods)
	workload.AdditionalDetailSample = w.AdditionalDetailSample
	workload.HealthAnnotations = w.HealthAnnotations
	workload.IstioReferences = []*IstioValidationKey{}

	/** Check the labels app and version required by Istio in template Pods*/
	_, workload.AppLabel = w.Labels[conf.IstioLabels.AppLabelName]
	_, workload.VersionLabel = w.Labels[conf.IstioLabels.VersionLabelName]
}

func (workload *Workload) parseObjectMeta(meta *meta_v1.ObjectMeta, tplMeta *meta_v1.ObjectMeta) {
	conf := config.Get()
	workload.Name = meta.Name
	if tplMeta != nil && tplMeta.Labels != nil {
		workload.Labels = tplMeta.Labels
		/** Check the labels app and version required by Istio in template Pods*/
		_, workload.AppLabel = tplMeta.Labels[conf.IstioLabels.AppLabelName]
		_, workload.VersionLabel = tplMeta.Labels[conf.IstioLabels.VersionLabelName]
	} else {
		workload.Labels = map[string]string{}
	}
	annotations := meta.Annotations
	if tplMeta.Annotations != nil {
		annotations = tplMeta.Annotations
	}
	var annotation string
	exist := false
	annotation, exist = annotations[conf.ExternalServices.Istio.IstioInjectionAnnotation]
	if value, err := strconv.ParseBool(annotation); exist && err == nil {
		workload.IstioInjectionAnnotation = &value
	}
	workload.CreatedAt = formatTime(meta.CreationTimestamp.Time)
	workload.ResourceVersion = meta.ResourceVersion
	workload.AdditionalDetails = GetAdditionalDetails(conf, annotations)
	workload.AdditionalDetailSample = GetFirstAdditionalIcon(conf, annotations)
	workload.DashboardAnnotations = GetDashboardAnnotation(annotations)
	workload.HealthAnnotations = GetHealthAnnotation(annotations, GetHealthConfigAnnotation())
}

func (workload *Workload) ParseDeployment(d *apps_v1.Deployment) {
	workload.Type = "Deployment"
	workload.parseObjectMeta(&d.ObjectMeta, &d.Spec.Template.ObjectMeta)
	if d.Spec.Replicas != nil {
		workload.DesiredReplicas = *d.Spec.Replicas
	}
	workload.CurrentReplicas = d.Status.Replicas
	workload.AvailableReplicas = d.Status.AvailableReplicas
}

func (workload *Workload) ParseReplicaSet(r *apps_v1.ReplicaSet) {
	workload.Type = "ReplicaSet"
	workload.parseObjectMeta(&r.ObjectMeta, &r.Spec.Template.ObjectMeta)
	if r.Spec.Replicas != nil {
		workload.DesiredReplicas = *r.Spec.Replicas
	}
	workload.CurrentReplicas = r.Status.Replicas
	workload.AvailableReplicas = r.Status.AvailableReplicas
}

func (workload *Workload) ParseReplicaSetParent(r *apps_v1.ReplicaSet, workloadName string, workloadType string) {
	// Some properties are taken from the ReplicaSet
	workload.parseObjectMeta(&r.ObjectMeta, &r.Spec.Template.ObjectMeta)
	// But name and type are coming from the parent
	// Custom properties from parent controller are not processed by Kiali
	workload.Type = workloadType
	workload.Name = workloadName
	if r.Spec.Replicas != nil {
		workload.DesiredReplicas = *r.Spec.Replicas
	}
	workload.CurrentReplicas = r.Status.Replicas
	workload.AvailableReplicas = r.Status.AvailableReplicas
}

func (workload *Workload) ParseReplicationController(r *core_v1.ReplicationController) {
	workload.Type = "ReplicationController"
	workload.parseObjectMeta(&r.ObjectMeta, &r.Spec.Template.ObjectMeta)
	if r.Spec.Replicas != nil {
		workload.DesiredReplicas = *r.Spec.Replicas
	}
	workload.CurrentReplicas = r.Status.Replicas
	workload.AvailableReplicas = r.Status.AvailableReplicas
}

func (workload *Workload) ParseDeploymentConfig(dc *osapps_v1.DeploymentConfig) {
	workload.Type = "DeploymentConfig"
	workload.parseObjectMeta(&dc.ObjectMeta, &dc.Spec.Template.ObjectMeta)
	workload.DesiredReplicas = dc.Spec.Replicas
	workload.CurrentReplicas = dc.Status.Replicas
	workload.AvailableReplicas = dc.Status.AvailableReplicas
}

func (workload *Workload) ParseStatefulSet(s *apps_v1.StatefulSet) {
	workload.Type = "StatefulSet"
	workload.parseObjectMeta(&s.ObjectMeta, &s.Spec.Template.ObjectMeta)
	if s.Spec.Replicas != nil {
		workload.DesiredReplicas = *s.Spec.Replicas
	}
	workload.CurrentReplicas = s.Status.Replicas
	workload.AvailableReplicas = s.Status.ReadyReplicas
}

func (workload *Workload) ParsePod(pod *core_v1.Pod) {
	workload.Type = "Pod"
	workload.parseObjectMeta(&pod.ObjectMeta, &pod.ObjectMeta)

	var podReplicas, podAvailableReplicas int32
	podReplicas = 1
	podAvailableReplicas = 1

	// When a Workload is a single pod we don't have access to any controller replicas
	// On this case we differentiate when pod is terminated with success versus not running
	// Probably it might be more cases to refine here
	if pod.Status.Phase == "Succeed" {
		podReplicas = 0
		podAvailableReplicas = 0
	} else if pod.Status.Phase != "Running" {
		podAvailableReplicas = 0
	}

	workload.DesiredReplicas = podReplicas
	// Pod has not concept of replica
	workload.CurrentReplicas = workload.DesiredReplicas
	workload.AvailableReplicas = podAvailableReplicas
}

func (workload *Workload) ParseJob(job *batch_v1.Job) {
	workload.Type = "Job"
	workload.parseObjectMeta(&job.ObjectMeta, &job.ObjectMeta)
	// Job controller does not use replica parameters as other controllers
	// this is a workaround to use same values from Workload perspective
	workload.DesiredReplicas = job.Status.Active + job.Status.Succeeded + job.Status.Failed
	workload.CurrentReplicas = workload.DesiredReplicas
	workload.AvailableReplicas = job.Status.Active + job.Status.Succeeded
}

func (workload *Workload) ParseCronJob(cnjb *batch_v1beta1.CronJob) {
	workload.Type = "CronJob"
	workload.parseObjectMeta(&cnjb.ObjectMeta, &cnjb.ObjectMeta)

	// We don't have the information of this controller
	// We will infer the number of replicas as the number of pods without succeed state
	// We will infer the number of available as the number of pods with running state
	// If this is not enough, we should try to fetch the controller, it is not doing now to not overload kiali fetching all types of controllers
	var podReplicas, podAvailableReplicas int32
	podReplicas = 0
	podAvailableReplicas = 0
	for _, pod := range workload.Pods {
		if pod.Status != "Succeeded" {
			podReplicas++
		}
		if pod.Status == "Running" {
			podAvailableReplicas++
		}
	}
	workload.DesiredReplicas = podReplicas
	workload.DesiredReplicas = workload.CurrentReplicas
	workload.AvailableReplicas = podAvailableReplicas
	workload.HealthAnnotations = GetHealthAnnotation(cnjb.Annotations, GetHealthConfigAnnotation())
}

func (workload *Workload) ParseDaemonSet(ds *apps_v1.DaemonSet) {
	workload.Type = "DaemonSet"
	workload.parseObjectMeta(&ds.ObjectMeta, &ds.Spec.Template.ObjectMeta)
	// This is a cornercase for DaemonSet controllers
	// Desired is the number of desired nodes in a cluster that are running a DaemonSet Pod
	// We are not going to change that terminology in the backend model yet, but probably add a note in the UI in the future
	workload.DesiredReplicas = ds.Status.DesiredNumberScheduled
	workload.CurrentReplicas = ds.Status.CurrentNumberScheduled
	workload.AvailableReplicas = ds.Status.NumberAvailable
	workload.HealthAnnotations = GetHealthAnnotation(ds.Annotations, GetHealthConfigAnnotation())
}

func (workload *Workload) ParsePods(controllerName string, controllerType string, pods []core_v1.Pod) {
	conf := config.Get()
	workload.Name = controllerName
	workload.Type = controllerType
	// We don't have the information of this controller
	// We will infer the number of replicas as the number of pods without succeed state
	// We will infer the number of available as the number of pods with running state
	// If this is not enough, we should try to fetch the controller, it is not doing now to not overload kiali fetching all types of controllers
	var podReplicas, podAvailableReplicas int32
	podReplicas = 0
	podAvailableReplicas = 0
	for _, pod := range pods {
		if pod.Status.Phase != "Succeeded" {
			podReplicas++
		}
		if pod.Status.Phase == "Running" {
			podAvailableReplicas++
		}
	}
	workload.DesiredReplicas = podReplicas
	workload.CurrentReplicas = workload.DesiredReplicas
	workload.AvailableReplicas = podAvailableReplicas
	// We fetch one pod as template for labels
	// There could be corner cases not correct, then we should support more controllers
	workload.Labels = map[string]string{}
	if len(pods) > 0 {
		if pods[0].Labels != nil {
			workload.Labels = pods[0].Labels
		}
		workload.CreatedAt = formatTime(pods[0].CreationTimestamp.Time)
		workload.ResourceVersion = pods[0].ResourceVersion
	}

	/** Check the labels app and version required by Istio in template Pods*/
	_, workload.AppLabel = workload.Labels[conf.IstioLabels.AppLabelName]
	_, workload.VersionLabel = workload.Labels[conf.IstioLabels.VersionLabelName]
}

func (workload *Workload) SetPods(pods []core_v1.Pod) {
	workload.Pods.Parse(pods)
	workload.IstioSidecar = workload.HasIstioSidecar()
}

func (workload *Workload) SetServices(svcs []core_v1.Service) {
	workload.Services.Parse(svcs)
}

// HasIstioSidecar return true if there is at least one pod and all pods have sidecars
func (workload *Workload) HasIstioSidecar() bool {
	// if no pods we can't prove there is no sidecar, so return true
	if len(workload.Pods) == 0 {
		return true
	}
	// All pods in a deployment should be the same
	if workload.Type == "Deployment" {
		return workload.Pods[0].HasIstioSidecar()
	}
	// Need to check each pod
	return workload.Pods.HasIstioSidecar()
}

// HasIstioSidecar returns true if there is at least one workload which has a sidecar
func (workloads WorkloadOverviews) HasIstioSidecar() bool {
	if len(workloads) > 0 {
		for _, w := range workloads {
			if w.IstioSidecar {
				return true
			}
		}
	}
	return false
}

func (wl WorkloadList) GetLabels() []labels.Set {
	wLabels := make([]labels.Set, 0, len(wl.Workloads))
	for _, w := range wl.Workloads {
		wLabels = append(wLabels, labels.Set(w.Labels))
	}
	return wLabels
}
