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

	// Add a couple nodes so we can test cordoner
	n1 := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node",
		},
	}
	n2 := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
		},
	}
	fakeClient.CoreV1().Nodes().Create(&n1)
	fakeClient.CoreV1().Nodes().Create(&n2)

	err := CordonNode(fakeClient, "node")
	if err != nil {
		t.Errorf("error: %v", err)
	}

	// expect "node" to be cordoned
	cordoned1, err := fakeClient.CoreV1().Nodes().Get("node", metav1.GetOptions{})
	if err != nil {
		t.Errorf("error: %v", err)
	}

	// expect "node2" to remain uncordoned
	cordoned2, err := fakeClient.CoreV1().Nodes().Get("node2", metav1.GetOptions{})
	if err != nil {
		t.Errorf("error: %v", err)
	}

	if !cordoned1.Spec.Unschedulable {
		t.Errorf("Node should be cordoned but is not")
	}

	if cordoned2.Spec.Unschedulable {
		t.Errorf("Node should not be cordoned but is")
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
