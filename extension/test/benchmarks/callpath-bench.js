/**
 * Call Path Provider Benchmarks - Tree Processing Performance
 * 
 * Measures performance of EnhancedCallPathProvider tree operations:
 * - getChildren() at varying tree depths
 * - sortChildren() with varying child count
 * - Recursive tree expansion
 * - Search filtering on tree
 * - Tree structure memory usage
 */

const PerformanceMonitor = require('../profiling/PerformanceMonitor');
const { generateCallTree, getTreeStats } = require('../fixtures/tree-fixtures');

// Mock vscode API for provider initialization
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
    this.event = null;
    this.fire = function() {};
  },
};

/**
 * Benchmark 1: getChildren() at varying tree depths
 * Generates call trees with depth 10, 15, 20, 30
 * Measures time to getChildren() for different tree sizes
 */
async function benchmarkGetChildrenDepth() {
  const monitor = new PerformanceMonitor();
  const EnhancedCallPathProvider = require('../../src/callpath-enhanced').EnhancedCallPathProvider;
  
  const depths = [10, 15, 20, 30];
  const iterations = 10;
  
  for (const depth of depths) {
    const opName = `getChildren-depth-${depth}`;
    
    for (let i = 0; i < iterations; i++) {
      // Generate tree with deterministic seed
      monitor.start(opName);
      
      const tree = generateCallTree(depth, 3, 42 + i);
      const provider = new EnhancedCallPathProvider(mockVscode);
      provider.setCallTree(tree);
      
      // Measure getChildren() at different levels
      const root = provider.getChildren(undefined);
      if (root && root.length > 0) {
        provider.getChildren(root[0]);
      }
      
      monitor.end(opName);
    }
  }
  
  // Aggregate results from all depths
  const aggregated = {
    name: 'Call Path: getChildren() at Varying Tree Depths',
    benchmarks: [],
    totalIterations: 0
  };
  
  for (const depth of depths) {
    const report = monitor.report(`getChildren-depth-${depth}`);
    aggregated.benchmarks.push({
      depth,
      latency: report.latency,
      memoryDelta: report.memoryDelta,
      gcCount: report.gcCount,
      gcTotalTime: report.gcTotalTime
    });
    aggregated.totalIterations += report.iterations;
  }
  
  monitor.destroy();
  return aggregated;
}

/**
 * Benchmark 2: sortChildren() with varying child count
 * Generates node with 100, 500, 1k children
 * Measures time to sort children by execution time
 */
async function benchmarkSortChildren() {
  const monitor = new PerformanceMonitor();
  const EnhancedCallPathProvider = require('../../src/callpath-enhanced').EnhancedCallPathProvider;
  
  const childCounts = [100, 500, 1000];
  const iterations = 10;
  
  for (const count of childCounts) {
    const opName = `sortChildren-count-${count}`;
    
    for (let i = 0; i < iterations; i++) {
      monitor.start(opName);
      
      // Generate a parent node with many children
      const children = [];
      for (let j = 0; j < count; j++) {
        children.push({
          id: `child-${j}`,
          name: `Function ${j}`,
          line: Math.floor(Math.random() * 1000) + 1,
          executionTime: Math.random() * 1000,
          callCount: Math.floor(Math.random() * 100) + 1,
          children: []
        });
      }
      
      const parentNode = {
        id: 'parent',
        name: 'Parent Function',
        line: 1,
        children: children
      };
      
      const provider = new EnhancedCallPathProvider(mockVscode);
      provider.setCallTree(parentNode);
      provider.setSortBy('executionTime');
      
      // Measure sortNodes() call
      const root = provider.getChildren(undefined);
      if (root && root.length > 0) {
        provider.sortNodes(root[0].children);
      }
      
      monitor.end(opName);
    }
  }
  
  // Aggregate results
  const aggregated = {
    name: 'Call Path: sortChildren() with Varying Child Count',
    benchmarks: [],
    totalIterations: 0
  };
  
  for (const count of childCounts) {
    const report = monitor.report(`sortChildren-count-${count}`);
    aggregated.benchmarks.push({
      childCount: count,
      latency: report.latency,
      memoryDelta: report.memoryDelta,
      gcCount: report.gcCount,
      gcTotalTime: report.gcTotalTime
    });
    aggregated.totalIterations += report.iterations;
  }
  
  monitor.destroy();
  return aggregated;
}

/**
 * Benchmark 3: Recursive tree expansion
 * Expands tree 3 levels deep from root
 * Measures total expansion time
 */
