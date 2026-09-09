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
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/odh-platform-utilities/framework/controller/types"
	"github.com/opendatahub-io/ray-module-operator/internal/constants"
)

func TestRequiredCertManagerGVKs(t *testing.T) {
	resources := []unstructured.Unstructured{{Object: map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Issuer",
	}}}

	got := requiredCertManagerGVKs(resources)
	if len(got) != 1 || got[0] != certManagerGVKs[0] {
		t.Fatalf("required cert-manager GVKs = %#v, want %#v", got, certManagerGVKs[:1])
	}
}

func TestCertManagerRequirementAction(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add CRD scheme: %v", err)
	}

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "cert-manager.io", Version: "v1"}})
	for _, gvk := range certManagerGVKs {
		mapper.Add(gvk, meta.RESTScopeNamespace)
	}

	issuer := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "issuers.cert-manager.io"},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(mapper).WithObjects(issuer).Build()
	rr := &types.ReconciliationRequest{
		Client: cli,
		Resources: []unstructured.Unstructured{
			{Object: map[string]any{
				"apiVersion": "cert-manager.io/v1",
				"kind":       "Issuer",
			}},
			{Object: map[string]any{
				"apiVersion": "cert-manager.io/v1",
				"kind":       "Certificate",
			}},
		},
		Extensions: map[string]any{constants.ExtKeyRemoved: false},
	}

	err := certManagerRequirementAction()(context.Background(), rr)
	if err == nil || !strings.Contains(err.Error(), "cert-manager is required") {
		t.Fatalf("cert-manager requirement error = %v, want requirement message", err)
	}
}
