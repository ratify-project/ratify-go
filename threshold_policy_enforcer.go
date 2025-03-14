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

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const AllRule = 0 // All nested rules must succeed for this rule to be satisfied.

// PolicyRule defines the policy rule for ThresholdBasedEnforcer.
type PolicyRule struct {
	// Verifier is the Verifier to be used for this rule.
	// If set, the rule requires the Verifier to verify the corresponding
	// artifact. Optional.
	Verifier Verifier

	// Threshold is the required number of satisfied nested rules defined in
	// this rule. If not set or set to 0, all nested rules must be satisfied.
	// Optional.
	Threshold int

	// Rules hold nested rules that could be applied to the artifact or referrer
	// artifacts. Optional.
	Rules []*PolicyRule
}

// evalStats holds the statistics for a policy rule during evaluation.
type evalStats struct {
	// verifierDecision indicates the decision made by the verifier for the
	// corresponding artifact if the Verifier is set in the rule.
	verifierDecision PolicyDecision

	// ruleDecision indicates the decision made by the nested rules and
	// threshold. If there is no nested rules, this field is set to Allow by
	// default.
	ruleDecision PolicyDecision

	// skippable indicates if the evaluation can be skipped.
	skippable bool
}

// thresholdPolicyState represents the state of the threshold policy during
// evaluation.
type thresholdPolicyState struct {
	// rule is the policy rule associated with this state.
	rule *PolicyRule

	// parentState is the parent state of the current state in the policy graph.
	parentState *thresholdPolicyState

	// childStates are the child states of the current state in the policy graph.
	childStates []*thresholdPolicyState

	// stats holds the evaluation statistics for the current state.
	stats *evalStats
}

// Decision returns the decision based on the current policy state.
// truth table:
// | decision 1 | decision 2 | result |
// | ---------- | ---------- | ------ |
// | 0          | 0          | 0      |
// | 0          | 1          | 1      |
// | 0          | 2          | 0      |
// | 1          | 0          | 1      |
// | 1          | 1          | 1      |
// | 1          | 2          | 1      |
// | 2          | 0          | 0      |
// | 2          | 1          | 1      |
// | 2          | 2          | 2      |
func (s *thresholdPolicyState) Decision() PolicyDecision {
	decision1 := s.stats.verifierDecision
	decision2 := s.stats.ruleDecision
	return ((decision1 & 1) | (decision2 & 1) | ((decision1 & 2) & (decision2 & 2)))
}

// NextState fetches the state in the state graph that is applicable to the
// given artifact. It traverses the state graph starting from the current state
// and returns the first state that is applicable to the artifact.
func (s *thresholdPolicyState) NextState(artifact ocispec.Descriptor) EvaluationState {
	if s.rule.Verifier != nil {
		// If the current rule is applicable to the artifact, return the current
		// state.
		if s.rule.Verifier.Verifiable(artifact) {
			return s
		}
		// Stop traversing since the nested rules are applicable to referrer
		// artifacts.
		return nil
	}

	if len(s.childStates) == 0 {
		return nil
	}

	for _, childState := range s.childStates {
		if state := childState.NextState(artifact); state != nil {
			return state
		}
	}
	return nil
}

// requireEvaluation returns true if the current state requires more verifier
// verifications to make a decision.
func (s *thresholdPolicyState) requireEvaluation() bool {
	return !s.stats.skippable && s.stats.verifierDecision == Undetermined
}

// handleSuccessfulVerification updates the state of the policy tree when a
// a verifier succeeds in verifying an artifact. It updates all impacted states
// in the tree, including ancestors and descendants along the path through the
// current state.
func (s *thresholdPolicyState) handleSuccessfulVerification(verifier Verifier) error {
	if s.rule.Verifier == nil || s.rule.Verifier != verifier {
		return fmt.Errorf("verifier mismatch")
	}
	s.stats.verifierDecision = Allow
	if s.stats.ruleDecision == Undetermined {
		return nil
	}
	// 1. Propagate the updated decision upwards through the policy graph until 
	// reaching the root state.
	current := s.parentState
outerLoop:
	for current != nil && current.Decision() == Undetermined {
		switch current.refreshDecision() {
		case Undetermined:
			break outerLoop
		default:
			current = current.parentState
		}
	}
	// 2. propagate skippable status down to leaf states.
	s.markAsSkippable()
	return nil
}

// refreshDecision refreshes the decision of the current state when the child
// rules might be re-evaluated and returns the refreshed decision.
func (s *thresholdPolicyState) refreshDecision() PolicyDecision {
	decision := s.Decision()
	if decision != Undetermined || len(s.childStates) == 0 {
		return decision
	}

	successCount := 0
	for _, childState := range s.childStates {
		if childState.Decision() == Allow {
			successCount++
		}
	}

	if (s.rule.Threshold == AllRule && successCount == len(s.childStates)) || (s.rule.Threshold != AllRule && successCount >= s.rule.Threshold) {
		s.stats.ruleDecision = Allow
	}
	decision = s.Decision()
	if decision != Undetermined {
		s.stats.skippable = true
	}
	return decision
}

// markAsSkippable marks the current state and all child states as skippable.
func (s *thresholdPolicyState) markAsSkippable() {
	s.stats.skippable = true
	for _, childState := range s.childStates {
		childState.markAsSkippable()
	}
}

