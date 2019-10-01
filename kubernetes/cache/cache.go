package cache

import (
	"regexp"
	"strings"
	"time"

	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	kube "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/kiali/kiali/log"
)

// Istio uses caches for pods and controllers.
// Kiali will use caches for specific namespaces and types
// https://github.com/istio/istio/blob/master/mixer/adapter/kubernetesenv/cache.go

type (
	KialiCache interface {
		// Control methods
		// Check if a namespace is listed to be cached; if yes, creates a cache for that namespace
		Check(namespace string) bool
		// Stop all caches
		Stop()

		// Business methods
		GetDeployments(namespace string) ([]apps_v1.Deployment, error)
		GetServices(namespace string) ([]core_v1.Service, error)
	}

	// This map will store Informers per specific types
	// i.e. map["Deployment"], map["Service"]
	typeCache map[string]cache.SharedIndexInformer

	kialiCacheImpl struct {
		clientset       kube.Interface
		refreshDuration time.Duration
		cacheNamespaces []string
		stopChan        chan struct{}
		nsCache         map[string]typeCache
	}
)

func NewKialiCache(clientset kube.Interface, refreshDuration time.Duration, cacheNamespaces []string) KialiCache {
	kialiCacheImpl := kialiCacheImpl{
		clientset:       clientset,
		refreshDuration: refreshDuration,
		cacheNamespaces: cacheNamespaces,
		stopChan:        make(chan struct{}),
		nsCache:     	 make(map[string]typeCache),
	}

	return &kialiCacheImpl
}

// It will indicate if a namespace should have a cache
func (c *kialiCacheImpl) isCached(namespace string) bool {
	for _, cacheNs := range c.cacheNamespaces {
		if matches, _ := regexp.MatchString(strings.TrimSpace(cacheNs), namespace); matches {
			return true
		}
	}
	return false
}

func (c *kialiCacheImpl) createCache(namespace string) bool {
	sharedInformers := informers.NewSharedInformerFactoryWithOptions(c.clientset, c.refreshDuration, informers.WithNamespace(namespace))
	informer := make(typeCache)
	informer["Deployment"] = sharedInformers.Apps().V1().Deployments().Informer()
	informer["Service"] = sharedInformers.Core().V1().Services().Informer()
	c.nsCache[namespace] = informer

	go func() {
		for _, informer := range c.nsCache[namespace] {
			go informer.Run(c.stopChan)
		}
		<- c.stopChan
		log.Infof("Kiali cache for [namespace: %s] stopped", namespace)
	}()

	log.Infof("Waiting for Kiali cache for [namespace: %s] to sync", namespace)
	isSynced := func () bool {
		hasSynced := true
		for _, informer := range c.nsCache[namespace] {
			hasSynced = hasSynced && informer.HasSynced()
		}
		return hasSynced
	}
	if synced := cache.WaitForCacheSync(c.stopChan, isSynced); !synced {
		c.stopChan <- struct{}{}
		log.Errorf("Kiali cache for [namespace: %s] sync failure", namespace)
		return false
	}
	log.Infof("Kiali cache for [namespace: %s] started", namespace)

	return true
}

func (c *kialiCacheImpl) Check(namespace string) bool {
	if !c.isCached(namespace) {
		return false
	}
	return c.createCache(namespace)
}

func (c *kialiCacheImpl) Stop() {
	if c.stopChan != nil {
		close(c.stopChan)
		c.stopChan = nil
	}
}