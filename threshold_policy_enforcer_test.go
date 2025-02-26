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
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	testArtifactType  = "testAritfactType"
	testArtifactType2 = "testArtifactType2"
	verifier1         = "verifier1"
	verifier2         = "verifier2"
	verifier3         = "verifier3"
)

// mockVerifier is a mock implementation of Verifier.
type mockPolicyVerifier struct {
	name string
}

func (m *mockPolicyVerifier) Name() string {
	return m.name
}

func (m *mockPolicyVerifier) Type() string {
	return "mock-verifier-type"
}

func (m *mockPolicyVerifier) Verifiable(_ ocispec.Descriptor) bool {
	return true
}

func (m *mockPolicyVerifier) Verify(ctx context.Context, opts *VerifyOptions) (*VerificationResult, error) {
	return nil, nil
}

func TestCreateNextState(t *testing.T) {
	tests := []struct {
		name         string
		rule         *PolicyRule
		artifact     ocispec.Descriptor
		expectedRule bool
	}{
		{
			name: "No dependent rules",
			rule: &PolicyRule{
				VerifierThresholds: map[string]int{verifier1: 1},
				DependentRules:     map[string]*PolicyRule{},
			},
			artifact:     ocispec.Descriptor{ArtifactType: testArtifactType},
			expectedRule: true,
		},
		{
			name: "Dependent rule exists",
			rule: &PolicyRule{
				VerifierThresholds: map[string]int{verifier1: 1},
				DependentRules: map[string]*PolicyRule{
					testArtifactType: {
						VerifierThresholds: map[string]int{verifier2: 1},
					},
				},
			},
			artifact:     ocispec.Descriptor{ArtifactType: testArtifactType},
			expectedRule: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{rule: tt.rule}
			nextState := s.CreateNextState(tt.artifact).(*state)
			if (nextState.rule != nil) != tt.expectedRule {
				t.Errorf("expected rule to be %v, got %v", tt.expectedRule, nextState.rule != nil)
			}
		})
	}
}

func TestDecision(t *testing.T) {
	tests := []struct {
		name               string
		artifactDecision   PolicyDecision
		dependencyDecision PolicyDecision
		expectedDecision   PolicyDecision
	}{
		{
			name:               "Both decisions are Allow",
			artifactDecision:   Allow,
			dependencyDecision: Allow,
			expectedDecision:   Allow,
		},
		{
			name:               "Artifact decision is Denied, dependency decision is Allow",
			artifactDecision:   Deny,
			dependencyDecision: Allow,
			expectedDecision:   Deny,
		},
		{
			name:               "Artifact decision is Allow, dependency decision is Denied",
			artifactDecision:   Allow,
			dependencyDecision: Deny,
			expectedDecision:   Deny,
		},
		{
			name:               "Both decisions are Denied",
			artifactDecision:   Deny,
			dependencyDecision: Deny,
			expectedDecision:   Deny,
		},
		{
			name:               "Artifact decision is Undetermined, dependency decision is Allow",
			artifactDecision:   Undetermined,
			dependencyDecision: Allow,
			expectedDecision:   Undetermined,
		},
		{
			name:               "Artifact decision is Allow, dependency decision is Undetermined",
			artifactDecision:   Allow,
			dependencyDecision: Undetermined,
			expectedDecision:   Undetermined,
		},
		{
			name:               "Both decisions are Undetermined",
			artifactDecision:   Undetermined,
			dependencyDecision: Undetermined,
			expectedDecision:   Undetermined,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{
				artifactDecision:   tt.artifactDecision,
				dependencyDecision: tt.dependencyDecision,
			}
			decision := s.Decision()
			if decision != tt.expectedDecision {
				t.Errorf("expected decision to be %v, got %v", tt.expectedDecision, decision)
			}
		})
	}
}

