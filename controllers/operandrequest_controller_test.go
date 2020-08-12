//
// Copyright 2020 IBM Corporation
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

package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	testdata "github.com/IBM/operand-deployment-lifecycle-manager/controllers/common"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("OperandRequest controller", func() {
	// Define utility constants for object names.
	const (
		name              = "ibm-cloudpak-name"
		namespace         = "ibm-cloudpak"
		registryName      = "common-service"
		registryNamespace = "ibm-common-service"
		operatorNamespace = "ibm-operators"
	)

	var (
		ctx context.Context

		registry    *operatorv1alpha1.OperandRegistry
		config      *operatorv1alpha1.OperandConfig
		request     *operatorv1alpha1.OperandRequest
		requestKey  types.NamespacedName
		registryKey types.NamespacedName
		configKey   types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		requestNamespaceName := createNSName(namespace)
		registryNamespaceName := createNSName(registryNamespace)
		operatorNamespaceName := createNSName(operatorNamespace)

		registry = testdata.OperandRegistryObj(registryName, registryNamespaceName, operatorNamespaceName)
		config = testdata.OperandConfigObj(registryName, registryNamespaceName)
		request = testdata.OperandRequestObj(registryName, registryNamespaceName, name, requestNamespaceName)
		requestKey = types.NamespacedName{Name: name, Namespace: requestNamespaceName}
		registryKey = types.NamespacedName{Name: registryName, Namespace: registryNamespaceName}
		configKey = types.NamespacedName{Name: registryName, Namespace: registryNamespaceName}

		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(requestNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(registryNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(operatorNamespaceName))).Should(Succeed())

		Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
		Expect(k8sClient.Create(ctx, config)).Should(Succeed())

		By("By creating a OperandRequest to trigger OperandRequest controller")
		Expect(k8sClient.Create(ctx, request)).Should(Succeed())
	})

	AfterEach(func() {
		By("Deleting the OperandRequest")
		Expect(k8sClient.Delete(ctx, request)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, config)).Should(Succeed())
	})

	Context("Sharing the the secret and configmap with public scope", func() {
		It("Should OperandRequest is completed", func() {

			By("Checking status of the OperandBindInfo")
			registryInstance := &operatorv1alpha1.OperandRegistry{}
			Expect(k8sClient.Get(ctx, registryKey, registryInstance)).Should(Succeed())
			configInstance := &operatorv1alpha1.OperandConfig{}
			Expect(k8sClient.Get(ctx, configKey, configInstance)).Should(Succeed())
			requestInstance := &operatorv1alpha1.OperandRequest{}
			Expect(k8sClient.Get(ctx, requestKey, requestInstance)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println(requestInstance.Status)
				return true
			}).Should(BeFalse())

		})
	})
})

// import (
// 	"context"
// 	"testing"
// 	"time"

// 	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
// 	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
// 	"github.com/stretchr/testify/assert"
// 	corev1 "k8s.io/api/core/v1"
// 	"k8s.io/apimachinery/pkg/api/errors"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	"k8s.io/apimachinery/pkg/types"
// 	"k8s.io/client-go/kubernetes/scheme"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// 	"sigs.k8s.io/controller-runtime/pkg/client/fake"
// 	"sigs.k8s.io/controller-runtime/pkg/reconcile"

// 	"github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
// 	fetch "github.com/IBM/operand-deployment-lifecycle-manager/controllers/common"
// 	constant "github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
// )

// // TestRequestController runs OperandRequestReconciler.Reconcile() against a
// // fake client that tracks a OperandRequest object.
// func TestRequestController(t *testing.T) {
// 	var (
// 		name              = "ibm-cloudpak-name"
// 		namespace         = "ibm-cloudpak"
// 		registryName      = "common-service"
// 		registryNamespace = "ibm-common-service"
// 		operatorNamespace = "ibm-operators"
// 	)

// 	req := getReconcileRequest(name, namespace)
// 	r := getReconciler(name, namespace, registryName, registryNamespace, operatorNamespace)
// 	requestInstance := &v1alpha1.OperandRequest{}

// 	initReconcile(t, r, req, requestInstance, registryName, registryNamespace, operatorNamespace)

// 	absentOperand(t, r, req, requestInstance, operatorNamespace)

// 	presentOperand(t, r, req, requestInstance, operatorNamespace)

// 	updateOperandCustomResource(t, r, req, registryName, registryNamespace)

// 	absentOperandCustomResource(t, r, req, registryName, registryNamespace)

// 	presentOperandCustomResource(t, r, req, registryName, registryNamespace)

// 	deleteOperandRequest(t, r, req, requestInstance, operatorNamespace)
// }

