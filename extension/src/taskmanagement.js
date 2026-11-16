'use strict';

/**
 * Task Management Panel
 * 创建、编辑、删除任务，管理依赖关系和执行历史
 */
const { randomUUID } = require('node:crypto');

class TaskManagementPanel {
  constructor(vscode, extensionContext) {
    this.vscode = vscode;
    this.context = extensionContext;
    this._tasks = new Map();
    this._panel = null;
    this._webviewReady = false;
  }

  /**
   * 创建或显示任务管理面板
   */
  async show() {
    if (this._panel) {
      this._panel.reveal(this.vscode.ViewColumn.Beside);
      return;
    }

    this._panel = this.vscode.window.createWebviewPanel(
      'intentlens.taskmanagement',
      '📋 任务管理',
      this.vscode.ViewColumn.Beside,
      {
        enableScripts: true,
        enableFindWidget: true,
        retainContextWhenHidden: true,
      }
    );

    this._panel.webview.onDidReceiveMessage(
      message => this.handleWebviewMessage(message)
    );

    this._panel.onDidDispose(() => {
      this._panel = null;
    });

    this._panel.webview.html = this.getWebviewContent();
  }

  /**
   * 获取 Webview HTML 内容
   */
  getWebviewContent() {
    return `
      <!DOCTYPE html>
      <html>
      <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <style>
          body {
            font-family: var(--vscode-font-family);
            background: var(--vscode-editor-background);
            color: var(--vscode-editor-foreground);
            margin: 0;
            padding: 16px;
          }
          
          .container {
            max-width: 800px;
            margin: 0 auto;
          }
          
          h2 {
            margin-top: 0;
            border-bottom: 1px solid var(--vscode-panel-border);
            padding-bottom: 8px;
          }
          
          .form-group {
            margin-bottom: 16px;
          }
          
          label {
            display: block;
            margin-bottom: 4px;
            font-weight: bold;
          }
          
          input[type="text"],
          textarea,
          select {
            width: 100%;
            padding: 8px;
            background: var(--vscode-input-background);
            color: var(--vscode-input-foreground);
            border: 1px solid var(--vscode-input-border);
            border-radius: 4px;
            font-family: inherit;
            box-sizing: border-box;
          }
          
          textarea {
            resize: vertical;
            min-height: 80px;
          }
          
          .priority-selector {
            display: flex;
            gap: 8px;
          }
          
          .priority-btn {
            padding: 8px 12px;
            border: 2px solid transparent;
            border-radius: 4px;
            background: var(--vscode-button-background);
            color: var(--vscode-button-foreground);
            cursor: pointer;
            font-weight: bold;
          }
          
          .priority-btn.active {
            border-color: var(--vscode-focusBorder);
            background: var(--vscode-button-hoverBackground);
          }
          
          .priority-btn.p0 { color: #FF4444; }
          .priority-btn.p1 { color: #FFA500; }
          .priority-btn.p2 { color: #4A90E2; }
          .priority-btn.p3 { color: #50C878; }
          
          .dependencies {
            display: flex;
            flex-wrap: wrap;
            gap: 8px;
            margin: 8px 0;
          }
          
          .tag {
            background: var(--vscode-button-background);
            padding: 4px 12px;
            border-radius: 12px;
            display: flex;
            align-items: center;
            gap: 8px;
          }
          
          .tag-close {
            cursor: pointer;
            font-weight: bold;
          }
          
          .button-group {
            display: flex;
            gap: 8px;
            margin-top: 16px;
          }
          
          button {
            padding: 8px 16px;
            background: var(--vscode-button-background);
            color: var(--vscode-button-foreground);
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-weight: bold;
          }
          
          button:hover {
            background: var(--vscode-button-hoverBackground);
          }
          
          .history {
            margin-top: 24px;
            padding-top: 16px;
            border-top: 1px solid var(--vscode-panel-border);
          }
          
          .history-item {
            padding: 8px;
            margin-bottom: 8px;
            background: var(--vscode-input-background);
            border-left: 3px solid transparent;
            border-radius: 2px;
          }
          
          .history-item.success {
            border-color: #50C878;
          }
          
          .history-item.failed {
            border-color: #FF4444;
          }
          
          .history-time {
            font-size: 12px;
            color: var(--vscode-descriptionForeground);
          }
        </style>
      </head>
      <body>
        <div class="container">
          <h2>📋 创建新任务</h2>
          
          <div class="form-group">
            <label>任务名称 *</label>
            <input type="text" id="taskName" placeholder="例: validateUserInput">
          </div>
          
          <div class="form-group">
            <label>优先级</label>
            <div class="priority-selector">
              <button class="priority-btn p0" data-priority="P0">P0 · 关键</button>
              <button class="priority-btn p1" data-priority="P1">P1 · 高</button>
              <button class="priority-btn p2 active" data-priority="P2">P2 · 中</button>
              <button class="priority-btn p3" data-priority="P3">P3 · 低</button>
            </div>
            <input type="hidden" id="priority" value="P2">
          </div>
          
          <div class="form-group">
            <label>描述</label>
            <textarea id="description" placeholder="描述这个任务的作用..."></textarea>
          </div>
          
          <div class="form-group">
            <label>依赖任务</label>
            <input type="text" id="dependencyInput" placeholder="输入任务名称并按 Enter 添加">
            <div class="dependencies" id="dependencies"></div>
          </div>
          
          <div class="button-group">
            <button onclick="createTask()">✓ 创建任务</button>
            <button onclick="resetForm()">↻ 重置</button>
          </div>
          
          <div class="history">
            <h3>📝 执行历史</h3>
            <div id="history"></div>
          </div>
        </div>
        
        <script>
          const vscode = acquireVsCodeApi();
          let selectedPriority = 'P2';
          let dependencies = [];
          
          document.querySelectorAll('.priority-btn').forEach(btn => {
            btn.addEventListener('click', () => {
              document.querySelectorAll('.priority-btn').forEach(b => 
                b.classList.remove('active')
              );
              btn.classList.add('active');
              selectedPriority = btn.dataset.priority;
              document.getElementById('priority').value = selectedPriority;
            });
          });
          
          document.getElementById('dependencyInput').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
              const value = e.target.value.trim();
              if (value && !dependencies.includes(value)) {
                dependencies.push(value);
                updateDependencies();
                e.target.value = '';
              }
            }
          });
          
          function updateDependencies() {
            const container = document.getElementById('dependencies');
            container.innerHTML = dependencies.map(dep =>
              \<div class="tag">\<span class="tag-close" onclick="removeDependency('\')">×</span></div>\
            ).join('');
          }
          
          function removeDependency(dep) {
            dependencies = dependencies.filter(d => d !== dep);
            updateDependencies();
          }
          
          function createTask() {
            const name = document.getElementById('taskName').value.trim();
            if (!name) {
              vscode.postMessage({
                command: 'error',
                message: '请输入任务名称'
              });
              return;
            }
            
            vscode.postMessage({
              command: 'createTask',
              data: {
                name,
                priority: selectedPriority,
                description: document.getElementById('description').value,
                dependencies,
              }
            });
            
            resetForm();
          }
          
          function resetForm() {
            document.getElementById('taskName').value = '';
            document.getElementById('description').value = '';
            dependencies = [];
            updateDependencies();
            selectedPriority = 'P2';
            document.querySelectorAll('.priority-btn').forEach(b => b.classList.remove('active'));
            document.querySelector('[data-priority="P2"]').classList.add('active');
          }
        </script>
      </body>
      </html>
    `;
  }

