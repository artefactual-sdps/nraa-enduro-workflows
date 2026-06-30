package activities_test

import (
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

func TestIdentifySIP(t *testing.T) {
	t.Parallel()

	path := fs.NewDir(t, "",
		fs.WithDir("SIP_20240606_NRAA_2024_001",
			fs.WithDir("content"),
			fs.WithDir("header"),
		),
	).Join("SIP_20240606_NRAA_2024_001")

	tests := []struct {
		name    string
		params  activities.IdentifySIPParams
		result  activities.IdentifySIPResult
		wantErr string
	}{
		{
			name:   "Identifies a SIP",
			params: activities.IdentifySIPParams{Path: path},
			result: activities.IdentifySIPResult{
				SIP: sip.SIP{
					Path:         path,
					ContentPath:  filepath.Join(path, "content"),
					HeaderPath:   filepath.Join(path, "header"),
					ManifestPath: filepath.Join(path, "header", "metadata.xml"),
					MetadataPath: filepath.Join(path, "header", "metadata.xml"),
					XSDPath:      filepath.Join(path, "header", "metadata.xsd"),
					TopLevelPaths: []string{
						filepath.Join(path, "content"),
						filepath.Join(path, "header"),
					},
				},
			},
		},
		{
			name:    "Fails to identify a non existing path",
			wantErr: "IdentifySIP: SIP: New: stat",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewIdentifySIP().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.IdentifySIPName},
			)

			var res activities.IdentifySIPResult
			future, err := env.ExecuteActivity(activities.IdentifySIPName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			assert.NilError(t, future.Get(&res))
			assert.DeepEqual(t, res, tt.result)
		})
	}
}
