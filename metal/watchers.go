package metal

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// createNodesWatcher create a cache.SharedIndexInformation for node handlers
func createNodesWatcher(ctx context.Context, informer informers.SharedInformerFactory, cloudServices []cloudService) (cache.SharedIndexInformer, error) {
	klog.V(5).Info("called createNodesWatcher")

	klog.V(5).Info("createNodesWatcher(): creating nodesInformer")
	nodesInformer := informer.Core().V1().Nodes().Informer()
	klog.V(5).Info("createNodesWatcher(): adding event handlers")
	nodesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			n := obj.(*v1.Node)
			for _, csvc := range cloudServices {
				if handler := csvc.nodeReconciler(); handler != nil {
					if err := handler(ctx, []*v1.Node{n}, ModeAdd); err != nil {
						klog.Errorf("%s failed to update and sync node for add %s for handler: %v", csvc.name(), n.Name, err)
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			n := obj.(*v1.Node)
			for _, csvc := range cloudServices {
				if handler := csvc.nodeReconciler(); handler != nil {
					if err := handler(ctx, []*v1.Node{n}, ModeRemove); err != nil {
						klog.Errorf("%s failed to update and sync node for remove %s for handler: %v", csvc.name(), n.Name, err)
					}
				}
			}
		},
	})

	return nodesInformer, nil
}

// createServicesWatcher create a cache.SharedIndexInformation for services handlers
func createServicesWatcher(ctx context.Context, informer informers.SharedInformerFactory, cloudServices []cloudService) (cache.SharedIndexInformer, error) {
	klog.V(5).Info("called createServicesWatcher")

	// register to capture all new services
	servicesInformer := informer.Core().V1().Services().Informer()
	servicesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*v1.Service)
			for _, csvc := range cloudServices {
				if handler := csvc.serviceReconciler(); handler != nil {
					if err := handler(ctx, []*v1.Service{svc}, ModeAdd); err != nil {
						klog.Errorf("%s failed to update and sync service for add %s/%s: %v", csvc.name(), svc.Namespace, svc.Name, err)
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			svc := obj.(*v1.Service)
			for _, csvc := range cloudServices {
				if handler := csvc.serviceReconciler(); handler != nil {
					if err := handler(ctx, []*v1.Service{svc}, ModeRemove); err != nil {
						klog.Errorf("%s failed to update and sync service for remove %s/%s: %v", csvc.name(), svc.Namespace, svc.Name, err)
					}
				}
			}
		},
	})

	return servicesInformer, nil
}

// startWatchers start the various informers in their own go routines, and wait for sync to be done
func startWatchers(ctx context.Context, informers []cache.SharedIndexInformer) error {
	klog.V(5).Info("startWatchers() started")
	// what this does:
	// when you create an informer, you start it by calling informer.Run()
	// however, it can take some time for the local state to sync up. If you use any methods before
	// it is completely synced, especially get or list, you can end up missing data. In order to
	// avoid the issue, you run it in the following order:
	//
	// 1. create your informer
	// 2. informer.Run()
	// 3. create a slice of sync functions []cache.InformerSynced. The function on each informer is informer.HasSynced
	// 4. use the utility function cache.WaitForCacheSync(), passing it your sync function slice
	// 5. when the utility function returns, the cache is synced and you are ready to use it
	//
	// for a good overview of controllers and their lifecycle, see https://engineering.bitnami.com/articles/a-deep-dive-into-kubernetes-controllers.html
	var syncFuncs []cache.InformerSynced
	for _, informer := range informers {
		go informer.Run(ctx.Done())
		syncFuncs = append(syncFuncs, informer.HasSynced)
	}
	klog.V(4).Infof("startWatchers(): waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), syncFuncs...) {
		return fmt.Errorf("syncing caches failed")
	}
	klog.Info("all watchers started")
	return nil
}

func timerLoop(ctx context.Context, informer informers.SharedInformerFactory, cloudServices []cloudService) {
	klog.V(5).Infof("timerLoop(): starting loop")
	servicesLister := informer.Core().V1().Services().Lister()
	nodesLister := informer.Core().V1().Nodes().Lister()
	for {
		select {
		case <-time.After(checkLoopTimerSeconds * time.Second):
			// with each loop, get all of the services and nodes,
			// then loop through each service and see if it provides a serviceReconciler
			// and a nodeReconciler.
			servicesList, err := servicesLister.List(labels.Everything())
			if err != nil {
				klog.Errorf("timer loop had error listing services, will try next loop: %v", err)
			}
			nodesList, err := nodesLister.List(labels.Everything())
			if err != nil {
				klog.Errorf("timer loop had error listing nodes, will try next loop: %v", err)
			}
			for _, svc := range cloudServices {
				if handler := svc.serviceReconciler(); handler != nil {
					if err := handler(ctx, servicesList, ModeSync); err != nil {
						klog.Errorf("%s: error updating and syncing services, will try next loop: %v", svc.name(), err)
					}
				}
				if handler := svc.nodeReconciler(); handler != nil {
					if err := handler(ctx, nodesList, ModeSync); err != nil {
						klog.Errorf("%s: error updating and syncing nodes, will try next loop: %v", svc.name(), err)
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
