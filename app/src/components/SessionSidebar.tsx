'use client';

import { memo, useCallback, useState } from 'react';
import type { Session } from '@/types';
import { PlusIcon, ChatIcon, TrashIcon, MenuIcon, CloseIcon } from './Icons';

interface SessionSidebarProps {
  sessions: Session[];
  currentSessionId: string | null;
  onSelectSession: (sessionId: string) => void;
  onNewSession: () => void;
  onDeleteSession: (sessionId: string) => void;
  isLoading?: boolean;
}

function formatDate(timestamp: number): string {
  const date = new Date(timestamp);
  const now = new Date();
  const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24));
  
  if (diffDays === 0) {
    return 'Today';
  } else if (diffDays === 1) {
    return 'Yesterday';
  } else if (diffDays < 7) {
    return date.toLocaleDateString([], { weekday: 'long' });
  } else {
    return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
  }
}

function SessionItem({ 
  session, 
  isActive, 
  onSelect, 
  onDelete 
}: { 
  session: Session; 
  isActive: boolean; 
  onSelect: () => void;
  onDelete: () => void;
}) {
  const [showDelete, setShowDelete] = useState(false);

  const handleDelete = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    onDelete();
  }, [onDelete]);

  return (
    <div
      className={`session-item ${isActive ? 'session-item-active' : ''}`}
      onClick={onSelect}
      onMouseEnter={() => setShowDelete(true)}
      onMouseLeave={() => setShowDelete(false)}
    >
      <div className="flex items-start gap-3 flex-1 min-w-0">
        <ChatIcon className="w-4 h-4 mt-0.5 text-[var(--text-muted)] flex-shrink-0" />
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium text-[var(--text-primary)] truncate">
            {session.title || 'New Chat'}
          </div>
          <div className="text-xs text-[var(--text-muted)] mt-0.5">
            {formatDate(session.createdAt)}
          </div>
        </div>
      </div>
      {showDelete && (
        <button
          onClick={handleDelete}
          className="p-1 text-[var(--text-muted)] hover:text-[var(--accent-error)] transition-colors"
          aria-label="Delete session"
        >
          <TrashIcon className="w-4 h-4" />
        </button>
      )}
    </div>
  );
}

export const SessionSidebar = memo(function SessionSidebar({
  sessions,
  currentSessionId,
  onSelectSession,
  onNewSession,
  onDeleteSession,
  isLoading = false,
}: SessionSidebarProps) {
  const [isOpen, setIsOpen] = useState(false);

  const toggleSidebar = useCallback(() => {
    setIsOpen(prev => !prev);
  }, []);

  const handleSelectSession = useCallback((sessionId: string) => {
    onSelectSession(sessionId);
    setIsOpen(false);
  }, [onSelectSession]);

  const handleNewSession = useCallback(() => {
    onNewSession();
    setIsOpen(false);
  }, [onNewSession]);

  return (
    <>
      <button
        onClick={toggleSidebar}
        className="sidebar-toggle md:hidden"
        aria-label="Toggle sidebar"
      >
        <MenuIcon className="w-5 h-5" />
      </button>

      {isOpen && (
        <div 
          className="sidebar-overlay md:hidden" 
          onClick={() => setIsOpen(false)}
        />
      )}

      <aside className={`sidebar ${isOpen ? 'sidebar-open' : ''}`}>
        <div className="sidebar-header">
          <h2 className="text-sm font-semibold text-[var(--text-primary)]">Chat History</h2>
          <button
            onClick={() => setIsOpen(false)}
            className="md:hidden p-1 text-[var(--text-muted)] hover:text-[var(--text-primary)]"
          >
            <CloseIcon className="w-5 h-5" />
          </button>
        </div>

        <button
          onClick={handleNewSession}
          className="new-session-btn"
          disabled={isLoading}
        >
          <PlusIcon className="w-4 h-4" />
          <span>New Chat</span>
        </button>

        <div className="session-list">
          {sessions.length === 0 ? (
            <div className="text-center text-[var(--text-muted)] text-sm py-8">
              No chat history yet
            </div>
          ) : (
            sessions.map(session => (
              <SessionItem
                key={session.id}
                session={session}
                isActive={session.id === currentSessionId}
                onSelect={() => handleSelectSession(session.id)}
                onDelete={() => onDeleteSession(session.id)}
              />
            ))
          )}
        </div>
      </aside>
    </>
  );
});
