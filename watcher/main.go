package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "net/http/pprof"

	"github.com/cilium/cilium/pkg/allocator"
	"github.com/cilium/cilium/pkg/identity/key"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"

	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumclientset "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	"github.com/cilium/cilium/pkg/k8s/informer"
	k8sUtils "github.com/cilium/cilium/pkg/k8s/utils"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	// byKeyIndex is the name of the index of the identities by key.
	byKeyIndex = "by-key-index"
)

func main() {

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	config.QPS = 100
	config.Burst = 200

	// Create Cilium clientset
	clientset, err := ciliumclientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	//pprof
	//watch and populate store
	count := 0
	store := cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{byKeyIndex: getIdentitiesByKeyFunc((&key.GlobalIdentity{}).PutKeyFromMap)})

	identityInformer := informer.NewInformerWithStore(
		k8sUtils.ListerWatcherFromTyped[*v2.CiliumIdentityList](clientset.CiliumV2().CiliumIdentities()),
		&v2.CiliumIdentity{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				count++
				if count%1000 == 0 {
					log.Printf("got %d identites", count)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {},
			DeleteFunc: func(obj interface{}) {
				count--
			},
		},
		nil,
		store,
	)
	term := make(chan os.Signal, 2)
	stopChan := make(chan struct{})
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-term
		stopChan <- struct{}{}
	}()
	identityInformer.Run(stopChan)
	//if ok := cache.WaitForCacheSync(stopChan, identityInformer.HasSynced); ok {

}

func getIdentitiesByKeyFunc(keyFunc func(map[string]string) allocator.AllocatorKey) func(obj interface{}) ([]string, error) {
	return func(obj interface{}) ([]string, error) {
		if identity, ok := obj.(*ciliumv2.CiliumIdentity); ok {
			return []string{keyFunc(identity.SecurityLabels).GetKey()}, nil
		}
		return []string{}, fmt.Errorf("object other than CiliumIdentity was pushed to the store")
	}
}
