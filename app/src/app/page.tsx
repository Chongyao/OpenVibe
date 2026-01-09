'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { MessageBubble, InputBar, StatusIndicator } from '@/components';
import { useWebSocket } from '@/hooks/useWebSocket';
import type { Message, ServerMessage, StreamPayload } from '@/types';

// Get WebSocket URL from environment or default
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

export default function Home() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const streamingMessageId = useRef<string | null>(null);

  const handleMessage = useCallback((msg: ServerMessage) => {
    switch (msg.type) {
      case 'response': {
        // Session created or other response
        // OpenCode returns { id: "ses_xxx", ... }
        const payload = msg.payload as { id?: string; sessionId?: string };
        const sessionId = payload.id || payload.sessionId;
        if (sessionId) {
          setCurrentSessionId(sessionId);
        }
        break;
      }
      case 'stream': {
        const payload = msg.payload as StreamPayload;
        const messageId = msg.id;
        
        if (!messageId) break;

        // If this is a new streaming message, create it
        if (streamingMessageId.current !== messageId) {
          streamingMessageId.current = messageId;
          setMessages(prev => [
            ...prev,
            {
              id: messageId,
              role: 'assistant',
              content: payload.text,
              timestamp: Date.now(),
              streaming: true,
            },
          ]);
        } else {
          // Append to existing streaming message
          setMessages(prev =>
            prev.map(m =>
              m.id === messageId
                ? { ...m, content: m.content + payload.text }
                : m
            )
          );
        }
        break;
      }
      case 'stream.end': {
        // Mark streaming as complete
        const messageId = msg.id;
        if (messageId) {
          setMessages(prev =>
            prev.map(m =>
              m.id === messageId ? { ...m, streaming: false } : m
            )
          );
          streamingMessageId.current = null;
        }
        break;
      }
      case 'error': {
        const payload = msg.payload as { error: string };
        // Add error as system message
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
        break;
      }
    }
  }, []);

  const { state, send } = useWebSocket({
    url: WS_URL,
    onMessage: handleMessage,
    onConnect: () => {
      // Create a new session on connect
      send({
        type: 'session.create',
        id: generateId(),
        payload: { title: 'New Chat' },
      });
    },
  });

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSend = useCallback((content: string) => {
    if (!currentSessionId || state !== 'connected') return;

    // Add user message
    const userMessage: Message = {
      id: generateId(),
      role: 'user',
      content,
      timestamp: Date.now(),
    };
    setMessages(prev => [...prev, userMessage]);

    // Send to server
    send({
      type: 'prompt',
      id: generateId(),
      payload: {
        sessionId: currentSessionId,
        content,
      },
    });
  }, [currentSessionId, state, send]);

  const isConnected = state === 'connected';
  const isReady = isConnected && currentSessionId;

  return (
    <div className="flex flex-col h-screen bg-[var(--bg-primary)]">
      {/* Header */}
      <header className="safe-area-top glass border-b border-[var(--border-color)] flex-shrink-0">
        <div className="flex items-center justify-between px-4 py-3 max-w-4xl mx-auto">
          <div className="flex items-center gap-3">
            <h1 className="text-lg font-semibold neon-text">OpenVibe</h1>
          </div>
          <StatusIndicator state={state} />
        </div>
      </header>

      {/* Messages area */}
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

      {/* Input bar */}
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
  );
}
