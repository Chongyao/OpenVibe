'use client';

import { useCallback, useMemo, useState } from 'react';
import type { Project, Session } from '@/types';

interface UseProjectOptions {
  sessions: Session[];
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
      sessionCount: data.count,
      lastUpdated: data.lastUpdated,
    });
  }
  
  projects.sort((a, b) => b.lastUpdated - a.lastUpdated);
  return projects;
}

export function useProject({ sessions }: UseProjectOptions) {
  const [activeProjectPath, setActiveProjectPath] = useState<string | null>(null);

  const projects = useMemo(() => {
    return extractProjectsFromSessions(sessions);
  }, [sessions]);

  const activeProject = useMemo(() => {
    if (!activeProjectPath) return null;
    return projects.find(p => p.path === activeProjectPath) || null;
  }, [projects, activeProjectPath]);

  const selectProject = useCallback((path: string) => {
    setActiveProjectPath(path);
  }, []);

  return {
    projects,
    activeProject,
    selectProject,
  };
}
