'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { MessageBubble, InputBar, StatusIndicator, SessionSidebar, SettingsPanel, MacroDeck } from '@/components';
import type { MacroAction } from '@/components';
import { useWebSocket } from '@/hooks/useWebSocket';
import type { Message, Session, ServerMessage, StreamPayload } from '@/types';

const WS_URL = process.env.NEXT_PUBLIC_WS_URL || 
  (typeof window !== 'undefined' && window.location.hostname !== 'localhost'
    ? `ws://${window.location.host}/ws`
    : 'ws://localhost:8080/ws');

function generateId(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}

const SESSIONS_STORAGE_KEY = 'openvibe_sessions';
const CURRENT_SESSION_KEY = 'openvibe_current_session';

function loadSessions(): Session[] {
  if (typeof window === 'undefined') return [];
  try {
    const stored = localStorage.getItem(SESSIONS_STORAGE_KEY);
    return stored ? JSON.parse(stored) : [];
  } catch {
    return [];
  }
}

function saveSessions(sessions: Session[]) {
  if (typeof window === 'undefined') return;
  localStorage.setItem(SESSIONS_STORAGE_KEY, JSON.stringify(sessions));
}

function loadCurrentSessionId(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(CURRENT_SESSION_KEY);
}

function saveCurrentSessionId(sessionId: string | null) {
  if (typeof window === 'undefined') return;
  if (sessionId) {
    localStorage.setItem(CURRENT_SESSION_KEY, sessionId);
  } else {
    localStorage.removeItem(CURRENT_SESSION_KEY);
  }
}

