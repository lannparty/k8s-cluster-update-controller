package controller

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	// apiv1 "k8s.io/api/core/v1"
	// corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	// "k8s.io/kubernetes/pkg/api"

	fcache "k8s.io/client-go/tools/cache/testing"
	// "k8s.io/kubernetes/pkg/api/unversioned"
)

func newController() (*Controller, *fcache.FakeControllerSource, chan string) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	clientset := k8sfake.NewSimpleClientset()
	source := fcache.NewFakeControllerSource()
	eventCh := make(chan string, 1000)
	indexer, informer := cache.NewIndexerInformer(source, &v1.Node{}, time.Millisecond*100, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
			// eventCh <- key
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
			eventCh <- key
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
			// eventCh <- key

		},
	}, cache.Indexers{})

	c := NewController(queue, indexer, informer)
	c.SetSession(clientset)

	return c, source, eventCh
}

func TestRun(t *testing.T) {
	ctlr, source, eventCh := newController()
	defer close(eventCh)

	stop := make(chan struct{})
	defer close(stop)
	go ctlr.Run(1, stop)
	time.Sleep(1 * time.Second)

	// Let's add a few objects to the source.
	testIDs := []string{"apple", "banana", "cactus"}
	for i, name := range testIDs {
		// Note that these nodes are not valid-- the fake source doesn't
		// call validation or anything.
		source.Add(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}})
		if i == 0 {
			source.Delete(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}})
		}
	}

	// Let's wait for the controller to process the things we just added.
	outputSet := sets.String{}
	for i := 0; i < len(testIDs); i++ {
		time.Sleep(6 * time.Second)
		outputSet.Insert(<-eventCh)
	}

	// for _, key := range outputSet.List() {
	// 	fmt.Println(key)
	// }
}
