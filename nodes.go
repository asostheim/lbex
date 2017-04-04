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
	"reflect"

	"github.com/golang/glog"

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

var (
	nodeAPIResource = unversioned.APIResource{Name: "nodes", Namespaced: false, Kind: "node"}
)

func newNodesListWatchController() *lwController {
	return &lwController{
		stopCh: make(chan struct{}),
	}
}

func newNodesListWatchControllerForClientset(lbex *lbExController) *lwController {

	lwc := newNodesListWatchController()

	//Setup an informer to call functions when the ListWatch changes
	listWatch := cache.NewListWatchFromClient(
		lbex.clientset.Core().RESTClient(), "Nodes", api.NamespaceAll, fields.Everything())

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    nodeCreatedFunc(lbex),
		DeleteFunc: nodeDeletedFunc(lbex),
		UpdateFunc: nodeUpdatedFunc(lbex),
	}

	lbex.nodesStore, lwc.controller = cache.NewInformer(listWatch, &v1.Node{}, resyncPeriod, eventHandler)

	return lwc
}

// filterNode returns true if the node should be filtered, false otherwise
func filterNode(obj interface{}) bool {
	// obj can be filtered for either a: type conversion failure
	// *Removed Criteria* b: node is marked as scheduleable for pod placement.
	// checking scheduleable makes it impossible to remove a node that
	// has been newly marked as unschduleable.
	_, ok := obj.(*v1.Node)
	return !ok
}

func nodeCreatedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		if filterNode(obj) {
			glog.V(5).Infof("AddFunc: filtering out node object")
			return
		}
		glog.V(5).Infof("AddFunc: enqueuing node object")
		lbex.nodesQueue.Enqueue(obj)
	}
}

func nodeDeletedFunc(lbex *lbExController) func(obj interface{}) {
	return func(obj interface{}) {
		if filterNode(obj) {
			glog.V(5).Infof("DeleteFunc: filtering out node object")
			return
		}
		glog.V(5).Infof("DeleteFunc: enqueuing node object")
		lbex.nodesQueue.Enqueue(obj)
	}
}
func nodeUpdatedFunc(lbex *lbExController) func(obj, newObj interface{}) {
	return func(obj, newObj interface{}) {
		if filterNode(obj) {
			glog.V(5).Infof("UpdateFunc: filtering out node object")
			return
		}
		// TODO: Replace DeepEqual with a comparison that ignores the
		//       status.condition fields (specifically the timestamps).
		//       Would also be more peformant than reflect.
		if !reflect.DeepEqual(obj, newObj) {
			glog.V(5).Infof("UpdateFunc: enqueuing unequal node object")
			lbex.nodesQueue.Enqueue(newObj)
		}
	}
}