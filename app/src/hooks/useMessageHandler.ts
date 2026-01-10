'use client';

import { useCallback, useRef } from 'react';
import type { Message, Session, ServerMessage, StreamPayload } from '@/types';
import { generateId } from '@/lib/utils';

interface OpenCodeMessage {
  info: { id: string; role: 'user' | 'assistant'; time?: { created: number } };
  parts: Array<{ type: string; text?: string }>;
}

interface ServerSession {
  id: string;
  title: string;
  directory?: string;
  time?: { created: number; updated: number };
}

interface UseMessageHandlerOptions {
  currentSessionId: string | null;
  isCreatingSession: boolean;
  setMessages: React.Dispatch<React.SetStateAction<Message[]>>;
  setSessions: (sessions: Session[]) => void;
  addSession: (session: Session) => void;
  updateSessionMessages: (sessionId: string, msgs: Message[]) => void;
  setCurrentSessionId: (id: string | null) => void;
  setIsCreatingSession: (creating: boolean) => void;
  onSessionCreated?: () => void;
  onError?: (error: string) => void;
}

function convertOpenCodeMessages(ocMessages: OpenCodeMessage[]): Message[] {
  const result: Message[] = [];
  for (const ocMsg of ocMessages) {
    const textParts = ocMsg.parts.filter(p => p.type === 'text' && p.text);
    if (textParts.length > 0) {
      result.push({
        id: ocMsg.info.id,
        role: ocMsg.info.role,
        content: textParts.map(p => p.text).join('\n'),
        timestamp: ocMsg.info.time?.created || Date.now(),
      });
    }
  }
  return result;
}

function mapServerSessions(serverSessions: ServerSession[]): Session[] {
  return serverSessions.map(s => ({
    id: s.id,
    title: s.title || 'New Chat',
    createdAt: s.time?.created || Date.now(),
    messages: [],
    directory: s.directory,
    time: s.time,
  }));
}

export function useMessageHandler(options: UseMessageHandlerOptions) {
  const {
    currentSessionId,
    isCreatingSession,
    setMessages,
    setSessions,
    addSession,
    updateSessionMessages,
    setCurrentSessionId,
    setIsCreatingSession,
    onSessionCreated,
    onError,
  } = options;

  const streamingMessageId = useRef<string | null>(null);
  const pendingRequest = useRef<{ requestId: string; sessionId: string } | null>(null);

  const setPendingRequest = useCallback((requestId: string, sessionId: string) => {
    pendingRequest.current = { requestId, sessionId };
  }, []);

  const handleMessage = useCallback((msg: ServerMessage) => {
    switch (msg.type) {
      case 'response': {
        const payload = msg.payload;

        if (Array.isArray(payload)) {
          const pending = pendingRequest.current;
          if (pending && msg.id === pending.requestId) {
            const firstItem = payload[0];
            const isMessageResponse = !firstItem || ('info' in firstItem && 'parts' in firstItem);

            if (isMessageResponse) {
              const converted = convertOpenCodeMessages(payload as OpenCodeMessage[]);
              if (pending.sessionId === currentSessionId) {
                setMessages(converted);
                updateSessionMessages(pending.sessionId, converted);
              }
              pendingRequest.current = null;
              return;
            }
          }

          const firstItem = payload[0];
          if (firstItem && 'id' in firstItem) {
            const mapped = mapServerSessions(payload as ServerSession[]);
            setSessions(mapped);
            if (mapped.length > 0 && !currentSessionId) {
              setCurrentSessionId(mapped[0].id);
            }
          }
          return;
        }

        const responsePayload = payload as { id?: string; sessionId?: string; title?: string; directory?: string };
        const sessionId = responsePayload.id || responsePayload.sessionId;
        if (sessionId && isCreatingSession) {
          addSession({
            id: sessionId,
            title: responsePayload.title || 'New Chat',
            createdAt: Date.now(),
            messages: [],
            directory: responsePayload.directory,
          });
          setCurrentSessionId(sessionId);
          setMessages([]);
          setIsCreatingSession(false);
          onSessionCreated?.();
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
              m.id === messageId ? { ...m, content: m.content + payload.text } : m
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
        onError?.(payload.error);
        break;
      }
    }
  }, [
    currentSessionId,
    isCreatingSession,
    setMessages,
    setSessions,
    addSession,
    updateSessionMessages,
    setCurrentSessionId,
    setIsCreatingSession,
    onSessionCreated,
    onError,
  ]);

  return { handleMessage, setPendingRequest };
}
