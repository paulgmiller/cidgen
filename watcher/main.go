package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/cilium/cilium/pkg/allocator"
	"github.com/cilium/cilium/pkg/identity/key"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumclientset "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
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

	// Create Cilium clientset
	clientset, err := ciliumclientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	var count int64 = 0
	store := cache.NewIndexer(
		cache.DeletionHandlingMetaNamespaceKeyFunc,
		cache.Indexers{byKeyIndex: getIdentitiesByKeyFunc((&key.GlobalIdentity{}).PutKeyFromMap)})

	//store := cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)

	// this didn't really seem to help
	/*trim := func(obj interface{}) (interface{}, error) {
		if identity, ok := obj.(*ciliumv2.CiliumIdentity); ok {
			trimmedIdentity := &ciliumv2.CiliumIdentity{
				TypeMeta: identity.TypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      identity.ObjectMeta.Name,
					Namespace: identity.ObjectMeta.Namespace,
				},
			}
			return trimmedIdentity, nil
		}
		log.Fatalf("DANGER NOT A CILOUM IDENTITY")
		return nil, fmt.Errorf("object other than CiliumIdentity was pushed to the store")
	}*/

	lw := clientset.CiliumV2().CiliumIdentities()

	var page int64 = 0
	pageFromEnv, err := strconv.ParseInt(os.Getenv("PAGE_SIZE"), 10, 64)
	if err != nil {
		page = pageFromEnv
	}

	identityInformer := NewInformerWithStore(
		k8sUtils.ListerWatcherFromTyped[*v2.CiliumIdentityList](lw),
		&v2.CiliumIdentity{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				count++

				if count%1000 == 0 {
					log.Printf("got %d identites", count)
					keyIndex, err := getIdentitiesByKeyFunc((&key.GlobalIdentity{}).PutKeyFromMap)(obj)
					if err == nil {
						log.Printf("example keyindex %v", keyIndex)
					} else {
						log.Printf("Error with key index %s", err)
					}

				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {},
			DeleteFunc: func(obj interface{}) {
				count--
			},
		},
		nil,
		store,
		page,
	)

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	go func() {
		for _ = range time.Tick(time.Minute) {
			debug.FreeOSMemory()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Printf("Alloc = %v MiB", bToMb(m.Alloc))
			log.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
			log.Printf("\tSys = %v MiB", bToMb(m.Sys))
			log.Printf("\tNumGC = %v\n", m.NumGC)
			if rss, err := getRSS(); err != nil {
				log.Printf("\tRSS = %v\n", rss)
			}

		}
	}()

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

func getRSS() (int64, error) {
	data, err := ioutil.ReadFile("/proc/self/status")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				rss, err := strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					return 0, err
				}
				// VmRSS is in kB, convert to Mb
				return rss / 1024, nil
			}
		}
	}
	return 0, fmt.Errorf("VmRSS not found")
}

func getIdentitiesByKeyFunc(keyFunc func(map[string]string) allocator.AllocatorKey) func(obj interface{}) ([]string, error) {
	return func(obj interface{}) ([]string, error) {
		if identity, ok := obj.(*ciliumv2.CiliumIdentity); ok {
			return []string{keyFunc(identity.SecurityLabels).GetKey()}, nil
		}
		return []string{}, fmt.Errorf("object other than CiliumIdentity was pushed to the store")
	}
}

func byName() func(obj interface{}) ([]string, error) {
	return func(obj interface{}) ([]string, error) {
		if identity, ok := obj.(*ciliumv2.CiliumIdentity); ok {
			return []string{identity.Name}, nil
		}
		return []string{}, fmt.Errorf("object other than CiliumIdentity was pushed to the store")
	}
}

func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}
