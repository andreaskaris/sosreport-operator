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
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//"sigs.k8s.io/controller-runtime/pkg/handler"
	//"sigs.k8s.io/controller-runtime/pkg/reconcile"
	//"sigs.k8s.io/controller-runtime/pkg/source"
	"k8s.io/client-go/tools/record"

	supportv1alpha1 "github.com/andreaskaris/sosreport-operator/api/v1alpha1"
)

// SosreportReconciler reconciles a Sosreport object
type SosreportReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
	// the runlist takes care of race conditions due to caching delay
	// a sosreport on the runlist will not be run again
	runlist map[types.UID]struct{}
}

var log logr.Logger
var ctx context.Context

// +kubebuilder:rbac:groups=support.openshift.io,resources=sosreports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=support.openshift.io,resources=sosreports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=support.openshift.io,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=support.openshift.io,resources=jobs/status,verbs=get

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
		sosreport.Status.InProgress, err = r.runSosreportJobs(sosreport)
		if err != nil {
			log.Error(err, "unable to run sosreport jobs")
			return ctrl.Result{}, err
		}
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

// trigger reconcile loop whenever the CRD is updated or associated
func (r *SosreportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("Sosreport")

	return ctrl.NewControllerManagedBy(mgr).
		For(&supportv1alpha1.Sosreport{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *SosreportReconciler) updateStatus(s *supportv1alpha1.Sosreport) {
	// update Sosreport resource status
	log.Info("Updating sosreport resource status")
	if err := r.Status().Update(ctx, s); err != nil {
		log.Info("unable to update Sosreport status", "err", err)
	}
}

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
	return true, nil
}

func jobGetController(j batchv1.Job) *metav1.OwnerReference {
	for _, ownerReference := range j.OwnerReferences {
		if *ownerReference.Controller == true {
			return &ownerReference
		}
	}
	return nil
}

func isJobDone(j batchv1.Job) (bool, batchv1.JobConditionType) {
	for _, c := range j.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) &&
			c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}
	return false, ""
}

func (r *SosreportReconciler) runSosreportJobs(s *supportv1alpha1.Sosreport) (bool, error) {
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
	for _, node := range nodeList.Items {
		nodeName := node.Name
		job, err := r.jobForSosreport(nodeName, s)
		if err != nil {
			return false, err
		}
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

func (r *SosreportReconciler) jobForSosreport(nodeName string, s *supportv1alpha1.Sosreport) (*batchv1.Job, error) {
	layout := "20060102150405"
	jobName := fmt.Sprintf("%s-%s-%s", s.Name, nodeName, time.Now().Format(layout))
	labels := r.labelsForSosreport(s.Name)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        jobName,
			Namespace:   s.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					NodeName:      nodeName,
					RestartPolicy: "Never",
					Containers: []corev1.Container{{
						Image:   "alpine",
						Name:    jobName,
						Command: []string{"sleep", strconv.Itoa(rand.Intn(60))},
					}},
				},
			},
		},
	}

	// Set Sosreport instance as the owner of this job
	ctrl.SetControllerReference(s, job, r.Scheme)

	return job, nil
}

func (r *SosreportReconciler) labelsForSosreport(name string) map[string]string {
	return map[string]string{"app": "sosreport", "sosreport-cr": name}
}
