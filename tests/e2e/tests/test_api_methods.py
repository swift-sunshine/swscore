import warnings

import pytest
import tests.conftest as conftest
import calendar
import time
from utils.common_utils import common_utils

gmt = time.gmtime
INVALID_PATH_NAMESPACE_WORKLOAD = {'namespace': 'invalid', 'workload':'details-v1'}
INVALID_PATH_WORKLOAD_WORKLOAD = {'namespace': 'bookinfo', 'workload':'invalid'}
INVALID_PATH_WORKLOAD = {'namespace': 'invalid', 'workload':'invalid'}
INVALID_PATH_NAMESPACE_SERVICE = {'namespace': 'invalid', 'service':'details'}
INVALID_PATH_SERVICE_SERVICE = {'namespace': 'bookinfo', 'service':'invalid'}
INVALID_PATH_SERVICE = {'namespace': 'invalid', 'service':'invalid'}
INVALID_PARAMS_STARTMICROS = {'startMicros': 'invalid' }
INVALID_PATH_NAMESPACE_APP = {'namespace': 'invalid', 'app':'details'}
INVALID_PATH_APP_APP = {'namespace': 'bookinfo', 'app':'invalid'}
INVALID_PATH_APP = {'namespace': 'invalid', 'app':'invalid'}

@pytest.fixture(scope="session", autouse=True)
def before_all_tests(kiali_client):
    global swagger_method_list, tested_method_list, control_plane_namespace
    control_plane_namespace = conftest.get_control_plane_namespace()
    swagger = kiali_client.swagger_parser.swagger
    swagger_method_list= []
    tested_method_list = ['root','jaegerInfo', 'grafanaInfo', 'getPermissions', 'getStatus', 'getConfig', 'authenticate',
                          'namespaceList', 'namespaceMetrics','namespaceHealth','istioStatus','iter8Experiments',
                          'istioConfigList', 'istioConfigDetails', 'istioConfigCreate', 'istioConfigDelete', 'objectValidations', ''
                          'serviceList', 'serviceDetails', 'serviceMetrics', 'serviceHealth',
                          'appHealth', 'appList', 'appDetails', 'appMetrics',
                          'workloadList', 'workloadDetails', 'workloadHealth', 'workloadMetrics',
                          'graphNamespaces', 'graphService', 'graphWorkload', 'graphApp', 'graphAppVersion',
                          'istioConfigDetailsSubtype', 'serviceDashboard', 'workloadDashboard', 'appDashboard',
                          'authenticationInfo', 'openshiftCheckToken', 'customDashboard', 'podDetails', 'podProxyDump', 'podProxyResource', 'podLogs',
                          'namespaceTls', 'getThreeScaleInfo', 'getThreeScaleHandlers', 'getThreeScaleService',
                          'meshTls', 'namespaceValidations', 'appSpans', 'appTraces', 'serviceTraces', 'workloadSpans', 'workloadTraces', 'serviceSpans']

    for key in swagger.operation:
        swagger_method_list.append(key)



def get_method_from_method_list(method_name):
    try:
        return tested_method_list[tested_method_list.index(method_name)]
    except ValueError:
        pytest.fail('Method not available on Tested Method List')

def get_kiali_version(kiali_client):
    try:
        response = common_utils.get_response(kiali_client, method_name='getStatus', path={})
        kiali_version = response.json().get('status')
    except AssertionError:
        pytest.fail(response.content)

    return kiali_version

def __test_swagger_coverage():
    difference_set = set(swagger_method_list) - set(tested_method_list)
    if len(difference_set) > 0:
        pytest.fail('Missing {0} Api Methods to Validate:'.format(str(len(difference_set))) + str(difference_set))
    else:
        pass

def test_swagger_double_api(kiali_client):
    swagger = kiali_client.swagger_parser.swagger

    for key, value in swagger.operation.items():
        assert not 'api/api' in value[0]

def test_root(kiali_client):
    common_utils.get_response(kiali_client, method_name='root')

