/*
Copyright 2020.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	// appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	//"sigs.k8s.io/controller-runtime/pkg/handler"
	//"sigs.k8s.io/controller-runtime/pkg/reconcile"
	//"sigs.k8s.io/controller-runtime/pkg/source"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/yaml"

	supportv1alpha1 "github.com/andreaskaris/sosreport-operator/api/v1alpha1"
)

const (
	GLOBAL_CONFIG_MAP_NAME        = "sosreport-global-configuration"        // name of the global ConfigMap with overrides
	DEVELOPMENT_CONFIG_MAP_NAME   = "sosreport-development-configuration"   // name of the global ConfigMap with overrides
	UPLOAD_CONFIG_MAP_NAME        = "sosreport-upload-configuration"        // name of the upload ConfigMap
	UPLOAD_SECRET_NAME            = "sosreport-upload-secret"               // name of the Secret for upload authentication
	DEFAULT_IMAGE_NAME            = "quay.io/akaris/sosreport-centos:0.0.2" // to point to final version of sosreport IMAGE
	DEFAULT_SOSREPORT_COMMAND     = "bash /scripts/entrypoint.sh"           // to point to the entrypoint
	DEFAULT_SOSREPORT_CONCURRENCY = 1
	DEFAULT_PVC_SIZE              = "10Gi"
	DEFAULT_IMAGE_PULL_POLICY     = ""   // Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	IS_DEVELOPER_MODE             = true // potentially unsafe settings that can easily be disabled
	DEBUG                         = 1
	INFO                          = 0
)

type SosreportLogLevel struct {
	MinLevel zapcore.Level
}

// SosreportReconciler reconciles a Sosreport object
type SosreportReconciler struct {
	client.Client
	Log             logr.Logger
	DynamicLogLevel *SosreportLogLevel
	Scheme          *runtime.Scheme
	recorder        record.EventRecorder
	// the runList takes care of race conditions due to caching delay
	// a sosreport on the runList will not be run again
	runList map[types.UID]struct{}
	// list of jobs to run
	jobToRunList map[types.UID]map[string]struct{} // will be jobToRunList[s.UID][nodeName]
	// list of jobs to currently running
	jobRunningList       map[types.UID]map[string]struct{} // will be jobToRunList[s.UID][nodeName]
	imageName            string                            // name of the soreport job's image
	sosreportCommand     string                            // command to run for the sosreport image
	sosreportConcurrency int                               // command to run for the sosreport image
	pvcStorageClass      string
	pvcCapacity          string
	imagePullPolicy      string
}

var log logr.Logger
var ctx context.Context

// +kubebuilder:rbac:groups=support.openshift.io,resources=sosreports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=support.openshift.io,resources=sosreports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=support.openshift.io,resources=sosreports/finalizers,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=jobs/finalizers,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/finalizers,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets/status,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events/status,verbs=get
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/status,verbs=get
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use

func (r *SosreportReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx = context.Background()
	log = r.Log.WithValues("sosreport", req.NamespacedName)

	log.V(DEBUG).Info("Reconciler loop triggered")

	// retrieve sosreport CR to be reconciliated
	sosreport := &supportv1alpha1.Sosreport{}
	if err := r.Get(ctx, req.NamespacedName, sosreport); err != nil {
		log.V(DEBUG).Info("Failed to get Sosreport custom resource - was it deleted?")
		//log.Error(err, "Failed to get Sosreport custom resource - was it deleted?")
		// return ctrl.Result{}, err
		return ctrl.Result{}, nil
	}

	/*
	 A sosreport will not be run if:
	 a) It is marked as Finished in the type
	 b) It is marked as InProgress in the type
	 c) It is on the runList (the runList avoids issues with caching delay)
	*/

	// don't look at finished sosreports, ever
	if sosreport.Status.Finished == true {
		return ctrl.Result{}, nil
	}

	// initialize maps to avoid assignment to entry in nil map
	r.init()
	// before we run this, read some configuration from configmap
	r.setGlobalSosreportReconcilerConfiguration(sosreport, req)
	if IS_DEVELOPER_MODE {
		r.setDevelopmentSosreportReconcilerConfiguration(sosreport, req)
	}

	// a sosreport is not yet running if its not on the runList
	// and if its status.inProgress is false
	_, inRunlist := r.runList[sosreport.UID]
	if !inRunlist && sosreport.Status.InProgress == false {
		log.V(INFO).Info("Starting sosreport jobs")
		var err error
		r.runList[sosreport.UID] = struct{}{}

		// only schedule sosreports here
		sosreport.Status.InProgress, err = r.scheduleSosreportJobs(sosreport, req)
		if err != nil {
			log.Error(err, "unable to schedule sosreport jobs")
			return ctrl.Result{}, err
		}
		log.V(DEBUG).Info("Updating sosreport status", "sosreport.Status.InProgress", sosreport.Status.InProgress)
		r.updateStatus(sosreport, req)
	} else {
		// synchronize job running cache - in case this sosreport controller was restarted
		err := r.synchronizeJobRunningCache(sosreport, req)
		if err != nil {
			log.Error(err, "Failed to synchronize job running cache")
		}
		// dequeue any jobs that are in running list and are done
		err = r.dequeueSosreportJobsDone(sosreport, req)
		if err != nil {
			log.Error(err, "Failed to determine sosreport done state")
		}
		// run sosreport jobs from the jobToRunList and move them to running list
		_, err = r.runSosreportJobs(sosreport, req)
		if err != nil {
			log.Error(err, "unable to run sosreport jobs")
			return ctrl.Result{}, err
		}

		// copy the annotation for running-list and to-run-list into the Status field
		r.synchronizeRunningStatus(sosreport, req)

		if r.isSosreportJobsDone(sosreport) {
			log.V(INFO).Info("Sosreport generation done")
			sosreport.Status.InProgress = false
			sosreport.Status.Finished = true
		}

		r.updateStatus(sosreport, req)
	}

	return ctrl.Result{}, nil
}

