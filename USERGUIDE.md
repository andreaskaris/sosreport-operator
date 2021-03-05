## Installing on OpenShift

First, deploy the catalog source:
~~~
cat <<'EOF' > catalogsource.yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: sosreport-operator-manifests
  namespace: "openshift-marketplace"
spec:
  sourceType: grpc
  image: "quay.io/akaris/sosreport-operator-index:0.0.1"
EOF
oc apply -f catalogsource.yaml
~~~

Verify with the following commands:
~~~
[root@openshift-jumpserver-0 sosreport-operator]# oc get catalogsources  -A | grep sosreport
openshift-marketplace   sosreport-operator-manifests 
[root@openshift-jumpserver-0 sosreport-operator]# oc get packagemanifests -A | grep sosreport
openshift-marketplace   sosreport-operator 
~~~

Then, deploy the Operator:
~~~
cat <<'EOF' > subscription.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: sosreport-operator
spec: {}
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: sosreport-og
  namespace: sosreport-operator
spec: {}
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: sosreport-operator-subscription
  namespace: sosreport-operator
spec:
  channel: alpha
  name: sosreport-operator
  source: sosreport-operator-manifests
  sourceNamespace: "openshift-marketplace"
EOF
oc apply -f subscription.yaml
~~~

Verify with the following commands:
~~~
[root@openshift-jumpserver-0 sosreport-operator]# oc project sosreport-operator
Now using project "sosreport-operator" on server "https://api.cluster.example.com:6443".
[root@openshift-jumpserver-0 sosreport-operator]# oc get og
NAME           AGE
sosreport-og   27m
[root@openshift-jumpserver-0 sosreport-operator]# oc get sub
NAME                              PACKAGE              SOURCE                         CHANNEL
sosreport-operator-subscription   sosreport-operator   sosreport-operator-manifests   alpha
[root@openshift-jumpserver-0 sosreport-operator]# oc get installplan
NAME            CSV                         APPROVAL    APPROVED
install-cgrvj   sosreport-operator.v0.0.1   Automatic   true
[root@openshift-jumpserver-0 sosreport-operator]# oc get csv
NAME                        DISPLAY              VERSION   REPLACES   PHASE
sosreport-operator.v0.0.1   sosreport-operator   0.0.1                Succeeded
[root@openshift-jumpserver-0 sosreport-operator]# oc get pods
NAME                                                     READY   STATUS    RESTARTS   AGE
sosreport-operator-controller-manager-7b4775d7b4-rzvnm   2/2     Running   0          14m
~~~

## Creating Sosreports

### Creating a new namespace and making the default user privileged

The sosreport operator pods run with wide privilege in order to allow them to capture the data that they need. Create a new namespace where you wish to run the sosreports and add the default service account to the privileged SCC:
~~~
oc new-project sosreport-test
oc adm policy add-scc-to-user privileged -z default
~~~

### Creating Sosreports on all systems

To run Sosreports on all systems without taints (this excludes the masters by default), simply apply the following configuration:
~~~
cat <<'EOF' | oc apply -f -
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
EOF
~~~

In order to avoid overloading the cluster, the Sosreport Operator will run a specific number of Sosreports at a time, by default 1. This is *per sosreport*. Hence, if you create 3 different sosreport resources, it will run 3 sosreports at a time.

### Adding Tolerations to Sosreports

Sosreport jobs will respect Node Taints. One can work around this by configuring tolerations.

For example, in order to spawn Sosreports on master nodes with a `NoSchedule` taint:
~~~
cat <<'EOF' | oc apply -f -
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
spec:
   tolerations:
   - key: node-role.kubernetes.io/master
     effect: NoSchedule
EOF
~~~

### Creating Sosreports on a subset of nodes

Sosreports can easily be executed on a subset of nodes.

#### Run Sosreports only on nodes with a specific role

For example, in order to generate Sosreports on all master nodes:
~~~
cat <<'EOF' | oc apply -f -
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
spec:
  nodeSelector:
    node-role.kubernetes.io/master: ""
   tolerations:
   - key: node-role.kubernetes.io/master
     effect: NoSchedule
EOF
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

## Monitoring Sosreport status

Sosreports emit events whenever something meaningful happens:
~~~
[root@openshift-jumpserver-0 samples]# oc describe sosreport
(...)
Events:
  Type    Reason                 Age                            From       Message
  ----    ------                 ----                           ----       -------
  Normal  Sosreport job started  14s                            Sosreport  Sosreport started on openshift-worker-0
  Normal  Sosreport finished     <invalid>                      Sosreport  Sosreport openshift-worker-0 finished
  Normal  Sosreport job started  <invalid>                      Sosreport  Sosreport started on openshift-worker-1
  Normal  Sosreport finished     <invalid>                      Sosreport  Sosreport openshift-worker-1 finished
  Normal  Sosreports finished    <invalid> (x2 over <invalid>)  Sosreport  All Sosreports finished
~~~

Running Sosreports will show `IN PROGRESS` = `true`:
~~~
[root@openshift-jumpserver-0 samples]# oc get sosreport
NAME               FINISHED   IN PROGRESS
sosreport-sample              true
~~~

Once all Sosreports executed, you will see:
~~~
[root@openshift-jumpserver-0 samples]# oc get sosreport
NAME               FINISHED   IN PROGRESS
sosreport-sample   true 
~~~

Also use `oc get jobs`, `oc get pods`, `oc get pvc`, `oc get pv`, `oc get events` for further details.

## Where are Sosreports stored?

Sosreports will be stored in dedicated Physical Volumes.

~~~
[root@openshift-jumpserver-0 samples]# oc get pvc
NAME                                                     STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
sosreport-sample-openshift-worker-0-20210305200645-pvc   Bound    pvc-064bd41d-0bf2-41df-830e-5ea95211a3bf   10Gi       RWO            managed-nfs-storage   95s
~~~

### Accessing Sosreports on PVs

(... TBD ...)

## Deleting Sosreports 

Simply run `oc delete sosreport <name>`. When deleting a Sosreport Custom Resource, all associated resources such as jobs, pods and also the PVCs and the PVs will be deleted. This makes it easy to reclaim the space used by Sosreports.

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

