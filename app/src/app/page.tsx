'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { MessageBubble, InputBar, StatusIndicator, SessionSidebar, SettingsPanel, MacroDeck, ProjectSelector, useToast } from '@/components';
import type { MacroAction } from '@/components';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useProjects } from '@/hooks/useProjects';
import { useSessionStorage } from '@/hooks/useSessionStorage';
import { useMessageHandler } from '@/hooks/useMessageHandler';
import { generateId } from '@/lib/utils';
import type { Message, ClientMessage, ServerMessage, Project } from '@/types';

const WS_URL = process.env.NEXT_PUBLIC_WS_URL ||
  (typeof window !== 'undefined' && window.location.hostname !== 'localhost'
    ? `ws://${window.location.host}/ws`
    : 'ws://localhost:8080/ws');

type SendFn = (msg: ClientMessage, handler?: (response: ServerMessage) => void) => boolean;

export default function Home() {
  const [isCreatingSession, setIsCreatingSession] = useState(false);
  const [activeProjectPath, setActiveProjectPath] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const { addToast } = useToast();

  const {
    sessions,
    setSessions,
    currentSessionId,
    setCurrentSessionId,
    messages,
    setMessages,
    updateSessionMessages,
    addSession,
    deleteSession,
  } = useSessionStorage();

  const sendRef = useRef<SendFn | null>(null);

  const {
    projects: backendProjects,
    loading: projectsLoading,
    listProjects,
    startProject,
    stopProject,
    handleResponse: handleProjectResponse,
    handleError: handleProjectError,
  } = useProjects({ send: (msg) => sendRef.current?.({ ...msg, payload: msg.payload } as ClientMessage) });

  const listProjectsRef = useRef(listProjects);
  listProjectsRef.current = listProjects;

  const { handleMessage, setPendingRequest } = useMessageHandler({
    currentSessionId,
    isCreatingSession,
    setMessages,
    setSessions,
    addSession,
    updateSessionMessages,
    setCurrentSessionId,
    setIsCreatingSession,
    onSessionCreated: () => addToast('success', 'New chat created'),
    onError: (error) => addToast('error', error),
    onResponse: handleProjectResponse,
    onErrorById: handleProjectError,
  });

  const { state, send } = useWebSocket({
    url: WS_URL,
    onMessage: handleMessage,
    onConnect: () => {
      listProjectsRef.current();
      send({
        type: 'session.list',
        id: generateId(),
        payload: {},
      });
    },
  });

  useEffect(() => {
    sendRef.current = send;
  }, [send]);

  const isConnected = state === 'connected';

  const projects = useMemo(() => {
    if (backendProjects.length > 0) return backendProjects;

    const projectMap = new Map<string, Project>();
    for (const session of sessions) {
      const dir = session.directory;
      if (!dir || projectMap.has(dir)) continue;
      projectMap.set(dir, {
        path: dir,
        name: dir.split('/').pop() || dir,
        port: 0,
        tmuxSession: '',
        status: 'stopped',
      });
    }
    return Array.from(projectMap.values());
  }, [backendProjects, sessions]);

  const activeProject = useMemo(() => {
    if (!activeProjectPath && projects.length > 0) {
      return projects[0];
    }
    if (!activeProjectPath) return null;
    return projects.find(p => p.path === activeProjectPath) || projects[0] || null;
  }, [projects, activeProjectPath]);

  const filteredSessions = useMemo(() => {
    const projectPath = activeProject?.path;
    if (!projectPath) return sessions;
    return sessions.filter(s => s.directory === projectPath);
  }, [sessions, activeProject]);

  const selectProject = useCallback((path: string) => {
    setActiveProjectPath(path);
    setCurrentSessionId(null);
    setMessages([]);
  }, [setCurrentSessionId, setMessages]);

  const handleStartProject = useCallback(async (path: string) => {
    try {
      await startProject(path);
      addToast('success', 'Project started');
    } catch (err) {
      addToast('error', err instanceof Error ? err.message : 'Failed to start project');
    }
  }, [startProject, addToast]);

  const handleStopProject = useCallback(async (path: string) => {
    try {
      await stopProject(path);
      addToast('success', 'Project stopped');
    } catch (err) {
      addToast('error', err instanceof Error ? err.message : 'Failed to stop project');
    }
  }, [stopProject, addToast]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  useEffect(() => {
    if (currentSessionId && state === 'connected' && messages.length === 0) {
      const session = sessions.find(s => s.id === currentSessionId);
      if (session && session.messages.length === 0) {
        const requestId = generateId();
        setPendingRequest(requestId, currentSessionId);
        send({
          type: 'session.messages',
          id: requestId,
          payload: { sessionId: currentSessionId },
        });
      }
    }
  }, [currentSessionId, state, sessions, messages.length, send, setPendingRequest]);

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

      if (session.messages.length > 0) {
        setMessages(session.messages);
      } else {
        setMessages([]);
        if (state === 'connected') {
          const requestId = generateId();
          setPendingRequest(requestId, sessionId);
          send({
            type: 'session.messages',
            id: requestId,
            payload: { sessionId },
          });
        }
      }
    }
  }, [sessions, state, send, setCurrentSessionId, setMessages, setPendingRequest]);

  const handleDeleteSession = useCallback((sessionId: string) => {
    if (state === 'connected') {
      send({
        type: 'session.delete',
        id: generateId(),
        payload: { sessionId },
      });
    }

    deleteSession(sessionId);
    if (currentSessionId === sessionId) {
      setCurrentSessionId(null);
      setMessages([]);
    }
    addToast('success', 'Chat deleted');
  }, [currentSessionId, state, send, addToast, deleteSession, setCurrentSessionId, setMessages]);

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
  }, [currentSessionId, state, send, setMessages, updateSessionMessages]);

  const isReady = isConnected && currentSessionId;
  const isStreaming = messages.some(m => m.streaming);

  const handleMacroAction = useCallback((_action: MacroAction, prompt: string) => {
    if (!isReady) return;
    handleSend(prompt);
  }, [isReady, handleSend]);

  return (
    <div className="flex h-[100dvh] bg-[var(--bg-primary)] overflow-hidden">
      <SessionSidebar
        sessions={filteredSessions}
        currentSessionId={currentSessionId}
        onSelectSession={handleSelectSession}
        onNewSession={handleNewSession}
        onDeleteSession={handleDeleteSession}
        isLoading={isCreatingSession}
      />

      <div className="flex flex-col flex-1 min-w-0 min-h-0">
        <header className="safe-area-top glass border-b border-[var(--border-color)] flex-shrink-0">
          <div className="flex items-center justify-between px-4 py-3 max-w-4xl mx-auto">
            <div className="flex items-center gap-3 pl-12 md:pl-0">
              <h1 className="text-lg font-semibold neon-text hidden md:block">OpenVibe</h1>
              {projects.length > 0 && (
                <ProjectSelector
                  projects={projects}
                  activeProject={activeProject}
                  onSelect={selectProject}
                  onStart={handleStartProject}
                  onStop={handleStopProject}
                  disabled={!isConnected}
                  loading={projectsLoading}
                />
              )}
            </div>
            <div className="flex items-center gap-3">
              <StatusIndicator state={state} />
              <SettingsPanel />
            </div>
          </div>
        </header>

        <main className="flex-1 overflow-y-auto min-h-0 -webkit-overflow-scrolling-touch">
          <div className="max-w-4xl mx-auto px-4 py-6 pb-4">
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
