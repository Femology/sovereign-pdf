import React from 'react';

interface ToolCardProps {
  title: string;
  desc: string;
  icon: React.ReactNode;
  active: boolean;
  onClick: () => void;
}

export const ToolCard: React.FC<ToolCardProps> = ({
  title,
  desc,
  icon,
  active,
  onClick,
}) => {
  return (
    <button
      className={`tool-card ${active ? 'active' : ''}`}
      onClick={onClick}
      type="button"
    >
      <div className="tool-icon">{icon}</div>
      <div className="tool-info">
        <span className="tool-title">{title}</span>
        <span className="tool-desc">{desc}</span>
      </div>
    </button>
  );
};
