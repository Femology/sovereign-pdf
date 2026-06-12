package engine

import (
	"fmt"
	"os/exec"
)

// ToolRequirement describes an external binary needed for certain job types.
type ToolRequirement struct {
	Binary      string
	DisplayName string
	InstallHint string
}

var jobToolRequirements = map[JobType][]ToolRequirement{
	JobCompress:      {{Binary: "gs", DisplayName: "Ghostscript", InstallHint: "sudo ./install_deps.sh"}},
	JobPDFA:          {{Binary: "gs", DisplayName: "Ghostscript", InstallHint: "sudo ./install_deps.sh"}},
	JobOCR:           {{Binary: "tesseract", DisplayName: "Tesseract OCR", InstallHint: "sudo ./install_deps.sh"}},
	JobOfficeConvert: {{Binary: "soffice", DisplayName: "LibreOffice", InstallHint: "sudo ./install_deps.sh"}},
	JobPDFToText:     {{Binary: "pdftotext", DisplayName: "Poppler (pdftotext)", InstallHint: "sudo ./install_deps.sh"}},
	JobPDFToImages:   {{Binary: "pdftoppm", DisplayName: "Poppler (pdftoppm)", InstallHint: "sudo ./install_deps.sh"}},
}

// RequiredTools returns external tool requirements for a job type.
func RequiredTools(jobType JobType) []ToolRequirement {
	return jobToolRequirements[jobType]
}

// IsToolAvailable checks if a binary exists on PATH.
func IsToolAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ValidateJobRequirements returns an error if required binaries are missing.
// Compress falls back to pdfcpu when Ghostscript is absent, so it is not enforced.
func ValidateJobRequirements(jobType JobType) error {
	for _, req := range RequiredTools(jobType) {
		if jobType == JobCompress && req.Binary == "gs" {
			continue // pdfcpu fallback available
		}
		if !IsToolAvailable(req.Binary) {
			// LibreOffice may be available as libreoffice instead of soffice
			if req.Binary == "soffice" && IsToolAvailable("libreoffice") {
				continue
			}
			return fmt.Errorf("%s is not installed — run: %s", req.DisplayName, req.InstallHint)
		}
	}
	return nil
}

// ProbeAllTools returns availability for all known external dependencies.
func ProbeAllTools() []struct {
	Name      string
	Available bool
} {
	names := []string{"gs", "pdftotext", "pdftoppm", "tesseract", "soffice", "libreoffice"}
	out := make([]struct {
		Name      string
		Available bool
	}, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, struct {
			Name      string
			Available bool
		}{Name: n, Available: IsToolAvailable(n)})
	}
	return out
}
