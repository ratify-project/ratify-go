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
	testRepo      = "test-registry/test-repo"
	testDigest1   = "sha256:cd0abf4135161b8aeb079b64b8215e433088d21463204771d070aadc52678aa0"
	testDigest2   = "sha256:e05b6fbf2432faf87115041d172aa1f587cff725b94c61d927f67c21e1e2d5b9"
	testDigest3   = "sha256:5ca41da4799a48a58ec307678155c52a37caad54492a96854b14d8c856a8c5d8"
	testDigest4   = "sha256:97fd9660fd193c8671ffa322453bf21e46ab8ab6543f82b065caa7f014155bc4"
	testDigest5   = "sha256:87f06eb9e99f17e1a57346c388d60e636a725f7d9bce33fb90e54156d36297e9"
	testImage     = testRepo + ":v1"
	testArtifact1 = testRepo + "@" + testDigest1
	testArtifact2 = testRepo + "@" + testDigest2
	testArtifact4 = testRepo + "@" + testDigest4
	validMessage1 = "valid signature 1"
	validMessage2 = "valid signature 2"
	validMessage3 = "valid signature 3"
	validMessage4 = "valid signature 4"
	validMessage5 = "valid signature 5"
)

// mockVerifier is a mock implementation of Verifier.
type mockVerifier struct {
	verifiable   bool
	verifyResult map[string]*VerificationResult
}

func (m *mockVerifier) Name() string {
	return "mock-verifier-name"
}

func (m *mockVerifier) Type() string {
	return "mock-verifier-type"
}

func (m *mockVerifier) Verifiable(_ ocispec.Descriptor) bool {
	return m.verifiable
}

func (m *mockVerifier) Verify(ctx context.Context, store Store, subject string, artifact ocispec.Descriptor) (*VerificationResult, error) {
	if m.verifyResult == nil {
		return &VerificationResult{}, errors.New("verify result not initialized")
	}
	if result, ok := m.verifyResult[artifact.Digest.String()]; ok {
		return result, nil
	}
	return &VerificationResult{}, nil
}

// mockStore is a mock implementation of Store.
type mockStore struct {
	tagToDesc         map[string]ocispec.Descriptor
	digestToReferrers map[string][]ocispec.Descriptor
}

func (m *mockStore) Name() string {
	return "mock-store-name"
}

func (m *mockStore) ListReferrers(ctx context.Context, ref string, artifactTypes []string, fn func(referrers []ocispec.Descriptor) error) error {
	if m.digestToReferrers == nil {
		return errors.New("referrers not initialized")
	}
	referrers := m.digestToReferrers[ref]
	return fn(referrers)
}

func (m *mockStore) FetchBlobContent(ctx context.Context, repo string, desc ocispec.Descriptor) ([]byte, error) {
	return nil, nil
}

func (m *mockStore) FetchImageManifest(ctx context.Context, repo string, desc ocispec.Descriptor) (*ocispec.Manifest, error) {
	return &ocispec.Manifest{}, nil
}

func (m *mockStore) Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	if m.tagToDesc == nil {
		return ocispec.Descriptor{}, errors.New("artifact to descriptor not initialized")
	}
	if desc, ok := m.tagToDesc[ref]; ok {
		return desc, nil
	}

	return ocispec.Descriptor{}, nil
}

// mockPolicyEnforcer is a mock implementation of PolicyEnforcer.
type mockPolicyEnforcer struct {
	returnErr bool
}

func (m *mockPolicyEnforcer) Evaluate(ctx context.Context, artifactReports []*ValidationReport) (bool, error) {
	if m.returnErr {
		return false, errors.New("error happened when evaluating policy")
	}
	return true, nil
}

