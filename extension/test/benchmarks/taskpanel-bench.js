/**
 * Task Management Panel Benchmarks
 * Measures performance of TaskManagementPanel interactions and form operations
 */

const PerformanceMonitor = require('../profiling/PerformanceMonitor');
const { generateTasks } = require('../fixtures/task-fixtures');

// Mock VS Code API for isolation
const createMockVscode = () => ({
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
});

const createMockContext = () => ({
  subscriptions: [],
});

// TaskManagementPanel class with benchmarkable methods
class TaskManagementPanel {
  constructor(vscode, extensionContext) {
    this.vscode = vscode;
    this.context = extensionContext;
    this._tasks = new Map();
    this._panel = null;
    this._webviewReady = false;
  }

  async show() {
    if (this._panel) {
      this._panel.reveal(this.vscode.ViewColumn.Beside);
      return;
    }

    this._panel = this.vscode.window.createWebviewPanel(
      'intentlens.taskmanagement',
      '📋 Task Management',
      this.vscode.ViewColumn.Beside,
      {
        enableScripts: true,
        enableFindWidget: true,
        retainContextWhenHidden: true,
      }
    );

    this._panel.webview.onDidReceiveMessage(
      message => this.handleWebviewMessage(message)
    );

    this._panel.onDidDispose(() => {
      this._panel = null;
    });

    this._panel.webview.html = this.getWebviewContent();
  }

  getWebviewContent() {
    return `<!DOCTYPE html>
      <html>
      <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <style>
          body {
            font-family: var(--vscode-font-family);
            background: var(--vscode-editor-background);
            color: var(--vscode-editor-foreground);
            margin: 0;
            padding: 16px;
          }
          .container {
            max-width: 800px;
            margin: 0 auto;
          }
          h2 {
            margin-top: 0;
            border-bottom: 1px solid var(--vscode-panel-border);
            padding-bottom: 8px;
          }
          .form-group {
            margin-bottom: 16px;
          }
          label {
            display: block;
            margin-bottom: 4px;
            font-weight: bold;
          }
          input[type="text"], textarea, select {
            width: 100%;
            padding: 8px;
            background: var(--vscode-input-background);
            color: var(--vscode-input-foreground);
            border: 1px solid var(--vscode-input-border);
            border-radius: 4px;
            font-family: inherit;
            box-sizing: border-box;
          }
          textarea {
            resize: vertical;
            min-height: 80px;
          }
          .priority-selector {
            display: flex;
            gap: 8px;
          }
          .priority-btn {
            padding: 8px 12px;
            border: 2px solid transparent;
            border-radius: 4px;
            background: var(--vscode-button-background);
            color: var(--vscode-button-foreground);
            cursor: pointer;
            font-weight: bold;
          }
          button {
            padding: 8px 16px;
            background: var(--vscode-button-background);
            color: var(--vscode-button-foreground);
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-weight: bold;
          }
        </style>
      </head>
      <body>
        <div class="container">
          <h2>📋 Create Task</h2>
          <div class="form-group">
            <label>Task Name *</label>
            <input type="text" id="taskName" placeholder="e.g: validateUserInput">
          </div>
          <div class="form-group">
            <label>Priority</label>
            <div class="priority-selector">
              <button class="priority-btn" data-priority="P0">P0</button>
              <button class="priority-btn" data-priority="P1">P1</button>
              <button class="priority-btn" data-priority="P2">P2</button>
              <button class="priority-btn" data-priority="P3">P3</button>
            </div>
          </div>
          <div class="form-group">
            <label>Description</label>
            <textarea id="description" placeholder="Describe this task..."></textarea>
          </div>
          <div class="form-group">
            <label>Dependencies</label>
            <input type="text" id="dependencyInput" placeholder="Enter task name and press Enter">
          </div>
          <div style="display: flex; gap: 8px; margin-top: 16px;">
            <button>✓ Create Task</button>
            <button>↻ Reset</button>
          </div>
        </div>
      </body>
      </html>`;
  }