def test_virtualservices(kiali_client):
    data = '{"metadata":{"namespace":"bookinfo","name":"reviews","labels":{"kiali_wizard":"weighted_routing"}},"spec":{"http":[{"route":[{"destination":{"host":"reviews","subset":"v1"},"weight":75},{"destination":{"host":"reviews","subset":"v2"},"weight":13},{"destination":{"host":"reviews","subset":"v3"},"weight":12}]}],"hosts":["reviews"],"gateways":null}}'    
    common_utils.get_response(kiali_client, method_name='istioConfigCreate', path={'namespace': 'bookinfo', 'object_type': 'virtualservices'}, data=data, http_method='POST')
    common_utils.get_response(kiali_client, method_name='istioConfigDelete', path={'namespace': 'bookinfo', 'object_type': 'virtualservices', 'object': 'reviews'}, http_method='DELETE')

def test_jaeger_info(kiali_client):
    response = kiali_client.request(method_name='jaegerInfo', path=None, params=None)
    if response.status_code == 503:
        pytest.skip()

    assert response.status_code == 200

def test_authentication_info(kiali_client):
    common_utils.get_response(kiali_client, method_name='authenticationInfo')

def test_openshift_checkToken(kiali_client):
    if conftest.get_kiali_auth_method() == "oauth":
        pytest.skip()
    else:
        common_utils.get_response(kiali_client, method_name='openshiftCheckToken')

def test_namespace_tls(kiali_client):
    common_utils.get_response(kiali_client, method_name='namespaceTls', path={'namespace': control_plane_namespace})

def test_pod_details(kiali_client):
    pod_id = common_utils.get_pod_id(kiali_client, namespace=control_plane_namespace, pod_name='kiali')
    common_utils.get_response(kiali_client, method_name='podDetails', path={'namespace': control_plane_namespace, 'pod': pod_id})

def test_pod_proxy_dump(kiali_client):
    pod_id = common_utils.get_pod_id(kiali_client, namespace='bookinfo', pod_name='productpage-v1')
    common_utils.get_response(kiali_client, method_name='podProxyDump', path={'namespace': 'bookinfo', 'pod': pod_id, 'object': 'config_dump'})

def test_pod_proxy_resource(kiali_client):
    pod_id = common_utils.get_pod_id(kiali_client, namespace='bookinfo', pod_name='productpage-v1')
    common_utils.get_response(kiali_client, method_name='podProxyResource', path={'namespace': 'bookinfo', 'pod': pod_id, 'object': 'config_dump', 'resource': 'resource'})

def test_pod_logs(kiali_client):
    pod_id = common_utils.get_pod_id(kiali_client, namespace=control_plane_namespace, pod_name='kiali')
    common_utils.get_response(kiali_client, method_name='podLogs', path={'namespace': control_plane_namespace, 'pod': pod_id})

def test_grafana_info(kiali_client):
    common_utils.get_response(kiali_client, method_name='grafanaInfo')
    
def test_get_permissions(kiali_client):
    common_utils.get_response(kiali_client, method_name='getPermissions')
    
def test_iter8_experiments(kiali_client):
    common_utils.get_response(kiali_client, method_name='iter8Experiments')

def test_istio_status(kiali_client):
    common_utils.get_response(kiali_client, method_name='istioStatus')  
    
def test_get_status(kiali_client):
    common_utils.get_response(kiali_client, method_name='getStatus')

def test_get_config(kiali_client):
    common_utils.get_response(kiali_client, method_name='getConfig')

def test_get_token(kiali_client):
    if conftest.get_kiali_auth_method() == "oauth":
        pytest.skip()
    else:
        common_utils.get_response(kiali_client, method_name='authenticate')


def test_namespace_list(kiali_client):
    common_utils.get_response(kiali_client, method_name='namespaceList')

def test_namespace_metrics(kiali_client):
    common_utils.get_response(kiali_client, method_name='namespaceMetrics', path={'namespace': control_plane_namespace})


def test_namespace_health(kiali_client):
    common_utils.get_response(kiali_client, method_name='namespaceHealth', path={'namespace': control_plane_namespace})


