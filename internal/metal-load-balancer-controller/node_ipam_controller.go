// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package metal_load_balancer_controller

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeIPAMReconciler reconciles a Node object
type NodeIPAMReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	NodeCIDRMaskSize int
}

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch

func (r *NodeIPAMReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	node := &corev1.Node{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if node.Spec.PodCIDR != "" {
		logger.Info("PodCIDR is already populated; patch was not done", "NodeIPAM", node.Name, "PodCIDR", node.Spec.PodCIDR)
		return ctrl.Result{}, nil
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			ip := net.ParseIP(addr.Address)
			if ip == nil {
				return ctrl.Result{}, fmt.Errorf("invalid IP address format")
			}

			maskedIP := zeroHostBits(ip, r.NodeCIDRMaskSize)
			podCIDR := fmt.Sprintf("%s/%d", maskedIP, r.NodeCIDRMaskSize)

			nodeBase := node.DeepCopy()
			node.Spec.PodCIDR = podCIDR
			if node.Spec.PodCIDRs == nil {
				node.Spec.PodCIDRs = []string{}
			}
			node.Spec.PodCIDRs = append(node.Spec.PodCIDRs, podCIDR)

			if err := r.Patch(ctx, node, client.MergeFrom(nodeBase)); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to patch Node's PodCIDR with error %w", err)
			}

			logger.Info("Patched Node's PodCIDR and PodCIDRs", "NodeIPAM", node.Name, "PodCIDR", podCIDR)
			break
		}
	}

	return ctrl.Result{}, nil
}

func zeroHostBits(ip net.IP, maskSize int) net.IP {
	if ip.To4() != nil {
		mask := net.CIDRMask(maskSize, 32)
		return ip.Mask(mask)
	} else {
		mask := net.CIDRMask(maskSize, 128)
		return ip.Mask(mask)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeIPAMReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
