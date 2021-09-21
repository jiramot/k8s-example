package main

import (
    "fmt"
    "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/fields"
    "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/apimachinery/pkg/util/wait"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"
    "log"
    "time"
)

type Controller struct {
    indexer  cache.Indexer
    queue    workqueue.RateLimitingInterface
    informer cache.Controller
}

func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller) *Controller {
    return &Controller{
        informer: informer,
        indexer:  indexer,
        queue:    queue,
    }
}

func (c *Controller) runWorker() {
    for c.processNextItem() {
    }
}

func (c *Controller) processNextItem() bool {
    key, quit := c.queue.Get()
    if quit {
        return false
    }
    defer c.queue.Done(key)

    err := c.doFunction(key.(string))
    c.handleErr(err, key)
    return true
}

func (c *Controller) doFunction(key string) error {
    log.Println("doFunction")
    obj, exists, err := c.indexer.GetByKey(key)
    if err != nil {
        klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
        return err
    }

    if !exists {
        fmt.Printf("Pod %s does not exist anymore\n", key)
    } else {
        fmt.Printf("Sync/Add/Update for Pod %s\n", obj.(*v1.Pod).GetName())
    }
    return nil
}

func (c *Controller) handleErr(err error, key interface{}) {
    if err == nil {
        c.queue.Forget(key)
        return
    }

    if c.queue.NumRequeues(key) < 5 {
        klog.Infof("Error syncing pod %v: %v", key, err)
        c.queue.AddRateLimited(key)
        return
    }

    c.queue.Forget(key)
    runtime.HandleError(err)
    klog.Infof("Dropping pod %q out of the queue: %v", key, err)
}

func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
    defer runtime.HandleCrash()

    defer c.queue.ShutDown()
    klog.Info("Starting Pod controller")

    go c.informer.Run(stopCh)

    if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
        runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
        return
    }

    for i := 0; i < threadiness; i++ {
        go wait.Until(c.runWorker, time.Second, stopCh)
    }

    <-stopCh
    klog.Info("Stopping Pod controller")
}

func main() {
    clusterConfig, err := rest.InClusterConfig()
    if err != nil {
        panic(err.Error())
    }
    clientset, err := kubernetes.NewForConfig(clusterConfig)
    if err != nil {
        panic(err.Error())
    }
    podListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "pods", v1.NamespaceAll, fields.Everything())
    queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

    indexer, informer := cache.NewIndexerInformer(podListWatcher, &v1.Pod{}, 0, cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            key, err := cache.MetaNamespaceKeyFunc(obj)
            log.Println("AddFunc: " + key)
            if err == nil {
                queue.Add(key)
            }
        },
        UpdateFunc: func(old interface{}, new interface{}) {
            key, err := cache.MetaNamespaceKeyFunc(new)
            log.Println("UpdateFunc: " + key)
            if err == nil {
                queue.Add(key)
            }
        },
        DeleteFunc: func(obj interface{}) {
            key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
            log.Println("DeleteFunc: " + key)
            if err == nil {
                queue.Add(key)
            }
        },
    }, cache.Indexers{})

    controller := NewController(queue, indexer, informer)

    // Now let's start the controller
    stop := make(chan struct{})
    defer close(stop)
    go controller.Run(1, stop)

    // Wait forever
    select {}
}
