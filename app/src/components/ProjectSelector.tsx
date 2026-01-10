'use client';

import { memo, useCallback, useEffect, useRef, useState } from 'react';
import type { Project, ProjectStatus } from '@/types';
import { FolderIcon, ChevronDownIcon } from './Icons';

interface ProjectSelectorProps {
  projects: Project[];
  activeProject: Project | null;
  onSelect: (path: string) => void;
  onStart?: (path: string) => void;
  onStop?: (path: string) => void;
  disabled?: boolean;
  loading?: boolean;
}

function StatusIndicator({ status }: { status: ProjectStatus }) {
  const statusConfig: Record<ProjectStatus, { color: string; label: string; animate?: boolean }> = {
    running: { color: 'bg-[var(--accent-primary)]', label: 'Running', animate: true },
    starting: { color: 'bg-yellow-400', label: 'Starting', animate: true },
    stopped: { color: 'bg-gray-500', label: 'Stopped' },
    error: { color: 'bg-[var(--accent-error)]', label: 'Error' },
  };

  const config = statusConfig[status] || statusConfig.stopped;

  return (
    <div className="flex items-center gap-1.5">
      <div className={`w-2 h-2 rounded-full ${config.color} ${config.animate ? 'animate-pulse' : ''}`} />
      <span className="text-xs text-[var(--text-muted)]">{config.label}</span>
    </div>
  );
}

function ProjectItem({
  project,
  isActive,
  onSelect,
  onStart,
  onStop,
}: {
  project: Project;
  isActive: boolean;
  onSelect: () => void;
  onStart?: () => void;
  onStop?: () => void;
}) {
  const handleAction = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (project.status === 'running') {
      onStop?.();
    } else if (project.status === 'stopped' || project.status === 'error') {
      onStart?.();
    }
  }, [project.status, onStart, onStop]);

  const canStart = project.status === 'stopped' || project.status === 'error';
  const canStop = project.status === 'running';
  const isLoading = project.status === 'starting';

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
        <StatusIndicator status={project.status} />
      </div>
      {(canStart || canStop) && (
        <button
          onClick={handleAction}
          disabled={isLoading}
          className={`
            px-2 py-1 text-xs font-medium rounded-md transition-all
            ${canStart 
              ? 'bg-[var(--accent-primary)] text-black hover:opacity-90' 
              : 'bg-[var(--bg-tertiary)] text-[var(--text-secondary)] hover:bg-[var(--accent-error)] hover:text-white'
            }
            ${isLoading ? 'opacity-50 cursor-not-allowed' : ''}
          `}
        >
          {canStart ? 'Start' : 'Stop'}
        </button>
      )}
      {isLoading && (
        <div className="w-4 h-4 border-2 border-[var(--accent-primary)] border-t-transparent rounded-full animate-spin" />
      )}
    </button>
  );
}

export const ProjectSelector = memo(function ProjectSelector({
  projects,
  activeProject,
  onSelect,
  onStart,
  onStop,
  disabled = false,
  loading = false,
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

  const runningCount = projects.filter(p => p.status === 'running').length;

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={handleToggle}
        disabled={disabled || loading}
        className={`
          flex items-center gap-2 px-3 py-1.5 rounded-lg
          bg-[var(--bg-tertiary)] border border-[var(--border-color)]
          transition-all duration-200
          ${disabled || loading ? 'opacity-50 cursor-not-allowed' : 'hover:border-[var(--accent-primary)] hover:shadow-[var(--glow-primary)]'}
          ${isOpen ? 'border-[var(--accent-primary)]' : ''}
        `}
      >
        {loading ? (
          <div className="w-4 h-4 border-2 border-[var(--accent-primary)] border-t-transparent rounded-full animate-spin" />
        ) : (
          <FolderIcon className="w-4 h-4 text-[var(--accent-primary)]" />
        )}
        <span className="text-sm font-medium text-[var(--text-primary)] max-w-[120px] truncate">
          {activeProject?.name || projects[0]?.name || 'Projects'}
        </span>
        {activeProject && (
          <div className={`w-2 h-2 rounded-full ${activeProject.status === 'running' ? 'bg-[var(--accent-primary)]' : 'bg-gray-500'}`} />
        )}
        <ChevronDownIcon
          className={`w-4 h-4 text-[var(--text-muted)] transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`}
        />
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 mt-2 w-80 z-50 animate-fade-in">
          <div className="glass rounded-xl shadow-lg overflow-hidden">
            <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-color)]">
              <span className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">
                Projects ({projects.length})
              </span>
              <span className="text-xs text-[var(--accent-primary)]">
                {runningCount} running
              </span>
            </div>
            <div className="max-h-64 overflow-y-auto p-1.5 space-y-0.5">
              {projects.map(project => (
                <ProjectItem
                  key={project.path}
                  project={project}
                  isActive={activeProject?.path === project.path}
                  onSelect={() => handleSelect(project.path)}
                  onStart={() => onStart?.(project.path)}
                  onStop={() => onStop?.(project.path)}
                />
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
});
