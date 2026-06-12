import React from 'react';
import { ListChecks, Loader2 } from 'lucide-react';
import { JobRow, Job } from './JobRow';

interface JobsTableProps {
  jobs: Job[];
  loading: boolean;
}

export const JobsTable: React.FC<JobsTableProps> = ({ jobs, loading }) => {
  return (
    <div className="glass-panel" style={{ overflow: 'hidden', padding: '24px 0' }}>
      <div className="jobs-tracker-header">
        <h3 className="section-title">
          <ListChecks size={20} style={{ color: 'var(--green-600)' }} />
          Processing History
        </h3>
        {loading && (
          <Loader2 size={16} style={{ animation: 'spin 1.5s infinite linear', color: 'var(--green-600)' }} />
        )}
      </div>

      {jobs.length === 0 ? (
        <div className="empty-state">
          <ListChecks size={40} className="empty-state-icon" />
          <p style={{ fontWeight: 500, fontSize: '14px', marginBottom: '4px', color: 'var(--text-secondary)' }}>
            No tasks yet
          </p>
          <p style={{ fontSize: '12px' }}>
            Upload a file and run a tool — results appear here in real time.
          </p>
        </div>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table className="jobs-table">
            <thead>
              <tr>
                <th style={{ width: '80px' }}>ID</th>
                <th>Operation</th>
                <th style={{ width: '120px' }}>Status</th>
                <th style={{ width: '120px' }}>Progress</th>
                <th style={{ width: '80px' }}>Time</th>
                <th style={{ width: '130px' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((job) => (
                <JobRow key={job.id} job={job} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
