package ocr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type TesseractEngine struct{}

func NewTesseractEngine() *TesseractEngine { return &TesseractEngine{} }

func (e *TesseractEngine) ExtractText(ctx context.Context, inputPath string) (string, error) {
	var out, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "tesseract", inputPath, "stdout", "-l", "eng")
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract failed: %s: %w", errBuf.String(), err)
	}
	return out.String(), nil
}

func (e *TesseractEngine) CreateSearchablePDF(ctx context.Context, inputPath string, outputPath string) error {
	base := filepath.Join(os.TempDir(), fmt.Sprintf("ocr_%d", os.Getpid()))
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "tesseract", inputPath, base, "pdf")
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tesseract failed: %s: %w", errBuf.String(), err)
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
