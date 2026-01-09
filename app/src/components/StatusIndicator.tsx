'use client';

import { memo } from 'react';
import type { ConnectionState } from '@/types';

interface StatusIndicatorProps {
  state: ConnectionState;
}

const stateLabels: Record<ConnectionState, string> = {
  connecting: 'Connecting...',
  connected: 'Connected',
  disconnected: 'Disconnected',
  error: 'Connection Error',
};

export const StatusIndicator = memo(function StatusIndicator({ state }: StatusIndicatorProps) {
  return (
    <div className="flex items-center gap-2">
      <div className={`status-dot status-${state}`} />
      <span className="text-xs text-[var(--text-secondary)]">
        {stateLabels[state]}
      </span>
    </div>
  );
});
