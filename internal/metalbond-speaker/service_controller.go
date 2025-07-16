// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package metalbondspeaker

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/go-logr/logr"
	"github.com/ironcore-dev/controller-utils/clientutils"
	"github.com/ironcore-dev/metalbond"
	"github.com/ironcore-dev/metalbond/pb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	VNI         int
	MetalBond   *metalbond.MetalBond
	NodeAddress string
}

var (
	ServiceFinalizer = "metal-loadbalancer.ironcore.dev/service"
)

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

	prefix := fmt.Sprintf("%s/128", service.Spec.ClusterIP)
	dest := metalbond.Destination{
		IPVersion: metalbond.IPV6,
		Prefix:    netip.MustParsePrefix(prefix),
	}
	nextHop := metalbond.NextHop{
		TargetAddress: netip.MustParseAddr(r.NodeAddress),
		TargetVNI:     uint32(r.VNI),
		Type:          pb.NextHopType_STANDARD,
	}
	if err := r.MetalBond.WithdrawRoute(metalbond.VNI(r.VNI), dest, nextHop); err != nil {
		return ctrl.Result{}, err
	}
	log.V(1).Info("Removed route", "VNI", r.VNI, "Destination", dest, "NextHop", nextHop)

	log.V(1).Info("Ensuring that the finalizer is removed")
	if modified, err := clientutils.PatchEnsureNoFinalizer(ctx, r.Client, service, ServiceFinalizer); err != nil || modified {
		return ctrl.Result{}, err
	}

	log.V(1).Info("Deleted Service")
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) reconcile(ctx context.Context, log logr.Logger, service *corev1.Service) (ctrl.Result, error) {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return ctrl.Result{}, nil
	}

	if modified, err := clientutils.PatchEnsureFinalizer(ctx, r.Client, service, ServiceFinalizer); err != nil || modified {
		return ctrl.Result{}, err
	}
	log.V(1).Info("Ensured finalizer has been added")

	prefix := fmt.Sprintf("%s/128", service.Spec.ClusterIP)
	dest := metalbond.Destination{
		// TODO:
		IPVersion: metalbond.IPV6,
		Prefix:    netip.MustParsePrefix(prefix),
	}
	nextHop := metalbond.NextHop{
		TargetAddress: netip.MustParseAddr(r.NodeAddress),
		TargetVNI:     uint32(r.VNI),
		Type:          pb.NextHopType_STANDARD,
	}

	if !r.MetalBond.IsRouteAnnounced(metalbond.VNI(r.VNI), dest, nextHop) {
		if err := r.MetalBond.AnnounceRoute(metalbond.VNI(r.VNI), dest, nextHop); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}, builder.WithPredicates(predicate.NewPredicateFuncs(
			func(obj client.Object) bool {
				service := obj.(*corev1.Service)
				return service.Spec.Type == corev1.ServiceTypeLoadBalancer
			}))).
		Complete(r)
}
