# 🚀 Phase 1 集成 - 修改清单

**目标**: 将 4 个新 UI 模块集成到 extension.js 中

## 阶段 1.1: 加载模块 (5 分钟)

### 在文件顶部添加新的 require 语句

位置: 第 1-20 行之后

需要添加:
\\\javascript
const { TaskTreeProvider } = require('./taskpanel');
const { TaskBoundaryDecorator } = require('./boundaries');
const { EnhancedCallPathProvider } = require('./callpath-enhanced');
const { TaskManagementPanel } = require('./taskmanagement');
\\\

---

## 阶段 1.2: 初始化任务面板 (10 分钟)

### 在 activate() 函数中初始化

位置: 在 'let callPathProvider' 之前 (约第 25-26 行)

需要添加:
\\\javascript
// 任务面板提供者
let taskProvider;
let taskBoundaryDecorator;
let enhancedCallPathProvider;
let taskManagementPanel;
\\\

---

## 阶段 1.3: 注册视图提供者 (15 分钟)

### 在创建 callPath 树视图之后

位置: 约第 430-435 行

需要添加:
\\\javascript
// 初始化任务面板
taskProvider = new TaskTreeProvider(vscode);
const taskView = vscode.window.createTreeView('intentlens.tasks', { treeDataProvider: taskProvider });

// 初始化任务边框装饰
taskBoundaryDecorator = new TaskBoundaryDecorator(vscode);
taskBoundaryDecorator.registerCommands(context);

// 初始化增强的调用路径
enhancedCallPathProvider = new EnhancedCallPathProvider(vscode);

// 初始化任务管理面板
taskManagementPanel = new TaskManagementPanel(vscode, context);

// 订阅变化
context.subscriptions.push(taskView, taskBoundaryDecorator);
\\\

---

## 阶段 1.4: 注册新命令 (20 分钟)

### 在现有 command() 调用之后

位置: 约第 580-600 行

需要添加:
\\\javascript
// 任务相关命令
command('task.create', async () => {
  await taskManagementPanel.show();
});

command('task.run', async (task) => {
  if (task && taskProvider) {
    await cli(...); // 执行任务逻辑
    taskProvider.refresh();
  }
});

command('task.edit', async (task) => {
  if (task && taskManagementPanel) {
    taskManagementPanel.show();
  }
});

command('task.delete', async (task) => {
  if (task && taskProvider) {
    // 删除任务逻辑
    taskProvider.refresh();
  }
});

command('taskmanagement.show', async () => {
  await taskManagementPanel.show();
});

command('boundary.toggle', async () => {
  if (taskBoundaryDecorator) {
    const enabled = taskBoundaryDecorator.toggle();
    const editor = vscode.window.activeTextEditor;
    if (editor) {
      // 更新边框显示
    }
  }
});

command('callpath.export', async () => {
  if (enhancedCallPathProvider) {
    const text = enhancedCallPathProvider.exportAsText();
    await vscode.env.clipboard.writeText(text);
    vscode.window.showInformationMessage('调用图已导出到剪贴板');
  }
});
\\\

---

## 阶段 1.5: 更新事件监听 (10 分钟)

### 在 onDidSaveTextDocument 之后

位置: 约第 670-680 行

需要添加:
\\\javascript
// 编辑器变化时更新任务边框
if (vscode.window.onDidChangeActiveTextEditor) {
  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor((editor) => {
      if (editor && taskBoundaryDecorator) {
        // 更新当前编辑器的任务边框
        // taskBoundaryDecorator.setTaskBoundaries(editor, tasks);
      }
    })
  );
}
\\\

---

## 总结

| 步骤 | 文件位置 | 代码行数 | 时间 |
|------|---------|--------|------|
| 1.1 加载模块 | 顶部 | 4 行 | 5 分钟 |
| 1.2 初始化变量 | 第 25 行 | 4 行 | 5 分钟 |
| 1.3 注册视图 | 第 430 行 | 15 行 | 10 分钟 |
| 1.4 注册命令 | 第 600 行 | 50 行 | 20 分钟 |
| 1.5 事件监听 | 第 680 行 | 10 行 | 10 分钟 |
| **总计** | | **83 行** | **50 分钟** |

---

**下一步**: 执行 Phase 1.1

