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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

type Installable struct {
	requestName string
	Operator    InstallableOperator
	Operand     InstallableOperand
}
type InstallableOperator struct {
	operatorGroup *olmv1.OperatorGroup
	namespace     *corev1.Namespace
	subscription  *olmv1alpha1.Subscription
	labels        map[string]string
	annotations   map[string]string
}

type InstallableOperand struct {
	customResourceList         []operand
	disabledcustomResourceList []operand
	source                     string
	labels                     map[string]string
	annotations                map[string]string
}

func (i *Installable) Install(ctx context.Context, r *Reconciler, instance *operatorv1alpha1.OperandRequest) error {
	if err := i.InstallOperator(ctx, r, instance); err != nil {
		return err
	}
	if err := i.CheckOperator(ctx, r, instance); err != nil {
		return err
	}
	if err := i.InstallCustomResource(ctx, r, instance); err != nil {
		return err
	}
	if err := i.DeleteCustomResource(ctx, r, instance); err != nil {
		return err
	}
	return nil
}

type operand struct {
	name                     string
	namespace                string
	customResource           unstructured.Unstructured
	customResourceFromALM    []byte
	customResourceFromConfig []byte
}

type Builder struct {
	Installable
}

func (i *Installable) InstallOperator(ctx context.Context, r *Reconciler, instance *operatorv1alpha1.OperandRequest) error {
	// Install namespace
	if i.Operator.namespace.Name != util.GetOperatorNamespace() && i.Operator.namespace.Name != constant.ClusterOperatorNamespace {
		if err := r.Create(ctx, i.Operator.namespace); err != nil && !apierrors.IsAlreadyExists(err) {
			klog.Warningf("failed to create the namespace %s, please make sure it exists: %s", i.Operator.namespace.Name, err)
		}
	}

	// Install operator group
	if i.Operator.namespace.Name != constant.ClusterOperatorNamespace {
		// Create required operatorgroup
		existOG := &olmv1.OperatorGroupList{}
		if err := r.Client.List(ctx, existOG, &client.ListOptions{Namespace: i.Operator.operatorGroup.Namespace}); err != nil {
			return err
		}
		if len(existOG.Items) == 0 {
			og := i.Operator.operatorGroup
			klog.V(3).Info("Creating the OperatorGroup" + og.Name)
			if err := r.Create(ctx, og); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	existingSub := &olmv1alpha1.Subscription{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: i.Operator.subscription.Name, Namespace: i.Operator.subscription.Namespace}, existingSub)
	if apierrors.IsNotFound(err) {
		// Install subscription
		instance.SetCreatingCondition(i.Operator.subscription.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, &r.Mutex)
		i.Operator.subscription.SetAnnotations(i.Operator.annotations)
		i.Operator.subscription.SetLabels(i.Operator.labels)
		if err := r.Create(ctx, i.Operator.subscription); err != nil && !apierrors.IsAlreadyExists(err) {
			instance.SetCreatingCondition(i.Operator.subscription.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse, &r.Mutex)
			return err
		}
	} else {
		return err
	}

	// Update subscription
	if _, ok := existingSub.Labels[constant.OpreqLabel]; !ok {
		// Subscription existing and not managed by OperandRequest controller
		klog.V(1).Infof("Subscription %s in namespace %s isn't created by ODLM. Ignore update/delete it.", i.Operator.subscription.Name, i.Operator.subscription.Namespace)
		return nil
	}

	originalSub := existingSub.DeepCopy()
	existingSub.Spec.CatalogSource = i.Operator.subscription.Spec.CatalogSource
	existingSub.Spec.Channel = i.Operator.subscription.Spec.Channel
	existingSub.Spec.CatalogSourceNamespace = i.Operator.subscription.Spec.CatalogSourceNamespace
	existingSub.Spec.InstallPlanApproval = i.Operator.subscription.Spec.InstallPlanApproval

	if existingSub.Annotations == nil {
		existingSub.Annotations = make(map[string]string)
	}
	for key, anno := range i.Operator.annotations {
		existingSub.Annotations[key] = anno
	}

	if equality.Semantic.DeepEqual(originalSub, existingSub) {
		return nil
	}

	if err = r.updateSubscription(ctx, instance, existingSub); err != nil {
		instance.SetMemberStatus(i.requestName, operatorv1alpha1.OperatorFailed, "", &r.Mutex)
		return err
	}
	instance.SetMemberStatus(i.requestName, operatorv1alpha1.OperatorUpdating, "", &r.Mutex)
	return nil
}

func (i *Installable) CheckOperator(ctx context.Context, r *Reconciler, instance *operatorv1alpha1.OperandRequest) error {

	// Checking installplan
	ip, err := r.GetInstallPlan(ctx, i.Operator.subscription)
	if err != nil {
		return err
	}
	// Checking CSV
	csv, err := r.GetCSV(ctx, ip)
	if err != nil {
		return err
	}

	if csv.Status.Phase == olmv1alpha1.CSVPhaseFailed {
		return fmt.Errorf("the ClusterServiceVersion %s/%s is Failed", csv.Namespace, csv.Name)
	}

	if csv.Status.Phase != olmv1alpha1.CSVPhaseSucceeded {
		return fmt.Errorf("the ClusterServiceVersion %s/%s is not Ready", csv.Namespace, csv.Name)
	}

	return nil
}

func (i *Installable) InstallCustomResource(ctx context.Context, r *Reconciler, instance *operatorv1alpha1.OperandRequest) error {
	for _, cr := range i.Operand.customResourceList {
		existingCR := &unstructured.Unstructured{}
		err := r.Client.Get(ctx, types.NamespacedName{Namespace: cr.namespace, Name: cr.name}, existingCR)
		if apierrors.IsNotFound(err) {
			if cr.customResource.GetLabels() == nil {
				cr.customResource.SetLabels(i.Operand.labels)
			} else {
				label := cr.customResource.GetLabels()
				for k, v := range i.Operand.labels {
					label[k] = v
				}
				cr.customResource.SetLabels(label)
			}
			if err := r.Client.Create(ctx, &cr.customResource); err != nil {
				//multi err
				return err
			}
			continue
		}

		if err != nil {
			return err
		}

		if !checkLabel(existingCR, map[string]string{constant.OpreqLabel: "true"}) {
			continue
		}

		if i.Operand.source == "OperandConfig" {
			existingCRRaw, err := json.Marshal(existingCR.Object["spec"])
			if err != nil {
				return err
			}
			// TODO: Check cr struct

			updatedExistingCRwithALM := util.MergeCR(cr.customResourceFromALM, existingCRRaw)
			updatedExistingCRwithALMRaw, err := json.Marshal(updatedExistingCRwithALM)
			if err != nil {
				return err
			}

			updatedExistingCRwithConfig := util.MergeCR(updatedExistingCRwithALMRaw, cr.customResourceFromConfig)

			if equality.Semantic.DeepEqual(existingCR.Object["spec"], updatedExistingCRwithConfig) {
				continue
			}
			existingCR.Object["spec"] = updatedExistingCRwithConfig
			err = r.Update(ctx, existingCR)

			if err != nil {
				return errors.Wrapf(err, "failed to update custom resource -- Kind: %s, NamespacedName: %s/%s", existingCR.GetKind(), existingCR.GetNamespace(), existingCR.GetName())
			}
		} else {
			if equality.Semantic.DeepEqual(existingCR.Object["spec"], cr.customResource.Object["spec"]) {
				continue
			}
			existingCR.Object["spec"] = cr.customResource.Object["spec"]
			err = r.Update(ctx, existingCR)
			if err != nil {
				return errors.Wrapf(err, "failed to update custom resource -- Kind: %s, NamespacedName: %s/%s", existingCR.GetKind(), existingCR.GetNamespace(), existingCR.GetName())
			}
		}
	}
	return nil
}

func (i *Installable) DeleteCustomResource(ctx context.Context, r *Reconciler, instance *operatorv1alpha1.OperandRequest) error {
	for _, cr := range i.Operand.disabledcustomResourceList{
		existingCR := &unstructured.Unstructured{}
		err := r.Client.Get(ctx, types.NamespacedName{Namespace: cr.namespace, Name: cr.name}, existingCR)
		if apierrors.IsNotFound(err) {
			continue
		}

		if err != nil{
			return err
		}

		if err := r.Delete(ctx, &cr.customResource); err != nil {
			return nil
		}
	}
	return nil
}
func (b *Builder) setOperator(o *operatorv1alpha1.Operator) *Builder {
	var ns string

	if o.InstallMode == operatorv1alpha1.InstallModeCluster {
		ns = constant.ClusterOperatorNamespace
	} else {
		ns = o.Namespace
	}

	sub := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name,
			Namespace: ns,
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                o.Channel,
			Package:                o.PackageName,
			CatalogSource:          o.SourceName,
			CatalogSourceNamespace: o.SourceNamespace,
			InstallPlanApproval:    o.InstallPlanApproval,
			StartingCSV:            o.StartingCSV,
		},
	}
	b.Installable.Operator.subscription = sub
	return b
}