func (r *SosreportReconciler) synchronizeRunningStatus(s *supportv1alpha1.Sosreport, req ctrl.Request) {
	var jobRunningList []string
	var jobToRunList []string

	if annotationJson, ok := s.Annotations["job-running-list"]; ok {
		annotationData := make(map[string]struct{})
		if err := json.Unmarshal([]byte(annotationJson), &annotationData); err == nil {
			for k, _ := range annotationData {
				jobRunningList = append(jobRunningList, k)
			}
		}
	}
	if annotationJson, ok := s.Annotations["job-to-run-list"]; ok {
		annotationData := make(map[string]struct{})
		if err := json.Unmarshal([]byte(annotationJson), &annotationData); err == nil {
			for k, _ := range annotationData {
				jobToRunList = append(jobToRunList, k)
			}
		}
	}

	s.Status.CurrentlyRunningNodes = jobRunningList
	s.Status.OutstandingNodes = jobToRunList
}

/*
Initialize maps to avoid assignment to entry in nil map
*/
func (r *SosreportReconciler) init() {
	if r.runList == nil {
		r.runList = make(map[types.UID]struct{})
	}
	if r.jobToRunList == nil {
		r.jobToRunList = make(map[types.UID]map[string]struct{})
	}
	if r.jobRunningList == nil {
		r.jobRunningList = make(map[types.UID]map[string]struct{})
	}
}

