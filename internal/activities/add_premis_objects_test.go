package activities_test

import (
	pseudorand "math/rand"
	"os"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/activities"
)

func TestAddPREMISObjects(t *testing.T) {
	t.Parallel()

	contentFilesNormal := fs.NewDir(t, "",
		fs.WithDir("SIP_20240606_NRAA_2024_001",
			fs.WithDir("header",
				fs.WithFile("metadata.xml", sipMetadata),
				fs.WithFile("metadata.xsd", ""),
			),
			fs.WithDir("content",
				fs.WithDir("f000001",
					fs.WithFile("d000001.txt", ""),
					fs.WithFile("d000002.pdf", ""),
				),
			),
		),
	)
	premisFilePathNormal := contentFilesNormal.Join("SIP_20240606_NRAA_2024_001", "metadata", "premis.xml")

	tests := []struct {
		name       string
		params     activities.AddPREMISObjectsParams
		result     activities.AddPREMISObjectsResult
		wantPREMIS string
		wantErr    string
	}{
		{
			name: "Add PREMIS objects for normal content",
			params: activities.AddPREMISObjectsParams{
				SIP:            testSIP(t, contentFilesNormal.Join("SIP_20240606_NRAA_2024_001")),
				PREMISFilePath: premisFilePathNormal,
			},
			result: activities.AddPREMISObjectsResult{},
			wantPREMIS: `<?xml version="1.0" encoding="UTF-8"?>
<premis:premis xmlns:premis="http://www.loc.gov/premis/v3" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd" version="3.0">
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>UUID</premis:objectIdentifierType>
      <premis:objectIdentifierValue>52fdfc07-2182-454f-963f-5f0f9a621d72</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/SIP_20240606_NRAA_2024_001/content/f000001/d000001.txt</premis:originalName>
  </premis:object>
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>UUID</premis:objectIdentifierType>
      <premis:objectIdentifierValue>9566c74d-1003-4c4d-bbbb-0407d1e2c649</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/SIP_20240606_NRAA_2024_001/content/f000001/d000002.pdf</premis:originalName>
  </premis:object>
</premis:premis>
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			rng := pseudorand.New(pseudorand.NewSource(1)) // #nosec G404
			env.RegisterActivityWithOptions(
				activities.NewAddPREMISObjects(rng).Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISObjectsName},
			)

			var res activities.AddPREMISObjectsResult
			future, err := env.ExecuteActivity(activities.AddPREMISObjectsName, tt.params)
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

			b, err := os.ReadFile(tt.params.PREMISFilePath)
			assert.NilError(t, err)
			assert.Equal(t, string(b), tt.wantPREMIS)
		})
	}
}
