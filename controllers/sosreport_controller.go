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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
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
	GLOBAL_CONFIG_MAP_NAME    = "sosreport-global-configuration"       // name of the global ConfigMap with overrides
	UPLOAD_CONFIG_MAP_NAME    = "sosreport-upload-configuration"       // name of the upload ConfigMap
	UPLOAD_SECRET_NAME        = "sosreport-upload-secret"              // name of the Secret for upload authentication
	DEFAULT_IMAGE_NAME        = "quay.io/akaris/sosreport-centos:main" // to point to final version of sosreport IMAGE
	DEFAULT_SOSREPORT_COMMAND = "bash /entrypoint.sh"                  // to point to the entrypoint
)

// SosreportReconciler reconciles a Sosreport object
type SosreportReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
	// the runlist takes care of race conditions due to caching delay
	// a sosreport on the runlist will not be run again
	runlist          map[types.UID]struct{}
	imageName        string // name of the soreport job's image
	sosreportCommand string // command to run for the sosreport image
}

var log logr.Logger
var ctx context.Context

// +kubebuilder:rbac:groups=support.openshift.io,resources=sosreports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=support.openshift.io,resources=sosreports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets/status,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events/status,verbs=get
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes/status,verbs=get

func (r *SosreportReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx = context.Background()
	log = r.Log.WithValues("sosreport", req.NamespacedName)

	log.Info("Reconciler loop triggered")

	// retrieve sosreport CR to be reconciliated
	sosreport := &supportv1alpha1.Sosreport{}
	if err := r.Get(ctx, req.NamespacedName, sosreport); err != nil {
		log.Info("Failed to get Sosreport custom resource - was it deleted?")
		//log.Error(err, "Failed to get Sosreport custom resource - was it deleted?")
		// return ctrl.Result{}, err
		return ctrl.Result{}, nil
	}

	/*
	 A sosreport will not be run if:
	 a) It is marked as Finished in the type
	 b) It is marked as InProgress in the type
	 c) It is on the runlist (the runlist avoids issues with caching delay)
	*/

	// don't look at finished sosreports, ever
	if sosreport.Status.Finished == true {
		return ctrl.Result{}, nil
	}

	// a sosreport is not yet running if its not on the runlist
	// and if its status.inProgress is false
	_, inRunlist := r.runlist[sosreport.UID]
	if !inRunlist && sosreport.Status.InProgress == false {
		log.Info("Starting sosreport jobs")
		var err error
		if r.runlist == nil {
			r.runlist = make(map[types.UID]struct{})
		}
		r.runlist[sosreport.UID] = struct{}{}

		// before we run this, read some configuration from configmap
		r.updateSosreportImageNameAndCommand(sosreport, req)

		sosreport.Status.InProgress, err = r.runSosreportJobs(sosreport, req)
		if err != nil {
			log.Error(err, "unable to run sosreport jobs")
			return ctrl.Result{}, err
		}
		log.Info("Updating sosreport status", "sosreport.Status.InProgress", sosreport.Status.InProgress)
		r.updateStatus(sosreport)
	} else {
		done, err := r.sosreportJobsDone(sosreport, req)
		if err != nil {
			log.Error(err, "Failed to determine sosreport done state")
		}
		if done {
			log.Info("Sosreport generation done")
			sosreport.Status.InProgress = false
			sosreport.Status.Finished = true
			r.updateStatus(sosreport)
		}
	}

	return ctrl.Result{}, nil
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
		Complete(r)
}

/*
This method reads custom configuration from a configmap that allows admins to overwrite the sosreport generation
image as well as the sosreport command
*/
func (r *SosreportReconciler) updateSosreportImageNameAndCommand(s *supportv1alpha1.Sosreport, req ctrl.Request) {
	sosreportImage := DEFAULT_IMAGE_NAME
	sosreportCommand := DEFAULT_SOSREPORT_COMMAND
	cm, err := r.getSosreportConfigMap(GLOBAL_CONFIG_MAP_NAME, s, req)
	if err == nil {
		sosreportImageCm, ok := cm.Data["sosreport-image"]
		if ok {
			sosreportImage = sosreportImageCm
		}
		sosreportCommandCm, ok := cm.Data["sosreport-command"]
		if ok {
			sosreportCommand = sosreportCommandCm
		}
	}
	log.Info("Using sosreport-image", "sosreport-image", sosreportImage)
	r.imageName = sosreportImage
	log.Info("Using sosreport-command", "sosreport-command", sosreportCommand)
	r.sosreportCommand = sosreportCommand
}