/*
Trigger reconcile loop whenever the CRD is updated or associated
Record events for CRD "Sosreport"
*/
func (r *SosreportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// record events for Sosreport CRD
	r.recorder = mgr.GetEventRecorderFor("Sosreport")

	return ctrl.NewControllerManagedBy(mgr).
		For(&supportv1alpha1.Sosreport{}).
		Owns(&batchv1.Job{}).
		// Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

/*
This method reads custom configuration from a configmap that allows admins to overwrite PVC settings, log-level and concurrency
*/
func (r *SosreportReconciler) setGlobalSosreportReconcilerConfiguration(s *supportv1alpha1.Sosreport, req ctrl.Request) {
	sosreportConcurrency := DEFAULT_SOSREPORT_CONCURRENCY
	sosreportDebug := false
	pvcStorageClass := ""
	pvcCapacity := DEFAULT_PVC_SIZE

	cm, err := r.getSosreportConfigMap(DEVELOPMENT_CONFIG_MAP_NAME, s, req)
	if err == nil {
		sosreportDebugCm, ok := cm.Data["debug"]
		if ok {
			if sosreportDebugCm == "true" {
				sosreportDebug = true
			} else {
				sosreportDebug = false
			}
		}
	}
	cm, err = r.getSosreportConfigMap(GLOBAL_CONFIG_MAP_NAME, s, req)
	if err == nil {
		sosreportConcurrencyCm, ok := cm.Data["concurrency"]
		if ok {
			if ic, err := strconv.Atoi(sosreportConcurrencyCm); err == nil {
				sosreportConcurrency = ic
			} else {
				log.V(INFO).Info("Cannot parse concurrency", "concurrency", sosreportConcurrencyCm)
			}
		}
		pvcStorageClassCm, ok := cm.Data["pvc-storage-class"]
		if ok {
			pvcStorageClass = pvcStorageClassCm
		}
		pvcCapacityCm, ok := cm.Data["pvc-capacity"]
		if ok {
			pvcCapacity = pvcCapacityCm
		}
	}
	log.V(DEBUG).Info("Setting loglevel to", "sosreportDebug", sosreportDebug)
	if sosreportDebug {
		r.DynamicLogLevel.MinLevel = zapcore.DebugLevel
	} else {
		r.DynamicLogLevel.MinLevel = zapcore.InfoLevel
	}
	log.V(DEBUG).Info("Using concurrency", "concurrency", sosreportConcurrency)
	r.sosreportConcurrency = sosreportConcurrency
	log.V(DEBUG).Info("PVC storage class", "pvcStorageClass", pvcStorageClass)
	r.pvcStorageClass = pvcStorageClass
	log.V(DEBUG).Info("PVC capacity", "pvcCapacity", pvcCapacity)
	r.pvcCapacity = pvcCapacity
}

/*
This method reads custom configuration from a configmap that allows admins to overwrite the sosreport generation
image as well as the sosreport command and image pull policy (developer settings)
*/
func (r *SosreportReconciler) setDevelopmentSosreportReconcilerConfiguration(s *supportv1alpha1.Sosreport, req ctrl.Request) {
	sosreportImage := DEFAULT_IMAGE_NAME
	sosreportCommand := DEFAULT_SOSREPORT_COMMAND
	imagePullPolicy := DEFAULT_IMAGE_PULL_POLICY

	cm, err := r.getSosreportConfigMap(DEVELOPMENT_CONFIG_MAP_NAME, s, req)
	if err == nil {
		sosreportImageCm, ok := cm.Data["sosreport-image"]
		if ok {
			sosreportImage = sosreportImageCm
		}
		sosreportCommandCm, ok := cm.Data["sosreport-command"]
		if ok {
			sosreportCommand = sosreportCommandCm
		}
		imagePullPolicyCm, ok := cm.Data["image-pull-policy"]
		if ok {
			if imagePullPolicyCm == string(corev1.PullAlways) ||
				imagePullPolicyCm == string(corev1.PullNever) ||
				imagePullPolicyCm == string(corev1.PullIfNotPresent) {
				imagePullPolicy = imagePullPolicyCm
			}
		}
	}
	log.V(DEBUG).Info("Using sosreport-image", "sosreport-image", sosreportImage)
	r.imageName = sosreportImage
	log.V(DEBUG).Info("Using sosreport-command", "sosreport-command", sosreportCommand)
	r.sosreportCommand = sosreportCommand
	log.V(DEBUG).Info("ImagePullPolicy", "imagePullPolicy", imagePullPolicy)
	r.imagePullPolicy = imagePullPolicy
}

/*
This method reads custom configuration from a configmap and secret and populates a map containing configuration items
*/
func (r *SosreportReconciler) getEnvConfigurationFromConfigMapAndSecret(s *supportv1alpha1.Sosreport, req ctrl.Request) map[string]string {
	keyMapUploadCm := map[string]string{
		"upload-method": "UPLOAD_METHOD",
		"case-number":   "CASE_NUMBER",
		"obfuscate":     "OBFUSCATE",
		"nfs-share":     "NFS_SHARE",
		"nfs-options":   "NFS_OPTIONS",
		"ftp-server":    "FTP_SERVER",
	}
	keyMapDevelopmentCm := map[string]string{
		"simulation-mode": "SIMULATION_MODE",
		"debug":           "DEBUG",
	}
	keyMapSecret := map[string]string{
		"username": "USERNAME",
		"password": "PASSWORD",
	}

	configurationMap := make(map[string]string)

	cmg, err := r.getSosreportConfigMap(UPLOAD_CONFIG_MAP_NAME, s, req)
	if err == nil {
		for k, v := range cmg.Data {
			// username and password shall be provided by secret
			if envK, ok := keyMapUploadCm[k]; ok {
				configurationMap[envK] = v
			}
		}
	}

	if IS_DEVELOPER_MODE {
		cmu, err := r.getSosreportConfigMap(DEVELOPMENT_CONFIG_MAP_NAME, s, req)
		if err == nil {
			for k, v := range cmu.Data {
				// username and password shall be provided by secret
				if envK, ok := keyMapDevelopmentCm[k]; ok {
					configurationMap[envK] = v
				}
			}
		}
	}
	secret, err := r.getSosreportSecret(s, req)
	if err == nil {
		for k, v := range secret.Data {
			// username and password shall be provided by secret
			if envK, ok := keyMapSecret[k]; ok {
				// remove newlines - found ending newlines while testing
				configurationMap[envK] = strings.Trim(string(v), "\n")
			}
		}
	}
	return configurationMap
}

/*
This method retrieves the config map which is used for configuration overrides
*/
func (r *SosreportReconciler) getSosreportConfigMap(configMapName string, s *supportv1alpha1.Sosreport, req ctrl.Request) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	nn := types.NamespacedName{Name: configMapName, Namespace: req.Namespace}
	log.V(DEBUG).Info("Retrieving ConfigMap", "NamespacedName", nn)
	if err := r.Get(ctx, nn, cm); err != nil {
		// all of the ConfigMaps are optional, so this should not be logged for INFO
		log.V(DEBUG).Info("unable to get configuration configmap", "err", err)
		return nil, err
	}
	return cm, nil
}

