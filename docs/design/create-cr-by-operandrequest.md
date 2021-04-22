# Create Custom Resource by OperandRequest

Using OperandConfig can apply a default custom resource according to the alm-example in the CSV, which provides convenience to users using a template to customize their own custom resource.

However, while it provides convenience, it also creates some limitations:

- Users can't create multiple custom resources for the same CustomResourceDefinition.
- Users can't create the custom resource in a different namespace from the operator.
- Users have to update the OperandConfig to customize the custom resource.

Thus, we implement creating Custom Resource by OperandRequest to decouple with alm-example and OperandConfig. Customized resources are completely generated by the configuration of OperandRequest.

## How to create Custom Resource by OperandRequest

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: OperandRequest
metadata:
  name: db2-instance1
  namespace: my-service
spec:
  requests:
  - registry: common-service
    registryNamespace: ibm-common-services
    operands:
    - name: ibm-db2-operator
      kind: db2
      apiVersion: operator.ibm.com/v1alpha1
      instanceName: db2-instance1
      spec:
        replicas: 3
    - name: ibm-db2-operator
      kind: db2
      apiVersion: operator.ibm.com/v1alpha1
      instanceName: db2-instance2
      spec:
        replicas: 3
```

The above `OperandRequest` will create two `db2` custom resources in the `my-service` namespace and the `db2` operator will be created in the namespace specified in the OperandRegistry `common-service` in the `ibm-common-service` namespace.

The first custom resource is

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: db2
metadata:
  name: db2-instance1
  namespace: my-service
spec:
  replicas: 3
```

The second custom resource is

```yaml
apiVersion: operator.ibm.com/v1alpha1
kind: db2
metadata:
  name: db2-instance2
  namespace: my-service
spec:
  replicas: 3
```