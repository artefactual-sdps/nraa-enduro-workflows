package activities_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/fformat"
	fake_fformat "github.com/artefactual-sdps/nraa-enduro-workflows/internal/fformat/fake"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/fvalidate"
	fake_fvalidate "github.com/artefactual-sdps/nraa-enduro-workflows/internal/fvalidate/fake"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

func TestValidateFiles(t *testing.T) {
	t.Parallel()

	s, err := sip.New(fs.NewDir(t, "",
		fs.WithDir("SIP_20240606_NRAA_2024_001",
			fs.WithDir("content",
				fs.WithDir("f000001",
					fs.WithFile("d000001.pdf", ""),
					fs.WithFile("d000002.txt", ""),
				),
			),
			fs.WithDir("header",
				fs.WithFile("metadata.xml", ""),
				fs.WithFile("metadata.xsd", ""),
			),
		),
	).Join("SIP_20240606_NRAA_2024_001"))
	assert.NilError(t, err)

	defaultIdentifierMock := func(m *fake_fformat.MockIdentifierMockRecorder) {
		m.Identify(
			filepath.Join(s.ContentPath, "f000001", "d000001.pdf"),
		).Return(
			&fformat.FileFormat{
				Namespace: "PRONOM",
				ID:        "fmt/354",
			},
			nil,
		)
		m.Identify(
			filepath.Join(s.ContentPath, "f000001", "d000002.txt"),
		).Return(
			&fformat.FileFormat{
				Namespace: "PRONOM",
				ID:        "fmt/101",
			},
			nil,
		)
	}

	tests := []struct {
		name      string
		params    activities.ValidateFilesParams
		expectId  func(*fake_fformat.MockIdentifierMockRecorder)
		expectVld func(*fake_fvalidate.MockValidatorMockRecorder)
		want      activities.ValidateFilesResult
		wantErr   string
	}{
		{
			name:     "Validates a PDF/A file",
			params:   activities.ValidateFilesParams{SIP: s},
			expectId: defaultIdentifierMock,
			expectVld: func(m *fake_fvalidate.MockValidatorMockRecorder) {
				m.Scope().Return(fvalidate.TargetTypeDir)
				m.FormatIDs().Return([]string{"fmt/354"})
				m.Validate(s.ContentPath).Return("", nil)
			},
		},
		{
			name:     "Reports PDF validation errors",
			params:   activities.ValidateFilesParams{SIP: s},
			expectId: defaultIdentifierMock,
			expectVld: func(m *fake_fvalidate.MockValidatorMockRecorder) {
				m.Scope().Return(fvalidate.TargetTypeDir)
				m.FormatIDs().Return([]string{"fmt/354"})
				m.Validate(s.ContentPath).Return(
					"One or more PDF/A files are invalid",
					nil,
				)
			},
			want: activities.ValidateFilesResult{
				Failures: []string{"One or more PDF/A files are invalid"},
			},
		},
		{
			name:     "Returns file not found error",
			params:   activities.ValidateFilesParams{SIP: s},
			expectId: defaultIdentifierMock,
			expectVld: func(m *fake_fvalidate.MockValidatorMockRecorder) {
				m.Scope().Return(fvalidate.TargetTypeDir)
				m.FormatIDs().Return([]string{"fmt/354"})
				m.Validate(s.ContentPath).Return(
					"",
					errors.New("validate: file not found: /fake/path"),
				)
			},
			wantErr: "validate: file not found: /fake/path",
		},
		{
			name:     "Reports an application error",
			params:   activities.ValidateFilesParams{SIP: s},
			expectId: defaultIdentifierMock,
			expectVld: func(m *fake_fvalidate.MockValidatorMockRecorder) {
				m.Scope().Return(fvalidate.TargetTypeDir)
				m.FormatIDs().Return([]string{"fmt/354"})
				m.Validate(s.ContentPath).Return(
					"",
					fvalidate.NewSystemError(
						"veraPDF",
						1,
						errors.New("permission denied"),
						"PDF/A validation failed with an application error",
					),
				)
			},
			wantErr: "PDF/A validation failed with an application error",
		},
		{
			name:     "Error when validator scope is not supported",
			params:   activities.ValidateFilesParams{SIP: s},
			expectId: defaultIdentifierMock,
			expectVld: func(m *fake_fvalidate.MockValidatorMockRecorder) {
				m.Scope().Return(fvalidate.TargetTypeFile)
			},
			wantErr: "validateFiles: unsupported validator scope",
		},
		{
			name:   "Skip validation when format identification fails",
			params: activities.ValidateFilesParams{SIP: s},
			expectId: func(m *fake_fformat.MockIdentifierMockRecorder) {
				m.Identify(
					filepath.Join(s.ContentPath, "f000001", "d000001.pdf"),
				).Return(
					nil,
					fmt.Errorf(
						"multiple file formats matched: %s",
						filepath.Join(s.ContentPath, "f000001", "d000001.pdf"),
					),
				)
				m.Identify(
					filepath.Join(s.ContentPath, "f000001", "d000002.txt"),
				).Return(
					&fformat.FileFormat{
						Namespace: "PRONOM",
						ID:        "fmt/101",
					},
					nil,
				)
			},
			expectVld: func(m *fake_fvalidate.MockValidatorMockRecorder) {
				m.Scope().Return(fvalidate.TargetTypeDir)
				m.FormatIDs().Return([]string{"fmt/354"})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()

			ctrl := gomock.NewController(t)

			mockIdr := fake_fformat.NewMockIdentifier(ctrl)
			if tt.expectId != nil {
				tt.expectId(mockIdr.EXPECT())
			}

			mockVdr := fake_fvalidate.NewMockValidator(ctrl)
			if tt.expectVld != nil {
				tt.expectVld(mockVdr.EXPECT())
			}

			env.RegisterActivityWithOptions(
				activities.NewValidateFiles(mockIdr, mockVdr).Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ValidateFilesName},
			)

			enc, err := env.ExecuteActivity(activities.ValidateFilesName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			var result activities.ValidateFilesResult
			assert.NilError(t, enc.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
