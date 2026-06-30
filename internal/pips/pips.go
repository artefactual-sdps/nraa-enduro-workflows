package pips

import (
	"path/filepath"
	"strings"

	"github.com/artefactual-sdps/nraa-enduro-workflows/internal/sip"
)

type PIP struct {
	// Path is the filepath of the PIP directory.
	Path string

	// ManifestPath is the filepath of the SIP manifest.
	ManifestPath string
}

func New(path string) PIP {
	return PIP{
		Path:         path,
		ManifestPath: filepath.Join(path, "objects", filepath.Base(path), "header", "metadata.xml"),
	}
}

func NewFromSIP(sip sip.SIP) PIP {
	return New(sip.Path)
}

func (p PIP) Name() string {
	return filepath.Base(p.Path)
}

func (p PIP) ConvertSIPPath(path string) string {
	switch name := filepath.Base(path); name {
	case "metadata.xml":
		return filepath.Join("objects", p.Name(), "header", name)
	case "metadata.xsd":
		return ""
	}

	if strings.HasPrefix(path, "content") {
		return filepath.Join("objects", p.Name(), path)
	}

	return ""
}
