# Sosreport Operator - Developer's Guide

This guide contains building and installation instructions for developers of this operator. 

See [USERGUIDE.md](USERGUIDE.md) for the user guide and user installation instructions.

## OperatorSDK dependency

This project was built with OperatorSDK version 1.2.0. Make sure to install it.
~~~
git clone https://github.com/operator-framework/operator-sdk
cd operator-sdk
git checkout v1.2.0
make install
~~~

## Building and testing locally 

### Building the sosreport-centos images for a local registry

The operator requires a special image to run the sosreport jobs. Build this 
image with the following commands.

For the CentOS image (`registry.example.com:5000/sosreport-centos`):
~~~
make podman-build-sosreport REGISTRY=registry.example.com:5000
make podman-push-sosreport REGISTRY=registry.example.com:5000
~~~

For the RHEL image (`registry.example.com:5000/sosreport-redhat-toolbox`):
~~~
make podman-build-sosreport REGISTRY=registry.example.com:5000 RHEL=true
make podman-push-sosreport REGISTRY=registry.example.com:5000  RHEL=true
~~~

> Adjust the `REGISTRY=(...)` value as needed.

> If building and using a RHEL toolbox image instead of CentOS, make sure that the local system is registered and provide `RHEL=true` to all commands that make a reference to `REGISTRY` or `SOSREPORT_IMG`

> Provide `SOSREPORT_IMG` to fully override the entire image path

> Go through the `Makefile` for more options

### Working on container images

Container common scripts are in `containers/scripts/`. These scripts are copied to the container sub directory with:
~~~
make podman-copy-sosreport-scripts 
~~~

~~~
make podman-copy-sosreport-scripts RHEL=true
~~~

This step is automatically executed when building the images.

When creating a new tag for quay.io, make sure to push the most recent scripts to the containers first, if any.

### Installing Custom Resource Definitions (CRDs)

Install Custom Resource Definitions with:
~~~
make generate
make manifests
make install
~~~

### Test running the operator locally

To test run the operator locally:
~~~
make run ENABLE_WEBHOOKS=false
~~~

### Installing the operator from a local registry

To install the operator:
~~~
make podman-build REGISTRY=registry.example.com:5000
make podman-push REGISTRY=registry.example.com:5000
make deploy REGISTRY=registry.example.com:5000
~~~

To remove the operator again:
~~~
make undeploy
~~~

## Installing the Operator from quay.io images

~~~
make deploy OPERATOR_IMG=quay.io/akaris/sosreport-operator:latest
~~~

To remove the operator again:
~~~
make undeploy
~~~

> quai.io will automatically build images from the latest tag in format `x.x.x`

> Make sure that ConfigMap `sosreport-global-configuration` field `sosreport-image` points to `quay.io/akaris/sosreport-centos:main` (that's also the default if unset)

## Example custom resources (CRs)

Example custom resources can be deployed with:
~~~
make deploy-examples REGISTRY=registry.example.com:5000 SIMULATION_MODE=false IMAGE_PULL_POLICY="Always"
~~~

And for the RHEL image:
~~~
make deploy-examples REGISTRY=registry.example.com:5000 SIMULATION_MODE=false IMAGE_PULL_POLICY="Always" RHEL=true
~~~

And undeployed with:
~~~
make undeploy-examples
~~~

> For environments that do not use RHEL for the OpenShift nodes such as kind, set `SIMULATION_MODE=true`

> For testing, set `IMAGE_PULL_POLICY` to "Always"

> For OpenShift make sure to set a privileged SCC:
~~~
oc new-project sosreport-test
oc adm policy add-scc-to-user privileged -z default
~~~

## Generating OLM bundle images

### For local registry
~~~
make bundle REGISTRY=registry.example.com:5000
make bundle-build-podman REGISTRY=registry.example.com:5000
make bundle-push-podman REGISTRY=registry.example.com:5000
make bundle-validate-podman REGISTRY=registry.example.com:5000
~~~

### For quay.io

~~~
make bundle REGISTRY=quay.io/akaris
~~~

Now, simply commit the current snapshot of the repository with a valid tag (e.g. 0.0.2).

Quay will automatically build an image from the tag.

## Generating OLM index images

> For further details, see [https://github.com/operator-framework/operator-registry#building-an-index-of-operators-using-opm](https://github.com/operator-framework/operator-registry#building-an-index-of-operators-using-opm)

Prerequisites - installing OPM:
~~~
make opm
~~~

### Local registry

~~~
make index-build REGISTRY=registry.example.com:5000
make index-push-podman REGISTRY=registry.example.com:5000
~~~

### Quay.io

~~~
make index-build REGISTRY=quay.io/akaris
podman login quay.io
make index-push-podman REGISTRY=quay.io/akaris
~~~

## Deploying via CatalogSource and Subscription

### Local registry

~~~
make deploy-index-subscription REGISTRY=registry.example.com:5000 CATALOG_SOURCE_NAMESPACE=olm
~~~

### Quay.io

~~~
make deploy-index-subscription REGISTRY=quay.io/akaris CATALOG_SOURCE_NAMESPACE=openshift-marketplace
~~~

### Undeploy

~~~
make undeploy-index-subscription
~~~

## Running automated tests

Running automated tests agains a testenv "fake" environment:
~~~
make test USE_EXISTING_CLUSTER=false
~~~

Running automated tests against a real environment
~~~
export KUBECONFIG=(...)
make test USE_EXISTING_CLUSTER=true REGISTRY=registry.example.com:5000
~~~

Running automated tests against a real environment with the RHEL image:
~~~
export KUBECONFIG=(...)
make test USE_EXISTING_CLUSTER=true REGISTRY=registry.example.com:5000 RHEL=true
~~~

## Spawning a deployment with the sosreport operator image

If you want to tinker around with the sosreport operator image in a deployment that simulated the jobs that would otherwise be deployed by the operator, run:
~~~
make deploy-test-deployment REGISTRY=registry.example.com:5000 # for CentOS
make deploy-test-deployment REGISTRY=registry.example.com:5000 RHEL=true # for RHEL
~~~
