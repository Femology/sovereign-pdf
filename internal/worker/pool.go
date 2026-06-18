package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

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
		meta := job.Metadata
		if meta == nil {
			meta = map[string]string{}
		}
		var err error

		switch job.Type {
		// Organize
		case engine.JobMerge:
			err = p.pdf.Merge(ctx, job.InputFiles, job.OutputFile)
		case engine.JobSplit:
			_, err = p.pdf.Split(ctx, job.InputFiles[0], job.OutputFile)
		case engine.JobReorder:
			pageSeq := meta["page_seq"]
			if pageSeq == "" {
				pageSeq = meta["pages"]
			}
			err = p.pdf.ReorderPages(ctx, job.InputFiles[0], job.OutputFile, pageSeq)
		case engine.JobDeletePages:
			err = p.pdf.DeletePages(ctx, job.InputFiles[0], job.OutputFile, meta["pages"])
		case engine.JobExtractPages:
			err = p.pdf.ExtractPages(ctx, job.InputFiles[0], job.OutputFile, meta["pages"])
		case engine.JobRotate:
			deg := 90
			if d := meta["degrees"]; d != "" {
				fmt.Sscanf(d, "%d", &deg)
			}
			err = p.pdf.Rotate(ctx, job.InputFiles[0], job.OutputFile, deg, meta["pages"])

		// Optimize & Repair
		case engine.JobCompress:
			profile := engine.CompressionProfile(meta["profile"])
			if profile == "" {
				profile = engine.CompressionMedium
			}
			err = p.pdf.Compress(ctx, job.InputFiles[0], job.OutputFile, profile)
		case engine.JobRepair:
			err = p.pdf.Repair(ctx, job.InputFiles[0], job.OutputFile)
		case engine.JobPDFA:
			err = p.pdf.ConvertToPDFA(ctx, job.InputFiles[0], job.OutputFile)

		// Convert
		case engine.JobOfficeConvert:
			err = p.office.ConvertToPDF(ctx, job.InputFiles[0], job.OutputFile)
		case engine.JobPDFToImages:
			format := engine.ImageFormat(meta["format"])
			if format == "" {
				format = engine.ImageFormatPNG
			}
			dpi := 150
			if d := meta["dpi"]; d != "" {
				fmt.Sscanf(d, "%d", &dpi)
			}
			_, err = p.pdf.ToImages(ctx, job.InputFiles[0], job.OutputFile, format, dpi)
		case engine.JobExtractImages:
			_, err = p.pdf.ExtractImages(ctx, job.InputFiles[0], job.OutputFile)
		case engine.JobPDFToText:
			var text string
			text, err = p.pdf.ToText(ctx, job.InputFiles[0])
			if err == nil {
				err = os.WriteFile(job.OutputFile, []byte(text), 0644)
			}

		// OCR
		case engine.JobOCR:
			if meta["format"] == "txt" {
				var text string
				text, err = p.ocr.ExtractText(ctx, job.InputFiles[0])
				if err == nil {
					err = os.WriteFile(job.OutputFile, []byte(text), 0644)
				}
			} else {
				err = p.ocr.CreateSearchablePDF(ctx, job.InputFiles[0], job.OutputFile)
			}

		// Edit & Layout
		case engine.JobWatermark:
			opts := engine.WatermarkOptions{
				Text:      meta["text"],
				ImagePath: meta["image_path"],
				Color:     meta["color"],
				Pages:     meta["pages"],
			}
			fmt.Sscanf(meta["opacity"], "%f", &opts.Opacity)
			fmt.Sscanf(meta["rotation"], "%f", &opts.Rotation)
			fmt.Sscanf(meta["scale"], "%f", &opts.Scale)
			err = p.pdf.Watermark(ctx, job.InputFiles[0], job.OutputFile, opts)
		case engine.JobPageNumbers:
			opts := engine.PageNumberOptions{
				Format:   meta["format"],
				Position: meta["position"],
				Pages:    meta["pages"],
			}
			fmt.Sscanf(meta["font_size"], "%d", &opts.FontSize)
			fmt.Sscanf(meta["margin"], "%d", &opts.Margin)
			err = p.pdf.AddPageNumbers(ctx, job.InputFiles[0], job.OutputFile, opts)
		case engine.JobCrop:
			opts := engine.CropOptions{Pages: meta["pages"]}
			fmt.Sscanf(meta["left"], "%f", &opts.LeftPt)
			fmt.Sscanf(meta["bottom"], "%f", &opts.BottomPt)
			fmt.Sscanf(meta["right"], "%f", &opts.RightPt)
			fmt.Sscanf(meta["top"], "%f", &opts.TopPt)
			err = p.pdf.Crop(ctx, job.InputFiles[0], job.OutputFile, opts)

		// Security
		case engine.JobProtect:
			userPW := meta["user_password"]
			ownerPW := meta["owner_password"]
			if ownerPW == "" {
				ownerPW = userPW
			}
			opts := engine.ProtectOptions{
				UserPassword:  userPW,
				OwnerPassword: ownerPW,
			}
			err = p.pdf.Protect(ctx, job.InputFiles[0], job.OutputFile, opts)
		case engine.JobUnlock:
			err = p.pdf.Unlock(ctx, job.InputFiles[0], job.OutputFile, meta["password"])

		// Forms & Compare
		case engine.JobFillForm:
			// fields encoded as "key=value,key2=value2" in metadata
			fields := map[string]string{}
			for _, pair := range strings.Split(meta["fields"], ",") {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					fields[kv[0]] = kv[1]
				}
			}
			err = p.pdf.FillForm(ctx, job.InputFiles[0], job.OutputFile, fields)
		case engine.JobCompare:
			if len(job.InputFiles) < 2 {
				err = fmt.Errorf("compare requires 2 input files")
			} else {
				_, err = p.pdf.Compare(ctx, job.InputFiles[0], job.InputFiles[1], job.OutputFile)
			}

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
