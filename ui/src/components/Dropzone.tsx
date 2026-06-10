import React, { useRef, useState } from 'react';
import { UploadCloud, File, X } from 'lucide-react';

interface DropzoneProps {
  accept: string;
  multiple: boolean;
  selectedFiles: File[];
  onFilesSelected: (files: File[]) => void;
  onFileRemoved: (index: number) => void;
}

export const Dropzone: React.FC<DropzoneProps> = ({
  accept,
  multiple,
  selectedFiles,
  onFilesSelected,
  onFileRemoved,
}) => {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [dragActive, setDragActive] = useState(false);

  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === "dragenter" || e.type === "dragover") {
      setDragActive(true);
    } else if (e.type === "dragleave") {
      setDragActive(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);

    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      const filesArray = Array.from(e.dataTransfer.files);
      // Optional filter based on accept
      const acceptedFiles = filterFiles(filesArray, accept);
      if (acceptedFiles.length > 0) {
        onFilesSelected(multiple ? [...selectedFiles, ...acceptedFiles] : [acceptedFiles[0]]);
      }
    }
  };

  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const filesArray = Array.from(e.target.files);
      onFilesSelected(multiple ? [...selectedFiles, ...filesArray] : [filesArray[0]]);
    }
  };

  const onButtonClick = () => {
    fileInputRef.current?.click();
  };

  // Helper to filter files by extension/mime (simple helper)
  const filterFiles = (files: File[], acceptTypes: string): File[] => {
    if (acceptTypes === '*') return files;
    const extensions = acceptTypes.split(',').map(ext => ext.trim().toLowerCase());
    return files.filter(file => {
      const fileName = file.name.toLowerCase();
      return extensions.some(ext => fileName.endsWith(ext));
    });
  };

  const formatSize = (bytes: number): string => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div
        className={`dropzone-container ${dragActive ? 'drag-active' : ''}`}
        onDragEnter={handleDrag}
        onDragOver={handleDrag}
        onDragLeave={handleDrag}
        onDrop={handleDrop}
        onClick={onButtonClick}
      >
        <input
          ref={fileInputRef}
          type="file"
          multiple={multiple}
          accept={accept}
          style={{ display: 'none' }}
          onChange={handleFileInputChange}
        />
        <UploadCloud size={40} className="dropzone-icon" />
        <h3 className="dropzone-title">Drag & drop files here</h3>
        <p className="dropzone-hint">
          {multiple ? 'Support for single or multiple files' : 'Supports single file upload'} • Allowed: {accept.replace(/\./g, ' ').toUpperCase()}
        </p>
        <button type="button" className="browse-btn" onClick={(e) => { e.stopPropagation(); onButtonClick(); }}>
          Browse Files
        </button>
      </div>

      {selectedFiles.length > 0 && (
        <div className="files-queue">
          {selectedFiles.map((file, idx) => (
            <div key={idx} className="file-item">
              <div className="file-meta">
                <File size={16} className="text-secondary" />
                <span className="file-name" title={file.name}>
                  {file.name}
                </span>
                <span className="file-size">{formatSize(file.size)}</span>
              </div>
              <button
                type="button"
                className="remove-file-btn"
                onClick={() => onFileRemoved(idx)}
                aria-label="Remove file"
              >
                <X size={16} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