def test_istio_config_list(kiali_client):
    common_utils.get_response(kiali_client, method_name='istioConfigList', path={'namespace': control_plane_namespace})


def __test_istio_config_details(kiali_client):
    common_utils.get_response(kiali_client, method_name='istioConfigDetails', path={'namespace': control_plane_namespace, 'object_type': 'rules', 'object': 'threescale'})

def __test_istio_config_details_subtype(kiali_client):
    common_utils.get_response(kiali_client, method_name='istioConfigDetailsSubtype', path={'namespace': control_plane_namespace, 'object_type': 'gateways', 'object_subtype': 'ingressgateway', 'object': 'ingressgateway'} )

def test_service_list(kiali_client):
    common_utils.get_response(kiali_client, method_name='serviceList', path={'namespace': control_plane_namespace})


def test_service_details(kiali_client):
    common_utils.get_response(kiali_client, method_name='serviceDetails',
                                           path={'namespace': control_plane_namespace, 'service': 'kiali'})

def test_service_metrics(kiali_client):
    common_utils.get_response(kiali_client, method_name='serviceMetrics',
                                        path={'namespace': control_plane_namespace, 'service': 'kiali'})

def test_service_health(kiali_client):
    common_utils.get_response(kiali_client, method_name='serviceHealth', path={'namespace': control_plane_namespace, 'service': 'kiali'})


def test_app_list(kiali_client):
    common_utils.get_response(kiali_client, method_name='appList', path={'namespace': control_plane_namespace})

def test_app_metrics(kiali_client):
    common_utils.get_response(kiali_client,method_name='appMetrics', path={'namespace': control_plane_namespace, 'app': 'kiali'})


def test_app_details(kiali_client):
    common_utils.get_response(kiali_client, method_name='appDetails', path={'namespace': control_plane_namespace, 'app': 'kiali'})

def test_app_health(kiali_client):
    common_utils.get_response(kiali_client, method_name='appHealth', path={'namespace': control_plane_namespace, 'app': 'kiali'})

def test_workload_list(kiali_client):
    common_utils.get_response(kiali_client, method_name='workloadList', path={'namespace': control_plane_namespace})


def test_workload_details(kiali_client):
    common_utils.get_response(kiali_client, method_name='workloadDetails', path={'namespace': 'bookinfo', 'workload':'details-v1'})

def test_workload_health(kiali_client):
    common_utils.get_response(kiali_client, method_name='workloadHealth', path={'namespace': 'bookinfo', 'workload':'details-v1'})

def test_workload_metrics(kiali_client):
    common_utils.get_response(kiali_client, method_name='workloadMetrics', path={'namespace': 'bookinfo', 'workload':'details-v1'})

def test_graph_namespaces(kiali_client):
    VERSIONED_APP_PARAMS = {'namespaces': 'bookinfo', 'graphType': 'versionedApp', 'duration': '60s'}
    WORKLOAD_PARAMS = {'namespaces': 'bookinfo', 'graphType': 'workload', 'duration': '60s'}
    APP_PARAMS = {'namespaces': 'bookinfo','graphType': 'app', 'duration': '60s'}

    common_utils.get_response(kiali_client, method_name='graphNamespaces', params=VERSIONED_APP_PARAMS)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', params=WORKLOAD_PARAMS)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', params=APP_PARAMS)


def test_graph_service(kiali_client):
    GRAPH_SERVICE_PATH = {'namespace': 'bookinfo', 'service': 'mongodb'}
    common_utils.get_response(kiali_client, method_name='graphService', path=GRAPH_SERVICE_PATH)


def test_graph_workload(kiali_client):
    GRAPH_WORKLOAD_PATH = {'namespace': 'bookinfo', 'workload': 'mongodb-v1'}
    common_utils.get_response(kiali_client, method_name='graphWorkload', path=GRAPH_WORKLOAD_PATH)

