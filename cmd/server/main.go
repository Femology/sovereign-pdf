package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sovereign-pdf/internal/api"
	"sovereign-pdf/internal/engine"
	"sovereign-pdf/internal/engine/ocr"
	"sovereign-pdf/internal/engine/office"
	"sovereign-pdf/internal/engine/pdf"
	"sovereign-pdf/internal/worker"
)

const banner = `
╭────────────────────────────────────────────────────────╮
│                                                        │
│   ▄▄▄▄▄▄   ▄▄▄▄▄▄  ▄▄   ▄▄ ▄▄▄▄▄▄ ▄▄▄▄▄▄ ▄▄▄▄▄▄▄ ▄▄▄▄▄  │
│  ██    ██ ██    ██ ██   ██ ██     ██  ██ ██       ██  ██ │
│  ██▄▄▄▄▄  ██    ██ ██   ██ ████   ██████ ████▄▄   ████   │
│       ██  ██    ██  ██ ██  ██     ██  ██ ██       ██  ██ │
│  ██████   ▀██████    ███   ██████ ██  ██ ███████ ██   ██ │
│                                                        │
│                    SOVEREIGN PDF                       │
│      Self-Hosted Private PDF & Vision Subsystem       │
│                                                        │
╰────────────────────────────────────────────────────────╯
`

func main() {
	port := flag.Int("port", 8080, "Port to run the server on")
	workers := flag.Int("workers", 4, "Number of worker threads")
	dataDir := flag.String("data", "./storage", "Base directory for temporary storage")
	flag.Parse()

	fmt.Print(banner)

	uploadDir := filepath.Join(*dataDir, "tmp_uploads")
	outputDir := filepath.Join(*dataDir, "tmp_outputs")

	// Ensure directories exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// 1. Initialize Engines
	pdfEngine := pdf.NewGoPDFEngine()
	ocrEngine := ocr.NewTesseractEngine()
	officeEngine := office.NewLibreOfficeEngine()

	// Proactively check dependencies
	log.Println("[Init] Probing system dependencies...")
	if ver, ok := ocr.CheckTesseractVersion(); ok {
		log.Printf("[Init]   - OCR Engine: Tesseract OCR available (%s)\n", ver)
	} else {
		log.Println("[Init]   - OCR Engine: Tesseract OCR NOT found on path. OCR features will fail.")
	}

	if ver, ok := office.CheckLibreOfficeVersion(); ok {
		log.Printf("[Init]   - Office Engine: LibreOffice available (%s)\n", ver)
	} else {
		log.Println("[Init]   - Office Engine: LibreOffice NOT found on path. Office conversions will fail.")
	}

	// 2. Initialize Queue and Store
	log.Println("[Init] Bootstrapping Job Queues...")
	jobStore := worker.NewJobStore()
	jobQueue := make(chan *engine.Job, 100)

	// 3. Spawn Workers
	workerPool := worker.NewWorkerPool(jobQueue, jobStore, pdfEngine, ocrEngine, officeEngine, *workers)
	workerPool.Start()

	// 4. Setup API router
	// Check UI build directory
	uiDist := "./ui/dist"
	if _, err := os.Stat(uiDist); err != nil {
		// Try absolute path check
		uiDist = filepath.Join(".", "ui", "dist")
	}

	handler := api.NewHandler(jobStore, jobQueue, uploadDir, outputDir)
	router := api.SetupRouter(handler, uiDist)

	addr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// 5. Run Server and handle graceful shutdown
	go func() {
		log.Printf("[Server] Starting Sovereign PDF on port %d...\n", *port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Server] Failed to listen: %v\n", err)
		}
	}()

	// Signal channel for interrupts
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Println("[Server] Gracefully shutting down...")

	// Stop accepting new connections
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[Server] Forced shutdown: %v\n", err)
	}

	// Stop Worker Pool
	log.Println("[Workers] Halting background worker threads...")
	workerPool.Stop()

	// Clean up temporary uploads/outputs (optional)
	log.Println("[Server] Server successfully halted. Goodbye!")
}
