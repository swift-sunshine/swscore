package appender

import (
	"testing"

	osproject_v1 "github.com/openshift/api/project/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kiali/kiali/business"
	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/kubernetes/kubetest"
)

func setupTrafficMap() (map[string]*graph.Node, string, string, string, string, string, string) {
	trafficMap := graph.NewTrafficMap()

	appNode := graph.NewNode(graph.Unknown, "testNamespace", "ratings", "testNamespace", graph.Unknown, "ratings", "", graph.GraphTypeVersionedApp)
	appNode.Metadata[graph.DestServices] = graph.NewDestServicesMetadata().Add("testNamespace ratings", graph.ServiceName{Namespace: "testNamespace", Name: "ratings"})
	trafficMap[appNode.ID] = &appNode

	appNodeV1 := graph.NewNode(graph.Unknown, "testNamespace", "ratings", "testNamespace", "ratings-v1", "ratings", "v1", graph.GraphTypeVersionedApp)
	appNodeV1.Metadata[graph.DestServices] = graph.NewDestServicesMetadata().Add("testNamespace ratings", graph.ServiceName{Namespace: "testNamespace", Name: "ratings"})
	trafficMap[appNodeV1.ID] = &appNodeV1

	appNodeV2 := graph.NewNode(graph.Unknown, "testNamespace", "ratings", "testNamespace", "ratings-v2", "ratings", "v2", graph.GraphTypeVersionedApp)
	appNodeV2.Metadata[graph.DestServices] = graph.NewDestServicesMetadata().Add("testNamespace ratings", graph.ServiceName{Namespace: "testNamespace", Name: "ratings"})
	trafficMap[appNodeV2.ID] = &appNodeV2

	serviceNode := graph.NewNode(graph.Unknown, "testNamespace", "ratings", "testNamespace", graph.Unknown, graph.Unknown, graph.Unknown, graph.GraphTypeVersionedApp)
	trafficMap[serviceNode.ID] = &serviceNode

	workloadNode := graph.NewNode(graph.Unknown, "testNamespace", "ratings", "testNamespace", "ratings-v1", graph.Unknown, graph.Unknown, graph.GraphTypeWorkload)
	workloadNode.Metadata[graph.DestServices] = graph.NewDestServicesMetadata().Add("testNamespace ratings", graph.ServiceName{Namespace: "testNamespace", Name: "ratings"})
	trafficMap[workloadNode.ID] = &workloadNode

	fooServiceNode := graph.NewNode(graph.Unknown, "testNamespace", "foo", "testNamespace", graph.Unknown, graph.Unknown, graph.Unknown, graph.GraphTypeVersionedApp)
	trafficMap[fooServiceNode.ID] = &fooServiceNode

	return trafficMap, appNode.ID, appNodeV1.ID, appNodeV2.ID, serviceNode.ID, workloadNode.ID, fooServiceNode.ID
}

func TestCBAll(t *testing.T) {
	assert := assert.New(t)
	config.Set(config.NewConfig())

	k8s := kubetest.NewK8SClientMock()
	dRule := kubernetes.GenericIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: "dRule-1",
		},
		Spec: map[string]interface{}{
			"host":          "ratings",
			"trafficPolicy": map[string]interface{}{"connectionPool": true},
		},
	}
	k8s.On("GetProject", mock.AnythingOfType("string")).Return(&osproject_v1.Project{}, nil)
	k8s.On("GetProjects", mock.AnythingOfType("string")).Return([]osproject_v1.Project{}, nil)
	k8s.On("GetIstioObjects", mock.AnythingOfType("string"), "destinationrules", "").Return([]kubernetes.IstioObject{
		dRule.DeepCopyIstioObject(),
	}, nil)
	k8s.On("GetEndpoints", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&core_v1.Endpoints{}, nil)
	k8s.On("GetServices", mock.AnythingOfType("string"), mock.Anything).Return([]core_v1.Service{{}}, nil)
	k8s.On("GetIstioObjects", mock.AnythingOfType("string"), "virtualservices", "").Return([]kubernetes.IstioObject{}, nil)

	businessLayer := business.NewWithBackends(k8s, nil, nil)
	trafficMap, appNodeId, appNodeV1Id, appNodeV2Id, svcNodeId, wlNodeId, _ := setupTrafficMap()

	assert.Equal(6, len(trafficMap))
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasVS])

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := IstioAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	assert.Equal(6, len(trafficMap))
	assert.Equal(true, trafficMap[appNodeId].Metadata[graph.HasCB])
	assert.Equal(true, trafficMap[appNodeV1Id].Metadata[graph.HasCB])
	assert.Equal(true, trafficMap[appNodeV2Id].Metadata[graph.HasCB])
	assert.Equal(true, trafficMap[svcNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasVS])
}

