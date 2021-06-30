package kubetest

import (
	apps_v1 "k8s.io/api/apps/v1"
	auth_v1 "k8s.io/api/authorization/v1"
	batch_v1 "k8s.io/api/batch/v1"
	batch_apps_v1 "k8s.io/api/batch/v1beta1"
	core_v1 "k8s.io/api/core/v1"

	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/util/httputil"
)

func (o *K8SClientMock) GetClusterServicesByLabels(labelsSelector string) ([]core_v1.Service, error) {
	args := o.Called(labelsSelector)
	return args.Get(0).([]core_v1.Service), args.Error(1)
}

func (o *K8SClientMock) GetConfigMap(namespace, name string) (*core_v1.ConfigMap, error) {
	args := o.Called(namespace, name)
	return args.Get(0).(*core_v1.ConfigMap), args.Error(1)
}

func (o *K8SClientMock) GetCronJobs(namespace string) ([]batch_apps_v1.CronJob, error) {
	args := o.Called(namespace)
	return args.Get(0).([]batch_apps_v1.CronJob), args.Error(1)
}

func (o *K8SClientMock) GetDaemonSet(namespace string, name string) (*apps_v1.DaemonSet, error) {
	args := o.Called(namespace, name)
	return args.Get(0).(*apps_v1.DaemonSet), args.Error(1)
}

func (o *K8SClientMock) GetDaemonSets(namespace string) ([]apps_v1.DaemonSet, error) {
	args := o.Called(namespace)
	return args.Get(0).([]apps_v1.DaemonSet), args.Error(1)
}

func (o *K8SClientMock) GetDeployment(namespace string, name string) (*apps_v1.Deployment, error) {
	args := o.Called(namespace, name)
	return args.Get(0).(*apps_v1.Deployment), args.Error(1)
}

func (o *K8SClientMock) GetDeployments(namespace string) ([]apps_v1.Deployment, error) {
	args := o.Called(namespace)
	return args.Get(0).([]apps_v1.Deployment), args.Error(1)
}

func (o *K8SClientMock) GetDeploymentsByLabel(namespace string, labelSelector string) ([]apps_v1.Deployment, error) {
	args := o.Called(namespace, labelSelector)
	return args.Get(0).([]apps_v1.Deployment), args.Error(1)
}

func (o *K8SClientMock) GetEndpoints(namespace string, name string) (*core_v1.Endpoints, error) {
	args := o.Called(namespace, name)
	return args.Get(0).(*core_v1.Endpoints), args.Error(1)
}

func (o *K8SClientMock) GetJobs(namespace string) ([]batch_v1.Job, error) {
	args := o.Called(namespace)
	return args.Get(0).([]batch_v1.Job), args.Error(1)
}

func (o *K8SClientMock) GetNamespace(namespace string) (*core_v1.Namespace, error) {
	args := o.Called(namespace)
	return args.Get(0).(*core_v1.Namespace), args.Error(1)
}

func (o *K8SClientMock) GetNamespaces(labelSelector string) ([]core_v1.Namespace, error) {
	args := o.Called(labelSelector)
	return args.Get(0).([]core_v1.Namespace), args.Error(1)
}

func (o *K8SClientMock) GetPods(namespace, labelSelector string) ([]core_v1.Pod, error) {
	args := o.Called(namespace, labelSelector)
	return args.Get(0).([]core_v1.Pod), args.Error(1)
}

func (o *K8SClientMock) GetPod(namespace, name string) (*core_v1.Pod, error) {
	args := o.Called(namespace, name)
	return args.Get(0).(*core_v1.Pod), args.Error(1)
}

func (o *K8SClientMock) GetPodLogs(namespace, name string, opts *core_v1.PodLogOptions) (*kubernetes.PodLogs, error) {
	args := o.Called(namespace, name, opts)
	return args.Get(0).(*kubernetes.PodLogs), args.Error(1)
}

func (o *K8SClientMock) GetPodPortForwarder(namespace, name, portMap string) (*httputil.PortForwarder, error) {
	args := o.Called(namespace, name, portMap)
	return args.Get(0).(*httputil.PortForwarder), args.Error(1)
}

func (o *K8SClientMock) GetReplicationControllers(namespace string) ([]core_v1.ReplicationController, error) {
	args := o.Called(namespace)
	return args.Get(0).([]core_v1.ReplicationController), args.Error(1)
}

func (o *K8SClientMock) GetReplicaSets(namespace string) ([]apps_v1.ReplicaSet, error) {
	args := o.Called(namespace)
	return args.Get(0).([]apps_v1.ReplicaSet), args.Error(1)
}

func (o *K8SClientMock) GetSecrets(namespace string, labelSelector string) ([]core_v1.Secret, error) {
	args := o.Called(namespace, labelSelector)
	return args.Get(0).([]core_v1.Secret), args.Error(1)
}

func (o *K8SClientMock) GetSelfSubjectAccessReview(namespace, api, resourceType string, verbs []string) ([]*auth_v1.SelfSubjectAccessReview, error) {
	args := o.Called(namespace, api, resourceType, verbs)
	return args.Get(0).([]*auth_v1.SelfSubjectAccessReview), args.Error(1)
}

func (o *K8SClientMock) GetService(namespace string, name string) (*core_v1.Service, error) {
	args := o.Called(namespace, name)
	return args.Get(0).(*core_v1.Service), args.Error(1)
}

func (o *K8SClientMock) GetServices(namespace string, selectorLabels map[string]string) ([]core_v1.Service, error) {
	args := o.Called(namespace, selectorLabels)
	return args.Get(0).([]core_v1.Service), args.Error(1)
}

func (o *K8SClientMock) GetServicesByLabels(namespace string, labelsSelector string) ([]core_v1.Service, error) {
	args := o.Called(namespace, labelsSelector)
	return args.Get(0).([]core_v1.Service), args.Error(1)
}

func (o *K8SClientMock) GetStatefulSet(namespace string, name string) (*apps_v1.StatefulSet, error) {
	args := o.Called(namespace, name)
	return args.Get(0).(*apps_v1.StatefulSet), args.Error(1)
}

func (o *K8SClientMock) GetStatefulSets(namespace string) ([]apps_v1.StatefulSet, error) {
	args := o.Called(namespace)
	return args.Get(0).([]apps_v1.StatefulSet), args.Error(1)
}

func (o *K8SClientMock) UpdateNamespace(namespace string, jsonPatch string) (*core_v1.Namespace, error) {
	args := o.Called(namespace, jsonPatch)
	return args.Get(0).(*core_v1.Namespace), args.Error(1)
}

func (o *K8SClientMock) UpdateWorkload(namespace string, name string, workloadType string, jsonPatch string) error {
	args := o.Called(namespace, name, workloadType, jsonPatch)
	return args.Error(1)
}

func (o *K8SClientMock) UpdateService(namespace string, name string, jsonPatch string) error {
	args := o.Called(namespace, name, jsonPatch)
	return args.Error(1)
}
