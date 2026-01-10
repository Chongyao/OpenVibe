'use client';

import { memo, useState, useCallback } from 'react';
import { useSettings } from '@/hooks/useSettings';
import type { Theme } from '@/types';
import { SettingsIcon, CloseIcon } from './Icons';

const themeOptions: { value: Theme; label: string }[] = [
  { value: 'dark', label: 'Dark' },
  { value: 'light', label: 'Light' },
  { value: 'system', label: 'System' },
];

export const SettingsPanel = memo(function SettingsPanel() {
  const [isOpen, setIsOpen] = useState(false);
  const { settings, updateSettings, resetSettings } = useSettings();

  const toggleOpen = useCallback(() => {
    setIsOpen(prev => !prev);
  }, []);

  const handleThemeChange = useCallback((theme: Theme) => {
    updateSettings({ theme });
  }, [updateSettings]);

  const handleToggleTimestamps = useCallback(() => {
    updateSettings({ showTimestamps: !settings.showTimestamps });
  }, [settings.showTimestamps, updateSettings]);

  const handleToggleHaptics = useCallback(() => {
    updateSettings({ enableHaptics: !settings.enableHaptics });
  }, [settings.enableHaptics, updateSettings]);

  return (
    <>
      <button
        onClick={toggleOpen}
        className="settings-toggle"
        aria-label="Open settings"
      >
        <SettingsIcon className="w-5 h-5" />
      </button>

      {isOpen && (
        <>
          <div 
            className="settings-overlay" 
            onClick={() => setIsOpen(false)}
          />
          <div className="settings-panel">
            <div className="settings-header">
              <h2 className="text-lg font-semibold text-[var(--text-primary)]">Settings</h2>
              <button
                onClick={() => setIsOpen(false)}
                className="p-1 text-[var(--text-muted)] hover:text-[var(--text-primary)]"
              >
                <CloseIcon className="w-5 h-5" />
              </button>
            </div>

            <div className="settings-content">
              <div className="settings-section">
                <h3 className="settings-section-title">Appearance</h3>
                
                <div className="settings-item">
                  <span className="settings-label">Theme</span>
                  <div className="theme-selector">
                    {themeOptions.map(option => (
                      <button
                        key={option.value}
                        onClick={() => handleThemeChange(option.value)}
                        className={`theme-option ${settings.theme === option.value ? 'theme-option-active' : ''}`}
                      >
                        {option.label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="settings-item">
                  <span className="settings-label">Show timestamps</span>
                  <button
                    onClick={handleToggleTimestamps}
                    className={`toggle-switch ${settings.showTimestamps ? 'toggle-switch-on' : ''}`}
                    aria-pressed={settings.showTimestamps}
                  >
                    <span className="toggle-switch-thumb" />
                  </button>
                </div>
              </div>

              <div className="settings-section">
                <h3 className="settings-section-title">Interaction</h3>
                
                <div className="settings-item">
                  <span className="settings-label">Haptic feedback</span>
                  <button
                    onClick={handleToggleHaptics}
                    className={`toggle-switch ${settings.enableHaptics ? 'toggle-switch-on' : ''}`}
                    aria-pressed={settings.enableHaptics}
                  >
                    <span className="toggle-switch-thumb" />
                  </button>
                </div>
              </div>

              <div className="settings-section">
                <button
                  onClick={resetSettings}
                  className="reset-settings-btn"
                >
                  Reset to defaults
                </button>
              </div>
            </div>
          </div>
        </>
      )}
    </>
  );
});
