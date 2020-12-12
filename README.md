# sosreport-operator

## Building the sosreport-centos images

The operator requires a special image to run the sosreports. Build this 
image with:
~~~
make podman-build-centos-sosreport IMG=kind:5000/sosreport-centos
make podman-push-centos-sosreport IMG=kind:5000/sosreport-centos
~~~
> Adjust the `IMG=(...)` value as needed.

## Installing Custom Resource Definitions (CRDs)

Install Custom Resource Definitions with:
~~~
make generate
make manifests
make install
~~~

## Test running the operator

To test run the operator locally:
~~~
make run ENABLE_WEBHOOKS=false
~~~

## Installing the operator

To install the operator:
~~~
(...)
~~~

## Example custom resources (CRs)

Example custom resources can be deployed and undeployed with:
~~~
make deploy-examples
make undeploy-examples
~~~

## Customizing the sosreport via ConfigMap

For testing purposes, it is possible to override a few settings to make it easier to run local images and custom commands. Modify the `sosreport-configuration` ConfigMap to set a few key settings:
~~~
$ cat ./config/samples/sosreport-config-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-configuration
data:
  sosreport-image: "kind:5000/sosreport-centos"
  sosreport-command: "bash /entrypoint.sh"
~~~
