import React from 'react';
import { Download } from 'lucide-react';

export interface Job {
  id: string;
  type: string;
  status: string;
  input_files: string[];
  output_file?: string;
  error_msg?: string;
  metadata?: Record<string, string>;
  created_at: string;
  completed_at?: string;
}

interface JobRowProps {
  job: Job;
}

const TOOL_LABELS: Record<string, string> = {
  merge: 'Merge PDF',
  split: 'Split PDF',
  compress: 'Compress PDF',
  ocr: 'OCR Vision',
  office_convert: 'Office to PDF',
  reorder: 'Reorder Pages',
  delete_pages: 'Delete Pages',
  extract_pages: 'Extract Pages',
  rotate: 'Rotate PDF',
  repair: 'Repair PDF',
  pdfa: 'PDF/A Archive',
  pdf_to_images: 'PDF to Images',
  extract_images: 'Extract Images',
  pdf_to_text: 'PDF to Text',
  watermark: 'Watermark',
  page_numbers: 'Page Numbers',
  crop: 'Crop PDF',
  protect: 'Protect PDF',
  unlock: 'Unlock PDF',
  fill_form: 'Fill Form',
  compare: 'Compare PDFs',
};

export const JobRow: React.FC<JobRowProps> = ({ job }) => {
  const label = TOOL_LABELS[job.type] || job.type.replace(/_/g, ' ');

  const getSourceLabel = (): string => {
    if (job.type === 'merge' || job.type === 'compare') {
      return `${job.input_files?.length || 0} file(s)`;
    }
    if (!job.input_files?.length) return 'Unknown';
    const base = job.input_files[0].split('/').pop() || '';
    const parts = base.split('_in');
    return parts.length > 1 ? `Input${parts[parts.length - 1]}` : base;
  };

  const isTextOutput =
    job.type === 'pdf_to_text' ||
    job.type === 'compare' ||
    (job.type === 'ocr' && job.metadata?.format === 'txt');

  const isZipOutput =
    job.type === 'split' ||
    job.type === 'pdf_to_images' ||
    job.type === 'extract_images';

  return (
    <tr>
      <td>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--text-muted)' }}>
          #{job.id.substring(0, 8)}
        </span>
      </td>
      <td>
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontWeight: 600 }}>{label}</span>
          <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{getSourceLabel()}</span>
        </div>
      </td>
      <td>
        <span className={`status-badge ${job.status}`}>
          {job.status === 'processing' && <span className="dot green" />}
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
          <span className="error-text" title={job.error_msg}>
            {job.error_msg || 'Failed'}
          </span>
        )}
      </td>
      <td>
        <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
          {new Date(job.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
        </span>
      </td>
      <td>
        {job.status === 'completed' && (
          <a
            href={`/api/download/${job.id}`}
            download
            className="download-btn"
          >
            <Download size={12} />
            {isTextOutput ? 'Download TXT' : isZipOutput ? 'Download ZIP' : 'Download'}
          </a>
        )}
      </td>
    </tr>
  );
};
