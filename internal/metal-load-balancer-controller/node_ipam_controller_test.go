// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package metal_load_balancer_controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("Node IPAM Controller", func() {
	Context("When the PodCIDR is not populated", func() {
		It("should populate PodCIDR and PodCIDRs", func(ctx SpecContext) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
				},
				Spec: corev1.NodeSpec{
					PodCIDR:  "",
					PodCIDRs: []string{},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "1a10:c0de::1",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, node)).Should(Succeed())
			DeferCleanup(k8sClient.Delete, node)

			Eventually(Object(node)).Should(SatisfyAll(
				HaveField("Spec.PodCIDR", Equal("1a10:c0de::1/64")),
				HaveField("Spec.PodCIDRs", ContainElement("1a10:c0de::1/64")),
			))

		})
	})

	Context("When the PodCIDR is already populated", func() {
		It("should not modify PodCIDR and PodCIDRs", func(ctx SpecContext) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
				},
				Spec: corev1.NodeSpec{
					PodCIDR: "cafe:c0de::/80",
					PodCIDRs: []string{
						"cafe:c0de::/80",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "1a10:code::1",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, node)).Should(Succeed())
			DeferCleanup(k8sClient.Delete, node)

			Eventually(Object(node)).Should(SatisfyAll(
				HaveField("Spec.PodCIDR", Equal("cafe:c0de::/80")),
				HaveField("Spec.PodCIDRs", ContainElement("cafe:c0de::/80")),
			))
		})
	})
})
