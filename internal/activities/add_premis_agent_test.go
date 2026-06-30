package activities_test

import (
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/premis"
)

func TestAddPREMISAgent(t *testing.T) {
	t.Parallel()

	contentFilesNormal := fs.NewDir(t, "",
		fs.WithDir("metadata"),
	)
	premisFilePathNormal := contentFilesNormal.Join("metadata", "premis.xml")

	contentNoFiles := fs.NewDir(t, "",
		fs.WithDir("metadata"),
	)
	premisFilePathNoFiles := contentNoFiles.Join("metadata", "premis.xml")

	contentNonExistent := fs.NewDir(t, "",
		fs.WithDir("metadata"),
	)
	premisFilePathNonExistent := contentNonExistent.Join("metadata", "premis.xml")
	contentNonExistent.Remove()

	tests := []struct {
		name    string
		params  activities.AddPREMISAgentParams
		result  activities.AddPREMISAgentResult
		wantErr string
	}{
		{
			name: "Add PREMIS agent for normal content",
			params: activities.AddPREMISAgentParams{
				PREMISFilePath: premisFilePathNormal,
				Agent:          premis.AgentDefault(),
			},
			result: activities.AddPREMISAgentResult{},
		},
		{
			name: "Add PREMIS agent for no content",
			params: activities.AddPREMISAgentParams{
				PREMISFilePath: premisFilePathNoFiles,
				Agent:          premis.AgentDefault(),
			},
			result: activities.AddPREMISAgentResult{},
		},
		{
			name: "Add PREMIS agent for bad path",
			params: activities.AddPREMISAgentParams{
				PREMISFilePath: premisFilePathNonExistent,
				Agent:          premis.AgentDefault(),
			},
			result:  activities.AddPREMISAgentResult{},
			wantErr: "no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewAddPREMISAgent().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISAgentName},
			)

			var res activities.AddPREMISAgentResult
			future, err := env.ExecuteActivity(activities.AddPREMISAgentName, tt.params)
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

			_, err = premis.ParseFile(tt.params.PREMISFilePath)
			assert.NilError(t, err)
		})
	}
}
