import React, { useState, useEffect, useMemo } from 'react';
import {
  Combine, Scissors, FileArchive, Eye, FileCheck2, ShieldCheck, HelpCircle,
  ArrowRight, Key, Trash2, Replace, RefreshCw, Type, Image as ImageIcon,
  FileSignature, Files, Search, Lock
} from 'lucide-react';
import { ToolCard } from './components/ToolCard';
import { Dropzone } from './components/Dropzone';
import { JobsTable } from './components/JobsTable';
import type { Job } from './components/JobRow';

export type ToolType =
  | 'merge' | 'split' | 'compress' | 'ocr' | 'office_convert'
  | 'reorder' | 'delete_pages' | 'extract_pages' | 'rotate' | 'repair'
  | 'pdfa' | 'pdf_to_images' | 'extract_images' | 'pdf_to_text'
  | 'watermark' | 'page_numbers' | 'crop' | 'protect' | 'unlock'
  | 'fill_form' | 'compare';

interface ToolDef {
  id: ToolType;
  title: string;
  desc: string;
  icon: React.ReactNode;
  section: string;
}

interface SystemTool {
  name: string;
  available: boolean;
}

const TOOLS: ToolDef[] = [
  { id: 'merge', title: 'Merge', desc: 'Combine multiple PDFs', icon: <Combine size={18} />, section: 'Organize' },
  { id: 'split', title: 'Split', desc: 'Split into pages', icon: <Scissors size={18} />, section: 'Organize' },
  { id: 'reorder', title: 'Reorder', desc: 'Change page order', icon: <RefreshCw size={18} />, section: 'Organize' },
  { id: 'delete_pages', title: 'Delete Pages', desc: 'Remove pages', icon: <Trash2 size={18} />, section: 'Organize' },
  { id: 'extract_pages', title: 'Extract Pages', desc: 'Keep specific pages', icon: <Files size={18} />, section: 'Organize' },
  { id: 'rotate', title: 'Rotate', desc: 'Rotate pages', icon: <RefreshCw size={18} />, section: 'Organize' },
  { id: 'compress', title: 'Compress', desc: 'Reduce file size', icon: <FileArchive size={18} />, section: 'Optimize' },
  { id: 'repair', title: 'Repair', desc: 'Fix broken PDFs', icon: <Replace size={18} />, section: 'Optimize' },
  { id: 'pdfa', title: 'PDF/A', desc: 'Long-term archive', icon: <ShieldCheck size={18} />, section: 'Optimize' },
  { id: 'office_convert', title: 'Office to PDF', desc: 'Word, Excel, PPT', icon: <FileCheck2 size={18} />, section: 'Convert' },
  { id: 'pdf_to_images', title: 'PDF to Images', desc: 'Export as JPG/PNG', icon: <ImageIcon size={18} />, section: 'Convert' },
  { id: 'extract_images', title: 'Extract Images', desc: 'Pull embedded images', icon: <ImageIcon size={18} />, section: 'Convert' },
  { id: 'pdf_to_text', title: 'PDF to Text', desc: 'Extract plain text', icon: <Type size={18} />, section: 'Convert' },
  { id: 'ocr', title: 'OCR Vision', desc: 'Make text searchable', icon: <Eye size={18} />, section: 'Edit & Security' },
  { id: 'watermark', title: 'Watermark', desc: 'Stamp text overlay', icon: <Type size={18} />, section: 'Edit & Security' },
  { id: 'page_numbers', title: 'Page Numbers', desc: 'Add numeration', icon: <Type size={18} />, section: 'Edit & Security' },
  { id: 'crop', title: 'Crop', desc: 'Adjust margins', icon: <Scissors size={18} />, section: 'Edit & Security' },
  { id: 'protect', title: 'Protect', desc: 'Password encrypt', icon: <Key size={18} />, section: 'Edit & Security' },
  { id: 'unlock', title: 'Unlock', desc: 'Remove password', icon: <Lock size={18} />, section: 'Edit & Security' },
  { id: 'fill_form', title: 'Fill Form', desc: 'Auto-fill fields', icon: <FileSignature size={18} />, section: 'Edit & Security' },
  { id: 'compare', title: 'Compare', desc: 'Diff two PDFs', icon: <Eye size={18} />, section: 'Edit & Security' },
];

const TOOL_REQUIRES: Partial<Record<ToolType, string>> = {
  compress: 'gs',
  pdfa: 'gs',
  ocr: 'tesseract',
  office_convert: 'libreoffice',
  pdf_to_text: 'pdftotext',
  pdf_to_images: 'pdftoppm',
};

