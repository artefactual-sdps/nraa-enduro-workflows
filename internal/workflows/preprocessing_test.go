package workflows_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/artefactual-sdps/temporal-activities/archiveextract"
	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"github.com/artefactual-sdps/temporal-activities/ffvalidate"
	"github.com/artefactual-sdps/temporal-activities/removefiles"
	"github.com/artefactual-sdps/temporal-activities/xmlvalidate"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.artefactual.dev/tools/fsutil"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	temporalsdk_worker "go.temporal.io/sdk/worker"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/config"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/pips"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/premis"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/workflows"
)

const (
	sipName = "SIP_20240606_NRAA_2024_001.zip"
	relPath = "incoming/SIP_20240606_NRAA_2024_001.zip"
)

var testTime = time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)

type PreprocessingTestSuite struct {
	suite.Suite
	temporalsdk_testsuite.WorkflowTestSuite

	env          *temporalsdk_testsuite.TestWorkflowEnvironment
	workflow     *workflows.PreprocessingWorkflow
	testDir      string
	sipPath      string
	ignoredNames []string
}

func (s *PreprocessingTestSuite) SetupTest(cfg config.Configuration) {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.SetStartTime(testTime)
	s.env.SetWorkerOptions(temporalsdk_worker.Options{EnableSessionWorker: true})

	s.testDir = s.T().TempDir()
	cfg.Preprocessing.SharedPath = s.testDir
	s.ignoredNames = cfg.Preprocessing.RemoveFiles.RemoveNames
	s.sipPath = filepath.Join(s.testDir, relPath)
	if err := os.MkdirAll(filepath.Dir(s.sipPath), os.FileMode(0o700)); err != nil {
		s.T().Fatalf("create SIP parent directory: %v", err)
	}

	s.registerActivities(cfg)
	s.workflow = workflows.NewPreprocessingWorkflow(cfg.Preprocessing)
}

func (s *PreprocessingTestSuite) registerActivities(cfg config.Configuration) {
	s.env.RegisterActivityWithOptions(
		activities.NewVerifySIPSize().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.VerifySIPSizeName},
	)
	s.env.RegisterActivityWithOptions(
		archiveextract.New(archiveextract.Config{}).Execute,
		temporalsdk_activity.RegisterOptions{Name: archiveextract.Name},
	)
	s.env.RegisterActivityWithOptions(
		removefiles.New().Execute,
		temporalsdk_activity.RegisterOptions{Name: removefiles.Name},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewIdentifySIP().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.IdentifySIPName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewValidateStructure().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateStructureName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewValidateSIPName().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateSIPNameName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewValidateMetadataEncoding().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateMetadataEncodingName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewVerifyManifest().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.VerifyManifestName},
	)
	s.env.RegisterActivityWithOptions(
		ffvalidate.New(cfg.Preprocessing.FileFormat).Execute,
		temporalsdk_activity.RegisterOptions{Name: ffvalidate.Name},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewValidateFiles(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateFilesName},
	)
	s.env.RegisterActivityWithOptions(
		xmlvalidate.New(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: xmlvalidate.Name},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewAddPREMISObjects(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISObjectsName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewAddPREMISEvent().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISEventName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewAddPREMISValidationEvent(nil, nil, nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISValidationEventName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewAddPREMISAgent().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISAgentName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewTransformSIP().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.TransformSIPName},
	)
	s.env.RegisterActivityWithOptions(
		activities.NewWriteIdentifierFile().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.WriteIdentifierFileName},
	)
	s.env.RegisterActivityWithOptions(
		bagcreate.New(cfg.Preprocessing.BagCreate).Execute,
		temporalsdk_activity.RegisterOptions{Name: bagcreate.Name},
	)
}

