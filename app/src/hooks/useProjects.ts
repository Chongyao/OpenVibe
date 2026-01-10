'use client';

import { useCallback, useRef, useState } from 'react';
import type { Project } from '@/types';
import { generateId } from '@/lib/utils';

type SendFn = (msg: { type: string; id: string; payload: Record<string, unknown> }) => void;

interface UseProjectsOptions {
  send: SendFn;
}

interface PendingRequest<T = unknown> {
  resolve: (value: T) => void;
  reject: (reason: unknown) => void;
}

export function useProjects({ send }: UseProjectsOptions) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pendingRequests = useRef<Map<string, PendingRequest>>(new Map());

  const listProjects = useCallback(() => {
    setLoading(true);
    setError(null);
    
    const id = generateId();
    send({
      type: 'project.list',
      id,
      payload: {},
    });

    return new Promise<Project[]>((resolve, reject) => {
      pendingRequests.current.set(id, { resolve: resolve as (value: unknown) => void, reject });
    });
  }, [send]);

  const startProject = useCallback((path: string) => {
    const id = generateId();
    send({
      type: 'project.start',
      id,
      payload: { path },
    });

    return new Promise<Project>((resolve, reject) => {
      pendingRequests.current.set(id, { resolve: resolve as (value: unknown) => void, reject });
    });
  }, [send]);

  const stopProject = useCallback((path: string) => {
    const id = generateId();
    send({
      type: 'project.stop',
      id,
      payload: { path },
    });

    return new Promise<boolean>((resolve, reject) => {
      pendingRequests.current.set(id, { resolve: resolve as (value: unknown) => void, reject });
    });
  }, [send]);

  const handleResponse = useCallback((msgId: string, payload: unknown) => {
    const pending = pendingRequests.current.get(msgId);
    if (!pending) return false;

    pendingRequests.current.delete(msgId);
    setLoading(false);

    const data = payload as Record<string, unknown>;
    
    if (data.error) {
      setError(data.error as string);
      pending.reject(new Error(data.error as string));
      return true;
    }

    if (data.projects) {
      const projectList = data.projects as Project[];
      setProjects(projectList);
      pending.resolve(projectList);
      return true;
    }

    if (data.project) {
      const project = data.project as Project;
      setProjects(prev => prev.map(p => p.path === project.path ? project : p));
      pending.resolve(project);
      return true;
    }

    if (data.success !== undefined) {
      pending.resolve(data.success as boolean);
      listProjects();
      return true;
    }

    pending.resolve(payload);
    return true;
  }, [listProjects]);

  const handleError = useCallback((msgId: string, errorMsg: string) => {
    const pending = pendingRequests.current.get(msgId);
    if (!pending) return false;

    pendingRequests.current.delete(msgId);
    setLoading(false);
    setError(errorMsg);
    pending.reject(new Error(errorMsg));
    return true;
  }, []);

  return {
    projects,
    loading,
    error,
    listProjects,
    startProject,
    stopProject,
    handleResponse,
    handleError,
  };
}
