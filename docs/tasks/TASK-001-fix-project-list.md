# TASK-001: 修复项目列表不显示的问题

## 问题描述

用户报告：网页上只显示 1 个项目 (SmartQuant)，但 Agent 配置了 3 个项目。

**实际调试发现**：网页显示 "Disconnected"，项目列表完全没有加载。

## 根因分析

### Bug 1: `isConnected` 硬编码为 `true`

**位置**: `app/src/app/page.tsx` 第 47 行

```tsx
} = useProjects({ send: ..., isConnected: true });  // ❌ 硬编码!
```

**问题**: `useProjects` 在组件挂载时立即发送 `project.list`，但 WebSocket 还没连接。

### Bug 2: 自动请求未注册到 pendingRequests

**位置**: `app/src/hooks/useProjects.ts` 第 124-134 行

```tsx
useEffect(() => {
  if (isConnected && !hasFetchedRef.current) {
    hasFetchedRef.current = true;
    const id = generateId();
    send({
      type: 'project.list',
      id,
      payload: {},
    });
    // ❌ 没有 pendingRequests.current.set(id, { resolve, reject })
  }
}, [isConnected, send]);
```

**问题**: 发送了请求但没注册回调，`handleResponse` 无法处理响应。

## 修复方案

### 修复 1: 传入真实的 `isConnected`

```tsx
// page.tsx
const { state, send } = useWebSocket({ ... });
const isConnected = state === 'connected';

const { projects, ... } = useProjects({ 
  send: ..., 
  isConnected  // ✅ 传入真实状态
});
```

### 修复 2: 在 `onConnect` 回调中触发 `project.list`

移除 `useProjects` 中的自动 useEffect，改为在 `page.tsx` 的 `onConnect` 中显式调用 `listProjects()`。

### 修复 3: 确保 `listProjects()` 正确注册 pendingRequests

`listProjects()` 函数已经正确注册了 pendingRequests（第 39-41 行），只需要在正确时机调用即可。

## 交付要求

### 需要修改的文件

1. `app/src/app/page.tsx`
2. `app/src/hooks/useProjects.ts`

### 验收标准

1. 网页加载后，项目选择器显示 3 个项目：OpenVibe, SmartQuant, Game2048
2. 状态显示为 "Connected"（绿色）
3. 无 console 错误

### 修改限制

- 不要修改后端代码
- 不要修改 CSS/样式
- 保持现有的 API 接口不变

## 测试步骤

1. 打开 http://121.36.218.61:8080
2. 等待 3 秒
3. 检查状态指示器是否为绿色 "Connected"
4. 检查项目选择器下拉菜单是否显示 3 个项目
