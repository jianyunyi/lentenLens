/**
 * Lazy Decoration Performance Comparison
 * Compares before/after optimization performance
 */

const PerformanceMonitor = require('../profiling/PerformanceMonitor');
const { generateTasks } = require('../fixtures/task-fixtures');
const { createMockTextEditor } = require('../fixtures/api-mocks');

// Load both versions for comparison
const TaskBoundaryDecorator = require('../../src/boundaries');

// Mock VS Code API
function createMockVscode() {
  return {
    window: {
      createTextEditorDecorationType: function(options) {
        return { dispose: () => {} };
      }
    },
    Range: function(startLine, startCol, endLine, endCol) {
      this.start = { line: startLine, character: startCol };
      this.end = { line: endLine, character: endCol };
    },
    ThemeColor: function(name) {
      this.name = name;
    },
    MarkdownString: function(str) {
      this.value = str;
    },
    OverviewRulerLane: { Right: 2 }
  };
}

/**
 * Create mock editor with visible range control
 */
function createMockEditorWithViewport(visibleStart, visibleEnd) {
  const editor = createMockTextEditor();
  
  // Override visible ranges to simulate viewport
  editor.visibleRanges = [
    {
      start: { line: visibleStart, character: 0 },
      end: { line: visibleEnd, character: 0 }
    }
  ];
  
  // Track decoration calls
  editor._decorationCalls = 0;
  const originalSetDecorations = editor.setDecorations || function() {};
  editor.setDecorations = function(type, decorations) {
    editor._decorationCalls++;
    return originalSetDecorations.call(this, type, decorations);
  };
  
  return editor;
}

/**
 * Benchmark: Lazy decoration vs full decoration
 */
async function benchmarkLazyVsFull() {
  const monitor = new PerformanceMonitor();
  const vscode = createMockVscode();
  
  const taskCounts = [100, 500, 1000];
  const results = [];
  
  for (const count of taskCounts) {
    console.log(`\nTesting with ${count} tasks...`);
    
    // Generate tasks with code ranges
    const tasks = generateTasks(count, 42).map((task, idx) => ({
      ...task,
      codeRange: {
        startLine: idx * 5,
        endLine: idx * 5 + 4,
        endColumn: 80
      }
    }));
    
    // Simulate viewport showing only 20% of file (e.g., lines 0-100 visible out of 500 lines)
    const visibleLines = Math.floor(count * 5 * 0.2);
    const editor = createMockEditorWithViewport(0, visibleLines);
    
    // Test 1: Full decoration (lazy mode OFF)
    const decoratorFull = new TaskBoundaryDecorator(vscode);
    decoratorFull._lazyMode = false;
    
    for (let i = 0; i < 10; i++) {
      monitor.start(`full-${count}`);
      decoratorFull.setTaskBoundaries(editor, tasks);
      // Wait for debounce
      await new Promise(resolve => setTimeout(resolve, 150));
      monitor.end(`full-${count}`);
    }
    
    const reportFull = monitor.report(`full-${count}`);
    
    // Test 2: Lazy decoration (lazy mode ON)
    const decoratorLazy = new TaskBoundaryDecorator(vscode);
    decoratorLazy._lazyMode = true;
    
    for (let i = 0; i < 10; i++) {
      monitor.start(`lazy-${count}`);
      decoratorLazy.setTaskBoundaries(editor, tasks);
      // Wait for debounce
      await new Promise(resolve => setTimeout(resolve, 150));
      monitor.end(`lazy-${count}`);
    }
    
    const reportLazy = monitor.report(`lazy-${count}`);
    
    // Calculate improvement
    const improvement = ((reportFull.latency.p50 - reportLazy.latency.p50) / reportFull.latency.p50 * 100).toFixed(1);
    
    results.push({
      taskCount: count,
      full: {
        p50: reportFull.latency.p50,
        p95: reportFull.latency.p95,
        memoryDelta: reportFull.memoryDelta.heapUsed
      },
      lazy: {
        p50: reportLazy.latency.p50,
        p95: reportLazy.latency.p95,
        memoryDelta: reportLazy.memoryDelta.heapUsed
      },
      improvement: parseFloat(improvement),
      improvementStatus: improvement >= 40 ? 'PASS' : 'REVIEW'
    });
    
    console.log(`  Full decoration: ${reportFull.latency.p50.toFixed(2)}ms`);
    console.log(`  Lazy decoration: ${reportLazy.latency.p50.toFixed(2)}ms`);
    console.log(`  Improvement: ${improvement}%`);
  }
  
  monitor.destroy();
  return results;
}

