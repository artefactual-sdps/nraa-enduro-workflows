package activities

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"unicode/utf8"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

const ValidateMetadataEncodingName = "validate-metadata-encoding"

var xmlEncodingPattern = regexp.MustCompile(`(?i)<\?xml\s+[^>]*encoding\s*=\s*["']([^"']*)["']`)

type (
	ValidateMetadataEncoding       struct{}
	ValidateMetadataEncodingParams struct {
		SIP sip.SIP
	}

	ValidateMetadataEncodingResult struct {
		Failures []string
	}
)

func NewValidateMetadataEncoding() *ValidateMetadataEncoding {
	return &ValidateMetadataEncoding{}
}

func (a *ValidateMetadataEncoding) Execute(
	ctx context.Context,
	params *ValidateMetadataEncodingParams,
) (*ValidateMetadataEncodingResult, error) {
	failures, err := validateMetadataEncoding(params.SIP)
	if err != nil {
		return nil, err
	}

	return &ValidateMetadataEncodingResult{Failures: failures}, nil
}

func validateMetadataEncoding(s sip.SIP) ([]string, error) {
	var failures []string

	for _, path := range []string{s.MetadataPath, s.XSDPath} {
		if isUTF8XML(path) {
			continue
		}
		relativePath, err := filepath.Rel(s.Path, path)
		if err != nil {
			return nil, fmt.Errorf("validate metadata encoding: relative path: %v", err)
		}
		failures = append(failures, fmt.Sprintf("%q must use UTF-8 encoding", relativePath))
	}

	return failures, nil
}

func isUTF8XML(path string) bool {
	b, err := os.ReadFile(path) // #nosec: G304 -- trusted SIP path.
	if err != nil {
		return false
	}
	if !utf8.Valid(b) {
		return false
	}

	matches := xmlEncodingPattern.FindSubmatch(bytes.TrimSpace(b))
	if matches == nil {
		return true
	}

	return bytes.EqualFold(matches[1], []byte("UTF-8"))
}