async function benchmarkRecursiveExpansion() {
  const monitor = new PerformanceMonitor();
  const EnhancedCallPathProvider = require('../../src/callpath-enhanced').EnhancedCallPathProvider;
  
  const depths = [10, 15, 20, 30];
  const expandDepth = 3; // How many levels to expand
  const iterations = 10;
  
  for (const depth of depths) {
    const opName = `recursive-expand-depth-${depth}`;
    
    for (let i = 0; i < iterations; i++) {
      monitor.start(opName);
      
      const tree = generateCallTree(depth, 3, 42 + i);
      const provider = new EnhancedCallPathProvider(mockVscode);
      provider.setCallTree(tree);
      
      // Recursively expand tree to specified depth
      function expandTree(node, currentDepth) {
        if (currentDepth >= expandDepth || !node) return;
        
        const children = provider.getChildren(node);
        for (const child of children) {
          expandTree(child, currentDepth + 1);
        }
      }
      
      const root = provider.getChildren(undefined);
      if (root && root.length > 0) {
        expandTree(root[0], 0);
      }
      
      monitor.end(opName);
    }
  }
  
  // Aggregate results
  const aggregated = {
    name: 'Call Path: Recursive Tree Expansion',
    expandDepth: 3,
    benchmarks: [],
    totalIterations: 0
  };
  
  for (const depth of depths) {
    const report = monitor.report(`recursive-expand-depth-${depth}`);
    aggregated.benchmarks.push({
      treeDepth: depth,
      latency: report.latency,
      memoryDelta: report.memoryDelta,
      gcCount: report.gcCount,
      gcTotalTime: report.gcTotalTime
    });
    aggregated.totalIterations += report.iterations;
  }
  
  monitor.destroy();
  return aggregated;
}

/**
 * Benchmark 4: Search filtering on tree
 * Search for functions in tree (depth 20)
 * Measures time to filter/search
 */
async function benchmarkSearchFiltering() {
  const monitor = new PerformanceMonitor();
  const EnhancedCallPathProvider = require('../../src/callpath-enhanced').EnhancedCallPathProvider;
  
  const searchPatterns = [
    { query: 'Function', expectedMatches: 'many' },
    { query: 'Function 5', expectedMatches: 'few' },
    { query: 'Function 99', expectedMatches: 'few' }
  ];
  
  const iterations = 10;
  
  for (const pattern of searchPatterns) {
    const opName = `search-${pattern.query.replace(/\s/g, '-')}`;
    
    for (let i = 0; i < iterations; i++) {
      monitor.start(opName);
      
      // Generate tree with depth 20 for search test
      const tree = generateCallTree(20, 3, 42 + i);
      const provider = new EnhancedCallPathProvider(mockVscode);
      provider.setCallTree(tree);
      
      // Perform search
      const results = provider.search(pattern.query);
      
      monitor.end(opName);
    }
  }
  
  // Aggregate results
  const aggregated = {
    name: 'Call Path: Search Filtering on Tree',
    treeDepth: 20,
    benchmarks: [],
    totalIterations: 0
  };
  
  for (const pattern of searchPatterns) {
    const opName = `search-${pattern.query.replace(/\s/g, '-')}`;
    const report = monitor.report(opName);
    aggregated.benchmarks.push({
      searchQuery: pattern.query,
      expectedMatches: pattern.expectedMatches,
      latency: report.latency,
      memoryDelta: report.memoryDelta,
      gcCount: report.gcCount,
      gcTotalTime: report.gcTotalTime
    });
    aggregated.totalIterations += report.iterations;
  }
  
  monitor.destroy();
  return aggregated;
}

/**
 * Benchmark 5: Tree structure memory usage
 * Load complete tree, measure memory delta
 */
