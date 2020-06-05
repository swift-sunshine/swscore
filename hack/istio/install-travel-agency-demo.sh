#/bin/bash

# This deploys the travel agency demo

: ${CLIENT_EXE:=oc}
: ${NAMESPACE_AGENCY:=travel-agency}
: ${NAMESPACE_PORTAL:=travel-portal}
: ${ENABLE_OPERATION_METRICS:=false}

while [ $# -gt 0 ]; do
  key="$1"
  case $key in
    -c|--client)
      CLIENT_EXE="$2"
      shift;shift
      ;;
    -eo|--enable-operation-metrics)
      ENABLE_OPERATION_METRICS="$2"
      shift;shift
      ;;
    -na|--namespace-agency)
      NAMESPACE_AGENCY="$2"
      shift;shift
      ;;
    -np|--namespace-portal)
      NAMESPACE_PORTAL="$2"
      shift;shift
      ;;
    -h|--help)
      cat <<HELPMSG
Valid command line arguments:
  -c|--client: either 'oc' or 'kubectl'
  -eo|--enable-operation-metrics: either 'true' or 'false'. Only works on Istio 1.6 installed in istio-system.
  -na|--namespace-agency: where to install the travel agency demo resources
  -np|--namespace-portal: where to install the travel portal demo resources
HELPMSG
      exit 1
      ;;
    *)
      echo "Unknown argument [$key]. Aborting."
      exit 1
      ;;
  esac
done

echo Will deploy Travel Agency using these settings:
echo CLIENT_EXE=${CLIENT_EXE}
echo NAMESPACE_AGENCY=${NAMESPACE_AGENCY}
echo NAMESPACE_PORTAL=${NAMESPACE_PORTAL}
echo ENABLE_OPERATION_METRICS=${ENABLE_OPERATION_METRICS}

# Create the demo namespaces

${CLIENT_EXE} create namespace ${NAMESPACE_AGENCY}
${CLIENT_EXE} label namespace ${NAMESPACE_AGENCY} istio-injection=enabled

${CLIENT_EXE} create namespace ${NAMESPACE_PORTAL}
${CLIENT_EXE} label namespace ${NAMESPACE_PORTAL} istio-injection=enabled

# Prepare the new demo namespaces for CNI

if [ "${CLIENT_EXE}" == "oc" ]; then
${CLIENT_EXE} adm policy add-scc-to-group privileged system:serviceaccounts:${NAMESPACE_AGENCY}
${CLIENT_EXE} adm policy add-scc-to-group anyuid system:serviceaccounts:${NAMESPACE_AGENCY}
cat <<EOF | ${CLIENT_EXE} -n ${NAMESPACE_AGENCY} create -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: istio-cni
EOF

${CLIENT_EXE} adm policy add-scc-to-group privileged system:serviceaccounts:${NAMESPACE_PORTAL}
${CLIENT_EXE} adm policy add-scc-to-group anyuid system:serviceaccounts:${NAMESPACE_PORTAL}
cat <<EOF | ${CLIENT_EXE} -n ${NAMESPACE_PORTAL} create -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: istio-cni
EOF
fi

# Deploy the demo

