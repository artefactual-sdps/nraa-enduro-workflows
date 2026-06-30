package workflows

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/artefactual-sdps/temporal-activities/archiveextract"
	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"github.com/artefactual-sdps/temporal-activities/ffvalidate"
	"github.com/artefactual-sdps/temporal-activities/removefiles"
	"github.com/artefactual-sdps/temporal-activities/xmlvalidate"
	"go.artefactual.dev/tools/fsutil"
	"go.artefactual.dev/tools/temporal"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/config"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/premis"
	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

type PreprocessingWorkflow struct {
	cfg config.PreprocessingConfig
}

func NewPreprocessingWorkflow(cfg config.PreprocessingConfig) *PreprocessingWorkflow {
	return &PreprocessingWorkflow{cfg: cfg}
}

func (w *PreprocessingWorkflow) Execute(
	ctx temporalsdk_workflow.Context,
	params *childwf.PreprocessingParams,
) (*childwf.PreprocessingResult, error) {
	var e error
	result := &childwf.PreprocessingResult{}
	logger := temporalsdk_workflow.GetLogger(ctx)
	logger.Debug("PreprocessingWorkflow workflow running!", "params", params)

	defer func() {
		logger.Debug("PreprocessingWorkflow workflow finished!", "result", result, "error", e)
	}()

	if params == nil || params.RelativePath == "" {
		e = temporal.NewNonRetryableError(fmt.Errorf("error calling workflow with unexpected inputs"))
		return nil, e
	}
	result.RelativePath = params.RelativePath

	localPath := filepath.Join(w.cfg.SharedPath, filepath.Clean(params.RelativePath))

	// Verify SIP size before extraction.
	task := result.NewTask(temporalsdk_workflow.Now(ctx), "Verify SIP size")
	var verifySIPSize activities.VerifySIPSizeResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.VerifySIPSizeName,
		&activities.VerifySIPSizeParams{Path: localPath},
	).Get(ctx, &verifySIPSize)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP size verification has failed.",
			"An error occurred while checking the SIP size. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	if verifySIPSize.Failures != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP size verification has failed.",
			ul(verifySIPSize.Failures),
			"Please ensure the zipped package does not exceed 8 GB.",
		)
		return result, nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP size is within the 8 GB limit")

	// Extract SIP.
	localPath = w.extractSIP(ctx, result, localPath, params.SIPName)
	if result.Outcome == childwf.OutcomeSystemError {
		return result, nil
	}

	// Remove unneeded files.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Remove unneeded files")
	var removeFilesResult removefiles.Result
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		removefiles.Name,
		&removefiles.Params{
			Path:        localPath,
			RemoveNames: w.cfg.RemoveFiles.RemoveNames,
		},
	).Get(ctx, &removeFilesResult)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"unneeded file removal has failed.",
			"An error occurred while removing unneeded files. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "Unneeded files removed: %d", removeFilesResult.Count)

	// Identify SIP.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Identify SIP structure")
	var identifySIP activities.IdentifySIPResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.IdentifySIPName,
		&activities.IdentifySIPParams{Path: localPath},
	).Get(ctx, &identifySIP)
	if e != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP identification has failed.",
			"Enduro could not identify the package structure. Please ensure that your SIP matches the supported package structure.",
		)
		return result, nil
	}

	sip := identifySIP.SIP
	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP structure identified")

	// Validate structure.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate SIP structure")
	var validateStructure activities.ValidateStructureResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.ValidateStructureName,
		&activities.ValidateStructureParams{SIP: sip},
	).Get(ctx, &validateStructure)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP structure validation has failed.",
			"An error occurred during the structure validation process. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	if validateStructure.Failures != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP structure validation has failed.",
			ul(validateStructure.Failures),
			"Please review the SIP and ensure that its structure matches the NRAA specifications.",
		)
	} else {
		task.Succeed(temporalsdk_workflow.Now(ctx), "SIP structure matches validation criteria")
	}

	// Validate SIP name.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate SIP name")
	var validateSIPName activities.ValidateSIPNameResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.ValidateSIPNameName,
		&activities.ValidateSIPNameParams{SIP: sip},
	).Get(ctx, &validateSIPName)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP name validation has failed.",
			"An error occurred during the SIP name validation process. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	if validateSIPName.Failures != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP name validation has failed.",
			"The name used for the package does not match the expected convention.",
			ul(validateSIPName.Failures),
			"Please review the naming conventions specified for this SIP.",
		)
	} else {
		task.Succeed(
			temporalsdk_workflow.Now(ctx),
			"SIP name matches expected naming convention for the identified structure type",
		)
	}

	// Validate metadata encoding.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate SIP metadata encoding")
	var validateMetadataEncoding activities.ValidateMetadataEncodingResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.ValidateMetadataEncodingName,
		&activities.ValidateMetadataEncodingParams{SIP: sip},
	).Get(ctx, &validateMetadataEncoding)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP metadata encoding validation has failed.",
			"An error occurred during metadata encoding validation. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	if len(validateMetadataEncoding.Failures) > 0 {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP metadata encoding validation has failed.",
			ul(validateMetadataEncoding.Failures),
			"Please ensure metadata.xml and metadata.xsd use UTF-8 encoding.",
		)
	} else {
		task.Succeed(temporalsdk_workflow.Now(ctx), "SIP metadata files use UTF-8 encoding")
	}

	// Verify that package contents match the manifest.
	manifestTask := result.NewTask(temporalsdk_workflow.Now(ctx), "Verify SIP manifest")
	checksumTask := result.NewTask(temporalsdk_workflow.Now(ctx), "Verify SIP checksums")
	var verifyManifest activities.VerifyManifestResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.VerifyManifestName,
		&activities.VerifyManifestParams{SIP: sip},
	).Get(ctx, &verifyManifest)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			manifestTask,
			"SIP manifest verification has failed.",
			"An error occurred during the manifest verification process. Please try again, or ask a system administrator to investigate.",
		)
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			checksumTask,
			"SIP checksum verification has failed.",
			"An error occurred during the checksum verification process. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}

	if len(verifyManifest.ManifestFailures) > 0 || len(verifyManifest.MissingFiles) > 0 ||
		len(verifyManifest.UnexpectedFiles) > 0 {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			manifestTask,
			fmt.Sprintf(
				"%q manifest could not be verified against the contents of the SIP.",
				filepath.Base(sip.ManifestPath),
			),
			ul(
				slices.Concat(
					verifyManifest.ManifestFailures,
					verifyManifest.MissingFiles,
					verifyManifest.UnexpectedFiles,
				),
			),
			"Please review the SIP and ensure that its contents match those listed in the metadata manifest.",
		)
	} else {
		manifestTask.Succeed(temporalsdk_workflow.Now(ctx), "SIP contents match manifest")
	}

	if len(verifyManifest.ChecksumFailures) > 0 {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			checksumTask,
			"SIP checksums do not match file contents.",
			ul(verifyManifest.ChecksumFailures),
			"Please review the SIP and ensure that the metadata checksums match those of the files.",
		)
	} else {
		checksumTask.Succeed(temporalsdk_workflow.Now(ctx), "SIP checksums match file contents")
	}

	// Check for disallowed file formats.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Check for disallowed file formats")
	var ffvalidateResult ffvalidate.Result
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		ffvalidate.Name,
		&ffvalidate.Params{Path: sip.ContentPath},
	).Get(ctx, &ffvalidateResult)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"file format check has failed.",
			"An error occurred when checking for disallowed file formats. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}

	if ffvalidateResult.Failures != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"file format check has failed.",
			"One or more file formats are not allowed:",
			ul(ffvalidateResult.Failures),
			"Please review the SIP and remove or replace all disallowed file formats.",
		)
	} else {
		task.Succeed(temporalsdk_workflow.Now(ctx), "No disallowed file formats found")
	}

	// Validate SIP file formats against the format specifications.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate SIP file formats")
	var validateFilesResult activities.ValidateFilesResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.ValidateFilesName,
		&activities.ValidateFilesParams{SIP: sip},
	).Get(ctx, &validateFilesResult)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"file format validation has failed.",
			"An error occurred during the file format validation process. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}

	if validateFilesResult.Failures != nil {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"file format validation has failed.",
			// TODO: Add tool name and version info.
			ul(validateFilesResult.Failures),
			"Please ensure all files are well-formed.",
		)
	} else {
		task.Succeed(temporalsdk_workflow.Now(ctx), "No invalid files found")
	}

	// Validate metadata.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Validate SIP metadata")
	var validateMetadata xmlvalidate.Result
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		xmlvalidate.Name,
		&xmlvalidate.Params{
			XMLPath: sip.MetadataPath,
			XSDPath: sip.XSDPath,
		},
	).Get(ctx, &validateMetadata)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"metadata validation has failed.",
			fmt.Sprintf(
				"An error has occurred while attempting to validate the %q file. Please try again, or ask a system administrator to investigate.",
				filepath.Base(sip.MetadataPath),
			),
		)
		return result, nil
	}

	if validateMetadata.Failures != nil {
		for idx, f := range validateMetadata.Failures {
			validateMetadata.Failures[idx] = strings.ReplaceAll(f, sip.Path+string(filepath.Separator), "")
		}
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"metadata validation has failed.",
			ul(validateMetadata.Failures),
			"Please ensure all metadata files are present and well-formed.",
		)
	} else {
		task.Succeed(
			temporalsdk_workflow.Now(ctx),
			"Metadata validation successful on the following file(s):\n\n%s",
			ul([]string{filepath.Base(sip.MetadataPath)}),
		)
	}

	// Stop here if the SIP content isn't valid.
	if result.Outcome == childwf.OutcomeContentError {
		return result, nil
	}

	// Write PREMIS XML.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Create premis.xml")
	if e = writePREMISFile(ctx, sip); e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"premis.xml creation has failed",
			"An error has occurred while attempting to create the premis.xml file and store it in the metadata directory. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "Created a premis.xml file and stored it in the metadata directory")

	// Re-structure SIP.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Restructure SIP")
	var transformSIP activities.TransformSIPResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.TransformSIPName,
		&activities.TransformSIPParams{SIP: sip},
	).Get(ctx, &transformSIP)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"restructuring has failed",
			"An error has occurred while attempting to restructure the SIP for preservation processing. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP has been restructured for preservation processing")

	// Write the identifiers.json file.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Create identifier.json")
	var writeIDFile activities.WriteIdentifierFileResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.WriteIdentifierFileName,
		&activities.WriteIdentifierFileParams{PIP: transformSIP.PIP},
	).Get(ctx, &writeIDFile)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"identifier.json creation has failed.",
			"An error has occurred while attempting to create the identifier.json file and store it in the metadata directory. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	task.Succeed(
		temporalsdk_workflow.Now(ctx),
		"Created an identifier.json file and stored it in the metadata directory",
	)

	// Bag the SIP for Enduro processing.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Bag SIP")
	var createBag bagcreate.Result
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		bagcreate.Name,
		&bagcreate.Params{SourcePath: localPath},
	).Get(ctx, &createBag)
	if e != nil {
		logger.Error("System error", "message", e.Error())
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP bagging has failed.",
			"An error has occurred while attempting to bag the SIP. Please try again, or ask a system administrator to investigate.",
		)
		return result, nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP has been bagged")

	return result, nil
}