func TestAddVerifierSuccessCount(t *testing.T) {
	tests := []struct {
		name           string
		initialCounts  map[string]int
		verifierName   string
		expectedCounts map[string]int
	}{
		{
			name:           "Add to empty map",
			initialCounts:  nil,
			verifierName:   verifier1,
			expectedCounts: map[string]int{verifier1: 1},
		},
		{
			name:           "Add to existing verifier",
			initialCounts:  map[string]int{verifier1: 1},
			verifierName:   verifier1,
			expectedCounts: map[string]int{verifier1: 2},
		},
		{
			name:           "Add to new verifier",
			initialCounts:  map[string]int{verifier1: 1},
			verifierName:   verifier2,
			expectedCounts: map[string]int{verifier1: 1, verifier2: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{verifierSuccessCount: tt.initialCounts}
			s.addVerifierSuccessCount(tt.verifierName)
			for verifier, count := range tt.expectedCounts {
				if s.verifierSuccessCount[verifier] != count {
					t.Errorf("expected count for %s to be %d, got %d", verifier, count, s.verifierSuccessCount[verifier])
				}
			}
		})
	}
}

func TestGetVerifierSuccessCount(t *testing.T) {
	tests := []struct {
		name          string
		initialCounts map[string]int
		verifierName  string
		expectedCount int
	}{
		{
			name:          "Verifier not in map",
			initialCounts: map[string]int{verifier1: 1},
			verifierName:  verifier2,
			expectedCount: 0,
		},
		{
			name:          "Verifier in map",
			initialCounts: map[string]int{verifier1: 1},
			verifierName:  verifier1,
			expectedCount: 1,
		},
		{
			name:          "Empty map",
			initialCounts: nil,
			verifierName:  verifier1,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{verifierSuccessCount: tt.initialCounts}
			count := s.getVerifierSuccessCount(tt.verifierName)
			if count != tt.expectedCount {
				t.Errorf("expected count for %s to be %d, got %d", tt.verifierName, tt.expectedCount, count)
			}
		})
	}
}

