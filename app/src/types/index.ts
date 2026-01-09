export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: number;
  streaming?: boolean;
}

export interface Session {
  id: string;
  title: string;
  createdAt: number;
}

export interface ClientMessage {
  type: 'ping' | 'session.create' | 'session.list' | 'prompt';
  id: string;
  payload: {
    sessionId?: string;
    content?: string;
    title?: string;
  };
}

export interface ServerMessage {
  type: 'pong' | 'response' | 'stream' | 'stream.end' | 'error';
  id?: string;
  payload: unknown;
}

export interface StreamPayload {
  text: string;
}

export interface ErrorPayload {
  error: string;
}

export type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';