def test_graph_app_and_graph_app_version(kiali_client):
    GRAPH_APP_PARAMS_NOT_VALID = {'graphType': 'notValid'}
    GRAPH_APP_PARAMS_APP = {'graphType': 'app'}
    GRAPH_APP_PARAMS_VERSION = {'graphType': 'versionedApp'}
    GRAPH_APP_PARAMS_WORKLOAD = {'graphType': 'workload'}
    GRAPH_APP_PARAMS_SERVICE = {'graphType': 'service'}
    
    
    GRAPH_APP_PATH = {'namespace': 'bookinfo', 'app': 'reviews'}

    for method_name in ['graphApp', 'graphAppVersion']:

        # Default Request (It is expected because the graphType is set to Workload as default it will fail for default case)
        common_utils.get_response(kiali_client, method_name=method_name, path=GRAPH_APP_PATH, status_code_expected=400)

        # Workload (also equals to Default)
        common_utils.get_response(kiali_client, method_name=method_name, path=GRAPH_APP_PATH, status_code_expected=400,
                        params=GRAPH_APP_PARAMS_WORKLOAD)

        # Service
        common_utils.get_response(kiali_client, method_name=method_name, path=GRAPH_APP_PATH, status_code_expected=400,
                        params=GRAPH_APP_PARAMS_SERVICE)

        # Invalid Value
        common_utils.get_response(kiali_client, method_name=method_name, path=GRAPH_APP_PATH,
                        params=GRAPH_APP_PARAMS_NOT_VALID, status_code_expected=400)

        # App Graph
        common_utils.get_response(kiali_client, method_name=method_name, path=GRAPH_APP_PATH, status_code_expected=200,
                        params=GRAPH_APP_PARAMS_APP)

        # Versioned App Graph
        common_utils.get_response(kiali_client, method_name=method_name, path=GRAPH_APP_PATH, status_code_expected=200,
                        params=GRAPH_APP_PARAMS_VERSION)


def test_service_dashboard(kiali_client):
    SERVICE_DASHBOARD_PATH = {'namespace': 'bookinfo', 'service': 'details'}
    common_utils.get_response(kiali_client, method_name='serviceDashboard', path=SERVICE_DASHBOARD_PATH)

def test_workload_dashboard(kiali_client):
    WORKLOAD_DASHBOARD_PATH = {'namespace': 'bookinfo', 'workload':'details-v1'}
    common_utils.get_response(kiali_client, method_name='workloadDashboard', path=WORKLOAD_DASHBOARD_PATH)

def test_app_dashboard(kiali_client):
    APP_DASHBOARD_PATH = {'namespace': 'bookinfo', 'app':'ratings'}
    common_utils.get_response(kiali_client, method_name='appDashboard', path=APP_DASHBOARD_PATH)

def __test_threescale_info(kiali_client):
    common_utils.get_response(kiali_client, method_name='getThreeScaleInfo')

def __test_threescale_handelers(kiali_client):
    common_utils.get_response(kiali_client, method_name='getThreeScaleHandlers')

def __test_threescale_service(kiali_client):
    common_utils.get_response(kiali_client, method_name='getThreeScaleService')

def test_mesh_tls(kiali_client):
    common_utils.get_response(kiali_client, method_name='meshTls')

def test_namespace_validations(kiali_client):
    if 'v1.0' in get_kiali_version(kiali_client).get('Kiali core version'):
        pytest.skip()

    common_utils.get_response(kiali_client, method_name='namespaceValidations', path={'namespace': 'bookinfo'})

def test_namespace_spans_list(kiali_client):
    if 'v1.0' in get_kiali_version(kiali_client).get('Kiali core version'):
        pytest.skip()

    common_utils.get_response(kiali_client, method_name='appSpans', path={'namespace': 'bookinfo', 'app': 'details'}, params={'startMicros': calendar.timegm(gmt())})

def test_namespace_traces_list(kiali_client):
    if 'v1.0' in get_kiali_version(kiali_client).get('Kiali core version'):
        pytest.skip()

    common_utils.get_response(kiali_client, method_name='appTraces', path={'namespace': 'bookinfo', 'app': 'details'}, params={'startMicros': calendar.timegm(gmt()) })
                                                                  
