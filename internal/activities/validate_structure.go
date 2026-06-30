package activities

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

const (
	ValidateStructureName = "validate-structure"

	maxSIPFiles      = 999_999
	maxSIPFolders    = 999_999
	maxFolderFiles   = 5_000
	maxSIPPathLength = 250
)

var (
	contentDirNamePattern  = regexp.MustCompile(`^f\d{6}$`)
	contentFileNamePattern = regexp.MustCompile(`^d\d{6}\.[A-Za-z0-9]+$`)
)

type (
	ValidateStructure       struct{}
	ValidateStructureParams struct {
		SIP sip.SIP
	}

	ValidateStructureResult struct {
		Failures []string
	}
)

type validationResult struct {
	fileCount   int
	folderCount int

	hasContentDir   bool
	hasHeaderDir    bool
	hasMetadataFile bool
	hasXSDFile      bool

	extraDirs          []string
	extraFiles         []string
	invalidContentDirs []string
	invalidFileNames   []string
	duplicateDirs      []string
	duplicateFiles     []string
	oversizedFolders   []string
	longPaths          []string
}

func NewValidateStructure() *ValidateStructure {
	return &ValidateStructure{}
}

func (a *ValidateStructure) Execute(
	ctx context.Context,
	params *ValidateStructureParams,
) (*ValidateStructureResult, error) {
	res, err := validateStructure(params.SIP)
	if err != nil {
		return nil, err
	}
	failures := reportFailures(res, params.SIP)

	return &ValidateStructureResult{Failures: failures}, nil
}

