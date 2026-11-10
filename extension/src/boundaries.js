'use strict';

/**
 * Task Boundaries Decorator with Lazy Rendering Optimization
 * Phase 4 Task 8: Only decorates visible tasks in viewport
 */

class TaskBoundaryDecorator {
  constructor(vscode) {
    this.vscode = vscode;
    this._decorationType = null;
    this._boundaries = [];
    this._allTasks = []; // Store all tasks
    this._enabled = true;
    this._lazyMode = true; // Enable lazy decoration by default
    this._debounceTimer = null;
    this._debounceDelay = 100; // 100ms debounce
    this.initializeDecorationTypes();
  }

  /**
   * Initialize decoration types
   */
  initializeDecorationTypes() {
    const colors = {
      P0: '#FF4444',
      P1: '#FFA500',
      P2: '#4A90E2',
      P3: '#50C878',
    };

    this._decorationTypes = {};
    for (const [priority, color] of Object.entries(colors)) {
      this._decorationTypes[priority] = 
        this.vscode.window.createTextEditorDecorationType({
          isWholeLine: true,
          borderStyle: 'dashed',
          borderColor: color,
          borderWidth: '1px 0px 1px 0px',
          backgroundColor: color + '15',
          overviewRulerColor: color,
          overviewRulerLane: this.vscode.OverviewRulerLane.Right,
        });
    }

    this._headerDecorationType = 
      this.vscode.window.createTextEditorDecorationType({
        backgroundColor: new this.vscode.ThemeColor('editor.lineHighlightBackground'),
        fontWeight: 'bold',
      });
  }

  /**
   * Get visible line range in editor (viewport culling)
   */
  getVisibleRange(editor) {
    if (!editor || !editor.visibleRanges || editor.visibleRanges.length === 0) {
      return null;
    }

    const firstRange = editor.visibleRanges[0];
    const lastRange = editor.visibleRanges[editor.visibleRanges.length - 1];

    return {
      start: firstRange.start.line,
      end: lastRange.end.line
    };
  }

  /**
   * Filter tasks to only those visible in viewport
   */
  filterVisibleTasks(tasks, editor) {
    if (!this._lazyMode) {
      return tasks; // Return all if lazy mode disabled
    }

    const visibleRange = this.getVisibleRange(editor);
    if (!visibleRange) {
      return tasks; // Return all if no visible range
    }

    // Filter tasks whose code range overlaps with visible range
    return tasks.filter(task => {
      if (!task.codeRange) return false;
      
      const taskStart = task.codeRange.startLine;
      const taskEnd = task.codeRange.endLine;
      
      // Check if task range overlaps with visible range
      return !(taskEnd < visibleRange.start || taskStart > visibleRange.end);
    });
  }

  /**
   * Set task boundaries with lazy rendering
   * Only decorates visible tasks in viewport
   */
  setTaskBoundaries(editor, tasks) {
    if (!this._enabled || !editor) return;

    // Store all tasks for later use
    this._allTasks = tasks;

    // Apply decorations (debounced)
    this._applyDecorationsDebounced(editor);
  }

  /**
   * Apply decorations with debouncing
   */
  _applyDecorationsDebounced(editor) {
    // Clear existing timer
    if (this._debounceTimer) {
      clearTimeout(this._debounceTimer);
    }

    // Set new timer
    this._debounceTimer = setTimeout(() => {
      this._applyDecorations(editor);
      this._debounceTimer = null;
    }, this._debounceDelay);
  }

  /**
   * Internal: Apply decorations to editor
   */
  _applyDecorations(editor) {
    if (!editor) return;

    // Filter to visible tasks only (lazy decoration)
    const visibleTasks = this.filterVisibleTasks(this._allTasks, editor);

    const boundaries = [];
    const decorations = {};

    // Initialize decoration lists
    for (const priority of ['P0', 'P1', 'P2', 'P3']) {
      decorations[priority] = [];
    }

    // Process visible tasks
    for (const task of visibleTasks) {
      if (!task.codeRange) continue;

      const priority = task.priority || 'P3';
      const range = new this.vscode.Range(
        task.codeRange.startLine,
        0,
        task.codeRange.endLine,
        task.codeRange.endColumn || 999
      );

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

    // Apply decorations
    for (const [priority, decs] of Object.entries(decorations)) {
      editor.setDecorations(this._decorationTypes[priority], decs);
    }

    this._boundaries = boundaries;
  }

  /**
   * Update decorations when editor scrolls
   */
  onEditorScroll(editor) {
    if (!this._enabled || !this._lazyMode || !editor) return;
    
    // Debounced update when scrolling
    this._applyDecorationsDebounced(editor);
  }

  /**
   * Build hover message for task
   */
  buildHoverMessage(task) {
    const parts = [
      `### 📋 ${task.name}`,
      '',
      `**Priority**: ${task.priority}`,
      `**Status**: ${task.status}`,
      `**Progress**: ${task.progress}%`,
    ];

    if (task.executionTime) {
      parts.push(`**Execution Time**: ${task.executionTime}ms`);
    }
    if (task.memoryUsage) {
      parts.push(`**Memory**: ${task.memoryUsage}MB`);
    }
    if (task.description) {
      parts.push('');
      parts.push(task.description);
    }

    return new this.vscode.MarkdownString(parts.join('\n'));
  }

  /**
   * Clear all boundaries
   */
  clear(editor) {
    if (!editor) return;
    
    for (const priority of ['P0', 'P1', 'P2', 'P3']) {
      editor.setDecorations(this._decorationTypes[priority], []);
    }
    editor.setDecorations(this._headerDecorationType, []);
    this._boundaries = [];
    this._allTasks = [];
  }

  /**
   * Toggle decoration rendering
   */
  toggle() {
    this._enabled = !this._enabled;
    return this._enabled;
  }

  /**
   * Toggle lazy mode
   */
  toggleLazyMode() {
    this._lazyMode = !this._lazyMode;
    return this._lazyMode;
  }

  /**
   * Get task at line
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
   * Highlight specific task
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
   * Dispose decorator
   */
  dispose() {
    if (this._debounceTimer) {
      clearTimeout(this._debounceTimer);
    }
    
    for (const type of Object.values(this._decorationTypes)) {
      type.dispose();
    }
    
    if (this._headerDecorationType) {
      this._headerDecorationType.dispose();
    }
  }
}

module.exports = TaskBoundaryDecorator;