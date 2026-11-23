# IntentLens UI 增强设计文档

**版本**: 1.0  
**日期**: 2026-11-06  
**状态**: 规划中

---

## 1. 总体架构

### 1.1 新 UI 组件树

`
VS Code Editor
├─ IntentLens Sidebar Container
│  ├─ Task Panel (新)
│  │  ├─ Task List View
│  │  ├─ Task Filter/Search
│  │  └─ Task Detail Editor
│  ├─ Call Path Panel
│  │  ├─ Call Hierarchy
│  │  ├─ Performance Metrics
│  │  └─ Source Evidence
│  ├─ Model Answer Panel
│  ├─ Rounds Panel
└─ Editor Decorations (新)
   ├─ Task Boundaries (任务边框)
   ├─ Call Paths (调用线)
   └─ Task Indicators (任务指示器)
`

---

## 2. 四大功能

### 2.1 侧边栏增强 (Sidebar Enhancement)

**功能**:
- 新增任务面板视图
- 任务列表显示 (优先级、状态、进度)
- 快速过滤和搜索
- 任务详情展开
- 右键快速操作菜单

**数据模型**:
`	ypescript
interface Task {
  id: string;
  name: string;
  priority: 'P0' | 'P1' | 'P2' | 'P3';
  status: 'pending' | 'running' | 'completed' | 'failed';
  progress: number;
  executionTime?: number;
  memoryUsage?: number;
  dependencies: string[];
}
`

### 2.2 任务边框 (Task Boundaries)

**功能**:
- 在编辑器中用虚线标记任务代码范围
- 彩色标识不同优先级
- 悬停显示任务信息
- 点击跳转到任务详情

**技术方案**:
- 使用 VS Code TextEditorDecorationType
- Dashed border + background color
- Theme color variables

### 2.3 调用线视图 (Call Path View)

**功能**:
- 显示完整调用链层级
- 每个节点显示执行时间
- 性能指标显示 (内存、CPU)
- 点击代码跳转
- 性能排序和过滤

**性能指标**:
- 执行时间 (ms)
- 调用计数
- 内存分配 (MB)
- CPU 使用百分比

### 2.4 任务面板 (Task Management Panel)

**功能**:
- 创建/编辑/删除任务
- 任务依赖管理 (拖拽排序、循环检测)
- 执行历史追踪
- 版本对比
- 日志查看

**UI 组件**:
- 任务创建表单
- 依赖关系编辑器
- 执行历史时间线
- 日志输出面板

---

## 3. 颜色方案

`json
{
  "P0": "#FF0000",  // 红色
  "P1": "#FF9900",  // 橙色
  "P2": "#0066FF",  // 蓝色
  "P3": "#00CC00"   // 绿色
}
`

---

## 4. 实现阶段

| 阶段 | 功能 | 时间 |
|------|------|------|
| 1 | 侧边栏 + 任务面板 | 1-2 周 |
| 2 | 任务边框 | 1-2 周 |
| 3 | 调用线增强 | 1 周 |
| 4 | 任务管理完整功能 | 1-2 周 |

**总耗时**: 4-7 周

