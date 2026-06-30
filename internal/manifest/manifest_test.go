package manifest_test

import (
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/manifest"
)

var SIPManifest = `
<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://nraa.gov.om/sip/v1" schemaVersion="1.0">
	<packageType>SIP</packageType>
	<toc>
		<folder>
			<name>header</name>
			<originalName>header</originalName>
			<digitalObject>
				<name>metadata.xsd</name>
				<originalName>metadata.xsd</originalName>
				<checksumAlgorithm>MD5</checksumAlgorithm>
				<checksum>aaa111</checksum>
			</digitalObject>
		</folder>
		<folder>
			<name>content</name>
			<originalName>content</originalName>
			<folder>
				<name>f000002</name>
				<originalName>f000002</originalName>
				<digitalObject id="d000001">
					<name>d000001.xls</name>
					<originalName>d000001</originalName>
					<checksumAlgorithm>MD5</checksumAlgorithm>
					<checksum>bbb222</checksum>
				</digitalObject>
				<digitalObject id="d000002">
					<name>d000002.png</name>
					<originalName>d000002</originalName>
					<checksumAlgorithm>MD5</checksumAlgorithm>
					<checksum>ccc333</checksum>
				</digitalObject>
			</folder>
		</folder>
	</toc>
</package>
`

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reader  io.Reader
		want    *manifest.Manifest
		wantErr string
	}{
		{
			name:   "Returns a SIP manifest",
			reader: strings.NewReader(SIPManifest),
			want: &manifest.Manifest{
				SchemaVersion: "1.0",
				Files: map[string]*manifest.File{
					"content/f000002/d000001.xls": {
						ID: "d000001",
						Checksum: manifest.Checksum{
							Algorithm: "MD5",
							Hash:      "bbb222",
						},
					},
					"content/f000002/d000002.png": {
						ID: "d000002",
						Checksum: manifest.Checksum{
							Algorithm: "MD5",
							Hash:      "ccc333",
						},
					},
					"header/metadata.xsd": {
						Checksum: manifest.Checksum{
							Algorithm: "MD5",
							Hash:      "aaa111",
						},
					},
				},
			},
		},
		{
			name:   "Returns an empty list from an empty manifest",
			reader: strings.NewReader(""),
			want:   &manifest.Manifest{Files: map[string]*manifest.File{}},
		},
		{
			name:    "Errors on a missing closing tag",
			reader:  strings.NewReader(`<digitalObject id="d000001"><name>d000001.xls</name>`),
			wantErr: "parse: XML syntax error on line 1: unexpected EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := manifest.Parse(tt.reader)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}