// // Init reconcile the OperandRequest
// func initReconcile(t *testing.T, r OperandRequestReconciler, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, registryName, registryNamespace, operatorNamespace string) {
// 	assert := assert.New(t)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}
// 	assert.NoError(err)

// 	registryKey := types.NamespacedName{Name: registryName, Namespace: registryNamespace}
// 	// Retrieve OperandRegistry
// 	registryInstance, err := fetch.FetchOperandRegistry(r, registryKey)
// 	assert.NoError(err)

// 	// Retrieve OperandRequest
// 	retrieveOperandRequest(t, r, req, requestInstance, 2, operatorNamespace)

// 	// err = checkOperandCustomResource(t, r, registryInstance, 3, 8081, "3.2.13")
// 	// assert.NoError(err)
// }

// // Retrieve OperandRequest instance
// func retrieveOperandRequest(t *testing.T, r OperandRequestReconciler, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, expectedMemNum int, operatorNamespace string) {
// 	assert := assert.New(t)
// 	err := r.Get(context.TODO(), req.NamespacedName, requestInstance)
// 	assert.NoError(err)
// 	assert.Equal(expectedMemNum, len(requestInstance.Status.Members), "operandrequest member list should have two elements")

// 	subs := &olmv1alpha1.SubscriptionList{}
// 	err := r.List(context.TODO(), subs, &client.ListOptions{Namespace: operatorNamespace})
// 	assert.NoError(err)
// 	assert.Equalf(expectedMemNum, len(subs.Items), "subscription list should have %s Subscriptions", expectedMemNum)

// 	csvs := &olmv1alpha1.ClusterServiceVersionList{}
// 	err := r.List(context.TODO(), csvs, &client.ListOptions{Namespace: operatorNamespace})
// 	assert.NoError(err)
// 	assert.Equalf(expectedMemNum, len(csvs.Items), "csv list should have %s ClusterServiceVersions", expectedMemNum)

// 	for _, m := range requestInstance.Status.Members {
// 		assert.Equalf(v1alpha1.OperatorRunning, m.Phase.OperatorPhase, "operator(%s) phase should be Running", m.Name)
// 		assert.Equalf(v1alpha1.ServiceRunning, m.Phase.OperandPhase, "operand(%s) phase should be Running", m.Name)
// 	}
// 	assert.Equalf(v1alpha1.ClusterPhaseRunning, requestInstance.Status.Phase, "request(%s/%s) phase should be Running", requestInstance.Namespace, requestInstance.Name)
// }

// // Absent an operator from request
// func absentOperand(t *testing.T, r OperandRequestReconciler, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, operatorNamespace string) {
// 	assert := assert.New(t)
// 	requestInstance.Spec.Requests[0].Operands = requestInstance.Spec.Requests[0].Operands[1:]
// 	err := r.Update(context.TODO(), requestInstance)
// 	assert.NoError(err)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}
// 	assert.NoError(err)
// 	retrieveOperandRequest(t, r, req, requestInstance, 1, operatorNamespace)
// }

// // Present an operator from request
// func presentOperand(t *testing.T, r OperandRequestReconciler, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, operatorNamespace string) {
// 	assert := assert.New(t)
// 	// Present an operator into request
// 	requestInstance.Spec.Requests[0].Operands = append(requestInstance.Spec.Requests[0].Operands, v1alpha1.Operand{Name: "etcd"})
// 	err := r.Update(context.TODO(), requestInstance)
// 	assert.NoError(err)

// 	err = r.createSubscription(requestInstance, &v1alpha1.Operator{
// 		Name:            "etcd",
// 		Namespace:       operatorNamespace,
// 		SourceName:      "community-operators",
// 		SourceNamespace: "openshift-marketplace",
// 		PackageName:     "etcd",
// 		Channel:         "singlenamespace-alpha",
// 	})
// 	assert.NoError(err)
// 	err = r.reconcileOperator(req.NamespacedName)
// 	assert.NoError(err)

// 	sub := sub("etcd", operatorNamespace, "0.0.1")
// 	err := r.Status().Update(context.TODO(), sub)
// 	assert.NoError(err)

// 	csv := csv("etcd-csv.v0.0.1", operatorNamespace, etcdExample)
// 	err := r.Create(context.TODO(), csv)
// 	assert.NoError(err)

// 	multiErr := r.reconcileOperand(req.NamespacedName)
// 	assert.Empty(multiErr.Errors, "all the operands reconcile should not be error")
// 	// err = r.updateMemberStatus(requestInstance)
// 	// assert.NoError(err)
// 	retrieveOperandRequest(t, r, req, requestInstance, 2, operatorNamespace)
// }

