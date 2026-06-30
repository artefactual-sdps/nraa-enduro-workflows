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

func TestWriteIdentifierFile(t *testing.T) {
	t.Parallel()

	pipWithManifest := pips.New(
		fs.NewDir(t, "",
			fs.WithDir("SIP_20240606_NRAA_2024_001",
				fs.WithDir("metadata"),
				fs.WithDir("objects",
					fs.WithDir("SIP_20240606_NRAA_2024_001",
						fs.WithDir("content",
							fs.WithDir("f000001",
								fs.WithFile("d000001.txt", ""),
								fs.WithFile("d000002.pdf", ""),
							),
						),
						fs.WithDir("header",
							fs.WithFile("metadata.xml", sipMetadata),
							fs.WithFile("metadata.xsd", ""),
						),
					),
				),
			),
		).Join("SIP_20240606_NRAA_2024_001"),
	)

	pipNoManifest := pips.New(fs.NewDir(t, "").Path())

	pipEmptyManifest := pips.New(
		fs.NewDir(t, "",
			fs.WithDir("SIP_20240606_NRAA_2024_001",
				fs.WithDir("metadata"),
				fs.WithDir("objects",
					fs.WithDir("SIP_20240606_NRAA_2024_001",
						fs.WithDir("header",
							fs.WithFile("metadata.xml", ""),
						),
					),
				),
			),
		).Join("SIP_20240606_NRAA_2024_001"),
	)

	pipReadOnly := pips.New(
		fs.NewDir(t, "",
			fs.WithDir("SIP_20240606_NRAA_2024_001",
				fs.WithDir("metadata", fs.WithMode(0o400)),
				fs.WithDir("objects",
					fs.WithDir("SIP_20240606_NRAA_2024_001",
						fs.WithDir("content",
							fs.WithDir("f000001",
								fs.WithFile("d000001.txt", ""),
							),
						),
						fs.WithDir("header",
							fs.WithFile("metadata.xml", sipMetadata),
						),
					),
				),
			),
		).Join("SIP_20240606_NRAA_2024_001"),
	)

	tests := []struct {
		name     string
		params   activities.WriteIdentifierFileParams
		wantJSON string
		wantErr  string
	}{
		{
			name: "Writes an NRAA SIP identifier file",
			params: activities.WriteIdentifierFileParams{
				PIP: pipWithManifest,
			},
			wantJSON: `[
    {
        "file": "objects/SIP_20240606_NRAA_2024_001/content/f000001/d000001.txt",
        "identifiers": [
            {
                "identifier": "d000001",
                "identifierType": "local"
            }
        ]
    },
    {
        "file": "objects/SIP_20240606_NRAA_2024_001/content/f000001/d000002.pdf",
        "identifiers": [
            {
                "identifier": "d000002",
                "identifierType": "local"
            }
        ]
    },
    {
        "file": "objects/SIP_20240606_NRAA_2024_001/header/metadata.xml",
        "identifiers": [
            {
                "identifier": "metadata",
                "identifierType": "local"
            }
        ]
    }
]`,
		},
		{
			name: "Errors when manifest is not readable",
			params: activities.WriteIdentifierFileParams{
				PIP: pipNoManifest,
			},
			wantErr: fmt.Sprintf(
				"write identifier file: open manifest: open %s: no such file or directory",
				pipNoManifest.ManifestPath,
			),
		},
		{
			name: "Errors when manifest is invalid",
			params: activities.WriteIdentifierFileParams{
				PIP: pipEmptyManifest,
			},
			wantErr: "write identifier file: get manifest identifiers: no files in manifest",
		},
		{
			name: "Errors when metadata path is not writable",
			params: activities.WriteIdentifierFileParams{
				PIP: pipReadOnly,
			},
			wantErr: fmt.Sprintf(
				"write identifier file: write identifiers.json: open %s: permission denied",
				pipReadOnly.Path+"/metadata/identifiers.json",
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewWriteIdentifierFile().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.WriteIdentifierFileName},
			)

			future, err := env.ExecuteActivity(activities.WriteIdentifierFileName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.Error(
						t,
						err,
						"activity error (type: write-identifier-file, scheduledEventID: 0, startedEventID: 0, identity: ): "+tt.wantErr,
					)
				}

				return
			}
			assert.NilError(t, err)

			var res activities.WriteIdentifierFileResult
			assert.NilError(t, future.Get(&res))
			p := filepath.Join(tt.params.PIP.Path, "metadata", "identifiers.json")
			assert.DeepEqual(t, res, activities.WriteIdentifierFileResult{Path: p})

			b, err := os.ReadFile(p)
			assert.NilError(t, err)
			assert.Equal(t, string(b), tt.wantJSON)
		})
	}
}

const sipMetadata = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://nraa.gov.om/sip/v1" schemaVersion="1.0">
    <packageType>SIP</packageType>
    <toc>
        <folder>
            <name>header</name>
            <originalName>header</originalName>
            <digitalObject id="metadata">
                <name>metadata.xml</name>
                <originalName>metadata.xml</originalName>
                <checksumAlgorithm>MD5</checksumAlgorithm>
                <checksum>aaa111</checksum>
            </digitalObject>
            <digitalObject id="metadata.xsd">
                <name>metadata.xsd</name>
                <originalName>metadata.xsd</originalName>
                <checksumAlgorithm>MD5</checksumAlgorithm>
                <checksum>bbb222</checksum>
            </digitalObject>
        </folder>
        <folder>
            <name>content</name>
            <originalName>content</originalName>
            <folder>
                <name>f000001</name>
                <originalName>f000001</originalName>
                <digitalObject id="d000002">
                    <name>d000002.pdf</name>
                    <originalName>d000002.pdf</originalName>
                    <checksumAlgorithm>MD5</checksumAlgorithm>
                    <checksum>ccc333</checksum>
                </digitalObject>
                <digitalObject id="d000001">
                    <name>d000001.txt</name>
                    <originalName>d000001.txt</originalName>
                    <checksumAlgorithm>MD5</checksumAlgorithm>
                    <checksum>ddd444</checksum>
                </digitalObject>
            </folder>
        </folder>
    </toc>
</package>`
