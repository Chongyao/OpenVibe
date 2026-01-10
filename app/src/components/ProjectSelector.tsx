'use client';

import { memo, useCallback, useEffect, useRef, useState } from 'react';
import type { Project } from '@/types';
import { FolderIcon, ChevronDownIcon, IconRefresh } from './Icons';

interface ProjectSelectorProps {
  projects: Project[];
  activeProject: Project | null;
  isLoading: boolean;
  onSelect: (path: string) => void;
  onRefresh: () => void;
  disabled?: boolean;
}

function getProjectTypeColor(type: string): string {
  const colors: Record<string, string> = {
    go: '#00ADD8',
    node: '#68A063',
    python: '#3776AB',
    rust: '#DEA584',
    java: '#ED8B00',
    ruby: '#CC342D',
    php: '#777BB4',
  };
  return colors[type.toLowerCase()] || 'var(--accent-primary)';
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
      <div
        className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0"
        style={{ backgroundColor: `${getProjectTypeColor(project.type)}20` }}
      >
        <FolderIcon
          className="w-4 h-4"
          style={{ color: getProjectTypeColor(project.type) }}
        />
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium text-[var(--text-primary)] truncate">
          {project.name}
        </div>
        <div className="text-xs text-[var(--text-muted)] flex items-center gap-1.5">
          <span
            className="inline-block w-1.5 h-1.5 rounded-full"
            style={{ backgroundColor: getProjectTypeColor(project.type) }}
          />
          <span>{project.type}</span>
          {project.active && (
            <span className="text-[var(--accent-primary)] ml-1">• running</span>
          )}
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
  isLoading,
  onSelect,
  onRefresh,
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

  const handleRefresh = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    onRefresh();
  }, [onRefresh]);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  useEffect(() => {
    onRefresh();
  }, [onRefresh]);

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
        {isLoading ? (
          <div className="w-4 h-4 border-2 border-[var(--accent-primary)] border-t-transparent rounded-full animate-spin" />
        ) : (
          <FolderIcon className="w-4 h-4 text-[var(--accent-primary)]" />
        )}
        <span className="text-sm font-medium text-[var(--text-primary)] max-w-[120px] truncate">
          {activeProject?.name || 'Select Project'}
        </span>
        {activeProject && (
          <span className="text-xs text-[var(--text-muted)]">
            • {activeProject.type}
          </span>
        )}
        <ChevronDownIcon
          className={`w-4 h-4 text-[var(--text-muted)] transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`}
        />
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 mt-2 w-72 z-50 animate-fade-in">
          <div className="glass rounded-xl shadow-lg overflow-hidden">
            <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-color)]">
              <span className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">
                Projects
              </span>
              <button
                onClick={handleRefresh}
                disabled={isLoading}
                className={`
                  p-1.5 rounded-md text-[var(--text-muted)]
                  hover:text-[var(--accent-primary)] hover:bg-[rgba(0,255,157,0.1)]
                  transition-all duration-200
                  ${isLoading ? 'animate-spin' : ''}
                `}
              >
                <IconRefresh className="w-4 h-4" />
              </button>
            </div>
            <div className="max-h-64 overflow-y-auto p-1.5 space-y-0.5">
              {projects.length === 0 ? (
                <div className="text-center py-6 text-[var(--text-muted)] text-sm">
                  {isLoading ? 'Loading projects...' : 'No projects found'}
                </div>
              ) : (
                projects.map(project => (
                  <ProjectItem
                    key={project.path}
                    project={project}
                    isActive={activeProject?.path === project.path}
                    onSelect={() => handleSelect(project.path)}
                  />
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
});