${CLIENT_EXE} apply -f <(curl -L https://raw.githubusercontent.com/lucasponce/travel-comparison-demo/master/travel_agency.yaml) -n ${NAMESPACE_AGENCY}
${CLIENT_EXE} apply -f <(curl -L https://raw.githubusercontent.com/lucasponce/travel-comparison-demo/master/travel_portal.yaml) -n ${NAMESPACE_PORTAL}

# Set up metric classification

if [ "${ENABLE_OPERATION_METRICS}" != "true" ]; then
  # No need to keep going - we are done and the user doesn't want to do anything else.
  exit 0
fi

# This only works if you have Istio 1.6 installed, and it is in istio-system namespace.
${CLIENT_EXE} -n istio-system get envoyfilter stats-filter-1.6 -o yaml > stats-filter-1.6.yaml
cat <<EOF | patch -o - | ${CLIENT_EXE} -n istio-system apply -f - && rm stats-filter-1.6.yaml
--- stats-filter-1.6.yaml	2020-06-02 11:10:29.476537126 -0400
+++ stats-filter-1.6.yaml.new	2020-06-02 09:59:26.434300000 -0400
@@ -95,7 +95,14 @@
               configuration: |
                 {
                   "debug": "false",
-                  "stat_prefix": "istio"
+                  "stat_prefix": "istio",
+                  "metrics": [
+                   {
+                     "name": "requests_total",
+                     "dimensions": {
+                       "request_operation": "istio_operationId"
+                     }
+                   }]
                 }
               root_id: stats_inbound
               vm_config:
EOF

cat <<EOF | ${CLIENT_EXE} -n istio-system apply -f -
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: attribgen-travelagency-hotels
spec:
  workloadSelector:
    labels:
      app: hotels
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      context: SIDECAR_INBOUND
      proxy:
        proxyVersion: '1\.6.*'
      listener:
        filterChain:
          filter:
            name: "envoy.http_connection_manager"
            subFilter:
              name: "istio.stats"
    patch:
      operation: INSERT_BEFORE
      value:
        name: istio.attributegen
        typed_config:
          "@type": type.googleapis.com/udpa.type.v1.TypedStruct
          type_url: type.googleapis.com/envoy.extensions.filters.http.wasm.v3.Wasm
          value:
            config:
              configuration: |
                {
                  "attributes": [
                    {
                      "output_attribute": "istio_operationId",
                      "match": [
                        {
                          "value": "LocalRentals",
                          "condition": "request.url_path.matches('^/hotels/[[:alnum:]]*$') && request.method == 'GET'"
                        }
                      ]
                    }
                  ]
                }
              vm_config:
                runtime: envoy.wasm.runtime.null
                code:
                  local: { inline_string: "envoy.wasm.attributegen" }
---
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: attribgen-travelagency-cars
spec:
  workloadSelector:
    labels:
      app: cars
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      context: SIDECAR_INBOUND
      proxy:
        proxyVersion: '1\.6.*'
      listener:
        filterChain:
          filter:
            name: "envoy.http_connection_manager"
            subFilter:
              name: "istio.stats"
    patch:
      operation: INSERT_BEFORE
      value:
        name: istio.attributegen
        typed_config:
          "@type": type.googleapis.com/udpa.type.v1.TypedStruct
          type_url: type.googleapis.com/envoy.extensions.filters.http.wasm.v3.Wasm
          value:
            config:
              configuration: |
                {
                  "attributes": [
                    {
                      "output_attribute": "istio_operationId",
                      "match": [
                        {
                          "value": "LocalRentals",
                          "condition": "request.url_path.matches('^/cars/[[:alnum:]]*$') && request.method == 'GET'"
                        }
                      ]
                    }
                  ]
                }
              vm_config:
                runtime: envoy.wasm.runtime.null
                code:
                  local: { inline_string: "envoy.wasm.attributegen" }
---
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: attribgen-travelagency-flights
spec:
  workloadSelector:
    labels:
      app: flights
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      context: SIDECAR_INBOUND
      proxy:
        proxyVersion: '1\.6.*'
      listener:
        filterChain:
          filter:
            name: "envoy.http_connection_manager"
            subFilter:
              name: "istio.stats"
    patch:
      operation: INSERT_BEFORE
      value:
        name: istio.attributegen
        typed_config:
          "@type": type.googleapis.com/udpa.type.v1.TypedStruct
          type_url: type.googleapis.com/envoy.extensions.filters.http.wasm.v3.Wasm
          value:
            config:
              configuration: |
                {
                  "attributes": [
                    {
                      "output_attribute": "istio_operationId",
                      "match": [
                        {
                          "value": "LongDistanceTransportation",
                          "condition": "request.url_path.matches('^/flights/[[:alnum:]]*$') && request.method == 'GET'"
                        }
                      ]
                    }
                  ]
                }
              vm_config:
                runtime: envoy.wasm.runtime.null
                code:
                  local: { inline_string: "envoy.wasm.attributegen" }
EOF