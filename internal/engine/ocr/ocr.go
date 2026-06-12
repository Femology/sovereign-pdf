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

	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type TesseractEngine struct{}

func NewTesseractEngine() *TesseractEngine { return &TesseractEngine{} }

func (e *TesseractEngine) ExtractText(ctx context.Context, inputPath string) (string, error) {
	if strings.EqualFold(filepath.Ext(inputPath), ".pdf") {
		return e.extractTextFromPDF(ctx, inputPath)
	}
	return e.runTesseractText(ctx, inputPath)
}

func (e *TesseractEngine) CreateSearchablePDF(ctx context.Context, inputPath string, outputPath string) error {
	if strings.EqualFold(filepath.Ext(inputPath), ".pdf") {
		return e.createSearchableFromPDF(ctx, inputPath, outputPath)
	}
	return e.runTesseractPDF(ctx, inputPath, outputPath)
}

func (e *TesseractEngine) runTesseractText(ctx context.Context, inputPath string) (string, error) {
	var out, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "tesseract", inputPath, "stdout", "-l", "eng")
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract failed: %s: %w", strings.TrimSpace(errBuf.String()), err)
	}
	return out.String(), nil
}

func (e *TesseractEngine) runTesseractPDF(ctx context.Context, inputPath, outputPath string) error {
	base := filepath.Join(os.TempDir(), fmt.Sprintf("ocr_%d", os.Getpid()))
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "tesseract", inputPath, base, "pdf")
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tesseract failed: %s: %w", strings.TrimSpace(errBuf.String()), err)
	}
	tmp := base + ".pdf"
	defer os.Remove(tmp)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}
	if err := os.Rename(tmp, outputPath); err != nil {
		data, rerr := os.ReadFile(tmp)
		if rerr != nil {
			return rerr
		}
		return os.WriteFile(outputPath, data, 0644)
	}
	return nil
}

func (e *TesseractEngine) extractTextFromPDF(ctx context.Context, inputPath string) (string, error) {
	images, cleanup, err := e.renderPDFPages(ctx, inputPath)
	defer cleanup()
	if err != nil {
		return "", err
	}
	var parts []string
	for _, img := range images {
		text, err := e.runTesseractText(ctx, img)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n"), nil
}

func (e *TesseractEngine) createSearchableFromPDF(ctx context.Context, inputPath, outputPath string) error {
	images, cleanup, err := e.renderPDFPages(ctx, inputPath)
	defer cleanup()
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "ocr_pdf_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	var pagePDFs []string
	for i, img := range images {
		out := filepath.Join(tmpDir, fmt.Sprintf("page_%03d.pdf", i+1))
		if err := e.runTesseractPDF(ctx, img, out); err != nil {
			return err
		}
		pagePDFs = append(pagePDFs, out)
	}
	if len(pagePDFs) == 1 {
		data, err := os.ReadFile(pagePDFs[0])
		if err != nil {
			return err
		}
		return os.WriteFile(outputPath, data, 0644)
	}
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	return pdfcpuapi.MergeCreateFile(pagePDFs, outputPath, false, conf)
}

func (e *TesseractEngine) renderPDFPages(ctx context.Context, inputPath string) ([]string, func(), error) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return nil, func() {}, fmt.Errorf("pdftoppm is required for PDF OCR — run: sudo ./install_deps.sh")
	}
	tmpDir, err := os.MkdirTemp("", "ocr_ppm_*")
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	prefix := filepath.Join(tmpDir, "page")
	cmd := exec.CommandContext(ctx, "pdftoppm", "-png", "-r", "200", inputPath, prefix)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("pdftoppm failed: %s: %w", strings.TrimSpace(errBuf.String()), err)
	}
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	var images []string
	for _, en := range entries {
		if !en.IsDir() && strings.HasSuffix(strings.ToLower(en.Name()), ".png") {
			images = append(images, filepath.Join(tmpDir, en.Name()))
		}
	}
	sort.Strings(images)
	if len(images) == 0 {
		cleanup()
		return nil, func() {}, fmt.Errorf("no pages rendered from PDF")
	}
	return images, cleanup, nil
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
