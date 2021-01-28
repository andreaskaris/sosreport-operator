## Creating Sosreports

### Creating Sosreports on all systems

To run Sosreports on all systems, simply apply the following configuration:
~~~
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
~~~

In order to avoid overloading the cluster, the Sosreport Operator will run a specific number of Sosreports at a time, by default 1. 

Sosreports will be stored on each node in /host/var/tmp. 

> The Sosreport Operator does currently not take care of cleanups.

### Adding Tolerations to Sosreports

Sosreport jobs will respect Node Taints. One can work around this by configuring tolerations.

For example, in order to spawn Sosreports on master nodes with a `NoSchedule` taint:
~~~
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
spec:
   tolerations:
   - key: node-role.kubernetes.io/master
     effect: NoSchedule
~~~

### Creating Sosreports on a subset of nodes

Sosreports can easily be executed on a subset of nodes.

#### Run Sosreports only on nodes with a specific role

For example, in order to generate Sosreports on all master nodes:
~~~
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
spec:
  nodeSelector:
    node-role.kubernetes.io/master: ""
~~~

#### Run Sosreports on a specific node

For example, to run Sosreports on the node with hostname `worker-0`, create the following Sosreport:
~~~
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample 
spec:
  nodeSelector:
    kubernetes.io/hostname: worker-0
~~~

## Customizing Sosreport configuration via ConfigMap

For specific purposes, it is possible to override a few settings to make it easier to run local images and custom commands. These parameters are explained in `For testing only`.

Other parameters can be used in production environments. See `For real world deployments` for these.

Create the `sosreport-global-configuration` ConfigMap to set a few key settings.

> **Note:** This ConfigMap must be in the same namespace as the `Sosreport` resource

~~~
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-global-configuration
data:
  sosreport-image: "kind:5000/sosreport-centos:latest"
  sosreport-command: "bash -x /scripts/entrypoint.sh"
  simulation-mode: "true"
  debug: "true"
  log-level: "0"
  concurrency: "1"
  pvc-storage-class: "standard"
  pvc-capacity: "5Gi"
~~~

### For testing only

* `sosreport-image`: Use a custom image for the Sosreport jobs
* `sosreport-command`: Use a custom entrypoing command for Sosreport jobs
* `simulation-mode`: Generate Sosreports locally in the container instead of on the node (required for testing in `kind` environments)
* `debug`: Set Sosreport jobs' scripts to debug mode

### For real world deployments

* `log-level`: Set log-level of log-messages. Currently, the lower the log-level (min `0`), the more verbose
* `concurrency`: Set number of concurrent Sosreports. The default is 1 and this should not be raised too high.
* `pvc-storage-class`: Name of PVC storage class
* `pvc-capacity`: Name of PVC capacity

## Configuring upload settings

The Sosreport operator has an automatic upload feature which can be configured via ConfigMap `sosreport-upload-configuration`.

> **Note:** This ConfigMap must be in the same namespace as the `Sosreport` resource

> **Note:** In all cases, the Sosreport operator will maintain a local copy of each Sosreport file

### Uploading directly to a case via RH support tool

In order to upload directly to a case via Red Hat support tool, set the following values:
~~~
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-upload-configuration
data:
  case-number: "00000000"
  upload-method: "case"
  obfuscate: "false"
~~~

* `upload-method`: Set this to `case`
* `case-number`: Red Hat support case number
* `obfuscate`: Remove sensitive data from the sosreport before uploading

Create the following secret:
~~~
apiVersion: v1
kind: Secret
metadata:
  name: sosreport-upload-secret
type: kubernetes.io/basic-auth
stringData:
  username: test@example.com
  password: password
~~~

* `username`: Red Hat username
* `password`: Red Hat password

> **Note:** Passwords which are stored in Kubernetes `Secrets` can be seen by any user who has the administrative rights to view `Secrets`. The Username and Password will be passed to the Jobs and Pods via environment variables and will be in clear text and are visible in the Pods' definitions.

### Upload to FTP

Create the following ConfigMap to upload to FTP:
~~~
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-upload-configuration
data:
  upload-method: "ftp"
  ftp-server: "ftp://kind"
~~~

* `upload-method`: Must be `ftp`
* `ftp-server`: Address of FTP server

Create the following secret:
~~~
apiVersion: v1
kind: Secret
metadata:
  name: sosreport-upload-secret
type: kubernetes.io/basic-auth
stringData:
  username: test@example.com
  password: password
~~~

* `username`: FTP username
* `password`: FTP password

> **Note:** Passwords which are stored in Kubernetes `Secrets` can be seen by any user who has the administrative rights to view `Secrets`. The Username and Password will be passed to the Jobs and Pods via environment variables and will be in clear text and are visible in the Pods' definitions.

> **Note:** FTPS was not tested and will more than likely not work at the moment.

### Upload to NFS

Create the following ConfigMap to upload to NFS
~~~
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-upload-configuration
data:
  upload-method: "nfs"
  nfs-share: "kind:/nfs" # must be set for nfs
  nfs-options: "" # optional for nfs
~~~

* `upload-method`: Must be `nfs`
* `nfs-share`: Path to the NFS share
* `nfs-options`: Options to be passed to the NFS mount (as `-o ${nfs-options}`)

