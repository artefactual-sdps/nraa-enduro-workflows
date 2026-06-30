package sip_test

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

func TestNew(t *testing.T) {
	t.Parallel()

	sipName := "SIP_20240606_NRAA_2024_001"
	sipPath := filepath.Join(sipTempDir(t, sipName), sipName)

	tests := []struct {
		name    string
		path    string
		wantSIP sip.SIP
		wantErr string
	}{
		{
			name: "Creates a new NRAA SIP",
			path: sipPath,
			wantSIP: sip.SIP{
				Path:         sipPath,
				ContentPath:  filepath.Join(sipPath, "content"),
				HeaderPath:   filepath.Join(sipPath, "header"),
				ManifestPath: filepath.Join(sipPath, "header", "metadata.xml"),
				MetadataPath: filepath.Join(sipPath, "header", "metadata.xml"),
				XSDPath:      filepath.Join(sipPath, "header", "metadata.xsd"),
				TopLevelPaths: []string{
					filepath.Join(sipPath, "content"),
					filepath.Join(sipPath, "header"),
				},
			},
		},
		{
			name:    "Fails with a non existing path",
			wantErr: "SIP: New: stat : no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, err := sip.New(tt.path)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}

			assert.NilError(t, err)
			assert.DeepEqual(t, s, tt.wantSIP)
			assert.Equal(t, s.HasValidName(), true)
		})
	}
}

func TestName(t *testing.T) {
	t.Parallel()

	s := sip.SIP{
		Path: "/path/to/SIP_20240606_NRAA_2024_001",
	}
	assert.Equal(t, s.Name(), "SIP_20240606_NRAA_2024_001")
}

func TestHasValidName(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		want bool
	}{
		{name: "SIP_20240606_NRAA_2024_001", want: true},
		{name: "SIP_20240230_NRAA_2024_001", want: false},
		{name: "SIP_20240606_N_2024_001", want: false},
		{name: "SIP_20240606_NRAAAA_2024_001", want: false},
		{name: "SIP_20240606_NRAA_2024_01", want: false},
		{name: "sip_20240606_NRAA_2024_001", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := fs.NewDir(t, "", fs.WithDir(tt.name))
			s, err := sip.New(filepath.Join(dir.Path(), tt.name))
			assert.NilError(t, err)
			assert.Equal(t, s.HasValidName(), tt.want)
		})
	}
}

func sipTempDir(t *testing.T, sipName string) string {
	t.Helper()

	return fs.NewDir(t, "",
		fs.WithDir(sipName,
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
	).Path()
}
