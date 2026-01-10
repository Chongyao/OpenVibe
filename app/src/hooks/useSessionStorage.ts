'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { Message, Session } from '@/types';

const SESSIONS_STORAGE_KEY = 'openvibe_sessions';
const CURRENT_SESSION_KEY = 'openvibe_current_session';

function loadFromStorage<T>(key: string, defaultValue: T): T {
  if (typeof window === 'undefined') return defaultValue;
  try {
    const stored = localStorage.getItem(key);
    return stored ? JSON.parse(stored) : defaultValue;
  } catch {
    return defaultValue;
  }
}

function saveToStorage<T>(key: string, value: T): void {
  if (typeof window === 'undefined') return;
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Storage quota exceeded or other error
  }
}

function getInitialSessions(): Session[] {
  return loadFromStorage<Session[]>(SESSIONS_STORAGE_KEY, []);
}

function getInitialSessionId(): string | null {
  return loadFromStorage<string | null>(CURRENT_SESSION_KEY, null);
}

export function useSessionStorage() {
  const [sessions, setSessions] = useState<Session[]>(getInitialSessions);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(getInitialSessionId);
  const [messages, setMessages] = useState<Message[]>(() => {
    const sessionId = getInitialSessionId();
    if (!sessionId) return [];
    const loadedSessions = getInitialSessions();
    const session = loadedSessions.find(s => s.id === sessionId);
    return session?.messages || [];
  });
  const hasInitialized = useRef(false);

  useEffect(() => {
    if (hasInitialized.current) return;
    hasInitialized.current = true;
  }, []);

  useEffect(() => {
    if (currentSessionId) {
      saveToStorage(CURRENT_SESSION_KEY, currentSessionId);
    } else if (typeof window !== 'undefined') {
      localStorage.removeItem(CURRENT_SESSION_KEY);
    }
  }, [currentSessionId]);

  const saveSessions = useCallback((newSessions: Session[]) => {
    setSessions(newSessions);
    saveToStorage(SESSIONS_STORAGE_KEY, newSessions);
  }, []);

  const updateSessionMessages = useCallback((sessionId: string, msgs: Message[]) => {
    setSessions(prev => {
      const updated = prev.map(s =>
        s.id === sessionId ? { ...s, messages: msgs } : s
      );
      saveToStorage(SESSIONS_STORAGE_KEY, updated);
      return updated;
    });
  }, []);

  const addSession = useCallback((session: Session) => {
    setSessions(prev => {
      const updated = [session, ...prev];
      saveToStorage(SESSIONS_STORAGE_KEY, updated);
      return updated;
    });
  }, []);

  const deleteSession = useCallback((sessionId: string) => {
    setSessions(prev => {
      const updated = prev.filter(s => s.id !== sessionId);
      saveToStorage(SESSIONS_STORAGE_KEY, updated);
      return updated;
    });
  }, []);

  return {
    sessions,
    setSessions: saveSessions,
    currentSessionId,
    setCurrentSessionId,
    messages,
    setMessages,
    updateSessionMessages,
    addSession,
    deleteSession,
  };
}