/*
This method retrieves the secret which is used for sosreport attachment authentication
*/
func (r *SosreportReconciler) getSosreportSecret(s *supportv1alpha1.Sosreport, req ctrl.Request) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	nn := types.NamespacedName{Name: UPLOAD_SECRET_NAME, Namespace: req.Namespace}
	log.V(DEBUG).Info("Retrieving Secret", "NamespacedName", nn)
	if err := r.Get(ctx, nn, secret); err != nil {
		log.V(INFO).Info("unable to get authentication Secret", "err", err)
		return nil, err
	}
	return secret, nil
}

/*
Update the "Sosreport" CR
*/
func (r *SosreportReconciler) update(s *supportv1alpha1.Sosreport, req ctrl.Request) {
	// update Sosreport resource
	log.V(DEBUG).Info("Updating sosreport CR")
	if err := r.Update(ctx, s); err != nil {
		log.V(DEBUG).Info("unable to update Sosreport CR", "err", err)
	}
	// after every update of a sosreport, get its new representation from the API
	r.refreshSosreport(s, req)
}

/*
Update the "Sosreport" CR's status
*/
func (r *SosreportReconciler) updateStatus(s *supportv1alpha1.Sosreport, req ctrl.Request) {
	// update Sosreport resource status
	log.V(DEBUG).Info("Updating sosreport resource status")
	if err := r.Status().Update(ctx, s); err != nil {
		log.V(DEBUG).Info("unable to update Sosreport status", "err", err)
	}
	// after every update of a sosreport, get its new representation from the API
	r.refreshSosreport(s, req)
}

/*
Refresh the "Sosreport" CR
*/
func (r *SosreportReconciler) refreshSosreport(s *supportv1alpha1.Sosreport, req ctrl.Request) {
	sosreport := &supportv1alpha1.Sosreport{}
	if err := r.Get(ctx, req.NamespacedName, sosreport); err == nil {
		s = sosreport
	} else {
		log.V(INFO).Info("Unable to refresh the Sosreport CR", "err", err)
	}
}

