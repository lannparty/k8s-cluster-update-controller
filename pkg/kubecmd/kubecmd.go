package kubecmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

func CordonNode(clientset kubernetes.Interface, nodeName string) error {
	nodeClient := clientset.CoreV1().Nodes()
	type patchUInt32Value struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value bool   `json:"value"`
	}
	payload := []patchUInt32Value{{
		Op:    "replace",
		Path:  "/spec/unschedulable",
		Value: true,
	}}
	payloadBytes, _ := json.Marshal(payload)
	nodeClient.Patch(nodeName, types.JSONPatchType, payloadBytes)
	return nil
}

func checkExemptLabels(podGet *v1.Pod, exemptLabel string) bool {
	for _, j := range podGet.Labels {
		if strings.Contains(exemptLabel, j) {
			return true
		}
	}
	return false
}

func EvictPodsOnCordonedNodes(clientset kubernetes.Interface, cordonedNodeName string, policyGroupVersion string) error {
	listOptionsModifier := metav1.ListOptions{FieldSelector: "spec.nodeName=" + cordonedNodeName}
	podList, err := clientset.CoreV1().Pods(v1.NamespaceAll).List(listOptionsModifier)
	if err != nil {
		klog.Errorf("List pods on cordoned nodes failed with error: %v, %v\n", podList, err)
		return err
	}

	for i, j := range podList.Items {
		podGet, err := clientset.CoreV1().Pods(podList.Items[i].Namespace).Get(podList.Items[i].Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if len(podGet.OwnerReferences) != 0 {
			if podGet.OwnerReferences[0].Kind == "DaemonSet" {
				fmt.Printf("Skipping pod %v because it's a Daemonset.\n", podGet.Name)
				continue
			}
		} else if checkExemptLabels(podGet, os.Getenv("EXEMPTPODLABELS")) == true {
			fmt.Printf("Skipping eviction of %s because it has one of the exempt labels %v\n", podGet.Name, os.Getenv("EXEMPTPODLABELS"))
			continue
		}
		podEviction := &policyv1beta1.Eviction{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policyGroupVersion,
				Kind:       "Eviction",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      j.Name,
				Namespace: j.Namespace,
			},
		}
		fmt.Printf("Evicting %v\n", podList.Items[i].Name)
		err = clientset.CoreV1().Pods(j.Namespace).Evict(podEviction)
		if err != nil {
			klog.Errorf("Eviction of pod %s failed with error %v\n", podList.Items[i].Name, err)
			return err
		}
	}
	return nil
}

func ValidateNamespaces(clientset kubernetes.Interface, namespaces []string) bool {
	for _, j := range namespaces {
		optionsModifier := metav1.ListOptions{FieldSelector: "status.phase!=Running"}
		podList, _ := clientset.CoreV1().Pods(j).List(optionsModifier)
		if len(podList.Items) != 0 {
			for _, j := range podList.Items {
				fmt.Println(j.Name)
			}
			return false
		}
	}
	return true
}
