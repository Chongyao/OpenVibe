'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { ClientMessage, ConnectionState, ServerMessage, SyncBatchPayload } from '@/types';
import { generateId } from '@/lib/utils';

const RECONNECT_DELAYS = [1000, 2000, 5000, 10000, 30000];

interface UseWebSocketOptions {
  url: string;
  sessionId?: string;
  onMessage?: (msg: ServerMessage) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
}

export function useWebSocket({ url, sessionId, onMessage, onConnect, onDisconnect }: UseWebSocketOptions) {
  const [connectionState, setConnectionState] = useState<ConnectionState>('disconnected');
  const [reconnectTrigger, setReconnectTrigger] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const messageHandlersRef = useRef<Map<string, (msg: ServerMessage) => void>>(new Map());
  const mountedRef = useRef(true);
  const lastAckIDRef = useRef<number>(0);
  const sessionIdRef = useRef<string | undefined>(sessionId);
  const callbacksRef = useRef({ onMessage, onConnect, onDisconnect });

  useEffect(() => {
    callbacksRef.current = { onMessage, onConnect, onDisconnect };
  }, [onMessage, onConnect, onDisconnect]);

  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  const requestSync = useCallback(() => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    if (!sessionIdRef.current || lastAckIDRef.current === 0) return;

    wsRef.current.send(JSON.stringify({
      type: 'sync',
      id: generateId(),
      payload: {
        sessionId: sessionIdRef.current,
        lastAckId: lastAckIDRef.current,
      },
    }));
  }, []);

  useEffect(() => {
    mountedRef.current = true;

    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }

    if (wsRef.current?.readyState === WebSocket.OPEN) {
      return;
    }

    let ws: WebSocket;
    try {
      ws = new WebSocket(url);
    } catch {
      queueMicrotask(() => {
        if (mountedRef.current) {
          setConnectionState('error');
        }
      });
      return;
    }

    queueMicrotask(() => {
      if (mountedRef.current) {
        setConnectionState('connecting');
      }
    });

    ws.onopen = () => {
      if (!mountedRef.current) {
        ws.close();
        return;
      }
      setConnectionState('connected');
      reconnectAttemptRef.current = 0;

      if (sessionIdRef.current && lastAckIDRef.current > 0) {
        setTimeout(requestSync, 100);
      }

      callbacksRef.current.onConnect?.();
    };

    ws.onclose = () => {
      if (!mountedRef.current) return;

      setConnectionState('disconnected');
      wsRef.current = null;
      callbacksRef.current.onDisconnect?.();

      const delay = RECONNECT_DELAYS[Math.min(reconnectAttemptRef.current, RECONNECT_DELAYS.length - 1)];
      reconnectTimerRef.current = setTimeout(() => {
        if (mountedRef.current) {
          reconnectAttemptRef.current++;
          setReconnectTrigger(t => t + 1);
        }
      }, delay);
    };

    ws.onerror = () => {
      if (!mountedRef.current) return;
      setConnectionState('error');
    };

    ws.onmessage = (event) => {
      if (!mountedRef.current) return;

      try {
        const msg: ServerMessage = JSON.parse(event.data);

        if (msg.msgId && msg.msgId > lastAckIDRef.current) {
          lastAckIDRef.current = msg.msgId;
          ws.send(JSON.stringify({
            type: 'ack',
            id: generateId(),
            payload: { msgId: msg.msgId },
          }));
        }

        if (msg.type === 'sync.batch') {
          const payload = msg.payload as SyncBatchPayload;
          if (payload.latestId) {
            lastAckIDRef.current = payload.latestId;
          }
          for (const bufferedMsg of payload.messages) {
            const syntheticMsg: ServerMessage = {
              type: bufferedMsg.type as ServerMessage['type'],
              id: bufferedMsg.requestId,
              msgId: bufferedMsg.id,
              payload: bufferedMsg.payload,
            };
            callbacksRef.current.onMessage?.(syntheticMsg);
          }
          return;
        }

        const handler = messageHandlersRef.current.get(msg.id ?? '');
        if (handler) {
          handler(msg);
          if (msg.type !== 'stream') {
            messageHandlersRef.current.delete(msg.id ?? '');
          }
        }

        callbacksRef.current.onMessage?.(msg);
      } catch {
        // Parse error - ignore malformed messages
      }
    };

    wsRef.current = ws;

    return () => {
      mountedRef.current = false;
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      ws.close();
      wsRef.current = null;
    };
  }, [url, reconnectTrigger, requestSync]);

  const disconnect = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    wsRef.current?.close();
    wsRef.current = null;
    setConnectionState('disconnected');
  }, []);

  const send = useCallback((msg: ClientMessage, handler?: (response: ServerMessage) => void) => {
    if (wsRef.current?.readyState !== WebSocket.OPEN) {
      return false;
    }

    if (handler) {
      messageHandlersRef.current.set(msg.id, handler);
    }

    wsRef.current.send(JSON.stringify(msg));
    return true;
  }, []);

  const connect = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    reconnectAttemptRef.current = 0;
    setReconnectTrigger(t => t + 1);
  }, []);

  return { state: connectionState, send, connect, disconnect };
}