func (w *PreprocessingWorkflow) extractSIP(
	ctx temporalsdk_workflow.Context,
	result *childwf.PreprocessingResult,
	path string,
	sipName string,
) string {
	logger := temporalsdk_workflow.GetLogger(ctx)

	task := result.NewTask(temporalsdk_workflow.Now(ctx), "Extract SIP")
	var archiveExtract archiveextract.Result
	e := temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		archiveextract.Name,
		&archiveextract.Params{SourcePath: path},
	).Get(ctx, &archiveExtract)
	if e != nil {
		logger.Error("System error", "message", fmt.Errorf("extract SIP: %w", e))
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP extraction has failed.",
			fmt.Sprintf(
				`%q could not be successfully extracted. Please try again, or ask a system administrator to investigate.`,
				filepath.Base(path),
			),
		)
		return ""
	}

	// Verify that the extraction directory has the same name as the uploaded
	// archive minus the file extension (e.g. "example.zip" -> "example").
	if filepath.Base(archiveExtract.ExtractPath) != fsutil.BaseNoExt(sipName) {
		result.ValidationError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP extraction has failed.",
			fmt.Sprintf(
				"The extracted SIP is missing the top-level %q folder.",
				fsutil.BaseNoExt(sipName),
			),
			"Please ensure that the SIP is well-formed and try again.",
		)
		return archiveExtract.ExtractPath
	}

	result.RelativePath, e = filepath.Rel(w.cfg.SharedPath, archiveExtract.ExtractPath)
	if e != nil {
		logger.Error("System error", "message", fmt.Errorf("extract SIP: set relative path: %w", e))
		result.SystemError(
			temporalsdk_workflow.Now(ctx),
			task,
			"SIP extraction has failed.",
			fmt.Sprintf(
				`%s could not be successfully extracted. Please try again, or ask a system administrator to investigate.`,
				filepath.Base(path),
			),
		)
		return ""
	}

	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP extracted")

	return archiveExtract.ExtractPath
}