def test_service_traces_list(kiali_client):
    
    common_utils.get_response(kiali_client, method_name='serviceTraces', path={'namespace': 'bookinfo', 'service':'details'}, params={'startMicros': calendar.timegm(gmt()) })

def test_service_spans_list(kiali_client):
    
    common_utils.get_response(kiali_client, method_name='serviceSpans', path={'namespace': 'bookinfo', 'service':'details'}, params={'startMicros': calendar.timegm(gmt()) })

def test_workload_traces_list(kiali_client):
    
    common_utils.get_response(kiali_client, method_name='workloadTraces', path={'namespace': 'bookinfo', 'workload':'details-v1'}, params={'startMicros': calendar.timegm(gmt()) })
    
def test_workload_spans_list(kiali_client):

    common_utils.get_response(kiali_client, method_name='workloadSpans', path={'namespace': 'bookinfo', 'workload':'details-v1'}, params={'startMicros': calendar.timegm(gmt()) })


def test_invalid_versioned_app_graphnamespaces_negative_400(kiali_client):
    
    INVALID_VERSIONED_APP_DURATION_GRAPHNAMESPACES  =   {'graphType': 'versionedApp', 'duration': 'invalid',  'namespaces':'bookinfo'}
    INVALID_VERSIONED_APP_NAMESPACE_GRAPHNAMESPACES =   {'graphType': 'versionedApp', 'duration': '60s',  'namespaces':'invalid'} 
    INVALID_VERSIONED_APP_DURATION_NAMESPACE_GRAPHNAMESPACES =   {'graphType': 'versionedApp', 'duration': 'invalid',  'namespaces':'invalid'} 

    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_VERSIONED_APP_DURATION_GRAPHNAMESPACES, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_VERSIONED_APP_NAMESPACE_GRAPHNAMESPACES, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_VERSIONED_APP_DURATION_NAMESPACE_GRAPHNAMESPACES, status_code_expected=400)
    
def test_invalid_app_graphnamespaces_negative_400(kiali_client):
    
    INVALID_APP_DURATION_GRAPHNAMESPACES  =   {'graphType': 'app', 'duration': 'invalid',  'namespaces':'bookinfo'}
    INVALID_APP_NAMESPACE_GRAPHNAMESPACES =   {'graphType': 'app', 'duration': '60s',  'namespaces':'invalid'} 
    INVALID_APP_DURATION_NAMESPACE_GRAPHNAMESPACES =   {'graphType': 'app', 'duration': 'invalid',  'namespaces':'invalid'} 

    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_APP_DURATION_GRAPHNAMESPACES, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_APP_NAMESPACE_GRAPHNAMESPACES, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_APP_DURATION_NAMESPACE_GRAPHNAMESPACES, status_code_expected=400)

def test_invalid_service_graphnamespaces_negative_400(kiali_client):
    
    INVALID_SERVICE_DURATION_GRAPHNAMESPACES  =   {'graphType': 'service', 'duration': 'invalid',  'namespaces':'bookinfo'}
    INVALID_SERVICE_NAMESPACE_GRAPHNAMESPACES =   {'graphType': 'service', 'duration': '60s',  'namespaces':'invalid'} 
    INVALID_SERVICE_DURATION_NAMESPACE_GRAPHNAMESPACES =   {'graphType': 'service', 'duration': 'invalid',  'namespaces':'invalid'} 

    INVALID_GRAPHTYPE_GRAPHNAMESPACES =      {'graphType': 'invalid', 'duration': '60s', 'namespaces':'bookinfo'}
    INVALID_GRAPHTYPE_DURATION_GRAPHNAMESPACES  =   {'graphType': 'invalid', 'duration': 'invalid', 'namespaces':'bookinfo'}
    INVALID_GRAPHTYPE_NAMESPACE_GRAPHNAMESPACES =   {'graphType': 'invalid', 'duration': '60s', 'namespaces':'invalid'}
    INVALID_GRAPHNAMESPACES =   {'graphType': 'invalid', 'duration': 'invalid', 'namespaces':'invalid'}


    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_SERVICE_DURATION_GRAPHNAMESPACES, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_SERVICE_NAMESPACE_GRAPHNAMESPACES, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_SERVICE_DURATION_NAMESPACE_GRAPHNAMESPACES, status_code_expected=400)

    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_GRAPHTYPE_GRAPHNAMESPACES, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_GRAPHTYPE_DURATION_GRAPHNAMESPACES, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_GRAPHTYPE_NAMESPACE_GRAPHNAMESPACES, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='graphNamespaces', path=INVALID_GRAPHNAMESPACES, status_code_expected=400)


