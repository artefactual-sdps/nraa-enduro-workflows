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

const (
	verifyManifestMetadata = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://nraa.gov.om/sip/v1" schemaVersion="1.0">
    <packageType>SIP</packageType>
    <toc>
        <folder>
            <name>header</name>
            <digitalObject id="metadata.xsd">
                <name>metadata.xsd</name>
                <checksumAlgorithm>MD5</checksumAlgorithm>
                <checksum>d41d8cd98f00b204e9800998ecf8427e</checksum>
            </digitalObject>
        </folder>
        <folder>
            <name>content</name>
            <folder>
                <name>f000001</name>
                <digitalObject id="d000001">
                    <name>d000001.txt</name>
                    <checksumAlgorithm>MD5</checksumAlgorithm>
                    <checksum>d41d8cd98f00b204e9800998ecf8427e</checksum>
                </digitalObject>
                <digitalObject id="d000002">
                    <name>d000002.pdf</name>
                    <checksumAlgorithm>MD5</checksumAlgorithm>
                    <checksum>d41d8cd98f00b204e9800998ecf8427e</checksum>
                </digitalObject>
            </folder>
        </folder>
    </toc>
</package>
`

	unsupportedSchemaManifest = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://nraa.gov.om/sip/v1" schemaVersion="2.0">
    <packageType>SIP</packageType>
    <toc></toc>
</package>
`

	unsupportedHashManifest = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://nraa.gov.om/sip/v1" schemaVersion="1.0">
    <packageType>SIP</packageType>
    <toc>
        <folder>
            <name>content</name>
            <folder>
                <name>f000001</name>
                <digitalObject id="d000001">
                    <name>d000001.txt</name>
                    <checksumAlgorithm>SHA-999</checksumAlgorithm>
                    <checksum>abc123</checksum>
                </digitalObject>
            </folder>
        </folder>
    </toc>
</package>
`
)

func TestVerifyManifest(t *testing.T) {
	t.Parallel()

	missingFilesSIP := fs.NewDir(t, "",
		fs.WithDir("SIP_20240606_NRAA_2024_001",
			fs.WithDir("content",
				fs.WithDir("f000001",
					fs.WithFile("d000002.pdf", ""),
				),
			),
			fs.WithDir("header",
				fs.WithFile("metadata.xml", verifyManifestMetadata),
			),
		),
	).Join("SIP_20240606_NRAA_2024_001")

	extraFilesSIP := fs.NewDir(t, "",
		fs.WithDir("SIP_20240606_NRAA_2024_001",
			fs.WithDir("content",
				fs.WithDir("f000001",
					fs.WithFile("d000001.txt", ""),
					fs.WithFile("d000002.pdf", ""),
					fs.WithFile("extra.txt", ""),
				),
			),
			fs.WithDir("header",
				fs.WithFile("metadata.xml", verifyManifestMetadata),
				fs.WithFile("metadata.xsd", ""),
				fs.WithFile("extra.xsd", ""),
			),
		),
	).Join("SIP_20240606_NRAA_2024_001")

	tests := []struct {
		name    string
		params  activities.VerifyManifestParams
		want    activities.VerifyManifestResult
		wantErr string
	}{
		{
			name: "Verifies an NRAA SIP manifest",
			params: activities.VerifyManifestParams{
				SIP: testSIP(t, fs.NewDir(t, "",
					fs.WithDir("SIP_20240606_NRAA_2024_001",
						fs.WithDir("content",
							fs.WithDir("f000001",
								fs.WithFile("d000001.txt", ""),
								fs.WithFile("d000002.pdf", ""),
							),
						),
						fs.WithDir("header",
							fs.WithFile("metadata.xml", verifyManifestMetadata),
							fs.WithFile("metadata.xsd", ""),
						),
					),
				).Join("SIP_20240606_NRAA_2024_001")),
			},
		},
		{
			name: "Returns a list of missing files",
			params: activities.VerifyManifestParams{
				SIP: testSIP(t, missingFilesSIP),
			},
			want: activities.VerifyManifestResult{
				MissingFiles: []string{
					fmt.Sprintf(
						"Missing file: %s/content/f000001/d000001.txt",
						filepath.Base(missingFilesSIP),
					),
					fmt.Sprintf(
						"Missing file: %s/header/metadata.xsd",
						filepath.Base(missingFilesSIP),
					),
				},
			},
		},
		{
			name: "Returns a list of extra files",
			params: activities.VerifyManifestParams{
				SIP: testSIP(t, extraFilesSIP),
			},
			want: activities.VerifyManifestResult{
				UnexpectedFiles: []string{
					fmt.Sprintf(
						"Unexpected file: %s/content/f000001/extra.txt",
						filepath.Base(extraFilesSIP),
					),
					fmt.Sprintf(
						"Unexpected file: %s/header/extra.xsd",
						filepath.Base(extraFilesSIP),
					),
				},
			},
		},
		{
			name: "Returns a list of mismatched checksums",
			params: activities.VerifyManifestParams{
				SIP: testSIP(t, fs.NewDir(t, "",
					fs.WithDir("SIP_20240606_NRAA_2024_001",
						fs.WithDir("content",
							fs.WithDir("f000001",
								fs.WithFile("d000001.txt", "wrong checksum"),
								fs.WithFile("d000002.pdf", ""),
							),
						),
						fs.WithDir("header",
							fs.WithFile("metadata.xml", verifyManifestMetadata),
							fs.WithFile("metadata.xsd", ""),
						),
					),
				).Join("SIP_20240606_NRAA_2024_001")),
			},
			want: activities.VerifyManifestResult{
				ChecksumFailures: []string{
					`Checksum mismatch for "content/f000001/d000001.txt" (expected: "d41d8cd98f00b204e9800998ecf8427e", got: "2714364e3a0ac68e8bf9b898b31ff303")`,
				},
			},
		},
		{
			name: "Returns an unsupported schema version error",
			params: activities.VerifyManifestParams{
				SIP: testSIP(t, fs.NewDir(t, "",
					fs.WithDir("SIP_20240606_NRAA_2024_001",
						fs.WithDir("content"),
						fs.WithDir("header",
							fs.WithFile("metadata.xml", unsupportedSchemaManifest),
						),
					),
				).Join("SIP_20240606_NRAA_2024_001")),
			},
			want: activities.VerifyManifestResult{
				ManifestFailures: []string{"Unsupported schema version: 2.0"},
			},
		},
		{
			name: "Errors when manifest is missing",
			params: activities.VerifyManifestParams{
				SIP: testSIP(t, fs.NewDir(t, "",
					fs.WithDir("SIP_20240606_NRAA_2024_001",
						fs.WithDir("content"),
						fs.WithDir("header"),
					),
				).Join("SIP_20240606_NRAA_2024_001")),
			},
			wantErr: "verify manifest: parse manifest: open",
		},
		{
			name: "Errors when checksum algorithm is unsupported",
			params: activities.VerifyManifestParams{
				SIP: testSIP(t, fs.NewDir(t, "",
					fs.WithDir("SIP_20240606_NRAA_2024_001",
						fs.WithDir("content",
							fs.WithDir("f000001",
								fs.WithFile("d000001.txt", ""),
							),
						),
						fs.WithDir("header",
							fs.WithFile("metadata.xml", unsupportedHashManifest),
						),
					),
				).Join("SIP_20240606_NRAA_2024_001")),
			},
			wantErr: `verify checksums: hash algorithm "SHA-999" is not supported`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewVerifyManifest().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.VerifyManifestName},
			)

			future, err := env.ExecuteActivity(activities.VerifyManifestName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			var res activities.VerifyManifestResult
			assert.NilError(t, future.Get(&res))
			assert.DeepEqual(t, res, tt.want)
		})
	}
}
