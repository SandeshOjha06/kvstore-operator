/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors" // For the IsNotFound check
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types" // For NamespacedName
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil" // For Owner References
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	datastorev1alpha1 "github.com/SandeshOjha06/kvstore-operator/api/v1alpha1"
)

// KVStoreReconciler reconciles a KVStore object
type KVStoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=datastore.core.systems,resources=kvstores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=datastore.core.systems,resources=kvstores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=datastore.core.systems,resources=kvstores/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KVStoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	var kvstore datastorev1alpha1.KVStore
	if err := r.Get(ctx, req.NamespacedName, &kvstore); err != nil {
		// If it's not found, the user deleted it. Just return nil.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	labels := map[string]string{"app": "kvstore"}

	// build the StatefulSet
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kvstore.Name + "-sts",
			Namespace: kvstore.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &kvstore.Spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "kvstore",
						Image: kvstore.Spec.ContainerImage,
						// Injecting the Size dynamically into the environment variable
						Env: []corev1.EnvVar{{
							Name:  "CLUSTER_SIZE",
							Value: fmt.Sprintf("%d", kvstore.Spec.Size),
						}},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 6379,
							Name:          "kv-port",
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "wal-storage", // Must match the PVC name exactly
							MountPath: "/app/data",
						}},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wal-storage",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: kvstore.Spec.Storage,
							},
						},
					},
				},
			},
		},
	}

	// if user deletes kvstore the stateful set is automatically deleted
	if err := controllerutil.SetControllerReference(&kvstore, sts, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if the StatefulSet already exists
	foundSts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, foundSts)

	if err != nil && apierrors.IsNotFound(err) {
		// The StatefulSet does not exist. Create it.
		log := logf.FromContext(ctx)
		log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)

		if err := r.Create(ctx, sts); err != nil {
			// Failed to create, return error to retry
			log.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
			return ctrl.Result{}, err
		}

		// Creation was successful. Requeue the loop to verify the state in the next pass.
		return ctrl.Result{Requeue: true}, nil

	} else if err != nil {
		// An unexpected error occurred while talking to the API server
		log := logf.FromContext(ctx)
		log.Error(err, "Failed to get StatefulSet")
		return ctrl.Result{}, err
	}

	// The StatefulSet already exists(nothing for phase 1).
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KVStoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&datastorev1alpha1.KVStore{}).
		Named("kvstore").
		Complete(r)
}
