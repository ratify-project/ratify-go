/*
Copyright The Ratify Authors.
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

package ratify

import (
	"context"
	"fmt"
)

const allRule = 0 // All nested rules must succeed for this rule to be allowed.

// PolicyRule defines the policy rule for [ThresholdPolicyEnforcer].
type PolicyRule struct {
	// Verifier is the Verifier to be used for this rule.
	// If set, the rule requires the Verifier to verify the corresponding
	// artifact. Either Verifier or Rules must be set.
	// Optional.
	Verifier string

	// Threshold is the required number of satisfied nested rules defined in
	// this rule. If not set or set to 0, all nested rules must be satisfied.
	// Optional.
	Threshold int

	// Rules hold nested rules that could be applied to referrer artifacts.
	// Optional.
	Rules []*PolicyRule

	// verifiers hold the verifiers that are available in the nested rules with
	// Verifier set. It's auto generated during the compilation of the policy.
	verifiers map[string]struct{}

	// ruleID is the unique ID of the rule in the policy tree. It's auto
	// generatd during the compilation of the policy.
	ruleID int
}

// evaluationNode holds the statistics for a policy rule during evaluation.
// An evaluationNode is regarded as a virtual node if it corresponds to a rule
// without Verifier set.
type evaluationNode struct {
	// rule is the policy rule for this node.
	rule *PolicyRule

	// subjectNode is the parent node of this node.
	subjectNode *evaluationNode

	// artifactDigest is the digest of the artifact being verified against in
	// this node.
	artifactDigest string

	// ruleDecision indicates the decision made by the nested rules and the
	// threshold. If there is no nested rules, this field is set to Allow by
	// default.
	ruleDecision PolicyDecision

	// childVirtualNodes is the index of the immediate virtual evaluation
	// nodes by rule ID.
	childVirtualNodes map[int]*evaluationNode

	// childNodes is the index of the immediate evaluation nodes with Verifers
	// by rule ID.
	childNodes map[int][]*evaluationNode

	// commited indicates whether the node is commited or not.
	commited bool
}

func (n *evaluationNode) addChildNode(rule *PolicyRule, artifactDigest string) *evaluationNode {
	node := &evaluationNode{
		rule:           rule,
		subjectNode:    n,
		artifactDigest: artifactDigest,
	}
	if len(rule.Rules) == 0 {
		node.ruleDecision = PolicyDecisionAllow
	} else {
		node.childNodes = make(map[int][]*evaluationNode)
		node.childVirtualNodes = make(map[int]*evaluationNode)
	}

	n.childNodes[rule.ruleID] = append(n.childNodes[rule.ruleID], node)
	return node
}

func (n *evaluationNode) addChildVirtualNode(rule *PolicyRule) *evaluationNode {
	node := &evaluationNode{
		rule:        rule,
		subjectNode: n,
	}
	node.childNodes = make(map[int][]*evaluationNode)
	node.childVirtualNodes = make(map[int]*evaluationNode)

	n.childVirtualNodes[rule.ruleID] = node
	return node
}

// refreshDecision refreshes the decision for the node and its ancestors.
func (n *evaluationNode) refreshDecision() {
	// 1. Calculate rule decision.
	if n.calculateDecision() == PolicyDecisionAllow {
		// 2. If the rule decision is allow, propagate it to ancestors.
		n.refreshAncestorsDecision()
	}
}

// calculateDecision calculates the decision for the node based on the gathered
// results.
func (n *evaluationNode) calculateDecision() PolicyDecision {
	if n.ruleDecision != PolicyDecisionUndetermined {
		return n.ruleDecision
	}

	successfulRuleCount := 0
	for _, nodes := range n.childNodes {
		// validate each rule
		for _, node := range nodes {
			if node.ruleDecision == PolicyDecisionAllow {
				successfulRuleCount++
				break
			}
		}
	}
	for _, node := range n.childVirtualNodes {
		if node.ruleDecision == PolicyDecisionAllow {
			successfulRuleCount++
		}
	}

	threshold := n.rule.Threshold
	if threshold == allRule {
		threshold = len(n.rule.Rules)
	}
	if successfulRuleCount >= threshold {
		n.ruleDecision = PolicyDecisionAllow
	}
	if n.isCommited() {
		n.ruleDecision = PolicyDecisionDeny
	}
	return n.ruleDecision
}

// isCommited checks if the node or any of its ancestors are commited.
// In single goroutine mode, we only need to check the commited field.
// In multi goroutine mode, we need to track referrers listed before committed.
func (n *evaluationNode) isCommited() bool {
	node := n
	for node != nil {
		if node.commited {
			return true
		}
		node = node.subjectNode
	}
	return false
}

// refreshAncestorsDecision refreshes the decision for all ancestors.
func (n *evaluationNode) refreshAncestorsDecision() {
	node := n.subjectNode
	for node != nil {
		if node.ruleDecision != PolicyDecisionUndetermined {
			break
		}
		// only propagate allow decision to ancestors.
		if node.calculateDecision() != PolicyDecisionAllow {
			break
		}
		node = node.subjectNode
	}
}

// A node is finalized if either it's determined or any of its ancestors is
// determined.
func (n *evaluationNode) finalized() bool {
	node := n
	for node != nil {
		if node.ruleDecision != PolicyDecisionUndetermined {
			return true
		}
		node = node.subjectNode
	}
	return false
}

// verifiable checks if the verifier is required in the rule.
func (n *evaluationNode) verifiable(verifier string) bool {
	_, ok := n.rule.verifiers[verifier]
	return ok
}

// thresholdEvaluator represents the state of the threshold policy during
// evaluation.
type thresholdEvaluator struct {
	// evalGraph is the root node of the evaluation graph.
	evalGraph *evaluationNode

	// subjectIndex is the index of the evaluation nodes by subject digest.
	subjectIndex map[string][]*evaluationNode

	// verifierIndex is the index of the evaluation nodes by concatenating subject
	// digest, artifact digest and verifier.
	verifierIndex map[string][]*evaluationNode
}

// verifierIndexKey generates the key for the verifier index.
func verifierIndexKey(subjectDigest, artifactDigest, verifier string) string {
	return fmt.Sprintf("%s:%s:%s", subjectDigest, artifactDigest, verifier)
}

// Pruned checks if whether the verifier is required to verify the subject
// against the artifact.
func (e *thresholdEvaluator) Pruned(ctx context.Context, subjectDigest, artifactDigest, verifier string) (PrunedState, error) {
	if _, ok := e.verifierIndex[verifierIndexKey(subjectDigest, artifactDigest, verifier)]; ok {
		return PrunedStateVerifierPruned, nil
	}
	nodes, ok := e.subjectIndex[subjectDigest]
	if !ok {
		return PrunedStateNone, fmt.Errorf("no applicable policy rule defined for the subject %s", subjectDigest)
	}

	// Return PrunedStateSubjectPruned if all nodes are finalized.
	// Otherwise, return PrunedStateVerifierPruned if all nodes don't require
	// the verifier.
	// Otherwise, return PrunedStateNone.
	subjectPruned := true
	verifierPruned := true
	for _, node := range nodes {
		if node.finalized() {
			continue
		}
		subjectPruned = false
		if node.verifiable(verifier) {
			verifierPruned = false
			break
		}
	}
	if subjectPruned {
		return PrunedStateSubjectPruned, nil
	}
	if verifierPruned {
		return PrunedStateVerifierPruned, nil
	}
	return PrunedStateNone, nil
}

// AddResult adds the successful verification result of the subject against the
// artifact to the evaluator for further evaluation.
func (e *thresholdEvaluator) AddResult(ctx context.Context, subjectDigest, artifactDigest string, artifactResult *VerificationResult) error {
	if artifactResult.Err != nil {
		// Only add successful verification result to the evaluator.
		return nil
	}

	nodes, err := e.createEvaluationNodes(subjectDigest, artifactDigest, artifactResult.Verifier.Name())
	if err != nil {
		return err
	}

	for _, node := range nodes {
		node.refreshDecision()
	}
	return nil
}

// createEvaluationNodes creates new evaluation nodes for the given subject,
// artifact and verifier.
// Note that multiple nodes may be created in terms of the policy rules.
func (e *thresholdEvaluator) createEvaluationNodes(subjectDigest, artifactDigest, verifier string) ([]*evaluationNode, error) {
	// Find subject nodes for the given subject digest.
	subjectNodes, ok := e.subjectIndex[subjectDigest]
	if !ok {
		return nil, fmt.Errorf("cannot find evaluation node for subject %s", subjectDigest)
	}

	// create new nodes for the combination of subject, artifact and verifier.
	var nodes []*evaluationNode
	for _, subjectNode := range subjectNodes {
		newNodes := e.createEvaluationNodesForSubject(subjectDigest, artifactDigest, verifier, subjectNode)
		if len(newNodes) > 0 {
			nodes = append(nodes, newNodes...)
		}
	}
	return nodes, nil
}

func (e *thresholdEvaluator) createEvaluationNodesForSubject(subjectDigest, artifactDigest, verifier string, subjectNode *evaluationNode) []*evaluationNode {
	var nodes []*evaluationNode
	for _, rule := range subjectNode.rule.Rules {
		if rule.Verifier != "" {
			nodes = e.createEvaluationNode(rule, subjectDigest, artifactDigest, verifier, subjectNode, nodes)
		} else {
			nodes = e.createVirtualEvaluationNode(rule, subjectDigest, artifactDigest, verifier, subjectNode, nodes)
		}
	}
	return nodes
}

func (e *thresholdEvaluator) createEvaluationNode(rule *PolicyRule, subjectDigest, artifactDigest, verifier string, subjectNode *evaluationNode, nodes []*evaluationNode) []*evaluationNode {
	if verifier != rule.Verifier {
		return nodes
	}

	node := subjectNode.addChildNode(rule, artifactDigest)
	nodes = append(nodes, node)

	verifierIndexKey := verifierIndexKey(subjectDigest, artifactDigest, verifier)
	e.verifierIndex[verifierIndexKey] = append(e.verifierIndex[verifierIndexKey], node)
	e.subjectIndex[artifactDigest] = append(e.subjectIndex[artifactDigest], node)

	return nodes
}

func (e *thresholdEvaluator) createVirtualEvaluationNode(rule *PolicyRule, subjectDigest, artifactDigest, verifier string, subjectNode *evaluationNode, nodes []*evaluationNode) []*evaluationNode {
	if _, ok := rule.verifiers[verifier]; !ok {
		return nodes
	}

	node, ok := subjectNode.childVirtualNodes[rule.ruleID]
	if !ok {
		node = subjectNode.addChildVirtualNode(rule)
	}

	newNodes := e.createEvaluationNodesForSubject(subjectDigest, artifactDigest, verifier, node)
	if len(newNodes) > 0 {
		nodes = append(nodes, newNodes...)
	}
	return nodes
}

// Commit marks related nodes as commited.
// In single goroutine mode, refresh commited nodes to calculate the decision.
// In multi goroutine mode, commited node cannot be refreshed as it may need
// more verification results to make a decision.
func (e *thresholdEvaluator) Commit(ctx context.Context, subjectDigest string) error {
	nodes, ok := e.subjectIndex[subjectDigest]
	if !ok {
		return fmt.Errorf("subject: %s has not been processed yet", subjectDigest)
	}
	for _, node := range nodes {
		node.commited = true
		node.refreshDecision()
	}
	return nil
}

// Evaluate makes the final decision based on aggregated evaluation graph.
func (e *thresholdEvaluator) Evaluate(ctx context.Context) (bool, error) {
	// Refresh the decision for the root node.
	e.evalGraph.refreshDecision()
	return e.evalGraph.ruleDecision == PolicyDecisionAllow, nil
}

// ThresholdPolicyEnforcer is an implementation of the PolicyEnforcer interface.
type ThresholdPolicyEnforcer struct {
	policy *PolicyRule
}

// NewThresholdPolicyEnforcer creates a new ThresholdPolicyEnforcer with the
// given policy rule. The policy rule must be non-nil.
func NewThresholdPolicyEnforcer(policy *PolicyRule) (*ThresholdPolicyEnforcer, error) {
	if err := validatePolicy(policy); err != nil {
		return nil, err
	}
	
	return &ThresholdPolicyEnforcer{
		policy: compilePolicy(policy),
	}, nil
}

func validatePolicy(policy *PolicyRule) error {
	if policy == nil {
		return fmt.Errorf("policy rule is nil")
	}
	if policy.Verifier != "" {
		return fmt.Errorf("the root rule must not have a verifier name")
	}

	return validateRule(policy)
}

func validateRule(rule *PolicyRule) error {
	if rule == nil {
		return fmt.Errorf("rule is nil")
	}

	// Validate threshold value.
	if rule.Threshold < 0 {
		return fmt.Errorf("threshold must be greater than or equal to 0")
	}
	if rule.Threshold > len(rule.Rules) {
		return fmt.Errorf("threshold must be less than or equal to the number of rules")
	}

	if rule.Verifier == "" && len(rule.Rules) == 0 {
		return fmt.Errorf("rule must have at least one nested rule if no verifier name is set")
	}

	for _, childRule := range rule.Rules {
		if err := validateRule(childRule); err != nil {
			return err
		}
	}
	return nil
}

// compilePolicy compiles the plain policy into a policy that can be used for
// evaluation directly.
func compilePolicy(policy *PolicyRule) *PolicyRule {
	initRuleID := 1
	_ = updateRules(policy, &initRuleID)
	return policy
}

func updateRules(policy *PolicyRule, ruleID *int) map[string]struct{} {
	policy.ruleID = *ruleID
	*ruleID++

	verifiers := make(map[string]struct{})
	for _, childRule := range policy.Rules {
		childVerifiers := updateRules(childRule, ruleID)
		for verifier := range childVerifiers {
			verifiers[verifier] = struct{}{}
		}
	}
	policy.verifiers = verifiers

	if policy.Verifier != "" {
		return map[string]struct{}{policy.Verifier: {}}
	}
	return verifiers
}

// Evaluator returns a thresholdEvaluator for the given subject digest.
func (e *ThresholdPolicyEnforcer) Evaluator(ctx context.Context, subjectDigest string) (Evaluator, error) {
	if e.policy == nil {
		return nil, fmt.Errorf("policy is nil")
	}

	rootNode := &evaluationNode{
		rule:              e.policy,
		artifactDigest:    subjectDigest,
		childNodes:        make(map[int][]*evaluationNode),
		childVirtualNodes: make(map[int]*evaluationNode),
	}

	return &thresholdEvaluator{
		subjectIndex: map[string][]*evaluationNode{
			subjectDigest: {rootNode},
		},
		verifierIndex: make(map[string][]*evaluationNode),
		evalGraph:     rootNode,
	}, nil
}
