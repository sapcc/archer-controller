# archer-controller
Kubernetes Controller for synchronizing Kubernetes Services to [Archer](https://github.com/sapcc/archer) Endpoint Services.

## Description
This is a simple implementation of an Kubernetes Controller that watches for Kubernetes Services with a specific annotation and creates or updates the corresponding Archer Endpoint Service.

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/archer-controller:tag
```

2. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/archer-controller:tag
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Configuration
The controller can be configured via the following parameters:
```sh
Usage of ./archer-controller:
  -health-probe-bind-address string
    	The address the probe endpoint binds to. (default ":8081")
  -kubeconfig string
    	Paths to a kubeconfig. Only required if out-of-cluster.
  -leader-elect
    	Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
  -metrics-bind-address string
    	The address the metric endpoint binds to. (default ":8080")
  -network-id string
    	The ID of the network to use for the service endpoint.
  -zap-devel
    	Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default true)
  -zap-encoder value
    	Zap log encoding (one of 'json' or 'console')
  -zap-log-level value
    	Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
  -zap-stacktrace-level value
    	Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
  -zap-time-encoding value
    	Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'epoch'.
```

## Usage
The controller watches for Kubernetes Services with the following annotation:
```yaml
cloud.sap/archer-create: true
    Boolean indicating whether a endpoint service needs to be created.
    Required.
cloud.sap/archer-network-id: <network-id>
    The ID of the network to use for the service.
    Optional.
cloud.sap/archer-service-name: <service-name>
    String specifying the name of the endpont service resource to be created.
    Defaults to the name of the service.
cloud.sap/archer-proxy-protocol: [true|false]
    Boolean indicating whether the endpoint service should use the proxy protocol.
    Defaults to false.
cloud.sap/archer-visibility: [public|private]
    String specifying the visibility of the endpoint service.
    Defaults to public.
cloud.sap/archer-require-approval: [true|false]
    Boolean indicating whether the endpoint service should require approval.
    Defaults to false.
cloud.sap/archer-tags: <tag1>,<tag2>,...
    Comma separated list of tags to be added to the endpoint service.
    Optional.
cloud.sap/archer-availability-zone: <availability-zone>
    String specifying the availability zone of the endpoint service.
    Optional.
cloud.sap/archer-port: <port>
    Integer specifying the IP port of the endpoint service.
    Optional.
```

Example:
```sh
kubectl create service clusterip --tcp 2345 test
kubectl annotate service test cloud.sap/archer-create="true"
```

## Contributing
If you are interested in contributing, please refer to the [CONTRIBUTING.md](CONTRIBUTING.md) document.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

