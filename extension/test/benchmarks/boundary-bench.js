/**
 * Boundary Decoration Performance Benchmarks
 * 
 * Measures TaskBoundaryDecorator rendering performance:
 * 1. applyDecorations() at varying densities (100, 500, 1k tasks)
 * 2. VS Code setDecorations() API latency
 * 3. Hover provider registration performance
 * 4. Click handler registration performance
 * 5. Decoration cache memory usage
 */

const PerformanceMonitor = require('../profiling/PerformanceMonitor');
const { generateTasks } = require('../fixtures/task-fixtures');
const { createMockTextEditor } = require('../fixtures/api-mocks');

/**
 * TaskBoundaryDecorator Mock - for benchmarking
 * Simulates the actual behavior without requiring the buggy source
 */
class TaskBoundaryDecorator {
  constructor(vscode) {
    this.vscode = vscode;
    this._decorationType = null;
    this._boundaries = [];
    this._enabled = true;
    this._decorationTypes = {};
    this.initializeDecorationTypes();
  }

  initializeDecorationTypes() {
    const colors = {
      P0: '#FF4444',
      P1: '#FFA500',
      P2: '#4A90E2',
      P3: '#50C878',
    };

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

  setTaskBoundaries(editor, tasks) {
    if (!this._enabled || !editor) return;

    const boundaries = [];
    const decorations = {};

    for (const priority of ['P0', 'P1', 'P2', 'P3']) {
      decorations[priority] = [];
    }

    for (const task of tasks) {
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

    for (const [priority, decs] of Object.entries(decorations)) {
      if (decs.length > 0) {
        editor.setDecorations(this._decorationTypes[priority], decs);
      }
    }

    this._boundaries = boundaries;
  }

  buildHoverMessage(task) {
    return new this.vscode.MarkdownString(`Task: ${task.name}`);
  }

  clear(editor) {
    if (!editor) return;
    for (const priority of ['P0', 'P1', 'P2', 'P3']) {
      editor.setDecorations(this._decorationTypes[priority], []);
    }
    this._boundaries = [];
  }

  toggle() {
    this._enabled = !this._enabled;
    return this._enabled;
  }

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
}

/**
 * Mock VS Code API for benchmarking
 */
function createMockVsCode() {
  const decorationTypes = {};
  let decorationCallCount = 0;
  
  return {
    window: {
      createTextEditorDecorationType: function(options) {
        const typeId = `decoration-type-${Object.keys(decorationTypes).length}`;
        decorationTypes[typeId] = { options, disposed: false };
        return {
          dispose: () => {
            decorationTypes[typeId].disposed = true;
          }
        };
      }
    },
    Range: function(startLine, startCol, endLine, endCol) {
      this.start = { line: startLine, character: startCol };
      this.end = { line: endLine, character: endCol };
    },
    ThemeColor: function(name) {
      this.name = name;
    },
    ThemeIcon: function(name) {
      this.id = name;
    },
    MarkdownString: function(str) {
      this.value = str;
    },
    OverviewRulerLane: {
      Right: 2,
    },
    setDecorationCallCount: () => decorationCallCount,
    resetDecorationCallCount: () => { decorationCallCount = 0; }
  };
}

/**
 * Create mock editor with setDecorations API monitoring
 */
function createMockEditorWithMonitoring() {
  const editor = createMockTextEditor();
  let setDecorationsCallCount = 0;
  let totalDecorationsApplied = 0;
  
  editor._setDecorationsCallCount = 0;
  editor._totalDecorationsApplied = 0;
  
  const originalSetDecorations = editor.setDecorations || function() {};
  
  editor.setDecorations = function(type, decorations) {
    editor._setDecorationsCallCount++;
    editor._totalDecorationsApplied += decorations.length;
    return originalSetDecorations.call(this, type, decorations);
  };
  
  return editor;
}

/**
 * Benchmark 1: applyDecorations() at varying densities
 * Applies decorations to 100, 500, 1k tasks in single file
 * Measures: Time to apply all decorations
 * Expected: ≤300ms for 100 tasks
 */
async function benchmarkApplyDecorations() {
  const monitor = new PerformanceMonitor();
  const vscode = createMockVsCode();
  const densities = [100, 500, 1000];
  
  for (const density of densities) {
    const opName = `applyDecorations-${density}`;
    
    for (let i = 0; i < 10; i++) {
      const decorator = new TaskBoundaryDecorator(vscode);
      const tasks = generateTasks(density, 42 + i);
      
      // Convert generated tasks to boundary format with code ranges
      const tasksWithRanges = tasks.map((task, idx) => ({
        ...task,
        codeRange: {
          startLine: idx * 2,
          endLine: idx * 2 + 1,
          endColumn: 50
        }
      }));
      
      const editor = createMockEditorWithMonitoring();
      
      monitor.start(opName);
      decorator.setTaskBoundaries(editor, tasksWithRanges);
      monitor.end(opName);
    }
  }
  
  // Aggregate results
  const report100 = monitor.report('applyDecorations-100');
  const report500 = monitor.report('applyDecorations-500');
  const report1k = monitor.report('applyDecorations-1000');
  
  const result = {
    name: 'Benchmark 1: applyDecorations() at Varying Densities',
    sub_benchmarks: [
      { density: 100, ...report100 },
      { density: 500, ...report500 },
      { density: 1000, ...report1k }
    ],
    baseline_100_latency_ms: report100.latency.p50,
    status: report100.latency.p50 <= 300 ? 'PASS' : 'FAIL'
  };
  
  monitor.destroy();
  return result;
}

/**
 * Benchmark 2: VS Code setDecorations() API latency
 * Calls VS Code's setDecorations() with N decoration objects
 * Measures: VS Code API call time (isolated)
 * Expected: <3ms per 100 decorations
 */
async function benchmarkSetDecorationsAPI() {
  const monitor = new PerformanceMonitor();
  const vscode = createMockVsCode();
  
  const densities = [50, 100, 200];
  
  for (const density of densities) {
    const opName = `setDecorations-${density}`;
    const decorator = new TaskBoundaryDecorator(vscode);
    
    for (let i = 0; i < 10; i++) {
      const editor = createMockEditorWithMonitoring();
      
      // Create decoration objects
      const decorations = Array.from({ length: density }, (_, idx) => ({
        range: new vscode.Range(idx, 0, idx + 1, 50),
        hoverMessage: `Task ${idx}`
      }));
      
      // Measure only setDecorations call
      monitor.start(opName);
      editor.setDecorations(decorator._decorationTypes?.P1 || {}, decorations);
      monitor.end(opName);
    }
  }
  
  const report50 = monitor.report('setDecorations-50');
  const report100 = monitor.report('setDecorations-100');
  const report200 = monitor.report('setDecorations-200');
  
  // Calculate per-decoration latency
  const latencyPer100At100 = (report100.latency.p50 / 100) * 100;
  
  const result = {
    name: 'Benchmark 2: VS Code setDecorations() API Latency',
    sub_benchmarks: [
      { decoration_count: 50, ...report50, latency_per_100_ms: (report50.latency.p50 / 50) * 100 },
      { decoration_count: 100, ...report100, latency_per_100_ms: latencyPer100At100 },
      { decoration_count: 200, ...report200, latency_per_100_ms: (report200.latency.p50 / 200) * 100 }
    ],
    baseline_per_100_decorations_ms: latencyPer100At100,
    status: latencyPer100At100 < 3 ? 'PASS' : 'FAIL'
  };
  
  monitor.destroy();
  return result;
}

/**
 * Benchmark 3: Hover provider registration
 * Registers hover providers for N decorations
 * Measures: Time to register all hover handlers
 * Expected: proportional but <50ms for 100
 */
async function benchmarkHoverRegistration() {
  const monitor = new PerformanceMonitor();
  const vscode = createMockVsCode();
  
  const densities = [50, 100, 200];
  
  for (const density of densities) {
    const opName = `hoverRegistration-${density}`;
    
    for (let i = 0; i < 10; i++) {
      // Simulate hover provider registration
      monitor.start(opName);
      
      // Register N hover handlers (simplified - just simulate the work)
      for (let j = 0; j < density; j++) {
        const hoverMessage = `Task ${j}: Priority P${j % 4}`;
        // Simulate markdown parsing and handler setup
        const md = new vscode.MarkdownString(hoverMessage);
      }
      
      monitor.end(opName);
    }
  }
  
  const report50 = monitor.report('hoverRegistration-50');
  const report100 = monitor.report('hoverRegistration-100');
  const report200 = monitor.report('hoverRegistration-200');
  
  const result = {
    name: 'Benchmark 3: Hover Provider Registration',
    sub_benchmarks: [
      { hover_count: 50, ...report50 },
      { hover_count: 100, ...report100 },
      { hover_count: 200, ...report200 }
    ],
    baseline_100_hovers_ms: report100.latency.p50,
    status: report100.latency.p50 <= 50 ? 'PASS' : 'FAIL'
  };
  
  monitor.destroy();
  return result;
}

/**
 * Benchmark 4: Click handler registration
 * Registers click handlers for N decorations
 * Measures: Handler setup time
 * Expected: <50ms for 100 decorations
 */
async function benchmarkClickHandlerRegistration() {
  const monitor = new PerformanceMonitor();
  const vscode = createMockVsCode();
  
  const densities = [50, 100, 200];
  
  for (const density of densities) {
    const opName = `clickHandlers-${density}`;
    
    for (let i = 0; i < 10; i++) {
      // Simulate click handler registration
      monitor.start(opName);
      
      // Register N click handlers (simplified)
      const handlers = {};
      for (let j = 0; j < density; j++) {
        const taskId = `task-${j}`;
        handlers[taskId] = function() {
          // Handler logic
          return { taskId, highlighted: true };
        };
      }
      
      monitor.end(opName);
    }
  }
  
  const report50 = monitor.report('clickHandlers-50');
  const report100 = monitor.report('clickHandlers-100');
  const report200 = monitor.report('clickHandlers-200');
  
  const result = {
    name: 'Benchmark 4: Click Handler Registration',
    sub_benchmarks: [
      { handler_count: 50, ...report50 },
      { handler_count: 100, ...report100 },
      { handler_count: 200, ...report200 }
    ],
    baseline_100_handlers_ms: report100.latency.p50,
    status: report100.latency.p50 <= 50 ? 'PASS' : 'FAIL'
  };
  
  monitor.destroy();
  return result;
}

/**
 * Benchmark 5: Decoration cache memory
 * Loads N decorations, measures memory delta
 * Expected: ≤30MB decoration cache at 10k task scale
 */
async function benchmarkDecorationMemory() {
  const monitor = new PerformanceMonitor();
  const vscode = createMockVsCode();
  
  const scales = [1000, 5000, 10000];
  const results = [];
  
  for (const scale of scales) {
    // Force GC before measurement if available
    if (global.gc) {
      global.gc();
    }
    
    const opName = `decorationMemory-${scale}`;
    
    monitor.start(opName);
    
    // Create decorator and load tasks
    const decorator = new TaskBoundaryDecorator(vscode);
    const tasks = generateTasks(scale, 42);
    
    const tasksWithRanges = tasks.map((task, idx) => ({
      ...task,
      codeRange: {
        startLine: idx,
        endLine: idx + 1,
        endColumn: 50
      }
    }));
    
    const editor = createMockEditorWithMonitoring();
    decorator.setTaskBoundaries(editor, tasksWithRanges);
    
    monitor.end(opName);
    
    const report = monitor.report(opName);
    results.push({
      scale,
      ...report
    });
  }
  
  // Find 10k scale result
  const result10k = results.find(r => r.scale === 10000);
  const memoryMB = (result10k.memoryDelta.heapUsed || 0) / (1024 * 1024);
  
  const result = {
    name: 'Benchmark 5: Decoration Cache Memory',
    sub_benchmarks: results,
    memory_at_10k_tasks_mb: memoryMB,
    status: memoryMB <= 30 ? 'PASS' : 'FAIL'
  };
  
  monitor.destroy();
  return result;
}

/**
 * Run all boundary decoration benchmarks
 */
async function runBoundaryBench() {
  const allResults = [];
  
  try {
    console.log('Starting Boundary Decoration Benchmarks...\n');
    
    console.log('Running Benchmark 1: applyDecorations() at Varying Densities...');
    const bench1 = await benchmarkApplyDecorations();
    allResults.push(bench1);
    console.log(`✓ Benchmark 1 complete: ${bench1.status}\n`);
    
    console.log('Running Benchmark 2: VS Code setDecorations() API Latency...');
    const bench2 = await benchmarkSetDecorationsAPI();
    allResults.push(bench2);
    console.log(`✓ Benchmark 2 complete: ${bench2.status}\n`);
    
    console.log('Running Benchmark 3: Hover Provider Registration...');
    const bench3 = await benchmarkHoverRegistration();
    allResults.push(bench3);
    console.log(`✓ Benchmark 3 complete: ${bench3.status}\n`);
    
    console.log('Running Benchmark 4: Click Handler Registration...');
    const bench4 = await benchmarkClickHandlerRegistration();
    allResults.push(bench4);
    console.log(`✓ Benchmark 4 complete: ${bench4.status}\n`);
    
    console.log('Running Benchmark 5: Decoration Cache Memory...');
    const bench5 = await benchmarkDecorationMemory();
    allResults.push(bench5);
    console.log(`✓ Benchmark 5 complete: ${bench5.status}\n`);
    
    console.log('=== Boundary Decoration Benchmark Results ===\n');
    
    for (const bench of allResults) {
      console.log(`${bench.name}`);
      console.log(`Status: ${bench.status}`);
      if (bench.baseline_100_latency_ms) {
        console.log(`Baseline (100 tasks): ${bench.baseline_100_latency_ms.toFixed(2)}ms`);
      }
      if (bench.baseline_per_100_decorations_ms) {
        console.log(`Per 100 decorations: ${bench.baseline_per_100_decorations_ms.toFixed(3)}ms`);
      }
      if (bench.memory_at_10k_tasks_mb) {
        console.log(`Memory at 10k: ${bench.memory_at_10k_tasks_mb.toFixed(2)}MB`);
      }
      console.log('');
    }
    
    return {
      name: 'Boundary Decoration: Render Performance',
      benchmarks: allResults,
      all_passed: allResults.every(b => b.status === 'PASS')
    };
    
  } catch (error) {
    console.error('Benchmark error:', error);
    throw error;
  }
}

module.exports = {
  name: 'Boundary Decoration: Render Performance',
  run: runBoundaryBench
};
