#!/bin/bash

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -at|--all-tests)
      ALL_TESTS="$2"
      shift;shift
      ;;
    -c|--color)
      COLOR="$2"
      shift;shift
      ;;
    -ce|--client-exe)
      TEST_CLIENT_EXE="$2"
      shift;shift
      ;;
    -ct|--cluster-type)
      CLUSTER_TYPE="$2"
      shift;shift
      ;;
    -d|--debug)
      MOLECULE_DEBUG="$2"
      shift;shift
      ;;
    -dorp|--docker-or-podman)
      DORP="$2"
      shift;shift
      ;;
    -hcr|--helm-charts-repo)
      HELM_CHARTS_REPO="$2"
      shift;shift
      ;;
    -ksh|--kiali-src-home)
      KIALI_SRC_HOME="$2"
      shift;shift
      ;;
    -kn|--kind-name)
      KIND_NAME="$2"
      shift;shift
      ;;
    -mp|--minikube-profile)
      MINIKUBE_PROFILE="$2"
      shift;shift
      ;;
    -nd|--never-destroy)
      MOLECULE_DESTROY_NEVER="$2"
      shift;shift
      ;;
    -oi|--operator-installer)
      MOLECULE_OPERATOR_INSTALLER="$2"
      shift;shift
      ;;
    -p|--profiler)
      MOLECULE_OPERATOR_PROFILER_ENABLED="$2"
      shift;shift
      ;;
    -st|--skip-tests)
      SKIP_TESTS="$2"
      shift;shift
      ;;
    -sv|--spec-version)
      MOLECULE_KIALI_CR_SPEC_VERSION="$2"
      shift;shift
      ;;
    -tld|--test-logs-dir)
      TEST_LOGS_DIR="$2"
      shift;shift
      ;;
    -udi|--use-dev-images)
      MOLECULE_USE_DEV_IMAGES="$2"
      shift;shift
      ;;
    -h|--help)
      cat <<HELPMSG

$0 [option...] command

-at|--all-tests          Space-separated list of all the molecule tests to be run. Note that this list may not be the
                         tests that are actually run - see --skip-tests.
                         The default is all the tests found in the operator/molecule directory in the Kiali source home directory.
-c|--color               True if you want color in the output. (default: true)
-ce|--client-exe         Location of the client executable (either referring to 'oc' or 'kubectl') (default: relies on path).
-ct|--cluster-type       The type of cluster being tested. Must be one of: minikube, openshift. (default: openshift)
-d|--debug               True if you want the molecule tests to output large amounts of debug messages. (default: true)
-dorp|--docker-or-podman What should be used - "docker" or "podman"
-hcr|--helm-charts-repo  Location of the helm charts git repo. (default: ../helm-charts)
-ksh|--kiali_src-home    Location of the Kiali source code, the makefiles, and operator/molecule tests. (default: ..)
-kn|--kind-name          If cluster type is 'kind' you can specify the cluster name that is in use via this option.
-mp|--minikube-profile   If cluster type is 'minikube' you can specify the profile that is in use via this option.
-nd|--never-destroy      Do not have the molecule framework destroy the test scaffolding. Setting this to true
                         will help test failures by allowing you to examine the operator logs after a test finished.
                         Default is 'false' - the operator resources will be deleted after a test completes, no matter
                         if the test succeeded or failed.
-oi|--operator-installer How the operator is to be installed by the molecule tests. It is either installed
                         via helm or the installation is skipped entirely. Use "skip" if you installed the
                         operator yourself (say, via OLM) and you want the molecule tests to use it rather
                         than to install its own operator. Valid values: "helm" or "skip" (default: helm)
-p|--profiler            True if you want to enable the ansible profiler in the operator (default: true)
-st|--skip-tests         Space-separated list of all the molecule tests to be skipped. (default: tests unable to run on cluster type)
-sv|--spec-version       The Kiali CR spec.version to test. (default: default)
-tld|--test-logs-dir     Location where the test log files will be stored. (default: /tmp/kiali-molecule-test-logs.<date-time>)
-udi|--use-dev-images    If true, the tests will use locally built dev images of Kiali and the operator. When using dev
                         images, you must have already pushed locally built dev images into your cluster.
                         If false, the cluster will put the latest images found on quay.io.
                         Default: false
