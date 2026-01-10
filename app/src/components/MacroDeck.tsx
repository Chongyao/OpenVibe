'use client';

import { memo, useMemo } from 'react';

export type MacroAction = 
  | 'approve'
  | 'reject'
  | 'retry'
  | 'stop'
  | 'continue'
  | 'explain'
  | 'simplify'
  | 'test';

interface MacroButton {
  action: MacroAction;
  label: string;
  icon: string;
  shortcut?: string;
}

const MACROS: MacroButton[] = [
  { action: 'approve', label: 'Yes', icon: '✓', shortcut: 'y' },
  { action: 'reject', label: 'No', icon: '✗', shortcut: 'n' },
  { action: 'continue', label: 'Continue', icon: '→' },
  { action: 'explain', label: 'Explain', icon: '?' },
  { action: 'simplify', label: 'Simplify', icon: '◇' },
  { action: 'test', label: 'Test', icon: '▶' },
];

interface MacroDeckProps {
  onAction: (action: MacroAction, prompt: string) => void;
  disabled?: boolean;
  isStreaming?: boolean;
}

const actionPrompts: Record<MacroAction, string> = {
  approve: 'Yes, proceed.',
  reject: 'No, do not proceed.',
  retry: 'Please try again.',
  stop: '/stop',
  continue: 'Please continue.',
  explain: 'Can you explain that in more detail?',
  simplify: 'Can you simplify that?',
  test: 'Please write tests for this.',
};

export const MacroDeck = memo(function MacroDeck({ 
  onAction, 
  disabled = false,
  isStreaming = false,
}: MacroDeckProps) {
  const activeMacros = useMemo(() => {
    if (isStreaming) {
      return [{ action: 'stop' as MacroAction, label: 'Stop', icon: '⏹' }];
    }
    return MACROS;
  }, [isStreaming]);

  return (
    <div className="macro-deck">
      <div className="macro-deck-scroll">
        {activeMacros.map(macro => (
          <button
            key={macro.action}
            onClick={() => onAction(macro.action, actionPrompts[macro.action])}
            disabled={disabled}
            className={`macro-btn ${macro.action === 'stop' ? 'macro-btn-danger' : ''}`}
            title={macro.shortcut ? `${macro.label} (${macro.shortcut})` : macro.label}
          >
            <span className="macro-icon">{macro.icon}</span>
            <span className="macro-label">{macro.label}</span>
          </button>
        ))}
      </div>
    </div>
  );
});