func TestGetNestedArtifactVerifierCount(t *testing.T) {
	tests := []struct {
		name          string
		initialCounts map[string]map[string]int
		artifactType  string
		verifierName  string
		expectedCount int
	}{
		{
			name: "Artifact type and verifier not in map",
			initialCounts: map[string]map[string]int{
				"artifactType1": {verifier1: 1},
			},
			artifactType:  "artifactType2",
			verifierName:  verifier2,
			expectedCount: 0,
		},
		{
			name: "Artifact type in map, verifier not in map",
			initialCounts: map[string]map[string]int{
				"artifactType1": {verifier1: 1},
			},
			artifactType:  "artifactType1",
			verifierName:  verifier2,
			expectedCount: 0,
		},
		{
			name: "Artifact type and verifier in map",
			initialCounts: map[string]map[string]int{
				"artifactType1": {verifier1: 1},
			},
			artifactType:  "artifactType1",
			verifierName:  verifier1,
			expectedCount: 1,
		},
		{
			name:          "Empty map",
			initialCounts: nil,
			artifactType:  "artifactType1",
			verifierName:  verifier1,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{nestedArtifactVerifierCount: tt.initialCounts}
			count := s.getNestedArtifactVerifierCount(tt.artifactType, tt.verifierName)
			if count != tt.expectedCount {
				t.Errorf("expected count for %s/%s to be %d, got %d", tt.artifactType, tt.verifierName, tt.expectedCount, count)
			}
		})
	}
}
func TestUpdateOnFailedDependentEval(t *testing.T) {
	tests := []struct {
		name             string
		initialRule      *PolicyRule
		artifactType     string
		expectedDecision PolicyDecision
	}{
		{
			name: "No dependent rules",
			initialRule: &PolicyRule{
				DependentRules: map[string]*PolicyRule{},
			},
			artifactType:     testArtifactType,
			expectedDecision: Undetermined,
		},
		{
			name: "Dependent rule exists, artifact type is not in dependent rules",
			initialRule: &PolicyRule{
				DependentRules: map[string]*PolicyRule{
					testArtifactType2: {
						VerifierThresholds: map[string]int{verifier2: 2},
					},
				},
			},
			artifactType:     testArtifactType,
			expectedDecision: Undetermined,
		},
		{
			name: "Dependent rule exists, all verifiers must succeed",
			initialRule: &PolicyRule{
				DependentRules: map[string]*PolicyRule{
					testArtifactType: {
						VerifierThresholds: map[string]int{verifier2: AllRule},
					},
				},
			},
			artifactType:     testArtifactType,
			expectedDecision: Deny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{rule: tt.initialRule}
			s.handleFailedDependentEval(tt.artifactType)
			if s.dependencyDecision != tt.expectedDecision {
				t.Errorf("expected decision to be %v, got %v", tt.expectedDecision, s.artifactDecision)
			}
		})
	}
}
func TestHandleSuccessfulDependentEval(t *testing.T) {
	tests := []struct {
		name                      string
		initialRule               *PolicyRule
		artifactType              string
		verifierCount             map[string]int
		initialNestedVerifierMap  map[string]map[string]int
		expectedDecision          PolicyDecision
		expectedNestedVerifierMap map[string]map[string]int
	}{
		{
			name: "No verifier count",
			initialRule: &PolicyRule{
				DependentRules: map[string]*PolicyRule{
					testArtifactType: {
						VerifierThresholds: map[string]int{verifier1: 1},
					},
				},
			},
			artifactType:              testArtifactType,
			verifierCount:             map[string]int{},
			initialNestedVerifierMap:  nil,
			expectedDecision:          Undetermined,
			expectedNestedVerifierMap: map[string]map[string]int{},
		},
		{
			name: "Verifier count meets threshold",
			initialRule: &PolicyRule{
				DependentRules: map[string]*PolicyRule{
					testArtifactType: {
						VerifierThresholds: map[string]int{verifier1: 1},
					},
				},
			},
			artifactType:             testArtifactType,
			verifierCount:            map[string]int{verifier1: 1},
			initialNestedVerifierMap: nil,
			expectedDecision:         Allow,
			expectedNestedVerifierMap: map[string]map[string]int{
				testArtifactType: {verifier1: 1},
			},
		},
		{
			name: "Verifier count does not meet threshold",
			initialRule: &PolicyRule{
				DependentRules: map[string]*PolicyRule{
					testArtifactType: {
						VerifierThresholds: map[string]int{verifier1: 2},
					},
				},
			},
			artifactType:             testArtifactType,
			verifierCount:            map[string]int{verifier1: 1},
			initialNestedVerifierMap: nil,
			expectedDecision:         Undetermined,
			expectedNestedVerifierMap: map[string]map[string]int{
				testArtifactType: {verifier1: 1},
			},
		},
		{
			name: "Verifier count does not contain required verifier",
			initialRule: &PolicyRule{
				DependentRules: map[string]*PolicyRule{
					testArtifactType: {
						VerifierThresholds: map[string]int{verifier1: 2},
					},
				},
			},
			artifactType:             testArtifactType,
			verifierCount:            map[string]int{verifier2: 1},
			initialNestedVerifierMap: nil,
			expectedDecision:         Undetermined,
			expectedNestedVerifierMap: map[string]map[string]int{
				testArtifactType: {verifier1: 0},
			},
		},
		{
			name: "Multiple verifiers, all meet threshold",
			initialRule: &PolicyRule{
				DependentRules: map[string]*PolicyRule{
					testArtifactType: {
						VerifierThresholds: map[string]int{verifier1: 1, verifier2: 1},
					},
				},
			},
			artifactType:             testArtifactType,
			verifierCount:            map[string]int{verifier1: 1, verifier2: 1},
			initialNestedVerifierMap: nil,
			expectedDecision:         Allow,
			expectedNestedVerifierMap: map[string]map[string]int{
				testArtifactType: {verifier1: 1, verifier2: 1},
			},
		},
		{
			name: "Multiple verifiers, one does not meet threshold",
			initialRule: &PolicyRule{
				DependentRules: map[string]*PolicyRule{
					testArtifactType: {
						VerifierThresholds: map[string]int{verifier1: 1, verifier2: 2},
					},
				},
			},
			artifactType:             testArtifactType,
			verifierCount:            map[string]int{verifier1: 1, verifier2: 1},
			initialNestedVerifierMap: nil,
			expectedDecision:         Undetermined,
			expectedNestedVerifierMap: map[string]map[string]int{
				testArtifactType: {verifier1: 1, verifier2: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{
				rule:                        tt.initialRule,
				nestedArtifactVerifierCount: tt.initialNestedVerifierMap,
			}
			s.handleSuccessfulDependentEval(tt.artifactType, tt.verifierCount)
			if s.dependencyDecision != tt.expectedDecision {
				t.Errorf("expected decision to be %v, got %v", tt.expectedDecision, s.dependencyDecision)
			}
			for artifactType, verifiers := range tt.expectedNestedVerifierMap {
				for verifier, count := range verifiers {
					if s.nestedArtifactVerifierCount[artifactType][verifier] != count {
						t.Errorf("expected count for %s/%s to be %d, got %d", artifactType, verifier, count, s.nestedArtifactVerifierCount[artifactType][verifier])
					}
				}
			}
		})
	}
}
func TestNewThresholdBasedEnforcer(t *testing.T) {
	tests := []struct {
		name   string
		policy map[string]*PolicyRule
	}{
		{
			name:   "Empty policy",
			policy: map[string]*PolicyRule{},
		},
		{
			name: "Single rule policy",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
				},
			},
		},
		{
			name: "Multiple rules policy",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
				},
				testArtifactType2: {
					VerifierThresholds: map[string]int{verifier2: 2},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewThresholdBasedEnforcer(tt.policy)
			if len(enforcer.rules) != len(tt.policy) {
				t.Errorf("expected %d rules, got %d", len(tt.policy), len(enforcer.rules))
			}
			for artifactType, rule := range tt.policy {
				if enforcer.rules[artifactType] == nil {
					t.Errorf("expected rule for %s to be non-nil", artifactType)
				}
				if len(enforcer.rules[artifactType].VerifierThresholds) != len(rule.VerifierThresholds) {
					t.Errorf("expected %d verifier thresholds, got %d", len(rule.VerifierThresholds), len(enforcer.rules[artifactType].VerifierThresholds))
				}
			}
		})
	}
}
func TestNewEvaluationState(t *testing.T) {
	tests := []struct {
		name   string
		policy map[string]*PolicyRule
	}{
		{
			name:   "Empty policy",
			policy: map[string]*PolicyRule{},
		},
		{
			name: "Single rule policy",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
				},
			},
		},
		{
			name: "Multiple rules policy",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
				},
				testArtifactType2: {
					VerifierThresholds: map[string]int{verifier2: 2},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewThresholdBasedEnforcer(tt.policy)
			state := enforcer.NewEvaluationState().(*state)
			if state.rule == nil {
				t.Errorf("expected rule to be non-nil")
			}
			if len(state.rule.VerifierThresholds) != 0 {
				t.Errorf("expected 0 verifier thresholds, got %d", len(state.rule.VerifierThresholds))
			}
			if len(state.rule.DependentRules) != len(tt.policy) {
				t.Errorf("expected %d dependent rules, got %d", len(tt.policy), len(state.rule.DependentRules))
			}
			if len(state.verifierSuccessCount) != 0 {
				t.Errorf("expected 0 verifier success counts, got %d", len(state.verifierSuccessCount))
			}
			if len(state.nestedArtifactVerifierCount) != 0 {
				t.Errorf("expected 0 nested artifact verifier counts, got %d", len(state.nestedArtifactVerifierCount))
			}
		})
	}
}
func TestAllowEvalDuringVerify(t *testing.T) {
	enforcer := NewThresholdBasedEnforcer(nil)
	if !enforcer.AllowEvalDuringVerify() {
		t.Errorf("expected AllowEvalDuringVerify to return true")
	}
}

