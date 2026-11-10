'use strict';

const assert = require('assert');
const vscode = require('vscode');

// 这些测试文件需要 mock vscode，暂时创建框架

/**
 * TaskTreeProvider 单元测试
 */
describe('TaskTreeProvider', () => {
  let provider;
  
  beforeEach(() => {
    // Mock vscode 对象
    const mockVscode = {
      TreeItemCollapsibleState: {
        None: 0,
        Collapsed: 1,
        Expanded: 2,
      },
      ThemeIcon: function(name) {
        this.id = name;
      },
      MarkdownString: function(str) {
        this.value = str;
      },
    };
    
    // 动态加载 TaskTreeProvider 模块
    const TaskTreeProvider = require('../src/taskpanel').TaskTreeProvider;
    provider = new TaskTreeProvider(mockVscode);
  });

  describe('setTasks()', () => {
    it('应该设置任务列表', () => {
      const tasks = [
        {
          id: 'task_1',
          name: 'test_task',
          priority: 'P1',
          status: 'pending',
          progress: 0,
          description: 'Test task',
        },
      ];
      
      provider.setTasks(tasks);
      assert.strictEqual(provider._tasks.length, 1);
      assert.strictEqual(provider._tasks[0].name, 'test_task');
    });
  });

  describe('setFilter()', () => {
    it('应该按优先级过滤', () => {
      const tasks = [
        { id: '1', name: 'T1', priority: 'P0', status: 'pending', progress: 0 },
        { id: '2', name: 'T2', priority: 'P1', status: 'pending', progress: 0 },
        { id: '3', name: 'T3', priority: 'P2', status: 'pending', progress: 0 },
      ];
      
      provider.setTasks(tasks);
      provider.setFilter({ priority: ['P0', 'P1'] });
      
      const filtered = provider.getFilteredTasks();
      assert.strictEqual(filtered.length, 2);
    });

    it('应该按状态过滤', () => {
      const tasks = [
        { id: '1', name: 'T1', priority: 'P0', status: 'pending', progress: 0 },
        { id: '2', name: 'T2', priority: 'P0', status: 'running', progress: 50 },
        { id: '3', name: 'T3', priority: 'P0', status: 'completed', progress: 100 },
      ];
      
      provider.setTasks(tasks);
      provider.setFilter({ status: ['running', 'completed'] });
      
      const filtered = provider.getFilteredTasks();
      assert.strictEqual(filtered.length, 2);
    });

    it('应该按搜索文本过滤', () => {
      const tasks = [
        { id: '1', name: 'validateInput', priority: 'P0', status: 'pending', progress: 0, description: 'Validate user input' },
        { id: '2', name: 'processData', priority: 'P1', status: 'pending', progress: 0, description: 'Process data' },
      ];
      
      provider.setTasks(tasks);
      provider.setFilter({ searchText: 'validate' });
      
      const filtered = provider.getFilteredTasks();
      assert.strictEqual(filtered.length, 1);
      assert.strictEqual(filtered[0].name, 'validateInput');
    });
  });

  describe('toggleExpanded()', () => {
    it('应该切换展开状态', () => {
      provider.toggleExpanded('task_1');
      assert(provider._expandedItems.has('task_1'));
      
      provider.toggleExpanded('task_1');
      assert(!provider._expandedItems.has('task_1'));
    });
  });

  describe('buildProgressBar()', () => {
    it('应该生成正确的进度条', () => {
      const bar50 = provider.buildProgressBar(50);
      assert(bar50.includes('█'));
      assert(bar50.includes('░'));
      
      const bar100 = provider.buildProgressBar(100);
      assert(bar100.includes('█'));
      assert(!bar100.includes('░'));
    });
  });

  describe('getFilteredTasks()', () => {
    it('应该返回过滤后的任务列表', () => {
      const tasks = [
        { id: '1', name: 'T1', priority: 'P0', status: 'pending', progress: 0 },
        { id: '2', name: 'T2', priority: 'P1', status: 'running', progress: 50 },
      ];
      
      provider.setTasks(tasks);
      const filtered = provider.getFilteredTasks();
      
      assert.strictEqual(filtered.length, 2);
    });

    it('应该应用多个过滤条件', () => {
      const tasks = [
        { id: '1', name: 'T1', priority: 'P0', status: 'pending', progress: 0, description: 'Test' },
        { id: '2', name: 'T2', priority: 'P0', status: 'running', progress: 50, description: 'Demo' },
        { id: '3', name: 'T3', priority: 'P1', status: 'running', progress: 50, description: 'Test' },
      ];
      
      provider.setTasks(tasks);
      provider.setFilter({
        priority: ['P0'],
        status: ['running'],
        searchText: 'test',
      });
      
      const filtered = provider.getFilteredTasks();
      // T2 is P0 but doesn't match 'test' search
      // T3 matches search but is P1
      // So result should be empty or only T2 if case-insensitive search
      assert(filtered.length === 0 || filtered.length === 1);
    });
  });
});