def test_negative_400(kiali_client):
    
    INVALID_PARAMS_ISTIOCONFIGDETAILS = {'namespace': 'invalid', 'object_type': 'invalid', 'object': 'promtcp'}

    common_utils.get_response(kiali_client, method_name='istioConfigDetails', path=INVALID_PARAMS_ISTIOCONFIGDETAILS, status_code_expected=400)


def test_negative_404(kiali_client):
    INVALID_PARAMS_SERVICEDETAILS = {'namespace': control_plane_namespace, 'service': 'invalid'}
    INVALID_PARAMS_WORKLOADHEALTH = {'namespace': 'bookinfo', 'workload': 'invalid'}

    common_utils.get_response(kiali_client, method_name='serviceDetails', path=INVALID_PARAMS_SERVICEDETAILS, status_code_expected=404)
    common_utils.get_response(kiali_client, method_name='workloadHealth', path=INVALID_PARAMS_WORKLOADHEALTH, status_code_expected=404)

def test_negative_403(kiali_client):
    if 'v1.0' in get_kiali_version(kiali_client).get('Kiali core version'):
        pytest.skip()

    INVALID_PARAMS_APPDETAILS = {'namespace': 'invalid', 'app': 'ratings'}
    INVALID_PARAMS_WORKLOADDETAILS = {'namespace': 'invalid', 'workload': 'details-v1'}
    INVALID_PARAMS_SERVICEHEALTH = {'namespace': 'invalid', 'service': 'kiali'}
    INVALID_PARAMS_APPHEALTH = {'namespace': 'invalid', 'app': 'kiali'}
    INVALID_PARAMS_WORKLOADHEALTH = {'namespace': 'invalid', 'workload': 'details-v1'}

    common_utils.get_response(kiali_client, method_name='appDetails', path=INVALID_PARAMS_APPDETAILS, status_code_expected=403)
    common_utils.get_response(kiali_client, method_name='workloadDetails', path=INVALID_PARAMS_WORKLOADDETAILS, status_code_expected=403)
    common_utils.get_response(kiali_client, method_name='serviceHealth', path=INVALID_PARAMS_SERVICEHEALTH, status_code_expected=403)
    common_utils.get_response(kiali_client, method_name='appHealth', path=INVALID_PARAMS_APPHEALTH, status_code_expected=403)
    common_utils.get_response(kiali_client, method_name='workloadHealth', path=INVALID_PARAMS_WORKLOADHEALTH, status_code_expected=403)

def test_negative_500(kiali_client):    
    INVALID_PARAMS_SERVICEDETAILS = {'namespace': 'invalid', 'service': 'kiali'}

    common_utils.get_response(kiali_client, method_name='serviceDetails', path=INVALID_PARAMS_SERVICEDETAILS, status_code_expected=500)

def test_invalid_workload_traces_list_negative_503(kiali_client):
     
 	common_utils.get_response(kiali_client, method_name='workloadTraces', path=INVALID_PATH_NAMESPACE_WORKLOAD, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)
 	common_utils.get_response(kiali_client, method_name='workloadTraces', path=INVALID_PATH_WORKLOAD_WORKLOAD, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)
 	common_utils.get_response(kiali_client, method_name='workloadTraces', path=INVALID_PATH_WORKLOAD, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)