func TestRequireFurtherVerification(t *testing.T) {
	tests := []struct {
		name           string
		policy         map[string]*PolicyRule
		artifactReport *ValidationReport
		artifact       ocispec.Descriptor
		verifier       Verifier
		expectedResult bool
		expectedError  bool
	}{
		{
			name:   "Subject decision is determined",
			policy: map[string]*PolicyRule{},
			artifactReport: &ValidationReport{
				SubjectReport: &ValidationReport{
					PolicyEvaluationState: &state{
						artifactDecision:   Allow,
						dependencyDecision: Allow,
					},
				},
			},
			verifier:       &mockVerifier{},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:   "Artifact decision is determined",
			policy: map[string]*PolicyRule{},
			artifactReport: &ValidationReport{
				SubjectReport: &ValidationReport{
					PolicyEvaluationState: &state{
						artifactDecision:   Undetermined,
						dependencyDecision: Undetermined,
					},
				},
				PolicyEvaluationState: &state{
					artifactDecision: Deny,
				},
			},
			verifier:       &mockVerifier{},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "artifact does not have rule defined",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
				},
			},
			artifactReport: &ValidationReport{
				SubjectReport: &ValidationReport{
					PolicyEvaluationState: &state{
						artifactDecision: Allow,
					},
				},
				PolicyEvaluationState: &state{
					artifactDecision: Undetermined,
				},
			},
			verifier:       &mockVerifier{},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "artifact's rule does not have verifierThreshold defined",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
				},
			},
			artifactReport: &ValidationReport{
				SubjectReport: &ValidationReport{
					PolicyEvaluationState: &state{
						artifactDecision: Allow,
					},
				},
				PolicyEvaluationState: &state{
					artifactDecision: Undetermined,
					rule:             &PolicyRule{},
				},
			},
			verifier:       &mockVerifier{},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "artifact's rule requires 0 successful verification",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
				},
			},
			artifactReport: &ValidationReport{
				SubjectReport: &ValidationReport{
					PolicyEvaluationState: &state{
						artifactDecision: Allow,
					},
				},
				PolicyEvaluationState: &state{
					artifactDecision: Undetermined,
					rule: &PolicyRule{
						VerifierThresholds: map[string]int{verifier1: 0},
					},
				},
			},
			verifier:       &mockVerifier{},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "Verifier count meets the threshold",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
				},
			},
			artifactReport: &ValidationReport{
				SubjectReport: &ValidationReport{
					PolicyEvaluationState: &state{
						artifactDecision: Undetermined,
					},
				},
				PolicyEvaluationState: &state{
					artifactDecision: Undetermined,
					rule: &PolicyRule{
						VerifierThresholds: map[string]int{"mock-verifier-name": 1},
					},
					verifierSuccessCount: map[string]int{
						"mock-verifier-name": 1,
					},
				},
			},
			artifact:       ocispec.Descriptor{ArtifactType: testArtifactType},
			verifier:       &mockVerifier{},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "Verifier count does not meet the threshold",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
				},
			},
			artifactReport: &ValidationReport{
				SubjectReport: &ValidationReport{
					PolicyEvaluationState: &state{
						artifactDecision: Undetermined,
					},
				},
				PolicyEvaluationState: &state{
					artifactDecision: Undetermined,
					rule: &PolicyRule{
						VerifierThresholds: map[string]int{"mock-verifier-name": 2},
					},
					verifierSuccessCount: map[string]int{
						"mock-verifier-name": 1,
					},
				},
			},
			artifact:       ocispec.Descriptor{ArtifactType: testArtifactType},
			verifier:       &mockVerifier{},
			expectedResult: true,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewThresholdBasedEnforcer(tt.policy)
			result, err := enforcer.RequireFurtherVerification(context.Background(), tt.artifactReport, tt.verifier)
			if result != tt.expectedResult {
				t.Errorf("expected result to be %v, got %v", tt.expectedResult, result)
			}
			if (err != nil) != tt.expectedError {
				t.Errorf("expected error to be %v, got %v", tt.expectedError, err != nil)
			}
		})
	}
}

