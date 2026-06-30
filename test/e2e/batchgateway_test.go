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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lburgazzoli/gomega-matchers/pkg/matchers/jq"

	componentsv1alpha1 "github.com/opendatahub-io/ai-gateway-operator/api/components/v1alpha1"
	"github.com/opendatahub-io/ai-gateway-operator/test/support"
)

func TestBatchGateway(t *testing.T) {
	operatorNamespace := support.OperatorNamespace()

	module := &componentsv1alpha1.AIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-batch-gateway",
		},
		Spec: componentsv1alpha1.AIGatewaySpec{
			BatchGateway: componentsv1alpha1.BatchGatewayComponent{
				ManagementState: "Managed",
			},
		},
	}

	workloadDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llm-d-batch-gateway-operator",
			Namespace: operatorNamespace,
		},
	}

	_ = k8sClient.Delete(ctx, module)
	waitForSingletonDeleted(t, module)

	t.Cleanup(func() {
		_ = k8sClient.Delete(ctx, module)
	})

	t.Run("should become ready", func(t *testing.T) {
		testBatchGatewayBecomesReady(t, module)
	})
	t.Run("should deploy batch-gateway operator", func(t *testing.T) {
		eventuallyDeploymentReady(t, workloadDeploy)
	})
	t.Run("should set platform labels on workload", func(t *testing.T) {
		testBatchGatewayPlatformLabels(t, module, workloadDeploy)
	})
	t.Run("should set owner references on workload", func(t *testing.T) {
		testBatchGatewayOwnerReferences(t, module, workloadDeploy)
	})
}

func testBatchGatewayBecomesReady(t *testing.T, module *componentsv1alpha1.AIGateway) {
	t.Helper()
	g := NewWithT(t)

	module.ResourceVersion = ""
	g.Expect(k8sClient.Create(ctx, module)).To(Succeed())

	g.Eventually(k.Get(module)).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(And(
		jq.Match(`.status.phase == "Ready"`),
		jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningSucceeded") | .status == "True"`),
	))
}

func testBatchGatewayPlatformLabels(t *testing.T, module *componentsv1alpha1.AIGateway, deploy *appsv1.Deployment) {
	t.Helper()
	g := NewWithT(t)

	// Refresh module to get its UID
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(module), module)).To(Succeed())

	g.Eventually(k.Get(deploy)).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(And(
		jq.Match(`.metadata.labels."%s" == "aigateway"`, labelPartOf),
		jq.Match(`.metadata.annotations."%s" == "%s"`,
			annotationInstanceName,
			module.GetName()),
		jq.Match(`.metadata.annotations."%s" == "%s"`,
			annotationInstanceUID,
			string(module.GetUID())),
	))
}

func testBatchGatewayOwnerReferences(t *testing.T, module *componentsv1alpha1.AIGateway, deploy *appsv1.Deployment) {
	t.Helper()
	g := NewWithT(t)

	// Refresh module to get its UID
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(module), module)).To(Succeed())

	g.Eventually(k.Get(deploy)).WithContext(ctx).WithTimeout(timeout).WithPolling(interval).Should(
		jq.Match(`.metadata.ownerReferences[] | select(.kind == "AIGateway") | .name == "%s"`,
			module.Name),
	)
}
