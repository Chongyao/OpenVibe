# AGENTS.md - React Components

UI components for the mobile terminal interface.

## Overview

| Component | Purpose | Key Props |
|-----------|---------|-----------|
| `MessageBubble` | Chat message with markdown | `message: Message` |
| `InputBar` | Auto-resize textarea + send | `onSend`, `disabled` |
| `SessionSidebar` | Session list + new/delete | `sessions`, `currentId` |
| `SettingsPanel` | Theme, server config | via `useSettings` |
| `StatusIndicator` | Connection state badge | `state: ConnectionState` |
| `ProjectSelector` | Multi-project picker | `projects`, `onSelect` |
| `MacroDeck` | Quick action buttons | `actions: MacroAction[]` |
| `Toast` | Notification system | `ToastProvider` context |
| `ErrorCard` | Error traceback display | `errorType`, `message` |
| `Icons` | SVG icon components | various |

## MessageBubble

Renders user/assistant messages with full markdown support.

### Features
- Syntax highlighting via `react-syntax-highlighter` (oneDark theme)
- Copy button for code blocks
- Error traceback detection and `ErrorCard` rendering
- Typing cursor animation for streaming

### Key Implementation

```typescript
// memo() for performance - prevents re-render on parent updates
export const MessageBubble = memo(function MessageBubble({ message }: Props) {
  const content = useMemo(() => {
    if (message.role === 'user') return <span>{message.content}</span>;
    return renderContentWithErrors(message.content);
  }, [message.content, message.role]);
  // ...
});
```

### CSS Classes
- `.message-user` - User bubble (right-aligned, accent color)
- `.message-assistant` - Assistant bubble (left-aligned, secondary bg)
- `.typing-cursor` - Blinking cursor for streaming
- `.code-block` - Fenced code styling
- `.inline-code` - Inline `code` styling

## InputBar

Auto-resizing textarea with Enter-to-send.

### Behavior
- `Enter` = send, `Shift+Enter` = newline
- Auto-resize up to 150px max height
- Disabled state when `disabled` prop or empty input

### Key Implementation

```typescript
const handleChange = useCallback((e) => {
  setValue(e.target.value);
  // Auto-resize
  const textarea = textareaRef.current;
  if (textarea) {
    textarea.style.height = 'auto';
    textarea.style.height = `${Math.min(textarea.scrollHeight, 150)}px`;
  }
}, []);
```

## SessionSidebar

Session management with swipe-to-delete (mobile).

### Features
- New chat button
- Session list with titles
- Active session highlight
- Delete confirmation

## ProjectSelector

Multi-project mode UI.

### States
- `stopped` - Gray, "Start" button
- `starting` - Yellow spinner
- `running` - Green, "Stop" button
- `error` - Red, error message

## Toast

Context-based notification system.

```typescript
const { showToast } = useToast();
showToast('Message saved', 'success');  // success | error | info
```

## CSS Variables (Theme)

```css
--bg-primary: #09090b;
--bg-secondary: #18181b;
--bg-tertiary: #27272a;
--text-primary: #fafafa;
--text-secondary: #a1a1aa;
--text-muted: #71717a;
--accent-primary: #00ff9d;
--accent-secondary: #00d4ff;
--accent-error: #ff0055;
--border-color: #3f3f46;
```

## Performance Patterns

1. **All exported components use `memo()`** - Prevents unnecessary re-renders
2. **`useMemo` for parsed content** - Markdown parsing is expensive
3. **`useCallback` for handlers** - Stable function references
4. **Barrel export via `index.ts`** - Tree-shaking friendly
