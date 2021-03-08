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

> In order to avoid overloading the cluster, the Sosreport Operator will run a specific number of Sosreports at a time, by default 1. This is *per sosreport*. Hence, if you create 3 different sosreport resources, it will run 3 sosreports at a time.

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

You can spawn a set of pods to access the Physical Volumes:
~~~
PVCS=$(oc get pvc -o name | sed 's#persistentvolumeclaim/##')
for PVC in $PVCS ; do 
cat <<EOF | oc apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: pod-$PVC
spec:
  volumes:
    - name: $PVC
      persistentVolumeClaim:
        claimName: $PVC
  containers:
    - name: pv-container
      image: fedora
      command:
      - sleep
      - infinity
      volumeMounts:
        - mountPath: "/pv"
          name: $PVC
EOF
done
~~~

This will spawn a set of additional pods:
~~~
[root@openshift-jumpserver-0 sosreport-operator]# oc get pods
NAME                                                         READY   STATUS      RESTARTS   AGE
pod-sosreport-sample-openshift-worker-0-20210305200645-pvc   1/1     Running     0          2m54s
pod-sosreport-sample-openshift-worker-1-20210305200927-pvc   1/1     Running     0          2m54s
sosreport-sample-openshift-worker-0-20210305200645-b479n     0/1     Completed   0          23m
sosreport-sample-openshift-worker-1-20210305200927-26bf4     0/1     Completed   0          20m
~~~

Once the pods are up, copy the files to your local drive:
~~~
PVCS=$(oc get pvc -o name | sed 's#persistentvolumeclaim/##')
for PVC in $PVCS ; do 
POD=pod-$PVC
f=$(oc exec -it $POD ls /pv  | tr -d '\n' | tr -d '\r')
oc cp ${POD}:/pv/${f} ${f}
done
~~~

Verify that the sosreports were copied:
~~~
[root@openshift-jumpserver-0 tmp]# ls -al sosreport*
-rw-r--r--. 1 root root 38783848 Mar  5 20:32 sosreport-openshift-worker-0-2021-03-05-xmcqjwu.tar.xz
-rw-r--r--. 1 root root 37731176 Mar  5 20:32 sosreport-openshift-worker-1-2021-03-05-pvrlfik.tar.xz
~~~

And tear down the pods:
~~~
PVCS=$(oc get pvc -o name | sed 's#persistentvolumeclaim/##')
for PVC in $PVCS ; do 
POD=pod-$PVC
oc delete pod $POD
done
~~~

## Deleting Sosreports 

Simply run `oc delete sosreport <name>`. When deleting a Sosreport Custom Resource, all associated resources such as jobs, pods and also the PVCs and the PVs will be deleted. This makes it easy to reclaim the space used by Sosreports.

## Sosreport upload settings

The Sosreport Operator allows the automatic upload of generated sosreports to:

* Red Hat support cases
* NFS
* FTP

### Automatic upload to Red Hat support cases

Create a ConfigMap named `sosreport-upload-configuration` in the Sosreport's namespace. Specify `upload-method` `case` and set the case number. Select `obfuscate` `true|false` depending on if you want to run the `sos` obfuscate feature:
~~~
cat <<'EOF' > sosreport-upload-configuration-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-upload-configuration
data:
  case-number: "00000000"
  upload-method: "case"
  obfuscate: "false"
EOF
oc apply -f sosreport-upload-configuration-configmap.yaml
~~~

Create a secret with your RHN username and password:
~~~
cat <<'EOF' > sosreport-upload-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: sosreport-upload-secret
type: kubernetes.io/basic-auth
stringData:
  username: test@example.com
  password: password
EOF
oc apply -f sosreport-upload-secret.yaml
~~~

> **Note:** Passwords which are stored in Kubernetes Secrets can be seen by any user who has the administrative rights to view Secrets. The Username and Password will be passed to the Jobs and Pods via environment variables and will be in clear text and are visible in the Pods' definitions.

