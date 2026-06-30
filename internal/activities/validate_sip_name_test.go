package activities_test

import (
	"fmt"
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

func TestValidateSIPName(t *testing.T) {
	t.Parallel()

	validSIPName := "SIP_20240606_NRAA_2024_001"
	validSIP := testNamedSIP(t, validSIPName)

	badDateName := "SIP_20240230_NRAA_2024_001"
	badDateSIP := testNamedSIP(t, badDateName)

	shortCodeName := "SIP_20240606_N_2024_001"
	shortCodeSIP := testNamedSIP(t, shortCodeName)

	longCodeName := "SIP_20240606_NRAAAA_2024_001"
	longCodeSIP := testNamedSIP(t, longCodeName)

	badSequenceName := "SIP_20240606_NRAA_2024_01"
	badSequenceSIP := testNamedSIP(t, badSequenceName)

	tests := []struct {
		name    string
		params  activities.ValidateSIPNameParams
		want    activities.ValidateSIPNameResult
		wantErr string
	}{
		{
			name:   "Validates a valid NRAA SIP name",
			params: activities.ValidateSIPNameParams{SIP: validSIP},
		},
		{
			name:   "Reports an invalid date",
			params: activities.ValidateSIPNameParams{SIP: badDateSIP},
			want: activities.ValidateSIPNameResult{
				Failures: []string{
					fmt.Sprintf("SIP name %q violates naming standard", badDateName),
				},
			},
		},
		{
			name:   "Reports a short code",
			params: activities.ValidateSIPNameParams{SIP: shortCodeSIP},
			want: activities.ValidateSIPNameResult{
				Failures: []string{
					fmt.Sprintf("SIP name %q violates naming standard", shortCodeName),
				},
			},
		},
		{
			name:   "Reports a long code",
			params: activities.ValidateSIPNameParams{SIP: longCodeSIP},
			want: activities.ValidateSIPNameResult{
				Failures: []string{
					fmt.Sprintf("SIP name %q violates naming standard", longCodeName),
				},
			},
		},
		{
			name:   "Reports a bad sequence",
			params: activities.ValidateSIPNameParams{SIP: badSequenceSIP},
			want: activities.ValidateSIPNameResult{
				Failures: []string{
					fmt.Sprintf("SIP name %q violates naming standard", badSequenceName),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewValidateSIPName().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ValidateSIPNameName},
			)

			enc, err := env.ExecuteActivity(activities.ValidateSIPNameName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			var result activities.ValidateSIPNameResult
			assert.NilError(t, enc.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}

func testNamedSIP(t *testing.T, name string) sip.SIP {
	t.Helper()

	path := fs.NewDir(t, "", fs.WithDir(name)).Join(name)
	s, err := sip.New(filepath.Clean(path))
	assert.NilError(t, err)

	return s
}