func TestValidateArtifact(t *testing.T) {
	tests := []struct {
		name           string
		opts           ValidateArtifactOptions
		store          Store
		verifiers      []Verifier
		policyEnforcer PolicyEnforcer
		want           *ValidationResult
		wantErr        bool
	}{
		{
			name: "Invalid reference",
			opts: ValidateArtifactOptions{
				Subject:        "testrepo:v1",
				ReferenceTypes: []string{"referenceType"},
			},
			store:          &mockStore{},
			verifiers:      []Verifier{&mockVerifier{}},
			policyEnforcer: &mockPolicyEnforcer{},
			want:           nil,
			wantErr:        true,
		},
		{
			name: "Error happened when resolving reference",
			opts: ValidateArtifactOptions{
				Subject: testImage,
			},
			store:          &mockStore{},
			verifiers:      []Verifier{&mockVerifier{}},
			policyEnforcer: &mockPolicyEnforcer{},
			want:           nil,
			wantErr:        true,
		},
		{
			name: "Error happened when listing referrers",
			opts: ValidateArtifactOptions{
				Subject: testImage,
			},
			store: &mockStore{
				tagToDesc: map[string]ocispec.Descriptor{
					testImage: {
						Digest: testDigest1,
					},
				},
			},
			verifiers:      []Verifier{&mockVerifier{}},
			policyEnforcer: &mockPolicyEnforcer{},
			want:           nil,
			wantErr:        true,
		},
		{
			name: "No referrers attached to the artifact",
			opts: ValidateArtifactOptions{
				Subject: testImage,
			},
			store: &mockStore{
				tagToDesc: map[string]ocispec.Descriptor{
					testImage: {
						Digest: testDigest1,
					},
				},
				digestToReferrers: map[string][]ocispec.Descriptor{
					testArtifact1: {},
				},
			},
			verifiers:      []Verifier{&mockVerifier{}},
			policyEnforcer: &mockPolicyEnforcer{},
			want: &ValidationResult{
				Succeeded: true,
			},
			wantErr: false,
		},
		{
			name: "Verifier is unable to verify the artifact",
			opts: ValidateArtifactOptions{
				Subject: testImage,
			},
			store: &mockStore{
				tagToDesc: map[string]ocispec.Descriptor{
					testImage: {
						Digest: testDigest1,
					},
				},
				digestToReferrers: map[string][]ocispec.Descriptor{
					testArtifact1: {
						{
							Digest: testDigest2,
						},
					},
				},
			},
			verifiers:      []Verifier{&mockVerifier{}},
			policyEnforcer: &mockPolicyEnforcer{},
			want: &ValidationResult{
				Succeeded: true,
				ArtifactReports: []*ValidationReport{
					{
						Results:         []*VerificationResult{},
						ArtifactReports: []*ValidationReport{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Error happened when verifying the artifact",
			opts: ValidateArtifactOptions{
				Subject: testImage,
			},
			store: &mockStore{
				tagToDesc: map[string]ocispec.Descriptor{
					testImage: {
						Digest: testDigest1,
					},
				},
				digestToReferrers: map[string][]ocispec.Descriptor{
					testArtifact1: {
						{
							Digest: testDigest2,
						},
					},
				},
			},
			verifiers: []Verifier{&mockVerifier{
				verifiable: true,
			}},
			policyEnforcer: &mockPolicyEnforcer{},
			want:           nil,
			wantErr:        true,
		},
		{
			name: "Verifier returned result without error",
			opts: ValidateArtifactOptions{
				Subject: testImage,
			},
			store: &mockStore{
				tagToDesc: map[string]ocispec.Descriptor{
					testImage: {
						Digest: testDigest1,
					},
				},
				digestToReferrers: map[string][]ocispec.Descriptor{
					testArtifact1: {
						{
							Digest: testDigest2,
						},
					},
				},
			},
			verifiers: []Verifier{&mockVerifier{
				verifiable: true,
				verifyResult: map[string]*VerificationResult{
					testDigest2: {
						Description: validMessage1,
					},
				},
			}},
			policyEnforcer: &mockPolicyEnforcer{},
			want: &ValidationResult{
				Succeeded: true,
				ArtifactReports: []*ValidationReport{
					{
						Results: []*VerificationResult{
							{
								Description: validMessage1,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Policy enforcer is not set",
			opts: ValidateArtifactOptions{
				Subject: testImage,
			},
			store: &mockStore{
				tagToDesc: map[string]ocispec.Descriptor{
					testImage: {
						Digest: testDigest1,
					},
				},
				digestToReferrers: map[string][]ocispec.Descriptor{
					testArtifact1: {
						{
							Digest: testDigest2,
						},
					},
				},
			},
			verifiers: []Verifier{&mockVerifier{
				verifiable: true,
				verifyResult: map[string]*VerificationResult{
					testDigest2: {
						Description: validMessage1,
					},
				},
			}},
			policyEnforcer: nil,
			want: &ValidationResult{
				Succeeded: false,
				ArtifactReports: []*ValidationReport{
					{
						Results: []*VerificationResult{
							{
								Description: validMessage1,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Policy enforcer returns error",
			opts: ValidateArtifactOptions{
				Subject: testImage,
			},
			store: &mockStore{
				tagToDesc: map[string]ocispec.Descriptor{
					testImage: {
						Digest: testDigest1,
					},
				},
				digestToReferrers: map[string][]ocispec.Descriptor{
					testArtifact1: {
						{
							Digest: testDigest2,
						},
					},
				},
			},
			verifiers: []Verifier{&mockVerifier{
				verifiable: true,
				verifyResult: map[string]*VerificationResult{
					testDigest2: {
						Description: validMessage1,
					},
				},
			}},
			policyEnforcer: &mockPolicyEnforcer{
				returnErr: true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "3-layer nested artifacts are verified",
			opts: ValidateArtifactOptions{
				Subject: testImage,
			},
			store: &mockStore{
				tagToDesc: map[string]ocispec.Descriptor{
					testImage: {
						Digest: testDigest1,
					},
				},
				digestToReferrers: map[string][]ocispec.Descriptor{
					testArtifact1: {
						{
							Digest: testDigest2,
						},
						{
							Digest: testDigest3,
						},
					},
					testArtifact2: {
						{
							Digest: testDigest4,
						},
					},
					testArtifact4: {
						{
							Digest: testDigest5,
						},
					},
				},
			},
			verifiers: []Verifier{&mockVerifier{
				verifiable: true,
				verifyResult: map[string]*VerificationResult{
					testDigest2: {
						Description: validMessage2,
					},
					testDigest3: {
						Description: validMessage3,
					},
					testDigest4: {
						Description: validMessage4,
					},
					testDigest5: {
						Description: validMessage5,
					},
				},
			},
			},
			policyEnforcer: &mockPolicyEnforcer{},
			want: &ValidationResult{
				Succeeded: true,
				ArtifactReports: []*ValidationReport{
					{
						Results: []*VerificationResult{
							{
								Description: validMessage3,
							},
						},
						ArtifactReports: []*ValidationReport{},
					},
					{
						Results: []*VerificationResult{
							{
								Description: validMessage2,
							},
						},
						ArtifactReports: []*ValidationReport{{
							Results: []*VerificationResult{
								{
									Description: validMessage4,
								},
							},
							ArtifactReports: []*ValidationReport{{
								Results: []*VerificationResult{
									{
										Description: validMessage5,
									},
								},
								ArtifactReports: []*ValidationReport{},
							}},
						}},
					},
				}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, _ := NewExecutor(tt.store, tt.verifiers, tt.policyEnforcer)
			got, err := executor.ValidateArtifact(context.Background(), tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArtifact() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !sameValidationResult(got, tt.want) {
				t.Errorf("ValidateArtifact() = %v, want %v", got, tt.want)
			}
		})
	}
}

func sameValidationResult(result1, result2 *ValidationResult) bool {
	if result1 == nil && result2 == nil {
		return true
	}
	if result1 == nil || result2 == nil {
		return false
	}

	if result1.Succeeded != result2.Succeeded {
		return false
	}
	if len(result1.ArtifactReports) != len(result2.ArtifactReports) {
		return false
	}
	for _, report := range result1.ArtifactReports {
		hasSameReport := false
		for _, report2 := range result2.ArtifactReports {
			if sameArtifactValidationReport(report, report2) {
				hasSameReport = true
				break
			}
		}
		if !hasSameReport {
			return false
		}
	}
	return true
}

func sameArtifactValidationReport(report1, report2 *ValidationReport) bool {
	if len(report1.Results) != len(report2.Results) {
		return false
	}
	for _, verifierReport := range report1.Results {
		hasSameReport := false
		for _, verifierReport2 := range report2.Results {
			if sameVerifierReport(verifierReport, verifierReport2) {
				hasSameReport = true
				break
			}
		}
		if !hasSameReport {
			return false
		}
	}
	if len(report1.ArtifactReports) != len(report2.ArtifactReports) {
		return false
	}
	for _, nestedReport := range report1.ArtifactReports {
		hasSameReport := false
		for _, nestedReport2 := range report2.ArtifactReports {
			if sameArtifactValidationReport(nestedReport, nestedReport2) {
				hasSameReport = true
				break
			}
		}
		if !hasSameReport {
			return false
		}
	}
	return true
}

func sameVerifierReport(report1, report2 *VerificationResult) bool {
	return report1.Err == report2.Err && report1.Description == report2.Description
}

func TestValidateExecutorSetup(t *testing.T) {
	tests := []struct {
		name    string
		store   Store
		verifiers []Verifier
		wantErr bool
	}{
		{
			name:    "Store is not set",
			store:   nil,
			verifiers: []Verifier{&mockVerifier{}},
			wantErr: true,
		},
		{
			name:    "Verifiers are not set",
			store:   &mockStore{},
			verifiers: nil,
			wantErr: true,
		},
		{
			name:    "All components are set",
			store:   &mockStore{},
			verifiers: []Verifier{&mockVerifier{}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExecutorSetup(tt.store, tt.verifiers)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateExecutorSetup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}