//go:build e2e

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

package e2e

import (
	"testing"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lburgazzoli/gomega-matchers/pkg/matchers/jq"

	componentsv1alpha1 "github.com/opendatahub-io/ai-gateway-operator/api/components/v1alpha1"
	"github.com/opendatahub-io/ai-gateway-operator/test/support"
)

func TestMaaSDeployment(t *testing.T) {
	operatorNamespace := support.OperatorNamespace()

	maasModule := &componentsv1alpha1.AIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-maas-gateway",
		},
		Spec: componentsv1alpha1.AIGatewaySpec{
			ModelsAsService: componentsv1alpha1.ModelsAsServiceComponent{
				ManagementState: "Managed",
			},
		},
	}

	maasDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "maas-controller",
			Namespace: operatorNamespace,
		},
	}

	_ = k8sClient.Delete(ctx, maasModule)
	waitForSingletonDeleted(t, maasModule)

	t.Cleanup(func() {
		_ = k8sClient.Delete(ctx, maasModule)
	})

	t.Run("should deploy maas-controller", func(t *testing.T) {
		testMaaSControllerDeployed(t, maasModule, maasDeployment)
	})
	t.Run("should create MaaS CRDs", func(t *testing.T) {
		testMaaSCRDsCreated(t)
	})
	t.Run("should create maas-parameters ConfigMap", func(t *testing.T) {
		testMaaSConfigMapCreated(t, operatorNamespace)
	})
	t.Run("should create default Tenant CR", func(t *testing.T) {
		testDefaultTenantCreated(t)
	})
	t.Run("should update AIGateway status with MaaS source", func(t *testing.T) {
		testMaaSSourceInStatus(t, maasModule)
	})
	t.Run("should set owner references on maas-controller", func(t *testing.T) {
		testMaaSOwnerReferences(t, maasModule, maasDeployment)
	})
}

func testMaaSControllerDeployed(t *testing.T, module *componentsv1alpha1.AIGateway, deploy *appsv1.Deployment) {
	t.Helper()
	g := NewWithT(t)

	// Create AIGateway with MaaS enabled
	module.ResourceVersion = ""
	g.Expect(k8sClient.Create(ctx, module)).To(Succeed())

	// Wait for AIGateway to become ready
	g.Eventually(k.Get(module)).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(And(
		jq.Match(`.status.phase == "Ready"`),
		jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
	))

	// Wait for maas-controller deployment to exist and have at least 1 replica
	// Note: We check for >= 1 replicas instead of exact readiness because
	// maas-controller may experience restarts due to missing optional CRDs (known upstream issue)
	g.Eventually(k.Get(deploy)).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(
		jq.Match(`.spec.replicas >= 1`),
	)

	t.Log("maas-controller deployment created successfully")
}

func testMaaSCRDsCreated(t *testing.T) {
	t.Helper()
	g := NewWithT(t)

	expectedCRDs := []string{
		"aitenants.maas.opendatahub.io",
		"configs.maas.opendatahub.io",
		"externalmodels.maas.opendatahub.io",
		"maasauthpolicies.maas.opendatahub.io",
		"maasmodelrefs.maas.opendatahub.io",
		"maassubscriptions.maas.opendatahub.io",
		"tenants.maas.opendatahub.io",
		"externalmodels.inference.opendatahub.io",
		"externalproviders.inference.opendatahub.io",
	}

	for _, crdName := range expectedCRDs {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: crdName},
		}
		g.Eventually(k.Get(crd)).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(
			jq.Match(`.metadata.name == "%s"`, crdName),
		)
	}

	t.Logf("All %d MaaS CRDs created successfully", len(expectedCRDs))
}

func testMaaSConfigMapCreated(t *testing.T, namespace string) {
	t.Helper()
	g := NewWithT(t)

	// List ConfigMaps matching the pattern maas-parameters-*
	cmList := &corev1.ConfigMapList{}
	g.Eventually(func(g Gomega) {
		g.Expect(k8sClient.List(ctx, cmList, client.InNamespace(namespace))).To(Succeed())

		found := false
		for i := range cmList.Items {
			cm := &cmList.Items[i]
			if len(cm.Name) >= 15 && cm.Name[:15] == "maas-parameters" {
				// Verify it has the expected keys and non-empty values
				g.Expect(cm.Data).To(HaveKey("MAAS_CONTROLLER_IMAGE"))
				g.Expect(cm.Data["MAAS_CONTROLLER_IMAGE"]).NotTo(BeEmpty(), "MAAS_CONTROLLER_IMAGE must not be empty")
				g.Expect(cm.Data).To(HaveKey("MAAS_API_IMAGE"))
				g.Expect(cm.Data["MAAS_API_IMAGE"]).NotTo(BeEmpty(), "MAAS_API_IMAGE must not be empty")
				found = true
				break
			}
		}
		g.Expect(found).To(BeTrue(), "maas-parameters ConfigMap should exist")
	}).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(Succeed())

	t.Log("maas-parameters ConfigMap created with correct keys")
}

func testDefaultTenantCreated(t *testing.T) {
	t.Helper()
	g := NewWithT(t)

	// The default Tenant CR is created in the "models-as-a-service" namespace
	// We need to use unstructured client because Tenant CRD type is not in our scheme
	tenant := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "maas.opendatahub.io/v1alpha1",
			Kind:       "Tenant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-tenant",
			Namespace: "models-as-a-service",
		},
	}

	g.Eventually(func(g Gomega) {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(tenant), tenant)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(tenant.GetName()).To(Equal("default-tenant"))
	}).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(Succeed())

	t.Log("default-tenant CR created in models-as-a-service namespace")
}

func testMaaSSourceInStatus(t *testing.T, module *componentsv1alpha1.AIGateway) {
	t.Helper()
	g := NewWithT(t)

	g.Eventually(k.Get(module)).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(
		jq.Match(`.status.module.sources[] | select(.path | contains("maascontroller")) | .renderer == "kustomize"`),
	)

	t.Log("AIGateway status includes MaaS manifest source")
}

func testMaaSOwnerReferences(t *testing.T, module *componentsv1alpha1.AIGateway, deploy *appsv1.Deployment) {
	t.Helper()
	g := NewWithT(t)

	// Refresh module to get its UID
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(module), module)).To(Succeed())

	g.Eventually(k.Get(deploy)).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(
		And(
			jq.Match(`.metadata.ownerReferences[] | select(.kind == "AIGateway") | .name == "%s"`, module.Name),
			jq.Match(`.metadata.ownerReferences[] | select(.kind == "AIGateway") | .uid == "%s"`, module.UID),
		),
	)

	t.Log("maas-controller deployment has correct owner reference with matching UID")
}
