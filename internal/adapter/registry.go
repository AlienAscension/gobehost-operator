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

package adapter

import (
	"fmt"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

var registry = make(map[string]GameAdapter)

func Register(a GameAdapter) {
	registry[a.Name()] = a
}

func Get(gs *gamesv1alpha1.GameServer) (GameAdapter, error) {
	a, ok := registry[gs.Spec.Game.Type]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for game type %q", gs.Spec.Game.Type)
	}
	return a, nil
}

func KnownGames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