func (b *Builder) setRequestName(request *operatorv1alpha1.Operand) *Builder {
	b.Installable.requestName = request.Name
	return b
}

func (b *Builder) setNamespace(o *operatorv1alpha1.Operator) *Builder {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.Namespace,
		},
	}
	b.Installable.Operator.namespace = ns
	return b
}

func (b *Builder) setOperatorGroup(o *operatorv1alpha1.Operator) *Builder {
	b.Installable.Operator.operatorGroup = generateOperatorGroup(o.Namespace, o.TargetNamespaces)
	return b
}

func (b *Builder) setOperatorLabel() *Builder {
	b.Installable.Operator.labels = map[string]string{
		constant.OpreqLabel: "true",
	}
	return b
}

func (b *Builder) setOperandLabel() *Builder {
	b.Installable.Operand.labels = map[string]string{
		constant.OpreqLabel: "true",
	}
	return b
}

func (b *Builder) setOperatorAnnotations(requestKey, registryKey types.NamespacedName) *Builder {
	b.Installable.Operator.annotations = map[string]string{
		registryKey.Namespace + "." + registryKey.Name + "/registry": "true",
		registryKey.Namespace + "." + registryKey.Name + "/config":   "true",
		requestKey.Namespace + "." + requestKey.Name + "/request":    "true",
	}
	return b
}