/*
Get all jobs which belong to a specific sosreport
We identify these by listing all jobs in the same namespace.
Then, we match the sosreport's UID with the job's ownerReference.UID from the job's metadata.
If the 2 match, then the job belongs to this sosreport.
*/
func (r *SosreportReconciler) getSosreportJobs(s *supportv1alpha1.Sosreport, req ctrl.Request) (*batchv1.JobList, error) {
	allSosreportJobs := &batchv1.JobList{}
	controllerSosreportJobs := &batchv1.JobList{}
	if err := r.List(ctx, allSosreportJobs, client.InNamespace(req.Namespace)); err != nil {
		log.Error(err, "unable to list child Jobs for sosreport")
		return nil, err
	}
	for _, sosreportJob := range allSosreportJobs.Items {
		// https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html
		ownerReference := objectGetController(sosreportJob.ObjectMeta)
		// there may be other jobs in this namespace with no owner
		if ownerReference == nil {
			continue
		}
		log.V(DEBUG).Info("Inspecting sosreport job's owner",
			"Name", sosreportJob.Name,
			"ownerReference.Kind", ownerReference.Kind,
			"ownerReference.UID", ownerReference.UID,
			"sosreport.UID", s.UID)
		if ownerReference.Kind == "Sosreport" && ownerReference.UID == s.UID {
			log.V(DEBUG).Info("ownerReference matches sosreport", "Kind", ownerReference.Kind,
				"UID", ownerReference.UID)
			controllerSosreportJobs.Items = append(controllerSosreportJobs.Items, sosreportJob)
		}
	}
	return controllerSosreportJobs, nil
}

/*
Go through all jobs that belong to this sosreport and check if they are done
A job is done if isJobDone returns true. That happens if the job is either JobComplete or JobFailed
*/
func (r *SosreportReconciler) dequeueSosreportJobsDone(s *supportv1alpha1.Sosreport, req ctrl.Request) error {
	sosreportJobs, err := r.getSosreportJobs(s, req)
	if err != nil {
		log.Error(err, "Error in dequeueSosreportJobsDone")
		return err
	}
	for _, sosreportJob := range sosreportJobs.Items {
		log.V(DEBUG).Info("Inspecting sosreport job", "Name", sosreportJob.Name)
		if done, _ := isJobDone(sosreportJob); !done {
			log.V(DEBUG).Info("sosreport job is still running", "Name", sosreportJob.Name)
		} else {
			// delete from jobRunningList and report an event
			log.V(DEBUG).Info("Delete job from jobRunningList", "sosreportJob.Annotations[\"nodeName\"]", sosreportJob.Annotations["nodeName"], "r.jobRunningList[s.UID]", r.jobRunningList[s.UID])
			if _, ok := r.jobRunningList[s.UID][sosreportJob.Annotations["nodeName"]]; ok {
				r.recorder.Event(s,
					corev1.EventTypeNormal,
					"Sosreport finished",
					"Sosreport "+sosreportJob.Annotations["nodeName"]+" finished",
				)
				delete(r.jobRunningList[s.UID], sosreportJob.Annotations["nodeName"])
			}
		}
	}

	if j, err := json.Marshal(r.jobRunningList[s.UID]); err == nil {
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		if s.Annotations["job-running-list"] != string(j) {
			s.Annotations["job-running-list"] = string(j)
			r.update(s, req)
		}
	}

	// log as an event for the sosreport
	// TBD - individual logging per jobs - this is more complex as we should only log the event once
	// per job. In the meantime, simply create an event when all jobs are done
	// r.recorder.Event(s, corev1.EventTypeNormal, "Sosreports finished", "Sosreport batch finished")

	return nil
}

/*
This method gets the owner reference for a job if the job has an owner
Returns nil otherwise
*/
func objectGetController(o metav1.ObjectMeta) *metav1.OwnerReference {
	for _, ownerReference := range o.OwnerReferences {
		if *ownerReference.Controller == true {
			return &ownerReference
		}
	}
	return nil
}

/*
A job is done if isJobDone returns true. That happens if the job is either JobComplete or JobFailed
*/
func isJobDone(j batchv1.Job) (bool, batchv1.JobConditionType) {
	for _, c := range j.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) &&
			c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}
	return false, ""
}

/*
Determine if the sosreport CR tolerates a specific node
*/
func (r *SosreportReconciler) tolerates(s *supportv1alpha1.Sosreport, n corev1.Node) bool {
	for _, taint := range n.Spec.Taints {
		log.V(DEBUG).Info("Checking taint", "taint", taint)

		if len(s.Spec.Tolerations) == 0 {
			return false
		}

		for ti, toleration := range s.Spec.Tolerations {
			log.V(DEBUG).Info("Checking toleration", "toleration", toleration, "ti", ti)

			if toleration.ToleratesTaint(&taint) {
				// break the inner loop and check the next Taint
				break
			}
			// if we have gone through all Tolerations and none
			// matches this taint - return false
			if ti == len(s.Spec.Tolerations)-1 {
				return false
			}
		}
	}

	return true
}