HELPMSG
      exit 1
      ;;
    *)
      echo "Unknown option: [$key]. Aborting."
      exit 1
      ;;
  esac
done

# Where the Kiali github source is located on the local machine.
# - The operator/molecule test directory should exist here.
# - The helm-charts directory should exist here (look at physical peer directory next to operator)
SCRIPT_ROOT="$( cd "$(dirname "$0")" ; pwd -P )"
KIALI_SRC_HOME="${KIALI_SRC_HOME:-${SCRIPT_ROOT}/..}"
if [ ! -d "${KIALI_SRC_HOME}" ]; then echo "Kiali source home directory is invalid: ${KIALI_SRC_HOME}"; exit 1; fi
if [ ! -d "${KIALI_SRC_HOME}/operator/molecule" ]; then echo "Kiali source home directory is missing the operator molecule tests: ${KIALI_SRC_HOME}"; exit 1; fi
KIALI_SRC_HOME="$(cd "${KIALI_SRC_HOME}"; pwd -P)"
if [ -z "${HELM_CHARTS_REPO:-}" ]; then
  if [ -L "${KIALI_SRC_HOME}/operator" -a -d "${KIALI_SRC_HOME}/operator" ]; then
    HELM_CHARTS_REPO="$(cd "$(cd "${KIALI_SRC_HOME}/operator" && pwd -P)/.." && pwd -P)/helm-charts"
  else
    HELM_CHARTS_REPO="$(cd "${KIALI_SRC_HOME}" && pwd -P)/helm-charts"
  fi
fi
if [ ! -f "${HELM_CHARTS_REPO}/kiali-operator/Chart.yaml" ]; then echo "Kiali helm charts repo directory is invalid: ${HELM_CHARTS_REPO}"; exit 1; fi

# Set this to "minikube" if you want to test on minikube; "openshift" if testing on OpenShift.
export CLUSTER_TYPE="${CLUSTER_TYPE:-openshift}"
if [ "${CLUSTER_TYPE}" != "openshift" -a "${CLUSTER_TYPE}" != "minikube" -a "${CLUSTER_TYPE}" != "kind" ]; then echo "Cluster type is invalid: ${CLUSTER_TYPE}"; exit 1; fi

# A list of all the tests.
# This list, minus the tests to be skipped (see SKIP_TESTS), are the tests that this script will run.
ALL_TESTS=${ALL_TESTS:-$(cd "${KIALI_SRC_HOME}/operator/molecule"; ls -d *-test)}

# Put the names of any tests in here if you do not want to run them (space separated).
if [ "${CLUSTER_TYPE}" == "openshift" ]; then
  SKIP_TESTS="${SKIP_TESTS:-header-auth-test openid-test}"
elif [ "${CLUSTER_TYPE}" == "minikube" ]; then
  SKIP_TESTS="${SKIP_TESTS:-os-console-links-test openshift-auth-test}"
elif [ "${CLUSTER_TYPE}" == "kind" ]; then
  SKIP_TESTS="${SKIP_TESTS:-header-auth-test openid-test os-console-links-test openshift-auth-test}"
fi

# If you want to test the latest release from quay, set this to "false".
# If this is set to true, the current dev images that have been pushed to the cluster will be tested.
export MOLECULE_USE_DEV_IMAGES="${MOLECULE_USE_DEV_IMAGES:-false}"

# Set this to true if you want molecule to output more noisy logs from Ansible.
export MOLECULE_DEBUG="${MOLECULE_DEBUG:-true}"

# Set this to true if you want molecule to keep the operator resources intact after a test completes.
export MOLECULE_DESTROY_NEVER="${MOLECULE_DESTROY_NEVER:-false}"