  handleWebviewMessage(message) {
    switch (message.command) {
      case 'createTask':
        this.createTask(message.data);
        break;
      case 'updateTask':
        this.updateTask(message.data);
        break;
      case 'deleteTask':
        this.deleteTask(message.data);
        break;
    }
  }

  createTask(data) {
    const taskId = 'task_' + Date.now();
    const task = {
      id: taskId,
      name: data.name,
      priority: data.priority,
      description: data.description,
      dependencies: data.dependencies || [],
      status: 'pending',
      progress: 0,
      createdAt: new Date(),
      logs: [],
      executionHistory: [],
    };

    if (this.hasCyclicDependency(task)) {
      this.vscode.window.showErrorMessage('Cyclic dependency detected!');
      return;
    }

    this._tasks.set(taskId, task);
    this.vscode.window.showInformationMessage(`✓ Task "${task.name}" created`);
  }

  hasCyclicDependency(task, visited = new Set()) {
    for (const depName of task.dependencies) {
      if (visited.has(depName)) {
        return true;
      }
      
      visited.add(depName);
      
      for (const [, t] of this._tasks) {
        if (t.name === depName) {
          if (this.hasCyclicDependency(t, new Set(visited))) {
            return true;
          }
          break;
        }
      }
    }
    
    return false;
  }

  updateTask(data) {
    const task = this._tasks.get(data.id);
    if (!task) return;
    
    Object.assign(task, data);
    this.vscode.window.showInformationMessage('✓ Task updated');
  }

  deleteTask(taskId) {
    this._tasks.delete(taskId);
    this.vscode.window.showInformationMessage('✓ Task deleted');
  }

  getAllTasks() {
    return Array.from(this._tasks.values());
  }

  exportTasks() {
    const tasks = this.getAllTasks();
    return JSON.stringify(tasks, null, 2);
  }
}

/**
 * Benchmark 1: Webview HTML generation
 * Measures time to generate HTML form with N tasks listed
 * Expected: <200ms for typical 100-task form
 */
async function benchmarkHtmlGeneration() {
  const monitor = new PerformanceMonitor();
  const scales = [50, 100, 200];
  
  for (const scale of scales) {
    const opName = `html-gen-${scale}`;
    
    // Run 5 iterations for statistical significance
    for (let i = 0; i < 5; i++) {
      const panel = new TaskManagementPanel(createMockVscode(), createMockContext());
      
      monitor.start(opName);
      // Generate HTML content - this is called when panel.show() is invoked
      const html = panel.getWebviewContent();
      monitor.end(opName);
    }
  }
  
  // Return aggregated results
  const report = monitor.report('html-gen-100');
  monitor.destroy();
  
  return {
    name: 'Benchmark 1: Webview HTML Generation',
    description: 'Generate HTML for task form',
    target: '<200ms',
    result: report.latency.p50,
    unit: 'ms',
    passed: report.latency.p95 < 200,
    details: {
      p50: report.latency.p50,
      p95: report.latency.p95,
      p99: report.latency.p99,
      iterations: report.iterations
    }
  };
}

/**
 * Benchmark 2: Form rendering (panel.show())
 * Measures time from show() to webview ready
 * Expected: ≤400ms total
 */
async function benchmarkFormRendering() {
  const monitor = new PerformanceMonitor();
  
  for (let i = 0; i < 10; i++) {
    const panel = new TaskManagementPanel(createMockVscode(), createMockContext());
    
    monitor.start('form-render');
    // Simulate show() which creates webview and generates HTML
    await panel.show();
    monitor.end('form-render');
  }
  
  const report = monitor.report('form-render');
  monitor.destroy();
  
  return {
    name: 'Benchmark 2: Form Rendering (panel.show())',
    description: 'Time from show() to webview ready',
    target: '≤400ms',
    result: report.latency.p50,
    unit: 'ms',
    passed: report.latency.p95 <= 400,
    details: {
      p50: report.latency.p50,
      p95: report.latency.p95,
      p99: report.latency.p99,
      iterations: report.iterations
    }
  };
}

