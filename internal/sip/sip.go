package sip

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type SIP struct {
	// Path is the filepath of the SIP directory.
	Path string

	// ContentPath is the filepath of the "content" directory.
	ContentPath string

	// HeaderPath is the filepath of the "header" directory.
	HeaderPath string

	// ManifestPath is the filepath of the SIP manifest.
	ManifestPath string

	// MetadataPath is the filepath of the "metadata.xml" file.
	MetadataPath string

	// XSDPath is the filepath of the "metadata.xsd" file.
	XSDPath string

	// TopLevelPaths is a list of all the top level SIP directories.
	TopLevelPaths []string
}

func New(path string) (SIP, error) {
	s := SIP{}

	if _, err := os.Stat(path); err != nil {
		return s, fmt.Errorf("SIP: New: %v", err)
	}

	s.Path = path
	s.ContentPath = filepath.Join(path, "content")
	s.HeaderPath = filepath.Join(path, "header")
	s.MetadataPath = filepath.Join(s.HeaderPath, "metadata.xml")
	s.ManifestPath = s.MetadataPath
	s.XSDPath = filepath.Join(s.HeaderPath, "metadata.xsd")
	s.TopLevelPaths = []string{
		s.ContentPath,
		s.HeaderPath,
	}

	return s, nil
}

func (s SIP) Name() string {
	return filepath.Base(s.Path)
}

func (s SIP) HasValidName() bool {
	matches := regexp.MustCompile(`^SIP_(\d{8})_([A-Za-z0-9]{2,5})_(\d{4})_(\d{3})$`).FindStringSubmatch(s.Name())
	if matches == nil {
		return false
	}

	_, err := time.Parse("20060102", matches[1])

	return err == nil
}
