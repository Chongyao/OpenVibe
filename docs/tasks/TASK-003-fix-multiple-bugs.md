# TASK-003: 修复多个前端 Bug

## 问题描述

用户报告了以下 bug：
1. 点击 Game2048 的 Start 按钮没有反应
2. 点击 New Chat 创建新会话后，弹出框消失了
3. 过了一会儿显示的是 SmartQuant 的项目介绍，而不是 Game2048

## Bug 1: Start 按钮无响应

### 根因

`page.tsx` 第 48 行：
```tsx
} = useProjects({ send: (msg) => sendRef.current?.({ ...msg, payload: msg.payload } as ClientMessage) });
```

问题：
1. `sendRef.current` 在 `useEffect` 执行前是 `null`
2. 可选链 `?.` 导致调用静默失败，无任何反馈
3. 没有检查 `send` 的返回值

### 修复方案

```tsx
} = useProjects({ 
  send: (msg) => {
    if (!sendRef.current) {
      console.warn('WebSocket not ready, cannot send:', msg.type);
      return;
    }
    sendRef.current({ ...msg, payload: msg.payload } as ClientMessage);
  }
});
```

或者更好的方案：确保 `send` 只在连接后调用：

```tsx
const { projects, ... } = useProjects({ 
  send: sendRef.current ?? (() => {}),
  isConnected  // 传入连接状态
});
```

## Bug 2 & 3: 项目选择和会话关联问题

### 根因

`activeProject` 的计算逻辑有缺陷（第 105-111 行）：
```tsx
const activeProject = useMemo(() => {
  if (!activeProjectPath && projects.length > 0) {
    return projects[0];  // 问题：默认返回第一个项目
  }
  ...
}, [projects, activeProjectPath]);
```

当 `projects` 列表顺序变化时，`projects[0]` 会变，导致 activeProject 意外切换。

### 修复方案

1. 持久化 `activeProjectPath` 到 localStorage
2. 或者当 `projects` 更新时，保持当前选择不变

```tsx
const activeProject = useMemo(() => {
  // 如果有选择的项目路径，优先使用
  if (activeProjectPath) {
    const found = projects.find(p => p.path === activeProjectPath);
    if (found) return found;
  }
  // 没有选择时，返回第一个，但不要自动切换
  return projects.length > 0 ? projects[0] : null;
}, [projects, activeProjectPath]);

// 当 projects 加载完成且没有 activeProjectPath 时，设置默认值
useEffect(() => {
  if (projects.length > 0 && !activeProjectPath) {
    setActiveProjectPath(projects[0].path);
  }
}, [projects, activeProjectPath]);
```

## Bug 4: 会话创建后的 directory 关联

### 问题

当用户在 Game2048 下创建新会话时，会话应该关联到 Game2048，但可能关联到了错误的项目。

### 检查点

1. `session.create` 请求是否带了 `directory` 参数？
2. 后端是否正确处理 `directory`？

### 修复方案

在 `handleNewSession` 中添加 `directory` 参数：

```tsx
const handleNewSession = useCallback(() => {
  if (state !== 'connected' || isCreatingSession) return;
  setIsCreatingSession(true);
  send({
    type: 'session.create',
    id: generateId(),
    payload: { 
      title: 'New Chat',
      directory: activeProject?.path  // 关联到当前项目
    },
  });
}, [state, send, isCreatingSession, activeProject]);
```

## 交付要求

### 需要修改的文件

1. `app/src/app/page.tsx`

### 验收标准

1. 点击 Start 按钮，项目状态变为 "Starting" 然后 "Running"
2. 点击 New Chat，会话创建成功，关联到当前选择的项目
3. 项目选择后，刷新页面仍然保持选择（可选）
4. 无 console 错误

### 修改限制

- 不要修改后端代码
- 不要修改 CSS/样式
- 保持现有的 API 接口不变

## 测试步骤

1. 打开 http://121.36.218.61:8080
2. 等待连接成功（绿色指示器）
3. 下拉选择 "Game2048" 项目
4. 点击 Start 按钮，验证状态变化
5. 点击 New Chat，验证会话创建
6. 发送一条消息，验证回复是关于 Game2048 的