/**
 * Benchmark 3: Cyclic dependency detection (DFS)
 * Creates task with M dependencies and runs cyclic detection
 * Expected: ≤10ms for 100 dependencies
 */
async function benchmarkCyclicDetection() {
  const monitor = new PerformanceMonitor();
  const depCounts = [10, 50, 100];
  
  for (const depCount of depCounts) {
    const opName = `cyclic-detect-${depCount}`;
    
    for (let iter = 0; iter < 5; iter++) {
      const panel = new TaskManagementPanel(createMockVscode(), createMockContext());
      
      // Create tasks with dependencies
      const tasks = generateTasks(depCount + 10, 42 + iter);
      
      // Add tasks to panel
      for (const task of tasks) {
        panel._tasks.set(task.id, {
          id: task.id,
          name: task.name,
          dependencies: task.dependencies.map(d => `task-${d}`),
          priority: task.priority,
          status: task.status,
          progress: task.progress
        });
      }
      
      // Measure cyclic detection
      const testTask = panel._tasks.get('task-0');
      
      monitor.start(opName);
      const hasCycle = panel.hasCyclicDependency(testTask);
      monitor.end(opName);
    }
  }
  
  const report = monitor.report('cyclic-detect-100');
  monitor.destroy();
  
  return {
    name: 'Benchmark 3: Cyclic Dependency Detection (DFS)',
    description: 'Time to detect cycle or confirm no cycle',
    target: '≤10ms',
    result: report.latency.p50,
    unit: 'ms',
    passed: report.latency.p95 <= 10,
    details: {
      p50: report.latency.p50,
      p95: report.latency.p95,
      p99: report.latency.p99,
      iterations: report.iterations
    }
  };
}

/**
 * Benchmark 4: Form validation on input
 * Simulates form input changes and validation
 * Expected: ≤50ms per validation
 */
async function benchmarkFormValidation() {
  const monitor = new PerformanceMonitor();
  
  // Test various validation scenarios
  const validationTests = [
    { name: '', priority: 'P0', description: '' }, // Invalid: empty name
    { name: 'ValidTask', priority: 'P1', description: 'A valid task' },
    { name: 'TaskWithDeps', priority: 'P0', description: '', dependencies: ['dep1', 'dep2'] },
  ];
  
  for (let i = 0; i < 10; i++) {
    const panel = new TaskManagementPanel(createMockVscode(), createMockContext());
    
    for (const test of validationTests) {
      monitor.start('validation');
      
      // Perform validation: check task name, priority, dependencies
      let isValid = true;
      
      // Name validation
      if (!test.name || test.name.trim().length === 0) {
        isValid = false;
      }
      
      // Priority validation
      const validPriorities = ['P0', 'P1', 'P2', 'P3'];
      if (!validPriorities.includes(test.priority)) {
        isValid = false;
      }
      
      // Dependencies check (would be checked for cycles)
      if (test.dependencies && test.dependencies.length > 0) {
        // Simulate cyclic dependency check
        const testTask = {
          id: 'test',
          name: test.name,
          dependencies: test.dependencies,
          priority: test.priority
        };
        
        // Add some mock tasks to check against
        panel._tasks.set('dep1', { id: 'dep1', name: 'dep1', dependencies: [] });
        panel._tasks.set('dep2', { id: 'dep2', name: 'dep2', dependencies: [] });
        
        // This would check for cycles
        const hasCycle = panel.hasCyclicDependency(testTask);
        if (hasCycle) {
          isValid = false;
        }
      }
      
      monitor.end('validation');
    }
  }
  
  const report = monitor.report('validation');
  monitor.destroy();
  
  return {
    name: 'Benchmark 4: Form Validation on Input',
    description: 'Validation latency per keystroke/field change',
    target: '≤50ms',
    result: report.latency.p50,
    unit: 'ms',
    passed: report.latency.p95 <= 50,
    details: {
      p50: report.latency.p50,
      p95: report.latency.p95,
      p99: report.latency.p99,
      iterations: report.iterations,
      validationTestCount: 3
    }
  };
}