def test_invalid_workload_traces_list_negative_400(kiali_client):
    
	common_utils.get_response(kiali_client, method_name='workloadTraces', path={'namespace': 'bookinfo', 'workload':'details-v1'}, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
	common_utils.get_response(kiali_client, method_name='workloadTraces', path=INVALID_PATH_WORKLOAD_WORKLOAD, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
	common_utils.get_response(kiali_client, method_name='workloadTraces', path=INVALID_PATH_NAMESPACE_WORKLOAD, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
	common_utils.get_response(kiali_client, method_name='workloadTraces', path=INVALID_PATH_WORKLOAD, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)

def test_invalid_workload_spans_list_negative_400(kiali_client):
    
    common_utils.get_response(kiali_client, method_name='workloadSpans', path={'namespace': 'bookinfo', 'workload':'details-v1'}, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='workloadSpans', path=INVALID_PATH_WORKLOAD_WORKLOAD, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='workloadSpans', path=INVALID_PATH_NAMESPACE_WORKLOAD, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='workloadSpans', path=INVALID_PATH_WORKLOAD, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)

def test_invalid_workload_spans_list_negative_503(kiali_client):
   	
    common_utils.get_response(kiali_client, method_name='workloadSpans', path=INVALID_PATH_NAMESPACE_WORKLOAD, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)
    common_utils.get_response(kiali_client, method_name='workloadSpans', path=INVALID_PATH_WORKLOAD_WORKLOAD, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)
    common_utils.get_response(kiali_client, method_name='workloadSpans', path=INVALID_PATH_WORKLOAD, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)

def test_invalid_service_traces_list_negative_503(kiali_client):
     
    common_utils.get_response(kiali_client, method_name='serviceTraces', path=INVALID_PATH_NAMESPACE_SERVICE, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)
    common_utils.get_response(kiali_client, method_name='serviceTraces', path=INVALID_PATH_SERVICE_SERVICE, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)
    common_utils.get_response(kiali_client, method_name='serviceTraces', path=INVALID_PATH_SERVICE, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)

def test_invalid_service_traces_list_negative_400(kiali_client):
    
	common_utils.get_response(kiali_client, method_name='serviceTraces', path={'namespace': 'bookinfo', 'service':'details'}, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
	common_utils.get_response(kiali_client, method_name='serviceTraces', path=INVALID_PATH_SERVICE_SERVICE, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
	common_utils.get_response(kiali_client, method_name='serviceTraces', path=INVALID_PATH_NAMESPACE_SERVICE, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
	common_utils.get_response(kiali_client, method_name='serviceTraces', path=INVALID_PATH_SERVICE, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)

def test_invalid_service_spans_list_negative_503(kiali_client):
   	
    common_utils.get_response(kiali_client, method_name='serviceSpans', path=INVALID_PATH_NAMESPACE_SERVICE, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)
    common_utils.get_response(kiali_client, method_name='serviceSpans', path=INVALID_PATH_SERVICE_SERVICE, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)
    common_utils.get_response(kiali_client, method_name='serviceSpans', path=INVALID_PATH_SERVICE, params={'startMicros': calendar.timegm(gmt()) }, status_code_expected=503)


def test_invalid_app_spans_list_negative_400(kiali_client):
    
    common_utils.get_response(kiali_client, method_name='appSpans', path={'namespace': 'bookinfo', 'app':'details'}, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='appSpans', path=INVALID_PATH_APP_APP, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='appSpans', path=INVALID_PATH_NAMESPACE_APP, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='appSpans', path=INVALID_PATH_APP, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)

def test_invalid_app_trace_list_negative_400(kiali_client):
    
    common_utils.get_response(kiali_client, method_name='appTraces', path={'namespace': 'bookinfo', 'app':'details'}, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='appTraces', path=INVALID_PATH_APP_APP, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='appTraces', path=INVALID_PATH_NAMESPACE_APP, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
    common_utils.get_response(kiali_client, method_name='appTraces', path=INVALID_PATH_APP, params=INVALID_PARAMS_STARTMICROS, status_code_expected=400)