// ThresholdBasedEnforcerOptions defines the options for creating a new
// ThresholdBasedEnforcer instance.
type ThresholdBasedEnforcerOptions struct {
	// Policy is a pointer to the policy rule that defines the threshold-based
	// policy. Required.
	Policy *PolicyRule
}

// ThresholdBasedEnforcer is an implementation of the PolicyEnforcer.
type ThresholdBasedEnforcer struct {
	// policy is a pointer to the policy rule that is used for evaluation.
	policy *PolicyRule
}

// NewThresholdBasedEnforcer creates a new instance of ThresholdBasedEnforcer.
func NewThresholdBasedEnforcer(opts ThresholdBasedEnforcerOptions) (*ThresholdBasedEnforcer, error) {
	if err := validatePolicy(opts.Policy); err != nil {
		return nil, err
	}
	return &ThresholdBasedEnforcer{
		policy: opts.Policy,
	}, nil
}

func validatePolicy(policy *PolicyRule) error {
	if policy == nil {
		return fmt.Errorf("policy rule is nil")
	}
	if policy.Verifier != nil {
		return fmt.Errorf("the root rule must not have a verifier")
	}
	
	_, err := validateRule(policy)
	return err
}

func validateRule(rule *PolicyRule) ([]Verifier, error) {
	if rule == nil {
		return nil, fmt.Errorf("rule is nil")
	}

	// Validate threshold value.
	if rule.Threshold < 0 {
		return nil, fmt.Errorf("threshold must be greater than or equal to 0")
	}
	if rule.Threshold > len(rule.Rules) {
		return nil, fmt.Errorf("threshold must be less than or equal to the number of rules")
	}

	var verifiers []Verifier
	if rule.Verifier != nil {
		verifiers = append(verifiers, rule.Verifier)
	}

	// Check duplicate verifiers and threshold in nested rules.
	verifierSet := make(map[Verifier]struct{})
	for _, childRule := range rule.Rules {
		childVerifiers, err := validateRule(childRule)
		if err != nil {
			return nil, err
		}
		for _, childVerifier := range childVerifiers {
			if _, exists := verifierSet[childVerifier]; exists {
				return nil, fmt.Errorf("duplicate verifier found: %s", childVerifier.Name())
			}
			verifierSet[childVerifier] = struct{}{}
		}
	}

	if len(verifiers) == 0 {
		// Aggregate the verifiers from child rules.
		for verifier, _ := range verifierSet {
			verifiers = append(verifiers, verifier)
		}
	}
	return verifiers, nil
}

// AllowEvalDuringVerify always returns true.
func (e *ThresholdBasedEnforcer) AllowEvalDuringVerify() bool {
	return true
}

// EvaluateReport determines the final outcome of validation report.
func (e *ThresholdBasedEnforcer) EvaluateReport(_ context.Context, report *ValidationReport) (PolicyDecision, error) {
	return report.policyEvaluationState.Decision(), nil
}

// EvaluateResult aggragates the verification result into the validation report
// and updates the policy evaluation state.
func (e *ThresholdBasedEnforcer) EvaluateResult(_ context.Context, report *ValidationReport, result *VerificationResult) error {
	if report == nil || result == nil {
		return fmt.Errorf("validation report or verification result is nil")
	}
	// Only update the state if it's a successful verification.
	if result.Err != nil {
		return nil
	}

	if decision := report.policyEvaluationState.Decision(); decision != Undetermined {
		return nil
	}
	state, ok := report.policyEvaluationState.(*thresholdPolicyState)
	if !ok {
		return fmt.Errorf("invalid policy evaluation state for ThresholdBasedEnforcer")
	}

	return state.handleSuccessfulVerification(result.Verifier)
}

// RequireFurtherVerification checks if further verification is required for
// the given artifact.
func (e *ThresholdBasedEnforcer) RequireFurtherVerification(_ context.Context, artifactReport *ValidationReport, verifier Verifier) (bool, error) {
	if artifactReport.policyEvaluationState.Decision() != Undetermined {
		return false, nil
	}
	state, ok := artifactReport.policyEvaluationState.(*thresholdPolicyState)
	if !ok {
		return true, fmt.Errorf("invalid policy evaluation state")
	}

	return state.requireEvaluation(), nil
}

// NewEvaluationState creates a new evaluation state based on the policy defined
// in the ThresholdBasedEnforcer.
func (e *ThresholdBasedEnforcer) NewEvaluationState(_ context.Context) (EvaluationState, error) {
	return generateStatesFromRule(e.policy, nil)
}

// It generates a state graph from the policy rule and returns the root state.
func generateStatesFromRule(rule *PolicyRule, parentState *thresholdPolicyState) (*thresholdPolicyState, error) {
	if rule == nil {
		return nil, fmt.Errorf("rule is nil")
	}

	state := &thresholdPolicyState{
		rule: rule,
		stats: &evalStats{
			verifierDecision: initialVerifierDecision(rule),
			ruleDecision:     initialRuleDecision(rule),
		},
		parentState: parentState,
	}

	for _, childRule := range rule.Rules {
		childState, err := generateStatesFromRule(childRule, state)
		if err != nil {
			return nil, err
		}
		state.childStates = append(state.childStates, childState)
	}

	return state, nil
}

func initialVerifierDecision(rule *PolicyRule) PolicyDecision {
	if rule.Verifier == nil {
		return Allow
	}
	return Undetermined
}

func initialRuleDecision(rule *PolicyRule) PolicyDecision {
	if len(rule.Rules) == 0 {
		return Allow
	}
	return Undetermined
}
