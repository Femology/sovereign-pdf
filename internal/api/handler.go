package api

import (
	"archive/zip"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sovereign-pdf/internal/engine"
	"sovereign-pdf/internal/worker"
)

type Handler struct {
	store      *worker.JobStore
	queue      chan *engine.Job
	uploadDir  string
	outputDir  string
}

func NewHandler(store *worker.JobStore, queue chan *engine.Job, uploadDir, outputDir string) *Handler {
	// Ensure directories exist
	_ = os.MkdirAll(uploadDir, 0755)
	_ = os.MkdirAll(outputDir, 0755)

	return &Handler{
		store:     store,
		queue:     queue,
		uploadDir: uploadDir,
		outputDir: outputDir,
	}
}

// Write JSON responses helper
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	slog.Error("api error", "status", status, "msg", msg)
	writeJSON(w, status, map[string]string{"error": msg})
}

// Generate unique ID helper
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// HandleSubmit is the unified endpoint for all 16+ job types: POST /api/jobs/submit
func (h *Handler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB
		writeError(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	jobType := engine.JobType(r.FormValue("job_type"))
	if jobType == "" {
		writeError(w, http.StatusBadRequest, "job_type is required")
		return
	}

	// Read all uploaded files
	var inputPaths []string
	jobID := generateID()

	if files := r.MultipartForm.File["files"]; len(files) > 0 {
		for i, fileHeader := range files {
			ext := filepath.Ext(fileHeader.Filename)
			inputFileName := fmt.Sprintf("%s_%s_in_%d%s", jobID, jobType, i, ext)
			inputPath := filepath.Join(h.uploadDir, inputFileName)

			dst, err := os.Create(inputPath)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to save file: "+err.Error())
				return
			}

			file, err := fileHeader.Open()
			if err != nil {
				dst.Close()
				writeError(w, http.StatusInternalServerError, "Failed to open uploaded file: "+err.Error())
				return
			}

			if _, err = io.Copy(dst, file); err != nil {
				file.Close()
				dst.Close()
				writeError(w, http.StatusInternalServerError, "Failed to write file: "+err.Error())
				return
			}
			file.Close()
			dst.Close()
			inputPaths = append(inputPaths, inputPath)
		}
	} else if file, header, err := r.FormFile("file"); err == nil {
		defer file.Close()
		ext := filepath.Ext(header.Filename)
		inputFileName := fmt.Sprintf("%s_%s_in%s", jobID, jobType, ext)
		inputPath := filepath.Join(h.uploadDir, inputFileName)

		dst, err := os.Create(inputPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to save file: "+err.Error())
			return
		}
		defer dst.Close()

		if _, err = io.Copy(dst, file); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to save file: "+err.Error())
			return
		}
		inputPaths = append(inputPaths, inputPath)
	} else {
		writeError(w, http.StatusBadRequest, "No files provided")
		return
	}

	// Gather metadata
	meta := make(map[string]string)
	for key, values := range r.MultipartForm.Value {
		if len(values) > 0 && key != "job_type" {
			meta[key] = values[0]
		}
	}

	// Determine output path based on job type
	var outputPath string
	switch jobType {
	case engine.JobSplit, engine.JobPDFToImages, engine.JobExtractImages:
		outputPath = filepath.Join(h.outputDir, fmt.Sprintf("%s_output_dir", jobID))
		_ = os.MkdirAll(outputPath, 0755)
	case engine.JobPDFToText:
		outputPath = filepath.Join(h.outputDir, fmt.Sprintf("%s_output.txt", jobID))
	case engine.JobOCR:
		if meta["format"] == "txt" {
			outputPath = filepath.Join(h.outputDir, fmt.Sprintf("%s_output.txt", jobID))
		} else {
			outputPath = filepath.Join(h.outputDir, fmt.Sprintf("%s_output.pdf", jobID))
		}
	case engine.JobCompare:
		outputPath = filepath.Join(h.outputDir, fmt.Sprintf("%s_diff.txt", jobID))
	default:
		outputPath = filepath.Join(h.outputDir, fmt.Sprintf("%s_output.pdf", jobID))
	}

	job := &engine.Job{
		ID:         jobID,
		Type:       jobType,
		Status:     engine.StatusPending,
		InputFiles: inputPaths,
		OutputFile: outputPath,
		Metadata:   meta,
		CreatedAt:  time.Now(),
	}

	h.store.Add(job)
	h.queue <- job

	slog.Info("job created", "id", job.ID, "type", job.Type)
	writeJSON(w, http.StatusAccepted, job)
}

