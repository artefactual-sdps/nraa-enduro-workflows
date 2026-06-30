package identifiers_test

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/identifiers"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/manifest"
)

func TestSortFunc(t *testing.T) {
	l := []identifiers.File{
		{Path: "dir/a.txt"},
		{Path: "dir/C.txt"},
		{Path: "b.txt"},
		{Path: "b.txt"},
		{Path: "e.txt"},
	}
	slices.SortFunc(l, identifiers.Compare)
	assert.DeepEqual(t, l, []identifiers.File{
		{Path: "b.txt"},
		{Path: "b.txt"},
		{Path: "dir/C.txt"}, // Uppercase letters sort before lowercase.
		{Path: "dir/a.txt"},
		{Path: "e.txt"},
	})
}

func TestFromManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest *manifest.Manifest
		want     []identifiers.File
		wantErr  string
	}{
		{
			name: "Returns an NRAA SIP identifier list",
			manifest: &manifest.Manifest{Files: map[string]*manifest.File{
				"content/f000001/d000001.txt": {
					ID: "d000001",
					Checksum: manifest.Checksum{
						Algorithm: "MD5",
						Hash:      "f7dc1f76a55cbdca0ae4a6dc8ae64644",
					},
				},
				"content/f000001/d000002.pdf": {
					ID: "d000002",
					Checksum: manifest.Checksum{
						Algorithm: "MD5",
						Hash:      "1428a269ff4e5b4894793b68646984b7",
					},
				},
				"header/metadata.xml": {
					ID: "metadata",
					Checksum: manifest.Checksum{
						Algorithm: "MD5",
						Hash:      "43c533d499c572fca699e77e06295ba3",
					},
				},
				"header/metadata.xsd": {
					ID: "metadata.xsd",
					Checksum: manifest.Checksum{
						Algorithm: "MD5",
						Hash:      "f8454632e1ebf97e0aa8d9527ce2641f",
					},
				},
			}},
			want: []identifiers.File{
				{
					Path: "content/f000001/d000001.txt",
					Identifiers: []identifiers.Identifier{
						{
							Value: "d000001",
							Type:  "local",
						},
					},
				},
				{
					Path: "content/f000001/d000002.pdf",
					Identifiers: []identifiers.Identifier{
						{
							Value: "d000002",
							Type:  "local",
						},
					},
				},
				{
					Path: "header/metadata.xml",
					Identifiers: []identifiers.Identifier{
						{
							Value: "metadata",
							Type:  "local",
						},
					},
				},
				{
					Path: "header/metadata.xsd",
					Identifiers: []identifiers.Identifier{
						{
							Value: "metadata.xsd",
							Type:  "local",
						},
					},
				},
			},
		},
		{
			name:     "Errors when manifest is empty",
			manifest: &manifest.Manifest{},
			wantErr:  "no files in manifest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := identifiers.FromManifest(tt.manifest)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}