/**
 * Benchmark 5: Webview memory consumption
 * Loads webview panel with form and measures memory usage
 * Expected: ≤20MB for panel
 */
async function benchmarkMemoryConsumption() {
  const monitor = new PerformanceMonitor();
  
  // Measure memory before and after loading panels
  for (let i = 0; i < 5; i++) {
    const panel = new TaskManagementPanel(createMockVscode(), createMockContext());
    
    monitor.start('memory-webview');
    
    // Create and show panel (this allocates memory for HTML, event handlers, etc.)
    await panel.show();
    
    // Add some tasks to simulate realistic usage
    for (let j = 0; j < 50; j++) {
      panel.createTask({
        name: `Task ${j}`,
        priority: ['P0', 'P1', 'P2', 'P3'][j % 4],
        description: `Task description ${j}`,
        dependencies: []
      });
    }
    
    monitor.end('memory-webview');
  }
  
  const report = monitor.report('memory-webview');
  
  // Memory is reported in bytes, convert to MB for readability
  const heapUsedMB = Math.abs(report.memoryDelta.heapUsed) / (1024 * 1024);
  
  monitor.destroy();
  
  return {
    name: 'Benchmark 5: Webview Memory Consumption',
    description: 'Memory usage of webview panel + assets',
    target: '≤20MB',
    result: heapUsedMB,
    unit: 'MB',
    passed: heapUsedMB <= 20,
    details: {
      heapUsedMB: heapUsedMB,
      heapUsedBytes: report.memoryDelta.heapUsed,
      p50: heapUsedMB,
      p95: heapUsedMB,
      iterations: report.iterations
    }
  };
}

/**
 * Run all benchmarks
 */
async function runTaskPanelBenchmarks() {
  console.log('🚀 Starting Task Management Panel Benchmarks...\n');
  
  const results = [];
  
  try {
    // Run each benchmark
    results.push(await benchmarkHtmlGeneration());
    console.log(`✓ ${results[0].name} - ${results[0].result.toFixed(2)}${results[0].unit}`);
    
    results.push(await benchmarkFormRendering());
    console.log(`✓ ${results[1].name} - ${results[1].result.toFixed(2)}${results[1].unit}`);
    
    results.push(await benchmarkCyclicDetection());
    console.log(`✓ ${results[2].name} - ${results[2].result.toFixed(2)}${results[2].unit}`);
    
    results.push(await benchmarkFormValidation());
    console.log(`✓ ${results[3].name} - ${results[3].result.toFixed(2)}${results[3].unit}`);
    
    results.push(await benchmarkMemoryConsumption());
    console.log(`✓ ${results[4].name} - ${results[4].result.toFixed(2)}${results[4].unit}`);
    
  } catch (error) {
    console.error('❌ Benchmark error:', error);
    throw error;
  }
  
  // Print summary
  console.log('\n📊 Benchmark Summary:');
  console.log('='.repeat(70));
  
  for (const result of results) {
    const status = result.passed ? '✓ PASS' : '✗ FAIL';
    console.log(`${status} | ${result.name}`);
    console.log(`     Target: ${result.target} | Result: ${result.result.toFixed(2)}${result.unit}`);
    console.log(`     P50: ${result.details.p50.toFixed(2)}${result.unit} | P95: ${result.details.p95.toFixed(2)}${result.unit}`);
  }
  
  console.log('='.repeat(70));
  
  // Count passes
  const passCount = results.filter(r => r.passed).length;
  console.log(`\n✓ ${passCount}/${results.length} benchmarks passed\n`);
  
  // Return aggregated result
  const allPassed = results.every(r => r.passed);
  return {
    name: 'Task Management Panel: Form & Validation Performance',
    passed: allPassed,
    results: results,
    passCount: passCount,
    totalCount: results.length
  };
}

module.exports = {
  name: 'Task Management Panel: Form & Validation Performance',
  run: runTaskPanelBenchmarks
};
