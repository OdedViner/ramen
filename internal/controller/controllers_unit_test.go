package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	rmn "github.com/ramendr/ramen/api/v1alpha1"
)

const (
	DRPCNameAnnotationTest      = "drplacementcontrol.ramendr.openshift.io/drpc-name"
	DRPCNamespaceAnnotationTest = "drplacementcontrol.ramendr.openshift.io/drpc-namespace"
	DRPCFinalizerTest           = "drpc.ramendr.openshift.io/finalizer"
	OCMBackupLabelKeyTest       = "cluster.open-cluster-management.io/backup"
	OCMBackupLabelValueTest     = "ramen"
)

func TestUpdateAndSetOwnerAnnotationsFinalizerOwnerReference(t *testing.T) {
	// 1. Create a test scheme and register corev1 and custom resource types.
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	assert.NoError(t, err, "Failed to add corev1 to scheme")
	err = rmn.AddToScheme(scheme)
	assert.NoError(t, err, "Failed to add rmn to scheme")

	// 2. Create test objects: DRPC and Placement (using a Pod for Placement).
	drpc := &rmn.DRPlacementControl{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-drpc",
			Namespace: "test-namespace",
		},
	}
	placementObj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-placement",
			Namespace: "test-namespace",
		},
	}

	// 3. Create a fake client with the test objects.
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(drpc, placementObj).
		Build()

	// 4. Create DRPlacementControlReconciler reconciler instance.
	reconciler := &DRPlacementControlReconciler{
		Client:    fakeClient,
		APIReader: fakeClient,
		Log:       ctrl.Log.WithName("test"),
		Scheme:    scheme,
	}

	// 5. Call updateAndSetOwner(), which should:
	//    - Add annotations to placement (DRPCNameAnnotation, DRPCNamespaceAnnotation)
	//    - Add finalizers to both DRPC and placement
	//    - Set the owner reference on DRPC pointing to placement
	//    - Add the OCM backup label to DRPC.
	updated, err := reconciler.updateAndSetOwner(context.TODO(), drpc, placementObj, reconciler.Log)
	assert.NoError(t, err, "updateAndSetOwner should not return an error")
	assert.True(t, updated, "updateAndSetOwner should indicate that an update occurred")

	// 6. Retrieve the updated DRPC.
	updatedDRPC := &rmn.DRPlacementControl{}
	err = fakeClient.Get(context.TODO(), types.NamespacedName{
		Namespace: drpc.GetNamespace(),
		Name:      drpc.GetName(),
	}, updatedDRPC)
	assert.NoError(t, err, "Failed to get updated DRPC")

	// 7. Retrieve the updated Placement.
	updatedPlacement := &corev1.Pod{}
	err = fakeClient.Get(context.TODO(), types.NamespacedName{
		Namespace: placementObj.GetNamespace(),
		Name:      placementObj.GetName(),
	}, updatedPlacement)
	assert.NoError(t, err, "Failed to get updated placement")

	// --- Verify Annotations on Placement ---
	annotations := updatedPlacement.GetAnnotations()
	assert.NotNil(t, annotations, "Placement should have annotations")
	assert.Equal(t, drpc.GetName(), annotations[DRPCNameAnnotationTest],
		"Placement should have the DRPC name annotation")
	assert.Equal(t, drpc.GetNamespace(), annotations[DRPCNamespaceAnnotationTest],
		"Placement should have the DRPC namespace annotation")

	// --- Verify Finalizers on DRPC and Placement ---
	assert.Contains(t, updatedDRPC.GetFinalizers(), DRPCFinalizerTest, "DRPC should contain the finalizer")
	assert.Contains(t, updatedPlacement.GetFinalizers(), DRPCFinalizerTest, "Placement should contain the finalizer")

	// --- Verify OwnerReference on DRPC ---
	ownerRefs := updatedDRPC.GetOwnerReferences()
	assert.NotEmpty(t, ownerRefs, "DRPC should have owner references")
	foundOwner := false
	for _, ownerRef := range ownerRefs {
		// Check that the owner reference matches the placement object's name and Kind ("Pod").
		if ownerRef.Name == placementObj.GetName() && ownerRef.Kind == "Pod" {
			foundOwner = true

			break
		}
	}
	assert.True(t, foundOwner, "DRPC should have an owner reference with the placement object's name")

	// --- Verify Label on DRPC ---
	labels := updatedDRPC.GetLabels()
	assert.NotNil(t, labels, "DRPC should have labels")
	assert.Equal(t, OCMBackupLabelValueTest, labels[OCMBackupLabelKeyTest],
		"DRPC should have the OCM backup label set correctly")
}
