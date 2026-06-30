package activities_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/activities"
)

func TestVerifySIPSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    func(t *testing.T) string
		want    *activities.VerifySIPSizeResult
		wantErr string
	}{
		{
			name: "Succeeds when SIP does not exceed 8 GB",
			path: func(t *testing.T) string {
				t.Helper()

				path := filepath.Join(t.TempDir(), "sip.zip")
				assert.NilError(t, os.WriteFile(path, []byte("zip"), os.FileMode(0o600)))

				return path
			},
			want: &activities.VerifySIPSizeResult{},
		},
		{
			name: "Returns a validation failure when SIP exceeds 8 GB",
			path: func(t *testing.T) string {
				t.Helper()

				path := filepath.Join(t.TempDir(), "sip.zip")
				f, err := os.Create(path)
				assert.NilError(t, err)
				assert.NilError(t, f.Truncate(8_000_000_001))
				assert.NilError(t, f.Close())

				return path
			},
			want: &activities.VerifySIPSizeResult{
				Failures: []string{
					"SIP size is 8 GB, exceeding the limit of 8 GB by 1 B",
				},
			},
		},
		{
			name: "Errors when path cannot be statted",
			path: func(t *testing.T) string {
				t.Helper()

				return filepath.Join(t.TempDir(), "missing.zip")
			},
			wantErr: "verify SIP size: stat:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res, err := activities.NewVerifySIPSize().Execute(
				context.Background(),
				&activities.VerifySIPSizeParams{Path: tt.path(t)},
			)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, res, tt.want)
		})
	}
}