// // Mock delete OperandRequest instance, mark OperandRequest instance as delete state
// func deleteOperandRequest(t *testing.T, r OperandRequestReconciler, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, operatorNamespace string) {
// 	assert := assert.New(t)
// 	deleteTime := metav1.NewTime(time.Now())
// 	requestInstance.SetDeletionTimestamp(&deleteTime)
// 	err := r.Update(context.TODO(), requestInstance)
// 	assert.NoError(err)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}

// 	subs := &olmv1alpha1.SubscriptionList{}
// 	err := r.List(context.TODO(), subs, &client.ListOptions{Namespace: operatorNamespace})
// 	assert.NoError(err)
// 	assert.Empty(subs.Items, "all the Subscriptions should be deleted")

// 	csvs := &olmv1alpha1.ClusterServiceVersionList{}
// 	err := r.List(context.TODO(), csvs, &client.ListOptions{Namespace: operatorNamespace})
// 	assert.NoError(err)
// 	assert.Empty(csvs.Items, "all the ClusterServiceVersions should be deleted")

// 	// Check if OperandRequest instance deleted
// 	err = r.Delete(context.TODO(), requestInstance)
// 	assert.NoError(err)
// 	err = r.Get(context.TODO(), req.NamespacedName, requestInstance)
// 	assert.True(errors.IsNotFound(err), "retrieve operand request should be return an error of type is 'NotFound'")
// }

// func getReconciler(name, namespace, registryName, registryNamespace, operatorNamespace string) OperandRequestReconciler {
// 	s := scheme.Scheme
// 	v1alpha1.SchemeBuilder.AddToScheme(s)
// 	olmv1.SchemeBuilder.AddToScheme(s)
// 	olmv1alpha1.SchemeBuilder.AddToScheme(s)

// 	initData := initClientData(name, namespace, registryName, registryNamespace, operatorNamespace)

// 	// Create a fake client to mock API calls.
// 	client := fake.NewFakeClient(initData.odlmObjs...)

// 	// Return a OperandRequestReconciler object with the scheme and fake client.
// 	return OperandRequestReconciler{
// 		Scheme: s,
// 		Client: client,
// 	}
// }

// // Mock request to simulate Reconcile() being called on an event for a watched resource
// func getReconcileRequest(name, namespace string) reconcile.Request {
// 	return reconcile.Request{
// 		NamespacedName: types.NamespacedName{
// 			Name:      name,
// 			Namespace: namespace,
// 		},
// 	}
// }

// // Update an operand custom resource from config
// func updateOperandCustomResource(t *testing.T, r OperandRequestReconciler, req reconcile.Request, registryName, registryNamespace string) {
// 	assert := assert.New(t)
// 	// Retrieve OperandConfig
// 	configInstance, err := fetch.FetchOperandConfig(r, types.NamespacedName{Name: registryName, Namespace: registryNamespace})
// 	assert.NoError(err)
// 	for id, operator := range configInstance.Spec.Services {
// 		if operator.Name == "etcd" {
// 			configInstance.Spec.Services[id].Spec = map[string]runtime.RawExtension{
// 				"etcdCluster": {Raw: []byte(`{"size": 1}`)},
// 			}
// 		}
// 	}
// 	err = r.Update(context.TODO(), configInstance)
// 	assert.NoError(err)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}
// 	assert.NoError(err)
// 	updatedRequestInstance := &v1alpha1.OperandRequest{}
// 	err = r.Get(context.TODO(), req.NamespacedName, updatedRequestInstance)
// 	assert.NoError(err)
// 	for _, operator := range updatedRequestInstance.Status.Members {
// 		if operator.Name == "etcd" {
// 			assert.Equal(v1alpha1.ServiceRunning, operator.Phase.OperandPhase, "The etcd phase status should be running in the OperandRequest")
// 		}
// 	}
// 	assert.Equal(v1alpha1.ClusterPhaseRunning, updatedRequestInstance.Status.Phase, "The cluster phase status should be running in the OperandRequest")

// 	registryInstance, err := fetch.FetchOperandRegistry(r, types.NamespacedName{Name: registryName, Namespace: registryNamespace})
// 	assert.NoError(err)
// 	err = checkOperandCustomResource(t, r, registryInstance, 1, 8081, "3.2.13")
// 	assert.NoError(err)
// }

