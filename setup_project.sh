#!/usr/bin/env bash
# ============================================================
#  setup_project.sh  — Sovereign PDF Full Build Script
#  Writes all source files, builds the frontend, copies
#  static assets into internal/api/static, and compiles
#  the final Go binary.
# ============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

echo "==> [1/6] Creating directory structure..."
mkdir -p internal/engine/pdf
mkdir -p internal/engine/ocr
mkdir -p internal/engine/office
mkdir -p internal/worker
mkdir -p internal/api
mkdir -p cmd/server
mkdir -p storage/tmp_uploads
mkdir -p storage/tmp_outputs

# ---------------------------------------------------------------
# internal/engine/engine.go
# ---------------------------------------------------------------
echo "==> [2/6] Writing Go source files..."

cat > internal/engine/engine.go << 'GOEOF'
package engine

import (
	"context"
	"time"
)

type JobType string
type JobStatus string

const (
	JobMerge         JobType = "merge"
	JobSplit         JobType = "split"
	JobCompress      JobType = "compress"
	JobOCR           JobType = "ocr"
	JobOfficeConvert JobType = "office_convert"
)

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

type Job struct {
	ID          string    `json:"id"`
	Type        JobType   `json:"type"`
	Status      JobStatus `json:"status"`
	InputFiles  []string  `json:"input_files"`
	OutputFile  string    `json:"output_file"`
	ErrorMsg    string    `json:"error_msg"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at"`
}

type PDFEngine interface {
	Merge(ctx context.Context, inputPaths []string, outputPath string) error
	Split(ctx context.Context, inputPath string, outputDir string) ([]string, error)
	Compress(ctx context.Context, inputPath string, outputPath string) error
}

type OCREngine interface {
	ExtractText(ctx context.Context, inputPath string) (string, error)
	CreateSearchablePDF(ctx context.Context, inputPath string, outputPath string) error
}

type OfficeEngine interface {
	ConvertToPDF(ctx context.Context, inputPath string, outputPath string) error
}
GOEOF

# ---------------------------------------------------------------
# internal/engine/pdf/pdf.go
# ---------------------------------------------------------------
cat > internal/engine/pdf/pdf.go << 'GOEOF'
package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type GoPDFEngine struct{}

func NewGoPDFEngine() *GoPDFEngine { return &GoPDFEngine{} }

func relaxed() *model.Configuration {
	c := model.NewDefaultConfiguration()
	c.ValidationMode = model.ValidationRelaxed
	return c
}

func (e *GoPDFEngine) Merge(ctx context.Context, inputPaths []string, outputPath string) error {
	ch := make(chan error, 1)
	go func() { ch <- api.MergeCreateFile(inputPaths, outputPath, false, relaxed()) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("pdfcpu merge failed: %w", err)
		}
		return nil
	}
}

func (e *GoPDFEngine) Split(ctx context.Context, inputPath string, outputDir string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}
	ch := make(chan error, 1)
	go func() { ch <- api.SplitFile(inputPath, outputDir, 1, relaxed()) }()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-ch:
		if err != nil {
			return nil, fmt.Errorf("pdfcpu split failed: %w", err)
		}
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read split dir: %w", err)
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".pdf" {
			paths = append(paths, filepath.Join(outputDir, e.Name()))
		}
	}
	return paths, nil
}

func (e *GoPDFEngine) Compress(ctx context.Context, inputPath string, outputPath string) error {
	ch := make(chan error, 1)
	go func() { ch <- api.OptimizeFile(inputPath, outputPath, relaxed()) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("pdfcpu compress failed: %w", err)
		}
		return nil
	}
}
GOEOF

# ---------------------------------------------------------------
# internal/engine/ocr/ocr.go
# ---------------------------------------------------------------
cat > internal/engine/ocr/ocr.go << 'GOEOF'
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
GOEOF

# ---------------------------------------------------------------
# internal/engine/office/office.go
# ---------------------------------------------------------------
cat > internal/engine/office/office.go << 'GOEOF'
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
GOEOF

# ---------------------------------------------------------------
# internal/worker/queue.go
# ---------------------------------------------------------------
cat > internal/worker/queue.go << 'GOEOF'
package worker