With this configuration in place, create a set of Sosreports, e.g. with:
~~~
cat <<'EOF' | oc apply -f -
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
EOF
~~~

You can monitor the sosreport generation and upload progress by following a Pod's logs, for example with:
~~~
$ oc logs -f sosreport-sample-openshift-worker-0-20210308103224-kwcbx
~~~

Towards the end of the log, you should see:
~~~
Uploading sosreport-openshift-worker-0-00000000-2021-03-08-eompair.tar.xz to the case ... completed successfully.
~~~

### Automatic upload to FTP servers

> **Note:** At this point in time, the FTP upload feature is not sufficiently implemented and tested. It may work, but it also may not.

Create a ConfigMap named `sosreport-upload-configuration` in the Sosreport's namespace. Specify `upload-method` `ftp` and set `ftp-server` to the address of the FTP server:
~~~
cat <<'EOF' > sosreport-upload-configuration-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-upload-configuration
data:
  upload-method: "ftp"
  ftp-server: "ftp://kind"
EOF
oc apply -f sosreport-upload-configuration-configmap.yaml
~~~

Create a secret with your FTP username and password:
~~~
cat <<'EOF' > sosreport-upload-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: sosreport-upload-secret
type: kubernetes.io/basic-auth
stringData:
  username: test@example.com
  password: password
EOF
oc apply -f sosreport-upload-secret.yaml
~~~

> **Note:** Passwords which are stored in Kubernetes Secrets can be seen by any user who has the administrative rights to view Secrets. The Username and Password will be passed to the Jobs and Pods via environment variables and will be in clear text and are visible in the Pods' definitions.

> **Note:** FTPS was not tested and will more than likely not work at the moment.

With this configuration in place, create a set of Sosreports, e.g. with:
~~~
cat <<'EOF' | oc apply -f -
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
EOF
~~~

### Automatic upload to NFS servers

Create a ConfigMap named `sosreport-upload-configuration` in the Sosreport's namespace. Specify `upload-method` `nfs` and set `ftp-share` to the share of the NFS serve. Set options to be passed to the NFS mount (as `-o ${nfs-options}`) via `nfs-options`:
~~~
cat <<'EOF' > sosreport-upload-configuration-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-upload-configuration
data:
  upload-method: "nfs"
  nfs-share: "kind:/nfs" # must be set for nfs
  nfs-options: "" # optional for nfs
EOF
oc apply -f sosreport-upload-configuration-configmap.yaml
~~~

With this configuration in place, create a set of Sosreports, e.g. with:
~~~
cat <<'EOF' | oc apply -f -
apiVersion: support.openshift.io/v1alpha1
kind: Sosreport
metadata:
  name: sosreport-sample
EOF
~~~

## Advanced customization of Sosreport configuration via ConfigMap

Create the `sosreport-global-configuration` ConfigMap to set a few key settings such as the log level, Sosreport concurrency and the PVC configuration.

> **Note:** This ConfigMap must be in the same namespace as the `Sosreport` resource

~~~
apiVersion: v1
kind: ConfigMap
metadata:
  name: sosreport-global-configuration
data:
  log-level: "0"
  concurrency: "1"
  pvc-storage-class: "standard"
  pvc-capacity: "5Gi"
~~~

* `log-level`: Set log-level of log-messages. Currently, the lower the log-level (min `0`), the more verbose
* `concurrency`: Set number of concurrent Sosreports. The default is 1 and this should not be raised too high.
* `pvc-storage-class`: Name of PVC storage class
* `pvc-capacity`: Name of PVC capacity


## For development and testing only

For specific purposes, it is possible to override a few settings to make it easier to run local images and custom commands. These parameters are explained here and are meant for development and troubleshooting purposes.

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
~~~

* `sosreport-image`: Use a custom image for the Sosreport jobs
* `sosreport-command`: Use a custom entrypoing command for Sosreport jobs
* `simulation-mode`: Generate Sosreports locally in the container instead of on the node (required for testing in `kind` environments)
* `debug`: Set Sosreport jobs' scripts to debug mode