func TestCBSubset(t *testing.T) {
	assert := assert.New(t)
	config.Set(config.NewConfig())

	k8s := kubetest.NewK8SClientMock()
	dRule := kubernetes.GenericIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: "dRule-1",
		},
		Spec: map[string]interface{}{
			"host": "ratings",
			"subsets": []interface{}{
				map[string]interface{}{
					"trafficPolicy": map[string]interface{}{"connectionPool": true},
					"labels":        map[string]interface{}{"version": "v1"},
				},
			},
		},
	}
	k8s.On("GetProject", mock.AnythingOfType("string")).Return(&osproject_v1.Project{}, nil)
	k8s.On("GetProjects", mock.AnythingOfType("string")).Return([]osproject_v1.Project{}, nil)
	k8s.On("GetIstioObjects", mock.AnythingOfType("string"), "destinationrules", "").Return([]kubernetes.IstioObject{
		dRule.DeepCopyIstioObject(),
	}, nil)
	k8s.On("GetEndpoints", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&core_v1.Endpoints{}, nil)
	k8s.On("GetServices", mock.AnythingOfType("string"), mock.Anything).Return([]core_v1.Service{{}}, nil)
	k8s.On("GetIstioObjects", mock.AnythingOfType("string"), "virtualservices", "").Return([]kubernetes.IstioObject{}, nil)

	businessLayer := business.NewWithBackends(k8s, nil, nil)
	trafficMap, appNodeId, appNodeV1Id, appNodeV2Id, svcNodeId, wlNodeId, _ := setupTrafficMap()

	assert.Equal(6, len(trafficMap))
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasVS])

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := IstioAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	assert.Equal(6, len(trafficMap))
	assert.Equal(true, trafficMap[appNodeId].Metadata[graph.HasCB])
	assert.Equal(true, trafficMap[appNodeV1Id].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasCB])
	assert.Equal(true, trafficMap[svcNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasVS])
}

func TestVS(t *testing.T) {
	assert := assert.New(t)
	config.Set(config.NewConfig())

	k8s := kubetest.NewK8SClientMock()
	vService := kubernetes.GenericIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: "vService-1",
		},
		Spec: map[string]interface{}{
			"hosts": []interface{}{
				"ratings",
			},
			"http": []interface{}{
				map[string]interface{}{
					"route": []interface{}{
						map[string]interface{}{
							"destination": map[string]interface{}{
								"host": "foo",
							},
						},
					},
				},
			},
		},
	}
	k8s.On("GetProject", mock.AnythingOfType("string")).Return(&osproject_v1.Project{}, nil)
	k8s.On("GetProjects", mock.AnythingOfType("string")).Return([]osproject_v1.Project{}, nil)
	k8s.On("GetIstioObjects", mock.AnythingOfType("string"), "destinationrules", "").Return([]kubernetes.IstioObject{}, nil)
	k8s.On("GetEndpoints", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&core_v1.Endpoints{}, nil)
	k8s.On("GetServices", mock.AnythingOfType("string"), mock.Anything).Return([]core_v1.Service{{}}, nil)
	k8s.On("GetIstioObjects", mock.AnythingOfType("string"), "virtualservices", "").Return([]kubernetes.IstioObject{
		vService.DeepCopyIstioObject(),
	}, nil)

	businessLayer := business.NewWithBackends(k8s, nil, nil)
	trafficMap, appNodeId, appNodeV1Id, appNodeV2Id, svcNodeId, wlNodeId, fooSvcNodeId := setupTrafficMap()

	assert.Equal(6, len(trafficMap))
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[fooSvcNodeId].Metadata[graph.HasVS])

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := IstioAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	assert.Equal(6, len(trafficMap))
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasCB])
	assert.Equal(nil, trafficMap[appNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV1Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[appNodeV2Id].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[svcNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[wlNodeId].Metadata[graph.HasVS])
	assert.Equal(true, trafficMap[fooSvcNodeId].Metadata[graph.HasVS])
}

func TestVSWithRoutingBadges(t *testing.T) {
	assert := assert.New(t)
	config.Set(config.NewConfig())

	k8s := kubetest.NewK8SClientMock()
	vService := kubernetes.GenericIstioObject{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: "vService-1",
			Labels: map[string]string{
				graph.WizardLabelKey: graph.WizardTrafficShiftingLabel,
			},
		},
		Spec: map[string]interface{}{
			"hosts": []interface{}{
				"ratings",
			},
			"http": []interface{}{
				map[string]interface{}{
					"route": []interface{}{
						map[string]interface{}{
							"destination": map[string]interface{}{
								"host": "foo",
							},
						},
					},
				},
			},
		},
	}
	k8s.On("GetProject", mock.AnythingOfType("string")).Return(&osproject_v1.Project{}, nil)
	k8s.On("GetProjects", mock.AnythingOfType("string")).Return([]osproject_v1.Project{}, nil)
	k8s.On("GetIstioObjects", mock.AnythingOfType("string"), "destinationrules", "").Return([]kubernetes.IstioObject{}, nil)
	k8s.On("GetEndpoints", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&core_v1.Endpoints{}, nil)
	k8s.On("GetServices", mock.AnythingOfType("string"), mock.Anything).Return([]core_v1.Service{{}}, nil)
	k8s.On("GetIstioObjects", mock.AnythingOfType("string"), "virtualservices", "").Return([]kubernetes.IstioObject{
		vService.DeepCopyIstioObject(),
	}, nil)

	businessLayer := business.NewWithBackends(k8s, nil, nil)
	trafficMap, _, _, _, _, _, fooSvcNodeId := setupTrafficMap()

	assert.Equal(nil, trafficMap[fooSvcNodeId].Metadata[graph.HasVS])
	assert.Equal(nil, trafficMap[fooSvcNodeId].Metadata[graph.HasTrafficShifting])

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := IstioAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	assert.Equal(true, trafficMap[fooSvcNodeId].Metadata[graph.HasTrafficShifting])
}
