'use client';

import { memo, useMemo } from 'react';
import type { Message } from '@/types';

interface MessageBubbleProps {
  message: Message;
}

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  });
}

function parseContent(content: string): React.ReactNode[] {
  const parts: React.ReactNode[] = [];
  const codeBlockRegex = /```(\w*)\n?([\s\S]*?)```/g;
  let lastIndex = 0;
  let match;
  let key = 0;

  while ((match = codeBlockRegex.exec(content)) !== null) {
    // Add text before code block
    if (match.index > lastIndex) {
      parts.push(
        <span key={key++}>
          {content.slice(lastIndex, match.index)}
        </span>
      );
    }

    // Add code block
    const language = match[1] || 'text';
    const code = match[2].trim();
    parts.push(
      <div key={key++} className="code-block my-3 overflow-hidden">
        <div className="flex items-center justify-between px-4 py-2 bg-[var(--bg-tertiary)] border-b border-[var(--border-color)]">
          <span className="text-xs text-[var(--text-muted)] font-mono">{language}</span>
          <button
            onClick={() => navigator.clipboard.writeText(code)}
            className="text-xs text-[var(--text-secondary)] hover:text-[var(--accent-primary)] transition-colors"
          >
            Copy
          </button>
        </div>
        <pre className="p-4 overflow-x-auto">
          <code className="text-sm text-[var(--text-secondary)]">{code}</code>
        </pre>
      </div>
    );

    lastIndex = match.index + match[0].length;
  }

  // Add remaining text
  if (lastIndex < content.length) {
    parts.push(
      <span key={key++}>
        {content.slice(lastIndex)}
      </span>
    );
  }

  return parts.length > 0 ? parts : [<span key={0}>{content}</span>];
}

export const MessageBubble = memo(function MessageBubble({ message }: MessageBubbleProps) {
  const isUser = message.role === 'user';
  const isStreaming = message.streaming && message.role === 'assistant';

  const parsedContent = useMemo(() => {
    if (isUser) {
      return <span>{message.content}</span>;
    }
    return parseContent(message.content);
  }, [message.content, isUser]);

  return (
    <div
      className={`flex animate-fade-in ${isUser ? 'justify-end' : 'justify-start'}`}
    >
      <div
        className={`max-w-[85%] sm:max-w-[75%] rounded-2xl px-4 py-3 ${
          isUser
            ? 'message-user rounded-br-md'
            : 'message-assistant rounded-bl-md'
        }`}
      >
        {/* Message content */}
        <div className={`text-sm sm:text-base leading-relaxed whitespace-pre-wrap break-words ${isStreaming ? 'typing-cursor' : ''}`}>
          {parsedContent}
        </div>

        {/* Timestamp */}
        <div
          className={`text-xs mt-2 ${
            isUser ? 'text-[var(--bg-tertiary)]' : 'text-[var(--text-muted)]'
          }`}
        >
          {formatTime(message.timestamp)}
        </div>
      </div>
    </div>
  );
});
