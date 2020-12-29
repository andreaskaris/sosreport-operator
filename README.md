# sosreport-operator

## Building and testing locally 

### Building the sosreport-centos images for a local registry

The operator requires a special image to run the sosreports. Build this 
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

> Make sure that ConfigMap `sosreport-global-configuration` field `sosreport-image` points to `quay.io/akaris/sosreport-centos:main` (that's also the default if unset)

## Example custom resources (CRs)

Example custom resources can be deployed and undeployed with:
~~~
make deploy-examples
make undeploy-examples
~~~

## Customizing the sosreport configuration via ConfigMap

For testing purposes, it is possible to override a few settings to make it easier to run local images and custom commands. Modify the `sosreport-configuration` ConfigMap to set a few key settings:
~~~
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-global-configuration
  # namespace: sosreport-operator-system
data:
  sosreport-image: "kind:5000/sosreport-centos:latest"
  sosreport-command: "bash -x /entrypoint.sh"
  simulation-mode: "true"
  debug: "true"
~~~

## Configuring upload settings

The sosreport operator has an automatic upload feature which can be configured via ConfigMap:
~~~
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-upload-configuration
  # namespace: sosreport-operator-system
data:
  case-number: "00000000"
  upload-sosreport: "false"
  obfuscate: "false"
~~~

In order to upload images, you will also have to provide your Red Hat account credentials in form of a secret:
~~~
apiVersion: v1
kind: Secret
metadata:
  name: sosreport-upload-secret
  # namespace: sosreport-operator-system
type: kubernetes.io/basic-auth
stringData:
  username: test@example.com
  password: password
~~~
