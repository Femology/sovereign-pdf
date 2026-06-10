package engine

import (
	"context"
	"time"
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

type JobType string
type JobStatus string
type CompressionProfile string
type ImageFormat string

const (
	// Organize
	JobMerge        JobType = "merge"
	JobSplit        JobType = "split"
	JobReorder      JobType = "reorder"
	JobDeletePages  JobType = "delete_pages"
	JobExtractPages JobType = "extract_pages"
	JobRotate       JobType = "rotate"

	// Optimize & Repair
	JobCompress JobType = "compress"
	JobRepair   JobType = "repair"
	JobPDFA     JobType = "pdfa"

	// Convert
	JobOfficeConvert JobType = "office_convert"
	JobPDFToImages   JobType = "pdf_to_images"
	JobExtractImages JobType = "extract_images"
	JobPDFToText     JobType = "pdf_to_text"

	// Edit & Layout
	JobWatermark   JobType = "watermark"
	JobPageNumbers JobType = "page_numbers"
	JobCrop        JobType = "crop"

	// Security
	JobProtect JobType = "protect"
	JobUnlock  JobType = "unlock"
	JobRedact  JobType = "redact"

	// OCR
	JobOCR JobType = "ocr"

	// Forms & Compare
	JobFillForm        JobType = "fill_form"
	JobExtractFormData JobType = "extract_form"
	JobCompare         JobType = "compare"
)

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

const (
	CompressionLow    CompressionProfile = "low"    // ghostscript /screen
	CompressionMedium CompressionProfile = "medium" // ghostscript /ebook
	CompressionHigh   CompressionProfile = "high"   // ghostscript /printer
)

const (
	ImageFormatPNG  ImageFormat = "png"
	ImageFormatJPEG ImageFormat = "jpg"
)

// ---------------------------------------------------------------------------
// Option Structs
// ---------------------------------------------------------------------------

type WatermarkOptions struct {
	Text     string  // Text to stamp (mutually exclusive with ImagePath)
	ImagePath string  // Path to watermark image (mutually exclusive with Text)
	Opacity  float64 // 0.0–1.0
	Rotation float64 // Degrees
	Scale    float64 // Scale factor relative to page (0.0–1.0)
	Color    string  // Hex or named colour, e.g. "0.5 0.5 0.5" (gray)
	Pages    string  // Page range expression, e.g. "1-3,5"
}

type PageNumberOptions struct {
	Format     string // e.g. "Page %d of %d"
	FontSize   int
	Margin     int    // Distance in points from edge
	Position   string // tl, tc, tr, bl, bc, br (top/bottom left/center/right)
	StartPage  int
	Pages      string // Page range
}

type CropOptions struct {
	// MediaBox values in points (72 pt = 1 inch)
	LeftPt   float64
	BottomPt float64
	RightPt  float64
	TopPt    float64
	Pages    string
}

type ProtectOptions struct {
	UserPassword  string
	OwnerPassword string
	Permissions   []string // print, modify, copy, annotate
}

type RedactArea struct {
	Page   int
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type CompareResult struct {
	Identical    bool
	DiffSummary  string
	DiffFilePath string // Path to visual diff PDF/image if generated
}

// ---------------------------------------------------------------------------
// Core Job Struct
// ---------------------------------------------------------------------------

type Job struct {
	ID          string            `json:"id"`
	Type        JobType           `json:"type"`
	Status      JobStatus         `json:"status"`
	InputFiles  []string          `json:"input_files"`
	OutputFile  string            `json:"output_file"`
	ErrorMsg    string            `json:"error_msg"`
	Metadata    map[string]string `json:"metadata,omitempty"` // Flexible key-value for job params
	CreatedAt   time.Time         `json:"created_at"`
	CompletedAt time.Time         `json:"completed_at"`
}

// ---------------------------------------------------------------------------
// Engine Interfaces
// ---------------------------------------------------------------------------

// PDFEngine handles all PDF-native operations.
type PDFEngine interface {
	// Organize
	Merge(ctx context.Context, inputPaths []string, outputPath string) error
	Split(ctx context.Context, inputPath string, outputDir string) ([]string, error)
	ReorderPages(ctx context.Context, inputPath string, outputPath string, pageSeq string) error
	DeletePages(ctx context.Context, inputPath string, outputPath string, pageRanges string) error
	ExtractPages(ctx context.Context, inputPath string, outputPath string, pageRanges string) error
	Rotate(ctx context.Context, inputPath string, outputPath string, degrees int, pageRanges string) error

	// Optimize & Repair
	Compress(ctx context.Context, inputPath string, outputPath string, profile CompressionProfile) error
	Repair(ctx context.Context, inputPath string, outputPath string) error
	ConvertToPDFA(ctx context.Context, inputPath string, outputPath string) error

	// Convert
	ToImages(ctx context.Context, inputPath string, outputDir string, format ImageFormat, dpi int) ([]string, error)
	ExtractImages(ctx context.Context, inputPath string, outputDir string) ([]string, error)
	ToText(ctx context.Context, inputPath string) (string, error)

	// Edit & Layout
	Watermark(ctx context.Context, inputPath string, outputPath string, opts WatermarkOptions) error
	AddPageNumbers(ctx context.Context, inputPath string, outputPath string, opts PageNumberOptions) error
	Crop(ctx context.Context, inputPath string, outputPath string, opts CropOptions) error

	// Security
	Protect(ctx context.Context, inputPath string, outputPath string, opts ProtectOptions) error
	Unlock(ctx context.Context, inputPath string, outputPath string, password string) error

	// Forms
	ExtractFormFields(ctx context.Context, inputPath string) (map[string]string, error)
	FillForm(ctx context.Context, inputPath string, outputPath string, fields map[string]string) error

	// Compare
	Compare(ctx context.Context, pathA string, pathB string, outputPath string) (*CompareResult, error)
}

// OCREngine handles optical character recognition tasks.
type OCREngine interface {
	ExtractText(ctx context.Context, inputPath string) (string, error)
	CreateSearchablePDF(ctx context.Context, inputPath string, outputPath string) error
}

// OfficeEngine handles Office document conversion.
type OfficeEngine interface {
	ConvertToPDF(ctx context.Context, inputPath string, outputPath string) error
}
