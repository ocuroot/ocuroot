package sdk

import (
	"testing"
)

func TestPackageValidate(t *testing.T) {
	// Test cases
	tests := []struct {
		name           string
		pkg            Package
		expectedErrors int
	}{
		{
			name: "Valid package - all environments in exactly one phase",
			pkg: Package{
				Phases: []Phase{
					{
						Name: "development",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "dev",
								},
							},
						},
					},
					{
						Name: "staging",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "staging",
								},
							},
						},
					},
					{
						Name: "production",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "prod",
								},
							},
						},
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Valid package - multiple work items in same phase with different names",
			pkg: Package{
				Phases: []Phase{
					{
						Name: "development",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "dev",
								},
							},
							{
								Call: &WorkCall{
									Name: "qa-approval",
									Fn: FunctionDef{
										Name: "approve_function",
									},
								},
							},
							{
								Call: &WorkCall{
									Name: "stability-period",
									Fn: FunctionDef{
										Name: "delay_function",
									},
								},
							},
							{
								Call: &WorkCall{
									Name: "notify-deployment",
									Fn: FunctionDef{
										Name: "handoff_function",
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Valid package - different work items across multiple phases",
			pkg: Package{
				Phases: []Phase{
					{
						Name: "development",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "dev",
								},
							},
							{
								Call: &WorkCall{
									Name: "dev-approval",
									Fn: FunctionDef{
										Name: "dev_approve_function",
									},
								},
							},
							{
								Call: &WorkCall{
									Name: "dev-notification",
									Fn: FunctionDef{
										Name: "dev_handoff_function",
									},
								},
							},
						},
					},
					{
						Name: "staging",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "staging",
								},
							},
							{
								Call: &WorkCall{
									Name: "staging-delay",
									Fn: FunctionDef{
										Name: "staging_delay_function",
									},
								},
							},
							{
								Call: &WorkCall{
									Name: "staging-approval",
									Fn: FunctionDef{
										Name: "staging_approve_function",
									},
								},
							},
						},
					},
					{
						Name: "production",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "prod",
								},
							},
							{
								Call: &WorkCall{
									Name: "prod-approval",
									Fn: FunctionDef{
										Name: "prod_approve_function",
									},
								},
							},
							{
								Call: &WorkCall{
									Name: "prod-notification",
									Fn: FunctionDef{
										Name: "prod_handoff_function",
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Invalid package - environment in multiple phases",
			pkg: Package{
				Phases: []Phase{
					{
						Name: "development",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "dev",
								},
							},
						},
					},
					{
						Name: "staging",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "staging",
								},
							},
							{
								Deployment: &Deployment{
									Environment: "prod",
								},
							},
						},
					},
					{
						Name: "production",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "prod",
								},
							},
						},
					},
				},
			},
			expectedErrors: 1, // One error for the duplicate environment
		},
		{
			name: "Invalid package - duplicate approval work names",
			pkg: Package{
				Phases: []Phase{
					{
						Name: "phase-one",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "dev",
								},
							},
							{
								Call: &WorkCall{
									Name: "approve-deploy",
									Fn: FunctionDef{
										Name: "approve_function",
									},
								},
							},
						},
					},
					{
						Name: "phase-two",
						Work: []Work{
							{
								Call: &WorkCall{
									Name: "approve-deploy", // Same name as previous approval
									Fn: FunctionDef{
										Name: "another_approve_function",
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: 1, // One error for the duplicate approval name
		},
		{
			name: "Invalid package - duplicate delay work names",
			pkg: Package{
				Phases: []Phase{
					{
						Name: "phase-one",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "dev",
								},
							},
							{
								Call: &WorkCall{
									Name: "wait-period",
									Fn: FunctionDef{
										Name: "delay_function",
									},
								},
							},
						},
					},
					{
						Name: "phase-two",
						Work: []Work{
							{
								Call: &WorkCall{
									Name: "wait-period", // Same name as previous delay
									Fn: FunctionDef{
										Name: "another_delay_function",
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: 1, // One error for the duplicate delay name
		},
		{
			name: "Invalid package - duplicate handoff work names",
			pkg: Package{
				Phases: []Phase{
					{
						Name: "phase-one",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "dev",
								},
							},
							{
								Call: &WorkCall{
									Name: "notify-team",
									Fn: FunctionDef{
										Name: "handoff_function",
									},
								},
							},
						},
					},
					{
						Name: "phase-two",
						Work: []Work{
							{
								Call: &WorkCall{
									Name: "notify-team", // Same name as previous handoff
									Fn: FunctionDef{
										Name: "another_handoff_function",
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: 1, // One error for the duplicate handoff name
		},
		{
			name: "Invalid package - duplicate names across different work types",
			pkg: Package{
				Phases: []Phase{
					{
						Name: "phase-one",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "dev",
								},
							},
							{
								Call: &WorkCall{
									Name: "shared-name",
									Fn: FunctionDef{
										Name: "approval_function",
									},
								},
							},
						},
					},
					{
						Name: "phase-two",
						Work: []Work{
							{
								Call: &WorkCall{
									Name: "shared-name", // Same name as approval in previous phase
									Fn: FunctionDef{
										Name: "delay_function",
									},
								},
							},
						},
					},
					{
						Name: "phase-three",
						Work: []Work{
							{
								Call: &WorkCall{
									Name: "shared-name", // Same name as approval and delay
									Fn: FunctionDef{
										Name: "handoff_function",
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: 1, // One error for the duplicate name across different work types
		},
		{
			name: "Invalid package - multiple duplicate work names",
			pkg: Package{
				Phases: []Phase{
					{
						Name: "phase-one",
						Work: []Work{
							{
								Deployment: &Deployment{
									Environment: "dev",
								},
							},
							{
								Call: &WorkCall{
									Name: "duplicate-one",
									Fn: FunctionDef{
										Name: "approval_function",
									},
								},
							},
							{
								Call: &WorkCall{
									Name: "duplicate-two",
									Fn: FunctionDef{
										Name: "delay_function",
									},
								},
							},
						},
					},
					{
						Name: "phase-two",
						Work: []Work{
							{
								Call: &WorkCall{
									Name: "duplicate-one", // Duplicate approval name
									Fn: FunctionDef{
										Name: "another_approval_function",
									},
								},
							},
							{
								Call: &WorkCall{
									Name: "duplicate-two", // Duplicate with delay name
									Fn: FunctionDef{
										Name: "handoff_function",
									},
								},
							},
						},
					},
				},
			},
			expectedErrors: 2, // Two errors for duplicate work names
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := tt.pkg.Validate()

			if len(errors) != tt.expectedErrors {
				t.Errorf("Package.Validate() = got %d errors, want %d errors", len(errors), tt.expectedErrors)
				for i, err := range errors {
					t.Logf("Error %d: %s", i+1, err.Error())
				}
			}
		})
	}
}
