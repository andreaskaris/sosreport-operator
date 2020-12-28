/*
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
	"context"
	"fmt"
	//"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	supportv1alpha1 "github.com/andreaskaris/sosreport-operator/api/v1alpha1"
)

var _ = Describe("Sosreport controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		SosreportName          = "test-sosreport"
		SosreportNamespace     = "default"
		GLOBAL_CONFIG_MAP_NAME = "sosreport-global-configuration"
		JobName                = "test-job"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("Creating a Sosreport", func() {
		It("Should successfully run sosreport jobs when a new Sosreport is created", func() {
			ctx := context.Background()

			By("Checking if nodes must be created")
			nodeList := &corev1.NodeList{}
			listOpts := []client.ListOption{}
			if err := k8sClient.List(ctx, nodeList, listOpts...); err != nil || len(nodeList.Items) == 0 {
				By("Creating a master node")
				masterNode := &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "master-0",
						Labels: map[string]string{
							"node-role.kubernetes.io/master": "",
						},
					},
				}
				Expect(k8sClient.Create(ctx, masterNode)).Should(Succeed())
			}

			By("By creating a new ConfigMap")

			// Create a new ConfigMap first
			cmg := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GLOBAL_CONFIG_MAP_NAME,
					Namespace: SosreportNamespace,
				},
				Data: map[string]string{
					"sosreport-image":   "kind:5000/sosreport-centos:latest",
					"sosreport-command": "bash -x /entrypoint.sh",
					"simulation-mode":   "true",
					"debug":             "true",
				},
			}
			Expect(k8sClient.Create(ctx, cmg)).Should(Succeed())

			// Make sure that the ConfigMap really gets created
			configMapLookupKey := types.NamespacedName{Name: GLOBAL_CONFIG_MAP_NAME, Namespace: SosreportNamespace}
			createdConfigMap := &corev1.ConfigMap{}

			// We'll need to retry getting this newly created ConfigMap, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, configMapLookupKey, createdConfigMap)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("By creating a new Sosreport")
			// Create a new Sosreport
			sosreport := &supportv1alpha1.Sosreport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "support.openshift.io/v1alpha1",
					Kind:       "Sosreport",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      SosreportName,
					Namespace: SosreportNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, sosreport)).Should(Succeed())

			// wait until the Sosreport is created
			sosreportLookupKey := types.NamespacedName{Name: SosreportName, Namespace: SosreportNamespace}
			createdSosreport := &supportv1alpha1.Sosreport{}

			// We'll need to retry getting this newly created Sosreport, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sosreportLookupKey, createdSosreport)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("By making sure that the Sosreport switches to InProgress")
			// We'll need to retry getting this newly created Sosreport, given that creation may not immediately happen.
			Eventually(func() bool {
				// We need to retrieve a new copy of the Sosreport object at each try
				err := k8sClient.Get(ctx, sosreportLookupKey, createdSosreport)
				if err != nil {
					return false
				}
				// fmt.Fprintf(GinkgoWriter, "Test: %v\n", createdSosreport.Status.InProgress)
				return createdSosreport.Status.InProgress
			}, timeout, interval).Should(BeTrue())

			By("Retrieving a list of all jobs that belong to this sosreport")
			allSosreportJobs := &batchv1.JobList{}
			controllerSosreportJobs := &batchv1.JobList{}

			err := k8sClient.List(ctx, allSosreportJobs, client.InNamespace(SosreportNamespace))
			Expect(err).ShouldNot(HaveOccurred())

			for _, sosreportJob := range allSosreportJobs.Items {
				// https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html
				ownerReference := jobGetController(sosreportJob)
				// there may be other jobs in this namespace with no owner
				if ownerReference == nil {
					continue
				}
				//log.Info("Inspecting sosreport job's owner",
				//	"Name", sosreportJob.Name,
				//	"ownerReference.Kind", ownerReference.Kind,
				//	"ownerReference.UID", ownerReference.UID,
				//	"sosreport.UID", s.UID)
				if ownerReference.Kind == "Sosreport" && ownerReference.UID == createdSosreport.UID {
					//log.Info("ownerReference matches sosreport", "Kind", ownerReference.Kind,
					//	"UID", ownerReference.UID)
					controllerSosreportJobs.Items = append(controllerSosreportJobs.Items, sosreportJob)
				}
			}

			By("Setting all jobs to done")
			for _, job := range controllerSosreportJobs.Items {
				job.Status.Conditions = append(job.Status.Conditions,
					batchv1.JobCondition{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					})
				fmt.Fprintf(GinkgoWriter, "Updating job: %v\n", job.Name)
				err := k8sClient.Status().Update(ctx, &job)
				Expect(err).ShouldNot(HaveOccurred())
			}

			By("By making sure that the Sosreport switches to Finished")
			Eventually(func() bool {
				// We need to retrieve a new copy of the Sosreport object at each try
				err := k8sClient.Get(ctx, sosreportLookupKey, createdSosreport)
				if err != nil {
					return false
				}
				// fmt.Fprintf(GinkgoWriter, "Test: %v\n", createdSosreport.Status.Finished)
				return createdSosreport.Status.Finished
			}, timeout, interval).Should(BeTrue())

		})
	})

})

/*
	After writing all this code, you can run `go test ./...` in your `controllers/` directory again to run your new test!
*/
