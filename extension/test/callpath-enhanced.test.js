'use strict';

const assert = require('assert');

/**
 * EnhancedCallPathProvider 单元测试
 */
describe('EnhancedCallPathProvider', () => {
  let provider;
  
  beforeEach(() => {
    const mockVscode = {
      TreeItemCollapsibleState: {
        None: 0,
        Collapsed: 1,
        Expanded: 2,
      },
      TreeItem: function(label, state) {
        this.label = label;
        this.collapsibleState = state;
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
      EventEmitter: function() {
        this.fire = function() {};
      },
    };
    
    const EnhancedCallPathProvider = require('../src/callpath-enhanced').EnhancedCallPathProvider;
    provider = new EnhancedCallPathProvider(mockVscode);
  });

  describe('setCallTree()', () => {
    it('应该设置调用树数据', () => {
      const callTree = {
        name: 'main',
        file: 'main.go',
        line: 1,
        column: 1,
        executionTime: 500,
        callCount: 1,
        memory: 128,
        cpu: 5,
        children: [],
      };
      
      provider.setCallTree(callTree);
      
      assert(provider._callTree !== null);
      assert.strictEqual(provider._callTree.name, 'main');
    });
  });

  describe('setSortBy()', () => {
    it('应该设置排序方式为执行时间', () => {
      provider.setSortBy('executionTime');
      assert.strictEqual(provider._sortBy, 'executionTime');
    });

    it('应该设置排序方式为名称', () => {
      provider.setSortBy('name');
      assert.strictEqual(provider._sortBy, 'name');
    });

    it('应该设置排序方式为调用次数', () => {
      provider.setSortBy('callCount');
      assert.strictEqual(provider._sortBy, 'callCount');
    });
  });

  describe('sortNodes()', () => {
    it('应该按执行时间排序', () => {
      const nodes = [
        { name: 'func1', executionTime: 100, callCount: 1 },
        { name: 'func2', executionTime: 500, callCount: 1 },
        { name: 'func3', executionTime: 200, callCount: 1 },
      ];
      
      provider.setSortBy('executionTime');
      const sorted = provider.sortNodes(nodes);
      
      assert.strictEqual(sorted[0].executionTime, 500);
      assert.strictEqual(sorted[1].executionTime, 200);
      assert.strictEqual(sorted[2].executionTime, 100);
    });

    it('应该按名称排序', () => {
      const nodes = [
        { name: 'zebra', executionTime: 100, callCount: 1 },
        { name: 'apple', executionTime: 200, callCount: 1 },
        { name: 'mango', executionTime: 150, callCount: 1 },
      ];
      
      provider.setSortBy('name');
      const sorted = provider.sortNodes(nodes);
      
      assert.strictEqual(sorted[0].name, 'apple');
      assert.strictEqual(sorted[1].name, 'mango');
      assert.strictEqual(sorted[2].name, 'zebra');
    });

    it('应该按调用次数排序', () => {
      const nodes = [
        { name: 'f1', executionTime: 100, callCount: 3 },
        { name: 'f2', executionTime: 200, callCount: 10 },
        { name: 'f3', executionTime: 150, callCount: 1 },
      ];
      
      provider.setSortBy('callCount');
      const sorted = provider.sortNodes(nodes);
      
      assert.strictEqual(sorted[0].callCount, 10);
      assert.strictEqual(sorted[1].callCount, 3);
      assert.strictEqual(sorted[2].callCount, 1);
    });
  });

  describe('search()', () => {
    it('应该按函数名搜索', () => {
      const callTree = {
        name: 'main',
        file: 'main.go',
        line: 1,
        column: 1,
        executionTime: 100,
        callCount: 1,
        memory: 10,
        cpu: 1,
        children: [
          {
            name: 'validateInput',
            file: 'validate.go',
            line: 10,
            column: 1,
            executionTime: 50,
            callCount: 1,
            memory: 5,
            cpu: 0.5,
            children: [],
          },
        ],
      };
      
      provider.setCallTree(callTree);
      const results = provider.search('validate');
      
      assert(results.length > 0);
      assert(results.some(n => n.name.includes('validate')));
    });

    it('应该按文件名搜索', () => {
      const callTree = {
        name: 'main',
        file: 'main.go',
        line: 1,
        column: 1,
        executionTime: 100,
        callCount: 1,
        memory: 10,
        cpu: 1,
        children: [],
      };
      
      provider.setCallTree(callTree);
      const results = provider.search('main.go');
      
      assert(results.length > 0);
      assert(results.some(n => n.file.includes('main.go')));
    });
  });

  describe('exportAsText()', () => {
    it('应该导出调用树为文本', () => {
      const callTree = {
        name: 'main',
        file: 'main.go',
        line: 1,
        column: 1,
        executionTime: 100,
        callCount: 1,
        memory: 10,
        cpu: 1,
        children: [
          {
            name: 'process',
            file: 'process.go',
            line: 10,
            column: 1,
            executionTime: 80,
            callCount: 1,
            memory: 8,
            cpu: 0.8,
            children: [],
          },
        ],
      };
      
      provider.setCallTree(callTree);
      const text = provider.exportAsText();
      
      assert(text.includes('main'));
      assert(text.includes('process'));
      assert(text.includes('100ms'));
    });

    it('应该包含树形结构', () => {
      const callTree = {
        name: 'root',
        file: 'root.go',
        line: 1,
        column: 1,
        executionTime: 100,
        callCount: 1,
        memory: 10,
        cpu: 1,
        children: [],
      };
      
      provider.setCallTree(callTree);
      const text = provider.exportAsText();
      
      assert(text.includes('└─'));
    });
  });
});

