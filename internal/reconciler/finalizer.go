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
	"context"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func AddFinalizer(ctx context.Context, c client.Client, gs *gamesv1alpha1.GameServer) (bool, error) {
	if HasFinalizer(gs) {
		return false, nil
	}

	gs.Finalizers = append(gs.Finalizers, gamesv1alpha1.GameServerFinalizer)
	if err := c.Update(ctx, gs); err != nil {
		return false, err
	}
	return true, nil
}

func RemoveFinalizer(ctx context.Context, c client.Client, gs *gamesv1alpha1.GameServer) error {
	if !HasFinalizer(gs) {
		return nil
	}

	finalizers := make([]string, 0, len(gs.Finalizers))
	for _, f := range gs.Finalizers {
		if f != gamesv1alpha1.GameServerFinalizer {
			finalizers = append(finalizers, f)
		}
	}
	gs.Finalizers = finalizers
	return c.Update(ctx, gs)
}

func HasFinalizer(gs *gamesv1alpha1.GameServer) bool {
	return slices.Contains(gs.Finalizers, gamesv1alpha1.GameServerFinalizer)
}
