package metal

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

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
