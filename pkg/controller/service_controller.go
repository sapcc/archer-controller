/*
Copyright 2023 SAP.

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

// Package controller implements business logic for the controller.
// It contains the ArcherServiceReconciler which reconciles a service object.
package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	archerclient "github.com/sapcc/archer/client"
	"github.com/sapcc/archer/client/service"
	"github.com/sapcc/archer/models"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Annotation constants of possible service annotations
const (
	// AnnotationEndpointServiceCreate Boolean indicating whether a endpoint service needs to be created.
	// Required
	AnnotationEndpointServiceCreate = "archer-create"

	// AnnotationEndpointServiceNetwork String specifying the network id for the service to be created.
	// Optional
	AnnotationEndpointServiceNetwork = "archer-network-id"

	// AnnotationEndpointServiceName String specifying the name of the endpont service resource to be created.
	// Defaults to the name of the service.
	AnnotationEndpointServiceName = "archer-service-name"

	// AnnotationEndpointServiceProxyProtocol Boolean indicating whether the TCP PROXY protocol should be enabled.
	// Defaults to "false".
	AnnotationEndpointServiceProxyProtocol = "archer-proxy-protocol"

	// AnnotationEndpointServiceVisibility String specifying the visibility of the endpoint service.
	// Defaults to "private".
	AnnotationEndpointServiceVisibility = "archer-visibility"

	// AnnotationEndpointServiceRequireApproval Boolean specifying an explicit project approval for the service
	// endpoint. Defaults to "false".
	AnnotationEndpointServiceRequireApproval = "archer-require-approval"

	// AnnotationEndpointServiceTags String specifying a space seperated list of tags. (Optional)
	AnnotationEndpointServiceTags = "archer-tags"

	// AnnotationEndpointServiceAvailabilityZone String specifying a specific availability zone. (Optional)
	AnnotationEndpointServiceAvailabilityZone = "archer-availability-zone"

	// AnnotationEndpointServicePort Integer specifying a specific service port.
	// Defaults to the first port in the service.
	AnnotationEndpointServicePort = "archer-port"
)

// ArcherServiceReconciler reconciles a service object
type ArcherServiceReconciler struct {
	client.Client
	NetworkID     strfmt.UUID
	Archer        *archerclient.Archer
	AuthInfo      runtime.ClientAuthInfoWriter
	AnnotationKey string
	Log           logr.Logger
}

// SetupWithManager sets up the controller with the Manager.
func (r *ArcherServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.AnnotationKey == "" {
		return errors.New("annotation for service resources not provided")
	}

	r.Client = mgr.GetClient()
	return ctrl.NewControllerManagedBy(mgr).
		Named("archer").
		For(&corev1.Service{}). // Service is the Application API
		Complete(r)
}

// Reconcile reconciles a service object.
func (r *ArcherServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("archer-controller", req.NamespacedName)

	var svc = new(corev1.Service)
	if err := r.Get(ctx, req.NamespacedName, svc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	createAnnotation := makeAnnotation(r.AnnotationKey, AnnotationEndpointServiceCreate)
	if !getAnnotationBoolean(createAnnotation, svc, false) {
		r.Log.V(2).Info("ignoring service with missing annotation",
			"annotation", createAnnotation)
		return ctrl.Result{}, nil
	}

	archerFinalizerName := makeAnnotation(r.AnnotationKey, "finalizer")
	if svc.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(svc, archerFinalizerName) {
			controllerutil.AddFinalizer(svc, archerFinalizerName)
			if err := r.Update(ctx, svc); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(svc, archerFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteEndpointService(ctx, svc); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}
		}

		// remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(svc, archerFinalizerName)
		if err := r.Update(ctx, svc); err != nil {
			return ctrl.Result{}, err
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Reconcile the endpoint service
	if err := r.reconcileEndpointService(ctx, svc); err != nil {
		r.Log.Error(err, "failed to reconcile service endpoint", "service", svc.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileEndpointService reconciles the endpoint service for a service object.
func (r *ArcherServiceReconciler) reconcileEndpointService(ctx context.Context, svc *corev1.Service) error {
	r.Log.Info("Reconcile service", "service", svc.Name)
	params := service.
		NewGetServiceParams().
		WithContext(ctx).
		WithTags([]string{"kubernetes", string(svc.UID)})
	res, err := r.Archer.Service.GetService(params, r.AuthInfo)
	if err != nil {
		return err
	}

	if len(res.Payload.Items) == 0 {
		return r.createEndpointService(ctx, svc)
	} else if len(res.Payload.Items) == 1 {
		return r.updateEndpointService(ctx, svc, res.Payload.Items[0])
	} else {
		return fmt.Errorf("multiple endpoint services found for service %s/%s", svc.Namespace, svc.Name)
	}
}

// getBodyFromService returns the body for a service object.
func (r *ArcherServiceReconciler) getBodyFromService(svc *corev1.Service) (*models.Service, error) {
	body := &models.Service{
		Name:        fmt.Sprintf("%s-%s", svc.Namespace, svc.Name),
		Description: fmt.Sprintf("Kubernetes service %s/%s", svc.Namespace, svc.Name),
		IPAddresses: []strfmt.IPv4{strfmt.IPv4(fmt.Sprint(svc.Spec.ClusterIP))},
		NetworkID:   &r.NetworkID,
		Tags:        []string{"kubernetes", string(svc.UID)},
		Visibility:  swag.String("public"),
	}

	if port, ok := getAnnotationString(makeAnnotation(r.AnnotationKey, AnnotationEndpointServicePort), svc); ok {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
		body.Port = int32(p)
	} else if len(svc.Spec.Ports) > 0 {
		body.Port = svc.Spec.Ports[0].Port
	} else {
		return nil, fmt.Errorf("service %s/%s has no ports", svc.Namespace, svc.Name)
	}

	if name, ok := getAnnotationString(makeAnnotation(r.AnnotationKey, AnnotationEndpointServiceName), svc); ok {
		body.Name = name
	}
	if network, ok := getAnnotationString(makeAnnotation(r.AnnotationKey, AnnotationEndpointServiceNetwork), svc); ok {
		networkID := strfmt.UUID(network)
		body.NetworkID = &networkID
	}
	if getAnnotationBoolean(makeAnnotation(r.AnnotationKey, AnnotationEndpointServiceProxyProtocol), svc, false) {
		body.ProxyProtocol = swag.Bool(true)
	}
	if getAnnotationBoolean(makeAnnotation(r.AnnotationKey, AnnotationEndpointServiceRequireApproval), svc, false) {
		body.RequireApproval = swag.Bool(true)
	}
	if tags, ok := getAnnotationString(makeAnnotation(r.AnnotationKey, AnnotationEndpointServiceTags), svc); ok {
		body.Tags = append(body.Tags, strings.Split(tags, " ")...)
	}
	if zone, ok := getAnnotationString(makeAnnotation(r.AnnotationKey, AnnotationEndpointServiceAvailabilityZone), svc); ok {
		body.AvailabilityZone = &zone
	}
	if visibility, ok := getAnnotationString(makeAnnotation(r.AnnotationKey, AnnotationEndpointServiceVisibility), svc); ok {
		body.Visibility = &visibility
	}
	return body, nil
}

// createEndpointService creates the endpoint service for a service object.
func (r *ArcherServiceReconciler) createEndpointService(ctx context.Context, svc *corev1.Service) error {
	body, err := r.getBodyFromService(svc)
	if err != nil {
		return err
	}
	params := service.
		NewPostServiceParams().
		WithContext(ctx).
		WithBody(body)

	r.Log.Info("Creating service endpoint", "service", svc.Name)
	res, err := r.Archer.Service.PostService(params, r.AuthInfo)
	if err != nil {
		return err
	}

	// Append the archer-id annotation to the service
	svc.Annotations[makeAnnotation(r.AnnotationKey, "archer-id")] = res.Payload.ID.String()
	if err := r.Update(ctx, svc); err != nil {
		return err
	}

	return nil
}

// updateEndpointService updates the endpoint service for a service object.
func (r *ArcherServiceReconciler) updateEndpointService(ctx context.Context, svc *corev1.Service, m *models.Service) error {
	body, err := r.getBodyFromService(svc)
	if err != nil {
		return err
	}

	if !serviceEqual(body, m) {
		b := &models.ServiceUpdatable{
			Description:     &body.Description,
			Enabled:         body.Enabled,
			IPAddresses:     body.IPAddresses,
			Name:            &body.Name,
			Port:            &body.Port,
			ProxyProtocol:   body.ProxyProtocol,
			RequireApproval: body.RequireApproval,
			Tags:            body.Tags,
			Visibility:      body.Visibility,
		}
		params := service.
			NewPutServiceServiceIDParams().
			WithContext(ctx).
			WithServiceID(m.ID).
			WithBody(b)
		r.Log.Info("Updating service endpoint", "service", svc.Name)
		_, err := r.Archer.Service.PutServiceServiceID(params, r.AuthInfo)
		if err != nil {
			return err
		}
	}
	return nil
}

// deleteEndpointService deletes the endpoint service for a service object.
func (r *ArcherServiceReconciler) deleteEndpointService(ctx context.Context, svc *corev1.Service) error {
	r.Log.Info("delete service endpoint", "service", svc.Name)
	id, ok := getAnnotationString(makeAnnotation(r.AnnotationKey, "id"), svc)
	if !ok {
		return nil
	}

	params := service.
		NewDeleteServiceServiceIDParams().
		WithContext(ctx).
		WithServiceID(strfmt.UUID(id))
	_, err := r.Archer.Service.DeleteServiceServiceID(params, r.AuthInfo)
	return err
}

// serviceEqual compares two service objects.
func serviceEqual(s1 *models.Service, s2 *models.Service) bool {
	if s1.Name != s2.Name {
		return false
	}
	if s1.Description != s2.Description {
		return false
	}
	if s1.Enabled != nil && *s1.Enabled != *s2.Enabled {
		return false
	}
	if len(s1.IPAddresses) != len(s2.IPAddresses) {
		return false
	}
	for _, ip := range s1.IPAddresses {
		found := false
		for _, ip2 := range s2.IPAddresses {
			if ip == ip2 || ip == strfmt.IPv4(fmt.Sprint(ip2, "/32")) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	if s1.NetworkID != nil && s2.NetworkID != nil && *s1.NetworkID != *s2.NetworkID {
		return false
	}
	if s1.Port != s2.Port {
		return false
	}
	if s1.ProxyProtocol != nil && s2.ProxyProtocol != nil && *s1.ProxyProtocol != *s2.ProxyProtocol {
		return false
	}
	if s1.RequireApproval != nil && s2.RequireApproval != nil && *s1.RequireApproval != *s2.RequireApproval {
		return false
	}
	if len(s1.Tags) != len(s2.Tags) {
		return false
	}
	for _, tag := range s1.Tags {
		found := false
		for _, tag2 := range s2.Tags {
			if tag == tag2 {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	if s1.Visibility != nil && s2.Visibility != nil && *s1.Visibility != *s2.Visibility {
		return false
	}
	return true
}