  /**
   * 处理 Webview 消息
   */
  handleWebviewMessage(message) {
    switch (message.command) {
      case 'createTask':
        this.createTask(message.data);
        break;
      case 'updateTask':
        this.updateTask(message.data);
        break;
      case 'deleteTask':
        this.deleteTask(message.data);
        break;
    }
  }

  /**
   * 创建任务
   */
  createTask(data) {
    const taskId = `task_${randomUUID()}`;
    const task = {
      id: taskId,
      name: data.name,
      priority: data.priority,
      description: data.description,
      dependencies: data.dependencies || [],
      status: 'pending',
      progress: 0,
      createdAt: new Date(),
      logs: [],
      executionHistory: [],
    };

    // 检测循环依赖
    if (this.hasCyclicDependency(task)) {
      this.vscode.window.showErrorMessage('检测到循环依赖！');
      return;
    }

    this._tasks.set(taskId, task);
    this.vscode.window.showInformationMessage(`✓ 任务 "${task.name}" 已创建`);
  }

  /**
   * 检测循环依赖
   */
  hasCyclicDependency(task, visited = new Set()) {
    for (const depName of task.dependencies) {
      if (visited.has(depName)) {
        return true;
      }
      
      visited.add(depName);
      
      for (const [, t] of this._tasks) {
        if (t.name === depName) {
          if (this.hasCyclicDependency(t, new Set(visited))) {
            return true;
          }
          break;
        }
      }
    }
    
    return false;
  }

  /**
   * 更新任务
   */
  updateTask(data) {
    const task = this._tasks.get(data.id);
    if (!task) return;
    
    Object.assign(task, data);
    this.vscode.window.showInformationMessage(`✓ 任务已更新`);
  }

  /**
   * 删除任务
   */
  deleteTask(taskId) {
    this._tasks.delete(taskId);
    this.vscode.window.showInformationMessage('✓ 任务已删除');
  }

  /**
   * 获取所有任务
   */
  getAllTasks() {
    return Array.from(this._tasks.values());
  }

  /**
   * 导出任务
   */
  exportTasks() {
    const tasks = this.getAllTasks();
    return JSON.stringify(tasks, null, 2);
  }
}

module.exports = {
  TaskManagementPanel,
};