/*
This method reads custom configuration from a configmap and secret and populates a map containing configuration items
*/
func (r *SosreportReconciler) getEnvConfigurationFromConfigMapAndSecret(s *supportv1alpha1.Sosreport, req ctrl.Request) map[string]string {
	keyMapUploadCm := map[string]string{
		"case-number":      "CASE_NUMBER",
		"upload-sosreport": "UPLOAD_SOSREPORT",
		"obfuscate":        "OBFUSCATE",
	}
	keyMapGlobalCm := map[string]string{
		"simulation-mode": "SIMULATION_MODE",
		"debug":           "DEBUG",
	}
	keyMapSecret := map[string]string{
		"username": "RH_USERNAME",
		"password": "RH_PASSWORD",
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
	cmu, err := r.getSosreportConfigMap(GLOBAL_CONFIG_MAP_NAME, s, req)
	if err == nil {
		for k, v := range cmu.Data {
			// username and password shall be provided by secret
			if envK, ok := keyMapGlobalCm[k]; ok {
				configurationMap[envK] = v
			}
		}
	}
	secret, err := r.getSosreportSecret(s, req)
	if err == nil {
		for k, v := range secret.Data {
			// username and password shall be provided by secret
			if envK, ok := keyMapSecret[k]; ok {
				configurationMap[envK] = string(v)
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
	log.Info("Retrieving ConfigMap", "NamespacedName", nn)
	if err := r.Get(ctx, nn, cm); err != nil {
		log.Info("unable to get configuration configmap", "err", err)
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
	log.Info("Retrieving Secret", "NamespacedName", nn)
	if err := r.Get(ctx, nn, secret); err != nil {
		log.Info("unable to get authentication Secret", "err", err)
		return nil, err
	}
	return secret, nil
}

/*
Update the "Sosreport" CR's status
*/
func (r *SosreportReconciler) updateStatus(s *supportv1alpha1.Sosreport) {
	// update Sosreport resource status
	log.Info("Updating sosreport resource status")
	if err := r.Status().Update(ctx, s); err != nil {
		log.Info("unable to update Sosreport status", "err", err)
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
		ownerReference := jobGetController(sosreportJob)
		// there may be other jobs in this namespace with no owner
		if ownerReference == nil {
			continue
		}
		log.Info("Inspecting sosreport job's owner",
			"Name", sosreportJob.Name,
			"ownerReference.Kind", ownerReference.Kind,
			"ownerReference.UID", ownerReference.UID,
			"sosreport.UID", s.UID)
		if ownerReference.Kind == "Sosreport" && ownerReference.UID == s.UID {
			log.Info("ownerReference matches sosreport", "Kind", ownerReference.Kind,
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
func (r *SosreportReconciler) sosreportJobsDone(s *supportv1alpha1.Sosreport, req ctrl.Request) (bool, error) {
	sosreportJobs, err := r.getSosreportJobs(s, req)
	if err != nil {
		log.Info("Error in sosreportJobsDone")
		return false, err
	}
	if len(sosreportJobs.Items) == 0 {
		log.Info("Sosreport list is empty. Not considering this as done.")
		return false, nil
	}
	for _, sosreportJob := range sosreportJobs.Items {
		log.Info("Inspecting sosreport job", "Name", sosreportJob.Name)
		if done, _ := isJobDone(sosreportJob); !done {
			log.Info("sosreport job is still running", "Name", sosreportJob.Name)
			return false, nil
		}
	}
	// log as an event for the sosreport
	// TBD - individual logging per jobs - this is more complex as we should only log the event once
	// per job. In the meantime, simply create an event when all jobs are done
	r.recorder.Event(s, corev1.EventTypeNormal, "Sosreports finished", "All Sosreports finished")

	return true, nil
}

/*
This method gets the owner reference for a job if the job has an owner
Returns nil otherwise
*/
func jobGetController(j batchv1.Job) *metav1.OwnerReference {
	for _, ownerReference := range j.OwnerReferences {
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
Run jobs for this sosreport on Nodes which match the NodeSelector.
*/
func (r *SosreportReconciler) runSosreportJobs(s *supportv1alpha1.Sosreport, req ctrl.Request) (bool, error) {
	// implement loop through nodes that are matched by sosreport's NodeSelector
	nodeList := &corev1.NodeList{}
	log.Info("Using NodeSelector", "s.Spec.NodeSelector", s.Spec.NodeSelector)
	listOpts := []client.ListOption{
		client.MatchingLabels(s.Spec.NodeSelector),
	}
	if err := r.List(ctx, nodeList, listOpts...); err != nil {
		return false, err
	}
	if len(nodeList.Items) == 0 {
		log.Info("Failed to list eligible nodes",
			"Sosreport.Namespace", s.Namespace,
			"Sosreport.Name", s.Name,
		)
		return false, nil
	}

	// merge the ConfigMap and Secret and retrieve them as a map[string]string
	configurationMap := r.getEnvConfigurationFromConfigMapAndSecret(s, req)

	for _, node := range nodeList.Items {
		nodeName := node.Name
		// Get a sosreport on this node
		job, err := r.jobForSosreport(nodeName, configurationMap, s)
		if err != nil {
			return false, err
		}
		// Create the job
		log.Info("Creating new job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
		err = r.Create(ctx, job)
		if err != nil {
			log.Error(err, "Failed to create new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
			return false, err
		}
		// log this job creation as an event for the sosreport
		r.recorder.Event(s, corev1.EventTypeNormal, "Sosreport job started", "Sosreport started on "+nodeName)
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
func (r *SosreportReconciler) jobForSosreport(nodeName string, environmentMap map[string]string, s *supportv1alpha1.Sosreport) (*batchv1.Job, error) {
	layout := "20060102150405"
	jobName := fmt.Sprintf("%s-%s-%s", s.Name, nodeName, time.Now().Format(layout))
	labels := r.labelsForSosreportJob(s.Name)

	// read job dynamically from template
	job, err := r.jobFromTemplate("sosreport.yaml")
	if err != nil {
		return nil, err
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
	job.Spec.Template.Spec.NodeName = nodeName
	job.Spec.Template.Spec.Containers[0].Image = r.imageName
	job.Spec.Template.Spec.Containers[0].Name = jobName
	job.Spec.Template.Spec.Containers[0].Command = strings.Split(r.sosreportCommand, " ")

	job.Spec.Template.Spec.Containers[0].Env = mapToEnvVarArr(environmentMap)

	// Set Sosreport instance as the owner of this job
	ctrl.SetControllerReference(s, job, r.Scheme)

	return job, nil
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
				log.Info("Dir '" + d + "' does not exist, skipping")
			} else {
				// other error
				log.Info("Other issue with dir '" + d + "': " + err.Error())
			}
			continue
		}
		log.Info("Base directory is: " + d)
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