export const App: React.FC = () => {
  const [activeTool, setActiveTool] = useState<ToolType>('merge');
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [isUploading, setIsUploading] = useState(false);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [toast, setToast] = useState<string | null>(null);
  const [options, setOptions] = useState<Record<string, string>>({});
  const [toolSearch, setToolSearch] = useState('');
  const [systemTools, setSystemTools] = useState<SystemTool[]>([]);

  useEffect(() => {
    fetch('/api/system/status')
      .then((r) => r.json())
      .then((data) => setSystemTools(data.tools || []))
      .catch(() => {});
  }, []);

  useEffect(() => {
    const eventSource = new EventSource('/api/jobs/stream');
    eventSource.onmessage = (event) => {
      try {
        const parsed = JSON.parse(event.data);
        if (Array.isArray(parsed)) setJobs(parsed);
      } catch {
        /* ignore malformed SSE */
      }
    };
    return () => eventSource.close();
  }, []);

  const showToast = (msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 3000);
  };

  const isToolAvailable = (tool: ToolType): boolean => {
    const required = TOOL_REQUIRES[tool];
    if (!required) return true;
    const found = systemTools.find((t) => t.name === required);
    return found?.available ?? true;
  };

  const handleToolChange = (tool: ToolType) => {
    setActiveTool(tool);
    setSelectedFiles([]);
    setOptions({});
  };

  const handleProcess = async () => {
    if (selectedFiles.length === 0) return;
    if (!isToolAvailable(activeTool)) {
      showToast(`This tool requires ${TOOL_REQUIRES[activeTool]} — not installed on server`);
      return;
    }

    setIsUploading(true);
    const formData = new FormData();
    formData.append('job_type', activeTool);
    selectedFiles.forEach((file) => formData.append('files', file));
    Object.entries(options).forEach(([key, value]) => formData.append(key, value));

    try {
      const response = await fetch('/api/jobs/submit', { method: 'POST', body: formData });
      if (!response.ok) {
        const err = await response.json().catch(() => ({ error: response.statusText }));
        throw new Error(err.error || response.statusText);
      }
      showToast('Job queued — processing locally');
      setSelectedFiles([]);
      setOptions({});
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Upload failed';
      showToast(msg);
    } finally {
      setIsUploading(false);
    }
  };

  const filteredTools = useMemo(() => {
    const q = toolSearch.toLowerCase().trim();
    if (!q) return TOOLS;
    return TOOLS.filter(
      (t) => t.title.toLowerCase().includes(q) || t.desc.toLowerCase().includes(q)
    );
  }, [toolSearch]);

  const sections = useMemo(() => {
    const map = new Map<string, ToolDef[]>();
    for (const t of filteredTools) {
      if (!map.has(t.section)) map.set(t.section, []);
      map.get(t.section)!.push(t);
    }
    return map;
  }, [filteredTools]);

  const getToolMetadata = () => {
    switch (activeTool) {
      case 'merge': return { title: 'Merge PDF', desc: 'Combine multiple PDF files into a single document.', accept: '.pdf', multiple: true };
      case 'split': return { title: 'Split PDF', desc: 'Split a PDF into individual page files.', accept: '.pdf', multiple: false };
      case 'compress': return { title: 'Compress PDF', desc: 'Reduce file size while preserving readability.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'ocr': return { title: 'OCR PDF', desc: 'Extract text or create a searchable PDF from scans.', accept: '.pdf,.png,.jpg,.jpeg,.tiff', multiple: false, hasOptions: true };
      case 'office_convert': return { title: 'Office to PDF', desc: 'Convert Word, Excel, or PowerPoint documents.', accept: '.docx,.xlsx,.pptx,.odt,.ods,.odp,.txt', multiple: false };
      case 'reorder': return { title: 'Reorder Pages', desc: 'Rearrange pages (e.g. 3,1,2 for a 3-page doc).', accept: '.pdf', multiple: false, hasOptions: true };
      case 'delete_pages': return { title: 'Delete Pages', desc: 'Remove specific pages from your PDF.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'extract_pages': return { title: 'Extract Pages', desc: 'Create a new PDF from selected pages.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'rotate': return { title: 'Rotate PDF', desc: 'Rotate pages by 90°, 180°, or 270°.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'repair': return { title: 'Repair PDF', desc: 'Attempt to fix corrupt or broken PDF files.', accept: '.pdf', multiple: false };
      case 'pdfa': return { title: 'PDF/A Archive', desc: 'Convert to long-term archival PDF/A format.', accept: '.pdf', multiple: false };
      case 'pdf_to_images': return { title: 'PDF to Images', desc: 'Export each page as a JPG or PNG image.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'extract_images': return { title: 'Extract Images', desc: 'Pull embedded images out of a PDF.', accept: '.pdf', multiple: false };
      case 'pdf_to_text': return { title: 'PDF to Text', desc: 'Extract raw text content from a PDF.', accept: '.pdf', multiple: false };
      case 'watermark': return { title: 'Add Watermark', desc: 'Stamp text across your document pages.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'page_numbers': return { title: 'Page Numbers', desc: 'Add page numbers to your document.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'crop': return { title: 'Crop PDF', desc: 'Crop page margins in points (72 pt = 1 inch).', accept: '.pdf', multiple: false, hasOptions: true };
      case 'protect': return { title: 'Protect PDF', desc: 'Encrypt and password-protect your PDF.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'unlock': return { title: 'Unlock PDF', desc: 'Remove password protection from a PDF.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'fill_form': return { title: 'Fill Form', desc: 'Auto-fill interactive PDF form fields.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'compare': return { title: 'Compare PDFs', desc: 'Compare two PDFs and highlight text differences.', accept: '.pdf', multiple: true };
      default: return { title: '', desc: '', accept: '.pdf', multiple: false };
    }
  };

  const metadata = getToolMetadata();
  const toolReady = isToolAvailable(activeTool);
  const missingDep = TOOL_REQUIRES[activeTool];

  const setOpt = (key: string, value: string) =>
    setOptions((prev) => ({ ...prev, [key]: value }));

  return (
    <div className="app-container">
      {toast && (
        <div className="toast">
          <ShieldCheck size={16} style={{ color: 'var(--green-600)' }} />
          <span>{toast}</span>
        </div>
      )}

      <header className="app-header">
        <div className="brand">
          <span className="brand-logo">Sovereign PDF</span>
          <span className="brand-badge">local</span>
        </div>
        <div className="system-status">
          <div className={`status-indicator ${toolReady ? '' : 'warn'}`}>
            <span className={`dot ${toolReady ? 'green' : 'amber'}`} />
            <span>{toolReady ? 'Engine Ready' : `${missingDep} not installed`}</span>
          </div>
          <div className="status-indicator">
            <ShieldCheck size={14} style={{ color: 'var(--green-600)' }} />
            <span>100% Private — files never leave this machine</span>
          </div>
        </div>
      </header>

      <main className="app-workspace">
        <aside className="tools-sidebar">
          <div style={{ position: 'relative' }}>
            <Search size={14} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
            <input
              className="tool-search"
              placeholder="Search tools…"
              value={toolSearch}
              onChange={(e) => setToolSearch(e.target.value)}
              style={{ paddingLeft: 34 }}
            />
          </div>

          {Array.from(sections.entries()).map(([section, tools]) => (
            <React.Fragment key={section}>
              <div className="tools-section-label">{section}</div>
              {tools.map((t) => (
                <ToolCard
                  key={t.id}
                  title={t.title}
                  desc={t.desc}
                  icon={t.icon}
                  active={activeTool === t.id}
                  onClick={() => handleToolChange(t.id)}
                />
              ))}
            </React.Fragment>
          ))}

          <div className="privacy-note">
            <strong><HelpCircle size={14} /> Local Execution</strong>
            All processing runs on your server. No cloud uploads, no third-party APIs.
          </div>
        </aside>

        <section style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
          <div className="glass-panel workspace-panel">
            <div className="panel-header">
              <h2 className="panel-title">{metadata.title}</h2>
              <p className="panel-desc">{metadata.desc}</p>
            </div>

            <Dropzone
              accept={metadata.accept}
              multiple={metadata.multiple}
              onFilesSelected={setSelectedFiles}
              onFileRemoved={(idx) => setSelectedFiles((f) => f.filter((_, i) => i !== idx))}
              selectedFiles={selectedFiles}
            />

            {(metadata as { hasOptions?: boolean }).hasOptions && selectedFiles.length > 0 && (
              <div className="options-panel">
                <h4>Configuration</h4>

                {activeTool === 'compress' && (
                  <select className="form-select" onChange={(e) => setOpt('profile', e.target.value)} defaultValue="medium">
                    <option value="medium">Medium — Ebook quality</option>
                    <option value="low">Low — Screen quality</option>
                    <option value="high">High — Print quality</option>
                  </select>
                )}

                {activeTool === 'ocr' && (
                  <select className="form-select" onChange={(e) => setOpt('format', e.target.value)} defaultValue="pdf">
                    <option value="pdf">Searchable PDF</option>
                    <option value="txt">Plain text extraction</option>
                  </select>
                )}

                {activeTool === 'pdf_to_images' && (
                  <>
                    <select className="form-select" onChange={(e) => setOpt('format', e.target.value)} defaultValue="png">
                      <option value="png">PNG</option>
                      <option value="jpg">JPEG</option>
                    </select>
                    <input className="form-input" type="number" placeholder="DPI (default 150)" onChange={(e) => setOpt('dpi', e.target.value)} />
                  </>
                )}

                {activeTool === 'reorder' && (
                  <input className="form-input" type="text" placeholder="Page order, e.g. 3,1,2" onChange={(e) => setOpt('page_seq', e.target.value)} />
                )}

                {['delete_pages', 'extract_pages'].includes(activeTool) && (
                  <input className="form-input" type="text" placeholder="Pages, e.g. 1,3,4-6" onChange={(e) => setOpt('pages', e.target.value)} />
                )}

                {activeTool === 'rotate' && (
                  <>
                    <select className="form-select" onChange={(e) => setOpt('degrees', e.target.value)} defaultValue="90">
                      <option value="90">90° clockwise</option>
                      <option value="180">180°</option>
                      <option value="270">270° clockwise</option>
                    </select>
                    <input className="form-input" type="text" placeholder="Pages (optional, e.g. 1,3)" onChange={(e) => setOpt('pages', e.target.value)} />
                  </>
                )}

                {activeTool === 'watermark' && (
                  <>
                    <input className="form-input" type="text" placeholder="Watermark text" onChange={(e) => setOpt('text', e.target.value)} />
                    <input className="form-input" type="text" placeholder="Color, e.g. #16a34a or 0.1 0.6 0.3" onChange={(e) => setOpt('color', e.target.value)} />
                  </>
                )}

                {activeTool === 'page_numbers' && (
                  <>
                    <input className="form-input" type="text" placeholder="Format, e.g. Page %d" onChange={(e) => setOpt('format', e.target.value)} />
                    <select className="form-select" onChange={(e) => setOpt('position', e.target.value)} defaultValue="bc">
                      <option value="bc">Bottom center</option>
                      <option value="bl">Bottom left</option>
                      <option value="br">Bottom right</option>
                      <option value="tc">Top center</option>
                      <option value="tr">Top right</option>
                    </select>
                  </>
                )}

                {activeTool === 'crop' && (
                  <div className="form-grid-2">
                    <input className="form-input" type="number" placeholder="Left (pt)" onChange={(e) => setOpt('left', e.target.value)} />
                    <input className="form-input" type="number" placeholder="Right (pt)" onChange={(e) => setOpt('right', e.target.value)} />
                    <input className="form-input" type="number" placeholder="Top (pt)" onChange={(e) => setOpt('top', e.target.value)} />
                    <input className="form-input" type="number" placeholder="Bottom (pt)" onChange={(e) => setOpt('bottom', e.target.value)} />
                  </div>
                )}

                {activeTool === 'protect' && (
                  <input className="form-input" type="password" placeholder="User password" onChange={(e) => setOpt('user_password', e.target.value)} />
                )}

                {activeTool === 'unlock' && (
                  <input className="form-input" type="password" placeholder="Current password" onChange={(e) => setOpt('password', e.target.value)} />
                )}

                {activeTool === 'fill_form' && (
                  <input className="form-input" type="text" placeholder="Fields, e.g. name=John,email=test@example.com" onChange={(e) => setOpt('fields', e.target.value)} />
                )}
              </div>
            )}

            <div className="panel-actions">
              <button
                className="btn-primary"
                onClick={handleProcess}
                disabled={selectedFiles.length === 0 || isUploading || !toolReady}
              >
                {isUploading ? 'Processing…' : (
                  <>Run {metadata.title} <ArrowRight size={16} /></>
                )}
              </button>
            </div>
          </div>

          <JobsTable jobs={jobs} loading={false} />
        </section>
      </main>

      <footer className="app-footer">
        © 2026 Sovereign PDF — Your documents, your infrastructure, your privacy.
      </footer>
    </div>
  );
};