func (b *Builder) setOperand(requestInstance *operatorv1alpha1.OperandRequest, request *operatorv1alpha1.Operand, opt *operatorv1alpha1.Operator, config *operatorv1alpha1.OperandConfig, pm *operatorsv1.PackageManifest, requestKey types.NamespacedName) *Builder {
	if request.Kind == "" {
		b.Operand.source = "OperandConfig"
		// Check the requested Service Config if exist in specific OperandConfig
		config := config.GetService(request.Name)
		if config == nil {
			return b
		}

		almExamples := getAlmFromPackageManifest(pm, opt.Channel)
		// Convert CR template string to slice
		var almExampleList []interface{}
		err := json.Unmarshal([]byte(almExamples), &almExampleList)
		if err != nil {
			klog.Errorf("failed to convert alm-examples from operator %s to slice: %v", request.Name, err)
			return b
		}

		foundMap := make(map[string]bool)
		for cr := range config.Spec {
			foundMap[cr] = false
		}

		crMapping := make(map[string]runtime.RawExtension)
		for crdName, crdConfig := range config.Spec {
			crMapping[strings.ToLower(crdName)] = crdConfig
		}

		for _, almExample := range almExampleList {
			// Create an unstructured object for CR and check its value
			var crFromALM unstructured.Unstructured
			crFromALM.Object = almExample.(map[string]interface{})

			var name string
			if request.InstanceName == "" {
				name = crFromALM.GetName()
			} else {
				name = request.InstanceName
			}

			spec := crFromALM.Object["spec"]
			if spec == nil {
				continue
			}

			specFromConfig, ok := crMapping[strings.ToLower(name)]
			if !ok {
				disabledOperand := operand{
					name:           name,
					namespace:      opt.Namespace,
					customResource: crFromALM,
				}
				b.Installable.Operand.disabledcustomResourceList = append(b.Installable.Operand.disabledcustomResourceList, disabledOperand)
			}

			//Convert CR template spec to string
			specJSONString, _ := json.Marshal(crFromALM.Object["spec"])
			// Merge CR template spec and OperandConfig spec
			mergedCR := util.MergeCR(specJSONString, specFromConfig.Raw)
			crFromALM.Object["spec"] = mergedCR

			operand := operand{
				name:                     name,
				namespace:                opt.Namespace,
				customResource:           crFromALM,
				customResourceFromALM:    specJSONString,
				customResourceFromConfig: specFromConfig.Raw,
			}
			b.Installable.Operand.customResourceList = append(b.Installable.Operand.customResourceList, operand)
		}

	} else {
		b.Operand.source = "OperandRequest"
		// Create an unstructured object for CR and check its value
		var crFromRequest unstructured.Unstructured

		if request.APIVersion == "" {
			klog.Errorf("The APIVersion of operand is empty for operator " + request.Name)
			return b
		}

		if request.Kind == "" {
			klog.Errorf("The Kind of operand is empty for operator " + request.Name)
			return b
		}

		var name string
		if request.InstanceName == "" {
			crInfo := sha256.Sum256([]byte(request.APIVersion + request.Kind))
			name = requestKey.Name + "-" + hex.EncodeToString(crInfo[:7])
		} else {
			name = request.InstanceName
		}

		crFromRequest.SetName(name)
		crFromRequest.SetNamespace(requestKey.Namespace)
		crFromRequest.SetAPIVersion(request.APIVersion)
		crFromRequest.SetKind(request.Kind)
		// crFromRequest.Object["spec"] = request.Spec
		json.Unmarshal(request.Spec.Raw, crFromRequest.Object["spec"])
		enableOperand := operand{
			name:           name,
			namespace:      requestKey.Namespace,
			customResource: crFromRequest,
		}
		b.Installable.Operand.customResourceList = append(b.Installable.Operand.customResourceList, enableOperand)

		// Get CustomResource should be deleted
		members := requestInstance.Status.Members

		customeResourceMap := make(map[string]operatorv1alpha1.OperandCRMember)
		for _, member := range members {
			if len(member.OperandCRList) != 0 {
				for _, cr := range member.OperandCRList {
					customeResourceMap[member.Name+"/"+cr.Kind+"/"+cr.Name] = cr
				}
			}
		}
		for _, req := range requestInstance.Spec.Requests {
			for _, opd := range req.Operands {
				if opd.Kind != "" {
					var name string
					if opd.InstanceName == "" {
						crInfo := sha256.Sum256([]byte(opd.APIVersion + opd.Kind))
						name = requestInstance.Name + "-" + hex.EncodeToString(crInfo[:7])
					} else {
						name = opd.InstanceName
					}
					delete(customeResourceMap, opd.Name+"/"+opd.Kind+"/"+name)
				}
			}
		}
		for _, opdMember := range customeResourceMap {
			var disabledCustomResource unstructured.Unstructured
			disabledCustomResource.SetAPIVersion(opdMember.APIVersion)
			disabledCustomResource.SetKind(opdMember.Kind)
			disabledOperand := operand{
				name:           opdMember.Name,
				namespace:      requestKey.Namespace,
				customResource: disabledCustomResource,
			}
			b.Installable.Operand.disabledcustomResourceList = append(b.Installable.Operand.disabledcustomResourceList, disabledOperand)
		}

	}
	return b
}

