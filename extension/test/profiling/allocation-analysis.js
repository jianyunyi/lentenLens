/**
 * Allocation Timeline Analysis
 * Identifies memory allocation hotspots during benchmark execution
 */

const fs = require('fs');
const path = require('path');
const { performance, PerformanceObserver } = require('perf_hooks');

const REPORTS_DIR = path.join(__dirname, '..', '..', '..', 'docs', 'performance-reports');

/**
 * Track allocations during operation
 */
class AllocationTracker {
  constructor() {
    this.allocations = [];
    this.startMem = null;
    this.endMem = null;
  }

  start() {
    if (global.gc) {
      global.gc();
    }
    this.startMem = process.memoryUsage();
    this.allocations = [];
  }

  recordAllocation(operation, sizeMB) {
    this.allocations.push({
      operation,
      sizeMB: sizeMB.toFixed(2),
      timestamp: Date.now()
    });
  }

  end() {
    if (global.gc) {
      global.gc();
    }
    this.endMem = process.memoryUsage();
  }

  getReport() {
    const heapDelta = this.endMem.heapUsed - this.startMem.heapUsed;
    const heapDeltaMB = heapDelta / (1024 * 1024);

    return {
      totalAllocationMB: heapDeltaMB.toFixed(2),
      allocations: this.allocations,
      topAllocations: this.allocations
        .sort((a, b) => parseFloat(b.sizeMB) - parseFloat(a.sizeMB))
        .slice(0, 10)
    };
  }
}

/**
 * Run allocation analysis on benchmarks
 */
async function analyzeAllocations() {
  console.log('🔍 Analyzing allocation hotspots...\n');

  const tracker = new AllocationTracker();
  const { generateTasks } = require('../fixtures/task-fixtures');
  const { generateCallTree } = require('../fixtures/tree-fixtures');

  const hotspots = [];

  // Test 1: Task generation allocation
  console.log('Testing task generation allocation...');
  tracker.start();
  const tasks1k = generateTasks(1000, 42);
  tracker.recordAllocation('generateTasks(1000)', process.memoryUsage().heapUsed / (1024 * 1024));
  
  const tasks10k = generateTasks(10000, 42);
  tracker.recordAllocation('generateTasks(10000)', process.memoryUsage().heapUsed / (1024 * 1024));
  tracker.end();
  
  hotspots.push({
    operation: 'Task Generation',
    ...tracker.getReport()
  });

  // Test 2: Tree generation allocation
  console.log('Testing tree generation allocation...');
  tracker.start();
  const tree10 = generateCallTree(10, 3, 42);
  tracker.recordAllocation('generateCallTree(depth=10)', process.memoryUsage().heapUsed / (1024 * 1024));
  
  const tree20 = generateCallTree(20, 3, 42);
  tracker.recordAllocation('generateCallTree(depth=20)', process.memoryUsage().heapUsed / (1024 * 1024));
  tracker.end();
  
  hotspots.push({
    operation: 'Tree Generation',
    ...tracker.getReport()
  });

  // Test 3: Filtering allocation
  console.log('Testing filtering allocation...');
  tracker.start();
  const filtered = tasks10k.filter(t => t.priority === 'P0');
  tracker.recordAllocation('filter(10k tasks)', process.memoryUsage().heapUsed / (1024 * 1024));
  tracker.end();
  
  hotspots.push({
    operation: 'Task Filtering',
    ...tracker.getReport()
  });

  // Test 4: Sorting allocation
  console.log('Testing sorting allocation...');
  tracker.start();
  const sorted = [...tasks10k].sort((a, b) => a.name.localeCompare(b.name));
  tracker.recordAllocation('sort(10k tasks)', process.memoryUsage().heapUsed / (1024 * 1024));
  tracker.end();
  
  hotspots.push({
    operation: 'Task Sorting',
    ...tracker.getReport()
  });

  return hotspots;
}

/**
 * Generate allocation analysis report
 */
function generateAllocationReport(hotspots) {
  console.log('\n📄 Generating allocation analysis report...\n');

  const report = {
    timestamp: new Date().toISOString(),
    hotspots: hotspots,
    topAllocations: hotspots
      .flatMap(h => h.topAllocations || [])
      .sort((a, b) => parseFloat(b.sizeMB) - parseFloat(a.sizeMB))
      .slice(0, 10)
  };

  // Ensure reports directory exists
  if (!fs.existsSync(REPORTS_DIR)) {
    fs.mkdirSync(REPORTS_DIR, { recursive: true });
  }

  const reportPath = path.join(REPORTS_DIR, 'allocation-analysis.json');
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
  
  console.log(`✓ Allocation analysis saved to: ${reportPath}\n`);

  return report;
}

/**
 * Print allocation summary
 */
function printSummary(report) {
  console.log('═'.repeat(70));
  console.log('ALLOCATION HOTSPOT ANALYSIS');
  console.log('═'.repeat(70));
  
  for (const hotspot of report.hotspots) {
    console.log(`\n${hotspot.operation}:`);
    console.log(`  Total Allocation: ${hotspot.totalAllocationMB} MB`);
    
    if (hotspot.topAllocations && hotspot.topAllocations.length > 0) {
      console.log('  Top Allocations:');
      hotspot.topAllocations.slice(0, 3).forEach((alloc, i) => {
        console.log(`    ${i + 1}. ${alloc.operation}: ${alloc.sizeMB} MB`);
      });
    }
  }
  
  console.log('\n' + '═'.repeat(70));
  console.log('Top 5 Overall Allocation Hotspots:');
  console.log('═'.repeat(70));
  
  report.topAllocations.slice(0, 5).forEach((alloc, i) => {
    console.log(`${i + 1}. ${alloc.operation}: ${alloc.sizeMB} MB`);
  });
  
  console.log('═'.repeat(70));
}

/**
 * Main execution
 */
async function main() {
  try {
    console.log('🚀 Allocation Timeline Analysis\n');
    
    if (!global.gc) {
      console.warn('⚠ GC not exposed. Run with: node --expose-gc\n');
    }

    // Analyze allocations
    const hotspots = await analyzeAllocations();

    // Generate report
    const report = generateAllocationReport(hotspots);

    // Print summary
    printSummary(report);

    console.log('\n✅ Allocation analysis complete!');
    process.exit(0);
  } catch (error) {
    console.error('\n❌ Allocation analysis failed:', error);
    process.exit(1);
  }
}

// Run if executed directly
if (require.main === module) {
  main();
}

module.exports = { AllocationTracker, analyzeAllocations, generateAllocationReport };