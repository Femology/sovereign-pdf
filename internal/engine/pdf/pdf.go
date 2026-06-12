package pdf

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
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"sovereign-pdf/internal/engine"
)

// ---------------------------------------------------------------------------
// Probe — Boot-time system dependency check
// ---------------------------------------------------------------------------

type ProbeResult struct {
	Tool      string
	Available bool
	Version   string
}

func ProbeTools() []ProbeResult {
	tools := []string{"pdfcpu", "gs", "pdftoppm", "pdfunite", "pdftotext"}
	results := make([]ProbeResult, 0, len(tools))
	for _, t := range tools {
		out, err := exec.Command(t, "--version").CombinedOutput()
		version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
		results = append(results, ProbeResult{
			Tool:      t,
			Available: err == nil,
			Version:   version,
		})
	}
	return results
}

func requireBinary(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("required system binary '%s' not found on PATH; please install it and restart the server", name)
	}
	return nil
}

func runCmd(ctx context.Context, name string, args ...string) error {
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %s: %w", name, strings.TrimSpace(errBuf.String()), err)
	}
	return nil
}

func runCmdOutput(ctx context.Context, name string, args ...string) (string, error) {
	var outBuf, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s failed: %s: %w", name, strings.TrimSpace(errBuf.String()), err)
	}
	return outBuf.String(), nil
}

// ---------------------------------------------------------------------------
// Engine struct
// ---------------------------------------------------------------------------

type GoPDFEngine struct{}

func NewGoPDFEngine() *GoPDFEngine { return &GoPDFEngine{} }

func relaxed() *model.Configuration {
	c := model.NewDefaultConfiguration()
	c.ValidationMode = model.ValidationRelaxed
	return c
}