import (
	"sync"

	"sovereign-pdf/internal/engine"
)

type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*engine.Job
}

func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]*engine.Job)}
}

func (s *JobStore) Add(job *engine.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *JobStore) Get(id string) (*engine.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

func (s *JobStore) List() []*engine.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*engine.Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}

func (s *JobStore) UpdateStatus(id string, status engine.JobStatus, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.ErrorMsg = errMsg
		if status == engine.StatusCompleted || status == engine.StatusFailed {
			j.CompletedAt = timeNow()
		}
	}
}

// timeNow is a variable so it can be patched in tests.
var timeNow = func() interface{} {
	import_time_placeholder := struct{}{}
	_ = import_time_placeholder
	return nil
}
GOEOF

# Fix the timeNow helper properly
cat > internal/worker/queue.go << 'GOEOF'
package worker

import (
	"sync"
	"time"

	"sovereign-pdf/internal/engine"
)

type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*engine.Job
}

func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]*engine.Job)}
}

func (s *JobStore) Add(job *engine.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *JobStore) Get(id string) (*engine.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

func (s *JobStore) List() []*engine.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*engine.Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}

func (s *JobStore) UpdateStatus(id string, status engine.JobStatus, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.ErrorMsg = errMsg
		if status == engine.StatusCompleted || status == engine.StatusFailed {
			j.CompletedAt = time.Now()
		}
	}
}
GOEOF

# ---------------------------------------------------------------
# internal/worker/pool.go
# ---------------------------------------------------------------
cat > internal/worker/pool.go << 'GOEOF'
package worker

import (
	"context"
	"log"
	"os"

	"sovereign-pdf/internal/engine"
)

type WorkerPool struct {
	jobQueue   chan *engine.Job
	store      *JobStore
	pdf        engine.PDFEngine
	ocr        engine.OCREngine
	office     engine.OfficeEngine
	numWorkers int
}

func NewWorkerPool(queue chan *engine.Job, store *JobStore, pdf engine.PDFEngine, ocr engine.OCREngine, office engine.OfficeEngine, numWorkers int) *WorkerPool {
	return &WorkerPool{
		jobQueue:   queue,
		store:      store,
		pdf:        pdf,
		ocr:        ocr,
		office:     office,
		numWorkers: numWorkers,
	}
}

func (p *WorkerPool) Start() {
	for i := 0; i < p.numWorkers; i++ {
		go p.workerLoop()
	}
}

func (p *WorkerPool) Stop() {
	close(p.jobQueue)
}

func (p *WorkerPool) workerLoop() {
	for job := range p.jobQueue {
		p.store.UpdateStatus(job.ID, engine.StatusProcessing, "")
		ctx := context.Background()
		var err error

		switch job.Type {
		case engine.JobMerge:
			log.Printf("[Worker] Merge job %s", job.ID)
			err = p.pdf.Merge(ctx, job.InputFiles, job.OutputFile)
		case engine.JobSplit:
			log.Printf("[Worker] Split job %s", job.ID)
			_, err = p.pdf.Split(ctx, job.InputFiles[0], job.OutputFile)
		case engine.JobCompress:
			log.Printf("[Worker] Compress job %s", job.ID)
			err = p.pdf.Compress(ctx, job.InputFiles[0], job.OutputFile)
		case engine.JobOCR:
			log.Printf("[Worker] OCR job %s", job.ID)
			err = p.ocr.CreateSearchablePDF(ctx, job.InputFiles[0], job.OutputFile)
		case engine.JobOfficeConvert:
			log.Printf("[Worker] Office Convert job %s", job.ID)
			err = p.office.ConvertToPDF(ctx, job.InputFiles[0], job.OutputFile)
		default:
			log.Printf("[Worker] Unknown job type %s for job %s", job.Type, job.ID)
		}

		// Always clean up input files after execution
		for _, f := range job.InputFiles {
			_ = os.Remove(f)
		}

		if err != nil {
			log.Printf("[Worker] Job %s failed: %v", job.ID, err)
			p.store.UpdateStatus(job.ID, engine.StatusFailed, err.Error())
		} else {
			p.store.UpdateStatus(job.ID, engine.StatusCompleted, "")
		}
	}
}
GOEOF

