import React from 'react';
import { ShieldCheck, Loader2 } from 'lucide-react';
import { JobRow, Job } from './JobRow';

interface JobsTableProps {
  jobs: Job[];
  loading: boolean;
  onCopyText: (text: string) => void;
}

export const JobsTable: React.FC<JobsTableProps> = ({ jobs, loading, onCopyText }) => {
  return (
    <div className="glass-panel" style={{ overflow: 'hidden', padding: '24px 0' }}>
      <div style={{ padding: '0 24px 16px 24px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h3 className="section-title">
          <ShieldCheck size={20} className="text-teal" style={{ color: 'var(--accent-teal)' }} />
          Private Task Processing History
        </h3>
        {loading && <Loader2 size={16} className="animated" style={{ animation: 'spin 1.5s infinite linear' }} />}
      </div>
      
      {jobs.length === 0 ? (
        <div className="empty-state">
          <ShieldCheck size={36} className="empty-state-icon" style={{ color: 'var(--text-muted)' }} />
          <p style={{ fontWeight: 500, fontSize: '14px', marginBottom: '4px' }}>No active background tasks</p>
          <p style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Any documents processed will appear here in real-time. No files leave this system.</p>
        </div>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table className="jobs-table">
            <thead>
              <tr>
                <th style={{ width: '80px' }}>Task ID</th>
                <th>Operation</th>
                <th style={{ width: '130px' }}>Status</th>
                <th style={{ width: '140px' }}>Progress</th>
                <th style={{ width: '100px' }}>Triggered</th>
                <th style={{ width: '150px' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((job) => (
                <JobRow key={job.id} job={job} onCopyText={onCopyText} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
