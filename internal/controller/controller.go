package controller

import (
	"strconv"
	"strings"

	"github.com/lannparty/k8s-cluster-update-controller/pkg/kubecmd"

	"fmt"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"k8s.io/client-go/tools/cache"

	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
	session  kubernetes.Interface
}

func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller) *Controller {
	return &Controller{
		informer: informer,
		indexer:  indexer,
		queue:    queue,
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two Nodes with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	err := c.syncToStdout(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

func (c *Controller) SetSession(clientset kubernetes.Interface) {
	c.session = clientset
}

// syncToStdout is the business logic of the controller. In this controller it simply prints
// information about the node to stdout. In case an error happened, it has to simply return the error.
// The retry logic should not be part of the business logic.
func (c *Controller) syncToStdout(key string) error {

	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v\n", key, err)
		return err
	}

	if !exists {
		// Below we will warm up our cache with a Node, so that we will see a delete for one node
		fmt.Printf("Node %s does not exist anymore\n", key)
	} else {
		err := c.RollingUpdate(obj)
		if err != nil {
			return err
		}
	}
	return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		klog.Infof("Error syncing node %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	klog.Infof("Dropping node %q out of the queue: %v", key, err)
}

func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	klog.Info("Starting Node controller")

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping Node controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

// RollingUpdate executes the cordon, drain, and validate functions and *blocks*
// until it returns with or without an error. Blocking limits the draining of
// nodes to one at a time.
func (c *Controller) RollingUpdate(obj interface{}) error {
	// Note that you also have to check the uid if you have a local controlled resource, which
	// is dependent on the actual instance, to detect that a Pod was recreated with the same name
	targetNode := obj.(*v1.Node).GetName()
	evictionWaitTimeVar := os.Getenv("EVICTIONWAITTIME")
	if evictionWaitTimeVar == "" {
		evictionWaitTimeVar = "5"
	}
	evictionWaitTime, err := strconv.Atoi(evictionWaitTimeVar)
	if err != nil {
		return err
	}
	validateWaitTimeVar := os.Getenv("VALIDATEWAITTIME")
	if validateWaitTimeVar == "" {
		validateWaitTimeVar = "5"
	}
	validateWaitTime, err := strconv.Atoi(validateWaitTimeVar)
	if err != nil {
		return err
	}
	retryThresholdVar := os.Getenv("RETRYTHRESHOLD")
	retryThreshold, err := strconv.Atoi(retryThresholdVar)
	evictionStrategy := os.Getenv("EVICTIONSTRATEGY")

	delayCordon, err := strconv.Atoi(os.Getenv("DELAY_CORDON"))
	if err != nil {
		delayCordon = 0
	}

	// Check the age of the node before targeting for cordon
	creationTime := obj.(*v1.Node).CreationTimestamp.Time
	before := time.Now().Local().Add(-time.Minute * time.Duration(delayCordon))
	if !creationTime.Before(before) {
		klog.Info(fmt.Sprintf("Not cordoning node %s: age of node must be >= %d minutes before it can be targed for cordon", targetNode, delayCordon))
		return nil
	}

	// Set node unschedulable.
	klog.Info(fmt.Sprintf("Cordoning node %s\n", targetNode))
	err = kubecmd.CordonNode(c.session, targetNode)
	if err != nil {
		klog.Errorf("Cordon node %s failed with error %v\n", targetNode, err)
	}
	retryCounter := 0
	for {
		err = kubecmd.EvictPodsOnCordonedNodes(c.session, targetNode, "v1beta1")
		time.Sleep(time.Duration(evictionWaitTime) * time.Second)
		if err != nil {
			if evictionStrategy == "retry" {
				retryCounter += 1
				klog.Errorf("Eviction of pods on %s failed with error %s, eviction strategy: retry, retry threshold: %v, retry count: %v\n", targetNode, err, retryThreshold, retryCounter)
				if retryCounter == retryThreshold {
					klog.Errorf("Retry threshold reached. Skipping node.")
					break
				}
			} else if evictionStrategy == "skip" {
				klog.Errorf("Eviction strategy skip. Skipping node.")
				break
			}
		} else {
			break
		}
	}

	vitalNamespaces := strings.Split(os.Getenv("VITALNAMESPACES"), ",")
	for kubecmd.ValidateNamespaces(c.session, vitalNamespaces) == false {
		fmt.Printf("Waiting for pods on vital namespaces %v to stablize.\n", vitalNamespaces)
		time.Sleep(time.Duration(validateWaitTime) * time.Second)
	}
	return nil
}
