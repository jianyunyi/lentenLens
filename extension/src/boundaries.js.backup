'use strict';

/**
 * Task Boundaries Decorator
 * 在编辑器中绘制任务代码范围的虚线边框
 */

class TaskBoundaryDecorator {
  constructor(vscode) {
    this.vscode = vscode;
    this._decorationType = null;
    this._boundaries = [];
    this._enabled = true;
    this.initializeDecorationTypes();
  }

  /**
   * 初始化装饰类型
   */
  initializeDecorationTypes() {
    const colors = {
      P0: '#FF4444', // 红色
      P1: '#FFA500', // 橙色
      P2: '#4A90E2', // 蓝色
      P3: '#50C878', // 绿色
    };

    // 为每个优先级创建装饰类型
    this._decorationTypes = {};
    for (const [priority, color] of Object.entries(colors)) {
      this._decorationTypes[priority] = 
        this.vscode.window.createTextEditorDecorationType({
          isWholeLine: true,
          borderStyle: 'dashed',
          borderColor: color,
          borderWidth: '1px 0px 1px 0px',
          backgroundColor: color + '15', // 透明度 15%
          overviewRulerColor: color,
          overviewRulerLane: this.vscode.OverviewRulerLane.Right,
          after: {
            contentText: '  ◀ Task Boundary ▶',
            color: color,
            fontStyle: 'italic',
            margin: '0 0 0 10px',
          },
        });
    }

    // 任务边界头部装饰
    this._headerDecorationType = 
      this.vscode.window.createTextEditorDecorationType({
        backgroundColor: new this.vscode.ThemeColor('editor.lineHighlightBackground'),
        fontWeight: 'bold',
        border: '2px solid',
        borderColor: new this.vscode.ThemeColor('editor.lineHighlightBorder'),
        after: {
          contentText: ' [Task Start]',
          color: new this.vscode.ThemeColor('editor.foreground'),
          margin: '0 0 0 10px',
          fontWeight: 'bold',
        },
      });
  }

  /**
   * 为编辑器添加任务边框
   */
  setTaskBoundaries(editor, tasks) {
    if (!this._enabled || !editor) return;

    const boundaries = [];
    const decorations = {};

    // 初始化每个优先级的装饰列表
    for (const priority of ['P0', 'P1', 'P2', 'P3']) {
      decorations[priority] = [];
    }

    // 处理每个任务
    for (const task of tasks) {
      if (!task.codeRange) continue;

      const priority = task.priority || 'P3';
      const range = new this.vscode.Range(
        task.codeRange.startLine,
        0,
        task.codeRange.endLine,
        task.codeRange.endColumn || 999
      );

      // 添加边框装饰
      decorations[priority].push({
        range,
        hoverMessage: this.buildHoverMessage(task),
      });

      boundaries.push({
        taskId: task.id,
        taskName: task.name,
        priority,
        range,
      });
    }

    // 应用装饰
    for (const [priority, decs] of Object.entries(decorations)) {
      if (decs.length > 0) {
        editor.setDecorations(this._decorationTypes[priority], decs);
      }
    }

    this._boundaries = boundaries;
  }

  /**
   * 构建悬停提示信息
   */
  buildHoverMessage(task) {
    const parts = [
      \### 📋 \\,
      '',
      \**优先级**: \\,
      \**状态**: \\,
      \**进度**: \%\,
    ];

    if (task.executionTime) {
      parts.push(\**执行时间**: \ms\);
    }
    if (task.memoryUsage) {
      parts.push(\**内存**: \MB\);
    }
    if (task.description) {
      parts.push('');
      parts.push(task.description);
    }

    return new this.vscode.MarkdownString(parts.join('\\n'));
  }

  /**
   * 清除所有边框
   */
  clear(editor) {
    if (!editor) return;
    
    for (const priority of ['P0', 'P1', 'P2', 'P3']) {
      editor.setDecorations(this._decorationTypes[priority], []);
    }
    editor.setDecorations(this._headerDecorationType, []);
    this._boundaries = [];
  }

  /**
   * 切换边框显示
   */
  toggle() {
    this._enabled = !this._enabled;
    return this._enabled;
  }

  /**
   * 获取指定位置的任务
   */
  getTaskAtLine(line) {
    for (const boundary of this._boundaries) {
      if (boundary.range.start.line <= line && 
          line <= boundary.range.end.line) {
        return {
          taskId: boundary.taskId,
          taskName: boundary.taskName,
          priority: boundary.priority,
        };
      }
    }
    return null;
  }

  /**
   * 高亮显示指定任务的边框
   */
  highlightTask(editor, taskId) {
    for (const priority of ['P0', 'P1', 'P2', 'P3']) {
      editor.setDecorations(this._decorationTypes[priority], []);
    }

    const boundary = this._boundaries.find(b => b.taskId === taskId);
    if (boundary) {
      editor.setDecorations(
        this._decorationTypes[boundary.priority],
        [{ range: boundary.range }]
      );
    }
  }

  /**
   * 注册快捷键处理
   */
  registerCommands(context) {
    context.subscriptions.push(
      this.vscode.commands.registerCommand(
        'intentlens.boundary.toggle',
        () => {
          const enabled = this.toggle();
          this.vscode.window.showInformationMessage(
            enabled ? '任务边框已启用' : '任务边框已禁用'
          );
        }
      )
    );

    context.subscriptions.push(
      this.vscode.commands.registerCommand(
        'intentlens.boundary.highlight',
        (taskId) => {
          const editor = this.vscode.window.activeTextEditor;
          if (editor) {
            this.highlightTask(editor, taskId);
          }
        }
      )
    );
  }
}

module.exports = {
  TaskBoundaryDecorator,
};