func (b *Builder) done() *Installable {
	return &b.Installable
}

func (r *Reconciler) newInstallableOperator(ctx context.Context, request *operatorv1alpha1.OperandRequest) []*Installable {
	var installableList []*Installable
	for _, req := range request.Spec.Requests {
		key := request.GetRegistryKey(req)
		registry, _ := r.GetOperandRegistry(ctx, key)
		config, _ := r.GetOperandConfig(ctx, key)
		for _, operand := range req.Operands {
			opt := registry.GetOperator(operand.Name)
			pm, err := r.GetPackageManifest(ctx, opt.PackageName, opt.Namespace, opt.Channel, key.Namespace)
			if err != nil || pm == nil {
				continue
			}
			op := buildInstallable(request, &operand, opt, config, pm, types.NamespacedName{Namespace: request.Namespace, Name: request.Name}, key)
			installableList = append(installableList, op)
		}
	}
	return installableList
}

func getAlmFromPackageManifest(pm *operatorsv1.PackageManifest, channelName string) string {
	for _, channel := range pm.Status.Channels {
		if channel.Name == channelName {
			continue
		}
		if channel.CurrentCSVDesc.Annotations != nil {
			return channel.CurrentCSVDesc.Annotations["alm-examples"]
		}
	}
	return ""
}

func buildInstallable(requestInstance *operatorv1alpha1.OperandRequest, request *operatorv1alpha1.Operand, opt *operatorv1alpha1.Operator, config *operatorv1alpha1.OperandConfig, pm *operatorsv1.PackageManifest, requestKey, registryKey types.NamespacedName) *Installable {
	return new(Builder).
		setRequestName(request).
		setNamespace(opt).
		setOperator(opt).
		setOperatorGroup(opt).
		setOperatorLabel().
		setOperatorAnnotations(requestKey, registryKey).
		setOperand(requestInstance, request, opt, config, pm, requestKey).
		done()

}
