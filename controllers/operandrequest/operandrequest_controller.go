//
// Copyright 2021 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package operandrequest

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/controllers/operator"
)

// Reconciler reconciles a OperandRequest object
type Reconciler struct {
	*deploy.ODLMOperator
	StepSize int
	Mutex    sync.Mutex
}
type clusterObjects struct {
	namespace     *corev1.Namespace
	operatorGroup *olmv1.OperatorGroup
	subscription  *olmv1alpha1.Subscription
}

// Reconcile reads that state of the cluster for a OperandRequest object and makes changes based on the state read
// and what is in the OperandRequest.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reconcileErr error) {
	// Fetch the OperandRequest instance
	requestInstance := &operatorv1alpha1.OperandRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, requestInstance); err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	originalInstance := requestInstance.DeepCopy()

	// Always attempt to patch the status after each reconciliation.
	defer func() {
		if equality.Semantic.DeepEqual(originalInstance.Status, requestInstance.Status) {
			return
		}
		if err := r.Client.Status().Patch(ctx, requestInstance, client.MergeFrom(originalInstance)); err != nil {
			reconcileErr = utilerrors.NewAggregate([]error{reconcileErr, fmt.Errorf("error while patching OperandRequest.Status: %v", err)})
		}
	}()

	// Remove finalizer when DeletionTimestamp none zero
	if !requestInstance.ObjectMeta.DeletionTimestamp.IsZero() {

		// Check and clean up the subscriptions
		err := r.checkFinalizer(ctx, requestInstance)
		if err != nil {
			klog.Errorf("failed to clean up the subscriptions for OperandRequest %s: %v", req.NamespacedName.String(), err)
			return ctrl.Result{}, err
		}

		originalReq := requestInstance.DeepCopy()
		// Update finalizer to allow delete CR
		if requestInstance.RemoveFinalizer() {
			err = r.Patch(ctx, requestInstance, client.MergeFrom(originalReq))
			if err != nil {
				klog.Errorf("failed to remove finalizer for OperandRequest %s: %v", req.NamespacedName.String(), err)
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if operator has the update permission to update OperandRequest
	hasPermission := r.checkPermission(ctx, req)
	if !hasPermission {
		klog.Warningf("No permission to update OperandRequest")
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	klog.V(1).Infof("Reconciling OperandRequest: %s", req.NamespacedName)
	// Update labels for the request
	if requestInstance.UpdateLabels() {
		if err := r.Patch(ctx, requestInstance, client.MergeFrom(originalInstance)); err != nil {
			klog.Errorf("failed to update the labels for OperandRequest %s: %v", req.NamespacedName.String(), err)
			return ctrl.Result{}, err
		}
	}

	// Initialize the status for OperandRequest instance
	if !requestInstance.InitRequestStatus() {
		return ctrl.Result{Requeue: true}, nil
	}

	// Add finalizer to the instance
	if isAdded, err := r.addFinalizer(ctx, requestInstance); err != nil {
		klog.Errorf("failed to add finalizer for OperandRequest %s: %v", req.NamespacedName.String(), err)
		return ctrl.Result{}, err
	} else if !isAdded {
		return ctrl.Result{Requeue: true}, err
	}

	// Reconcile Operators
	installableOperatorList := r.newInstallableOperator(ctx, requestInstance)

	for _, installableOperator := range installableOperatorList {
		installableOperator.Install(ctx, r, requestInstance)
	}
	// Reconcile Operators
	if err := r.reconcileOperator(ctx, requestInstance); err != nil {
		klog.Errorf("failed to reconcile Operators for OperandRequest %s: %v", req.NamespacedName.String(), err)
		return ctrl.Result{}, err
	}

	// Reconcile Operands
	if merr := r.reconcileOperand(ctx, requestInstance); len(merr.Errors) != 0 {
		klog.Errorf("failed to reconcile Operands for OperandRequest %s: %v", req.NamespacedName.String(), merr)
		return ctrl.Result{}, merr
	}

	// Check if all csv deploy succeed
	if requestInstance.Status.Phase != operatorv1alpha1.ClusterPhaseRunning {
		klog.V(2).Info("Waiting for all operators and operands to be deployed successfully ...")
		return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
	}

	klog.V(1).Infof("Finished reconciling OperandRequest: %s", req.NamespacedName)
	return ctrl.Result{RequeueAfter: constant.DefaultSyncPeriod}, nil
}

func (r *Reconciler) getRegistryToRequestMapper() handler.MapFunc {
	ctx := context.Background()
	return func(object client.Object) []ctrl.Request {
		requestList, _ := r.ListOperandRequestsByRegistry(ctx, types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()})

		requests := []ctrl.Request{}
		for _, request := range requestList {
			namespaceName := types.NamespacedName{Name: request.Name, Namespace: request.Namespace}
			req := ctrl.Request{NamespacedName: namespaceName}
			requests = append(requests, req)
		}
		return requests
	}
}

func (r *Reconciler) getSubToRequestMapper() handler.MapFunc {
	return func(object client.Object) []ctrl.Request {
		reg, _ := regexp.Compile(`^(.*)\.(.*)\/request`)
		annotations := object.GetAnnotations()
		var reqName, reqNamespace string
		for annotation := range annotations {
			if reg.MatchString(annotation) {
				annotationSlices := strings.Split(annotation, ".")
				reqNamespace = annotationSlices[0]
				reqName = strings.Split(annotationSlices[1], "/")[0]
			}
		}
		if reqNamespace == "" || reqName == "" {
			return []ctrl.Request{}
		}
		return []ctrl.Request{
			{NamespacedName: types.NamespacedName{
				Name:      reqName,
				Namespace: reqNamespace,
			}},
		}
	}
}

func (r *Reconciler) getConfigToRequestMapper() handler.MapFunc {
	ctx := context.Background()
	return func(object client.Object) []ctrl.Request {
		requestList, _ := r.ListOperandRequestsByConfig(ctx, types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()})

		requests := []ctrl.Request{}
		for _, request := range requestList {
			namespaceName := types.NamespacedName{Name: request.Name, Namespace: request.Namespace}
			req := ctrl.Request{NamespacedName: namespaceName}
			requests = append(requests, req)
		}
		return requests
	}
}

// SetupWithManager adds OperandRequest controller to the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.OperandRequest{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &olmv1alpha1.Subscription{}}, handler.EnqueueRequestsFromMapFunc(r.getSubToRequestMapper()), builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject := e.ObjectOld.(*olmv1alpha1.Subscription)
				newObject := e.ObjectNew.(*olmv1alpha1.Subscription)
				if oldObject.Labels != nil && oldObject.Labels[constant.OpreqLabel] == "true" {
					return (oldObject.Status.InstalledCSV != "" && newObject.Status.InstalledCSV != "" && oldObject.Status.InstalledCSV != newObject.Status.InstalledCSV)
				}
				return false
			},
		})).
		Watches(&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}}, handler.EnqueueRequestsFromMapFunc(r.getRegistryToRequestMapper()), builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject := e.ObjectOld.(*operatorv1alpha1.OperandRegistry)
				newObject := e.ObjectNew.(*operatorv1alpha1.OperandRegistry)
				return !equality.Semantic.DeepEqual(oldObject.Spec, newObject.Spec)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Evaluates to false if the object has been confirmed deleted.
				return !e.DeleteStateUnknown
			},
		})).
		Watches(&source.Kind{Type: &operatorv1alpha1.OperandConfig{}}, handler.EnqueueRequestsFromMapFunc(r.getConfigToRequestMapper()), builder.WithPredicates(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Evaluates to false if the object has been confirmed deleted.
				return !e.DeleteStateUnknown
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject := e.ObjectOld.(*operatorv1alpha1.OperandConfig)
				newObject := e.ObjectNew.(*operatorv1alpha1.OperandConfig)
				return !equality.Semantic.DeepEqual(oldObject.Spec, newObject.Spec)
			},
		})).Complete(r)
}

