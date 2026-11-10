'use strict';

const assert = require('assert');

/**
 * TaskManagementPanel 单元测试
 */
describe('TaskManagementPanel', () => {
  let panel;
  
  beforeEach(() => {
    const mockVscode = {
      window: {
        createWebviewPanel: () => ({
          webview: {
            onDidReceiveMessage: () => {},
            html: '',
          },
          onDidDispose: () => {},
          reveal: () => {},
          dispose: () => {},
        }),
        showInformationMessage: () => Promise.resolve(),
        showErrorMessage: () => Promise.resolve(),
        showInputBox: () => Promise.resolve(),
      },
      ViewColumn: {
        Beside: 2,
      },
      Uri: {
        file: (path) => ({ fsPath: path }),
      },
    };
    
    const mockContext = {
      subscriptions: [],
    };
    
    const TaskManagementPanel = require('../src/taskmanagement').TaskManagementPanel;
    panel = new TaskManagementPanel(mockVscode, mockContext);
  });

  describe('createTask()', () => {
    it('应该创建新任务', () => {
      const taskData = {
        name: 'testTask',
        priority: 'P1',
        description: 'Test task description',
        dependencies: [],
      };
      
      panel.createTask(taskData);
      
      assert.strictEqual(panel._tasks.size, 1);
    });

    it('应该为任务生成唯一 ID', () => {
      const taskData1 = {
        name: 'task1',
        priority: 'P0',
        description: 'Task 1',
        dependencies: [],
      };
      
      const taskData2 = {
        name: 'task2',
        priority: 'P1',
        description: 'Task 2',
        dependencies: [],
      };
      
      panel.createTask(taskData1);
      panel.createTask(taskData2);
      
      const taskIds = Array.from(panel._tasks.keys());
      assert.strictEqual(taskIds.length, 2);
      assert.notStrictEqual(taskIds[0], taskIds[1]);
    });

    it('应该初始化任务状态为 pending', () => {
      const taskData = {
        name: 'testTask',
        priority: 'P1',
        description: 'Test',
        dependencies: [],
      };
      
      panel.createTask(taskData);
      
      const task = Array.from(panel._tasks.values())[0];
      assert.strictEqual(task.status, 'pending');
      assert.strictEqual(task.progress, 0);
    });
  });

  describe('hasCyclicDependency()', () => {
    it('应该检测直接循环依赖', () => {
      // 创建任务 A 依赖 B，B 依赖 A
      const taskA = {
        id: 'a',
        name: 'TaskA',
        priority: 'P0',
        status: 'pending',
        progress: 0,
        dependencies: ['TaskB'],
      };
      
      panel._tasks.set('a', taskA);
      panel._tasks.set('b', {
        id: 'b',
        name: 'TaskB',
        priority: 'P0',
        status: 'pending',
        progress: 0,
        dependencies: ['TaskA'],
      });
      
      const hasCycle = panel.hasCyclicDependency(taskA);
      assert(hasCycle);
    });

    it('应该检测间接循环依赖', () => {
      // A -> B -> C -> A
      panel._tasks.set('a', {
        id: 'a',
        name: 'TaskA',
        dependencies: ['TaskB'],
      });
      panel._tasks.set('b', {
        id: 'b',
        name: 'TaskB',
        dependencies: ['TaskC'],
      });
      panel._tasks.set('c', {
        id: 'c',
        name: 'TaskC',
        dependencies: ['TaskA'],
      });
      
      const taskA = panel._tasks.get('a');
      const hasCycle = panel.hasCyclicDependency(taskA);
      assert(hasCycle);
    });

    it('应该接受无循环依赖', () => {
      // A -> B, B -> C (valid chain)
      panel._tasks.set('a', {
        id: 'a',
        name: 'TaskA',
        dependencies: ['TaskB'],
      });
      panel._tasks.set('b', {
        id: 'b',
        name: 'TaskB',
        dependencies: ['TaskC'],
      });
      panel._tasks.set('c', {
        id: 'c',
        name: 'TaskC',
        dependencies: [],
      });
      
      const taskA = panel._tasks.get('a');
      const hasCycle = panel.hasCyclicDependency(taskA);
      assert(!hasCycle);
    });
  });

  describe('updateTask()', () => {
    it('应该更新任务信息', () => {
      const taskData = {
        name: 'originalTask',
        priority: 'P0',
        description: 'Original',
        dependencies: [],
      };
      
      panel.createTask(taskData);
      const taskId = Array.from(panel._tasks.keys())[0];
      
      const updateData = {
        id: taskId,
        name: 'updatedTask',
        priority: 'P1',
        status: 'running',
        progress: 50,
      };
      
      panel.updateTask(updateData);
      
      const task = panel._tasks.get(taskId);
      assert.strictEqual(task.name, 'updatedTask');
      assert.strictEqual(task.priority, 'P1');
      assert.strictEqual(task.progress, 50);
    });
  });

  describe('deleteTask()', () => {
    it('应该删除任务', () => {
      const taskData = {
        name: 'taskToDelete',
        priority: 'P1',
        description: 'Delete me',
        dependencies: [],
      };
      
      panel.createTask(taskData);
      const taskId = Array.from(panel._tasks.keys())[0];
      
      panel.deleteTask(taskId);
      
      assert(!panel._tasks.has(taskId));
    });
  });

  describe('getAllTasks()', () => {
    it('应该返回所有任务', () => {
      panel.createTask({ name: 'T1', priority: 'P0', description: '', dependencies: [] });
      panel.createTask({ name: 'T2', priority: 'P1', description: '', dependencies: [] });
      panel.createTask({ name: 'T3', priority: 'P2', description: '', dependencies: [] });
      
      const allTasks = panel.getAllTasks();
      
      assert.strictEqual(allTasks.length, 3);
    });
  });

  describe('exportTasks()', () => {
    it('应该导出任务为 JSON', () => {
      panel.createTask({ name: 'T1', priority: 'P0', description: 'Task 1', dependencies: [] });
      
      const exported = panel.exportTasks();
      const parsed = JSON.parse(exported);
      
      assert(Array.isArray(parsed));
      assert.strictEqual(parsed.length, 1);
      assert.strictEqual(parsed[0].name, 'T1');
    });

    it('导出的 JSON 应该可以被解析', () => {
      panel.createTask({ name: 'T1', priority: 'P0', description: 'T1', dependencies: [] });
      panel.createTask({ name: 'T2', priority: 'P1', description: 'T2', dependencies: [] });
      
      const exported = panel.exportTasks();
      
      assert.doesNotThrow(() => {
        JSON.parse(exported);
      });
    });
  });
});

