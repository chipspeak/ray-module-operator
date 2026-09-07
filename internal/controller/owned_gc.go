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

	"github.com/opendatahub-io/odh-platform-utilities/framework/controller/types"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/framework/controller/actions/gc"
	componentsv1alpha1 "github.com/opendatahub-io/ray-module-operator/api/v1alpha1"
	"github.com/opendatahub-io/ray-module-operator/internal/constants"
)

// namespacedOwnedLists are operand kinds rendered into applicationsNamespace.
var namespacedOwnedLists = []func() client.ObjectList{
	func() client.ObjectList { return &appsv1.DeploymentList{} },
	func() client.ObjectList { return &corev1.ConfigMapList{} },
	func() client.ObjectList { return &corev1.ServiceList{} },
	func() client.ObjectList { return &corev1.ServiceAccountList{} },
}

// clusterOwnedLists are cluster-scoped operands the module SSA-applies.
// CRDs are intentionally omitted — they must survive Removed.
var clusterOwnedLists = []func() client.ObjectList{
	func() client.ObjectList { return &admissionregistrationv1.MutatingWebhookConfigurationList{} },
	func() client.ObjectList { return &rbacv1.ClusterRoleList{} },
	func() client.ObjectList { return &rbacv1.ClusterRoleBindingList{} },
}

// deleteOwnedOperands deletes objects whose ownerRef is the Ray CR.
// Framework Removed GC only lists platform.opendatahub.io/part-of=ray; if
// that label was dropped, owned Deployments (and similar) are left behind.
func deleteOwnedOperands(ctx context.Context, rr *types.ReconciliationRequest) error {
	if rr == nil || rr.Client == nil || rr.Instance == nil {
		return nil
	}

	ray, ok := rr.Instance.(*componentsv1alpha1.Ray)
	if !ok {
		return nil
	}
	if ray.GetUID() == "" {
		return nil
	}

	ns := ray.Spec.ApplicationsNamespace
	if ns != "" {
		for _, newList := range namespacedOwnedLists {
			if err := deleteOwnedFromList(ctx, rr.Client, ray.GetUID(), newList(), client.InNamespace(ns)); err != nil {
				return err
			}
		}
	}

	for _, newList := range clusterOwnedLists {
		if err := deleteOwnedFromList(ctx, rr.Client, ray.GetUID(), newList()); err != nil {
			return err
		}
	}

	return nil
}

func deleteOwnedFromList(ctx context.Context, cli client.Client, ownerUID k8stypes.UID, list client.ObjectList, opts ...client.ListOption) error {
	if err := cli.List(ctx, list, opts...); err != nil {
		if k8serr.IsNotFound(err) || meta.IsNoMatchError(err) || runtime.IsNotRegisteredError(err) {
			return nil
		}

		return fmt.Errorf("list %T for owned-operand GC: %w", list, err)
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return fmt.Errorf("extract %T: %w", list, err)
	}

	for _, item := range items {
		obj, ok := item.(client.Object)
		if !ok {
			continue
		}
		if !ownedByUID(obj, ownerUID) && !labeledOperand(obj) {
			continue
		}

		if err := cli.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !k8serr.IsNotFound(err) {
			return fmt.Errorf("delete owned %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return nil
}

func labeledOperand(obj client.Object) bool {
	return obj.GetLabels()[gc.DefaultPartOfLabelKey] == constants.ComponentName
}

func ownedByUID(obj client.Object, ownerUID k8stypes.UID) bool {
	if ownerUID == "" {
		return false
	}

	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == ownerUID {
			return true
		}
	}

	return false
}
