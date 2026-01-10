'use client';

import { useCallback, useState } from 'react';
import type { ClientMessage, Project, ServerMessage } from '@/types';

interface UseProjectOptions {
  send: (msg: ClientMessage, handler?: (response: ServerMessage) => void) => boolean;
  isConnected: boolean;
}

export function useProject({ send, isConnected }: UseProjectOptions) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeProject, setActiveProject] = useState<Project | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const generateId = () => {
    if (typeof crypto !== 'undefined' && crypto.randomUUID) {
      return crypto.randomUUID();
    }
    return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
  };

  const fetchProjects = useCallback(() => {
    if (!isConnected) return;
    
    setIsLoading(true);
    setError(null);
    
    send({
      type: 'project.list',
      id: generateId(),
      payload: {},
    }, (response) => {
      setIsLoading(false);
      if (response.type === 'error') {
        const payload = response.payload as { error: string };
        setError(payload.error);
        return;
      }
      
      interface ServerProject {
        path: string;
        name: string;
        type: string;
        status: 'running' | 'stopped';
        port?: number;
      }
      
      const payload = response.payload as { projects: ServerProject[] };
      if (payload.projects) {
        const mapped: Project[] = payload.projects.map(p => ({
          path: p.path,
          name: p.name,
          type: p.type,
          active: p.status === 'running',
        }));
        setProjects(mapped);
        const active = mapped.find(p => p.active);
        if (active) {
          setActiveProject(active);
        }
      }
    });
  }, [isConnected, send]);

  const selectProject = useCallback((path: string) => {
    if (!isConnected) return;
    
    setIsLoading(true);
    setError(null);
    
    send({
      type: 'project.select',
      id: generateId(),
      payload: { path },
    }, (response) => {
      setIsLoading(false);
      if (response.type === 'error') {
        const payload = response.payload as { error: string };
        setError(payload.error);
        return;
      }
      
      interface ServerSelectResponse {
        path: string;
        name: string;
        status: string;
        port: number;
      }
      
      const payload = response.payload as ServerSelectResponse;
      if (payload.path) {
        const selected: Project = {
          path: payload.path,
          name: payload.name,
          type: '',
          active: true,
        };
        
        setProjects(prev => {
          const found = prev.find(p => p.path === path);
          if (found) {
            selected.type = found.type;
          }
          return prev.map(p => ({
            ...p,
            active: p.path === path,
          }));
        });
        
        setActiveProject(selected);
      }
    });
  }, [isConnected, send]);

  const stopProject = useCallback((path: string) => {
    if (!isConnected) return;
    
    setIsLoading(true);
    setError(null);
    
    send({
      type: 'project.stop',
      id: generateId(),
      payload: { path },
    }, (response) => {
      setIsLoading(false);
      if (response.type === 'error') {
        const payload = response.payload as { error: string };
        setError(payload.error);
        return;
      }
      
      if (activeProject?.path === path) {
        setActiveProject(null);
      }
      setProjects(prev => prev.map(p => ({
        ...p,
        active: p.path === path ? false : p.active,
      })));
    });
  }, [isConnected, send, activeProject]);

  return {
    projects,
    activeProject,
    isLoading,
    error,
    fetchProjects,
    selectProject,
    stopProject,
  };
}
