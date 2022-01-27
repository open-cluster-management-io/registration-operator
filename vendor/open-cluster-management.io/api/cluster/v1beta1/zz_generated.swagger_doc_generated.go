package v1beta1

// This file contains a collection of methods that can be used from go-restful to
// generate Swagger API documentation for its models. Please read this PR for more
// information on the implementation: https://github.com/emicklei/go-restful/pull/215
//
// TODOs are ignored from the parser (e.g. TODO(andronat):... || TODO:...) if and only if
// they are on one line! For multiple line or blocks that you want to ignore use ---.
// Any context after a --- is ignored.
//
// Those methods can be generated by using hack/update-swagger-docs.sh

// AUTO-GENERATED FUNCTIONS START HERE
var map_ManagedClusterSet = map[string]string{
	"":       "ManagedClusterSet defines a group of ManagedClusters that user's workload can run on. A workload can be defined to deployed on a ManagedClusterSet, which mean:\n  1. The workload can run on any ManagedCluster in the ManagedClusterSet\n  2. The workload cannot run on any ManagedCluster outside the ManagedClusterSet\n  3. The service exposed by the workload can be shared in any ManagedCluster in the ManagedClusterSet\n\nIn order to assign a ManagedCluster to a certian ManagedClusterSet, add a label with name `cluster.open-cluster-management.io/clusterset` on the ManagedCluster to refers to the ManagedClusterSet. User is not allow to add/remove this label on a ManagedCluster unless they have a RBAC rule to CREATE on a virtual subresource of managedclustersets/join. In order to update this label, user must have the permission on both the old and new ManagedClusterSet.",
	"spec":   "Spec defines the attributes of the ManagedClusterSet",
	"status": "Status represents the current status of the ManagedClusterSet",
}

func (ManagedClusterSet) SwaggerDoc() map[string]string {
	return map_ManagedClusterSet
}

var map_ManagedClusterSetList = map[string]string{
	"":         "ManagedClusterSetList is a collection of ManagedClusterSet.",
	"metadata": "Standard list metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
	"items":    "Items is a list of ManagedClusterSet.",
}

func (ManagedClusterSetList) SwaggerDoc() map[string]string {
	return map_ManagedClusterSetList
}

var map_ManagedClusterSetSpec = map[string]string{
	"": "ManagedClusterSetSpec describes the attributes of the ManagedClusterSet",
}

func (ManagedClusterSetSpec) SwaggerDoc() map[string]string {
	return map_ManagedClusterSetSpec
}

var map_ManagedClusterSetStatus = map[string]string{
	"":           "ManagedClusterSetStatus represents the current status of the ManagedClusterSet.",
	"conditions": "Conditions contains the different condition statuses for this ManagedClusterSet.",
}

func (ManagedClusterSetStatus) SwaggerDoc() map[string]string {
	return map_ManagedClusterSetStatus
}

var map_ManagedClusterSetBinding = map[string]string{
	"":     "ManagedClusterSetBinding projects a ManagedClusterSet into a certain namespace. User is able to create a ManagedClusterSetBinding in a namespace and bind it to a ManagedClusterSet if they have an RBAC rule to CREATE on the virtual subresource of managedclustersets/bind. Workloads created in the same namespace can only be distributed to ManagedClusters in ManagedClusterSets bound in this namespace by higher level controllers.",
	"spec": "Spec defines the attributes of ManagedClusterSetBinding.",
}

func (ManagedClusterSetBinding) SwaggerDoc() map[string]string {
	return map_ManagedClusterSetBinding
}

var map_ManagedClusterSetBindingList = map[string]string{
	"":         "ManagedClusterSetBindingList is a collection of ManagedClusterSetBinding.",
	"metadata": "Standard list metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
	"items":    "Items is a list of ManagedClusterSetBinding.",
}

func (ManagedClusterSetBindingList) SwaggerDoc() map[string]string {
	return map_ManagedClusterSetBindingList
}