// // Absent an operand custom resource from config
// func absentOperandCustomResource(t *testing.T, r OperandRequestReconciler, req reconcile.Request, registryName, registryNamespace string) {
// 	assert := assert.New(t)
// 	// Retrieve OperandConfig
// 	registryKey := types.NamespacedName{Name: registryName, Namespace: registryNamespace}
// 	configInstance, err := fetch.FetchOperandConfig(r, registryKey)
// 	assert.NoError(err)
// 	for id, operator := range configInstance.Spec.Services {
// 		if operator.Name == "etcd" {
// 			configInstance.Spec.Services[id].Spec = make(map[string]runtime.RawExtension)
// 		}
// 	}
// 	err = r.Update(context.TODO(), configInstance)
// 	assert.NoError(err)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}
// 	assert.NoError(err)
// 	configInstance, err = fetch.FetchOperandConfig(r, registryKey)
// 	assert.NoError(err)
// 	assert.Equal(v1alpha1.ServiceNone, configInstance.Status.ServiceStatus["etcd"].CrStatus["etcdCluster"], "The status of etcdCluster should be cleaned up in the OperandConfig")
// 	updatedRequestInstance := &v1alpha1.OperandRequest{}
// 	err = r.Get(context.TODO(), req.NamespacedName, updatedRequestInstance)
// 	assert.NoError(err)
// 	// for _, operator := range updatedRequestInstance.Status.Members {
// 	// 	if operator.Name == "etcd" {
// 	// 		assert.Equal(v1alpha1.ServiceNone, operator.Phase.OperandPhase, "The etcd phase status should be none in the OperandRequest")
// 	// 	}
// 	// }
// 	assert.Equal(v1alpha1.ClusterPhaseRunning, updatedRequestInstance.Status.Phase, "The cluster phase status should be running in the OperandRequest")
// }

// // Present an operand custom resource from config
// func presentOperandCustomResource(t *testing.T, r OperandRequestReconciler, req reconcile.Request, registryName, registryNamespace string) {
// 	assert := assert.New(t)
// 	// Retrieve OperandConfig
// 	configInstance := operandConfig(registryName, registryNamespace)
// 	err := r.Update(context.TODO(), configInstance)
// 	assert.NoError(err)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}
// 	assert.NoError(err)
// 	updatedRequestInstance := &v1alpha1.OperandRequest{}
// 	err = r.Get(context.TODO(), req.NamespacedName, updatedRequestInstance)
// 	assert.NoError(err)
// 	for _, operator := range updatedRequestInstance.Status.Members {
// 		if operator.Name == "etcd" {
// 			assert.Equal(v1alpha1.ServiceRunning, operator.Phase.OperandPhase, "The etcd phase status should be none in the OperandRequest")
// 		}
// 	}
// 	assert.Equal(v1alpha1.ClusterPhaseRunning, updatedRequestInstance.Status.Phase, "The cluster phase status should be running in the OperandRequest")
// }

// // func checkOperandCustomResource(t *testing.T, r OperandRequestReconciler, registryInstance *v1alpha1.OperandRegistry, etcdSize, jenkinsPort int, etcdVersion string) error {
// // 	assert := assert.New(t)
// // 	for _, operator := range registryInstance.Spec.Operators {
// // 		if operator.Name == "etcd" {
// // 			etcdCluster := &v1beta2.EtcdCluster{}
// // 			err := r.Get(context.TODO(), types.NamespacedName{
// // 				Name:      "example",
// // 				Namespace: operator.Namespace,
// // 			}, etcdCluster)
// // 			if err != nil {
// // 				return err
// // 			}
// // 			assert.Equalf(etcdCluster.Spec.Size, etcdSize, "The size of etcdCluster should be %d, but it is %d", etcdSize, etcdCluster.Spec.Size)
// // 			assert.Equalf(etcdCluster.Spec.Version, etcdVersion, "The version of etcdCluster should be %s, but it is %s", etcdVersion, etcdCluster.Spec.Version)
// // 		}
// // 		if operator.Name == "jenkins" {
// // 			jenkins := &v1alpha2.Jenkins{}
// // 			err := r.Get(context.TODO(), types.NamespacedName{
// // 				Name:      "example",
// // 				Namespace: operator.Namespace,
// // 			}, jenkins)
// // 			if err != nil {
// // 				return err
// // 			}
// // 			assert.Equalf(jenkins.Spec.Service.Port, int32(jenkinsPort), "The post of jenkins service should be %d, but it is %d", int32(jenkinsPort), jenkins.Spec.Service.Port)
// // 		}
// // 	}
// // 	return nil
// // }

const etcdExample string = `
[
	{
	  "apiVersion": "etcd.database.coreos.com/v1beta2",
	  "kind": "EtcdCluster",
	  "metadata": {
		"name": "example"
	  },
	  "spec": {
		"size": 3,
		"version": "3.2.13"
	  }
	}
]
`
const jenkinsExample string = `
[
	{
	  "apiVersion": "jenkins.io/v1alpha2",
	  "kind": "Jenkins",
	  "metadata": {
		"name": "example"
	  },
	  "spec": {
		"service": {"port": 8081}
	  }
	}
]
`
