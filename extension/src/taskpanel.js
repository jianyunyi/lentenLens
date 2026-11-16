'use strict';

/**
 * Task Panel Provider for VS Code Tree View
 * 管理任务列表、过滤、搜索和交互
 */

class TaskTreeItem {
  constructor(vscode, task, isExpanded = false) {
    this.task = task;
    this.label = `${this.getStatusIcon(task.status)} ${task.name}`;
    this.description = `优先级: ${task.priority} · 进度: ${task.progress}%`;
    this.collapsibleState = isExpanded ? 
      vscode.TreeItemCollapsibleState.Expanded : 
      vscode.TreeItemCollapsibleState.Collapsed;
    this.iconPath = this.getIconPath(vscode, task.priority);
    this.contextValue = 'intentlens.task.item';
    this.tooltip = this.buildTooltip(task);
    this.command = {
      command: 'intentlens.task.viewDetails',
      title: 'View Task Details',
      arguments: [task],
    };
  }

  getStatusIcon(status) {
    const icons = {
      'pending': '⏳',
      'running': '▶',
      'completed': '✓',
      'failed': '✗',
    };
    return icons[status] || '?';
  }

  getIconPath(vscode, priority) {
    const iconMap = {
      'P0': new vscode.ThemeIcon('error'),
      'P1': new vscode.ThemeIcon('warning'),
      'P2': new vscode.ThemeIcon('info'),
      'P3': new vscode.ThemeIcon('pass'),
    };
    return iconMap[priority] || new vscode.ThemeIcon('question');
  }

  buildTooltip(task) {
    const lines = [
      `📋 Task: ${task.name}`,
      `优先级: ${task.priority}`,
      `状态: ${task.status}`,
      `进度: ${task.progress}%`,
    ];
    
    if (task.executionTime) {
      lines.push(`执行时间: ${task.executionTime}ms`);
    }
    if (task.memoryUsage) {
      lines.push(`内存: ${task.memoryUsage}MB`);
    }
    if (task.description) {
      lines.push('---');
      lines.push(task.description);
    }
    
    return new vscode.MarkdownString(lines.join('  \n'));
  }
}

class TaskDetailsItem {
  constructor(vscode, label, value, icon = undefined) {
    this.label = label;
    this.description = value;
    this.collapsibleState = vscode.TreeItemCollapsibleState.None;
    if (icon) {
      this.iconPath = new vscode.ThemeIcon(icon);
    }
    this.contextValue = 'intentlens.task.detail';
  }
}

class TaskTreeProvider {
  constructor(vscode) {
    this.vscode = vscode;
    this._onDidChangeTreeData = new vscode.EventEmitter();
    this.onDidChangeTreeData = this._onDidChangeTreeData.event;
    this._tasks = [];
    this._filter = { status: [], priority: [], searchText: '' };
    this._expandedItems = new Set();
  }

  /**
   * 设置任务列表
   */
  setTasks(tasks) {
    this._tasks = tasks;
    this.refresh();
  }

  /**
   * 刷新树视图
   */
  refresh() {
    this._onDidChangeTreeData.fire(undefined);
  }

  /**
   * 获取树项
   */
  getTreeItem(element) {
    if (element instanceof TaskDetailsItem) {
      return element;
    }
    
    const treeItem = new TaskTreeItem(
      this.vscode,
      element.task,
      this._expandedItems.has(element.task.id)
    );
    
    // 添加进度条的可视化
    if (element.task.status === 'running') {
      const progressBar = this.buildProgressBar(element.task.progress);
      treeItem.label += ` ${progressBar}`;
    }
    
    return treeItem;
  }

  /**
   * 获取子项
   */
  getChildren(element) {
    if (!element) {
      // 根元素：返回过滤后的任务
      return this.getFilteredTasks().map(task => ({
        task,
        _isTaskItem: true,
      }));
    }

    if (element._isTaskItem) {
      // 任务详情项
      const task = element.task;
      const details = [];

      if (task.description) {
        details.push(
          new TaskDetailsItem(this.vscode, '📝 描述', task.description)
        );
      }

      if (task.dependencies?.length > 0) {
        details.push(
          new TaskDetailsItem(
            this.vscode,
            '🔗 依赖',
            task.dependencies.join(', '),
            'link'
          )
        );
      }

      if (task.status === 'completed' || task.status === 'failed') {
        if (task.executionTime) {
          details.push(
            new TaskDetailsItem(
              this.vscode,
              '⏱ 执行时间',
              `${task.executionTime}ms`,
              'clock'
            )
          );
        }
        if (task.memoryUsage) {
          details.push(
            new TaskDetailsItem(
              this.vscode,
              '💾 内存',
              `${task.memoryUsage}MB`,
              'memory'
            )
          );
        }
      }

      if (task.error) {
        details.push(
          new TaskDetailsItem(
            this.vscode,
            '❌ 错误',
            task.error,
            'error'
          )
        );
      }

      // 添加日志
      if (task.logs?.length > 0) {
        details.push(
          new TaskDetailsItem(
            this.vscode,
            '📋 日志',
            `${task.logs.length} 条`,
            'output'
          )
        );
      }

      return details;
    }

    return [];
  }

  /**
   * 获取过滤后的任务
   */
  getFilteredTasks() {
    return this._tasks.filter(task => {
      // 按优先级过滤
      if (this._filter.priority.length > 0 && 
          !this._filter.priority.includes(task.priority)) {
        return false;
      }

      // 按状态过滤
      if (this._filter.status.length > 0 && 
          !this._filter.status.includes(task.status)) {
        return false;
      }

      // 搜索文本
      if (this._filter.searchText) {
        const searchLower = this._filter.searchText.toLowerCase();
        const matches = 
          task.name.toLowerCase().includes(searchLower) ||
          task.description?.toLowerCase().includes(searchLower);
        if (!matches) return false;
      }

      return true;
    });
  }

  /**
   * 设置过滤条件
   */
  setFilter(filter) {
    this._filter = { ...this._filter, ...filter };
    this.refresh();
  }

  /**
   * 切换任务展开状态
   */
  toggleExpanded(taskId) {
    if (this._expandedItems.has(taskId)) {
      this._expandedItems.delete(taskId);
    } else {
      this._expandedItems.add(taskId);
    }
    this.refresh();
  }

  /**
   * 构建进度条
   */
  buildProgressBar(progress) {
    const filled = Math.round(progress / 10);
    const empty = 10 - filled;
    return '[' + '█'.repeat(filled) + '░'.repeat(empty) + ']';
  }
}

module.exports = {
  TaskTreeProvider,
  TaskTreeItem,
  TaskDetailsItem,
};