/*
Schedule jobs for this sosreport on Nodes which match the NodeSelector.
*/
func (r *SosreportReconciler) scheduleSosreportJobs(s *supportv1alpha1.Sosreport, req ctrl.Request) (bool, error) {
	// implement loop through nodes that are matched by sosreport's NodeSelector
	nodeList := &corev1.NodeList{}
	log.V(DEBUG).Info("Using NodeSelector", "s.Spec.NodeSelector", s.Spec.NodeSelector)
	listOpts := []client.ListOption{
		client.MatchingLabels(s.Spec.NodeSelector),
	}
	if err := r.List(ctx, nodeList, listOpts...); err != nil {
		return false, err
	}
	if len(nodeList.Items) == 0 {
		log.V(INFO).Info("Failed to list eligible nodes",
			"Sosreport.Namespace", s.Namespace,
			"Sosreport.Name", s.Name,
		)
		return false, nil
	}

	nodeNameList := make(map[string]struct{})
	for _, node := range nodeList.Items {
		// exclude nodes with Taints which do not match Toleration
		if r.tolerates(s, node) {
			nodeName := node.Labels["kubernetes.io/hostname"]
			nodeNameList[nodeName] = struct{}{}
		} else {
			log.V(INFO).Info("Node is not tolerated by Sosreport, skipping", "node.Name", node.Name, "node.Spec.Taints", node.Spec.Taints, "s.Spec.Tolerations", s.Spec.Tolerations)
		}
	}

	if j, err := json.Marshal(nodeNameList); err == nil {
		// every sosreport can have its list of names to run on
		r.jobToRunList[s.UID] = nodeNameList
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		s.Annotations["job-to-run-list"] = string(j)
		r.update(s, req)
	}

	return true, nil
}

func (r *SosreportReconciler) isSosreportJobsDone(s *supportv1alpha1.Sosreport) bool {
	toRunList, ok1 := r.jobToRunList[s.UID]
	runningList, ok2 := r.jobRunningList[s.UID]
	// we are done if either list does not exist or if either list is empty
	isDone := (!ok1 || len(toRunList) == 0) &&
		(!ok2 || len(runningList) == 0)

	if isDone {
		r.recorder.Event(s,
			corev1.EventTypeNormal,
			"Sosreports finished",
			"All Sosreports finished",
		)
	}

	return isDone
}

/*
Synchronize annotation with cache - in case the sosreport operator is restarted
*/
func (r *SosreportReconciler) synchronizeJobRunningCache(s *supportv1alpha1.Sosreport, req ctrl.Request) error {
	// the sosreport operator might have been restarted in the middle of a sosreport run
	if _, inRunList := r.jobToRunList[s.UID]; !inRunList {
		log.V(DEBUG).Info("Current jobToRunList for this job does not exist. Trying to load jobToRunList from s.Annotations[\"job-to-run-list\"]")
		if annotationJson, annotationExists := s.Annotations["job-to-run-list"]; annotationExists {
			log.V(DEBUG).Info("Annotation exists. Updating r.jobToRunList[s.UID]")
			annotationData := make(map[string]struct{})
			if err := json.Unmarshal([]byte(annotationJson), &annotationData); err == nil {
				r.jobToRunList[s.UID] = annotationData
				log.V(DEBUG).Info("Value is", "r.jobToRunList[s.UID]", r.jobToRunList[s.UID])
			} else {
				log.Error(err, "Failed to unmarshal annotation", "s.Annotations[\"job-to-run-list\"]", annotationJson)
				return err
			}
		}
	}

	// the sosreport operator might have been restarted in the middle of a sosreport run
	if _, inRunList := r.jobRunningList[s.UID]; !inRunList {
		log.V(DEBUG).Info("Current jobRunningList for this job does not exist. Trying to load jobRunningList from s.Annotations[\"job-running-list\"]")
		if annotationJson, annotationExists := s.Annotations["job-running-list"]; annotationExists {
			log.V(DEBUG).Info("Annotation exists. Updating r.jobRunningList[s.UID]")
			annotationData := make(map[string]struct{})
			if err := json.Unmarshal([]byte(annotationJson), &annotationData); err == nil {
				r.jobRunningList[s.UID] = annotationData
				log.V(DEBUG).Info("Value is", "r.jobRunningList[s.UID]", r.jobRunningList[s.UID])
			} else {
				log.Error(err, "Failed to unmarshal annotation", "s.Annotations[\"job-running-list\"]", annotationJson)
				return err
			}
		}
	}

	return nil
}