func writePREMISFile(ctx temporalsdk_workflow.Context, sip sip.SIP) error {
	var e error
	path := filepath.Join(sip.Path, "metadata", "premis.xml")

	// Add PREMIS objects.
	var addPREMISObjects activities.AddPREMISObjectsResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISObjectsName,
		&activities.AddPREMISObjectsParams{
			SIP:            sip,
			PREMISFilePath: path,
		},
	).Get(ctx, &addPREMISObjects)
	if e != nil {
		return e
	}

	// Add PREMIS event noting validate structure result.
	validateStructureOutcomeDetail := "SIP structure identified. SIP structure matches validation criteria."

	var addPREMISEvent activities.AddPREMISEventResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISEventName,
		&activities.AddPREMISEventParams{
			PREMISFilePath: path,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP structure\"",
			OutcomeDetail:  validateStructureOutcomeDetail,
			Failures:       nil,
		},
	).Get(ctx, &addPREMISEvent)
	if e != nil {
		return e
	}

	// Add PREMIS event noting validate SIP name result.
	validateSIPNameOutcomeDetail := fmt.Sprintf(
		"SIP name %q matches validation criteria.",
		sip.Name(),
	)

	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISEventName,
		&activities.AddPREMISEventParams{
			PREMISFilePath: path,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP name\"",
			OutcomeDetail:  validateSIPNameOutcomeDetail,
			Failures:       nil,
		},
	).Get(ctx, &addPREMISEvent)
	if e != nil {
		return e
	}

	// Add PREMIS events for the disallowed file format check.
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISEventName,
		&activities.AddPREMISEventParams{
			PREMISFilePath: path,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Check for disallowed file formats\"",
			OutcomeDetail:  "Format allowed",
			Failures:       nil,
		},
	).Get(ctx, &addPREMISEvent)
	if e != nil {
		return e
	}

	// Add PREMIS events for file format validation.
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISValidationEventName,
		&activities.AddPREMISValidationEventParams{
			SIP:            sip,
			PREMISFilePath: path,
			Summary: premis.EventSummary{
				Type:          "validation",
				Detail:        "name=\"Validate SIP file formats\"",
				Outcome:       "valid",
				OutcomeDetail: "File format complies with specification",
			},
		},
	).Get(ctx, &addPREMISEvent)
	if e != nil {
		return e
	}

	// Add PREMIS events for metadata validation.
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISEventName,
		&activities.AddPREMISEventParams{
			PREMISFilePath: path,
			Agent:          premis.AgentDefault(),
			Type:           "validation",
			Detail:         "name=\"Validate SIP metadata\"",
			OutcomeDetail:  "Metadata validation successful",
			Failures:       nil,
		},
	).Get(ctx, &addPREMISEvent)
	if e != nil {
		return e
	}

	// Add Enduro PREMIS agent.
	var addPREMISEnduroAgent activities.AddPREMISAgentResult
	e = temporalsdk_workflow.ExecuteActivity(
		withFilesystemActivityOpts(ctx),
		activities.AddPREMISAgentName,
		&activities.AddPREMISAgentParams{
			PREMISFilePath: path,
			Agent:          premis.AgentDefault(),
		},
	).Get(ctx, &addPREMISEnduroAgent)
	if e != nil {
		return e
	}

	return nil
}

func withFilesystemActivityOpts(ctx temporalsdk_workflow.Context) temporalsdk_workflow.Context {
	return temporalsdk_workflow.WithActivityOptions(ctx, temporalsdk_workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 2,
		RetryPolicy: &temporalsdk_temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})
}

// ul formats a list of strings as an unordered, Markdown-style list.
func ul(items []string) string {
	if len(items) == 0 {
		return ""
	}

	var s strings.Builder
	for _, i := range items {
		fmt.Fprintf(&s, "- %s\n", i)
	}

	return strings.TrimSuffix(s.String(), "\n")
}
