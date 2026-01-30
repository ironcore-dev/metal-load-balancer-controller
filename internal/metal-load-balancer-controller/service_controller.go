// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package metal_load_balancer_controller

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	service := &corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, service); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return r.reconcileExists(ctx, log, service)
}

func (r *ServiceReconciler) reconcileExists(ctx context.Context, log logr.Logger, service *corev1.Service) (ctrl.Result, error) {
	if !service.DeletionTimestamp.IsZero() {
		return r.delete(ctx, log, service)
	}
	return r.reconcile(ctx, log, service)
}

func (r *ServiceReconciler) delete(ctx context.Context, log logr.Logger, service *corev1.Service) (ctrl.Result, error) {
	log.V(1).Info("Deleting Service")

	log.V(1).Info("Deleted Service")
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) reconcile(ctx context.Context, _ logr.Logger, service *corev1.Service) (ctrl.Result, error) {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return ctrl.Result{}, nil
	}

	newServiceIP, err := generateServiceIP(service)
	if err != nil {
		return ctrl.Result{}, err
	}

	serviceBase := service.DeepCopy()
	service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
		{
			IP: newServiceIP,
		},
	}
	if err := r.Status().Patch(ctx, service, client.MergeFrom(serviceBase)); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func generateServiceIP(service *corev1.Service) (string, error) {
	ip, err := netip.ParseAddr(service.Spec.ClusterIP)
	if err != nil {
		return "", fmt.Errorf("invalid ClusterIP format: %w", err)
	}
	if ip.Is4() {
		return "", fmt.Errorf("IPv4 is not supported")
	}

	b := ip.As16()
	// add plus one to the 14th byte to get a new IP for the service
	b[13]++

	if b[13] == 0 {
		return "", fmt.Errorf("unsupported service range")
	}
	return netip.AddrFrom16(b).String(), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}
