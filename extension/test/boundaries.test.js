'use strict';

const assert = require('assert');

/**
 * TaskBoundaryDecorator 单元测试
 */
describe('TaskBoundaryDecorator', () => {
  let decorator;
  
  beforeEach(() => {
    // Mock vscode 对象
    const mockVscode = {
      window: {
        createTextEditorDecorationType: () => ({
          dispose: () => {},
        }),
      },
      Range: function(startLine, startCol, endLine, endCol) {
        this.start = { line: startLine, character: startCol };
        this.end = { line: endLine, character: endCol };
      },
      ThemeIcon: function(name, color) {
        this.id = name;
        this.color = color;
      },
      ThemeColor: function(name) {
        this.name = name;
      },
      MarkdownString: function(str) {
        this.value = str;
      },
      OverviewRulerLane: {
        Right: 2,
      },
    };
    
    const TaskBoundaryDecorator = require('../src/boundaries').TaskBoundaryDecorator;
    decorator = new TaskBoundaryDecorator(mockVscode);
  });

  describe('setTaskBoundaries()', () => {
    it('应该为编辑器设置任务边框', () => {
      const mockEditor = {
        setDecorations: function(type, decorations) {
          this.decorations = decorations;
        },
      };
      
      const tasks = [
        {
          id: 'task_1',
          name: 'testFunction',
          priority: 'P1',
          status: 'running',
          progress: 50,
          codeRange: {
            startLine: 45,
            endLine: 89,
            endColumn: 10,
          },
        },
      ];
      
      decorator.setTaskBoundaries(mockEditor, tasks);
      
      assert.strictEqual(decorator._boundaries.length, 1);
      assert.strictEqual(decorator._boundaries[0].taskId, 'task_1');
    });

    it('应该处理多个任务边框', () => {
      const mockEditor = {
        setDecorations: function(type, decorations) {
          this.decorations = decorations;
        },
      };
      
      const tasks = [
        { id: 't1', name: 'func1', priority: 'P0', status: 'pending', progress: 0, codeRange: { startLine: 10, endLine: 20, endColumn: 5 } },
        { id: 't2', name: 'func2', priority: 'P1', status: 'running', progress: 50, codeRange: { startLine: 30, endLine: 40, endColumn: 8 } },
        { id: 't3', name: 'func3', priority: 'P2', status: 'completed', progress: 100, codeRange: { startLine: 50, endLine: 60, endColumn: 10 } },
      ];
      
      decorator.setTaskBoundaries(mockEditor, tasks);
      
      assert.strictEqual(decorator._boundaries.length, 3);
    });
  });

  describe('clear()', () => {
    it('应该清除所有边框', () => {
      const mockEditor = {
        setDecorations: function(type, decorations) {
          this.decorations = decorations;
        },
      };
      
      decorator._boundaries = [{ taskId: 'task_1' }];
      decorator.clear(mockEditor);
      
      assert.strictEqual(decorator._boundaries.length, 0);
    });
  });

  describe('toggle()', () => {
    it('应该切换边框显示状态', () => {
      const initialState = decorator._enabled;
      const newState = decorator.toggle();
      
      assert.strictEqual(newState, !initialState);
      assert.strictEqual(decorator._enabled, !initialState);
    });

    it('切换两次应该恢复原状态', () => {
      const initialState = decorator._enabled;
      decorator.toggle();
      decorator.toggle();
      
      assert.strictEqual(decorator._enabled, initialState);
    });
  });

  describe('getTaskAtLine()', () => {
    it('应该获取指定行的任务', () => {
      decorator._boundaries = [
        {
          taskId: 'task_1',
          taskName: 'func1',
          priority: 'P1',
          range: {
            start: { line: 10 },
            end: { line: 20 },
          },
        },
      ];
      
      const task = decorator.getTaskAtLine(15);
      
      assert.notStrictEqual(task, null);
      assert.strictEqual(task.taskId, 'task_1');
    });

    it('行不在边框范围内时应该返回 null', () => {
      decorator._boundaries = [
        {
          taskId: 'task_1',
          range: {
            start: { line: 10 },
            end: { line: 20 },
          },
        },
      ];
      
      const task = decorator.getTaskAtLine(50);
      
      assert.strictEqual(task, null);
    });
  });

  describe('highlightTask()', () => {
    it('应该高亮指定任务的边框', () => {
      let highlightedDecorationType = null;
      let highlightedRanges = [];
      
      const mockEditor = {
        setDecorations: function(type, ranges) {
          highlightedDecorationType = type;
          highlightedRanges = ranges;
        },
      };
      
      decorator._boundaries = [
        {
          taskId: 'task_1',
          priority: 'P1',
          range: { start: { line: 10 }, end: { line: 20 } },
        },
      ];
      
      decorator.highlightTask(mockEditor, 'task_1');
      
      assert(highlightedDecorationType !== null);
      assert(highlightedRanges.length > 0);
    });
  });

  describe('buildHoverMessage()', () => {
    it('应该为任务构建悬停信息', () => {
      const task = {
        id: 'task_1',
        name: 'validateInput',
        priority: 'P1',
        status: 'running',
        progress: 50,
        description: 'Validate user input',
        executionTime: 234,
        memoryUsage: 45,
      };
      
      const message = decorator.buildHoverMessage(task);
      
      assert(message.value.includes('validateInput'));
      assert(message.value.includes('P1'));
      assert(message.value.includes('234'));
    });
  });
});