async function benchmarkMemoryUsage() {
  const monitor = new PerformanceMonitor();
  const EnhancedCallPathProvider = require('../../src/callpath-enhanced').EnhancedCallPathProvider;
  
  const depths = [10, 15, 20, 25];
  const iterations = 10;
  
  for (const depth of depths) {
    const opName = `memory-depth-${depth}`;
    
    for (let i = 0; i < iterations; i++) {
      monitor.start(opName);
      
      const tree = generateCallTree(depth, 3, 42 + i);
      const stats = getTreeStats(tree);
      
      const provider = new EnhancedCallPathProvider(mockVscode);
      provider.setCallTree(tree);
      
      // Force accessing all nodes to ensure they're in memory
      function traverseAll(node) {
        provider.getChildren(node);
        if (node && node.children) {
          for (const child of node.children) {
            traverseAll(child);
          }
        }
      }
      
      const root = provider.getChildren(undefined);
      if (root && root.length > 0) {
        traverseAll(root[0]);
      }
      
      monitor.end(opName);
    }
  }
  
  // Aggregate results with tree stats
  const aggregated = {
    name: 'Call Path: Tree Structure Memory Usage',
    benchmarks: [],
    totalIterations: 0
  };
  
  for (const depth of depths) {
    const opName = `memory-depth-${depth}`;
    const report = monitor.report(opName);
    
    // Generate sample tree to get stats
    const tree = generateCallTree(depth, 3, 42);
    const stats = getTreeStats(tree);
    
    aggregated.benchmarks.push({
      depth,
      nodeCount: stats.nodeCount,
      maxDepth: stats.maxDepth,
      avgChildren: stats.avgChildren,
      latency: report.latency,
      memoryDelta: report.memoryDelta,
      gcCount: report.gcCount,
      gcTotalTime: report.gcTotalTime
    });
    aggregated.totalIterations += report.iterations;
  }
  
  monitor.destroy();
  return aggregated;
}

/**
 * Main benchmark runner - executes all benchmarks and returns results
 */
async function runCallPathBench() {
  console.log('🚀 Starting Call Path Provider Benchmarks...\n');
  
  const results = {
    benchmarks: []
  };
  
  try {
    console.log('📊 Benchmark 1: getChildren() at Varying Tree Depths');
    const bench1 = await benchmarkGetChildrenDepth();
    results.benchmarks.push(bench1);
    console.log(`   ✓ Completed ${bench1.totalIterations} iterations`);
    console.log(`   Latency (p50/p95/p99): ${bench1.benchmarks.map(b => `${b.latency.p50.toFixed(2)}/${b.latency.p95.toFixed(2)}/${b.latency.p99.toFixed(2)}ms`).join(', ')}\n`);
    
    console.log('📊 Benchmark 2: sortChildren() with Varying Child Count');
    const bench2 = await benchmarkSortChildren();
    results.benchmarks.push(bench2);
    console.log(`   ✓ Completed ${bench2.totalIterations} iterations`);
    console.log(`   Latency (p50/p95/p99): ${bench2.benchmarks.map(b => `${b.latency.p50.toFixed(2)}/${b.latency.p95.toFixed(2)}/${b.latency.p99.toFixed(2)}ms`).join(', ')}\n`);
    
    console.log('📊 Benchmark 3: Recursive Tree Expansion');
    const bench3 = await benchmarkRecursiveExpansion();
    results.benchmarks.push(bench3);
    console.log(`   ✓ Completed ${bench3.totalIterations} iterations`);
    console.log(`   Latency (p50/p95/p99): ${bench3.benchmarks.map(b => `${b.latency.p50.toFixed(2)}/${b.latency.p95.toFixed(2)}/${b.latency.p99.toFixed(2)}ms`).join(', ')}\n`);
    
    console.log('📊 Benchmark 4: Search Filtering on Tree');
    const bench4 = await benchmarkSearchFiltering();
    results.benchmarks.push(bench4);
    console.log(`   ✓ Completed ${bench4.totalIterations} iterations`);
    console.log(`   Latency (p50/p95/p99): ${bench4.benchmarks.map(b => `${b.latency.p50.toFixed(2)}/${b.latency.p95.toFixed(2)}/${b.latency.p99.toFixed(2)}ms`).join(', ')}\n`);
    
    console.log('📊 Benchmark 5: Tree Structure Memory Usage');
    const bench5 = await benchmarkMemoryUsage();
    results.benchmarks.push(bench5);
    console.log(`   ✓ Completed ${bench5.totalIterations} iterations`);
    console.log(`   Latency (p50/p95/p99): ${bench5.benchmarks.map(b => `${b.latency.p50.toFixed(2)}/${b.latency.p95.toFixed(2)}/${b.latency.p99.toFixed(2)}ms`).join(', ')}\n`);
    
    console.log('✅ All benchmarks completed successfully!\n');
  } catch (error) {
    console.error('❌ Benchmark execution failed:', error);
    throw error;
  }
  
  return results;
}

module.exports = {
  name: 'Call Path: Tree Processing Performance',
  run: runCallPathBench,
  benchmarks: {
    benchmarkGetChildrenDepth,
    benchmarkSortChildren,
    benchmarkRecursiveExpansion,
    benchmarkSearchFiltering,
    benchmarkMemoryUsage
  }
};