// validateStructure walks the SIP directory tree and checks for structural
// issues like missing required files, invalid names, count limits, and duplicate
// names.
func validateStructure(s sip.SIP) (*validationResult, error) {
	res := &validationResult{}
	dirNames := make(map[string]string)
	fileNames := make(map[string]string)
	folderFiles := make(map[string]int)

	err := filepath.WalkDir(s.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(s.Path, path)
		if err != nil {
			return fmt.Errorf("ValidateStructure: relative path: %w", err)
		}

		if path != s.Path {
			validatePath(res, filepath.Join(s.Name(), relativePath))
		}

		if d.IsDir() {
			if path != s.Path {
				res.folderCount++
				recordDuplicate(relativePath, d.Name(), dirNames, &res.duplicateDirs)
			}
		} else {
			res.fileCount++
			folderFiles[filepath.Dir(relativePath)]++
			recordDuplicate(relativePath, d.Name(), fileNames, &res.duplicateFiles)
		}

		// Skip the rest of the checks for the SIP base path.
		if path == s.Path {
			return nil
		}

		switch {
		case path == s.ContentPath && d.IsDir():
			res.hasContentDir = true
		case isDescendantPath(s.ContentPath, path):
			validateContentNames(res, relativePath, d)
		case path == s.HeaderPath && d.IsDir():
			res.hasHeaderDir = true
		case isDescendantPath(s.HeaderPath, path):
			validateHeaderContents(res, s, path, relativePath, d)
		case filepath.Dir(relativePath) == ".":
			validateRootContents(res, s, path, relativePath, d)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ValidateStructure: %v", err)
	}

	for folder, count := range folderFiles {
		if count > maxFolderFiles {
			res.oversizedFolders = append(res.oversizedFolders, fmt.Sprintf(
				"Folder %q contains %d files, exceeding the limit of %d",
				folder,
				count,
				maxFolderFiles,
			))
		}
	}
	slices.Sort(res.oversizedFolders)

	return res, nil
}

func recordDuplicate(
	path string,
	name string,
	seen map[string]string,
	duplicates *[]string,
) {
	if first, ok := seen[name]; ok {
		*duplicates = append(*duplicates, fmt.Sprintf("%q is used by both %q and %q", name, first, path))

		return
	}

	seen[name] = path
}

func validatePath(res *validationResult, path string) {
	if utf8.RuneCountInString(path) > maxSIPPathLength {
		res.longPaths = append(res.longPaths, path)
	}
}

func validateRootContents(
	res *validationResult,
	s sip.SIP,
	path string,
	relativePath string,
	d fs.DirEntry,
) {
	switch {
	case d.IsDir() && path != s.ContentPath && path != s.HeaderPath:
		res.extraDirs = append(res.extraDirs, relativePath)
	case !d.IsDir():
		res.extraFiles = append(res.extraFiles, relativePath)
	}
}

func validateContentNames(
	res *validationResult,
	relativePath string,
	d fs.DirEntry,
) {
	if d.IsDir() {
		if !contentDirNamePattern.MatchString(d.Name()) {
			res.invalidContentDirs = append(res.invalidContentDirs, relativePath)
		}
		return
	}

	if !contentFileNamePattern.MatchString(d.Name()) {
		res.invalidFileNames = append(res.invalidFileNames, relativePath)
	}
}

func validateHeaderContents(
	res *validationResult,
	s sip.SIP,
	path string,
	relativePath string,
	d fs.DirEntry,
) {
	switch path {
	case s.MetadataPath:
		if !d.IsDir() {
			res.hasMetadataFile = true
		}
	case s.XSDPath:
		if !d.IsDir() {
			res.hasXSDFile = true
		}
	default:
		if d.IsDir() {
			res.extraDirs = append(res.extraDirs, relativePath)
		} else {
			res.extraFiles = append(res.extraFiles, relativePath)
		}
	}
}

func isDescendantPath(parent, path string) bool {
	return strings.HasPrefix(path, parent+string(os.PathSeparator))
}

// reportFailures takes the result of validateStructure and returns a list of
// human-readable failure messages.
func reportFailures(res *validationResult, sip sip.SIP) []string {
	var failures []string

	// Report an empty SIP and stop further checks to avoid reporting multiple
	// failures that are a consequence of the SIP being empty.
	if res.folderCount == 0 && res.fileCount == 0 {
		failures = append(failures, "The SIP is empty")
		return failures
	}

	if res.fileCount > maxSIPFiles {
		failures = append(
			failures,
			fmt.Sprintf("SIP contains %d files, exceeding the limit of %d", res.fileCount, maxSIPFiles),
		)
	}

	if res.folderCount > maxSIPFolders {
		failures = append(
			failures,
			fmt.Sprintf("SIP contains %d folders, exceeding the limit of %d", res.folderCount, maxSIPFolders),
		)
	}

	failures = append(failures, res.oversizedFolders...)

	// Report missing content directory.
	if !res.hasContentDir {
		failures = append(failures, "Content folder is missing")
	}

	// Report missing header directory.
	if !res.hasHeaderDir {
		failures = append(failures, "Header folder is missing")
	}

	// Report missing metadata file.
	if !res.hasMetadataFile {
		failures = append(failures, fmt.Sprintf("%s is missing", filepath.Base(sip.MetadataPath)))
	}

	// Report missing XSD file.
	if !res.hasXSDFile {
		failures = append(failures, fmt.Sprintf("%s is missing", filepath.Base(sip.XSDPath)))
	}

	// Report unexpected directories.
	for _, path := range res.extraDirs {
		failures = append(failures, fmt.Sprintf("Unexpected directory: %q", path))
	}

	// Report unexpected files.
	for _, path := range res.extraFiles {
		failures = append(failures, fmt.Sprintf("Unexpected file: %q", path))
	}

	for _, path := range res.invalidContentDirs {
		failures = append(failures, fmt.Sprintf(
			"Content directory %q does not match naming convention %q",
			path,
			"f000000",
		))
	}

	for _, duplicate := range res.duplicateDirs {
		failures = append(failures, fmt.Sprintf("Folder name is not unique: %s", duplicate))
	}

	for _, path := range res.invalidFileNames {
		failures = append(failures, fmt.Sprintf(
			"File %q does not match naming convention %q",
			path,
			"d000000.ext",
		))
	}

	for _, duplicate := range res.duplicateFiles {
		failures = append(failures, fmt.Sprintf("File name is not unique: %s", duplicate))
	}

	for _, path := range res.longPaths {
		failures = append(failures, fmt.Sprintf(
			"Path %q exceeds the %d character limit",
			path,
			maxSIPPathLength,
		))
	}

	return failures
}