# ---------------------------------------------------------------
# internal/worker/reaper.go
# ---------------------------------------------------------------
cat > internal/worker/reaper.go << 'GOEOF'
package worker

import (
	"context"
	"log/slog"
	"os"
	"time"

	"sovereign-pdf/internal/engine"
)

func StartTTLReaper(ctx context.Context, store *JobStore, retentionPeriod time.Duration) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	slog.Info("TTL reaper started", "retention", retentionPeriod)
	for {
		select {
		case <-ctx.Done():
			slog.Info("TTL reaper shutting down")
			return
		case <-ticker.C:
			for _, job := range store.List() {
				if job.Status != engine.StatusCompleted && job.Status != engine.StatusFailed {
					continue
				}
				if time.Since(job.CompletedAt) < retentionPeriod {
					continue
				}
				if job.OutputFile == "" {
					continue
				}
				if _, err := os.Stat(job.OutputFile); os.IsNotExist(err) {
					continue
				}
				if err := os.RemoveAll(job.OutputFile); err != nil {
					slog.Error("reaper failed to purge file", "job_id", job.ID, "file", job.OutputFile, "error", err)
				} else {
					slog.Info("reaper purged stale file", "job_id", job.ID, "file", job.OutputFile)
				}
			}
		}
	}
}
GOEOF

# ---------------------------------------------------------------
# internal/api/router.go  — uses internal static/ dir for embed
# ---------------------------------------------------------------
cat > internal/api/router.go << 'GOEOF'
package api

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:static
var embeddedFiles embed.FS

func SetupRouter(h *Handler, uiDistDir string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/jobs/stream", h.HandleJobsStream)
	mux.HandleFunc("/api/jobs/merge", h.HandleMerge)
	mux.HandleFunc("/api/jobs/split", h.HandleSplit)
	mux.HandleFunc("/api/jobs/compress", h.HandleCompress)
	mux.HandleFunc("/api/jobs/ocr", h.HandleOCR)
	mux.HandleFunc("/api/jobs/convert", h.HandleConvert)
	mux.HandleFunc("/api/jobs/", h.HandleJobs)
	mux.HandleFunc("/api/download/", h.HandleDownload)

	// Try embedded static assets first (production binary)
	subFS, err := fs.Sub(embeddedFiles, "static")
	if err == nil {
		if _, statErr := subFS.Open("index.html"); statErr == nil {
			fileServer := http.FileServer(http.FS(subFS))
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				clean := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
				if _, openErr := subFS.Open(clean); openErr != nil {
					// SPA fallback
					indexData, _ := fs.ReadFile(subFS, "index.html")
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					_, _ = w.Write(indexData)
					return
				}
				fileServer.ServeHTTP(w, r)
			})
			return EnableCORS(mux)
		}
	}

	// Development fallback: serve from local ui/dist on disk
	if uiDistDir != "" {
		if _, statErr := os.Stat(uiDistDir); statErr == nil {
			fileServer := http.FileServer(http.Dir(uiDistDir))
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				path := filepath.Join(uiDistDir, r.URL.Path)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					http.ServeFile(w, r, filepath.Join(uiDistDir, "index.html"))
					return
				}
				fileServer.ServeHTTP(w, r)
			})
		}
	}

	return EnableCORS(mux)
}
GOEOF

# ---------------------------------------------------------------
# Build frontend
# ---------------------------------------------------------------
echo "==> [3/6] Building React frontend..."
cd ui
npm install --silent
npm run build
cd ..

# ---------------------------------------------------------------
# Copy dist into internal/api/static  (embed-friendly location)
# ---------------------------------------------------------------
echo "==> [4/6] Copying frontend assets to internal/api/static..."
rm -rf internal/api/static
cp -r ui/dist internal/api/static

# ---------------------------------------------------------------
# Compile Go binary
# ---------------------------------------------------------------
echo "==> [5/6] Compiling Go binary..."
go build -o sovereign-pdf ./cmd/server/main.go

# ---------------------------------------------------------------
# Done
# ---------------------------------------------------------------
echo ""
echo "==> [6/6] Done! Binary is ready:"
ls -lh sovereign-pdf
echo ""
echo "Run with:  ./sovereign-pdf"
echo "Open:      http://localhost:8080"
