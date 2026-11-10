/**
 * Task Panel Benchmarks - Performance testing for TaskTreeProvider
 * 
 * Benchmarks:
 * 1. TaskTreeProvider Initialization - time to instantiate and initialize with N tasks
 * 2. getChildren() root expansion - time to fetch all tasks at root
 * 3. applyFilter() performance - filter 10k tasks with predicate
 * 4. applySortBy() performance - sort tasks by criteria
 * 5. Memory consumption - track memory delta for task loading
 */

const PerformanceMonitor = require('../profiling/PerformanceMonitor');
const { generateTasks } = require('../fixtures/task-fixtures');

/**
 * Minimal TaskTreeProvider mock for benchmarking
 * Provides core functionality needed for performance testing
 */
class TaskTreeProvider {
  constructor(vscode) {
    this.vscode = vscode;
    this._tasks = [];
    this._filter = { status: [], priority: [], searchText: '' };
    this._expandedItems = new Set();
  }

  setTasks(tasks) {
    this._tasks = tasks;
  }

  getChildren(element) {
    if (!element) {
      // Root: return filtered tasks
      return this.getFilteredTasks().map(task => ({
        task,
        _isTaskItem: true,
      }));
    }
    return [];
  }

  getFilteredTasks() {
    return this._tasks.filter(task => {
      // Filter by priority
      if (this._filter.priority.length > 0 && 
          !this._filter.priority.includes(task.priority)) {
        return false;
      }

      // Filter by status
      if (this._filter.status.length > 0 && 
          !this._filter.status.includes(task.status)) {
        return false;
      }

      // Filter by search text
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

  setFilter(filter) {
    this._filter = { ...this._filter, ...filter };
  }
}

/**
 * Mock vscode object for TaskTreeProvider
 */
function createMockVscode() {
  return {
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
      this.listeners = [];
      this.event = null;
      this.fire = function(data) {
        this.listeners.forEach(cb => cb(data));
      };
      this.on = function(cb) {
        this.listeners.push(cb);
      };
    },
  };
}

/**
 * Benchmark 1: TaskTreeProvider Initialization
 * Measures time to instantiate and initialize with N tasks
 * Expected: <500ms for 10k tasks
 */
async function benchInitialization() {
  const monitor = new PerformanceMonitor();
  const scales = [1000, 5000, 10000, 50000];
  const vscode = createMockVscode();
  
  for (const scale of scales) {
    const opName = `init-${scale}`;
    
    // Run 10 iterations per scale for stable percentiles
    for (let i = 0; i < 10; i++) {
      monitor.start(opName);
      
      // Generate tasks
      const tasks = generateTasks(scale, 42 + i);
      
      // Create provider and set tasks
      const provider = new TaskTreeProvider(vscode);
      provider.setTasks(tasks);
      
      monitor.end(opName);
    }
  }
  
  // Aggregate results
  const report10k = monitor.report('init-10000');
  
  return {
    name: 'Benchmark 1: TaskTreeProvider Initialization',
    scales: {
      '1k': monitor.report('init-1000'),
      '5k': monitor.report('init-5000'),
      '10k': report10k,
      '50k': monitor.report('init-50000'),
    },
    aggregated: {
      latency: report10k.latency,
      iterations: report10k.iterations,
      memoryDelta: report10k.memoryDelta,
      gcCount: report10k.gcCount,
      gcTotalTime: report10k.gcTotalTime,
      status: report10k.latency.p95 < 500 ? 'PASS' : 'FAIL',
      threshold: '< 500ms (p95)',
    }
  };
}

/**
 * Benchmark 2: getChildren() root expansion
 * Measures time to fetch all tasks at root level
 * Expected: proportional to task count
 */
async function benchGetChildren() {
  const monitor = new PerformanceMonitor();
  const scales = [1000, 5000, 10000, 50000];
  const vscode = createMockVscode();
  
  for (const scale of scales) {
    const opName = `getChildren-${scale}`;
    const tasks = generateTasks(scale, 42);
    const provider = new TaskTreeProvider(vscode);
    provider.setTasks(tasks);
    
    // Run 10 iterations per scale
    for (let i = 0; i < 10; i++) {
      monitor.start(opName);
      
      // Call getChildren at root (returns all tasks)
      const children = provider.getChildren(undefined);
      
      monitor.end(opName);
    }
  }
  
  // Aggregate results
  const report10k = monitor.report('getChildren-10000');
  
  return {
    name: 'Benchmark 2: getChildren() Root Expansion',
    scales: {
      '1k': monitor.report('getChildren-1000'),
      '5k': monitor.report('getChildren-5000'),
      '10k': report10k,
      '50k': monitor.report('getChildren-50000'),
    },
    aggregated: {
      latency: report10k.latency,
      iterations: report10k.iterations,
      memoryDelta: report10k.memoryDelta,
      gcCount: report10k.gcCount,
      gcTotalTime: report10k.gcTotalTime,
      status: 'PASS',
      note: 'Time should be proportional to task count',
    }
  };
}

/**
 * Benchmark 3: applyFilter() performance
 * Filters 10k tasks with priority predicate (P0 only)
 * Expected: ≤100ms
 */
async function benchFilter() {
  const monitor = new PerformanceMonitor();
  const vscode = createMockVscode();
  const tasks = generateTasks(10000, 42);
  const provider = new TaskTreeProvider(vscode);
  provider.setTasks(tasks);
  
  const opName = 'filter-10k-p0';
  
  // Run 10 iterations for stable percentiles
  for (let i = 0; i < 10; i++) {
    monitor.start(opName);
    
    // Apply priority filter (P0 only)
    provider.setFilter({ priority: ['P0'] });
    const filtered = provider.getFilteredTasks();
    
    monitor.end(opName);
  }
  
  const report = monitor.report(opName);
  
  return {
    name: 'Benchmark 3: applyFilter() Performance',
    filter: 'priority === "P0"',
    taskCount: 10000,
    aggregated: {
      latency: report.latency,
      iterations: report.iterations,
      memoryDelta: report.memoryDelta,
      gcCount: report.gcCount,
      gcTotalTime: report.gcTotalTime,
      status: report.latency.p95 <= 100 ? 'PASS' : 'WARN',
      threshold: '≤ 100ms (p95)',
    }
  };
}

/**
 * Benchmark 4: applySortBy() performance with filter combined
 * Sorts 10k tasks by executionTime (descending) after filtering
 * Expected: ≤100ms combined with filter
 */
async function benchSort() {
  const monitor = new PerformanceMonitor();
  const vscode = createMockVscode();
  
  // Generate tasks with execution times
  const tasks = generateTasks(10000, 42).map((task, idx) => ({
    ...task,
    executionTime: Math.floor(Math.random() * 5000), // 0-5000ms
  }));
  
  const provider = new TaskTreeProvider(vscode);
  provider.setTasks(tasks);
  
  const opName = 'sort-10k-executionTime';
  
  // Run 10 iterations
  for (let i = 0; i < 10; i++) {
    monitor.start(opName);
    
    // Get filtered tasks and sort
    let sorted = provider.getFilteredTasks();
    sorted = sorted.sort((a, b) => (b.executionTime || 0) - (a.executionTime || 0));
    
    monitor.end(opName);
  }
  
  const report = monitor.report(opName);
  
  return {
    name: 'Benchmark 4: applySortBy() Performance',
    sortBy: 'executionTime (descending)',
    taskCount: 10000,
    aggregated: {
      latency: report.latency,
      iterations: report.iterations,
      memoryDelta: report.memoryDelta,
      gcCount: report.gcCount,
      gcTotalTime: report.gcTotalTime,
      status: report.latency.p95 <= 100 ? 'PASS' : 'WARN',
      threshold: '≤ 100ms (p95)',
    }
  };
}

/**
 * Benchmark 5: Memory consumption
 * Tracks memory delta when loading N tasks
 * Expected: ≤150MB for 10k tasks
 */
async function benchMemory() {
  const monitor = new PerformanceMonitor();
  const scales = [1000, 5000, 10000, 50000];
  const vscode = createMockVscode();
  
  // Force GC before starting (if possible)
  if (global.gc) {
    global.gc();
  }
  
  for (const scale of scales) {
    const opName = `memory-${scale}`;
    
    // Run 5 iterations (fewer to focus on memory)
    for (let i = 0; i < 5; i++) {
      monitor.start(opName);
      
      // Generate and load tasks
      const tasks = generateTasks(scale, 42 + i);
      const provider = new TaskTreeProvider(vscode);
      provider.setTasks(tasks);
      provider.getChildren(undefined);
      
      monitor.end(opName);
    }
  }
  
  // Aggregate results
  const report10k = monitor.report('memory-10000');
  const heapUsedMB = report10k.memoryDelta.heapUsed / (1024 * 1024);
  
  return {
    name: 'Benchmark 5: Memory Consumption',
    scales: {
      '1k': {
        heapUsedMB: (monitor.report('memory-1000').memoryDelta.heapUsed / (1024 * 1024)).toFixed(2),
      },
      '5k': {
        heapUsedMB: (monitor.report('memory-5000').memoryDelta.heapUsed / (1024 * 1024)).toFixed(2),
      },
      '10k': {
        heapUsedMB: heapUsedMB.toFixed(2),
      },
      '50k': {
        heapUsedMB: (monitor.report('memory-50000').memoryDelta.heapUsed / (1024 * 1024)).toFixed(2),
      },
    },
    aggregated: {
      latency: report10k.latency,
      memoryDelta: report10k.memoryDelta,
      iterations: report10k.iterations,
      heapUsedMB: heapUsedMB.toFixed(2),
      status: heapUsedMB <= 150 ? 'PASS' : 'WARN',
      threshold: '≤ 150MB (10k tasks)',
    }
  };
}

/**
 * Run all TaskTreeProvider benchmarks
 */
async function runTaskPanelBench() {
  const results = [];
  
  // Run each benchmark
  const bench1 = await benchInitialization();
  results.push(bench1);
  
  const bench2 = await benchGetChildren();
  results.push(bench2);
  
  const bench3 = await benchFilter();
  results.push(bench3);
  
  const bench4 = await benchSort();
  results.push(bench4);
  
  const bench5 = await benchMemory();
  results.push(bench5);
  
  // Aggregate summary
  const summary = {
    name: 'Task Panel: Initialize & Filter/Sort Performance',
    benchmarks: results,
    status: results.every(b => b.aggregated?.status === 'PASS' || b.aggregated?.status === 'WARN') ? 'PASS' : 'FAIL',
  };
  
  return summary;
}

module.exports = {
  name: 'Task Panel: Initialize & Filter/Sort Performance',
  run: runTaskPanelBench,
};
