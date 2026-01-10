export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: number;
  streaming?: boolean;
  msgId?: number; // Buffer message ID for sync
}

export interface Session {
  id: string;
  title: string;
  createdAt: number;
  messages: Message[];
  directory?: string;
  time?: { created: number; updated: number };
}

export interface ClientMessage {
  type: 'ping' | 'session.create' | 'session.list' | 'prompt' | 'sync' | 'ack';
  id: string;
  payload: {
    sessionId?: string;
    content?: string;
    title?: string;
    lastAckId?: number;
    msgId?: number;
  };
}

export interface ServerMessage {
  type: 'pong' | 'response' | 'stream' | 'stream.end' | 'error' | 'sync.batch';
  id?: string;
  msgId?: number;
  payload: unknown;
}

export interface StreamPayload {
  text: string;
}

export interface ErrorPayload {
  error: string;
}

export interface SyncBatchPayload {
  messages: BufferedMessage[];
  latestId: number;
}

export interface BufferedMessage {
  id: number;
  type: string;
  requestId: string;
  payload: unknown;
  timestamp: number;
}

export type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';

export type Theme = 'dark' | 'light' | 'system';

export interface Project {
  path: string;
  name: string;
  sessionCount: number;
  lastUpdated: number;
}

export interface AppSettings {
  theme: Theme;
  showTimestamps: boolean;
  enableHaptics: boolean;
  serverUrl: string;
}

export const DEFAULT_SETTINGS: AppSettings = {
  theme: 'dark',
  showTimestamps: true,
  enableHaptics: true,
  serverUrl: '',
};
