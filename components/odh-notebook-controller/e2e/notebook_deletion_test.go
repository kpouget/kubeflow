package e2e

import (
	"fmt"
	"log"
	"testing"

	nbv1 "github.com/kubeflow/kubeflow/components/notebook-controller/api/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/require"
	apiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func deletionTestSuite(t *testing.T) {
	testCtx, err := NewTestContext()
	require.NoError(t, err)
	for _, nbContext := range testCtx.testNotebooks {
		// prepend Notebook name to every subtest
		t.Run(nbContext.nbObjectMeta.Name, func(t *testing.T) {
			t.Run("Notebook Deletion", func(t *testing.T) {
				err = testCtx.testNotebookDeletion(nbContext.nbObjectMeta)
				require.NoError(t, err, "error deleting Notebook object ")
			})
			t.Run("Dependent Resource Deletion", func(t *testing.T) {
				err = testCtx.testNotebookResourcesDeletion(nbContext.nbObjectMeta)
				require.NoError(t, err, "error deleting dependent resources ")
			})
		})
	}
}

func (tc *testContext) testNotebookDeletion(nbMeta *metav1.ObjectMeta) error {
	// Delete test Notebook resource if found
	notebookLookupKey := types.NamespacedName{Name: nbMeta.Name, Namespace: nbMeta.Namespace}
	createdNotebook := &nbv1.Notebook{}

	err := tc.customClient.Get(tc.ctx, notebookLookupKey, createdNotebook)
	if err == nil {
		nberr := tc.customClient.Delete(tc.ctx, createdNotebook, &client.DeleteOptions{})
		if nberr != nil {
			return fmt.Errorf("error deleting test Notebook %s: %v", nbMeta.Name, nberr)
		}
	} else if !errors.IsNotFound(err) {
		if err != nil {
			return fmt.Errorf("error getting test Notebook instance :%v", err)
		}
	}
	return nil
}

func (tc *testContext) testNotebookResourcesDeletion(nbMeta *metav1.ObjectMeta) error {
	// Verify Notebook StatefulSet resource is deleted
	err := wait.Poll(tc.resourceRetryInterval, tc.resourceCreationTimeout, func() (done bool, err error) {
		_, err = tc.kubeClient.AppsV1().StatefulSets(tc.testNamespace).Get(tc.ctx, nbMeta.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			log.Printf("Failed to get %s statefulset", nbMeta.Name)
			return false, err

		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("unable to delete Statefulset %s :%v ", nbMeta.Name, err)
	}

	// Verify Notebook Route is deleted
	nbRouteLookupKey := types.NamespacedName{Name: nbMeta.Name, Namespace: tc.testNamespace}
	nbRoute := &routev1.Route{}
	err = wait.Poll(tc.resourceRetryInterval, tc.resourceCreationTimeout, func() (done bool, err error) {
		err = tc.customClient.Get(tc.ctx, nbRouteLookupKey, nbRoute)
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			log.Printf("Failed to get %s Route", nbMeta.Name)
			return false, err

		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("unable to delete Route %s : %v", nbMeta.Name, err)
	}
	return nil
}

func (tc *testContext) isNotebookCRD() error {
	apiextclient, err := apiext.NewForConfig(tc.cfg)
	if err != nil {
		return fmt.Errorf("error creating the apiextension client object %v", err)
	}
	_, err = apiextclient.CustomResourceDefinitions().Get(tc.ctx, "notebooks.kubeflow.org", metav1.GetOptions{})

	return err

}
