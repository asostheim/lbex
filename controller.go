/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var (
	resyncPeriod = 30 * time.Second
)

// List Watch (lw) Controller (lwc)
type lwController struct {
	controller *cache.Controller
	stopCh     chan struct{}
}

// External LB Controller (lbex)
type lbExController struct {
	client    *dynamic.Client
	clientset *kubernetes.Clientset

	endpointsLWC  *lwController
	endpointStore cache.Store

	servciesLWC   *lwController
	servicesStore cache.Store

	stopCh chan struct{}
	queue  *TaskQueue
}

func newLbExController(client *dynamic.Client, clientset *kubernetes.Clientset) *lbExController {
	// create external loadbalancer controller struct
	lbexc := lbExController{
		client:    client,
		clientset: clientset,
		stopCh:    make(chan struct{}),
	}
	lbexc.queue = NewTaskQueue(lbexc.sync)
	lbexc.servciesLWC = newServicesListWatchControllerForClientset(&lbexc)
	lbexc.endpointsLWC = newEndpointsListWatchControllerForClientset(&lbexc)

	return &lbexc
}

func (lbex *lbExController) sync(obj interface{}) error {

	if lbex.queue.IsShuttingDown() {
		return nil
	}

	key, ok := obj.(string)
	if !ok {
		return errors.New("Invalid conversion from object any to string for key")
	}

	storeObj, exists, err := lbex.servicesStore.GetByKey(key)
	if err != nil {
		return err
	} else if exists {
		glog.V(3).Infof("sync: updating services for key: %s", key)
		glog.V(4).Infof("sync: updating services object %v", storeObj)
	} else {
		// TODO: this check needs to be outside the else condition, or have a
		// key that is guranteed to be unique from the service key.  Otherwise
		// endpoint objects will never get processed.
		storeObj, exists, err = lbex.endpointStore.GetByKey(key)
		if err != nil {
			return err
		} else if exists {
			glog.V(3).Infof("sync: updating endpoints for key %s", key)
			glog.V(4).Infof("sync: updating endpoint object %v", storeObj)
		} else {
			glog.V(3).Infof("sync: unable to find services or endpoint object for key value: %s", key)
		}
	}
	return nil
}
