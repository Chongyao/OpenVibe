'use client';

import { memo, useMemo, useCallback, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import type { Message } from '@/types';
import { ErrorCard, parseErrors } from './ErrorCard';

interface MessageBubbleProps {
  message: Message;
}

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  });
}

function CodeBlock({ 
  language, 
  children 
}: { 
  language: string; 
  children: string;
}) {
  const [copied, setCopied] = useState(false);
  
  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(children);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [children]);

  return (
    <div className="code-block my-3 overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2 bg-[var(--bg-tertiary)] border-b border-[var(--border-color)]">
        <span className="text-xs text-[var(--text-muted)] font-mono">{language || 'text'}</span>
        <button
          onClick={handleCopy}
          className="text-xs text-[var(--text-secondary)] hover:text-[var(--accent-primary)] transition-colors"
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>
      <SyntaxHighlighter
        style={oneDark}
        language={language || 'text'}
        PreTag="div"
        customStyle={{
          margin: 0,
          padding: '1rem',
          background: 'var(--bg-primary)',
          fontSize: '0.875rem',
        }}
      >
        {children}
      </SyntaxHighlighter>
    </div>
  );
}

function renderContentWithErrors(content: string): React.ReactNode[] {
  const errors = parseErrors(content);
  
  if (errors.length === 0) {
    return [
      <ReactMarkdown key="content" components={markdownComponents}>
        {content}
      </ReactMarkdown>
    ];
  }

  const parts: React.ReactNode[] = [];
  let lastIndex = 0;

  for (let i = 0; i < errors.length; i++) {
    const error = errors[i];
    
    if (error.startIndex > lastIndex) {
      const textBefore = content.slice(lastIndex, error.startIndex);
      if (textBefore.trim()) {
        parts.push(
          <ReactMarkdown key={`text-${i}`} components={markdownComponents}>
            {textBefore}
          </ReactMarkdown>
        );
      }
    }

    parts.push(
      <ErrorCard
        key={`error-${i}`}
        errorType={error.type}
        errorMessage={error.message}
        fullTraceback={error.fullMatch}
      />
    );

    lastIndex = error.endIndex;
  }

  if (lastIndex < content.length) {
    const textAfter = content.slice(lastIndex);
    if (textAfter.trim()) {
      parts.push(
        <ReactMarkdown key="text-end" components={markdownComponents}>
          {textAfter}
        </ReactMarkdown>
      );
    }
  }

  return parts;
}

const markdownComponents = {
  code({ className, children, ...props }: { className?: string; children?: React.ReactNode }) {
    const match = /language-(\w+)/.exec(className || '');
    const isInline = !match && !className;
    const codeString = String(children).replace(/\n$/, '');
    
    if (isInline) {
      return (
        <code className="inline-code" {...props}>
          {children}
        </code>
      );
    }
    
    return (
      <CodeBlock language={match?.[1] || ''}>
        {codeString}
      </CodeBlock>
    );
  },
  h1: ({ children }: { children?: React.ReactNode }) => <h1 className="markdown-h1">{children}</h1>,
  h2: ({ children }: { children?: React.ReactNode }) => <h2 className="markdown-h2">{children}</h2>,
  h3: ({ children }: { children?: React.ReactNode }) => <h3 className="markdown-h3">{children}</h3>,
  h4: ({ children }: { children?: React.ReactNode }) => <h4 className="markdown-h4">{children}</h4>,
  h5: ({ children }: { children?: React.ReactNode }) => <h5 className="markdown-h5">{children}</h5>,
  h6: ({ children }: { children?: React.ReactNode }) => <h6 className="markdown-h6">{children}</h6>,
  ul: ({ children }: { children?: React.ReactNode }) => <ul className="markdown-ul">{children}</ul>,
  ol: ({ children }: { children?: React.ReactNode }) => <ol className="markdown-ol">{children}</ol>,
  li: ({ children }: { children?: React.ReactNode }) => <li className="markdown-li">{children}</li>,
  blockquote: ({ children }: { children?: React.ReactNode }) => <blockquote className="markdown-blockquote">{children}</blockquote>,
  hr: () => <hr className="markdown-hr" />,
  a: ({ href, children }: { href?: string; children?: React.ReactNode }) => (
    <a href={href} target="_blank" rel="noopener noreferrer" className="markdown-link">
      {children}
    </a>
  ),
  strong: ({ children }: { children?: React.ReactNode }) => <strong className="markdown-strong">{children}</strong>,
  em: ({ children }: { children?: React.ReactNode }) => <em className="markdown-em">{children}</em>,
  p: ({ children }: { children?: React.ReactNode }) => <p className="markdown-p">{children}</p>,
};

export const MessageBubble = memo(function MessageBubble({ message }: MessageBubbleProps) {
  const isUser = message.role === 'user';
  const isStreaming = message.streaming && message.role === 'assistant';

  const content = useMemo(() => {
    if (isUser) {
      return <span>{message.content}</span>;
    }
    
    return renderContentWithErrors(message.content);
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
        <div className={`text-sm sm:text-base leading-relaxed break-words ${isStreaming ? 'typing-cursor' : ''} ${!isUser ? 'markdown-content' : ''}`}>
          {content}
        </div>

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
