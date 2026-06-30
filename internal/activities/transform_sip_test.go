package activities_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/pips"
)

func TestTransformSIP(t *testing.T) {
	t.Parallel()

	var (
		dmode = os.FileMode(0o700)
		fmode = os.FileMode(0o600)
	)

	validSIPPath := fs.NewDir(t, "",
		fs.WithDir("SIP_20240606_NRAA_2024_001",
			fs.WithDir("content",
				fs.WithDir("f000001",
					fs.WithFile("d000001.txt", ""),
				),
			),
			fs.WithDir("header",
				fs.WithFile("metadata.xml", ""),
				fs.WithFile("metadata.xsd", ""),
			),
		),
	).Join("SIP_20240606_NRAA_2024_001")
	validSIP := testSIP(t, validSIPPath)

	missingMetadataSIP := testSIP(t, fs.NewDir(t, "",
		fs.WithDir("SIP_20240606_NRAA_2024_001",
			fs.WithDir("content",
				fs.WithDir("f000001",
					fs.WithFile("d000001.txt", ""),
				),
			),
			fs.WithDir("header"),
		),
	).Join("SIP_20240606_NRAA_2024_001"))

	missingContentSIP := testSIP(t, fs.NewDir(t, "",
		fs.WithDir("SIP_20240606_NRAA_2024_001",
			fs.WithDir("header",
				fs.WithFile("metadata.xml", ""),
			),
		),
	).Join("SIP_20240606_NRAA_2024_001"))

	expectedSIP := fs.Expected(t,
		fs.WithDir("objects", fs.WithMode(dmode),
			fs.WithDir(filepath.Base(validSIPPath), fs.WithMode(dmode),
				fs.WithDir("content", fs.WithMode(dmode),
					fs.WithDir("f000001", fs.WithMode(dmode),
						fs.WithFile("d000001.txt", "", fs.WithMode(fmode)),
					),
				),
				fs.WithDir("header", fs.WithMode(dmode),
					fs.WithFile("metadata.xml", "", fs.WithMode(fmode)),
				),
			),
		),
		fs.WithDir("metadata", fs.WithMode(dmode)),
	)

	tests := []struct {
		name    string
		params  activities.TransformSIPParams
		want    activities.TransformSIPResult
		wantSIP fs.Manifest
		wantErr string
	}{
		{
			name:    "Transforms an NRAA SIP",
			params:  activities.TransformSIPParams{SIP: validSIP},
			want:    activities.TransformSIPResult{PIP: pips.NewFromSIP(validSIP)},
			wantSIP: expectedSIP,
		},
		{
			name:   "Fails when the metadata file is missing",
			params: activities.TransformSIPParams{SIP: missingMetadataSIP},
			wantErr: fmt.Sprintf(
				"rename %s/header/metadata.xml %s/objects/%s/header/metadata.xml",
				missingMetadataSIP.Path,
				missingMetadataSIP.Path,
				filepath.Base(missingMetadataSIP.Path),
			),
		},
		{
			name:    "Fails when the content directory is missing",
			params:  activities.TransformSIPParams{SIP: missingContentSIP},
			wantErr: "no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewTransformSIP().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.TransformSIPName},
			)

			future, err := env.ExecuteActivity(activities.TransformSIPName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			var result activities.TransformSIPResult
			assert.NilError(t, future.Get(&result))
			assert.DeepEqual(t, result, tt.want)
			assert.Assert(t, fs.Equal(tt.params.SIP.Path, tt.wantSIP))
		})
	}
}
