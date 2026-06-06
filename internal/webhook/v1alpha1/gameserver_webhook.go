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
	"k8s.io/utils/ptr"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// nolint:unused
// log is for logging in this package.
var gameserverlog = logf.Log.WithName("gameserver-resource")

// SetupGameServerWebhookWithManager registers the webhook for GameServer in the manager.
func SetupGameServerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &gamesv1alpha1.GameServer{}).
		WithValidator(&GameServerCustomValidator{}).
		WithDefaulter(&GameServerCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-games-gobehost-com-v1alpha1-gameserver,mutating=true,failurePolicy=fail,sideEffects=None,groups=games.gobehost.com,resources=gameservers,verbs=create;update,versions=v1alpha1,name=mgameserver-v1alpha1.kb.io,admissionReviewVersions=v1

// GameServerCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind GameServer when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type GameServerCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind GameServer.
func (d *GameServerCustomDefaulter) Default(_ context.Context, obj *gamesv1alpha1.GameServer) error {
	gameserverlog.Info("Defaulting for GameServer", "name", obj.GetName)

	spec := &obj.Spec

	if spec.Runtime.ImagePullPolicy == "" {
		spec.Runtime.ImagePullPolicy = corev1.PullIfNotPresent
	}

	if len(spec.Storage.AccessModes) == 0 {
		spec.Storage.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}

	if spec.Network.ServiceType == "" {
		spec.Network.ServiceType = corev1.ServiceTypeLoadBalancer
	}

	for i := range spec.Network.Ports {
		p := &spec.Network.Ports[i]
		if p.Protocol == "" {
			p.Protocol = corev1.ProtocolTCP
		}
		if p.TargetPort == nil {
			p.TargetPort = ptr.To(p.Port)
		}
	}

	if spec.Security == nil {
		spec.Security = &gamesv1alpha1.SecuritySpec{}
	}
	if spec.Security.RunAsNonRoot == nil {
		spec.Security.RunAsNonRoot = ptr.To(true)
	}
	if spec.Security.RunAsUser == nil {
		spec.Security.RunAsUser = ptr.To(int64(1000))
	}
	if spec.Security.RunAsGroup == nil {
		spec.Security.RunAsGroup = ptr.To(int64(1000))
	}
	if spec.Security.FSGroup == nil {
		spec.Security.FSGroup = ptr.To(int64(1000))
	}
	if spec.Security.SeccompProfile == "" {
		spec.Security.SeccompProfile = corev1.SeccompProfileTypeRuntimeDefault
	}
	if spec.Security.DropAllCapabilities == nil {
		spec.Security.DropAllCapabilities = ptr.To(true)
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-games-gobehost-com-v1alpha1-gameserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=games.gobehost.com,resources=gameservers,verbs=create;update,versions=v1alpha1,name=vgameserver-v1alpha1.kb.io,admissionReviewVersions=v1

// GameServerCustomValidator struct is responsible for validating the GameServer resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type GameServerCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type GameServer.
func (v *GameServerCustomValidator) ValidateCreate(_ context.Context, obj *gamesv1alpha1.GameServer) (admission.Warnings, error) {
	gameserverlog.Info("Validation for GameServer upon creation", "name", obj.GetName())

	return nil, validateGameServer(obj).ToAggregate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type GameServer.
func (v *GameServerCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *gamesv1alpha1.GameServer) (admission.Warnings, error) {
	gameserverlog.Info("Validation for GameServer upon update", "name", newObj.GetName())

	allErrs := validateGameServer(newObj)

	if oldObj.Spec.Game.Type != newObj.Spec.Game.Type {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec", "game", "type"), "game type is immutable"),
		)
	}

	return nil, allErrs.ToAggregate()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type GameServer.
func (v *GameServerCustomValidator) ValidateDelete(_ context.Context, obj *gamesv1alpha1.GameServer) (admission.Warnings, error) {
	gameserverlog.Info("Validation for GameServer upon deletion", "name", obj.GetName())

	return nil, nil
}

func validateGameServer(gs *gamesv1alpha1.GameServer) field.ErrorList {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	if gs.Spec.Game.Type == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("game", "type"), "game type is required"))
	}
	if gs.Spec.Game.Version == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("game", "version"), "game version is required"))
	}
	if gs.Spec.Runtime.Image == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("runtime", "image"), "runtime image is required"))
	}
	if len(gs.Spec.Network.Ports) == 0 {
		allErrs = append(allErrs, field.Required(specPath.Child("network", "ports"), "at least one port is required"))
	}
	if equality.Semantic.DeepEqual(gs.Spec.Storage.Size, gamesv1alpha1.StorageSpec{}.Size) {
		allErrs = append(allErrs, field.Required(specPath.Child("storage", "size"), "storage size is required"))
	}

	for i, p := range gs.Spec.Network.Ports {
		portPath := specPath.Child("network", "ports").Index(i)
		if p.Port < 1 || p.Port > 65535 {
			allErrs = append(allErrs, field.Invalid(portPath.Child("port"), p.Port, "port must be between 1 and 65535"))
		}
		if p.TargetPort != nil && (*p.TargetPort < 1 || *p.TargetPort > 65535) {
			allErrs = append(allErrs, field.Invalid(portPath.Child("targetPort"), *p.TargetPort, "targetPort must be between 1 and 65535"))
		}
	}

	return allErrs
}
