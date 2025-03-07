/*
Copyright 2025.

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

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"reflect"
	"time"

	v1 "github.fkinternal.com/Flipkart/my-operator-sample/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	configMapSyncFinalizer = "configmapsync.apps.example.com/finalizer"
	requeueAfterError      = time.Second * 10
)

// ConfigMapSyncReconciler reconciles a ConfigMapSync object
type ConfigMapSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.example.com,resources=configmapsyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.example.com,resources=configmapsyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.example.com,resources=configmapsyncs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConfigMapSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("configmapsync", req.NamespacedName)
	logger.Info("Starting reconciliation")

	// Fetch the ConfigMapSync instance
	configMapSync := &v1.ConfigMapSync{}
	if err := r.Get(ctx, req.NamespacedName, configMapSync); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("ConfigMapSync resource not found, ignoring since object must have been deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ConfigMapSync")
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}

	// Handle deletion - check if object is being deleted
	if !configMapSync.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, configMapSync, logger)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(configMapSync, configMapSyncFinalizer) {
		logger.Info("Adding finalizer")
		controllerutil.AddFinalizer(configMapSync, configMapSyncFinalizer)
		if err := r.Update(ctx, configMapSync); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{RequeueAfter: requeueAfterError}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Fetch the source ConfigMap
	sourceConfigMap := &corev1.ConfigMap{}
	sourceConfigMapName := types.NamespacedName{
		Namespace: configMapSync.Spec.SourceNamespace,
		Name:      configMapSync.Spec.ConfigMapName,
	}
	if err := r.Get(ctx, sourceConfigMapName, sourceConfigMap); err != nil {
		if errors.IsNotFound(err) {
			return r.updateStatusWithError(ctx, configMapSync, "SourceConfigMapNotFound",
				fmt.Sprintf("Source ConfigMap %s not found in namespace %s",
					configMapSync.Spec.ConfigMapName, configMapSync.Spec.SourceNamespace))
		}
		logger.Error(err, "Failed to get source ConfigMap")
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}

	// Handle the destination ConfigMap (create or update)
	result, err := r.syncConfigMap(ctx, configMapSync, sourceConfigMap, logger)
	if err != nil {
		logger.Error(err, "Failed to sync ConfigMap")
		return result, err
	}

	// Update status to indicate successful sync
	return r.updateStatusWithSuccess(ctx, configMapSync, sourceConfigMap)
}

// syncConfigMap creates or updates the destination ConfigMap
func (r *ConfigMapSyncReconciler) syncConfigMap(
	ctx context.Context,
	configMapSync *v1.ConfigMapSync,
	sourceConfigMap *corev1.ConfigMap,
	logger logr.Logger,
) (ctrl.Result, error) {
	destinationConfigMap := &corev1.ConfigMap{}
	destinationConfigMapName := types.NamespacedName{
		Namespace: configMapSync.Spec.DestinationNamespace,
		Name:      configMapSync.Spec.ConfigMapName,
	}

	err := r.Get(ctx, destinationConfigMapName, destinationConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating ConfigMap in destination namespace",
				"Namespace", configMapSync.Spec.DestinationNamespace,
				"Name", configMapSync.Spec.ConfigMapName)

			newConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapSync.Spec.ConfigMapName,
					Namespace: configMapSync.Spec.DestinationNamespace,
				},
				Data:       sourceConfigMap.Data,
				BinaryData: sourceConfigMap.BinaryData,
			}

			if err := controllerutil.SetControllerReference(configMapSync, newConfigMap, r.Scheme); err != nil {
				logger.Error(err, "Failed to set controller reference")
				return ctrl.Result{RequeueAfter: requeueAfterError}, err
			}

			if err := r.Create(ctx, newConfigMap); err != nil {
				logger.Error(err, "Failed to create destination ConfigMap")
				return r.updateStatusWithError(ctx, configMapSync, "CreateFailed",
					fmt.Sprintf("Failed to create destination ConfigMap: %v", err))
			}

			logger.Info("Created destination ConfigMap successfully")
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Failed to get destination ConfigMap")
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}

	if !reflect.DeepEqual(destinationConfigMap.Data, sourceConfigMap.Data) ||
		!reflect.DeepEqual(destinationConfigMap.BinaryData, sourceConfigMap.BinaryData) {

		logger.Info("Updating destination ConfigMap with new data")
		destinationConfigMap.Data = sourceConfigMap.Data
		destinationConfigMap.BinaryData = sourceConfigMap.BinaryData

		if err := controllerutil.SetControllerReference(configMapSync, destinationConfigMap, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference")
			return ctrl.Result{RequeueAfter: requeueAfterError}, err
		}

		if err := r.Update(ctx, destinationConfigMap); err != nil {
			logger.Error(err, "Failed to update destination ConfigMap")
			return r.updateStatusWithError(ctx, configMapSync, "UpdateFailed",
				fmt.Sprintf("Failed to update destination ConfigMap: %v", err))
		}

		logger.Info("Updated destination ConfigMap successfully")
	} else {
		logger.Info("Destination ConfigMap is already in sync, no update needed")
	}

	return ctrl.Result{}, nil
}

func (r *ConfigMapSyncReconciler) handleDeletion(
	ctx context.Context,
	configMapSync *v1.ConfigMapSync,
	logger logr.Logger,
) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(configMapSync, configMapSyncFinalizer) {
		return ctrl.Result{}, nil
	}

	logger.Info("Processing deletion")

	logger.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(configMapSync, configMapSyncFinalizer)
	if err := r.Update(ctx, configMapSync); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}

	logger.Info("Successfully removed finalizer")
	return ctrl.Result{}, nil
}

func (r *ConfigMapSyncReconciler) updateStatusWithError(
	ctx context.Context,
	configMapSync *v1.ConfigMapSync,
	reason string,
	message string,
) (ctrl.Result, error) {
	configMapSync.Status.SyncStatus = "Failed"
	configMapSync.Status.LastSyncTime = metav1.Now()
	configMapSync.Status.Message = message
	configMapSync.Status.Reason = reason

	if err := r.Status().Update(ctx, configMapSync); err != nil {
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfterError}, fmt.Errorf(message)
}

func (r *ConfigMapSyncReconciler) updateStatusWithSuccess(
	ctx context.Context,
	configMapSync *v1.ConfigMapSync,
	sourceConfigMap *corev1.ConfigMap,
) (ctrl.Result, error) {
	configMapSync.Status.SyncStatus = "Synced"
	configMapSync.Status.LastSyncTime = metav1.Now()
	configMapSync.Status.Message = "ConfigMap successfully synced"
	configMapSync.Status.Reason = "Synced"
	configMapSync.Status.SourceGeneration = sourceConfigMap.Generation

	if err := r.Status().Update(ctx, configMapSync); err != nil {
		return ctrl.Result{RequeueAfter: requeueAfterError}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

func (r *ConfigMapSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMapSync{}).
		Owns(&corev1.ConfigMap{}). // Watch owned ConfigMaps for changes
		Complete(r)
}
