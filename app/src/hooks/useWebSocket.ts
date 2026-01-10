'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { ClientMessage, ConnectionState, ServerMessage, SyncBatchPayload } from '@/types';

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
  
  // Refs for latest callbacks (updated in effect)
  const callbacksRef = useRef({ onMessage, onConnect, onDisconnect });

  useEffect(() => {
    callbacksRef.current = { onMessage, onConnect, onDisconnect };
  }, [onMessage, onConnect, onDisconnect]);

  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  // Request sync after reconnection
  const requestSync = useCallback(() => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;
    if (!sessionIdRef.current || lastAckIDRef.current === 0) return;

    const syncMsg: ClientMessage = {
      type: 'sync',
      id: crypto.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`,
      payload: {
        sessionId: sessionIdRef.current,
        lastAckId: lastAckIDRef.current,
      },
    };

    wsRef.current.send(JSON.stringify(syncMsg));
  }, []);

  // Main connection effect
  useEffect(() => {
    mountedRef.current = true;
    console.log('[WS] Effect triggered, url:', url, 'reconnectTrigger:', reconnectTrigger);

    // Clear any existing timer
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }

    // Don't connect if already connected
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      console.log('[WS] Already connected, skipping');
      return;
    }

    let ws: WebSocket;
    try {
      console.log('[WS] Creating WebSocket to:', url);
      ws = new WebSocket(url);
      console.log('[WS] WebSocket created, readyState:', ws.readyState);
    } catch (e) {
      console.error('[WS] Failed to create WebSocket:', e);
      // Use microtask to avoid synchronous setState warning
      queueMicrotask(() => {
        if (mountedRef.current) {
          setConnectionState('error');
        }
      });
      return;
    }

    // Set connecting state via microtask
    queueMicrotask(() => {
      if (mountedRef.current) {
        console.log('[WS] Setting state to connecting');
        setConnectionState('connecting');
      }
    });

    ws.onopen = () => {
      console.log('[WS] onopen fired, mounted:', mountedRef.current);
      if (!mountedRef.current) {
        ws.close();
        return;
      }
      console.log('[WS] Connected successfully!');
      setConnectionState('connected');
      reconnectAttemptRef.current = 0;

      // If reconnecting with existing session, request sync
      if (sessionIdRef.current && lastAckIDRef.current > 0) {
        setTimeout(requestSync, 100);
      }

      callbacksRef.current.onConnect?.();
    };

    ws.onclose = (event) => {
      console.log('[WS] onclose fired, code:', event.code, 'reason:', event.reason, 'wasClean:', event.wasClean);
      if (!mountedRef.current) return;

      setConnectionState('disconnected');
      wsRef.current = null;
      callbacksRef.current.onDisconnect?.();

      // Schedule reconnect
      const delay = RECONNECT_DELAYS[Math.min(reconnectAttemptRef.current, RECONNECT_DELAYS.length - 1)];
      console.log('[WS] Scheduling reconnect in', delay, 'ms, attempt:', reconnectAttemptRef.current);
      reconnectTimerRef.current = setTimeout(() => {
        if (mountedRef.current) {
          reconnectAttemptRef.current++;
          // Trigger reconnection by updating state
          setReconnectTrigger(t => t + 1);
        }
      }, delay);
    };

    ws.onerror = (event) => {
      console.error('[WS] onerror fired:', event);
      if (!mountedRef.current) return;
      setConnectionState('error');
    };

    ws.onmessage = (event) => {
      if (!mountedRef.current) return;
      console.log('[WS] Message received:', event.data.substring(0, 200));

      try {
        const msg: ServerMessage = JSON.parse(event.data);

        // Track message IDs for sync
        if (msg.msgId && msg.msgId > lastAckIDRef.current) {
          lastAckIDRef.current = msg.msgId;
          // Send ack
          ws.send(JSON.stringify({
            type: 'ack',
            id: crypto.randomUUID?.() || `${Date.now()}`,
            payload: { msgId: msg.msgId },
          }));
        }

        // Handle sync.batch specially
        if (msg.type === 'sync.batch') {
          const payload = msg.payload as SyncBatchPayload;
          if (payload.latestId) {
            lastAckIDRef.current = payload.latestId;
          }
          // Process buffered messages
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
      } catch (e) {
        console.error('Failed to parse message:', e);
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
      console.error('WebSocket not connected');
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
