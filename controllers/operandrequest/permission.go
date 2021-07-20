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
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) checkPermission(ctx context.Context, req ctrl.Request) bool {
	// Check update permission
	if !r.checkUpdateAuth(ctx, req.Namespace, "operator.ibm.com", "operandrequests") {
		return false
	}
	if !r.checkUpdateAuth(ctx, req.Namespace, "operator.ibm.com", "operandrequests/status") {
		return false
	}
	return true
}

// Check if operator has permission to update OperandRequest
func (r *Reconciler) checkUpdateAuth(ctx context.Context, namespace, group, resource string) bool {
	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "update",
				Group:     group,
				Resource:  resource,
			},
		},
	}

	if err := r.Create(ctx, sar); err != nil {
		klog.Errorf("Failed to check operator update permission: %v", err)
		return false
	}

	klog.V(3).Infof("Operator update permission in namespace %s, Allowed: %t, Denied: %t, Reason: %s", namespace, sar.Status.Allowed, sar.Status.Denied, sar.Status.Reason)
	return sar.Status.Allowed
}
