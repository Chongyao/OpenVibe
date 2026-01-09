'use client';

import { memo, useState, useRef, useCallback, KeyboardEvent, ChangeEvent } from 'react';

interface InputBarProps {
  onSend: (content: string) => void;
  disabled?: boolean;
  placeholder?: string;
}

export const InputBar = memo(function InputBar({
  onSend,
  disabled = false,
  placeholder = "Type your message...",
}: InputBarProps) {
  const [value, setValue] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const handleChange = useCallback((e: ChangeEvent<HTMLTextAreaElement>) => {
    setValue(e.target.value);
    
    // Auto-resize textarea
    const textarea = textareaRef.current;
    if (textarea) {
      textarea.style.height = 'auto';
      textarea.style.height = `${Math.min(textarea.scrollHeight, 150)}px`;
    }
  }, []);

  const handleSend = useCallback(() => {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    
    onSend(trimmed);
    setValue('');
    
    // Reset textarea height
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  }, [value, disabled, onSend]);

  const handleKeyDown = useCallback((e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }, [handleSend]);

  return (
    <div className="safe-area-bottom bg-[var(--bg-secondary)] border-t border-[var(--border-color)]">
      <div className="flex items-end gap-3 p-4 max-w-4xl mx-auto">
        <div className="flex-1 relative">
          <textarea
            ref={textareaRef}
            value={value}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            placeholder={placeholder}
            disabled={disabled}
            rows={1}
            className="input-cyber w-full px-4 py-3 rounded-2xl resize-none text-sm sm:text-base focus:outline-none disabled:opacity-50"
            style={{ minHeight: '48px', maxHeight: '150px' }}
          />
        </div>
        <button
          onClick={handleSend}
          disabled={disabled || !value.trim()}
          className="btn-cyber h-12 w-12 rounded-full flex items-center justify-center flex-shrink-0"
          aria-label="Send message"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="currentColor"
            className="w-5 h-5"
          >
            <path d="M3.478 2.404a.75.75 0 0 0-.926.941l2.432 7.905H13.5a.75.75 0 0 1 0 1.5H4.984l-2.432 7.905a.75.75 0 0 0 .926.94 60.519 60.519 0 0 0 18.445-8.986.75.75 0 0 0 0-1.218A60.517 60.517 0 0 0 3.478 2.404Z" />
          </svg>
        </button>
      </div>
    </div>
  );
});
