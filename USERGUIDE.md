## Creating sosreports

### Creating sosreports on all systems

To run sosreports on all systems, simply apply the following configuration:
~~~
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
#spec:
#  nodeSelector:
#    type: worker
~~~

In order to avoid overloading the cluster, the Sosreport Operator will run a specific number of sosreports at a time, by default 1. 

Sosreports will be stored on each node in /host/var/tmp. 

> The Sosreport Operator does currently not take care of cleanups.

### Creating sosreports on a subset of nodes

Sosreports can easily be executes on a subset of nodes.

#### Run sosreports only on nodes with the worker role

In order to generate sosreports on all master nodes:
~~~
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
spec:
  nodeSelector:
    node-role.kubernetes.io/master: ""
~~~

#### Run sosreports on a specific node

To run sosreports on the node with hostname `worker-0`, create the following sosreport:
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

For specific purposes, it is possible to override a few settings to make it easier to run local images and custom commands. Create the `sosreport-configuration` ConfigMap to set a few key settings:
~~
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
~~~

> **Note:** This ConfigMap must be in the same namespace as the `Sosreport` resource

### For testing only

* `sosreport-image`: Use a custom image for the Sosreport jobs
* `sosreport-command`: Use a custom entrypoing command for Sosreport jobs
* `simulation-mode`: Generate Sosreports locally in the container instead of on the node (required for `kind`)
* `debug`: Set Sosreport jobs' scripts to debug mode

### For real world deployments

* `log-level`: Set log-level of log-messages. Currently, the lower the log-level (min `0`), the more verbose
* `concurrency`: Set number of concurrent Sosreports. The default is 1 and this should not be raised too high.

## Configuring upload settings

The sosreport operator has an automatic upload feature which can be configured via ConfigMap `sosreport-upload-configuration`.

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

> **Note:** Passwords which are stored in Kubernetes `Secrets` can be seen by any user who has the administrative rights to view secrets. The Username and Password will be passed to the Jobs and Pods via environment variables and will be in clear.

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

> **Note:** Passwords which are stored in Kubernetes `Secrets` can be seen by any user who has the administrative rights to view secrets. The Username and Password will be passed to the Jobs and Pods via environment variables and will be in clear.
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