func TestEvaluateReport(t *testing.T) {
	tests := []struct {
		name             string
		policy           map[string]*PolicyRule
		artifactReport   *ValidationReport
		expectedDecision PolicyDecision
		expectedError    bool
	}{
		{
			name:   "Artifact decision is already determined",
			policy: map[string]*PolicyRule{},
			artifactReport: &ValidationReport{
				PolicyEvaluationState: &state{
					artifactDecision: Deny,
				},
			},
			expectedDecision: Deny,
			expectedError:    false,
		},
		{
			name: "Verifier thresholds not met",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 2},
				},
			},
			artifactReport: &ValidationReport{
				PolicyEvaluationState: &state{
					rule: &PolicyRule{
						VerifierThresholds: map[string]int{verifier1: 2},
					},
					verifierSuccessCount: map[string]int{verifier1: 1},
				},
			},
			expectedDecision: Deny,
			expectedError:    false,
		},
		{
			name: "Dependent rules satisfied",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
					DependentRules: map[string]*PolicyRule{
						testArtifactType2: {
							VerifierThresholds: map[string]int{verifier2: 1},
						},
					},
				},
			},
			artifactReport: &ValidationReport{
				PolicyEvaluationState: &state{
					rule: &PolicyRule{
						VerifierThresholds: map[string]int{verifier1: 1},
						DependentRules: map[string]*PolicyRule{
							testArtifactType2: {
								VerifierThresholds: map[string]int{
									verifier2: 1,
									verifier3: 0,
								},
							},
						},
					},
					verifierSuccessCount: map[string]int{verifier1: 1},
					nestedArtifactVerifierCount: map[string]map[string]int{
						testArtifactType2: {verifier2: 1},
					},
				},
				ArtifactReports: []*ValidationReport{
					{
						Artifact: ocispec.Descriptor{ArtifactType: testArtifactType2},
						PolicyEvaluationState: &state{
							rule: &PolicyRule{
								VerifierThresholds: map[string]int{verifier2: 1},
							},
							dependencyDecision:   Allow,
							verifierSuccessCount: map[string]int{verifier2: 1},
						},
					},
				},
			},
			expectedDecision: Allow,
			expectedError:    false,
		},
		{
			name: "Dependent rules not satisfied",
			policy: map[string]*PolicyRule{
				testArtifactType: {
					VerifierThresholds: map[string]int{verifier1: 1},
					DependentRules: map[string]*PolicyRule{
						testArtifactType2: {
							VerifierThresholds: map[string]int{verifier2: 2},
						},
					},
				},
			},
			artifactReport: &ValidationReport{
				PolicyEvaluationState: &state{
					rule: &PolicyRule{
						VerifierThresholds: map[string]int{verifier1: 1},
						DependentRules: map[string]*PolicyRule{
							testArtifactType2: {
								VerifierThresholds: map[string]int{
									verifier2: 2,
								},
							},
						},
					},
					verifierSuccessCount: map[string]int{verifier1: 1},
					nestedArtifactVerifierCount: map[string]map[string]int{
						testArtifactType2: {verifier2: 1},
					},
				},
				ArtifactReports: []*ValidationReport{
					{
						Artifact: ocispec.Descriptor{ArtifactType: testArtifactType2},
						PolicyEvaluationState: &state{
							rule: &PolicyRule{
								VerifierThresholds: map[string]int{verifier2: 2},
							},
							dependencyDecision:   Allow,
							verifierSuccessCount: map[string]int{verifier2: 1},
						},
					},
				},
			},
			expectedDecision: Deny,
			expectedError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewThresholdBasedEnforcer(tt.policy)
			decision, err := enforcer.EvaluateReport(context.Background(), tt.artifactReport)
			if decision != tt.expectedDecision {
				t.Errorf("expected decision to be %v, got %v", tt.expectedDecision, decision)
			}
			if (err != nil) != tt.expectedError {
				t.Errorf("expected error to be %v, got %v", tt.expectedError, err != nil)
			}
		})
	}
}

