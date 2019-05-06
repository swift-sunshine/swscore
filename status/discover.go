package status

import (
	"io/ioutil"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/log"
)

// The Kiali ServiceAccount token.
var saToken string

var clientFactory kubernetes.ClientFactory

func DiscoverJaeger() (url string, err error) {
	conf := config.Get()

	if conf.ExternalServices.Tracing.URL != "" {
		return conf.ExternalServices.Tracing.URL, nil
	}

	ns := config.IstioDefaultNamespace

	// User set a service
	if conf.ExternalServices.Tracing.Service != "" {
		url, err = discoverUrlService(conf.ExternalServices.Tracing.Namespace, conf.ExternalServices.Tracing.Service)
		// If the service is correct set it
		if err == nil {
			conf.ExternalServices.Tracing.EnableJaeger = true
			conf.ExternalServices.Tracing.URL = url
		}
	} else {
		// No set a service, go discover
		tracing := config.TracingDefaultService
		jaeger := config.JaegerQueryDefaultService

		// Check if there is a Tracing service in the namespace
		log.Debugf("Kiali is looking for Tracing/Jaeger service ...")
		url, err = discoverUrlService(ns, tracing)
		conf.ExternalServices.Tracing.Service = tracing
		if err != nil || url == "" {
			log.Debugf("Kiali not found Tracing in service %s of ns %s error: %s", tracing, ns, err)
			// Check if there is a Jaeger-Query service in the namespace
			url, err = discoverUrlService(ns, jaeger)
			if err != nil || url == "" {
				conf.ExternalServices.Tracing.EnableJaeger = false
				conf.ExternalServices.Tracing.Service = ""
				log.Debugf("Kiali not found Jaeger in service %s of ns %s  error: %s", jaeger, ns, err)
				return "", err
			}
			log.Debugf("Kiali found Jaeger in %s", url)
			conf.ExternalServices.Tracing.Service = jaeger
		}
		log.Debugf("Kiali found Tracing in %s", url)
		conf.ExternalServices.Tracing.EnableJaeger = true
		conf.ExternalServices.Tracing.URL = url
	}
	config.Set(conf)
	return url, err
}

func DiscoverGrafana() (url string, err error) {
	conf := config.Get()
	// If display link is disable in Grafana configuration return empty string and avoid discover
	if !conf.ExternalServices.Grafana.DisplayLink {
		return "", nil
	}
	if conf.ExternalServices.Grafana.URL != "" {
		return conf.ExternalServices.Grafana.URL, nil
	}
	log.Debugf("Kiali is looking for Grafana service ...")
	url, err = discoverUrlService(config.Get().ExternalServices.Grafana.ServiceNamespace, config.Get().ExternalServices.Grafana.Service)
	if err != nil {
		log.Debugf("Kiali not found Grafana in service %s error: %s", config.Get().ExternalServices.Grafana.Service, err)
	} else {
		log.Debugf("Kiali found Grafana in %s", url)
	}
	conf.ExternalServices.Grafana.URL = url
	config.Set(conf)
	return url, err
}

func discoverUrlService(ns string, service string) (url string, err error) {
	if saToken == "" {
		token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err != nil {
			return "", err
		}
		saToken = string(token)
	}

	if clientFactory == nil {
		userClientFactory, err := kubernetes.GetClientFactory()
		if err != nil {
			return "", err
		}
		clientFactory = userClientFactory
	}

	client, err := clientFactory.GetClient(saToken)
	if err != nil {
		return "", err
	}
	// If the client is not openshift return and avoid discover
	if !client.IsOpenShift() {
		return "", nil
	}
	route, err := client.GetRoute(ns, service)
	if err != nil {
		return "", err
	}

	host := route.Spec.Host
	if route.Spec.TLS != nil {
		return "https://" + host, nil
	} else {
		return "http://" + host, nil
	}
}
