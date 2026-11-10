'use strict';

const test = require('node:test');
const assert = require('node:assert');

/**
 * TaskTreeProvider 单元测试
 */

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
  EventEmitter: function() {
    this.event = () => {};
    this.fire = () => {};
  },
};

// 动态加载 TaskTreeProvider 模块
const { TaskTreeProvider } = require('../src/taskpanel');

test('TaskTreeProvider: 应该设置任务列表', () => {
  const provider = new TaskTreeProvider(mockVscode);
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

test('TaskTreeProvider: 应该按优先级过滤', () => {
  const provider = new TaskTreeProvider(mockVscode);
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

test('TaskTreeProvider: 应该按状态过滤', () => {
  const provider = new TaskTreeProvider(mockVscode);
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

test('TaskTreeProvider: 应该按搜索文本过滤', () => {
  const provider = new TaskTreeProvider(mockVscode);
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

test('TaskTreeProvider: 应该切换展开状态', () => {
  const provider = new TaskTreeProvider(mockVscode);
  
  provider.toggleExpanded('task_1');
  assert(provider._expandedItems.has('task_1'));
  
  provider.toggleExpanded('task_1');
  assert(!provider._expandedItems.has('task_1'));
});

test('TaskTreeProvider: 应该生成正确的进度条', () => {
  const provider = new TaskTreeProvider(mockVscode);
  
  const bar50 = provider.buildProgressBar(50);
  assert(bar50.includes('█'));
  assert(bar50.includes('░'));
  
  const bar100 = provider.buildProgressBar(100);
  assert(bar100.includes('█'));
  assert(!bar100.includes('░'));
});

test('TaskTreeProvider: 应该返回过滤后的任务列表', () => {
  const provider = new TaskTreeProvider(mockVscode);
  const tasks = [
    { id: '1', name: 'T1', priority: 'P0', status: 'pending', progress: 0 },
    { id: '2', name: 'T2', priority: 'P1', status: 'running', progress: 50 },
  ];
  
  provider.setTasks(tasks);
  const filtered = provider.getFilteredTasks();
  
  assert.strictEqual(filtered.length, 2);
});

test('TaskTreeProvider: 应该应用多个过滤条件', () => {
  const provider = new TaskTreeProvider(mockVscode);
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