// Handle Jobs Stream: GET /api/jobs/stream
func (h *Handler) HandleJobsStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming unsupported")
		return
	}

	ctx := r.Context()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Push immediately upon connection
	h.pushJobsToStream(w, flusher)

	for {
		select {
		case <-ctx.Done():
			slog.Info("SSE client disconnected")
			return
		case <-ticker.C:
			h.pushJobsToStream(w, flusher)
		}
	}
}

func (h *Handler) pushJobsToStream(w http.ResponseWriter, flusher http.Flusher) {
	jobs := h.store.List()
	// Sort by creation time (newest first)
	for i := 0; i < len(jobs); i++ {
		for j := i + 1; j < len(jobs); j++ {
			if jobs[i].CreatedAt.Before(jobs[j].CreatedAt) {
				jobs[i], jobs[j] = jobs[j], jobs[i]
			}
		}
	}

	data, err := json.Marshal(jobs)
	if err != nil {
		slog.Error("failed to marshal jobs for SSE", "error", err.Error())
		return
	}

	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()
}

// Handle Job Status: GET /api/jobs/:id or GET /api/jobs
func (h *Handler) HandleJobs(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) > 3 && parts[3] != "" {
		id := parts[3]
		job, exists := h.store.Get(id)
		if !exists {
			writeError(w, http.StatusNotFound, "Job not found")
			return
		}
		writeJSON(w, http.StatusOK, job)
		return
	}

	list := h.store.List()
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].CreatedAt.Before(list[j].CreatedAt) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	writeJSON(w, http.StatusOK, list)
}

// Handle Download: GET /api/download/:id
func (h *Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[3] == "" {
		writeError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	id := parts[3]
	job, exists := h.store.Get(id)
	if !exists {
		writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	if job.Status != engine.StatusCompleted {
		writeError(w, http.StatusBadRequest, "Job is not completed yet")
		return
	}

	if job.Type == engine.JobSplit {
		h.serveSplitAsZip(w, job)
		return
	}

	if job.Type == engine.JobOCR {
		if strings.HasSuffix(job.OutputFile, ".txt") {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"ocr_%s.txt\"", id))
			file, err := os.Open(job.OutputFile)
			if err != nil {
				writeError(w, http.StatusNotFound, "OCR Text not found on disk")
				return
			}
			defer file.Close()
			_, _ = io.Copy(w, file)
			return
		}
	}

	if job.OutputFile == "" {
		writeError(w, http.StatusInternalServerError, "Job completed but no output path registered")
		return
	}

	file, err := os.Open(job.OutputFile)
	if err != nil {
		writeError(w, http.StatusNotFound, "Output file not found on disk")
		return
	}
	defer file.Close()

	filename := fmt.Sprintf("%s_%s.pdf", job.Type, id)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	_, _ = io.Copy(w, file)
}

func (h *Handler) serveSplitAsZip(w http.ResponseWriter, job *engine.Job) {
	if job.OutputFile == "" {
		writeError(w, http.StatusInternalServerError, "No split directory available")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"split_%s.zip\"", job.ID))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	files, err := os.ReadDir(job.OutputFile)
	if err != nil {
		slog.Error("failed to read split directory", "directory", job.OutputFile, "error", err)
		return
	}

	for _, entry := range files {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(job.OutputFile, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			slog.Error("failed to open split page", "path", path, "error", err)
			continue
		}

		writer, err := zipWriter.Create(entry.Name())
		if err != nil {
			file.Close()
			slog.Error("failed to create zip entry", "path", path, "error", err)
			continue
		}

		_, _ = io.Copy(writer, file)
		file.Close()
	}
}

// CORS Middleware
func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
