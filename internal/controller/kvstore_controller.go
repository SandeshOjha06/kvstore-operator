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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KVStore object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *KVStoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// TODO(user): your logic here
	var kvstore datastorev1alpha1.KVStore
	if err := r.Get(ctx, req.NamespacedName, &kvstore); err != nil {
    // If it's not found, the user deleted it. Just return nil.
    	return ctrl.Result{}, client.IgnoreNotFound(err)
}

	labels := map[string]string{"app": "kvstore"}

	// build the StatefulSet
	sts :=&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: kvstore.Name + "-sts",
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
                    // 
					VolumeMounts: []corev1.VolumeMount{{
    					Name:      "wal-storage", // Must match the PVC name exactly
    					MountPath: "/app/data",
						}},
                }},
            },
        },
		VolumeClaimTempelats: []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{

					Name: "wal-storage",
				},
			Spec : PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessModes{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
                    // corev1.ResourceStorage is a constant for the string "storage"
                    corev1.ResourceStorage: kvstore.Spec.Storage,
                },
				},
			},
		},
		//
		},
	},


	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KVStoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&datastorev1alpha1.KVStore{}).
		Named("kvstore").
		Complete(r)
}
