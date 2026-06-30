package activities

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

func TestReportFailuresReportsCountLimits(t *testing.T) {
	t.Parallel()

	failures := reportFailures(&validationResult{
		fileCount:       maxSIPFiles + 1,
		folderCount:     maxSIPFolders + 1,
		hasContentDir:   true,
		hasHeaderDir:    true,
		hasMetadataFile: true,
		hasXSDFile:      true,
		oversizedFolders: []string{
			`Folder "content/f000001" contains 5001 files, exceeding the limit of 5000`,
		},
	}, sip.SIP{
		MetadataPath: filepath.Join("header", "metadata.xml"),
		XSDPath:      filepath.Join("header", "metadata.xsd"),
	})

	assert.DeepEqual(t, failures, []string{
		"SIP contains 1000000 files, exceeding the limit of 999999",
		"SIP contains 1000000 folders, exceeding the limit of 999999",
		`Folder "content/f000001" contains 5001 files, exceeding the limit of 5000`,
	})
}