/**
 * Test scroll performance
 */
async function benchmarkScrollPerformance() {
  const monitor = new PerformanceMonitor();
  const vscode = createMockVscode();
  
  console.log('\nTesting scroll performance...');
  
  const tasks = generateTasks(1000, 42).map((task, idx) => ({
    ...task,
    codeRange: {
      startLine: idx * 5,
      endLine: idx * 5 + 4,
      endColumn: 80
    }
  }));
  
  const decorator = new TaskBoundaryDecorator(vscode);
  decorator._lazyMode = true;
  
  // Simulate scrolling through file
  const scrollPositions = [
    { start: 0, end: 100 },
    { start: 500, end: 600 },
    { start: 1000, end: 1100 },
    { start: 2000, end: 2100 },
    { start: 4000, end: 4100 }
  ];
  
  for (let i = 0; i < scrollPositions.length; i++) {
    const pos = scrollPositions[i];
    const editor = createMockEditorWithViewport(pos.start, pos.end);
    
    monitor.start('scroll-update');
    decorator.onEditorScroll(editor);
    await new Promise(resolve => setTimeout(resolve, 150)); // Wait for debounce
    monitor.end('scroll-update');
  }
  
  const report = monitor.report('scroll-update');
  
  console.log(`  Scroll update latency: ${report.latency.p50.toFixed(2)}ms (p50)`);
  console.log(`  Target: <50ms for 60 FPS`);
  console.log(`  Status: ${report.latency.p95 < 50 ? 'PASS' : 'REVIEW'}`);
  
  monitor.destroy();
  
  return {
    p50: report.latency.p50,
    p95: report.latency.p95,
    target: 50,
    status: report.latency.p95 < 50 ? 'PASS' : 'REVIEW'
  };
}

/**
 * Main execution
 */
async function main() {
  console.log('🚀 Lazy Decoration Performance Comparison\n');
  console.log('Testing Phase 4 Task 8 optimization...\n');
  
  try {
    // Run lazy vs full comparison
    const comparisonResults = await benchmarkLazyVsFull();
    
    // Run scroll performance test
    const scrollResults = await benchmarkScrollPerformance();
    
    // Generate summary
    console.log('\n' + '═'.repeat(70));
    console.log('LAZY DECORATION OPTIMIZATION RESULTS');
    console.log('═'.repeat(70));
    
    console.log('\nComparison Results:');
    for (const result of comparisonResults) {
      console.log(`\n${result.taskCount} tasks:`);
      console.log(`  Full: ${result.full.p50.toFixed(2)}ms | Lazy: ${result.lazy.p50.toFixed(2)}ms`);
      console.log(`  Improvement: ${result.improvement}% (${result.improvementStatus})`);
    }
    
    console.log('\nScroll Performance:');
    console.log(`  Latency: ${scrollResults.p50.toFixed(2)}ms (p50), ${scrollResults.p95.toFixed(2)}ms (p95)`);
    console.log(`  Status: ${scrollResults.status}`);
    
    // Overall status
    const allPass = comparisonResults.every(r => r.improvementStatus === 'PASS') &&
                    scrollResults.status === 'PASS';
    
    console.log('\n' + '═'.repeat(70));
    console.log(`Overall: ${allPass ? '✅ PASS' : '⚠ REVIEW REQUIRED'}`);
    console.log('═'.repeat(70));
    
    // Save results
    const fs = require('fs');
    const path = require('path');
    const reportDir = path.join(__dirname, '..', '..', '..', 'docs', 'performance-reports');
    
    if (!fs.existsSync(reportDir)) {
      fs.mkdirSync(reportDir, { recursive: true });
    }
    
    const report = {
      timestamp: new Date().toISOString(),
      optimization: 'Lazy Decoration (Task 8)',
      comparisonResults,
      scrollResults,
      summary: {
        avgImprovement: (comparisonResults.reduce((sum, r) => sum + r.improvement, 0) / comparisonResults.length).toFixed(1) + '%',
        scrollLatency: scrollResults.p50.toFixed(2) + 'ms',
        allPass
      }
    };
    
    fs.writeFileSync(
      path.join(reportDir, 'lazy-decoration-optimization.json'),
      JSON.stringify(report, null, 2)
    );
    
    console.log('\n✓ Results saved to: docs/performance-reports/lazy-decoration-optimization.json\n');
    
    process.exit(allPass ? 0 : 1);
  } catch (error) {
    console.error('\n❌ Benchmark failed:', error);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = { benchmarkLazyVsFull, benchmarkScrollPerformance };