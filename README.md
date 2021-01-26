# sosreport-operator

This guide contains building and installatoin instructions. See [USERGUIDE.md](USERGUIDE.md) for the user guide.

## Building and testing locally 

### Building the sosreport-centos images for a local registry

The operator requires a special image to run the sosreport jobs. Build this 
image with:
~~~
make podman-build-centos-sosreport IMG=kind:5000/sosreport-centos
make podman-push-centos-sosreport IMG=kind:5000/sosreport-centos
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
make podman-build IMG=kind:5000/sosreport-operator:v0.0.1
make podman-push IMG=kind:5000/sosreport-operator:v0.0.1
make deploy IMG=kind:5000/sosreport-operator:v0.0.1
~~~

To remove the operator again:
~~~
make undeploy
~~~

## Installing the Operator from quay.io images

~~~
make deploy IMG=quay.io/akaris/sosreport-operator:latest
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
make deploy-examples
make undeploy-examples
~~~

## Generating OLM bundle images

### For local registry
~~~
make bundle IMG=kind:5000/sosreport-operator:v0.0.1
make bundle-build-podman BUNDLE_IMG=kind:5000/sosreport-operator-bundle:v0.0.1
make bundle-push-podman BUNDLE_IMG=kind:5000/sosreport-operator-bundle:v0.0.1
make bundle-validate-podman BUNDLE_IMG=kind:5000/sosreport-operator-bundle:v0.0.1
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
make index-build BUNDLE_IMG=kind:5000/sosreport-operator-bundle:v0.0.1 INDEX_IMG=kind:5000/sosreport-operator-index:v0.0.1
make index-push-podman INDEX_IMG=kind:5000/sosreport-operator-index:v0.0.1
~~~

### Quay.io

~~~
make index-build BUNDLE_IMG=quay.io/akaris/sosreport-operator-bundle:latest INDEX_IMG=quay.io/akaris/sosreport-operator-index:latest
podman login quay.io
make index-push-podman INDEX_IMG=quay.io/akaris/sosreport-operator-index:latest
~~~
