package ocr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type TesseractEngine struct{}

func NewTesseractEngine() *TesseractEngine { return &TesseractEngine{} }

// convertPDFToImages converts a PDF to PNG images using pdftoppm.
// Returns the list of generated image paths in page order.
func convertPDFToImages(ctx context.Context, inputPath string) ([]string, string, error) {
	tmpDir, err := os.MkdirTemp("", "spdf-ocr-pages-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}
	prefix := filepath.Join(tmpDir, "page")
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdftoppm", "-r", "150", "-png", inputPath, prefix)
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return nil, "", fmt.Errorf("pdftoppm failed: %s: %w", strings.TrimSpace(errBuf.String()), err)
	}
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, "", fmt.Errorf("read pages dir: %w", err)
	}
	var pages []string
	for _, en := range entries {
		if !en.IsDir() {
			pages = append(pages, filepath.Join(tmpDir, en.Name()))
		}
	}
	sort.Strings(pages)
	return pages, tmpDir, nil
}

// isImageFile returns true if the path has a known image extension.
func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".tiff", ".tif", ".bmp", ".gif", ".webp":
		return true
	}
	return false
}

// ExtractText runs OCR and returns extracted text.
// If the input is a PDF, pages are first rasterised with pdftoppm.
func (e *TesseractEngine) ExtractText(ctx context.Context, inputPath string) (string, error) {
	inputs := []string{inputPath}
	var tmpDir string

	if !isImageFile(inputPath) {
		pages, dir, err := convertPDFToImages(ctx, inputPath)
		if err != nil {
			return "", err
		}
		inputs = pages
		tmpDir = dir
		defer os.RemoveAll(tmpDir)
	}

	var allText strings.Builder
	for _, img := range inputs {
		var out, errBuf bytes.Buffer
		cmd := exec.CommandContext(ctx, "tesseract", img, "stdout", "-l", "eng")
		cmd.Stdout = &out
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("tesseract failed on %s: %s: %w", filepath.Base(img), errBuf.String(), err)
		}
		allText.WriteString(out.String())
	}
	return allText.String(), nil
}

// CreateSearchablePDF creates a searchable PDF via Tesseract.
// If the input is a PDF, pages are rasterised first; the per-page PDFs are
// then merged using pdfunite.
func (e *TesseractEngine) CreateSearchablePDF(ctx context.Context, inputPath string, outputPath string) error {
	inputs := []string{inputPath}
	var tmpDir string

	if !isImageFile(inputPath) {
		pages, dir, err := convertPDFToImages(ctx, inputPath)
		if err != nil {
			return err
		}
		inputs = pages
		tmpDir = dir
		defer os.RemoveAll(tmpDir)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}

	// Run Tesseract on each page
	var pagePDFs []string
	for i, img := range inputs {
		pageBase := filepath.Join(os.TempDir(), fmt.Sprintf("spdf-ocr-%d-%d", os.Getpid(), i))
		var errBuf bytes.Buffer
		cmd := exec.CommandContext(ctx, "tesseract", img, pageBase, "pdf")
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tesseract failed on page %d: %s: %w", i+1, strings.TrimSpace(errBuf.String()), err)
		}
		pagePDF := pageBase + ".pdf"
		pagePDFs = append(pagePDFs, pagePDF)
	}
	defer func() {
		for _, p := range pagePDFs {
			os.Remove(p)
		}
	}()

	if len(pagePDFs) == 1 {
		if err := os.Rename(pagePDFs[0], outputPath); err != nil {
			data, rerr := os.ReadFile(pagePDFs[0])
			if rerr != nil {
				return rerr
			}
			return os.WriteFile(outputPath, data, 0644)
		}
		return nil
	}

	// Merge page PDFs into the final output using pdfunite
	args := append(pagePDFs, outputPath)
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdfunite", args...)
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pdfunite failed: %s: %w", strings.TrimSpace(errBuf.String()), err)
	}
	return nil
}

func CheckTesseractVersion() (string, bool) {
	out, err := exec.Command("tesseract", "--version").CombinedOutput()
	if err != nil {
		return "", false
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		return lines[0], true
	}
	return "", true
}
