# sosreport-operator

This guide contains building and installatoin instructions. See [USERGUIDE.md](USERGUIDE.md) for the user guide.

## Building and testing locally 

### Building the sosreport-centos images for a local registry

The operator requires a special image to run the sosreport jobs. Build this 
image with:
~~~
make podman-build-centos-sosreport REGISTRY=kind:5000
make podman-push-centos-sosreport REGISTRY=kind:5000
~~~
> Adjust the `IMG=(...)` value as needed.

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
make podman-build REGISTRY=kind:5000
make podman-push REGISTRY=kind:5000
make deploy REGISTRY=kind:5000
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

> quai.io will automatically build images from the latest commit

> Make sure that ConfigMap `sosreport-global-configuration` field `sosreport-image` points to `quay.io/akaris/sosreport-centos:main` (that's also the default if unset)

## Example custom resources (CRs)

Example custom resources can be deployed and undeployed with:
~~~
make deploy-examples REGISTRY=kind:5000 SIMULATION_MODE=false IMAGE_PULL_POLICY="Always"
make undeploy-examples
~~~
> For kind deployments, set SIMULATION_MODE=true
> For testing, set IMAGE_PULL_POLICY to "Always"

## Generating OLM bundle images

### For local registry
~~~
make bundle REGISTRY=kind:5000
make bundle-build-podman REGISTRY=kind:5000
make bundle-push-podman REGISTRY=kind:5000
make bundle-validate-podman REGISTRY=kind:5000
~~~

### For quay.io

~~~
make bundle IMG=quay.io/akaris/sosreport-operator:latest
~~~

Simply commit the current snapshot of the repository. Quay will automatically build an image from the latest snapshot.

## Generating OLM index images

> For further details, see [https://github.com/operator-framework/operator-registry#building-an-index-of-operators-using-opm](https://github.com/operator-framework/operator-registry#building-an-index-of-operators-using-opm)

Prerequisites - installing OPM:
~~~
make opm
~~~

### Local registry

~~~
make index-build REGISTRY=kind:5000
make index-push-podman REGISTRY=kind:5000
~~~

### Quay.io

~~~
make index-build BUNDLE_IMG=quay.io/akaris/sosreport-operator-bundle:latest INDEX_IMG=quay.io/akaris/sosreport-operator-index:latest
podman login quay.io
make index-push-podman INDEX_IMG=quay.io/akaris/sosreport-operator-index:latest
~~~

### Running automated tests

Running automated tests agains a testenv "fake" environment:
~~~
make test USE_EXISTING_CLUSTER=false
~~~

Running automated tests against a real environment
~~~
export KUBECONFIG=(...)
make test USE_EXISTING_CLUSTER=true REGISTRY=registry.example.com:5000
~~~


