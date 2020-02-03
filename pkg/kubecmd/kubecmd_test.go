package kubecmd

import (
	"os"
	"testing"

	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// "k8s.io/kubernetes/pkg/api"
	// "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCordonNode(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()

	err := CordonNode(fakeClient, "node")
	if err != nil {
		t.Errorf("error: %v", err)
	}
}

func TestEvictPodsOnCordonedNodes(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	podClient := fakeClient.CoreV1().Pods(apiv1.NamespaceDefault)

	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:  "web",
					Image: "nginx:1.12",
				},
			},
		},
	}
	podClient.Create(testPod)

	// Test pod with specific label value
	// TODO We should test the key or both key/value of the label
	os.Setenv("EXEMPTPODLABELS", "some-value")
	labels := map[string]string{
		"KEY": "some-value",
	}
	labeledPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-envs-pod",
			Labels: labels,
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:  "web",
					Image: "nginx:1.12",
				},
			},
		},
	}
	podClient.Create(labeledPod)

	// Test ownerReference
	controller := true
	ownedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-daemonset-pod",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "CoreV1",
					Kind:       "DaemonSet",
					Controller: &controller,
				},
			},
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:  "web",
					Image: "nginx:1.12",
				},
			},
		},
	}
	podClient.Create(ownedPod)

	err := EvictPodsOnCordonedNodes(fakeClient, "cordonedNodeName", "policyGroupVersion")
	if err != nil {
		t.Errorf("error: %v", err)
	}
}

func TestValidateNamespaces(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	namespaces := []string{"default"}
	validateTrue := ValidateNamespaces(fakeClient, namespaces)
	if validateTrue != true {
		t.Errorf("Namespace should have been validated, but returned false")
	}

	podClient := fakeClient.CoreV1().Pods(apiv1.NamespaceDefault)

	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:  "web",
					Image: "nginx:1.12",
				},
			},
		},
	}
	podClient.Create(testPod)

	validateFalse := ValidateNamespaces(fakeClient, namespaces)
	if validateFalse == true {
		t.Errorf("Namespace should not have validated, but returned true")
	}

}
