import React, { useState, useEffect } from 'react';
import { 
  Combine, Scissors, FileArchive, Eye, FileCheck2, ShieldCheck, HelpCircle,
  ArrowRight, Key, Trash2, Replace, RefreshCw, Type, Image as ImageIcon, FileSignature, Files
} from 'lucide-react';
import { ToolCard } from './components/ToolCard';
import { Dropzone } from './components/Dropzone';
import { JobsTable } from './components/JobsTable';
import type { Job } from './components/JobRow';

export type ToolType = 'merge' | 'split' | 'compress' | 'ocr' | 'office_convert' | 'reorder' | 'delete_pages' | 'extract_pages' | 'rotate' | 'repair' | 'pdfa' | 'pdf_to_images' | 'extract_images' | 'pdf_to_text' | 'watermark' | 'page_numbers' | 'crop' | 'protect' | 'unlock' | 'fill_form' | 'compare';

export const App: React.FC = () => {
  const [activeTool, setActiveTool] = useState<ToolType>('merge');
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [isUploading, setIsUploading] = useState(false);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [toast, setToast] = useState<string | null>(null);
  
  // Options state
  const [options, setOptions] = useState<Record<string, string>>({});

  useEffect(() => {
    const eventSource = new EventSource('/api/jobs/stream');

    eventSource.onmessage = (event) => {
      try {
        const parsedJobs = JSON.parse(event.data);
        if (Array.isArray(parsedJobs)) {
          setJobs(parsedJobs);
        }
      } catch (err) {
        console.error('Failed to parse SSE payload', err);
      }
    };

    eventSource.onerror = (error) => {
      console.error('SSE Error:', error);
    };

    return () => {
      eventSource.close();
    };
  }, []);

  const showToast = (msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 3000);
  };

  const handleToolChange = (tool: ToolType) => {
    setActiveTool(tool);
    setSelectedFiles([]);
    setOptions({});
  };

  const handleProcess = async () => {
    if (selectedFiles.length === 0) return;
    
    setIsUploading(true);
    const formData = new FormData();
    formData.append('job_type', activeTool);
    selectedFiles.forEach((file) => formData.append('files', file));
    
    // Add options to formData
    Object.entries(options).forEach(([key, value]) => {
      formData.append(key, value);
    });

    try {
      const response = await fetch('/api/jobs/submit', {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const err = await response.text();
        throw new Error(err || response.statusText);
      }

      showToast(`Job queued successfully`);
      setSelectedFiles([]);
      setOptions({});
    } catch (err: any) {
      alert(`Upload Failed: ${err.message}`);
    } finally {
      setIsUploading(false);
    }
  };

  const getToolMetadata = () => {
    switch (activeTool) {
      case 'merge': return { title: 'Merge PDF', desc: 'Combine multiple PDFs into one.', accept: '.pdf', multiple: true };
      case 'split': return { title: 'Split PDF', desc: 'Extract pages or split into multiple files.', accept: '.pdf', multiple: false };
      case 'compress': return { title: 'Compress PDF', desc: 'Reduce file size while preserving quality.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'ocr': return { title: 'OCR PDF', desc: 'Make scanned PDFs searchable.', accept: '.pdf,.png,.jpg', multiple: false, hasOptions: true };
      case 'office_convert': return { title: 'Office to PDF', desc: 'Convert Word, Excel, PPT to PDF.', accept: '.docx,.xlsx,.pptx,.txt', multiple: false };
      case 'reorder': return { title: 'Reorder Pages', desc: 'Change page order (e.g. 3,1,2).', accept: '.pdf', multiple: false, hasOptions: true };
      case 'delete_pages': return { title: 'Delete Pages', desc: 'Remove specific pages from PDF.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'extract_pages': return { title: 'Extract Pages', desc: 'Extract specific pages into a new PDF.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'rotate': return { title: 'Rotate PDF', desc: 'Rotate pages by degrees (90, 180, 270).', accept: '.pdf', multiple: false, hasOptions: true };
      case 'repair': return { title: 'Repair PDF', desc: 'Fix corrupt or broken PDF files.', accept: '.pdf', multiple: false };
      case 'pdfa': return { title: 'PDF/A Archive', desc: 'Convert to long-term PDF/A format.', accept: '.pdf', multiple: false };
      case 'pdf_to_images': return { title: 'PDF to Images', desc: 'Convert PDF pages to JPG/PNG.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'extract_images': return { title: 'Extract Images', desc: 'Extract embedded images from PDF.', accept: '.pdf', multiple: false };
      case 'pdf_to_text': return { title: 'PDF to Text', desc: 'Extract raw text from PDF.', accept: '.pdf', multiple: false };
      case 'watermark': return { title: 'Add Watermark', desc: 'Stamp text or images onto PDF.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'page_numbers': return { title: 'Page Numbers', desc: 'Add page numbers to document.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'crop': return { title: 'Crop PDF', desc: 'Crop page margins by points.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'protect': return { title: 'Protect PDF', desc: 'Encrypt and password protect.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'unlock': return { title: 'Unlock PDF', desc: 'Remove password protection.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'fill_form': return { title: 'Fill Form', desc: 'Fill interactive PDF form fields.', accept: '.pdf', multiple: false, hasOptions: true };
      case 'compare': return { title: 'Compare PDFs', desc: 'Compare two PDFs for text differences.', accept: '.pdf', multiple: true };
      default: return { title: '', desc: '', accept: '', multiple: false };
    }
  };

  const metadata = getToolMetadata();

  return (
    <div className="app-container">
      {toast && (
        <div className="toast">
          <ShieldCheck size={16} style={{ color: 'var(--accent-teal)' }} />
          <span>{toast}</span>
        </div>
      )}

      <header className="app-header">
        <div className="brand">
          <span className="brand-logo">Sovereign PDF</span>
          <span className="brand-badge">pro</span>
        </div>
        <div className="system-status">
          <div className="status-indicator">
            <span className="dot green"></span>
            <span>Local Engine Active</span>
          </div>
        </div>
      </header>

      <main className="app-workspace">
        <aside className="tools-list" style={{ overflowY: 'auto', maxHeight: 'calc(100vh - 100px)' }}>
          <h4 style={{ color: 'var(--text-muted)', fontSize: '11px', textTransform: 'uppercase', marginBottom: '10px' }}>Organize</h4>
          <ToolCard title="Merge" desc="Combine multiple files" icon={<Combine size={18}/>} active={activeTool==='merge'} onClick={()=>handleToolChange('merge')}/>
          <ToolCard title="Split" desc="Extract pages" icon={<Scissors size={18}/>} active={activeTool==='split'} onClick={()=>handleToolChange('split')}/>
          <ToolCard title="Reorder" desc="Change page order" icon={<RefreshCw size={18}/>} active={activeTool==='reorder'} onClick={()=>handleToolChange('reorder')}/>
          <ToolCard title="Delete Pages" desc="Remove pages" icon={<Trash2 size={18}/>} active={activeTool==='delete_pages'} onClick={()=>handleToolChange('delete_pages')}/>
          <ToolCard title="Extract Pages" desc="Keep specific pages" icon={<Files size={18}/>} active={activeTool==='extract_pages'} onClick={()=>handleToolChange('extract_pages')}/>
          <ToolCard title="Rotate" desc="Rotate pages" icon={<RefreshCw size={18}/>} active={activeTool==='rotate'} onClick={()=>handleToolChange('rotate')}/>
          
          <h4 style={{ color: 'var(--text-muted)', fontSize: '11px', textTransform: 'uppercase', margin: '15px 0 10px 0' }}>Optimize & Convert</h4>
          <ToolCard title="Compress" desc="Reduce file size" icon={<FileArchive size={18}/>} active={activeTool==='compress'} onClick={()=>handleToolChange('compress')}/>
          <ToolCard title="Repair" desc="Fix broken PDFs" icon={<Replace size={18}/>} active={activeTool==='repair'} onClick={()=>handleToolChange('repair')}/>
          <ToolCard title="PDF/A" desc="Long-term archiving" icon={<ShieldCheck size={18}/>} active={activeTool==='pdfa'} onClick={()=>handleToolChange('pdfa')}/>
          <ToolCard title="Office to PDF" desc="Word, Excel, PPT" icon={<FileCheck2 size={18}/>} active={activeTool==='office_convert'} onClick={()=>handleToolChange('office_convert')}/>
          <ToolCard title="PDF to Images" desc="Convert to JPG/PNG" icon={<ImageIcon size={18}/>} active={activeTool==='pdf_to_images'} onClick={()=>handleToolChange('pdf_to_images')}/>
          <ToolCard title="Extract Images" desc="Pull out images" icon={<ImageIcon size={18}/>} active={activeTool==='extract_images'} onClick={()=>handleToolChange('extract_images')}/>
          <ToolCard title="PDF to Text" desc="Extract raw text" icon={<Type size={18}/>} active={activeTool==='pdf_to_text'} onClick={()=>handleToolChange('pdf_to_text')}/>

          <h4 style={{ color: 'var(--text-muted)', fontSize: '11px', textTransform: 'uppercase', margin: '15px 0 10px 0' }}>Edit, Security & Data</h4>
          <ToolCard title="OCR Vision" desc="Make text searchable" icon={<Eye size={18}/>} active={activeTool==='ocr'} onClick={()=>handleToolChange('ocr')}/>
          <ToolCard title="Watermark" desc="Stamp text/image" icon={<Type size={18}/>} active={activeTool==='watermark'} onClick={()=>handleToolChange('watermark')}/>
          <ToolCard title="Page Numbers" desc="Add numeration" icon={<Type size={18}/>} active={activeTool==='page_numbers'} onClick={()=>handleToolChange('page_numbers')}/>
          <ToolCard title="Crop" desc="Adjust margins" icon={<Scissors size={18}/>} active={activeTool==='crop'} onClick={()=>handleToolChange('crop')}/>
          <ToolCard title="Protect" desc="Add password" icon={<Key size={18}/>} active={activeTool==='protect'} onClick={()=>handleToolChange('protect')}/>
          <ToolCard title="Unlock" desc="Remove password" icon={<Key size={18}/>} active={activeTool==='unlock'} onClick={()=>handleToolChange('unlock')}/>
          <ToolCard title="Fill Form" desc="Automate fields" icon={<FileSignature size={18}/>} active={activeTool==='fill_form'} onClick={()=>handleToolChange('fill_form')}/>
          <ToolCard title="Compare" desc="Text differences" icon={<Eye size={18}/>} active={activeTool==='compare'} onClick={()=>handleToolChange('compare')}/>

          <div className="glass-panel" style={{ padding: '20px', marginTop: '12px', fontSize: '11px', color: 'var(--text-muted)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '8px' }}>
              <HelpCircle size={14} /> Local Execution
            </div>
            100% private. Files never leave your local machine.
          </div>
        </aside>

        <section style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <div className="glass-panel workspace-panel">
            <div className="panel-header">
              <h2 className="panel-title">{metadata.title}</h2>
              <p className="panel-desc">{metadata.desc}</p>
            </div>
            
            <Dropzone
              accept={metadata.accept}
              multiple={metadata.multiple}
              onFilesSelected={setSelectedFiles}
              onFileRemoved={(idx) => setSelectedFiles(selectedFiles.filter((_, i) => i !== idx))}
              selectedFiles={selectedFiles}
            />

            {(metadata as any).hasOptions && selectedFiles.length > 0 && (
              <div className="options-panel" style={{ marginTop: '20px', padding: '15px', background: 'rgba(255,255,255,0.02)', borderRadius: '8px', border: '1px solid rgba(255,255,255,0.05)' }}>
                <h4 style={{ marginBottom: '10px', fontSize: '13px', color: 'var(--text-primary)' }}>Job Configuration</h4>
                
                {activeTool === 'compress' && (
                  <select onChange={(e) => setOptions({...options, profile: e.target.value})} style={inputStyle}>
                    <option value="medium">Medium (Ebook)</option>
                    <option value="low">Low (Screen)</option>
                    <option value="high">High (Printer)</option>
                  </select>
                )}

                {activeTool === 'ocr' && (
                  <select onChange={(e) => setOptions({...options, format: e.target.value})} style={inputStyle}>
                    <option value="pdf">Searchable PDF</option>
                    <option value="txt">Extracted Text</option>
                  </select>
                )}

                {activeTool === 'pdf_to_images' && (
                  <>
                    <select onChange={(e) => setOptions({...options, format: e.target.value})} style={inputStyle}>
                      <option value="png">PNG</option>
                      <option value="jpg">JPEG</option>
                    </select>
                    <input type="number" placeholder="DPI (e.g. 150)" onChange={(e) => setOptions({...options, dpi: e.target.value})} style={inputStyle} />
                  </>
                )}

                {['reorder', 'delete_pages', 'extract_pages'].includes(activeTool) && (
                  <input type="text" placeholder="Pages (e.g. 1,3,4-6)" onChange={(e) => setOptions({...options, pages: e.target.value})} style={inputStyle} />
                )}

                {activeTool === 'rotate' && (
                  <>
                    <input type="number" placeholder="Degrees (90, 180, 270)" onChange={(e) => setOptions({...options, degrees: e.target.value})} style={inputStyle} />
                    <input type="text" placeholder="Pages (optional)" onChange={(e) => setOptions({...options, pages: e.target.value})} style={inputStyle} />
                  </>
                )}

                {activeTool === 'watermark' && (
                  <>
                    <input type="text" placeholder="Text to stamp" onChange={(e) => setOptions({...options, text: e.target.value})} style={inputStyle} />
                    <input type="text" placeholder="Color (e.g. #FF0000)" onChange={(e) => setOptions({...options, color: e.target.value})} style={inputStyle} />
                  </>
                )}

                {activeTool === 'page_numbers' && (
                  <>
                    <input type="text" placeholder="Format (e.g. Page %d)" onChange={(e) => setOptions({...options, format: e.target.value})} style={inputStyle} />
                    <select onChange={(e) => setOptions({...options, position: e.target.value})} style={inputStyle}>
                      <option value="bc">Bottom Center</option>
                      <option value="tr">Top Right</option>
                      <option value="bl">Bottom Left</option>
                    </select>
                  </>
                )}

                {activeTool === 'crop' && (
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
                    <input type="number" placeholder="Left pt" onChange={(e) => setOptions({...options, left: e.target.value})} style={inputStyle} />
                    <input type="number" placeholder="Right pt" onChange={(e) => setOptions({...options, right: e.target.value})} style={inputStyle} />
                    <input type="number" placeholder="Top pt" onChange={(e) => setOptions({...options, top: e.target.value})} style={inputStyle} />
                    <input type="number" placeholder="Bottom pt" onChange={(e) => setOptions({...options, bottom: e.target.value})} style={inputStyle} />
                  </div>
                )}

                {activeTool === 'protect' && (
                  <input type="password" placeholder="User Password" onChange={(e) => setOptions({...options, user_password: e.target.value})} style={inputStyle} />
                )}

                {activeTool === 'unlock' && (
                  <input type="password" placeholder="Current Password" onChange={(e) => setOptions({...options, password: e.target.value})} style={inputStyle} />
                )}
                
                {activeTool === 'fill_form' && (
                  <input type="text" placeholder="Fields (e.g. name=John,age=30)" onChange={(e) => setOptions({...options, fields: e.target.value})} style={inputStyle} />
                )}

              </div>
            )}

            <div className="panel-actions">
              <button 
                className="btn-primary" 
                onClick={handleProcess} 
                disabled={selectedFiles.length === 0 || isUploading}
                style={{ width: '100%', padding: '16px' }}
              >
                {isUploading ? (
                  <span>Processing...</span>
                ) : (
                  <>
                    Run Local Engine <ArrowRight size={16} />
                  </>
                )}
              </button>
            </div>
          </div>

          <JobsTable jobs={jobs} loading={false} onCopyText={async (text) => { await navigator.clipboard.writeText(text); showToast('Copied to clipboard'); }} />
        </section>
      </main>

      <footer className="app-footer">
        © 2026 Sovereign PDF — Your Security, Your Documents, Your Infrastructure.<br/>
        <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Powered by pdfcpu & Tesseract Vision Subsystems</span>
      </footer>
    </div>
  );
};

const inputStyle = {
  width: '100%',
  padding: '10px',
  marginBottom: '10px',
  background: 'rgba(0,0,0,0.2)',
  border: '1px solid rgba(255,255,255,0.1)',
  color: 'white',
  borderRadius: '4px',
  fontFamily: 'inherit'
};
