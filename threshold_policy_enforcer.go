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

const AllRule = -1 // All verifiers must succeed for a specific artifact type.

// PolicyRule defines the policy rule for ThresholdBasedEnforcer.
type PolicyRule struct {
	// VerifierThresholds holds the mapping from verifier name to the number of
	// successful verification results required to pass the policy.
	// -1 means all verifiers must succeed.
	VerifierThresholds map[string]int

	// DependentRules holds the mapping from artifactType to PolicyRule. It is
	// used to define the rules for referrer artifacts.
	DependentRules map[string]*PolicyRule
}

// state implements [EvaluationState] interface. It holds required information
// for ThresholdBasedEnforcer to make policy decision for a specific artifact.
type state struct {
	// Policy rule for evalution.
	rule *PolicyRule

	// Decision based on the artifact verification.
	artifactDecision PolicyDecision

	// Decision based on the referrer artifact verification and dependent rules.
	dependencyDecision PolicyDecision

	// verifierSuccessCount is a mapping from verifier name to the number of
	// successful verification results.
	verifierSuccessCount map[string]int

	// nestedArtifactVerifierCount is a nested map. The inner map holds mapping
	// from verifier name to the number of successful verification results. The
	// outer map holds mapping from artifactType to the inner map.
	nestedArtifactVerifierCount map[string]map[string]int
}

// CreateNextState creates the next state corresponding to the given artifact.
func (s *state) CreateNextState(artifact ocispec.Descriptor) EvaluationState {
	if s.rule == nil || len(s.rule.DependentRules) == 0 {
		return &state{
			rule: &PolicyRule{},
		}
	}
	return &state{
		rule:                        s.rule.DependentRules[artifact.ArtifactType],
		verifierSuccessCount:        make(map[string]int),
		nestedArtifactVerifierCount: make(map[string]map[string]int),
	}
}

// Decision returns the resolved decision based on the artifact and dependency
// decisions.
func (s *state) Decision() PolicyDecision {
	return resolvePolicyDecision(s.artifactDecision, s.dependencyDecision)
}

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
func resolvePolicyDecision(decision1, decision2 PolicyDecision) PolicyDecision {
	return ((decision1 & 1) | (decision2 & 1) | ((decision1 & 2) & (decision2 & 2)))
}

func (s *state) addVerifierSuccessCount(verifierName string) {
	if s.verifierSuccessCount == nil {
		s.verifierSuccessCount = make(map[string]int)
	}
	s.verifierSuccessCount[verifierName]++
}

func (s *state) getVerifierSuccessCount(verifierName string) int {
	if s.verifierSuccessCount == nil {
		return 0
	}
	return s.verifierSuccessCount[verifierName]
}

func (s *state) getNestedArtifactVerifierCount(artifactType, verifierName string) int {
	if s.nestedArtifactVerifierCount == nil || s.nestedArtifactVerifierCount[artifactType] == nil {
		return 0
	}
	return s.nestedArtifactVerifierCount[artifactType][verifierName]
}

// handleFailedDependentEval updates the dependencyDecision when a referrer
// artifact failed its policy evaluation.
func (s *state) handleFailedDependentEval(artifactType string) {
	if len(s.rule.DependentRules) == 0 || s.rule.DependentRules[artifactType] == nil{
		return
	}

	// A referrer artifact failed evaluation in 2 ways:
	// 1. its artifactDecision is denied.
	// 2. its dependencyDecision is denied, in this case, we'll regard its all
	//    verifiers as failed.
	for _, threshold := range s.rule.DependentRules[artifactType].VerifierThresholds {
		if threshold == AllRule {
			s.dependencyDecision = Deny
			return
		}
	}
}

// handleSuccessfulDependentEval updates the dependencyDecision when a referrer
// artifact passed its policy evaluation.
func (s *state) handleSuccessfulDependentEval(artifactType string, verifierCount map[string]int) {
	if len(verifierCount) == 0 || s.rule.DependentRules == nil || s.rule.DependentRules[artifactType] == nil {
		return
	}

	// Add verifierCount to nestedArtifactVerifierCount
	if s.nestedArtifactVerifierCount == nil {
		s.nestedArtifactVerifierCount = make(map[string]map[string]int)
	}
	if s.nestedArtifactVerifierCount[artifactType] == nil {
		s.nestedArtifactVerifierCount[artifactType] = make(map[string]int)
	}
	for verifierName, count := range verifierCount {
		s.nestedArtifactVerifierCount[artifactType][verifierName] += count
	}

	// check if the dependent rules are satisfied.
	allVerifierMeet := true
	for _, dependentRule := range s.rule.DependentRules {
		for verifierName, threshold := range dependentRule.VerifierThresholds {
			// a threshold is satisfied only if
			// 1. the threshold is not AllRule, and
			// 2. the verifier count meets the threshold.
			if threshold == AllRule || verifierCount[verifierName] < threshold {
				allVerifierMeet = false
				break
			}
		}
	}
	if allVerifierMeet {
		s.dependencyDecision = Allow
	}
}

