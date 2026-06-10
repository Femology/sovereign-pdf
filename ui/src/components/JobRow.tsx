import React, { useState } from 'react';
import { Download, FileText, Check, Copy, Eye, EyeOff } from 'lucide-react';

export interface Job {
  id: string;
  type: string;
  status: string;
  input_paths: string[];
  output_path?: string;
  split_paths?: string[];
  ocr_text?: string;
  error?: string;
  created_at: string;
  updated_at: string;
  options?: Record<string, any>;
}

interface JobRowProps {
  job: Job;
  onCopyText: (text: string) => void;
}

export const JobRow: React.FC<JobRowProps> = ({ job, onCopyText }) => {
  const [showOcrText, setShowOcrText] = useState(false);
  const [isCopied, setIsCopied] = useState(false);

  const formatType = (type: string): string => {
    switch (type) {
      case 'merge': return 'PDF Merge';
      case 'split': return 'PDF Split';
      case 'compress': return 'PDF Compress';
      case 'ocr': return 'OCR Vision';
      case 'convert': return 'Office to PDF';
      default: return type.toUpperCase();
    }
  };

  const getSourceFileName = (paths: string[]): string => {
    if (!paths || paths.length === 0) return 'Unknown File';
    const path = paths[0];
    const base = path.split('/').pop() || '';
    // Format is {jobID}_{jobType}_in_{ext}
    const parts = base.split('_in');
    if (parts.length > 1) {
       return 'Input' + parts[parts.length-1];
    }
    return base;
  };

  const handleCopy = () => {
    if (job.ocr_text) {
      onCopyText(job.ocr_text);
      setIsCopied(true);
      setTimeout(() => setIsCopied(false), 2000);
    }
  };

  const isOCRText = job.type === 'ocr' && (!job.options || job.options.format === 'txt');

  return (
    <>
      <tr>
        <td>
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--text-muted)' }}>
            #{job.id.substring(0, 8)}
          </span>
        </td>
        <td>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span style={{ fontWeight: 600 }}>{formatType(job.type)}</span>
            <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
              {job.type === 'merge' ? `${job.input_paths.length} PDFs` : getSourceFileName(job.input_paths)}
            </span>
          </div>
        </td>
        <td>
          <span className={`status-badge ${job.status}`}>
            {job.status === 'processing' && <span className="dot green"></span>}
            {job.status}
          </span>
        </td>
        <td>
          {job.status === 'processing' && (
            <div className="progress-bar-container">
              <div className="progress-bar-fill animated" />
            </div>
          )}
          {job.status === 'pending' && (
            <div className="progress-bar-container">
              <div className="progress-bar-fill" style={{ width: '0%' }} />
            </div>
          )}
          {job.status === 'completed' && (
            <div className="progress-bar-container">
              <div className="progress-bar-fill" style={{ width: '100%' }} />
            </div>
          )}
          {job.status === 'failed' && (
            <span style={{ color: 'var(--accent-error)', fontSize: '11px' }} title={job.error}>
              Error
            </span>
          )}
        </td>
        <td>
          <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
            {new Date(job.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
          </span>
        </td>
        <td>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            {job.status === 'completed' && (
              <>
                {isOCRText ? (
                  <button
                    type="button"
                    className="browse-btn"
                    style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '6px 12px', fontSize: '11px' }}
                    onClick={() => setShowOcrText(!showOcrText)}
                  >
                    {showOcrText ? <EyeOff size={12} /> : <Eye size={12} />}
                    {showOcrText ? 'Hide Text' : 'View Text'}
                  </button>
                ) : (
                  <a
                    href={`/api/download/${job.id}`}
                    download
                    className="download-btn"
                    style={{ textDecoration: 'none' }}
                  >
                    <Download size={12} />
                    Download
                  </a>
                )}
              </>
            )}
            {job.status === 'failed' && (
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'block', maxWidth: '120px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={job.error}>
                {job.error}
              </span>
            )}
          </div>
        </td>
      </tr>
      
      {showOcrText && job.ocr_text && (
        <tr>
          <td colSpan={6} style={{ padding: '0 20px 20px 20px' }}>
            <div className="ocr-result-container">
              <div className="ocr-result-header">
                <span style={{ fontWeight: 600, display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <FileText size={14} className="text-teal" /> Extracted Text Layer
                </span>
                <button
                  type="button"
                  className="browse-btn"
                  style={{ padding: '4px 10px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
                  onClick={handleCopy}
                >
                  {isCopied ? <Check size={12} className="text-teal" /> : <Copy size={12} />}
                  {isCopied ? 'Copied' : 'Copy Text'}
                </button>
              </div>
              <textarea
                className="ocr-textarea"
                readOnly
                value={job.ocr_text}
              />
            </div>
          </td>
        </tr>
      )}
    </>
  );
};