# Set this to true if you want molecule to install the operator with its profiler enabled.
# This will dump profiler logs after each reconciliation run so will make the logs a little bigger.
export MOLECULE_OPERATOR_PROFILER_ENABLED="${MOLECULE_OPERATOR_PROFILER_ENABLED:-true}"

# Set this to helm if you want the molecule tests to install the operator via helm.
export MOLECULE_OPERATOR_INSTALLER="${MOLECULE_OPERATOR_INSTALLER:-helm}"

# When the tests create Kiali CR resources, this is its spec.version value.
export MOLECULE_KIALI_CR_SPEC_VERSION="${MOLECULE_KIALI_CR_SPEC_VERSION:-default}"

# The parent directory where all the test logs are going to be stored.
TEST_LOGS_DIR="${TEST_LOGS_DIR:-/tmp/kiali-molecule-test-logs.$(date +'%Y-%m-%d_%H-%M-%S')}"

# If you want color in the output, set this to 'true'.
COLOR=${COLOR:-true}

echo "========== SETTINGS =========="
echo DORP="$DORP"
echo KIALI_SRC_HOME="$KIALI_SRC_HOME"
echo ALL_TESTS="$ALL_TESTS"
echo SKIP_TESTS="$SKIP_TESTS"
echo CLUSTER_TYPE="$CLUSTER_TYPE"
echo MOLECULE_USE_DEV_IMAGES="$MOLECULE_USE_DEV_IMAGES"
echo MOLECULE_DEBUG="$MOLECULE_DEBUG"
echo MOLECULE_DESTROY_NEVER="$MOLECULE_DESTROY_NEVER"
echo MOLECULE_KIALI_CR_SPEC_VERSION="${MOLECULE_KIALI_CR_SPEC_VERSION}"
echo MOLECULE_OPERATOR_INSTALLER="$MOLECULE_OPERATOR_INSTALLER"
echo MOLECULE_OPERATOR_PROFILER_ENABLED="$MOLECULE_OPERATOR_PROFILER_ENABLED"
echo TEST_LOGS_DIR="$TEST_LOGS_DIR"
echo TEST_CLIENT_EXE="$TEST_CLIENT_EXE"
echo COLOR="$COLOR"
echo MINIKUBE_PROFILE="$MINIKUBE_PROFILE"
echo KIND_NAME="$KIND_NAME"
echo HELM_CHARTS_REPO="$HELM_CHARTS_REPO"
echo "=============================="

# Make sure the cluster is accessible
if [ "${CLUSTER_TYPE}" == "openshift" ]; then
  if ! ${TEST_CLIENT_EXE:-oc} whoami > /dev/null; then
    echo "You either did not 'oc login' or the OpenShift cluster is not accessible. Aborting."
    exit 1
  fi
else
  if ! ${TEST_CLIENT_EXE:-kubectl} get ns > /dev/null; then
    echo "The minikube cluster is not accessible. Aborting."
    exit 1
  fi
fi

# Create the directory where the test logs will go
if ! mkdir -p ${TEST_LOGS_DIR}; then echo "Failed to create the test logs directory [$TEST_LOGS_DIR]"; exit 1; fi
echo "All molecule test logs will go here: ${TEST_LOGS_DIR}"

dim() {
  if [ "$COLOR" == "true" ]; then echo -e "\e[2m${1}\e[22m"; else echo "${1}"; fi
}

red() {
  if [ "$COLOR" == "true" ]; then echo -e "\e[31m${1}\e[39m"; else echo "${1}"; fi
}

green() {
  if [ "$COLOR" == "true" ]; then echo -e "\e[32m${1}\e[39m"; else echo "${1}"; fi
}

# Go to the main Kiali source directory
cd "${KIALI_SRC_HOME}"

# There might be some things we have to do to prepare for the tests, do those things now.
# We also will need to clean up these things when the tests are finished.

