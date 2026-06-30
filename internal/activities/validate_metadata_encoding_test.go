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
)

func TestValidateMetadataEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params activities.ValidateMetadataEncodingParams
		want   activities.ValidateMetadataEncodingResult
	}{
		{
			name: "Validates UTF-8 metadata files",
			params: activities.ValidateMetadataEncodingParams{
				SIP: validNRAASIP(t),
			},
		},
		{
			name: "Returns failures when metadata files are not UTF-8",
			params: activities.ValidateMetadataEncodingParams{
				SIP: nraaSIP(t,
					validContent(t),
					fs.WithDir("header",
						fs.WithFile("metadata.xml", `<?xml version="1.0" encoding="ISO-8859-1"?><metadata/>`),
						fs.WithFile("metadata.xsd", string([]byte{0xff})),
					),
				),
			},
			want: activities.ValidateMetadataEncodingResult{
				Failures: []string{
					fmt.Sprintf("%q must use UTF-8 encoding", filepath.Join("header", "metadata.xml")),
					fmt.Sprintf("%q must use UTF-8 encoding", filepath.Join("header", "metadata.xsd")),
				},
			},
		},
		{
			name: "Returns failures when metadata encoding is empty",
			params: activities.ValidateMetadataEncodingParams{
				SIP: nraaSIP(t,
					validContent(t),
					fs.WithDir("header",
						fs.WithFile("metadata.xml", `<?xml version="1.0" encoding=""?><metadata/>`),
						fs.WithFile("metadata.xsd", `<?xml version="1.0" encoding="UTF-8"?><schema/>`),
					),
				),
			},
			want: activities.ValidateMetadataEncodingResult{
				Failures: []string{
					fmt.Sprintf("%q must use UTF-8 encoding", filepath.Join("header", "metadata.xml")),
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
				activities.NewValidateMetadataEncoding().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ValidateMetadataEncodingName},
			)

			enc, err := env.ExecuteActivity(activities.ValidateMetadataEncodingName, tt.params)
			assert.NilError(t, err)

			var result activities.ValidateMetadataEncodingResult
			assert.NilError(t, enc.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