export default function Home() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null);
  const [isCreatingSession, setIsCreatingSession] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const streamingMessageId = useRef<string | null>(null);
  const hasInitialized = useRef(false);

  useEffect(() => {
    if (!hasInitialized.current) {
      hasInitialized.current = true;
      const loaded = loadSessions();
      setSessions(loaded);
      
      const savedCurrentId = loadCurrentSessionId();
      if (savedCurrentId && loaded.find(s => s.id === savedCurrentId)) {
        setCurrentSessionId(savedCurrentId);
        const session = loaded.find(s => s.id === savedCurrentId);
        if (session) {
          setMessages(session.messages);
        }
      }
    }
  }, []);

  useEffect(() => {
    saveCurrentSessionId(currentSessionId);
  }, [currentSessionId]);

  const updateSessionMessages = useCallback((sessionId: string, msgs: Message[]) => {
    setSessions(prev => {
      const updated = prev.map(s => 
        s.id === sessionId ? { ...s, messages: msgs } : s
      );
      saveSessions(updated);
      return updated;
    });
  }, []);

  const handleMessage = useCallback((msg: ServerMessage) => {
    switch (msg.type) {
      case 'response': {
        const payload = msg.payload as { id?: string; sessionId?: string; title?: string };
        const sessionId = payload.id || payload.sessionId;
        if (sessionId && isCreatingSession) {
          const newSession: Session = {
            id: sessionId,
            title: payload.title || 'New Chat',
            createdAt: Date.now(),
            messages: [],
          };
          setSessions(prev => {
            const updated = [newSession, ...prev];
            saveSessions(updated);
            return updated;
          });
          setCurrentSessionId(sessionId);
          setMessages([]);
          setIsCreatingSession(false);
        }
        break;
      }
      case 'stream': {
        const payload = msg.payload as StreamPayload;
        const messageId = msg.id;
        
        if (!messageId) break;

        if (streamingMessageId.current !== messageId) {
          streamingMessageId.current = messageId;
          setMessages(prev => {
            const updated = [
              ...prev,
              {
                id: messageId,
                role: 'assistant' as const,
                content: payload.text,
                timestamp: Date.now(),
                streaming: true,
              },
            ];
            if (currentSessionId) {
              updateSessionMessages(currentSessionId, updated);
            }
            return updated;
          });
        } else {
          setMessages(prev => {
            const updated = prev.map(m =>
              m.id === messageId
                ? { ...m, content: m.content + payload.text }
                : m
            );
            if (currentSessionId) {
              updateSessionMessages(currentSessionId, updated);
            }
            return updated;
          });
        }
        break;
      }
      case 'stream.end': {
        const messageId = msg.id;
        if (messageId) {
          setMessages(prev => {
            const updated = prev.map(m =>
              m.id === messageId ? { ...m, streaming: false } : m
            );
            if (currentSessionId) {
              updateSessionMessages(currentSessionId, updated);
            }
            return updated;
          });
          streamingMessageId.current = null;
        }
        break;
      }
      case 'error': {
        const payload = msg.payload as { error: string };
        setMessages(prev => [
          ...prev,
          {
            id: generateId(),
            role: 'assistant',
            content: `Error: ${payload.error}`,
            timestamp: Date.now(),
          },
        ]);
        streamingMessageId.current = null;
        setIsCreatingSession(false);
        break;
      }
    }
  }, [currentSessionId, isCreatingSession, updateSessionMessages]);

  const { state, send } = useWebSocket({
    url: WS_URL,
    onMessage: handleMessage,
    onConnect: () => {
      if (sessions.length === 0 && !currentSessionId) {
        handleNewSession();
      }
    },
  });

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleNewSession = useCallback(() => {
    if (state !== 'connected' || isCreatingSession) return;
    setIsCreatingSession(true);
    send({
      type: 'session.create',
      id: generateId(),
      payload: { title: 'New Chat' },
    });
  }, [state, send, isCreatingSession]);

  const handleSelectSession = useCallback((sessionId: string) => {
    const session = sessions.find(s => s.id === sessionId);
    if (session) {
      setCurrentSessionId(sessionId);
      setMessages(session.messages);
    }
  }, [sessions]);

  const handleDeleteSession = useCallback((sessionId: string) => {
    setSessions(prev => {
      const updated = prev.filter(s => s.id !== sessionId);
      saveSessions(updated);
      return updated;
    });
    if (currentSessionId === sessionId) {
      setCurrentSessionId(null);
      setMessages([]);
    }
  }, [currentSessionId]);

  const handleSend = useCallback((content: string) => {
    if (!currentSessionId || state !== 'connected') return;

    const userMessage: Message = {
      id: generateId(),
      role: 'user',
      content,
      timestamp: Date.now(),
    };
    
    setMessages(prev => {
      const updated = [...prev, userMessage];
      updateSessionMessages(currentSessionId, updated);
      return updated;
    });

    send({
      type: 'prompt',
      id: generateId(),
      payload: {
        sessionId: currentSessionId,
        content,
      },
    });
  }, [currentSessionId, state, send, updateSessionMessages]);

  const isConnected = state === 'connected';
  const isReady = isConnected && currentSessionId;
  const isStreaming = messages.some(m => m.streaming);

  const handleMacroAction = useCallback((action: MacroAction, prompt: string) => {
    if (!isReady) return;
    handleSend(prompt);
  }, [isReady, handleSend]);

  return (
    <div className="flex h-screen bg-[var(--bg-primary)]">
      <SessionSidebar
        sessions={sessions}
        currentSessionId={currentSessionId}
        onSelectSession={handleSelectSession}
        onNewSession={handleNewSession}
        onDeleteSession={handleDeleteSession}
        isLoading={isCreatingSession}
      />

      <div className="flex flex-col flex-1 min-w-0">
        <header className="safe-area-top glass border-b border-[var(--border-color)] flex-shrink-0">
          <div className="flex items-center justify-between px-4 py-3 max-w-4xl mx-auto">
            <div className="flex items-center gap-3 pl-12 md:pl-0">
              <h1 className="text-lg font-semibold neon-text">OpenVibe</h1>
            </div>
            <div className="flex items-center gap-3">
              <StatusIndicator state={state} />
              <SettingsPanel />
            </div>
          </div>
        </header>

        <main className="flex-1 overflow-y-auto">
          <div className="max-w-4xl mx-auto px-4 py-6">
            {messages.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full min-h-[50vh] text-center">
                <div className="animate-slide-up">
                  <div className="w-16 h-16 rounded-full bg-[var(--bg-secondary)] border border-[var(--border-color)] flex items-center justify-center mb-4 mx-auto">
                    <span className="text-2xl">🚀</span>
                  </div>
                  <h2 className="text-xl font-semibold text-[var(--text-primary)] mb-2">
                    Welcome to OpenVibe
                  </h2>
                  <p className="text-[var(--text-secondary)] max-w-sm">
                    {isReady
                      ? "Start a conversation with your AI coding assistant."
                      : "Connecting to your development server..."}
                  </p>
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                {messages.map(message => (
                  <MessageBubble key={message.id} message={message} />
                ))}
                <div ref={messagesEndRef} />
              </div>
            )}
          </div>
        </main>

        <MacroDeck
          onAction={handleMacroAction}
          disabled={!isReady}
          isStreaming={isStreaming}
        />

        <InputBar
          onSend={handleSend}
          disabled={!isReady}
          placeholder={
            !isConnected
              ? "Connecting..."
              : !currentSessionId
              ? "Initializing session..."
              : "Ask me anything about coding..."
          }
        />
      </div>
    </div>
  );
}