var map_ManagedClusterSetBindingSpec = map[string]string{
	"":           "ManagedClusterSetBindingSpec defines the attributes of ManagedClusterSetBinding.",
	"clusterSet": "ClusterSet is the name of the ManagedClusterSet to bind. It must match the instance name of the ManagedClusterSetBinding and cannot change once created. User is allowed to set this field if they have an RBAC rule to CREATE on the virtual subresource of managedclustersets/bind.",
}

func (ManagedClusterSetBindingSpec) SwaggerDoc() map[string]string {
	return map_ManagedClusterSetBindingSpec
}

var map_AddOnScore = map[string]string{
	"":             "AddOnScore represents the configuration of the addon score source.",
	"resourceName": "ResourceName defines the resource name of the AddOnPlacementScore. The placement prioritizer selects AddOnPlacementScore CR by this name.",
	"scoreName":    "ScoreName defines the score name inside AddOnPlacementScore. AddOnPlacementScore contains a list of score name and score value, ScoreName specify the score to be used by the prioritizer.",
}

func (AddOnScore) SwaggerDoc() map[string]string {
	return map_AddOnScore
}

var map_ClusterClaimSelector = map[string]string{
	"":                 "ClusterClaimSelector is a claim query over a set of ManagedClusters. An empty cluster claim selector matches all objects. A null cluster claim selector matches no objects.",
	"matchExpressions": "matchExpressions is a list of cluster claim selector requirements. The requirements are ANDed.",
}

func (ClusterClaimSelector) SwaggerDoc() map[string]string {
	return map_ClusterClaimSelector
}

var map_ClusterPredicate = map[string]string{
	"":                        "ClusterPredicate represents a predicate to select ManagedClusters.",
	"requiredClusterSelector": "RequiredClusterSelector represents a selector of ManagedClusters by label and claim. If specified, 1) Any ManagedCluster, which does not match the selector, should not be selected by this ClusterPredicate; 2) If a selected ManagedCluster (of this ClusterPredicate) ceases to match the selector (e.g. due to\n   an update) of any ClusterPredicate, it will be eventually removed from the placement decisions;\n3) If a ManagedCluster (not selected previously) starts to match the selector, it will either\n   be selected or at least has a chance to be selected (when NumberOfClusters is specified);",
}

func (ClusterPredicate) SwaggerDoc() map[string]string {
	return map_ClusterPredicate
}

var map_ClusterSelector = map[string]string{
	"":              "ClusterSelector represents the AND of the containing selectors. An empty cluster selector matches all objects. A null cluster selector matches no objects.",
	"labelSelector": "LabelSelector represents a selector of ManagedClusters by label",
	"claimSelector": "ClaimSelector represents a selector of ManagedClusters by clusterClaims in status",
}

func (ClusterSelector) SwaggerDoc() map[string]string {
	return map_ClusterSelector
}

var map_Placement = map[string]string{
	"":       "Placement defines a rule to select a set of ManagedClusters from the ManagedClusterSets bound to the placement namespace.\n\nHere is how the placement policy combines with other selection methods to determine a matching list of ManagedClusters: 1) Kubernetes clusters are registered with hub as cluster-scoped ManagedClusters; 2) ManagedClusters are organized into cluster-scoped ManagedClusterSets; 3) ManagedClusterSets are bound to workload namespaces; 4) Namespace-scoped Placements specify a slice of ManagedClusterSets which select a working set\n   of potential ManagedClusters;\n5) Then Placements subselect from that working set using label/claim selection.\n\nNo ManagedCluster will be selected if no ManagedClusterSet is bound to the placement namespace. User is able to bind a ManagedClusterSet to a namespace by creating a ManagedClusterSetBinding in that namespace if they have a RBAC rule to CREATE on the virtual subresource of `managedclustersets/bind`.\n\nA slice of PlacementDecisions with label cluster.open-cluster-management.io/placement={placement name} will be created to represent the ManagedClusters selected by this placement.\n\nIf a ManagedCluster is selected and added into the PlacementDecisions, other components may apply workload on it; once it is removed from the PlacementDecisions, the workload applied on this ManagedCluster should be evicted accordingly.",
	"spec":   "Spec defines the attributes of Placement.",
	"status": "Status represents the current status of the Placement",
}

