# k8s-cluster-update-controller 

A cloud-neutral controller for Kubernetes cluster updates that takes advantage of cluster-autoscaler, pod-disruption-budgets and anti-affinity policy for zero downtime cluster updates.  

![alt text](https://github.com/lannparty/k8s-cluster-update-controller/blob/master/Controller%20Logic.png)  

Example on AWS:
Cluster manifest managed by kops.
4 Replicas  
Pod Disruption Budget: 50% - Only 2/4 pods can be inactive at any time.  
Anti-Affinity - No two pods of the same labels can be on the same node.  

1. Cluster specifies AMI A, controller looks for node labels NOT AMI A, finds nothing, repeat.  
2. Cluster specifies AMI B, controller looks for node labels NOT AMI B, finds 4 nodes. Finds first node, cordons, evicts pods on node one. Finds second node, evict pods on node 2.  
3. Finds third node, cordons, can't evict anymore pods because 50% of pods are pending in the queue.   
4. Pods in queues are spun up on AMI B.  
5. Nodes are spun down by cluster autoscaler because no pods with tag "safe-to-evict: false" running on them.  
  
Repeat  

From values.yaml the controller will ignore pods with labels specified by "exemptPodLabels". After every eviction the controller will validate pods on namespaces specified by "vitalNamespaces" are all Running before moving on.  

If the controller evicts itself it will be rescheduled to another node and continue its logic.  

# Chart

From root of repository run  
```
helm upgrade --install k8s-cluster-update-controller chart/k8s-cluster-update-controller --namespace ilmn-system -f chart/k8s-cluster-update-controller/values.yaml
```
