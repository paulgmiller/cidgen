// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"

	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"github.com/cilium/cilium/pkg/time"
)

type privateRunner struct {
	cache.Controller
	cacheMutationDetector cache.MutationDetector
}

func (p *privateRunner) Run(stopCh <-chan struct{}) {
	go p.cacheMutationDetector.Run(stopCh)
	p.Controller.Run(stopCh)
}

// NewInformerWithStore uses the same arguments as NewInformer for which a caller can also set a
// cache.Store and includes the default cache MutationDetector.
func NewInformerWithStore(
	lw cache.ListerWatcher,
	objType k8sRuntime.Object,
	resyncPeriod time.Duration,
	h cache.ResourceEventHandler,
	transformer cache.TransformFunc,
	clientState cache.Store,
	pageSize int64,
) cache.Controller {

	// This will hold incoming changes. Note how we pass clientState in as a
	// KeyLister, that way resync operations will result in the correct set
	// of update/delete deltas.
	opts := cache.DeltaFIFOOptions{KeyFunction: cache.MetaNamespaceKeyFunc, KnownObjects: clientState}
	fifo := cache.NewDeltaFIFOWithOptions(opts)

	cacheMutationDetector := cache.NewCacheMutationDetector(fmt.Sprintf("%T", objType))

	cfg := &cache.Config{
		Queue:             fifo,
		ListerWatcher:     lw,
		ObjectType:        objType,
		FullResyncPeriod:  resyncPeriod,
		RetryOnError:      false,
		WatchListPageSize: pageSize,

		Process: func(obj interface{}, isInInitialList bool) error {
			// from oldest to newest
			for _, d := range obj.(cache.Deltas) {

				var obj interface{}
				if transformer != nil {
					var err error
					if obj, err = transformer(d.Object); err != nil {
						return err
					}
				} else {
					obj = d.Object
				}

				// In CI we detect if the objects were modified and panic
				// this is a no-op in production environments.
				cacheMutationDetector.AddObject(obj)

				switch d.Type {
				case cache.Sync, cache.Added, cache.Updated:
					if old, exists, err := clientState.Get(obj); err == nil && exists {
						if err := clientState.Update(obj); err != nil {
							return err
						}
						h.OnUpdate(old, obj)
					} else {
						if err := clientState.Add(obj); err != nil {
							return err
						}
						h.OnAdd(obj, isInInitialList)
					}
				case cache.Deleted:
					if err := clientState.Delete(obj); err != nil {
						return err
					}
					h.OnDelete(obj)
				}
			}
			return nil
		},
	}
	return &privateRunner{
		Controller:            cache.New(cfg),
		cacheMutationDetector: cacheMutationDetector,
	}
}