// ---------------------------------------------------------------------------
// Organize — Merge
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Merge(ctx context.Context, inputPaths []string, outputPath string) error {
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.MergeCreateFile(inputPaths, outputPath, false, relaxed())
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("merge failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Organize — Split
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Split(ctx context.Context, inputPath string, outputDir string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	ch := make(chan error, 1)
	go func() { ch <- pdfcpuapi.SplitFile(inputPath, outputDir, 1, relaxed()) }()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-ch:
		if err != nil {
			return nil, fmt.Errorf("split failed: %w", err)
		}
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("read split dir: %w", err)
	}
	var paths []string
	for _, en := range entries {
		if !en.IsDir() && strings.EqualFold(filepath.Ext(en.Name()), ".pdf") {
			paths = append(paths, filepath.Join(outputDir, en.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// ---------------------------------------------------------------------------
// Organize — Reorder pages  (pdfcpu collect)
// pageSeq: comma-separated page numbers in desired order e.g. "3,1,2"
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) ReorderPages(ctx context.Context, inputPath, outputPath, pageSeq string) error {
	pages, err := engine.ParsePageSelection(pageSeq)
	if err != nil {
		return err
	}
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.CollectFile(inputPath, outputPath, pages, relaxed())
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("reorder failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Organize — Delete pages  (pdfcpu remove pages)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) DeletePages(ctx context.Context, inputPath, outputPath, pageRanges string) error {
	// pdfcpu removes pages in-place; copy first, then modify
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	pages, err := engine.ParsePageSelection(pageRanges)
	if err != nil {
		return err
	}
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.RemovePagesFile(outputPath, "", pages, relaxed())
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("delete pages failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Organize — Extract pages  (pdfcpu collect subset)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) ExtractPages(ctx context.Context, inputPath, outputPath, pageRanges string) error {
	pages, err := engine.ParsePageSelection(pageRanges)
	if err != nil {
		return err
	}
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.CollectFile(inputPath, outputPath, pages, relaxed())
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("extract pages failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Organize — Rotate pages
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Rotate(ctx context.Context, inputPath, outputPath string, degrees int, pageRanges string) error {
	var pages []string
	if pageRanges != "" {
		pages = strings.Split(pageRanges, ",")
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.RotateFile(outputPath, "", degrees, pages, relaxed())
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("rotate failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Optimize — Compress (ghostscript profiles)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Compress(ctx context.Context, inputPath, outputPath string, profile engine.CompressionProfile) error {
	if err := requireBinary("gs"); err != nil {
		// Fallback: pdfcpu optimize when Ghostscript is unavailable
		ch := make(chan error, 1)
		go func() { ch <- pdfcpuapi.OptimizeFile(inputPath, outputPath, relaxed()) }()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case optErr := <-ch:
			if optErr != nil {
				return fmt.Errorf("compress failed (install ghostscript for better results): %w", optErr)
			}
			return nil
		}
	}
	var setting string
	switch profile {
	case engine.CompressionLow:
		setting = "/screen"
	case engine.CompressionMedium:
		setting = "/ebook"
	case engine.CompressionHigh:
		setting = "/printer"
	default:
		setting = "/ebook"
	}
	return runCmd(ctx, "gs",
		"-sDEVICE=pdfwrite",
		"-dCompatibilityLevel=1.4",
		fmt.Sprintf("-dPDFSETTINGS=%s", setting),
		"-dNOPAUSE", "-dQUIET", "-dBATCH",
		fmt.Sprintf("-sOutputFile=%s", outputPath),
		inputPath,
	)
}

// ---------------------------------------------------------------------------
// Optimize — Repair (pdfcpu validate + clean)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Repair(ctx context.Context, inputPath, outputPath string) error {
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.OptimizeFile(inputPath, outputPath, relaxed())
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("repair failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Optimize — PDF/A conversion (ghostscript)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) ConvertToPDFA(ctx context.Context, inputPath, outputPath string) error {
	if err := requireBinary("gs"); err != nil {
		return err
	}
	return runCmd(ctx, "gs",
		"-dPDFA=2",
		"-dBATCH", "-dNOPAUSE",
		"-sColorConversionStrategy=UseDeviceIndependentColor",
		"-sDEVICE=pdfwrite",
		"-dPDFACompatibilityPolicy=1",
		fmt.Sprintf("-sOutputFile=%s", outputPath),
		inputPath,
	)
}

// ---------------------------------------------------------------------------
// Convert — PDF to Images (pdftoppm)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) ToImages(ctx context.Context, inputPath, outputDir string, format engine.ImageFormat, dpi int) ([]string, error) {
	if err := requireBinary("pdftoppm"); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	prefix := filepath.Join(outputDir, "page")
	args := []string{fmt.Sprintf("-r%d", dpi)}
	switch format {
	case engine.ImageFormatJPEG:
		args = append(args, "-jpeg")
	default:
		args = append(args, "-png")
	}
	args = append(args, inputPath, prefix)
	if err := runCmd(ctx, "pdftoppm", args...); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("read output dir: %w", err)
	}
	var paths []string
	for _, en := range entries {
		if !en.IsDir() {
			paths = append(paths, filepath.Join(outputDir, en.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// ---------------------------------------------------------------------------
// Convert — Extract embedded images (pdfcpu extract images)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) ExtractImages(ctx context.Context, inputPath, outputDir string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.ExtractImagesFile(inputPath, outputDir, nil, relaxed())
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-ch:
		if err != nil {
			return nil, fmt.Errorf("extract images failed: %w", err)
		}
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("read output dir: %w", err)
	}
	var paths []string
	for _, en := range entries {
		if !en.IsDir() {
			paths = append(paths, filepath.Join(outputDir, en.Name()))
		}
	}
	return paths, nil
}

// ---------------------------------------------------------------------------
// Convert — PDF to plain text (pdftotext via poppler)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) ToText(ctx context.Context, inputPath string) (string, error) {
	if err := requireBinary("pdftotext"); err != nil {
		return "", err
	}
	return runCmdOutput(ctx, "pdftotext", "-layout", inputPath, "-")
}

// ---------------------------------------------------------------------------
// Edit — Watermark / Stamp (pdfcpu)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Watermark(ctx context.Context, inputPath, outputPath string, opts engine.WatermarkOptions) error {
	var pages []string
	if opts.Pages != "" {
		pages = strings.Split(opts.Pages, ",")
	}
	if opts.Text != "" {
		desc := fmt.Sprintf("fontname:Helvetica, points:24, color:%s, rotation:%.0f, scalefactor:%.2f, opacity:%.2f",
			colourOrDefault(opts.Color, "0.5 0.5 0.5"),
			opts.Rotation,
			scaleOrDefault(opts.Scale),
			opacityOrDefault(opts.Opacity),
		)
		ch := make(chan error, 1)
		go func() {
			wm, err := pdfcpuapi.TextWatermark(opts.Text, desc, true, false, types.POINTS)
			if err != nil {
				ch <- err
				return
			}
			ch <- pdfcpuapi.AddWatermarksFile(inputPath, outputPath, pages, wm, relaxed())
		}()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-ch:
			if err != nil {
				return fmt.Errorf("watermark failed: %w", err)
			}
			return nil
		}
	}
	if opts.ImagePath != "" {
		desc := fmt.Sprintf("rotation:%.0f, scalefactor:%.2f, opacity:%.2f", opts.Rotation, scaleOrDefault(opts.Scale), opacityOrDefault(opts.Opacity))
		ch := make(chan error, 1)
		go func() {
			wm, err := pdfcpuapi.ImageWatermark(opts.ImagePath, desc, true, false, types.POINTS)
			if err != nil {
				ch <- err
				return
			}
			ch <- pdfcpuapi.AddWatermarksFile(inputPath, outputPath, pages, wm, relaxed())
		}()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-ch:
			if err != nil {
				return fmt.Errorf("image watermark failed: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("watermark: either Text or ImagePath must be provided")
}

// normalizeColor converts hex (#RRGGBB) or rgb() values to pdfcpu's "R G B" format (0–1 floats).
func normalizeColor(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return "0.5 0.5 0.5"
	}
	if strings.HasPrefix(c, "#") {
		hex := strings.TrimPrefix(c, "#")
		if len(hex) == 3 {
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		}
		if len(hex) == 6 {
			var r, g, b int
			if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b); err == nil {
				return fmt.Sprintf("%.3f %.3f %.3f", float64(r)/255, float64(g)/255, float64(b)/255)
			}
		}
	}
	return c
}

func colourOrDefault(c, def string) string {
	if c == "" {
		return def
	}
	return normalizeColor(c)
}

func scaleOrDefault(s float64) float64 {
	if s == 0 {
		return 0.5
	}
	return s
}

func opacityOrDefault(o float64) float64 {
	if o == 0 {
		return 0.3
	}
	return o
}

// ---------------------------------------------------------------------------
// Edit — Add Page Numbers (pdfcpu stamp)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) AddPageNumbers(ctx context.Context, inputPath, outputPath string, opts engine.PageNumberOptions) error {
	format := opts.Format
	if format == "" {
		format = "%d"
	}
	fontSize := opts.FontSize
	if fontSize == 0 {
		fontSize = 12
	}
	position := opts.Position
	if position == "" {
		position = "bc"
	}
	desc := fmt.Sprintf("fontname:Helvetica, points:%d, position:%s, offset:0 %d",
		fontSize, position, opts.Margin)

	var pages []string
	if opts.Pages != "" {
		pages = strings.Split(opts.Pages, ",")
	}

	ch := make(chan error, 1)
	go func() {
		wm, err := pdfcpuapi.TextWatermark(format, desc, true, false, types.POINTS)
		if err != nil {
			ch <- err
			return
		}
		ch <- pdfcpuapi.AddWatermarksFile(inputPath, outputPath, pages, wm, relaxed())
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("page numbers failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Edit — Crop (pdfcpu crop)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Crop(ctx context.Context, inputPath, outputPath string, opts engine.CropOptions) error {
	box := &model.Box{
		Rect: &types.Rectangle{
			LL: types.Point{X: opts.LeftPt, Y: opts.BottomPt},
			UR: types.Point{X: opts.RightPt, Y: opts.TopPt},
		},
	}
	var pages []string
	if opts.Pages != "" {
		pages = strings.Split(opts.Pages, ",")
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.CropFile(outputPath, "", pages, box, relaxed())
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("crop failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Security — Protect (pdfcpu encrypt)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Protect(ctx context.Context, inputPath, outputPath string, opts engine.ProtectOptions) error {
	conf := relaxed()
	conf.UserPW = opts.UserPassword
	conf.OwnerPW = opts.OwnerPassword
	conf.EncryptUsingAES = true
	conf.EncryptKeyLength = 256
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.EncryptFile(inputPath, outputPath, conf)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("protect failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Security — Unlock (pdfcpu decrypt)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Unlock(ctx context.Context, inputPath, outputPath, password string) error {
	conf := relaxed()
	conf.UserPW = password
	conf.OwnerPW = password
	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.DecryptFile(inputPath, outputPath, conf)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("unlock failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Forms — Extract form fields (pdfcpu)
// ---------------------------------------------------------------------------

// ExtractFormFields lists form field names by parsing pdfcpu's text output.
func (e *GoPDFEngine) ExtractFormFields(ctx context.Context, inputPath string) (map[string]string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	type result struct {
		fields interface{}
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		fields, ferr := pdfcpuapi.FormFields(f, relaxed())
		ch <- result{fields, ferr}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, fmt.Errorf("extract form fields failed: %w", res.err)
		}
		// FormFields returns []form.Field; use Stringer/name accessor via fmt
		result := make(map[string]string)
		if lines, ok := res.fields.([]interface{}); ok {
			for _, l := range lines {
				result[fmt.Sprint(l)] = ""
			}
		}
		return result, nil
	}
}

// FillForm fills PDF form fields using a JSON temp file passed to pdfcpu.
func (e *GoPDFEngine) FillForm(ctx context.Context, inputPath, outputPath string, fields map[string]string) error {
	// Build a minimal JSON structure pdfcpu's FillFormFile expects
	var sb strings.Builder
	sb.WriteString(`{"fields":[`)
	first := true
	for k, v := range fields {
		if !first {
			sb.WriteString(",")
		}
		first = false
		v = strings.ReplaceAll(v, `"`, `\"`)
		k = strings.ReplaceAll(k, `"`, `\"`)
		fmt.Fprintf(&sb, `{"id":"%s","value":"%s"}`, k, v)
	}
	sb.WriteString(`]}`)

	tmpJSON, err := os.CreateTemp("", "spdf-form-*.json")
	if err != nil {
		return fmt.Errorf("create temp json: %w", err)
	}
	defer os.Remove(tmpJSON.Name())
	if _, err := tmpJSON.WriteString(sb.String()); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	tmpJSON.Close()

	ch := make(chan error, 1)
	go func() {
		ch <- pdfcpuapi.FillFormFile(inputPath, tmpJSON.Name(), outputPath, relaxed())
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("fill form failed: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Compare (text-based diff using pdftotext)
// ---------------------------------------------------------------------------

func (e *GoPDFEngine) Compare(ctx context.Context, pathA, pathB, outputPath string) (*engine.CompareResult, error) {
	if err := requireBinary("pdftotext"); err != nil {
		return nil, err
	}
	textA, err := runCmdOutput(ctx, "pdftotext", "-layout", pathA, "-")
	if err != nil {
		return nil, fmt.Errorf("extract text A: %w", err)
	}
	textB, err := runCmdOutput(ctx, "pdftotext", "-layout", pathB, "-")
	if err != nil {
		return nil, fmt.Errorf("extract text B: %w", err)
	}

	if textA == textB {
		return &engine.CompareResult{Identical: true, DiffSummary: "Documents are textually identical."}, nil
	}

	linesA := strings.Split(textA, "\n")
	linesB := strings.Split(textB, "\n")
	var diffLines []string
	maxLen := len(linesA)
	if len(linesB) > maxLen {
		maxLen = len(linesB)
	}
	changedCount := 0
	for i := 0; i < maxLen; i++ {
		la, lb := "", ""
		if i < len(linesA) {
			la = linesA[i]
		}
		if i < len(linesB) {
			lb = linesB[i]
		}
		if la != lb {
			changedCount++
			diffLines = append(diffLines, fmt.Sprintf("Line %d:\n  A: %s\n  B: %s", i+1, la, lb))
			if len(diffLines) >= 50 {
				diffLines = append(diffLines, "... (diff truncated at 50 lines)")
				break
			}
		}
	}

	summary := fmt.Sprintf("%d differing lines detected between documents.", changedCount)
	diffContent := strings.Join(diffLines, "\n\n")

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(diffContent), 0644); err != nil {
			return nil, fmt.Errorf("write diff file: %w", err)
		}
	}

	return &engine.CompareResult{
		Identical:    false,
		DiffSummary:  summary,
		DiffFilePath: outputPath,
	}, nil
}