func (Placement) SwaggerDoc() map[string]string {
	return map_Placement
}

var map_PlacementList = map[string]string{
	"":         "PlacementList is a collection of Placements.",
	"metadata": "Standard list metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
	"items":    "Items is a list of Placements.",
}

func (PlacementList) SwaggerDoc() map[string]string {
	return map_PlacementList
}

var map_PlacementSpec = map[string]string{
	"":                  "PlacementSpec defines the attributes of Placement. An empty PlacementSpec selects all ManagedClusters from the ManagedClusterSets bound to the placement namespace. The containing fields are ANDed.",
	"clusterSets":       "ClusterSets represent the ManagedClusterSets from which the ManagedClusters are selected. If the slice is empty, ManagedClusters will be selected from the ManagedClusterSets bound to the placement namespace, otherwise ManagedClusters will be selected from the intersection of this slice and the ManagedClusterSets bound to the placement namespace.",
	"numberOfClusters":  "NumberOfClusters represents the desired number of ManagedClusters to be selected which meet the placement requirements. 1) If not specified, all ManagedClusters which meet the placement requirements (including ClusterSets,\n   and Predicates) will be selected;\n2) Otherwise if the nubmer of ManagedClusters meet the placement requirements is larger than\n   NumberOfClusters, a random subset with desired number of ManagedClusters will be selected;\n3) If the nubmer of ManagedClusters meet the placement requirements is equal to NumberOfClusters,\n   all of them will be selected;\n4) If the nubmer of ManagedClusters meet the placement requirements is less than NumberOfClusters,\n   all of them will be selected, and the status of condition `PlacementConditionSatisfied` will be\n   set to false;",
	"predicates":        "Predicates represent a slice of predicates to select ManagedClusters. The predicates are ORed.",
	"prioritizerPolicy": "PrioritizerPolicy defines the policy of the prioritizers. If this field is unset, then default prioritizer mode and configurations are used. Referring to PrioritizerPolicy to see more description about Mode and Configurations.",
	"tolerations":       "Tolerations are applied to placements, and allow (but do not require) the managed clusters with certain taints to be selected by placements with matching tolerations.",
}

func (PlacementSpec) SwaggerDoc() map[string]string {
	return map_PlacementSpec
}

var map_PlacementStatus = map[string]string{
	"numberOfSelectedClusters": "NumberOfSelectedClusters represents the number of selected ManagedClusters",
	"conditions":               "Conditions contains the different condition status for this Placement.",
}

func (PlacementStatus) SwaggerDoc() map[string]string {
	return map_PlacementStatus
}

var map_PrioritizerConfig = map[string]string{
	"":                "PrioritizerConfig represents the configuration of prioritizer",
	"scoreCoordinate": "ScoreCoordinate represents the configuration of the prioritizer and score source.",
	"weight":          "Weight defines the weight of the prioritizer score. The value must be ranged in [-10,10]. Each prioritizer will calculate an integer score of a cluster in the range of [-100, 100]. The final score of a cluster will be sum(weight * prioritizer_score). A higher weight indicates that the prioritizer weights more in the cluster selection, while 0 weight indicates that the prioritizer is disabled. A negative weight indicates wants to select the last ones.",
}

func (PrioritizerConfig) SwaggerDoc() map[string]string {
	return map_PrioritizerConfig
}

var map_PrioritizerPolicy = map[string]string{
	"":     "PrioritizerPolicy represents the policy of prioritizer",
	"mode": "Mode is either Exact, Additive, \"\" where \"\" is Additive by default. In Additive mode, any prioritizer not explicitly enumerated is enabled in its default Configurations, in which Steady and Balance prioritizers have the weight of 1 while other prioritizers have the weight of 0. Additive doesn't require configuring all prioritizers. The default Configurations may change in the future, and additional prioritization will happen. In Exact mode, any prioritizer not explicitly enumerated is weighted as zero. Exact requires knowing the full set of prioritizers you want, but avoids behavior changes between releases.",
}

