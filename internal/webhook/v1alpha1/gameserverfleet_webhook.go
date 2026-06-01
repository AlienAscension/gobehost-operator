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

package v1alpha1

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var gameserverfleetlog = logf.Log.WithName("gameserverfleet-resource")

func SetupGameServerFleetWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &gamesv1alpha1.GameServerFleet{}).
		WithValidator(&GameServerFleetCustomValidator{}).
		WithDefaulter(&GameServerFleetCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-games-gobehost-com-v1alpha1-gameserverfleet,mutating=true,failurePolicy=fail,sideEffects=None,groups=games.gobehost.com,resources=gameserverfleets,verbs=create;update,versions=v1alpha1,name=mgameserverfleet-v1alpha1.kb.io,admissionReviewVersions=v1

type GameServerFleetCustomDefaulter struct{}

func (d *GameServerFleetCustomDefaulter) Default(_ context.Context, obj *gamesv1alpha1.GameServerFleet) error {
	gameserverfleetlog.Info("Defaulting for GameServerFleet", "name", obj.GetName())

	fleet := obj

	if fleet.Spec.Replicas == 0 {
		fleet.Spec.Replicas = 1
	}

	if fleet.Spec.Strategy.Type == "" {
		fleet.Spec.Strategy.Type = gamesv1alpha1.RollingUpdateStrategyType
	}

	spec := &fleet.Spec.Template.Spec

	if spec.Runtime.ImagePullPolicy == "" {
		spec.Runtime.ImagePullPolicy = corev1.PullIfNotPresent
	}

	if len(spec.Storage.AccessModes) == 0 {
		spec.Storage.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}

	if spec.Network.ServiceType == "" {
		spec.Network.ServiceType = corev1.ServiceTypeLoadBalancer
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-games-gobehost-com-v1alpha1-gameserverfleet,mutating=false,failurePolicy=fail,sideEffects=None,groups=games.gobehost.com,resources=gameserverfleets,verbs=create;update,versions=v1alpha1,name=vgameserverfleet-v1alpha1.kb.io,admissionReviewVersions=v1

type GameServerFleetCustomValidator struct{}

func (v *GameServerFleetCustomValidator) ValidateCreate(_ context.Context, obj *gamesv1alpha1.GameServerFleet) (admission.Warnings, error) {
	gameserverfleetlog.Info("Validation for GameServerFleet upon creation", "name", obj.GetName())
	return nil, validateGameServerFleet(obj).ToAggregate()
}

func (v *GameServerFleetCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *gamesv1alpha1.GameServerFleet) (admission.Warnings, error) {
	gameserverfleetlog.Info("Validation for GameServerFleet upon update", "name", newObj.GetName())

	allErrs := validateGameServerFleet(newObj)

	if oldObj.Spec.Replicas != newObj.Spec.Replicas {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "replicas"), "replicas is immutable"),
		)
	}

	return nil, allErrs.ToAggregate()
}

func (v *GameServerFleetCustomValidator) ValidateDelete(_ context.Context, obj *gamesv1alpha1.GameServerFleet) (admission.Warnings, error) {
	gameserverfleetlog.Info("Validation for GameServerFleet upon deletion", "name", obj.GetName())
	return nil, nil
}

func validateGameServerFleet(fleet *gamesv1alpha1.GameServerFleet) field.ErrorList {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	if fleet.Spec.Replicas != 1 {
		allErrs = append(allErrs, field.Forbidden(specPath.Child("replicas"), "replicas must equal 1"))
	}

	templatePath := field.NewPath("spec", "template", "spec")
	template := fleet.Spec.Template.Spec

	if template.Game.Type == "" {
		allErrs = append(allErrs, field.Required(templatePath.Child("game", "type"), "game type is required"))
	}
	if template.Game.Version == "" {
		allErrs = append(allErrs, field.Required(templatePath.Child("game", "version"), "game version is required"))
	}
	if template.Runtime.Image == "" {
		allErrs = append(allErrs, field.Required(templatePath.Child("runtime", "image"), "runtime image is required"))
	}
	if len(template.Network.Ports) == 0 {
		allErrs = append(allErrs, field.Required(templatePath.Child("network", "ports"), "at least one port is required"))
	}
	if equality.Semantic.DeepEqual(template.Storage.Size, gamesv1alpha1.StorageSpec{}.Size) {
		allErrs = append(allErrs, field.Required(templatePath.Child("storage", "size"), "storage size is required"))
	}

	return allErrs
}
