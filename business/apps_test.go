package business

import (
	"testing"

	osapps_v1 "github.com/openshift/api/apps/v1"
	osproject_v1 "github.com/openshift/api/project/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apps_v1 "k8s.io/api/apps/v1"
	batch_v1 "k8s.io/api/batch/v1"
	batch_v1beta1 "k8s.io/api/batch/v1beta1"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes/kubetest"
	"github.com/kiali/kiali/prometheus/prometheustest"
)

func setupAppService(k8s *kubetest.K8SClientMock) AppService {
	prom := new(prometheustest.PromClientMock)
	layer := NewWithBackends(k8s, prom, nil)
	return AppService{k8s: k8s, prom: prom, businessLayer: layer}
}

func TestGetAppListFromDeployments(t *testing.T) {
	assert := assert.New(t)
	conf := config.NewConfig()
	config.Set(conf)

	// Setup mocks
	k8s := new(kubetest.K8SClientMock)
	// Auxiliar fake* tests defined in workload_test.go
	k8s.On("IsOpenShift").Return(true)
	// Not needed a result, just to not send an error to test this usecase
	k8s.On("GetProject", mock.AnythingOfType("string")).Return(&osproject_v1.Project{ObjectMeta: v1.ObjectMeta{Name: "Namespace"}}, nil)
	k8s.On("GetDeployments", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(FakeDeployments(), nil)
	k8s.On("GetDeploymentConfigs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]osapps_v1.DeploymentConfig{}, nil)
	k8s.On("GetReplicaSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.ReplicaSet{}, nil)
	k8s.On("GetReplicationControllers", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]core_v1.ReplicationController{}, nil)
	k8s.On("GetStatefulSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.StatefulSet{}, nil)
	k8s.On("GetDaemonSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.DaemonSet{}, nil)
	k8s.On("GetJobs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]batch_v1.Job{}, nil)
	k8s.On("GetCronJobs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]batch_v1beta1.CronJob{}, nil)
	k8s.On("GetPods", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]core_v1.Pod{}, nil)
	k8s.On("GetServices", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return([]core_v1.Service{}, nil)
	svc := setupAppService(k8s)

	appList, _ := svc.GetAppList("Namespace", false)

	assert.Equal("Namespace", appList.Namespace.Name)

	assert.Equal(1, len(appList.Apps))
	assert.Equal("httpbin", appList.Apps[0].Name)
}

func TestGetAppFromDeployments(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	k8s := new(kubetest.K8SClientMock)
	k8s.On("IsOpenShift").Return(true)
	k8s.On("GetProject", mock.AnythingOfType("string")).Return(&osproject_v1.Project{ObjectMeta: v1.ObjectMeta{Name: "Namespace"}}, nil)
	k8s.On("GetDeployments", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(FakeDeployments(), nil)
	k8s.On("GetDeploymentConfigs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]osapps_v1.DeploymentConfig{}, nil)
	k8s.On("GetReplicaSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.ReplicaSet{}, nil)
	k8s.On("GetReplicationControllers", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]core_v1.ReplicationController{}, nil)
	k8s.On("GetStatefulSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.StatefulSet{}, nil)
	k8s.On("GetDaemonSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.DaemonSet{}, nil)
	k8s.On("GetPods", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]core_v1.Pod{}, nil)
	k8s.On("GetJobs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]batch_v1.Job{}, nil)
	k8s.On("GetCronJobs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]batch_v1beta1.CronJob{}, nil)
	k8s.On("GetServices", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return(FakeServices(), nil)

	conf := config.NewConfig()
	conf.ExternalServices.CustomDashboards.Enabled = false
	config.Set(conf)
	svc := setupAppService(k8s)

	appDetails, _ := svc.GetApp("Namespace", "httpbin")

	assert.Equal("Namespace", appDetails.Namespace.Name)
	assert.Equal("httpbin", appDetails.Name)

	assert.Equal(2, len(appDetails.Workloads))
	assert.Equal("httpbin-v1", appDetails.Workloads[0].WorkloadName)
	assert.Equal("httpbin-v2", appDetails.Workloads[1].WorkloadName)
	assert.Equal(1, len(appDetails.ServiceNames))
	assert.Equal("httpbin", appDetails.ServiceNames[0])
}

func TestGetAppListFromReplicaSets(t *testing.T) {
	assert := assert.New(t)
	conf := config.NewConfig()
	config.Set(conf)

	// Setup mocks
	k8s := new(kubetest.K8SClientMock)
	// Auxiliar fake* tests defined in workload_test.go
	k8s.On("IsOpenShift").Return(true)
	k8s.On("GetProject", mock.AnythingOfType("string")).Return(&osproject_v1.Project{ObjectMeta: v1.ObjectMeta{Name: "Namespace"}}, nil)
	k8s.On("GetDeployments", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.Deployment{}, nil)
	k8s.On("GetDeploymentConfigs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]osapps_v1.DeploymentConfig{}, nil)
	k8s.On("GetReplicaSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(FakeReplicaSets(), nil)
	k8s.On("GetReplicationControllers", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]core_v1.ReplicationController{}, nil)
	k8s.On("GetStatefulSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.StatefulSet{}, nil)
	k8s.On("GetDaemonSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.DaemonSet{}, nil)
	k8s.On("GetJobs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]batch_v1.Job{}, nil)
	k8s.On("GetCronJobs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]batch_v1beta1.CronJob{}, nil)
	k8s.On("GetPods", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]core_v1.Pod{}, nil)
	k8s.On("GetServices", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return([]core_v1.Service{}, nil)
	svc := setupAppService(k8s)

	appList, _ := svc.GetAppList("Namespace", false)

	assert.Equal("Namespace", appList.Namespace.Name)

	assert.Equal(1, len(appList.Apps))
	assert.Equal("httpbin", appList.Apps[0].Name)
}

func TestGetAppFromReplicaSets(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	k8s := new(kubetest.K8SClientMock)
	k8s.On("IsOpenShift").Return(true)
	k8s.On("GetProject", mock.AnythingOfType("string")).Return(&osproject_v1.Project{ObjectMeta: v1.ObjectMeta{Name: "Namespace"}}, nil)
	k8s.On("GetDeployments", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.Deployment{}, nil)
	k8s.On("GetDeploymentConfigs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]osapps_v1.DeploymentConfig{}, nil)
	k8s.On("GetReplicaSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(FakeReplicaSets(), nil)
	k8s.On("GetReplicationControllers", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]core_v1.ReplicationController{}, nil)
	k8s.On("GetStatefulSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.StatefulSet{}, nil)
	k8s.On("GetDaemonSets", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]apps_v1.DaemonSet{}, nil)
	k8s.On("GetPods", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]core_v1.Pod{}, nil)
	k8s.On("GetJobs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]batch_v1.Job{}, nil)
	k8s.On("GetCronJobs", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]batch_v1beta1.CronJob{}, nil)
	k8s.On("GetServices", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return(FakeServices(), nil)

	conf := config.NewConfig()
	conf.ExternalServices.CustomDashboards.Enabled = false
	config.Set(conf)

	svc := setupAppService(k8s)

	appDetails, _ := svc.GetApp("Namespace", "httpbin")

	assert.Equal("Namespace", appDetails.Namespace.Name)
	assert.Equal("httpbin", appDetails.Name)

	assert.Equal(2, len(appDetails.Workloads))
	assert.Equal("httpbin-v1", appDetails.Workloads[0].WorkloadName)
	assert.Equal("httpbin-v2", appDetails.Workloads[1].WorkloadName)
	assert.Equal(1, len(appDetails.ServiceNames))
	assert.Equal("httpbin", appDetails.ServiceNames[0])
}

func TestJoinMap(t *testing.T) {
	assert := assert.New(t)
	tempLabels := map[string][]string{}
	labelsA := map[string]string{
		"key1": "val1",
		"key2": "val2",
	}

	joinMap(tempLabels, labelsA)
	assert.Len(tempLabels, 2)
	assert.Equal([]string{"val1"}, tempLabels["key1"])
	assert.Equal([]string{"val2"}, tempLabels["key2"])

	// Test with an added value on key1
	labelsB := map[string]string{
		"key1": "val3",
		"key3": "val4",
	}
	joinMap(tempLabels, labelsB)
	assert.Len(tempLabels, 3)
	assert.Equal([]string{"val1", "val3"}, tempLabels["key1"])
	assert.Equal([]string{"val2"}, tempLabels["key2"])
	assert.Equal([]string{"val4"}, tempLabels["key3"])

	// Test with duplicates; val3 is duplicated, al4 is not (is substring)
	// al4 must also appear before val4 on final labels (sorted)
	labelsC := map[string]string{
		"key1": "val3",
		"key3": "al4",
	}
	joinMap(tempLabels, labelsC)
	assert.Len(tempLabels, 3)
	assert.Equal([]string{"val1", "val3"}, tempLabels["key1"])
	assert.Equal([]string{"val2"}, tempLabels["key2"])
	assert.Equal([]string{"val4", "al4"}, tempLabels["key3"])

	labels := buildFinalLabels(tempLabels)
	assert.Len(labels, 3)
	assert.Equal("val1,val3", labels["key1"])
	assert.Equal("val2", labels["key2"])
	assert.Equal("al4,val4", labels["key3"])
}
