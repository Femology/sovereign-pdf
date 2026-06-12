package engine

import (
	"fmt"
	"strings"
)

// ParsePageSelection splits a user page expression like "1,3,5-7" into pdfcpu tokens.
func ParsePageSelection(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("page selection is required (e.g. 1,3,5-7)")
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("page selection is required (e.g. 1,3,5-7)")
	}
	return out, nil
}

// PageSeqFromMeta returns the page sequence from job metadata (page_seq or pages).
func PageSeqFromMeta(meta map[string]string) string {
	if meta == nil {
		return ""
	}
	if s := strings.TrimSpace(meta["page_seq"]); s != "" {
		return s
	}
	return strings.TrimSpace(meta["pages"])
}

// ValidateJobMetadata returns an error if required options are missing for a job type.
func ValidateJobMetadata(jobType JobType, meta map[string]string, fileCount int) error {
	if meta == nil {
		meta = map[string]string{}
	}

	switch jobType {
	case JobReorder, JobDeletePages, JobExtractPages:
		if _, err := ParsePageSelection(PageSeqFromMeta(meta)); err != nil {
			label := "pages"
			if jobType == JobReorder {
				label = "page order (page_seq)"
			}
			return fmt.Errorf("%s: %w", label, err)
		}
	case JobWatermark:
		if strings.TrimSpace(meta["text"]) == "" && strings.TrimSpace(meta["image_path"]) == "" {
			return fmt.Errorf("watermark text or image is required")
		}
	case JobProtect:
		if strings.TrimSpace(meta["user_password"]) == "" {
			return fmt.Errorf("user password is required")
		}
	case JobUnlock:
		if strings.TrimSpace(meta["password"]) == "" {
			return fmt.Errorf("current password is required")
		}
	case JobFillForm:
		if strings.TrimSpace(meta["fields"]) == "" {
			return fmt.Errorf("form fields are required (e.g. name=John,email=a@b.com)")
		}
	case JobCompare:
		if fileCount < 2 {
			return fmt.Errorf("compare requires exactly 2 PDF files")
		}
	}
	return nil
}