func (s *PreprocessingTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func TestPreprocessingWorkflow(t *testing.T) {
	suite.Run(t, new(PreprocessingTestSuite))
}

func (s *PreprocessingTestSuite) expectedSIP(path string) sip.SIP {
	if err := os.MkdirAll(path, os.FileMode(0o700)); err != nil {
		s.T().Fatalf("create SIP directory: %v", err)
	}

	res, err := sip.New(path)
	s.NoError(err)

	return res
}

func (s *PreprocessingTestSuite) preValidationActivities(expectedSIP sip.SIP, extractPath string) {
	ctx := mock.Anything

	s.env.OnActivity(
		activities.VerifySIPSizeName,
		ctx,
		&activities.VerifySIPSizeParams{Path: s.sipPath},
	).Return(&activities.VerifySIPSizeResult{}, nil)
	s.env.OnActivity(
		archiveextract.Name,
		ctx,
		&archiveextract.Params{SourcePath: s.sipPath},
	).Return(&archiveextract.Result{ExtractPath: extractPath}, nil)
	s.env.OnActivity(
		removefiles.Name,
		ctx,
		&removefiles.Params{
			Path:        extractPath,
			RemoveNames: s.ignoredNames,
		},
	).Return(&removefiles.Result{Count: 2}, nil)
	s.env.OnActivity(
		activities.IdentifySIPName,
		ctx,
		&activities.IdentifySIPParams{Path: extractPath},
	).Return(&activities.IdentifySIPResult{SIP: expectedSIP}, nil)
}

func (s *PreprocessingTestSuite) validationSuccessActivities(expectedSIP sip.SIP) {
	ctx := mock.Anything

	s.env.OnActivity(
		activities.ValidateStructureName,
		ctx,
		&activities.ValidateStructureParams{SIP: expectedSIP},
	).Return(&activities.ValidateStructureResult{}, nil)
	s.env.OnActivity(
		activities.ValidateSIPNameName,
		ctx,
		&activities.ValidateSIPNameParams{SIP: expectedSIP},
	).Return(&activities.ValidateSIPNameResult{}, nil)
	s.env.OnActivity(
		activities.ValidateMetadataEncodingName,
		ctx,
		&activities.ValidateMetadataEncodingParams{SIP: expectedSIP},
	).Return(&activities.ValidateMetadataEncodingResult{}, nil)
	s.env.OnActivity(
		activities.VerifyManifestName,
		ctx,
		&activities.VerifyManifestParams{SIP: expectedSIP},
	).Return(&activities.VerifyManifestResult{}, nil)
	s.env.OnActivity(
		ffvalidate.Name,
		ctx,
		&ffvalidate.Params{Path: expectedSIP.ContentPath},
	).Return(&ffvalidate.Result{}, nil)
	s.env.OnActivity(
		activities.ValidateFilesName,
		ctx,
		&activities.ValidateFilesParams{SIP: expectedSIP},
	).Return(&activities.ValidateFilesResult{}, nil)
	s.env.OnActivity(
		xmlvalidate.Name,
		ctx,
		&xmlvalidate.Params{
			XMLPath: expectedSIP.MetadataPath,
			XSDPath: expectedSIP.XSDPath,
		},
	).Return(&xmlvalidate.Result{}, nil)
}

func (s *PreprocessingTestSuite) postValidationActivities(expectedSIP sip.SIP) {
	ctx := mock.Anything
	premisFilePath := filepath.Join(expectedSIP.Path, "metadata", "premis.xml")
	expectedPIP := pips.NewFromSIP(expectedSIP)

	s.env.OnActivity(
		activities.AddPREMISObjectsName,
		ctx,
		&activities.AddPREMISObjectsParams{
			SIP:            expectedSIP,
			PREMISFilePath: premisFilePath,
		},
	).Return(&activities.AddPREMISObjectsResult{}, nil)
	s.env.OnActivity(
		activities.AddPREMISEventName,
		ctx,
		&activities.AddPREMISEventParams{
			PREMISFilePath: premisFilePath,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP structure\"",
			OutcomeDetail:  "SIP structure identified. SIP structure matches validation criteria.",
			Failures:       nil,
		},
	).Return(&activities.AddPREMISEventResult{}, nil)
	s.env.OnActivity(
		activities.AddPREMISEventName,
		ctx,
		&activities.AddPREMISEventParams{
			PREMISFilePath: premisFilePath,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP name\"",
			OutcomeDetail:  fmt.Sprintf("SIP name %q matches validation criteria.", expectedSIP.Name()),
			Failures:       nil,
		},
	).Return(&activities.AddPREMISEventResult{}, nil)
	s.env.OnActivity(
		activities.AddPREMISEventName,
		ctx,
		&activities.AddPREMISEventParams{
			PREMISFilePath: premisFilePath,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Check for disallowed file formats\"",
			OutcomeDetail:  "Format allowed",
			Failures:       nil,
		},
	).Return(&activities.AddPREMISEventResult{}, nil)
	s.env.OnActivity(
		activities.AddPREMISValidationEventName,
		ctx,
		&activities.AddPREMISValidationEventParams{
			SIP:            expectedSIP,
			PREMISFilePath: premisFilePath,
			Summary: premis.EventSummary{
				Type:          "validation",
				Detail:        "name=\"Validate SIP file formats\"",
				Outcome:       "valid",
				OutcomeDetail: "File format complies with specification",
			},
		},
	).Return(&activities.AddPREMISValidationEventResult{}, nil)
	s.env.OnActivity(
		activities.AddPREMISEventName,
		ctx,
		&activities.AddPREMISEventParams{
			PREMISFilePath: premisFilePath,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP metadata\"",
			OutcomeDetail:  "Metadata validation successful",
			Failures:       nil,
		},
	).Return(&activities.AddPREMISEventResult{}, nil)
	s.env.OnActivity(
		activities.AddPREMISAgentName,
		ctx,
		&activities.AddPREMISAgentParams{
			PREMISFilePath: premisFilePath,
			Agent:          premis.AgentDefault(),
		},
	).Return(&activities.AddPREMISAgentResult{}, nil)
	s.env.OnActivity(
		activities.TransformSIPName,
		ctx,
		&activities.TransformSIPParams{SIP: expectedSIP},
	).Return(&activities.TransformSIPResult{PIP: expectedPIP}, nil)
	s.env.OnActivity(
		activities.WriteIdentifierFileName,
		ctx,
		&activities.WriteIdentifierFileParams{PIP: expectedPIP},
	).Return(&activities.WriteIdentifierFileResult{Path: filepath.Join(expectedSIP.Path, "metadata", "identifiers.json")}, nil)
	s.env.OnActivity(
		bagcreate.Name,
		ctx,
		&bagcreate.Params{SourcePath: expectedSIP.Path},
	).Return(&bagcreate.Result{BagPath: expectedSIP.Path}, nil)
}

func (s *PreprocessingTestSuite) TestSuccess() {
	s.SetupTest(config.Configuration{
		Preprocessing: config.PreprocessingConfig{
			RemoveFiles: config.RemoveFilesConfig{
				RemoveNames: []string{"thumbs.db", ".DS_Store"},
			},
		},
	})

	extractPath := filepath.Join(filepath.Dir(s.sipPath), fsutil.BaseNoExt(sipName))
	expectedSIP := s.expectedSIP(extractPath)
	s.preValidationActivities(expectedSIP, extractPath)
	s.validationSuccessActivities(expectedSIP)
	s.postValidationActivities(expectedSIP)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPName:      sipName,
		},
	)
	s.True(s.env.IsWorkflowCompleted())

	updatedRelPath, err := filepath.Rel(s.testDir, extractPath)
	s.NoError(err)

	var result childwf.PreprocessingResult
	err = s.env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeSuccess,
			RelativePath: updatedRelPath,
			Tasks: []*childwf.Task{
				successTask("Verify SIP size", "SIP size is within the 8 GB limit"),
				successTask("Extract SIP", "SIP extracted"),
				successTask("Remove unneeded files", "Unneeded files removed: 2"),
				successTask("Identify SIP structure", "SIP structure identified"),
				successTask("Validate SIP structure", "SIP structure matches validation criteria"),
				successTask(
					"Validate SIP name",
					"SIP name matches expected naming convention for the identified structure type",
				),
				successTask("Validate SIP metadata encoding", "SIP metadata files use UTF-8 encoding"),
				successTask("Verify SIP manifest", "SIP contents match manifest"),
				successTask("Verify SIP checksums", "SIP checksums match file contents"),
				successTask("Check for disallowed file formats", "No disallowed file formats found"),
				successTask("Validate SIP file formats", "No invalid files found"),
				successTask(
					"Validate SIP metadata",
					"Metadata validation successful on the following file(s):\n\n- metadata.xml",
				),
				successTask(
					"Create premis.xml",
					"Created a premis.xml file and stored it in the metadata directory",
				),
				successTask("Restructure SIP", "SIP has been restructured for preservation processing"),
				successTask(
					"Create identifier.json",
					"Created an identifier.json file and stored it in the metadata directory",
				),
				successTask("Bag SIP", "SIP has been bagged"),
			},
		},
		&result,
	)
}

