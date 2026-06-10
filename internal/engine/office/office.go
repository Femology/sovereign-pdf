package office

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type LibreOfficeEngine struct{}

func NewLibreOfficeEngine() *LibreOfficeEngine { return &LibreOfficeEngine{} }

func resolveBinary() (string, error) {
	for _, b := range []string{"soffice", "libreoffice"} {
		if _, err := exec.LookPath(b); err == nil {
			return b, nil
		}
	}
	return "", fmt.Errorf("neither soffice nor libreoffice found on PATH")
}

func (e *LibreOfficeEngine) ConvertToPDF(ctx context.Context, inputPath string, outputPath string) error {
	bin, err := resolveBinary()
	if err != nil {
		return err
	}
	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("mkdir failed: %w", err)
	}
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "--headless", "--convert-to", "pdf", "--outdir", outDir, inputPath)
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("libreoffice failed: %s: %w", errBuf.String(), err)
	}
	base := filepath.Base(inputPath)
	ext := filepath.Ext(base)
	generated := filepath.Join(outDir, base[:len(base)-len(ext)]+".pdf")
	if generated != outputPath {
		if err := os.Rename(generated, outputPath); err != nil {
			return fmt.Errorf("rename failed: %w", err)
		}
	}
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("output PDF not found at %s", outputPath)
	}
	return nil
}

func CheckLibreOfficeVersion() (string, bool) {
	bin, err := resolveBinary()
	if err != nil {
		return "", false
	}
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}
