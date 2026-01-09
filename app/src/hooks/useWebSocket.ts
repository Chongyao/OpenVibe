'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { ClientMessage, ConnectionState, ServerMessage } from '@/types';

const RECONNECT_DELAYS = [1000, 2000, 5000, 10000, 30000];

interface UseWebSocketOptions {
  url: string;
  onMessage?: (msg: ServerMessage) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
}

export function useWebSocket({ url, onMessage, onConnect, onDisconnect }: UseWebSocketOptions) {
  const [connectionState, setConnectionState] = useState<ConnectionState>('disconnected');
  const [reconnectTrigger, setReconnectTrigger] = useState(0);
  
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const messageHandlersRef = useRef<Map<string, (msg: ServerMessage) => void>>(new Map());
  const mountedRef = useRef(true);
  
  // Refs for latest callbacks (updated in effect)
  const callbacksRef = useRef({ onMessage, onConnect, onDisconnect });

  useEffect(() => {
    callbacksRef.current = { onMessage, onConnect, onDisconnect };
  }, [onMessage, onConnect, onDisconnect]);

  // Main connection effect
  useEffect(() => {
    mountedRef.current = true;

    // Clear any existing timer
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }

    // Don't connect if already connected
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      return;
    }

    let ws: WebSocket;
    try {
      ws = new WebSocket(url);
    } catch (e) {
      console.error('Failed to create WebSocket:', e);
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
      callbacksRef.current.onConnect?.();
    };

    ws.onclose = () => {
      if (!mountedRef.current) return;

      setConnectionState('disconnected');
      wsRef.current = null;
      callbacksRef.current.onDisconnect?.();

      // Schedule reconnect
      const delay = RECONNECT_DELAYS[Math.min(reconnectAttemptRef.current, RECONNECT_DELAYS.length - 1)];
      reconnectTimerRef.current = setTimeout(() => {
        if (mountedRef.current) {
          reconnectAttemptRef.current++;
          // Trigger reconnection by updating state
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
  }, [url, reconnectTrigger]);

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
