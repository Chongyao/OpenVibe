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

export function useSessionStorage() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const hasInitialized = useRef(false);

  useEffect(() => {
    if (hasInitialized.current) return;
    hasInitialized.current = true;

    const loadedSessions = loadFromStorage<Session[]>(SESSIONS_STORAGE_KEY, []);
    setSessions(loadedSessions);

    const savedSessionId = loadFromStorage<string | null>(CURRENT_SESSION_KEY, null);
    if (savedSessionId) {
      const session = loadedSessions.find(s => s.id === savedSessionId);
      if (session) {
        setCurrentSessionId(savedSessionId);
        setMessages(session.messages);
      }
    }
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
