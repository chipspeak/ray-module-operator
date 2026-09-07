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
	"path/filepath"

	"github.com/opendatahub-io/odh-platform-utilities/framework/controller/actions"
	"github.com/opendatahub-io/odh-platform-utilities/framework/controller/actions/gc"
	"github.com/opendatahub-io/odh-platform-utilities/framework/controller/types"
	"github.com/opendatahub-io/odh-platform-utilities/pkg/render/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/opendatahub-io/ray-module-operator/internal/constants"
)

// RenderKustomize returns an action that renders Kustomize manifests from
// the filesystem using the root kustomize engine. This sidesteps the
// framework's Engine interface mismatch (P5) by using the root engine
// directly — it defaults to filesys.MakeFsOnDisk(), which reads the
// vendored manifests at basePath without any embed.FS adapter.
func RenderKustomize(basePath string, namespaceFn actions.Getter[string]) actions.Fn {
	return func(ctx context.Context, rr *types.ReconciliationRequest) error {
		if len(rr.Manifests) == 0 {
			return nil
		}

		ns, err := namespaceFn(ctx, rr)
		if err != nil {
			return err
		}

		engine := kustomize.NewEngine()
		renderBase := basePath
		if p, ok := rr.Extensions[extKeyWritableManifests].(string); ok && p != "" {
			renderBase = p
		}

		for _, m := range rr.Manifests {
			path := m.Path
			if m.ContextDir != "" {
				path = filepath.Join(path, m.ContextDir)
			}
			if m.SourcePath != "" {
				path = filepath.Join(path, m.SourcePath)
			}
			path = filepath.Join(renderBase, path)

			resources, err := engine.Render(path, kustomize.WithNamespace(ns))
			if err != nil {
				return fmt.Errorf("render manifests from %s: %w", path, err)
			}
			// RHOAI operand YAML hardcodes redhat-ods-applications on some
			// namespaced objects; kustomize does not always override those.
			forceNamespacedResources(ns, resources)
			stampPartOfLabels(resources)

			rr.Resources = append(rr.Resources, resources...)
		}

		rr.Generated = true

		return nil
	}
}

func forceNamespacedResources(ns string, objs []unstructured.Unstructured) {
	if ns == "" {
		return
	}

	for i := range objs {
		if objs[i].GetNamespace() != "" {
			objs[i].SetNamespace(ns)
		}
	}
}

// stampPartOfLabels always sets platform.opendatahub.io/part-of on rendered
// operands. Framework GC lists by that label; the deploy action only adds it
// when the desired object is missing it, so a kustomize/SSA pass can drop it
// and Removed then never sees the object. CRDs stay unlabeled so GC's
// unremovable CRD rule is not the only thing keeping them.
func stampPartOfLabels(objs []unstructured.Unstructured) {
	for i := range objs {
		if objs[i].GetKind() == "CustomResourceDefinition" {
			continue
		}

		labels := objs[i].GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[gc.DefaultPartOfLabelKey] = constants.ComponentName
		objs[i].SetLabels(labels)
	}
}
