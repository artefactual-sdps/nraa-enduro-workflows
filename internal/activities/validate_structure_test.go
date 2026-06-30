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

const nraaSIPName = "SIP_20240606_NRAA_2024_001"

func nraaSIP(t *testing.T, ops ...fs.PathOp) sip.SIP {
	t.Helper()

	path := fs.NewDir(t, "extract-",
		fs.WithDir(nraaSIPName, ops...),
	).Join(nraaSIPName)

	return testSIP(t, path)
}

func validNRAASIP(t *testing.T) sip.SIP {
	t.Helper()

	return nraaSIP(t, validContent(t), validHeader(t))
}

func unexpectedNamesSIP(t *testing.T) sip.SIP {
	t.Helper()

	return nraaSIP(t,
		fs.WithFile("d000002.txt", ""),
		fs.WithDir("extra",
			fs.WithFile("d000003.txt", ""),
		),
		validContent(t),
		validHeader(t),
	)
}

func missingRequiredSIP(t *testing.T) sip.SIP {
	t.Helper()

	return nraaSIP(t,
		fs.WithDir("header",
			fs.WithFile("metadata.xml", `<?xml version="1.0" encoding="UTF-8"?><metadata/>`),
		),
	)
}

func badNamesSIP(t *testing.T) sip.SIP {
	t.Helper()

	return nraaSIP(t,
		fs.WithDir("content",
			fs.WithDir("bad",
				fs.WithFile("d000003.txt", "content"),
			),
			fs.WithDir("f000001",
				fs.WithFile("d000002", "content"),
				fs.WithFile("d000003.t-xt", "content"),
				fs.WithFile("d000004.t_xt", "content"),
			),
		),
		fs.WithDir("header",
			fs.WithFile("content!.txt", ""),
			fs.WithFile("metadata.xml", `<?xml version="1.0" encoding="UTF-8"?><metadata/>`),
			fs.WithFile("metadata.xsd", `<?xml version="1.0" encoding="UTF-8"?><schema/>`),
		),
	)
}

func duplicateNamesSIP(t *testing.T) sip.SIP {
	t.Helper()

	return nraaSIP(t,
		fs.WithDir("content",
			fs.WithDir("f000001",
				fs.WithFile("d000001.txt", "content"),
				fs.WithDir("f000001",
					fs.WithFile("d000002.txt", "content"),
				),
			),
			fs.WithDir("f000002",
				fs.WithFile("d000001.txt", "content"),
				fs.WithDir("f000001",
					fs.WithFile("d000003.txt", "content"),
				),
			),
			fs.WithDir("f000003",
				fs.WithFile("d000001.txt", "content"),
			),
		),
		validHeader(t),
	)
}

func validContent(t *testing.T) fs.PathOp {
	t.Helper()

	return fs.WithDir("content",
		fs.WithDir("f000001",
			fs.WithFile("d000001.txt", "content"),
		),
	)
}

func validHeader(t *testing.T) fs.PathOp {
	t.Helper()

	return fs.WithDir("header",
		fs.WithFile("metadata.xml", `<?xml version="1.0" encoding="UTF-8"?><metadata/>`),
		fs.WithFile("metadata.xsd", `<?xml version="1.0" encoding="UTF-8"?><schema/>`),
	)
}

func contentDirNames(t *testing.T, count int) []string {
	t.Helper()

	names := make([]string, count)
	for i := range count {
		names[i] = fmt.Sprintf("f%06d", i+1)
	}

	return names
}

func nestedContentDirs(t *testing.T, names []string, ops ...fs.PathOp) fs.PathOp {
	t.Helper()

	for i := len(names) - 1; i >= 0; i-- {
		ops = []fs.PathOp{fs.WithDir(names[i], ops...)}
	}

	return fs.WithDir("content", ops...)
}

func sipFixturePath(t *testing.T, parts ...string) string {
	t.Helper()

	parts = append([]string{nraaSIPName}, parts...)

	return filepath.Join(parts...)
}

func TestValidateStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		params  activities.ValidateStructureParams
		want    activities.ValidateStructureResult
		wantErr string
	}{
		{
			name: "Validates an NRAA SIP",
			params: activities.ValidateStructureParams{
				SIP: validNRAASIP(t),
			},
		},
		{
			name: "Returns failures when a SIP is empty",
			params: activities.ValidateStructureParams{
				SIP: nraaSIP(t),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					"The SIP is empty",
				},
			},
		},
		{
			name: "Returns failures when a SIP has a single file",
			params: activities.ValidateStructureParams{
				SIP: nraaSIP(t,
					fs.WithFile("file.txt", ""),
				),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					"Content folder is missing",
					"Header folder is missing",
					"metadata.xml is missing",
					"metadata.xsd is missing",
					`Unexpected file: "file.txt"`,
				},
			},
		},
		{
			name: "Returns failures when a SIP has unexpected files or directories",
			params: activities.ValidateStructureParams{
				SIP: unexpectedNamesSIP(t),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					`Unexpected directory: "extra"`,
					`Unexpected file: "d000002.txt"`,
				},
			},
		},
		{
			name: "Returns failures when a SIP is missing files or directories",
			params: activities.ValidateStructureParams{
				SIP: missingRequiredSIP(t),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					"Content folder is missing",
					"metadata.xsd is missing",
				},
			},
		},
		{
			name: "Returns failures when an NRAA SIP has bad file or directory names",
			params: activities.ValidateStructureParams{
				SIP: badNamesSIP(t),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					fmt.Sprintf("Unexpected file: %q", filepath.Join("header", "content!.txt")),
					fmt.Sprintf(
						"Content directory %q does not match naming convention %q",
						filepath.Join("content", "bad"),
						"f000000",
					),
					fmt.Sprintf(
						"File %q does not match naming convention %q",
						filepath.Join("content", "f000001", "d000002"),
						"d000000.ext",
					),
					fmt.Sprintf(
						"File %q does not match naming convention %q",
						filepath.Join("content", "f000001", "d000003.t-xt"),
						"d000000.ext",
					),
					fmt.Sprintf(
						"File %q does not match naming convention %q",
						filepath.Join("content", "f000001", "d000004.t_xt"),
						"d000000.ext",
					),
				},
			},
		},
		{
			name: "Returns failures when an NRAA SIP has duplicate names",
			params: activities.ValidateStructureParams{
				SIP: duplicateNamesSIP(t),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					fmt.Sprintf(
						"Folder name is not unique: %q is used by both %q and %q",
						"f000001",
						filepath.Join("content", "f000001"),
						filepath.Join("content", "f000001", "f000001"),
					),
					fmt.Sprintf(
						"Folder name is not unique: %q is used by both %q and %q",
						"f000001",
						filepath.Join("content", "f000001"),
						filepath.Join("content", "f000002", "f000001"),
					),
					fmt.Sprintf(
						"File name is not unique: %q is used by both %q and %q",
						"d000001.txt",
						filepath.Join("content", "f000001", "d000001.txt"),
						filepath.Join("content", "f000002", "d000001.txt"),
					),
					fmt.Sprintf(
						"File name is not unique: %q is used by both %q and %q",
						"d000001.txt",
						filepath.Join("content", "f000001", "d000001.txt"),
						filepath.Join("content", "f000003", "d000001.txt"),
					),
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
				activities.NewValidateStructure().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ValidateStructureName},
			)

			enc, err := env.ExecuteActivity(activities.ValidateStructureName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			var result activities.ValidateStructureResult
			assert.NilError(t, enc.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}

func TestValidateStructureReportsLongPaths(t *testing.T) {
	t.Parallel()

	folderNames := contentDirNames(t, 28)
	fileNames := contentDirNames(t, 26)

	tests := []struct {
		name   string
		params activities.ValidateStructureParams
		want   activities.ValidateStructureResult
	}{
		{
			name: "Reports long folder paths",
			params: activities.ValidateStructureParams{
				SIP: nraaSIP(t,
					nestedContentDirs(t, folderNames),
					validHeader(t),
				),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					fmt.Sprintf(
						`Path %q exceeds the 250 character limit`,
						// Path-length validation includes the SIP root directory name.
						sipFixturePath(t, append([]string{"content"}, folderNames...)...),
					),
				},
			},
		},
		{
			name: "Reports long file paths",
			params: activities.ValidateStructureParams{
				SIP: nraaSIP(t,
					nestedContentDirs(t, fileNames, fs.WithFile("d000001.txt", "content")),
					validHeader(t),
				),
			},
			want: activities.ValidateStructureResult{
				Failures: []string{
					fmt.Sprintf(
						`Path %q exceeds the 250 character limit`,
						// Path-length validation includes the SIP root directory name.
						sipFixturePath(t, append(append([]string{"content"}, fileNames...), "d000001.txt")...),
					),
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
				activities.NewValidateStructure().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ValidateStructureName},
			)

			enc, err := env.ExecuteActivity(activities.ValidateStructureName, tt.params)
			assert.NilError(t, err)

			var result activities.ValidateStructureResult
			assert.NilError(t, enc.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