func TestEvaluateResult(t *testing.T) {
	subjectReport := &ValidationReport{
		Artifact: ocispec.Descriptor{ArtifactType: testArtifactType2},
	}
	artifactReport := &ValidationReport{
		Artifact:      ocispec.Descriptor{ArtifactType: testArtifactType},
		SubjectReport: subjectReport,
	}
	subjectReport.ArtifactReports = []*ValidationReport{artifactReport}

	tests := []struct {
		name                     string
		artifactState            *state
		subjectState             *state
		verificationResult       *VerificationResult
		expectedArtifactDecision PolicyDecision
		expectedSubjectDecision  PolicyDecision
		expectedError            bool
	}{
		{
			name: "Decision is already determined",
			artifactState: &state{
				artifactDecision:   Allow,
				dependencyDecision: Allow,
			},
			subjectState:             &state{},
			expectedArtifactDecision: Allow,
			expectedSubjectDecision:  Undetermined,
			expectedError:            false,
		},
		{
			name: "Artifact has no rules defined",
			artifactState: &state{
				rule:                 &PolicyRule{},
				verifierSuccessCount: map[string]int{verifier1: 1},
			},
			subjectState: &state{
				rule: &PolicyRule{
					DependentRules: map[string]*PolicyRule{
						testArtifactType: {},
					},
				},
				artifactDecision: Allow,
			},
			verificationResult: &VerificationResult{
				Verifier: &mockVerifier{},
				Err:      nil,
			},
			expectedArtifactDecision: Allow,
			expectedSubjectDecision:  Allow,
			expectedError:            false,
		},
		{
			name: "Successful verification, update subject report",
			artifactState: &state{
				rule: &PolicyRule{
					VerifierThresholds: map[string]int{verifier1: 1},
				},
				verifierSuccessCount: map[string]int{verifier1: 1},
			},
			subjectState: &state{
				rule: &PolicyRule{
					DependentRules: map[string]*PolicyRule{
						testArtifactType: {
							VerifierThresholds: map[string]int{verifier1: 1},
						},
					},
				},
				artifactDecision: Allow,
			},
			verificationResult: &VerificationResult{
				Verifier: &mockVerifier{},
				Err:      nil,
			},
			expectedArtifactDecision: Allow,
			expectedSubjectDecision:  Allow,
			expectedError:            false,
		},
		{
			name: "Verification result is unsuccessful, AllRule is specified",
			artifactState: &state{
				rule: &PolicyRule{
					VerifierThresholds: map[string]int{verifier1: -1},
				},
				verifierSuccessCount: map[string]int{},
			},
			subjectState: &state{
				rule: &PolicyRule{
					DependentRules: map[string]*PolicyRule{
						testArtifactType: {
							VerifierThresholds: map[string]int{verifier1: -1},
						},
					},
				},
				artifactDecision: Undetermined,
			},
			verificationResult: &VerificationResult{
				Verifier: &mockPolicyVerifier{name: verifier1},
				Err:      errors.New("verification failed"),
			},
			expectedArtifactDecision: Deny,
			expectedSubjectDecision:  Deny,
			expectedError:            false,
		},
		{
			name: "Verification result is unsuccessful, threshold is specified",
			artifactState: &state{
				rule: &PolicyRule{
					VerifierThresholds: map[string]int{verifier1: 3},
				},
				verifierSuccessCount: map[string]int{},
			},
			subjectState: &state{
				rule: &PolicyRule{
					DependentRules: map[string]*PolicyRule{
						testArtifactType: {
							VerifierThresholds: map[string]int{verifier1: 1},
						},
					},
				},
				dependencyDecision: Undetermined,
			},
			verificationResult: &VerificationResult{
				Verifier: &mockPolicyVerifier{name: verifier1},
				Err:      errors.New("verification failed"),
			},
			expectedArtifactDecision: Undetermined,
			expectedSubjectDecision:  Undetermined,
			expectedError:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewThresholdBasedEnforcer(nil)
			artifactReport.PolicyEvaluationState = tt.artifactState
			subjectReport.PolicyEvaluationState = tt.subjectState
			decision, err := enforcer.EvaluateResult(context.Background(), artifactReport, tt.verificationResult)
			if decision != tt.expectedArtifactDecision {
				t.Errorf("expected decision to be %v, got %v", tt.expectedArtifactDecision, decision)
			}
			if tt.expectedSubjectDecision != tt.subjectState.Decision() {
				t.Errorf("expected subject decision to be %v, got %v", tt.expectedSubjectDecision, tt.subjectState.Decision())
			}
			if (err != nil) != tt.expectedError {
				t.Errorf("expected error to be %v, got %v", tt.expectedError, err != nil)
			}
		})
	}
}
func TestVerifierThresholdsMet(t *testing.T) {
	tests := []struct {
		name                 string
		rule                 *PolicyRule
		verifierSuccessCount map[string]int
		expectedResult       bool
	}{
		{
			name: "No verifier thresholds",
			rule: &PolicyRule{
				VerifierThresholds: map[string]int{},
			},
			verifierSuccessCount: map[string]int{},
			expectedResult:       true,
		},
		{
			name: "Single verifier threshold met",
			rule: &PolicyRule{
				VerifierThresholds: map[string]int{verifier1: 1},
			},
			verifierSuccessCount: map[string]int{verifier1: 1},
			expectedResult:       true,
		},
		{
			name: "Single verifier threshold not met",
			rule: &PolicyRule{
				VerifierThresholds: map[string]int{verifier1: 2},
			},
			verifierSuccessCount: map[string]int{verifier1: 1},
			expectedResult:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{
				rule:                 tt.rule,
				verifierSuccessCount: tt.verifierSuccessCount,
			}
			result := s.verifierThresholdsMet()
			if result != tt.expectedResult {
				t.Errorf("expected result to be %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestUpdateSubjectOnSuccessfulEval(t *testing.T) {
	tests := []struct {
		name                    string
		subjectState            *state
		artifactState           *state
		expectedSubjectDecision PolicyDecision
		expectedError           bool
	}{
		{
			name: "Dependent evaluation not successful",
			subjectState: &state{
				rule: &PolicyRule{
					DependentRules: map[string]*PolicyRule{
						testArtifactType: {
							VerifierThresholds: map[string]int{verifier1: 2},
						},
					},
				},
				artifactDecision: Deny,
			},
			artifactState: &state{
				verifierSuccessCount: map[string]int{verifier1: 1},
			},
			expectedSubjectDecision: Deny,
			expectedError:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewThresholdBasedEnforcer(nil)
			artifactReport := &ValidationReport{
				PolicyEvaluationState: tt.artifactState,
			}
			subjectReport := &ValidationReport{
				PolicyEvaluationState: tt.subjectState,
				ArtifactReports:       []*ValidationReport{artifactReport},
			}
			artifactReport.SubjectReport = subjectReport

			err := enforcer.updateSubjectOnSuccessfulEval(context.Background(), artifactReport)
			if tt.expectedError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.subjectState.Decision() != tt.expectedSubjectDecision {
				t.Errorf("expected subject decision to be %v, got %v", tt.expectedSubjectDecision, tt.subjectState.Decision())
			}
		})
	}
}
