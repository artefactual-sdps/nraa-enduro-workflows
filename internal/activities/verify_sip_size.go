package activities

import (
	"context"
	"fmt"
	"os"
)

const (
	VerifySIPSizeName = "verify-sip-size"

	maxSIPSizeBytes int64 = 8_000_000_000
)

type (
	VerifySIPSize       struct{}
	VerifySIPSizeParams struct {
		Path string
	}
	VerifySIPSizeResult struct {
		Failures []string
	}
)

func NewVerifySIPSize() *VerifySIPSize {
	return &VerifySIPSize{}
}

func (a *VerifySIPSize) Execute(ctx context.Context, params *VerifySIPSizeParams) (*VerifySIPSizeResult, error) {
	info, err := os.Stat(params.Path)
	if err != nil {
		return nil, fmt.Errorf("verify SIP size: stat: %v", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("verify SIP size: %q is not a file", params.Path)
	}

	if info.Size() > maxSIPSizeBytes {
		return &VerifySIPSizeResult{Failures: []string{
			fmt.Sprintf(
				"SIP size is %s, exceeding the limit of %s by %s",
				humanFileSize(info.Size()),
				humanFileSize(maxSIPSizeBytes),
				humanFileSize(info.Size()-maxSIPSizeBytes),
			),
		}}, nil
	}

	return &VerifySIPSizeResult{}, nil
}

func humanFileSize(size int64) string {
	const unit = 1000
	units := []string{"B", "KB", "MB", "GB", "TB"}

	value := float64(size)
	for _, suffix := range units {
		if value < unit || suffix == units[len(units)-1] {
			return fmt.Sprintf("%.3g %s", value, suffix)
		}
		value /= unit
	}

	return fmt.Sprintf("%d B", size)
}
