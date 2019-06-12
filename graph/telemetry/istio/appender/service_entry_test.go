package appender

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kiali/kiali/business"
	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/kubernetes/kubetest"
)

func setupServiceEntries() *business.Layer {
	k8s := kubetest.NewK8SClientMock()

	externalEntry := kubernetes.GenericIstioObject{
		Spec: map[string]interface{}{
			"hosts":    []interface{}{"ExternalServiceEntry"},
			"location": "MESH_EXTERNAL",
		},
	}
	externalWildcardEntry := kubernetes.GenericIstioObject{
		Spec: map[string]interface{}{
			"hosts":    []interface{}{"*.external.com"},
			"location": "MESH_EXTERNAL",
		},
	}
	internalEntry := kubernetes.GenericIstioObject{
		Spec: map[string]interface{}{
			"hosts":    []interface{}{"InternalServiceEntry"},
			"location": "MESH_INTERNAL",
		},
	}
	internalPrefixEntry := kubernetes.GenericIstioObject{
		Spec: map[string]interface{}{
			"hosts":    []interface{}{"InternalPrefixServiceEntry.namespace.svc.cluster.local"},
			"location": "MESH_INTERNAL",
		},
	}
	defaultEntry := kubernetes.GenericIstioObject{
		Spec: map[string]interface{}{
			"hosts": []interface{}{"DefaultServiceEntry"},
		},
	}

	k8s.On("GetServiceEntries", mock.AnythingOfType("string")).Return([]kubernetes.IstioObject{
		&externalEntry,
		&externalWildcardEntry,
		&internalEntry,
		&internalPrefixEntry,
		&defaultEntry}, nil)
	config.Set(config.NewConfig())

	businessLayer := business.NewWithBackends(k8s, nil)
	return businessLayer
}

func TestServiceEntry(t *testing.T) {
	assert := assert.New(t)

	businessLayer := setupServiceEntries()
	trafficMap := serviceEntriesTrafficMap()

	assert.Equal(8, len(trafficMap))
	notServiceEntryID, _ := graph.Id("testNamespace", "NotServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)
	notServiceEntryNode, found := trafficMap[notServiceEntryID]
	assert.Equal(true, found)
	assert.Equal(1, len(notServiceEntryNode.Edges))
	assert.Equal(nil, notServiceEntryNode.Metadata[graph.IsServiceEntry])

	extServiceEntryID, _ := graph.Id("testNamespace", "ExternalServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)
	extServiceEntryNode, found2 := trafficMap[extServiceEntryID]
	assert.Equal(true, found2)
	assert.Equal(0, len(extServiceEntryNode.Edges))
	assert.Equal(nil, extServiceEntryNode.Metadata[graph.IsServiceEntry])

	extWildcardServiceEntryID, _ := graph.Id("testNamespace", "foo.external.com", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)
	extWildcardServiceEntryNode, found3 := trafficMap[extWildcardServiceEntryID]
	assert.Equal(true, found3)
	assert.Equal(0, len(extWildcardServiceEntryNode.Edges))
	assert.Equal(nil, extWildcardServiceEntryNode.Metadata[graph.IsServiceEntry])

	intServiceEntryID, _ := graph.Id("testNamespace", "InternalServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)
	intServiceEntryNode, found4 := trafficMap[intServiceEntryID]
	assert.Equal(true, found4)
	assert.Equal(0, len(intServiceEntryNode.Edges))
	assert.Equal(nil, intServiceEntryNode.Metadata[graph.IsServiceEntry])

	intPrefixServiceEntryID, _ := graph.Id("testNamespace", "InternalPrefixServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)
	intPrefixServiceEntryNode, found5 := trafficMap[intPrefixServiceEntryID]
	assert.Equal(true, found5)
	assert.Equal(0, len(intPrefixServiceEntryNode.Edges))
	assert.Equal(nil, intPrefixServiceEntryNode.Metadata[graph.IsServiceEntry])

	defaultServiceEntryID, _ := graph.Id("testNamespace", "DefaultServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)
	defaultServiceEntryNode, found6 := trafficMap[defaultServiceEntryID]
	assert.Equal(true, found6)
	assert.Equal(0, len(defaultServiceEntryNode.Edges))
	assert.Equal(nil, defaultServiceEntryNode.Metadata[graph.IsServiceEntry])

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := ServiceEntryAppender{
		AccessibleNamespaces: map[string]time.Time{"testNamespace": time.Now()},
	}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	assert.Equal(nil, notServiceEntryNode.Metadata[graph.IsServiceEntry])
	assert.Equal("MESH_EXTERNAL", extServiceEntryNode.Metadata[graph.IsServiceEntry])
	assert.Equal("MESH_EXTERNAL", extWildcardServiceEntryNode.Metadata[graph.IsServiceEntry])
	assert.Equal("MESH_INTERNAL", intServiceEntryNode.Metadata[graph.IsServiceEntry])
	assert.Equal("MESH_INTERNAL", intPrefixServiceEntryNode.Metadata[graph.IsServiceEntry])
	assert.Equal("MESH_EXTERNAL", defaultServiceEntryNode.Metadata[graph.IsServiceEntry])
}

func serviceEntriesTrafficMap() map[string]*graph.Node {
	trafficMap := make(map[string]*graph.Node)

	n0 := graph.NewNode(graph.Unknown, "", graph.Unknown, graph.Unknown, graph.Unknown, graph.Unknown, graph.GraphTypeVersionedApp)

	n1 := graph.NewNode("testNamespace", "NotServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)

	n2 := graph.NewNode("testNamespace", "NotServiceEntry", "testNamespace", "TestWorkload-v1", "TestApp", "v1", graph.GraphTypeVersionedApp)

	n3 := graph.NewNode("testNamespace", "ExternalServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)

	n4 := graph.NewNode("testNamespace", "foo.external.com", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)

	n5 := graph.NewNode("testNamespace", "InternalServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)

	n6 := graph.NewNode("testNamespace", "InternalPrefixServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)

	n7 := graph.NewNode("testNamespace", "DefaultServiceEntry", "testNamespace", "", "", "", graph.GraphTypeVersionedApp)

	trafficMap[n0.ID] = &n0
	trafficMap[n1.ID] = &n1
	trafficMap[n2.ID] = &n2
	trafficMap[n3.ID] = &n3
	trafficMap[n4.ID] = &n4
	trafficMap[n5.ID] = &n5
	trafficMap[n6.ID] = &n6
	trafficMap[n7.ID] = &n7

	n0.AddEdge(&n1)
	n1.AddEdge(&n2)
	n2.AddEdge(&n3)
	n2.AddEdge(&n4)
	n2.AddEdge(&n5)
	n2.AddEdge(&n6)
	n2.AddEdge(&n7)

	return trafficMap
}