/*
Run jobs for this sosreport - get jobs from the jobToRunList
*/
func (r *SosreportReconciler) runSosreportJobs(s *supportv1alpha1.Sosreport, req ctrl.Request) (bool, error) {
	// implement loop through nodes that are matched by sosreport's NodeSelector
	nodeList := r.jobToRunList[s.UID]
	// merge the ConfigMap and Secret and retrieve them as a map[string]string
	configurationMap := r.getEnvConfigurationFromConfigMapAndSecret(s, req)

	maxNewSosreports := r.sosreportConcurrency - len(r.jobRunningList[s.UID])
	log.V(DEBUG).Info("runSosreportJobs",
		"r.sosreportConcurrency", r.sosreportConcurrency,
		"len(r.jobRunningList[s.UID])", len(r.jobRunningList[s.UID]))
	i := 0
	var newRunningNodes []string
	for nodeName, _ := range nodeList {
		if i >= maxNewSosreports {
			break
		}

		// Get a sosreport on this node
		job, pvc, err := r.jobForSosreport(nodeName, configurationMap, s)
		if err != nil {
			log.Error(err, "Could not generate job", "nodeName", nodeName, "err", err)
			continue
		}
		// Create the pvc
		log.V(INFO).Info("Creating new PVC", "Job.Namespace", job.Namespace, "Job.Name", pvc.Name)
		err = r.Create(ctx, pvc)
		if err != nil {
			log.Error(err, "Failed to create new PVC", "Job.Namespace", job.Namespace, "Job.Name", pvc.Name)
			continue
		}

		// Create the job
		log.V(INFO).Info("Creating new job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
		err = r.Create(ctx, job)
		if err != nil {
			log.Error(err, "Failed to create new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
			continue
		}
		// log this job creation as an event for the sosreport
		r.recorder.Event(s, corev1.EventTypeNormal, "Sosreport job started", "Sosreport started on "+nodeName)

		// remember which job switched to running
		newRunningNodes = append(newRunningNodes, nodeName)

		//increase the counter
		i++
	}

	// move a running sosreport from jobToRunList to jobRunningList
	if r.jobRunningList[s.UID] == nil {
		r.jobRunningList[s.UID] = make(map[string]struct{})
	}
	for _, nodeName := range newRunningNodes {
		r.jobRunningList[s.UID][nodeName] = struct{}{}
		delete(r.jobToRunList[s.UID], nodeName)
	}

	// update the CR annotation
	doUpdate := false
	if j, err := json.Marshal(r.jobToRunList[s.UID]); err == nil {
		if s.Annotations["job-to-run-list"] != string(j) {
			s.Annotations["job-to-run-list"] = string(j)
			doUpdate = true
		}
	}
	if j, err := json.Marshal(r.jobRunningList[s.UID]); err == nil {
		if s.Annotations["job-running-list"] != string(j) {
			s.Annotations["job-running-list"] = string(j)
			doUpdate = true
		}
	}
	if doUpdate {
		r.update(s, req)
	}

	return true, nil
}

/*
Convert a map[string]string into []corev1.EnvVar
*/
func mapToEnvVarArr(configurationMap map[string]string) []corev1.EnvVar {
	var envArr []corev1.EnvVar
	for k, v := range configurationMap {
		envArr = append(envArr, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return envArr
}

/*
Return a single job
*/
func (r *SosreportReconciler) jobForSosreport(nodeName string, environmentMap map[string]string, s *supportv1alpha1.Sosreport) (*batchv1.Job, *corev1.PersistentVolumeClaim, error) {
	layout := "20060102150405"

	// fix https://github.com/andreaskaris/sosreport-operator/issues/21
	// only take the short hostname and cut off the shortName at 48 characters
	// also account for the pvc name overhead of 4 characters
	maxLen := 63 - 2 - len(s.Name) - len(layout) - 4
	shortName := strings.Split(nodeName, ".")[0]
	if len(shortName) > maxLen {
		shortName = shortName[:maxLen]
	}

	jobName := fmt.Sprintf("%s-%s-%s", s.Name, shortName, time.Now().Format(layout))
	pvcName := fmt.Sprintf("%s-pvc", jobName)
	labels := r.labelsForSosreportJob(s.Name)

	var storageClassName *string
	if r.pvcStorageClass != "" {
		storageClassName = &r.pvcStorageClass
	}
	pvc := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: storageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resourcev1.MustParse(r.pvcCapacity),
				},
			},
		},
	}
	pvc.Name = pvcName
	pvc.Namespace = s.Namespace
	pvc.Labels = labels

	// read job dynamically from template
	job, err := r.jobFromTemplate("sosreport.yaml")
	if err != nil {
		return nil, nil, err
	}
	// set this job's specific fields
	job.ObjectMeta = metav1.ObjectMeta{
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		Name:        jobName,
		Namespace:   s.Namespace,
	}
	job.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Labels: labels,
	}

	job.Spec.Template.Spec.Tolerations = s.Spec.Tolerations
	// This used to be:
	// job.Spec.Template.Spec.NodeName = nodeName
	// explanation for why this does not work with PVCs is here:
	// https://github.com/openebs/openebs/issues/2915#issuecomment-623135043
	// Using Affinity also will make Taints and Tolerations work
	job.Spec.Template.Spec.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							corev1.NodeSelectorRequirement{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values: []string{
									nodeName,
								},
							},
						},
					},
				},
			},
		},
	}
	// required for dequeuing from the run list
	job.Annotations["nodeName"] = nodeName

	pvcVolume := corev1.Volume{}
	pvcVolume.Name = pvc.Name
	pvcVolume.VolumeSource = corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: pvcName,
		},
	}
	job.Spec.Template.Spec.Volumes = append(
		job.Spec.Template.Spec.Volumes,
		pvcVolume,
	)

	job.Spec.Template.Spec.Containers[0].Image = r.imageName
	job.Spec.Template.Spec.Containers[0].Name = jobName
	job.Spec.Template.Spec.Containers[0].Command = strings.Split(r.sosreportCommand, " ")

	job.Spec.Template.Spec.Containers[0].Env = mapToEnvVarArr(environmentMap)

	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
		job.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      pvcName,
			MountPath: "/pv",
		},
	)

	if r.imagePullPolicy != "" {
		job.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullPolicy(r.imagePullPolicy)
	}

	// Set ownerReferences
	// Set Sosreport instance as the owner of this pvc
	ctrl.SetControllerReference(s, pvc, r.Scheme)
	// Set Sosreport instance as the owner of this job
	ctrl.SetControllerReference(s, job, r.Scheme)

	return job, pvc, nil
}

