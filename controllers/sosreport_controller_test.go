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
	"os"
	//"reflect"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	errorsv1 "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	supportv1alpha1 "github.com/andreaskaris/sosreport-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Sosreport controller", func() {

	// Define utility constants for object names and testing TIMEOUTs/DURATIONs and INTERVALs.
	const (
		SOSREPORT_NAME         = "test-sosreport"
		SOSREPORT_NAMESPACE    = "sosreport-test"
		GLOBAL_CONFIG_MAP_NAME = "sosreport-global-configuration"
		JOB_NAME               = "test-job"

		TIMEOUT                      = time.Second * 10
		USE_EXISTING_CLUSTER_TIMEOUT = time.Second * 600
		DURATION                     = time.Second * 10
		INTERVAL                     = time.Millisecond * 250
	)

	Context("Creating a Sosreport", func() {
		It("Should successfully run sosreport jobs when a new Sosreport is created", func() {
			ctx := context.Background()
			var err error
			sosreportImage := os.Getenv("SOSREPORT_IMG")
			useExistingCluster := false
			if os.Getenv("USE_EXISTING_CLUSTER") == "true" {
				useExistingCluster = true
			}
			fmt.Fprintf(GinkgoWriter, "sosreportImage: '%v'\n", sosreportImage)
			fmt.Fprintf(GinkgoWriter, "useExistingCluster: '%v'\n", useExistingCluster)
			isOpenShift := false

			By("Sleeping for a bit so that the cache can initialize")
			time.Sleep(5000 * time.Millisecond)

			if useExistingCluster {
				By("Determining what type of cluster this is")
				// Using a unstructured object.
				u := &unstructured.UnstructuredList{}
				u.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "config.openshift.io",
					Kind:    "ClusterVersion",
					Version: "v1",
				})
				err = k8sClient.List(context.Background(), u)
				// if we find a resource ClusterVersion, then this is OCP
				if err == nil {
					isOpenShift = true
				}
			}
			fmt.Fprintf(GinkgoWriter, "isOpenShift: '%v'\n", isOpenShift)

			By("Listing existing nodes")
			nodeList := &corev1.NodeList{}
			listOpts := []client.ListOption{}
			err = k8sClient.List(ctx, nodeList, listOpts...)
			Expect(err).ShouldNot(HaveOccurred())

			if useExistingCluster {
				By("Making sure that nodeList is not empty when connecting to a real cluster")
				Expect(len(nodeList.Items)).NotTo(Equal(0))
				// fmt.Fprintf(GinkgoWriter, "%v: %v, %v: %v\n", "nodeList.Items", nodeList.Items, "len(nodeList.Items)", len(nodeList.Items))
			}

			if len(nodeList.Items) == 0 {
				By("Creating nodes")
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
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							corev1.Taint{
								Key:    "node-role.kubernetes.io/master",
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, masterNode)).Should(Succeed())

				By("Creating a worker node")
				workerNodeOne := &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-0",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				}
				Expect(k8sClient.Create(ctx, workerNodeOne)).Should(Succeed())

				By("Creating another worker node")
				workerNodeTwo := &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				}
				Expect(k8sClient.Create(ctx, workerNodeTwo)).Should(Succeed())

				By("Creating a worker node which shall be skipped")
				workerNodeThree := &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-2",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							corev1.Taint{
								Key:    "node-role.kubernetes.io/do-not-schedule",
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, workerNodeThree)).Should(Succeed())
			} // end if len(nodeList.Items) == 0

			if useExistingCluster {
				By("Making sure that a valid sosreportImage is set when useExistingCluster = true")
				Expect(sosreportImage).NotTo(Equal(""))
			}

			By("Checking if namespace" + SOSREPORT_NAMESPACE + " already exists")
			namespace := &corev1.Namespace{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: SOSREPORT_NAMESPACE}, namespace)
			if err != nil {
				if statusError, ok := err.(*errorsv1.StatusError); !ok ||
					statusError.Status().Reason != metav1.StatusReasonNotFound {
					Expect(err).ShouldNot(HaveOccurred())
				}

				By("Creating namespace" + SOSREPORT_NAMESPACE)
				newNamespace := &corev1.Namespace{}
				newNamespace.Name = SOSREPORT_NAMESPACE
				Expect(k8sClient.Create(ctx, newNamespace)).Should(Succeed())

				// Make sure that the Namespace really gets created
				createdNamespace := &corev1.Namespace{}
				// We'll need to retry getting this newly created Namespace, given that creation may not immediately happen.
				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKey{Name: SOSREPORT_NAMESPACE}, createdNamespace)
					if err != nil {
						return false
					}
					return true
				}, TIMEOUT, INTERVAL).Should(BeTrue())
			}

			if isOpenShift {
				By("Getting Privileged SCC ClusterRoleBinding")
				crb := &rbacv1.ClusterRoleBinding{}

				err = k8sClient.Get(
					ctx,
					client.ObjectKey{Name: "system:openshift:scc:privileged"},
					crb,
				)
				Expect(err).ShouldNot(HaveOccurred())

				By("Setting Privileged SCC for the namespace")
				crbFound := false
				for _, s := range crb.Subjects {
					if s.Kind == "ServiceAccount" &&
						s.Name == "default" &&
						s.Namespace == SOSREPORT_NAMESPACE {
						crbFound = true
						break
					}
				}
				if !crbFound {
					crb.Subjects = append(
						crb.Subjects,
						rbacv1.Subject{
							Kind:      "ServiceAccount",
							Name:      "default",
							Namespace: SOSREPORT_NAMESPACE,
						},
					)
					Expect(k8sClient.Update(ctx, crb)).Should(Succeed())
				}
			}

			By("Determining if a global ConfigMap already exists")
			cmg := &corev1.ConfigMap{}
			namespacedNameCm := types.NamespacedName{Name: GLOBAL_CONFIG_MAP_NAME, Namespace: SOSREPORT_NAMESPACE}
			err = k8sClient.Get(ctx, namespacedNameCm, cmg)
			statusError, ok := err.(*errorsv1.StatusError)
			if !ok || statusError.Status().Reason != metav1.StatusReasonNotFound {
				Expect(err).ShouldNot(HaveOccurred())
			}

			// Create a new ConfigMap first
			cmg.TypeMeta.APIVersion = "v1"
			cmg.TypeMeta.Kind = "ConfigMap"
			cmg.ObjectMeta.Name = GLOBAL_CONFIG_MAP_NAME
			cmg.ObjectMeta.Namespace = SOSREPORT_NAMESPACE
			if cmg.Data == nil {
				cmg.Data = make(map[string]string)
			}
			cmg.Data["sosreport-image"] = sosreportImage
			cmg.Data["sosreport-command"] = "bash -x /scripts/entrypoint.sh"
			if isOpenShift {
				cmg.Data["simulation-mode"] = "false"
			}
			cmg.Data["image-pull-policy"] = "Always"
			if ok && statusError.Status().Reason == metav1.StatusReasonNotFound {
				By("By creating a new global ConfigMap")
				Expect(k8sClient.Create(ctx, cmg)).Should(Succeed())
			} else {
				By("By updating the existing ConfigMap")
				Expect(k8sClient.Update(ctx, cmg)).Should(Succeed())
			}

			// Make sure that the ConfigMap really gets created
			configMapLookupKey := types.NamespacedName{Name: GLOBAL_CONFIG_MAP_NAME, Namespace: SOSREPORT_NAMESPACE}
			createdConfigMap := &corev1.ConfigMap{}
			// We'll need to retry getting this newly created ConfigMap, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, configMapLookupKey, createdConfigMap)
				if err != nil {
					return false
				}
				return true
			}, TIMEOUT, INTERVAL).Should(BeTrue())

			By("By checking if a sosreport already exists")
			namespacedNameSosreport := types.NamespacedName{Name: SOSREPORT_NAME, Namespace: SOSREPORT_NAMESPACE}
			sosreport := &supportv1alpha1.Sosreport{}
			err = k8sClient.Get(ctx, namespacedNameSosreport, sosreport)
			sosreportExists := false
			if err == nil {
				// sosreport exists if no err returned
				sosreportExists = true
			} else {
				statusError, ok := err.(*errorsv1.StatusError)
				if !ok || statusError.Status().Reason != metav1.StatusReasonNotFound {
					// either this is not a status error or the error is not not found
					// throw err
					Expect(err).ShouldNot(HaveOccurred())
				} else {
					// this is a StatusError error and it is StatusReasonNotFound
					sosreportExists = false
				}
			}

			if sosreportExists {
				By("Deleting the existing Sosreport")
				sosreport.ObjectMeta = metav1.ObjectMeta{
					Namespace: SOSREPORT_NAMESPACE,
					Name:      SOSREPORT_NAME,
				}
				err = k8sClient.Delete(ctx, sosreport)
				Expect(err).ShouldNot(HaveOccurred())

				// We'll need to retry getting this deleted Sosreport, given that creation may not immediately happen.
				Eventually(func() bool {
					err := k8sClient.Get(ctx, namespacedNameSosreport, sosreport)
					if err == nil {
						return false
					}
					return true
				}, TIMEOUT, INTERVAL).Should(BeTrue())
			}

			By("By creating a new Sosreport")
			// Create a new Sosreport
			sosreport = &supportv1alpha1.Sosreport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "support.openshift.io/v1alpha1",
					Kind:       "Sosreport",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      SOSREPORT_NAME,
					Namespace: SOSREPORT_NAMESPACE,
				},
				Spec: supportv1alpha1.SosreportSpec{
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
					Tolerations: []corev1.Toleration{
						corev1.Toleration{
							Key:    "node-role.kubernetes.io/master",
							Effect: corev1.TaintEffectNoSchedule,
						},
						corev1.Toleration{
							Key:    "node.kubernetes.io/not-ready",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sosreport)).Should(Succeed())

			createdSosreport := &supportv1alpha1.Sosreport{}

			// We'll need to retry getting this newly created Sosreport, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedNameSosreport, createdSosreport)
				if err != nil {
					return false
				}
				return true
			}, TIMEOUT, INTERVAL).Should(BeTrue())

			By("By making sure that the Sosreport switches to InProgress")
			// We'll need to retry getting this newly created Sosreport, given that creation may not immediately happen.
			Eventually(func() bool {
				// We need to retrieve a new copy of the Sosreport object at each try
				err := k8sClient.Get(ctx, namespacedNameSosreport, createdSosreport)
				if err != nil {
					return false
				}
				// fmt.Fprintf(GinkgoWriter, "Test: %v\n", createdSosreport.Status.InProgress)
				return createdSosreport.Status.InProgress
			}, TIMEOUT, INTERVAL).Should(BeTrue())

			By("By making sure that the Sosreport has a job in the job-to-run-list")
			// We'll need to retry getting this newly created Sosreport, given that creation may not immediately happen.
			Eventually(func() bool {
				// We need to retrieve a new copy of the Sosreport object at each try
				err := k8sClient.Get(ctx, namespacedNameSosreport, createdSosreport)
				if err != nil {
					return false
				}

				if _, ok = createdSosreport.Annotations["job-to-run-list"]; !ok {
					return false
				}

				fmt.Fprintf(GinkgoWriter, "createdSosreport.Annotations[\"job-to-run-list\"]: %v\n", createdSosreport.Annotations["job-to-run-list"])
				var jobToRunList map[string]struct{}
				err = json.Unmarshal(
					[]byte(createdSosreport.Annotations["job-to-run-list"]),
					&jobToRunList,
				)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "err %v", err)
					return false
				}
				if len(jobToRunList) > 0 {
					return true
				}
				return false
			}, TIMEOUT, INTERVAL).Should(BeTrue())

			By("By making sure that the Sosreport has a job in the job-running-list")
			// We'll need to retry getting this newly created Sosreport, given that creation may not immediately happen.

			Eventually(func() bool {
				// We need to retrieve a new copy of the Sosreport object at each try
				err := k8sClient.Get(ctx, namespacedNameSosreport, createdSosreport)
				if err != nil {
					return false
				}

				if _, ok = createdSosreport.Annotations["job-running-list"]; !ok {
					return false
				}

				fmt.Fprintf(GinkgoWriter, "createdSosreport.Annotations[\"job-running-list\"]: %v\n", createdSosreport.Annotations["job-running-list"])
				var jobRunningList map[string]struct{}
				err = json.Unmarshal(
					[]byte(createdSosreport.Annotations["job-running-list"]),
					&jobRunningList,
				)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "err %v", err)
					return false
				}
				if len(jobRunningList) > 0 {
					return true
				}
				return false
			}, TIMEOUT, INTERVAL).Should(BeTrue())

			By("Retrieving a list of all jobs that belong to this sosreport")
			allSosreportJobs := &batchv1.JobList{}
			controllerSosreportJobs := &batchv1.JobList{}

			err = k8sClient.List(ctx, allSosreportJobs, client.InNamespace(SOSREPORT_NAMESPACE))
			Expect(err).ShouldNot(HaveOccurred())

			for _, sosreportJob := range allSosreportJobs.Items {
				// https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html
				ownerReference := objectGetController(sosreportJob.ObjectMeta)
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

			if !useExistingCluster {
				By("Setting all jobs to done")
				for _, job := range controllerSosreportJobs.Items {
					job.Status.Conditions = append(job.Status.Conditions,
						batchv1.JobCondition{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						})
					// fmt.Fprintf(GinkgoWriter, "Updating job: %v\n", job.Name)
					err := k8sClient.Status().Update(ctx, &job)
					Expect(err).ShouldNot(HaveOccurred())
				}

				By("By making sure that the Sosreport has no job in the job-to-run-list")
				// We'll need to retry getting this newly created Sosreport, given that creation may not immediately happen.
				Eventually(func() bool {
					// We need to retrieve a new copy of the Sosreport object at each try
					err := k8sClient.Get(ctx, namespacedNameSosreport, createdSosreport)
					if err != nil {
						return false
					}
					// fmt.Fprintf(GinkgoWriter, "createdSosreport.Annotations[\"job-to-run-list\"]: %v\n", createdSosreport.Annotations["job-to-run-list"])
					return createdSosreport.Annotations["job-to-run-list"] == "{}"
				}, TIMEOUT, INTERVAL).Should(BeTrue())

				By("By making sure that the Sosreport has a job in the job-running-list")
				// We'll need to retry getting this newly created Sosreport, given that creation may not immediately happen.
				Eventually(func() bool {
					// We need to retrieve a new copy of the Sosreport object at each try
					err := k8sClient.Get(ctx, namespacedNameSosreport, createdSosreport)
					if err != nil {
						return false
					}
					// fmt.Fprintf(GinkgoWriter, "createdSosreport.Annotations[\"job-running-list\"]: %v\n", createdSosreport.Annotations["job-running-list"])
					return createdSosreport.Annotations["job-running-list"] == "{\"worker-1\":{}}" ||
						createdSosreport.Annotations["job-running-list"] == "{\"worker-0\":{}}"
				}, TIMEOUT, INTERVAL).Should(BeTrue())

				By("Retrieving a list of all jobs that belong to this sosreport")
				allSosreportJobs = &batchv1.JobList{}
				controllerSosreportJobs = &batchv1.JobList{}

				err = k8sClient.List(ctx, allSosreportJobs, client.InNamespace(SOSREPORT_NAMESPACE))
				Expect(err).ShouldNot(HaveOccurred())

				for _, sosreportJob := range allSosreportJobs.Items {
					// https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html
					ownerReference := objectGetController(sosreportJob.ObjectMeta)
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
					// fmt.Fprintf(GinkgoWriter, "Updating job: %v\n", job.Name)
					err := k8sClient.Status().Update(ctx, &job)
					Expect(err).ShouldNot(HaveOccurred())
				}
			} // if !useExistingCluster

			By("By making sure that the Sosreport switches to Finished")
			timeout := TIMEOUT
			if useExistingCluster {
				timeout = USE_EXISTING_CLUSTER_TIMEOUT
			}
			Eventually(func() bool {
				// We need to retrieve a new copy of the Sosreport object at each try
				err := k8sClient.Get(ctx, namespacedNameSosreport, createdSosreport)
				if err != nil {
					return false
				}
				// fmt.Fprintf(GinkgoWriter, "Test: %v\n", createdSosreport.Status.Finished)
				return createdSosreport.Status.Finished
			}, timeout, INTERVAL).Should(BeTrue())

			if useExistingCluster {
				By("Retrieving a list of all jobs that belong to this sosreport")
				allSosreportJobs = &batchv1.JobList{}
				controllerSosreportJobs = &batchv1.JobList{}

				err = k8sClient.List(ctx, allSosreportJobs, client.InNamespace(SOSREPORT_NAMESPACE))
				Expect(err).ShouldNot(HaveOccurred())

				for _, sosreportJob := range allSosreportJobs.Items {
					// https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html
					ownerReference := objectGetController(sosreportJob.ObjectMeta)
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

				By("Retrieving a list of all PVCs that belong to this sosreport")
				allSosreportPVCs := &corev1.PersistentVolumeClaimList{}
				controllerSosreportPVCs := &corev1.PersistentVolumeClaimList{}

				err = k8sClient.List(ctx, allSosreportPVCs, client.InNamespace(SOSREPORT_NAMESPACE))
				Expect(err).ShouldNot(HaveOccurred())

				for _, sosreportPVC := range allSosreportPVCs.Items {
					// https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html
					ownerReference := objectGetController(sosreportPVC.ObjectMeta)
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
						controllerSosreportPVCs.Items = append(controllerSosreportPVCs.Items, sosreportPVC)
					}
				}

				By("Making sure that the same number of PVCs and of Jobs was created")
				Expect(len(controllerSosreportPVCs.Items)).To(Equal(len(controllerSosreportJobs.Items)))

			} // if useExistingCluster

		})
	})

})

/*
	After writing all this code, you can run `go test ./...` in your `controllers/` directory again to run your new test!
*/
