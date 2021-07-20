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

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
)

func (r *Reconciler) addFinalizer(ctx context.Context, cr *operatorv1alpha1.OperandRequest) (bool, error) {
	if cr.GetDeletionTimestamp() == nil {
		originalReq := cr.DeepCopy()
		added := cr.EnsureFinalizer()
		if added {
			// Add finalizer to OperandRequest instance
			err := r.Patch(ctx, cr, client.MergeFrom(originalReq))
			if err != nil {
				return false, errors.Wrapf(err, "failed to update the OperandRequest %s/%s", cr.Namespace, cr.Name)
			}
			return false, nil
		}
	}
	return true, nil
}

func (r *Reconciler) checkFinalizer(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest) error {
	klog.V(1).Infof("Deleting OperandRequest %s in the namespace %s", requestInstance.Name, requestInstance.Namespace)
	existingSub := &olmv1alpha1.SubscriptionList{}

	opts := []client.ListOption{
		client.MatchingLabels(map[string]string{constant.OpreqLabel: "true"}),
	}

	if err := r.Client.List(ctx, existingSub, opts...); err != nil {
		return err
	}
	if len(existingSub.Items) == 0 {
		return nil
	}
	// Delete all the subscriptions that created by current request
	if err := r.absentOperatorsAndOperands(ctx, requestInstance); err != nil {
		return err
	}
	return nil
}