func (s *PreprocessingTestSuite) TestVerifySIPSizeContentErrorStopsWorkflow() {
	s.SetupTest(config.Configuration{
		Preprocessing: config.PreprocessingConfig{
			RemoveFiles: config.RemoveFilesConfig{
				RemoveNames: []string{".DS_Store"},
			},
		},
	})

	s.env.OnActivity(
		activities.VerifySIPSizeName,
		mock.Anything,
		&activities.VerifySIPSizeParams{Path: s.sipPath},
	).Return(&activities.VerifySIPSizeResult{
		Failures: []string{"SIP size is 8 GB, exceeding the limit of 8 GB by 1 B"},
	}, nil)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPName:      sipName,
		},
	)
	s.True(s.env.IsWorkflowCompleted())

	var result childwf.PreprocessingResult
	err := s.env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeContentError,
			RelativePath: relPath,
			Tasks: []*childwf.Task{
				validationTask(
					"Verify SIP size",
					"SIP size verification has failed.",
					"- SIP size is 8 GB, exceeding the limit of 8 GB by 1 B",
					"Please ensure the zipped package does not exceed 8 GB.",
				),
			},
		},
		&result,
	)
}

func (s *PreprocessingTestSuite) TestValidationErrorCollection() {
	s.SetupTest(config.Configuration{})

	extractPath := filepath.Join(filepath.Dir(s.sipPath), fsutil.BaseNoExt(sipName))
	expectedSIP := s.expectedSIP(extractPath)
	s.preValidationActivities(expectedSIP, extractPath)

	ctx := mock.Anything
	s.env.OnActivity(
		activities.ValidateStructureName,
		ctx,
		&activities.ValidateStructureParams{SIP: expectedSIP},
	).Return(&activities.ValidateStructureResult{Failures: []string{"metadata.xsd is missing"}}, nil)
	s.env.OnActivity(
		activities.ValidateSIPNameName,
		ctx,
		&activities.ValidateSIPNameParams{SIP: expectedSIP},
	).Return(&activities.ValidateSIPNameResult{Failures: []string{"SIP name \"bad\" violates naming standard"}}, nil)
	s.env.OnActivity(
		activities.ValidateMetadataEncodingName,
		ctx,
		&activities.ValidateMetadataEncodingParams{SIP: expectedSIP},
	).Return(&activities.ValidateMetadataEncodingResult{
		Failures: []string{fmt.Sprintf("%q must use UTF-8 encoding", filepath.Join("header", "metadata.xml"))},
	}, nil)
	s.env.OnActivity(
		activities.VerifyManifestName,
		ctx,
		&activities.VerifyManifestParams{SIP: expectedSIP},
	).Return(&activities.VerifyManifestResult{
		ManifestFailures: []string{"Unsupported schema version: 5.1"},
		MissingFiles:     []string{"Missing file: SIP_20240606_NRAA_2024_001/content/f000001/d000001.txt"},
		ChecksumFailures: []string{"Checksum mismatch for \"content/f000001/d000001.txt\""},
	}, nil)
	s.env.OnActivity(
		ffvalidate.Name,
		ctx,
		&ffvalidate.Params{Path: expectedSIP.ContentPath},
	).Return(&ffvalidate.Result{Failures: []string{"file format \"fmt/11\" disallowed: \"f000001/d000001.png\""}}, nil)
	s.env.OnActivity(
		activities.ValidateFilesName,
		ctx,
		&activities.ValidateFilesParams{SIP: expectedSIP},
	).Return(&activities.ValidateFilesResult{Failures: []string{"One or more PDF/A files are invalid"}}, nil)
	s.env.OnActivity(
		xmlvalidate.Name,
		ctx,
		&xmlvalidate.Params{
			XMLPath: expectedSIP.MetadataPath,
			XSDPath: expectedSIP.XSDPath,
		},
	).Return(&xmlvalidate.Result{Failures: []string{"metadata.xml does not match expected metadata requirements"}}, nil)

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPName:      sipName,
		},
	)
	s.True(s.env.IsWorkflowCompleted())

	var result childwf.PreprocessingResult
	err := s.env.GetWorkflowResult(&result)
	s.NoError(err)
	updatedRelPath, err := filepath.Rel(s.testDir, extractPath)
	s.NoError(err)
	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeContentError,
			RelativePath: updatedRelPath,
			Tasks: []*childwf.Task{
				successTask("Verify SIP size", "SIP size is within the 8 GB limit"),
				successTask("Extract SIP", "SIP extracted"),
				successTask("Remove unneeded files", "Unneeded files removed: 2"),
				successTask("Identify SIP structure", "SIP structure identified"),
				validationTask(
					"Validate SIP structure",
					"SIP structure validation has failed.",
					"- metadata.xsd is missing",
					"Please review the SIP and ensure that its structure matches the NRAA specifications.",
				),
				validationTask(
					"Validate SIP name",
					"SIP name validation has failed.",
					"The name used for the package does not match the expected convention.",
					"- SIP name \"bad\" violates naming standard",
					"Please review the naming conventions specified for this SIP.",
				),
				validationTask(
					"Validate SIP metadata encoding",
					"SIP metadata encoding validation has failed.",
					"- \"header/metadata.xml\" must use UTF-8 encoding",
					"Please ensure metadata.xml and metadata.xsd use UTF-8 encoding.",
				),
				validationTask(
					"Verify SIP manifest",
					"\"metadata.xml\" manifest could not be verified against the contents of the SIP.",
					"- Unsupported schema version: 5.1\n"+
						"- Missing file: SIP_20240606_NRAA_2024_001/content/f000001/d000001.txt",
					"Please review the SIP and ensure that its contents match those listed in the metadata manifest.",
				),
				validationTask(
					"Verify SIP checksums",
					"SIP checksums do not match file contents.",
					"- Checksum mismatch for \"content/f000001/d000001.txt\"",
					"Please review the SIP and ensure that the metadata checksums match those of the files.",
				),
				validationTask(
					"Check for disallowed file formats",
					"file format check has failed.",
					"One or more file formats are not allowed:",
					"- file format \"fmt/11\" disallowed: \"f000001/d000001.png\"",
					"Please review the SIP and remove or replace all disallowed file formats.",
				),
				validationTask(
					"Validate SIP file formats",
					"file format validation has failed.",
					"- One or more PDF/A files are invalid",
					"Please ensure all files are well-formed.",
				),
				validationTask(
					"Validate SIP metadata",
					"metadata validation has failed.",
					"- metadata.xml does not match expected metadata requirements",
					"Please ensure all metadata files are present and well-formed.",
				),
			},
		},
		&result,
	)
}