/*
Return the labels that shall be attached to a sosreport's job
*/
func (r *SosreportReconciler) labelsForSosreportJob(name string) map[string]string {
	return map[string]string{"app": "sosreport", "sosreport-cr": name}
}

func getTemplatesDir() (string, error) {
	// This should normally find a templates directory right in the directory where this application is running
	// in case of unit tests, we might find templates at "../templates", instead
	for _, d := range []string{"templates", "../templates"} {
		if _, err := os.Stat(d); err != nil {
			if os.IsNotExist(err) {
				// file does not exist
				log.V(INFO).Info("Dir '" + d + "' does not exist, skipping")
			} else {
				// other error
				log.V(INFO).Info("Other issue with dir '" + d + "': " + err.Error())
			}
			continue
		}
		log.V(DEBUG).Info("Base directory is: " + d)
		return d, nil
	}

	// return the default value
	return "", errors.New("Cannot find a valid base directory for templates")
}

/*
Dynamically read a job from a template in the templates/ subfolder
See https://github.com/kubernetes/client-go/issues/193
*/
func (r *SosreportReconciler) jobFromTemplate(templateName string) (*batchv1.Job, error) {
	templatesDir, err := getTemplatesDir()
	if err != nil {
		return nil, err
	}
	yamlBytes, err := ioutil.ReadFile(templatesDir + "/" + templateName)
	if err != nil {
		log.Error(err, "Could not open template file")
		return nil, err
	}
	jsonBytes, err := yaml.YAMLToJSON(yamlBytes)
	if err != nil {
		log.Error(err, "Could not convert from YAML to JSON")
		return nil, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode

	obj, _, err := decode(jsonBytes, nil, nil)
	if err != nil {
		log.Error(err, "Could not decode from template")
		return nil, err
	}

	job := obj.(*batchv1.Job)
	return job, nil
}
