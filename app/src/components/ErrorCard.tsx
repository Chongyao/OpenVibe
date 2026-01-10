'use client';

import { memo, useState, useCallback } from 'react';

interface ErrorCardProps {
  errorType: string;
  errorMessage: string;
  fullTraceback: string;
}

export const ErrorCard = memo(function ErrorCard({ 
  errorType, 
  errorMessage, 
  fullTraceback 
}: ErrorCardProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  const toggleExpand = useCallback(() => {
    setIsExpanded(prev => !prev);
  }, []);

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(fullTraceback);
  }, [fullTraceback]);

  return (
    <div className="error-card my-3">
      <button
        className="error-card-header"
        onClick={toggleExpand}
        aria-expanded={isExpanded}
      >
        <div className="flex items-center gap-2">
          <span className="error-icon">🚨</span>
          <span className="error-type">{errorType}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="error-message">{errorMessage}</span>
          <span className={`error-chevron ${isExpanded ? 'error-chevron-up' : ''}`}>
            ▼
          </span>
        </div>
      </button>
      
      {isExpanded && (
        <div className="error-card-body">
          <div className="error-card-actions">
            <button onClick={handleCopy} className="error-copy-btn">
              Copy
            </button>
          </div>
          <pre className="error-traceback">
            <code>{fullTraceback}</code>
          </pre>
        </div>
      )}
    </div>
  );
});

const TRACEBACK_PATTERNS = [
  {
    regex: /Traceback \(most recent call last\):([\s\S]*?)(\w+Error|\w+Exception): (.+?)(?=\n\n|\n$|$)/g,
    getType: (match: RegExpExecArray) => match[2],
    getMessage: (match: RegExpExecArray) => match[3],
  },
  {
    regex: /error\[E\d+\]:([\s\S]*?)(?=\n\n|\nerror\[|\n$|$)/gi,
    getType: () => 'Rust Error',
    getMessage: (match: RegExpExecArray) => match[1].split('\n')[0].trim(),
  },
  {
    regex: /^Error: (.+)$/gm,
    getType: () => 'Error',
    getMessage: (match: RegExpExecArray) => match[1],
  },
  {
    regex: /panic: (.+)\n(goroutine[\s\S]*?)(?=\n\n|$)/g,
    getType: () => 'Go Panic',
    getMessage: (match: RegExpExecArray) => match[1],
  },
];

export interface ParsedError {
  type: string;
  message: string;
  fullMatch: string;
  startIndex: number;
  endIndex: number;
}

export function parseErrors(content: string): ParsedError[] {
  const errors: ParsedError[] = [];
  
  for (const pattern of TRACEBACK_PATTERNS) {
    let match;
    const regex = new RegExp(pattern.regex.source, pattern.regex.flags);
    
    while ((match = regex.exec(content)) !== null) {
      errors.push({
        type: pattern.getType(match),
        message: pattern.getMessage(match).substring(0, 100),
        fullMatch: match[0],
        startIndex: match.index,
        endIndex: match.index + match[0].length,
      });
    }
  }
  
  errors.sort((a, b) => a.startIndex - b.startIndex);
  
  const nonOverlapping: ParsedError[] = [];
  let lastEnd = -1;
  for (const error of errors) {
    if (error.startIndex >= lastEnd) {
      nonOverlapping.push(error);
      lastEnd = error.endIndex;
    }
  }
  
  return nonOverlapping;
}