func (s *PreprocessingTestSuite) TestExtractionSystemError() {
	s.SetupTest(config.Configuration{})

	s.env.OnActivity(
		activities.VerifySIPSizeName,
		mock.Anything,
		&activities.VerifySIPSizeParams{Path: s.sipPath},
	).Return(&activities.VerifySIPSizeResult{}, nil)
	s.env.OnActivity(
		archiveextract.Name,
		mock.Anything,
		&archiveextract.Params{SourcePath: s.sipPath},
	).Return(nil, errors.New("not a valid archive"))

	s.env.ExecuteWorkflow(
		s.workflow.Execute,
		&childwf.PreprocessingParams{
			RelativePath: relPath,
			SIPName:      sipName,
		},
	)
	s.True(s.env.IsWorkflowCompleted())

	var result childwf.PreprocessingResult
	err := s.env.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(
		&childwf.PreprocessingResult{
			Outcome:      childwf.OutcomeSystemError,
			RelativePath: relPath,
			Tasks: []*childwf.Task{
				successTask("Verify SIP size", "SIP size is within the 8 GB limit"),
				systemTask(
					"Extract SIP",
					"SIP extraction has failed.",
					"\"SIP_20240606_NRAA_2024_001.zip\" could not be successfully extracted. Please try again, or ask a system administrator to investigate.",
				),
			},
		},
		&result,
	)
}

func successTask(name, message string) *childwf.Task {
	return expectedTask(name, message, childwf.TaskOutcomeSuccess)
}

func validationTask(name string, messageParts ...string) *childwf.Task {
	return expectedTask(
		name,
		childWorkflowErrorMessage("Content error", messageParts...),
		childwf.TaskOutcomeValidationFailure,
	)
}

func systemTask(name string, messageParts ...string) *childwf.Task {
	return expectedTask(
		name,
		childWorkflowErrorMessage("System error", messageParts...),
		childwf.TaskOutcomeSystemFailure,
	)
}

func expectedTask(name, message string, outcome childwf.TaskOutcome) *childwf.Task {
	return &childwf.Task{
		Name:        name,
		Message:     message,
		Outcome:     outcome,
		StartedAt:   testTime,
		CompletedAt: testTime,
	}
}

func childWorkflowErrorMessage(prefix string, messageParts ...string) string {
	return prefix + ": " + strings.Join(messageParts, "\n\n")
}