// ThresholdBasedEnforcer implements PolicyEnforcer with predefined rules.
type ThresholdBasedEnforcer struct {
	// rules is a mapping from artifactType to PolicyRule.
	rules map[string]*PolicyRule
}

// NewThresholdBasedEnforcer creates a new ThresholdBasedEnforcer with the given
// policy.
func NewThresholdBasedEnforcer(policy map[string]*PolicyRule) *ThresholdBasedEnforcer {
	return &ThresholdBasedEnforcer{
		rules: policy,
	}
}

// NewEvaluationState creates a new evaluation state for the subject being
// validated by the executor.
func (e *ThresholdBasedEnforcer) NewEvaluationState() EvaluationState {
	return &state{
		rule: &PolicyRule{
			VerifierThresholds: make(map[string]int),
			DependentRules:     e.rules,
		},
		verifierSuccessCount:        make(map[string]int),
		nestedArtifactVerifierCount: make(map[string]map[string]int),
	}
}

// RequireFurtherVerification checks if further verification is required to be
// executed by verifiers for the given artifact.
func (e *ThresholdBasedEnforcer) RequireFurtherVerification(ctx context.Context, artifactReport *ValidationReport, verifier Verifier) (bool, error) {
	// 1. Check the subject evaluation result.
	// 2. Check the current artifact verifier evalution result.
	// 3. Check if no rules are defined for the current artifact type.
	// 4. Check if for this specific verifier, the verifier count is met.
	// 5. Return true if any of the above is false.

	if artifactReport.SubjectReport.PolicyEvaluationState.Decision() != Undetermined || artifactReport.PolicyEvaluationState.Decision() != Undetermined {
		return false, nil
	}
	evalState, ok := artifactReport.PolicyEvaluationState.(*state)
	if !ok {
		return false, fmt.Errorf("failed to cast PolicyEvaluationState to State")
	}
	if evalState.rule == nil || len(evalState.rule.VerifierThresholds) == 0 || evalState.rule.VerifierThresholds[verifier.Name()] == 0 {
		return false, nil
	}

	if evalState.verifierSuccessCount[verifier.Name()] >= evalState.rule.VerifierThresholds[verifier.Name()] {
		return false, nil
	}

	return true, nil
}

// EvaluateReport evaluates the report and returns the policy decision.
func (e *ThresholdBasedEnforcer) EvaluateReport(ctx context.Context, report *ValidationReport) (PolicyDecision, error) {
	decision := report.PolicyEvaluationState.Decision()
	if decision != Undetermined {
		// no need to evaluate again.
		return decision, nil
	}

	evalState, ok := report.PolicyEvaluationState.(*state)
	if !ok {
		return Undetermined, fmt.Errorf("failed to cast PolicyEvaluationState to State")
	}

	// 1. Evaluate artifact decision.
	if evalState.verifierThresholdsMet() {
		evalState.artifactDecision = Allow
	} else {
		// No need to evaluate referrer artifacts since it's already failed.
		evalState.artifactDecision = Deny
		return Deny, nil
	}

	// 2. Evaluate reports of referrer artifacts and update the state of the
	//    current report.
	evalState.nestedArtifactVerifierCount = make(map[string]map[string]int)
	artifactReports := report.ArtifactReports
	for _, artifactReport := range artifactReports {
		artifactDecision, err := e.EvaluateReport(ctx, artifactReport)
		if err != nil {
			return Undetermined, err
		}
		// do nothing on undetermined/failure decision.
		if artifactDecision != Allow || artifactReport.PolicyEvaluationState == nil {
			continue
		}
		artifactEvalState, ok := artifactReport.PolicyEvaluationState.(*state)
		if !ok {
			return Undetermined, fmt.Errorf("failed to cast PolicyEvaluationState to State")
		}
		evalState.handleSuccessfulDependentEval(artifactReport.Artifact.ArtifactType, artifactEvalState.verifierSuccessCount)
	}

	for artifactType, rule := range evalState.rule.DependentRules {
		for verifierName, threshold := range rule.VerifierThresholds {
			if threshold < 1 {
				// ALLPolicy has been evaluated while verifying and 0 does not
				// need to evaluate.
				continue
			}
			if evalState.getNestedArtifactVerifierCount(artifactType, verifierName) < threshold {
				evalState.dependencyDecision = Deny
				break
			}
		}
	}

	// 3. Calculate overall evaluation result.
	return evalState.Decision(), nil
}