prepare_test() {
  case $1 in
    # if using dev images on openshift, we have to grant an additional priviledge for this test
    default-namespace-test)
      if [ "${CLUSTER_TYPE}" == "openshift" -a "${MOLECULE_USE_DEV_IMAGES}" == "true" ]; then
        ${TEST_CLIENT_EXE:-oc} policy add-role-to-user system:image-puller system:serviceaccount:anothernamespace:kiali-service-account --namespace=kiali >> ${TEST_LOGS_DIR}/${1}.log 2>&1
      fi
      ;;

    # if running the non-OpenShift openid-test or header-auth-test, create a rolebinding so the test can log in
    header-auth-test|openid-test)
      if [ "${CLUSTER_TYPE}" != "openshift" ]; then
        ${TEST_CLIENT_EXE:-kubectl} create rolebinding openid-rolebinding-istio-system --clusterrole=kiali --user=admin@example.com --namespace=istio-system >> ${TEST_LOGS_DIR}/${1}.log 2>&1
      fi
      ;;

    # nothing to do for any other test
    *) ;;
  esac
}

unprepare_test() {
  case $1 in
    # remove that additional priviledge that was granted
    default-namespace-test)
      if [ "${CLUSTER_TYPE}" == "openshift" -a "${MOLECULE_USE_DEV_IMAGES}" == "true" ]; then
        ${TEST_CLIENT_EXE:-oc} policy remove-role-from-user system:image-puller system:serviceaccount:anothernamespace:kiali-service-account --namespace=kiali >> ${TEST_LOGS_DIR}/${1}.log 2>&1
      fi
      ;;

    # remove the rolebinding that was created
    header-auth-test|openid-test)
      if [ "${CLUSTER_TYPE}" != "openshift" ]; then
        ${TEST_CLIENT_EXE:-kubectl} delete rolebinding openid-rolebinding-istio-system --namespace=istio-system >> ${TEST_LOGS_DIR}/${1}.log 2>&1
      fi
      ;;

    # nothing to do for any other test
    *) ;;
  esac
}

# Prepare some environment variables needed by the makefile

# tell make what client to use if we were explicitly given one
if [ ! -z "${TEST_CLIENT_EXE}" ]; then
  export OC="${TEST_CLIENT_EXE}"
fi

# we have to explicitly tell the makefile about the DORP value
if [ -z "${DORP}" ]; then
  if ! which docker > /dev/null 2>&1; then
    if which podman > /dev/null 2>&1; then
      DORP="podman"
    else
      echo "You do not have 'docker' or 'podman' in PATH - aborting."
      exit 1
    fi
  else
    DORP="docker"
  fi
fi
export DORP

# the user may have specified a specific minikube profile to use - export this so make knows about it
export MINIKUBE_PROFILE

# the user may have specified a specific KinD cluster name to use - export this so make knows about it
export KIND_NAME

# build the latest Helm Chart
echo
echo "========================="
echo "=== BUILD HELM CHARTS ==="
echo "========================="
echo

if [ "${MOLECULE_OPERATOR_INSTALLER}" == "helm" ]; then
  make -C ${HELM_CHARTS_REPO} build-helm-charts
else
  echo "Skipping helm - will use the operator that is already installed"
fi

# Run the tests
echo
echo "====================="
echo "=== TEST RESULTS: ==="
echo "====================="

for t in ${ALL_TESTS}
do
  printf '\n%40s... ' "${t}"

  if [[ "${SKIP_TESTS}" == *"$t"* ]]; then
    printf '%s' "$(dim 'skipped')"
    continue;
  fi

  # $SECONDS is a built-in bash var - a timer that just ticks seconds
  SECONDS=0

  prepare_test ${t}

  export MOLECULE_SCENARIO="${t}"
  make molecule-test >> ${TEST_LOGS_DIR}/${t}.log 2>&1
  exitcode="$?"

  unprepare_test ${t}

  endtime=$SECONDS
  duration="$(($endtime / 60))m $(($endtime %60))s"

  if [ "${exitcode}" == "0" ]; then
    printf '%s [%s]' "$(green 'success')" "${duration}"
  else
    printf '%s [%s]' "$(red 'FAILURE')" "${duration}"
  fi

done

echo
echo
echo "Test logs can be found at: ${TEST_LOGS_DIR}"
echo