func (PrioritizerPolicy) SwaggerDoc() map[string]string {
	return map_PrioritizerPolicy
}

var map_ScoreCoordinate = map[string]string{
	"":        "ScoreCoordinate represents the configuration of the score type and score source",
	"type":    "Type defines the type of the prioritizer score. Type is either \"BuiltIn\", \"AddOn\" or \"\", where \"\" is \"BuiltIn\" by default. When the type is \"BuiltIn\", need to specify a BuiltIn prioritizer name in BuiltIn. When the type is \"AddOn\", need to configure the score source in AddOn.",
	"builtIn": "BuiltIn defines the name of a BuiltIn prioritizer. Below are the valid BuiltIn prioritizer names. 1) Balance: balance the decisions among the clusters. 2) Steady: ensure the existing decision is stabilized. 3) ResourceAllocatableCPU & ResourceAllocatableMemory: sort clusters based on the allocatable.",
	"addOn":   "When type is \"AddOn\", AddOn defines the resource name and score name.",
}

func (ScoreCoordinate) SwaggerDoc() map[string]string {
	return map_ScoreCoordinate
}

var map_Toleration = map[string]string{
	"":                  "Toleration represents the toleration object that can be attached to a placement. The placement this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.",
	"key":               "Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.",
	"operator":          "Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a placement can tolerate all taints of a particular category.",
	"value":             "Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.",
	"effect":            "Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSelect, PreferNoSelect and NoSelectIfNew.",
	"tolerationSeconds": "TolerationSeconds represents the period of time the toleration (which must be of effect NoSelect/PreferNoSelect, otherwise this field is ignored) tolerates the taint. The default value is nil, which indicates it tolerates the taint forever. The start time of counting the TolerationSeconds should be the TimeAdded in Taint, not the cluster scheduled time or TolerationSeconds added time.",
}

func (Toleration) SwaggerDoc() map[string]string {
	return map_Toleration
}

var map_ClusterDecision = map[string]string{
	"":            "ClusterDecision represents a decision from a placement An empty ClusterDecision indicates it is not scheduled yet.",
	"clusterName": "ClusterName is the name of the ManagedCluster. If it is not empty, its value should be unique cross all placement decisions for the Placement.",
	"reason":      "Reason represents the reason why the ManagedCluster is selected.",
}

func (ClusterDecision) SwaggerDoc() map[string]string {
	return map_ClusterDecision
}

var map_PlacementDecision = map[string]string{
	"":       "PlacementDecision indicates a decision from a placement PlacementDecision should has a label cluster.open-cluster-management.io/placement={placement name} to reference a certain placement.\n\nIf a placement has spec.numberOfClusters specified, the total number of decisions contained in status.decisions of PlacementDecisions should always be NumberOfClusters; otherwise, the total number of decisions should be the number of ManagedClusters which match the placement requirements.\n\nSome of the decisions might be empty when there are no enough ManagedClusters meet the placement requirements.",
	"status": "Status represents the current status of the PlacementDecision",
}

func (PlacementDecision) SwaggerDoc() map[string]string {
	return map_PlacementDecision
}

var map_PlacementDecisionList = map[string]string{
	"":         "ClusterDecisionList is a collection of PlacementDecision.",
	"metadata": "Standard list metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
	"items":    "Items is a list of PlacementDecision.",
}

func (PlacementDecisionList) SwaggerDoc() map[string]string {
	return map_PlacementDecisionList
}

var map_PlacementDecisionStatus = map[string]string{
	"":          "PlacementDecisionStatus represents the current status of the PlacementDecision.",
	"decisions": "Decisions is a slice of decisions according to a placement The number of decisions should not be larger than 100",
}

func (PlacementDecisionStatus) SwaggerDoc() map[string]string {
	return map_PlacementDecisionStatus
}

// AUTO-GENERATED FUNCTIONS END HERE
