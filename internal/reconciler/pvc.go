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

package reconciler

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func BuildPVC(gs *gamesv1alpha1.GameServer) *corev1.PersistentVolumeClaim {
	accessModes := gs.Spec.Storage.AccessModes
	if len(accessModes) == 0 {
		accessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name + "-data",
			Namespace: gs.Namespace,
			Labels:    GameServerLabels(gs),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: gs.Spec.Storage.Size,
				},
			},
		},
	}

	if gs.Spec.Storage.StorageClass != nil {
		pvc.Spec.StorageClassName = gs.Spec.Storage.StorageClass
	}

	return pvc
}

func mergeStorageQuantity(existing resource.Quantity, fallback resource.Quantity) resource.Quantity {
	if existing.IsZero() {
		return fallback
	}
	return existing
}
