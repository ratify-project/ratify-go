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
	"errors"
	"reflect"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	artifactType1 = "artifact-type-1"
	notationType  = "application/vnd.cncf.notary.signature"
	cosignType    = "application/vnd.dev.cosign.artifact.sig.v1+json"
	sbomType      = "application/vnd.cyclonedx+json"
)

// mockPolicyVerifier implements Verifier interface for testing purposes.
type mockPolicyVerifier struct {
	name         string
	artifactType string
}

func (m *mockPolicyVerifier) Name() string {
	return m.name
}

func (m *mockPolicyVerifier) Type() string {
	return "mock-verifier-type"
}

func (m *mockPolicyVerifier) Verify(_ context.Context, _ *VerifyOptions) (*VerificationResult, error) {
	return nil, nil
}

func (m *mockPolicyVerifier) Verifiable(artifact ocispec.Descriptor) bool {
	return artifact.ArtifactType == m.artifactType
}

func TestDecision(t *testing.T) {
	tests := []struct {
		name     string
		stats    evalStats
		expected PolicyDecision
	}{
		{
			name: "Both decisions are Allow",
			stats: evalStats{
				verifierDecision: Allow,
				ruleDecision:     Allow,
			},
			expected: Allow,
		},
		{
			name: "Verifier decision is Allow, rule decision is Deny",
			stats: evalStats{
				verifierDecision: Allow,
				ruleDecision:     Deny,
			},
			expected: Deny,
		},
		{
			name: "Verifier decision is Deny, rule decision is Allow",
			stats: evalStats{
				verifierDecision: Deny,
				ruleDecision:     Allow,
			},
			expected: Deny,
		},
		{
			name: "Both decisions are Deny",
			stats: evalStats{
				verifierDecision: Deny,
				ruleDecision:     Deny,
			},
			expected: Deny,
		},
		{
			name: "Verifier decision is Undetermined, rule decision is Allow",
			stats: evalStats{
				verifierDecision: Undetermined,
				ruleDecision:     Allow,
			},
			expected: Undetermined,
		},
		{
			name: "Verifier decision is Allow, rule decision is Undetermined",
			stats: evalStats{
				verifierDecision: Allow,
				ruleDecision:     Undetermined,
			},
			expected: Undetermined,
		},
		{
			name: "Both decisions are Undetermined",
			stats: evalStats{
				verifierDecision: Undetermined,
				ruleDecision:     Undetermined,
			},
			expected: Undetermined,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &thresholdPolicyState{
				stats: &tt.stats,
			}
			if got := state.Decision(); got != tt.expected {
				t.Errorf("Decision() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMarkAsSkippable(t *testing.T) {
	childState1 := &thresholdPolicyState{
		stats: &evalStats{skippable: false},
	}
	childState2 := &thresholdPolicyState{
		stats: &evalStats{skippable: false},
	}
	parentState := &thresholdPolicyState{
		stats:       &evalStats{skippable: false},
		childStates: []*thresholdPolicyState{childState1, childState2},
	}

	parentState.markAsSkippable()

	if !parentState.stats.skippable {
		t.Errorf("parentState.stats.skippable = false, want true")
	}
	if !childState1.stats.skippable {
		t.Errorf("childState1.stats.skippable = false, want true")
	}
	if !childState2.stats.skippable {
		t.Errorf("childState2.stats.skippable = false, want true")
	}
}

func TestRequireEvaluation(t *testing.T) {
	tests := []struct {
		name     string
		stats    evalStats
		expected bool
	}{
		{
			name:     "Decision is Undetermined and cannot be skipped",
			stats:    evalStats{},
			expected: true,
		},
		{
			name: "Decision is made but cannot be skipped",
			stats: evalStats{
				verifierDecision: Deny,
				skippable:        false,
			},
			expected: false,
		},
		{
			name: "Decision is made and can be skipped",
			stats: evalStats{
				verifierDecision: Deny,
				skippable:        true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &thresholdPolicyState{
				stats: &tt.stats,
			}
			if got := state.requireEvaluation(); got != tt.expected {
				t.Errorf("requireEvaluation() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNextState(t *testing.T) {
	verifier1 := &mockPolicyVerifier{
		name:         "verifier1",
		artifactType: artifactType1,
	}
	state1 := &thresholdPolicyState{
		rule: &PolicyRule{
			Verifier: verifier1,
		},
	}
	state2 := &thresholdPolicyState{
		rule: &PolicyRule{},
		childStates: []*thresholdPolicyState{
			{
				rule: &PolicyRule{
					Verifier: verifier1,
				},
			},
		},
	}

	tests := []struct {
		name           string
		state          *thresholdPolicyState
		artifact       ocispec.Descriptor
		expectedResult *thresholdPolicyState
	}{
		{
			name:  "Root state has verifier configured",
			state: state1,
			artifact: ocispec.Descriptor{
				ArtifactType: artifactType1,
			},
			expectedResult: state1,
		},
		{
			name: "No child states",
			state: &thresholdPolicyState{
				rule: &PolicyRule{},
			},
			artifact: ocispec.Descriptor{
				ArtifactType: artifactType1,
			},
			expectedResult: nil,
		},
		{
			name:  "Expected state is not found",
			state: state2,
			artifact: ocispec.Descriptor{
				ArtifactType: notationType,
			},
			expectedResult: nil,
		},
		{
			name:  "Expected state in sub-graph",
			state: state2,
			artifact: ocispec.Descriptor{
				ArtifactType: artifactType1,
			},
			expectedResult: state1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.state.NextState(tt.artifact)
			if result != nil && tt.expectedResult != nil {
				if !reflect.DeepEqual(result, tt.expectedResult) {
					t.Fatalf("NextState() = %v, want %v", result, tt.expectedResult)
				}
			} else if result != nil || tt.expectedResult != nil {
				t.Fatalf("NextState() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestAllowEvalDuringVerify(t *testing.T) {
	e := &ThresholdBasedEnforcer{}
	if e.AllowEvalDuringVerify() != true {
		t.Errorf("AllowEvalDuringVerify() = %v, want %v", e.AllowEvalDuringVerify(), true)
	}
}
func TestEvaluateReport(t *testing.T) {
	tests := []struct {
		name           string
		state          EvaluationState
		expectedResult PolicyDecision
	}{
		{
			name: "EvaluateReport returns Allow",
			state: &thresholdPolicyState{
				stats: &evalStats{
					verifierDecision: Allow,
					ruleDecision:     Allow,
				},
			},
			expectedResult: Allow,
		},
		{
			name: "EvaluateReport returns Deny",
			state: &thresholdPolicyState{
				stats: &evalStats{
					verifierDecision: Deny,
					ruleDecision:     Allow,
				},
			},
			expectedResult: Deny,
		},
		{
			name: "EvaluateReport returns Undetermined",
			state: &thresholdPolicyState{
				stats: &evalStats{
					verifierDecision: Undetermined,
					ruleDecision:     Allow,
				},
			},
			expectedResult: Undetermined,
		},
	}

	enforcer := &ThresholdBasedEnforcer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &ValidationReport{
				policyEvaluationState: tt.state,
			}
			got, err := enforcer.EvaluateReport(context.Background(), report)
			if err != nil {
				t.Errorf("EvaluateReport() returned error: %v", err)
			}
			if got != tt.expectedResult {
				t.Errorf("EvaluateReport() = %v, want %v", got, tt.expectedResult)
			}
		})
	}
}
func TestEvaluateResult(t *testing.T) {
	notationVerifier1 := &mockPolicyVerifier{
		name:         "notatin-verifier-1",
		artifactType: notationType,
	}
	sbomVerifier1 := &mockPolicyVerifier{
		name:         "sbom-verifier-1",
		artifactType: sbomType,
	}
	cosignVerifier1 := &mockPolicyVerifier{
		name:         "cosign-verifier-1",
		artifactType: cosignType,
	}

	notationRule1 := &PolicyRule{
		Verifier: notationVerifier1,
	}
	cosignRule1 := &PolicyRule{
		Verifier: cosignVerifier1,
	}
	sbomRule1 := &PolicyRule{
		Verifier:  sbomVerifier1,
		Threshold: 1,
		Rules:     []*PolicyRule{notationRule1, cosignRule1},
	}
	imageRule1 := &PolicyRule{
		Rules: []*PolicyRule{notationRule1, sbomRule1},
	}
	imageRule2 := &PolicyRule{
		Threshold: 1,
		Rules:     []*PolicyRule{notationRule1, cosignRule1},
	}

	state1, err := generateStatesFromRule(imageRule1, nil)
	if err != nil {
		t.Fatalf("Failed to generate states: %v", err)
	}
	state2, err := generateStatesFromRule(imageRule2, nil)
	if err != nil {
		t.Fatalf("Failed to generate states: %v", err)
	}
	e := &ThresholdBasedEnforcer{}

	// Test case on nil report
	if err := e.EvaluateResult(context.Background(), nil, nil); err == nil {
		t.Fatalf("EvaluateResult() should return error for nil report")
	}

	// Test case on state2
	report1 := &ValidationReport{
		policyEvaluationState: state2.childStates[0],
	}
	requireEvaluation, err := e.RequireFurtherVerification(context.Background(), report1, notationVerifier1)
	if err != nil {
		t.Fatalf("RequireFurtherVerification() returned error: %v", err)
	}
	if requireEvaluation == false {
		t.Fatalf("RequireFurtherVerification() = %v, want true", requireEvaluation)
	}
	result := &VerificationResult{
		Verifier: notationVerifier1,
	}
	if err = e.EvaluateResult(context.Background(), report1, result); err != nil {
		t.Fatalf("EvaluateResult() returned error: %v", err)
	}
	requireEvaluation, err = e.RequireFurtherVerification(context.Background(), report1, cosignVerifier1)
	if err != nil {
		t.Fatalf("RequireFurtherVerification() returned error: %v", err)
	}
	if requireEvaluation == true {
		t.Fatalf("RequireFurtherVerification() = %v, want false", requireEvaluation)
	}

	// Test case on state1
	report1 = &ValidationReport{
		policyEvaluationState: state1.childStates[0],
	}
	result = &VerificationResult{
		Verifier: notationVerifier1,
	}
	if err := e.EvaluateResult(context.Background(), report1, result); err != nil {
		t.Fatalf("EvaluateResult() returned error: %v", err)
	}
	report2 := &ValidationReport{
		policyEvaluationState: state1.childStates[1],
	}
	if requireEvaluation, _ := e.RequireFurtherVerification(context.Background(), report2, sbomVerifier1); !requireEvaluation {
		t.Fatalf("RequireFurtherVerification() = %v, want true", requireEvaluation)
	}
	result = &VerificationResult{
		Verifier: sbomVerifier1,
	}
	if err := e.EvaluateResult(context.Background(), report2, result); err != nil {
		t.Fatalf("EvaluateResult() returned error: %v", err)
	}
	report3 := &ValidationReport{
		policyEvaluationState: state1.childStates[1].childStates[1],
	}
	if requireEvaluation, _ := e.RequireFurtherVerification(context.Background(), report3, cosignVerifier1); !requireEvaluation {
		t.Fatalf("RequireFurtherVerification() = %v, want true", requireEvaluation)
	}
	result = &VerificationResult{
		Verifier: cosignVerifier1,
		Err:      errors.New("test error"),
	}
	if err := e.EvaluateResult(context.Background(), report3, result); err != nil {
		t.Fatalf("EvaluateResult() returned error: %v", err)
	}
	result = &VerificationResult{
		Verifier: cosignVerifier1,
	}
	if err := e.EvaluateResult(context.Background(), report3, result); err != nil {
		t.Fatalf("EvaluateResult() returned error: %v", err)
	}
	if state1.Decision() != Allow {
		t.Fatalf("Decision() = %v, want Allow", state1.Decision())
	}
}
func TestNewEvaluationState(t *testing.T) {
	notationVerifier := &mockPolicyVerifier{
		name:         "notation-verifier",
		artifactType: notationType,
	}
	sbomVerifier := &mockPolicyVerifier{
		name:         "sbom-verifier",
		artifactType: sbomType,
	}
	cosignVerifier := &mockPolicyVerifier{
		name:         "cosign-verifier",
		artifactType: cosignType,
	}

	notationRule := &PolicyRule{
		Verifier: notationVerifier,
	}
	cosignRule := &PolicyRule{
		Verifier: cosignVerifier,
	}
	sbomRule := &PolicyRule{
		Verifier:  sbomVerifier,
		Threshold: 1,
		Rules:     []*PolicyRule{notationRule, cosignRule},
	}
	imageRule := &PolicyRule{
		Rules: []*PolicyRule{notationRule, sbomRule},
	}

	enforcer := &ThresholdBasedEnforcer{
		policy: imageRule,
	}

	tests := []struct {
		name    string
		policy  *PolicyRule
		wantErr bool
	}{
		{
			name:    "Valid policy",
			policy:  imageRule,
			wantErr: false,
		},
		{
			name:    "Nil policy",
			policy:  nil,
			wantErr: true,
		},
		{
			name: "child rules contain a nil rule",
			policy: &PolicyRule{
				Rules: []*PolicyRule{nil},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer.policy = tt.policy
			state, err := enforcer.NewEvaluationState(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEvaluationState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && state == nil {
				t.Errorf("NewEvaluationState() returned nil state")
			}
		})
	}
}
func TestNewThresholdBasedEnforcer(t *testing.T) {
	verifier1 := &mockPolicyVerifier{
		name:         "verifier1",
		artifactType: artifactType1,
	}
	tests := []struct {
		name    string
		options ThresholdBasedEnforcerOptions
		wantErr bool
	}{
		{
			name: "Valid policy",
			options: ThresholdBasedEnforcerOptions{
				Policy: &PolicyRule{
					Rules: []*PolicyRule{
						{
							Verifier: verifier1,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Nil policy",
			options: ThresholdBasedEnforcerOptions{
				Policy: nil,
			},
			wantErr: true,
		},
		{
			name: "Root rule has verifier",
			options: ThresholdBasedEnforcerOptions{
				Policy: &PolicyRule{
					Verifier: verifier1,
				},
			},
			wantErr: true,
		},
		{
			name: "Child rules contain nil rule",
			options: ThresholdBasedEnforcerOptions{
				Policy: &PolicyRule{
					Rules: []*PolicyRule{nil},
				},
			},
			wantErr: true,
		},
		{
			name: "Threshold greater than number of rules",
			options: ThresholdBasedEnforcerOptions{
				Policy: &PolicyRule{
					Threshold: 2,
					Rules: []*PolicyRule{
						{
							Verifier: verifier1,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Negative threshold",
			options: ThresholdBasedEnforcerOptions{
				Policy: &PolicyRule{
					Threshold: -1,
					Rules: []*PolicyRule{
						{
							Verifier: verifier1,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Nested rule has negative threshold",
			options: ThresholdBasedEnforcerOptions{
				Policy: &PolicyRule{
					Rules: []*PolicyRule{
						{
							Verifier:  verifier1,
							Threshold: -1,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Duplicate verifiers",
			options: ThresholdBasedEnforcerOptions{
				Policy: &PolicyRule{
					Rules: []*PolicyRule{
						{
							Verifier: verifier1,
						},
						{
							Verifier: verifier1,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Nested duplicate verifiers",
			options: ThresholdBasedEnforcerOptions{
				Policy: &PolicyRule{
					Rules: []*PolicyRule{
						{
							Rules: []*PolicyRule{
								{
									Verifier: verifier1,
								},
								{
									Verifier: verifier1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewThresholdBasedEnforcer(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewThresholdBasedEnforcer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