func (s *state) verifierThresholdsMet() bool {
	if len(s.rule.VerifierThresholds) == 0 {
		return true
	}
	allVerifierMet := true
	for verifierName, threshold := range s.rule.VerifierThresholds {
		if s.getVerifierSuccessCount(verifierName) < threshold {
			allVerifierMet = false
			break
		}
	}
	return allVerifierMet
}

// EvaluateResult evaluates the aggregated report and a single verification
// result and returns the policy decision. It will also update subject's
// evaluation state if needed.
func (e *ThresholdBasedEnforcer) EvaluateResult(ctx context.Context, artifactReport *ValidationReport, verificationResult *VerificationResult) (PolicyDecision, error) {
	// 1. Evaluate artifact decision.
	// 2. Calculate overall evaluation result
	// 3. Update to the root report if needed.

	decision := artifactReport.PolicyEvaluationState.Decision()
	if decision != Undetermined {
		// no need to evaluate again.
		return decision, nil
	}

	// 1. Evaluate artifact decision.
	artifactEvalState, ok := artifactReport.PolicyEvaluationState.(*state)
	if !ok {
		return Undetermined, fmt.Errorf("failed to cast PolicyEvaluationState to State")
	}

	verifierName := verificationResult.Verifier.Name()
	if len(artifactEvalState.rule.VerifierThresholds) == 0 {
		// no rule for this artifact type, so we don't need to evaluate it.
		artifactEvalState.artifactDecision = Allow
	} else {
		if verificationResult.Err == nil {
			artifactEvalState.addVerifierSuccessCount(verifierName)
			if artifactEvalState.verifierThresholdsMet() {
				artifactEvalState.artifactDecision = Allow
			}
		} else {
			if artifactEvalState.rule.VerifierThresholds[verifierName] == AllRule {
				artifactEvalState.artifactDecision = Deny
			}
		}
	}

	// no need to evaluate reports of referrer artifacts.
	if len(artifactEvalState.rule.DependentRules) == 0 {
		artifactEvalState.dependencyDecision = Allow
	}

	// 2. Calculate overall evaluation result
	overallDecision := artifactEvalState.Decision()

	// 3. Update the subject report if needed recursively.
	// a) if the overall result is success or failure, update the subject report.
	// b) if the overall result is undetermined, stop.
	switch overallDecision {
	case Allow:
		return Allow, e.updateSubjectOnSuccessfulEval(ctx, artifactReport)
	case Deny:
		return Deny, e.updateSubjectOnFailedEval(ctx, artifactReport)
	default:
		return Undetermined, nil
	}
}

func (e *ThresholdBasedEnforcer) updateSubjectOnSuccessfulEval(ctx context.Context, artifactReport *ValidationReport) error {
	subjectReport := artifactReport.SubjectReport
	if subjectReport == nil || subjectReport.PolicyEvaluationState == nil {
		return nil
	}

	subjectEvalState, ok := subjectReport.PolicyEvaluationState.(*state)
	if !ok {
		return fmt.Errorf("failed to cast PolicyEvaluationState to State")
	}
	artifactEvalState, ok := artifactReport.PolicyEvaluationState.(*state)
	if !ok {
		return fmt.Errorf("failed to cast PolicyEvaluationState to State")
	}
	if subjectEvalState.Decision() != Undetermined {
		return nil
	}

	subjectEvalState.handleSuccessfulDependentEval(artifactReport.Artifact.ArtifactType, artifactEvalState.verifierSuccessCount)

	if subjectEvalState.Decision() == Allow {
		return e.updateSubjectOnSuccessfulEval(ctx, subjectReport)
	}
	return nil
}

func (e *ThresholdBasedEnforcer) updateSubjectOnFailedEval(ctx context.Context, artifactReport *ValidationReport) error {
	subjectReport := artifactReport.SubjectReport
	if subjectReport == nil || subjectReport.PolicyEvaluationState == nil {
		return nil
	}
	subjectEvalState, ok := subjectReport.PolicyEvaluationState.(*state)
	if !ok {
		return fmt.Errorf("failed to cast PolicyEvaluationState to State")
	}
	if subjectEvalState.Decision() != Undetermined {
		return nil
	}

	subjectEvalState.handleFailedDependentEval(artifactReport.Artifact.ArtifactType)

	if subjectEvalState.Decision() == Deny {
		return e.updateSubjectOnFailedEval(ctx, subjectReport)
	}
	return nil
}

// AllowEvalDuringVerify always returns true.
func (e *ThresholdBasedEnforcer) AllowEvalDuringVerify() bool {
	return true
}
