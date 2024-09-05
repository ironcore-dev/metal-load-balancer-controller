// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package metal_load_balancer_controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("Service Controller", func() {
	ns := SetupTest()

	It("should add the cluster IP to the Service status", func(ctx SpecContext) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    ns.Name,
				GenerateName: "test-",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "foo",
						Protocol:   corev1.ProtocolTCP,
						Port:       10000,
						TargetPort: intstr.IntOrString{IntVal: 10000},
						NodePort:   30000,
					},
				},
				Type: corev1.ServiceTypeLoadBalancer,
			},
		}
		Expect(k8sClient.Create(ctx, service)).To(Succeed())
		DeferCleanup(k8sClient.Delete, service)

		Eventually(Object(service)).Should(SatisfyAll(
			HaveField("Status.LoadBalancer.Ingress", ContainElement(corev1.LoadBalancerIngress{
				IP:     service.Spec.ClusterIP,
				IPMode: ptr.To(corev1.LoadBalancerIPModeVIP),
			})),
		))
	})

	It("should not add the cluster IP to the Service status for non LoadBalancer type Services", func(ctx SpecContext) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    ns.Name,
				GenerateName: "test-",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "foo",
						Protocol:   corev1.ProtocolTCP,
						Port:       10000,
						TargetPort: intstr.IntOrString{IntVal: 10000},
					},
				},
				Type: corev1.ServiceTypeClusterIP,
			},
		}
		Expect(k8sClient.Create(ctx, service)).To(Succeed())
		DeferCleanup(k8sClient.Delete, service)

		Eventually(Object(service)).Should(SatisfyAll(
			HaveField("Status.LoadBalancer.Ingress", BeEmpty()),
		))
	})
})
