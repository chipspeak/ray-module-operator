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
	"testing"

	"github.com/opendatahub-io/odh-platform-utilities/framework/controller/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	componentsv1alpha1 "github.com/opendatahub-io/ray-module-operator/api/v1alpha1"
	"github.com/opendatahub-io/ray-module-operator/internal/constants"
)

func TestDeleteOwnedOperandsRemovesOwnedDeployment(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := componentsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("apps scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("core scheme: %v", err)
	}

	const ownerUID k8stypes.UID = "ray-uid"
	ray := &componentsv1alpha1.Ray{
		ObjectMeta: metav1.ObjectMeta{Name: constants.InstanceName, UID: ownerUID},
		Spec:       componentsv1alpha1.RaySpec{ApplicationsNamespace: "apps"},
	}
	ctrl := true
	owned := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kuberay-operator",
			Namespace: "apps",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "components.platform.opendatahub.io/v1alpha1",
				Kind:       "Ray",
				Name:       constants.InstanceName,
				UID:        ownerUID,
				Controller: &ctrl,
			}},
		},
	}
	unrelated := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "apps"},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ray, owned, unrelated).Build()
	rr := &types.ReconciliationRequest{Client: cli, Instance: ray}
	if err := deleteOwnedOperands(context.Background(), rr); err != nil {
		t.Fatalf("deleteOwnedOperands: %v", err)
	}

	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(owned), &appsv1.Deployment{}); !apierrors.IsNotFound(err) {
		t.Fatalf("owned Deployment still present: %v", err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(unrelated), &appsv1.Deployment{}); err != nil {
		t.Fatalf("unrelated Deployment should remain: %v", err)
	}
}

func TestDeleteOwnedOperandsRemovesLabeledDeploymentWithoutOwner(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := componentsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("apps scheme: %v", err)
	}

	const ownerUID k8stypes.UID = "ray-uid"
	ray := &componentsv1alpha1.Ray{
		ObjectMeta: metav1.ObjectMeta{Name: constants.InstanceName, UID: ownerUID},
		Spec:       componentsv1alpha1.RaySpec{ApplicationsNamespace: "apps"},
	}
	labeled := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kuberay-operator",
			Namespace: "apps",
			Labels:    map[string]string{"platform.opendatahub.io/part-of": constants.ComponentName},
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ray, labeled).Build()
	rr := &types.ReconciliationRequest{Client: cli, Instance: ray}
	if err := deleteOwnedOperands(context.Background(), rr); err != nil {
		t.Fatalf("deleteOwnedOperands: %v", err)
	}

	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(labeled), &appsv1.Deployment{}); !apierrors.IsNotFound(err) {
		t.Fatalf("labeled Deployment still present: %v", err)
	}
}

func TestOwnedByUID(t *testing.T) {
	if ownedByUID(&appsv1.Deployment{}, "x") {
		t.Fatal("empty ownerRefs should not match")
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{{UID: "x"}},
		},
	}
	if !ownedByUID(dep, "x") {
		t.Fatal("expected UID match")
	}
	if ownedByUID(dep, "y") {
		t.Fatal("expected UID mismatch")
	}
}
