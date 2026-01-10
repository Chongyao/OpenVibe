'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import type { Project, Session } from '@/types';

interface UseProjectOptions {
  sessions: Session[];
  onProjectChange?: (projectPath: string | null) => void;
}

function extractProjectsFromSessions(sessions: Session[]): Project[] {
  const projectMap = new Map<string, { count: number; lastUpdated: number }>();
  
  for (const session of sessions) {
    const dir = session.directory;
    if (!dir) continue;
    
    const updatedAt = session.time?.updated || session.createdAt;
    const existing = projectMap.get(dir);
    
    if (existing) {
      existing.count++;
      if (updatedAt > existing.lastUpdated) {
        existing.lastUpdated = updatedAt;
      }
    } else {
      projectMap.set(dir, { count: 1, lastUpdated: updatedAt });
    }
  }
  
  const projects: Project[] = [];
  for (const [path, data] of projectMap) {
    const name = path.split('/').pop() || path;
    projects.push({
      path,
      name,
      port: 0,
      tmuxSession: '',
      status: 'stopped',
      sessionCount: data.count,
      lastUpdated: data.lastUpdated,
    });
  }
  
  projects.sort((a, b) => (b.lastUpdated || 0) - (a.lastUpdated || 0));
  return projects;
}

export function useProject({ sessions, onProjectChange }: UseProjectOptions) {
  const [activeProjectPath, setActiveProjectPath] = useState<string | null>(null);

  const projects = useMemo(() => {
    return extractProjectsFromSessions(sessions);
  }, [sessions]);

  // Auto-select first project when projects are loaded and none is selected
  useEffect(() => {
    if (projects.length > 0 && !activeProjectPath) {
      const firstProjectPath = projects[0].path;
      setActiveProjectPath(firstProjectPath);
      onProjectChange?.(firstProjectPath);
    }
  }, [projects, activeProjectPath, onProjectChange]);

  // If active project no longer exists in the list, reset to first available
  useEffect(() => {
    if (activeProjectPath && projects.length > 0) {
      const stillExists = projects.some(p => p.path === activeProjectPath);
      if (!stillExists) {
        const firstProjectPath = projects[0].path;
        setActiveProjectPath(firstProjectPath);
        onProjectChange?.(firstProjectPath);
      }
    }
  }, [projects, activeProjectPath, onProjectChange]);

  const activeProject = useMemo(() => {
    if (!activeProjectPath) return projects[0] || null;
    return projects.find(p => p.path === activeProjectPath) || projects[0] || null;
  }, [projects, activeProjectPath]);

  const selectProject = useCallback((path: string) => {
    setActiveProjectPath(path);
    onProjectChange?.(path);
  }, [onProjectChange]);

  // Filter sessions by active project
  const filteredSessions = useMemo(() => {
    if (!activeProjectPath) return sessions;
    return sessions.filter(s => s.directory === activeProjectPath);
  }, [sessions, activeProjectPath]);

  return {
    projects,
    activeProject,
    activeProjectPath,
    selectProject,
    filteredSessions,
  };
}
