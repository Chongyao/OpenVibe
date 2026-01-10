'use client';

import { memo, useCallback, useEffect, useRef, useState } from 'react';
import type { Project } from '@/types';
import { FolderIcon, ChevronDownIcon } from './Icons';

interface ProjectSelectorProps {
  projects: Project[];
  activeProject: Project | null;
  onSelect: (path: string) => void;
  disabled?: boolean;
}

function formatTimeAgo(timestamp: number): string {
  const now = Date.now();
  const diff = now - timestamp;
  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(diff / 3600000);
  const days = Math.floor(diff / 86400000);
  
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days < 7) return `${days}d ago`;
  return new Date(timestamp).toLocaleDateString();
}

function ProjectItem({
  project,
  isActive,
  onSelect,
}: {
  project: Project;
  isActive: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      onClick={onSelect}
      className={`
        w-full flex items-center gap-3 px-3 py-2.5 text-left
        transition-all duration-200 rounded-lg
        ${isActive
          ? 'bg-[rgba(0,255,157,0.1)] border border-[rgba(0,255,157,0.3)]'
          : 'hover:bg-[var(--bg-tertiary)] border border-transparent'
        }
      `}
    >
      <div className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0 bg-[rgba(0,255,157,0.1)]">
        <FolderIcon className="w-4 h-4 text-[var(--accent-primary)]" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium text-[var(--text-primary)] truncate">
          {project.name}
        </div>
        <div className="text-xs text-[var(--text-muted)] flex items-center gap-1.5">
          <span>{project.sessionCount} sessions</span>
          <span>•</span>
          <span>{formatTimeAgo(project.lastUpdated)}</span>
        </div>
      </div>
      {isActive && (
        <div className="w-2 h-2 rounded-full bg-[var(--accent-primary)] animate-pulse-glow" />
      )}
    </button>
  );
}

export const ProjectSelector = memo(function ProjectSelector({
  projects,
  activeProject,
  onSelect,
  disabled = false,
}: ProjectSelectorProps) {
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const handleToggle = useCallback(() => {
    if (disabled) return;
    setIsOpen(prev => !prev);
  }, [disabled]);

  const handleSelect = useCallback((path: string) => {
    onSelect(path);
    setIsOpen(false);
  }, [onSelect]);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  if (projects.length === 0) {
    return null;
  }

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={handleToggle}
        disabled={disabled}
        className={`
          flex items-center gap-2 px-3 py-1.5 rounded-lg
          bg-[var(--bg-tertiary)] border border-[var(--border-color)]
          transition-all duration-200
          ${disabled ? 'opacity-50 cursor-not-allowed' : 'hover:border-[var(--accent-primary)] hover:shadow-[var(--glow-primary)]'}
          ${isOpen ? 'border-[var(--accent-primary)]' : ''}
        `}
      >
        <FolderIcon className="w-4 h-4 text-[var(--accent-primary)]" />
        <span className="text-sm font-medium text-[var(--text-primary)] max-w-[120px] truncate">
          {activeProject?.name || projects[0]?.name || 'Projects'}
        </span>
        <ChevronDownIcon
          className={`w-4 h-4 text-[var(--text-muted)] transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`}
        />
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 mt-2 w-72 z-50 animate-fade-in">
          <div className="glass rounded-xl shadow-lg overflow-hidden">
            <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-color)]">
              <span className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">
                Projects ({projects.length})
              </span>
            </div>
            <div className="max-h-64 overflow-y-auto p-1.5 space-y-0.5">
              {projects.map(project => (
                <ProjectItem
                  key={project.path}
                  project={project}
                  isActive={activeProject?.path === project.path}
                  onSelect={() => handleSelect(project.path)}
                />
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
});
