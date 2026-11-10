/**
 * Baseline Heap Snapshot Profiling
 * Captures heap snapshots at different scales
 * Analyzes memory growth and object retention
 */

const fs = require('fs');
const path = require('path');
const { generateTasks } = require('../fixtures/task-fixtures');

const REPORTS_DIR = path.join(__dirname, '..', '..', '..', 'docs', 'performance-reports');

/**
 * Capture memory snapshot at current state
 */
function captureMemorySnapshot(label) {
  // Force GC if available
  if (global.gc) {
    global.gc();
  }

  const mem = process.memoryUsage();
  return {
    label,
    timestamp: new Date().toISOString(),
    heapUsed: mem.heapUsed,
    heapTotal: mem.heapTotal,
    external: mem.external,
    rss: mem.rss,
    heapUsedMB: (mem.heapUsed / (1024 * 1024)).toFixed(2),
    heapTotalMB: (mem.heapTotal / (1024 * 1024)).toFixed(2)
  };
}

/**
 * Load tasks at different scales and capture snapshots
 */
async function captureHeapSnapshots() {
  console.log('📸 Capturing heap snapshots at different scales...\n');

  const snapshots = [];
  const scales = [0, 1000, 5000, 10000, 50000];

  // Baseline (no tasks)
  console.log('Capturing baseline (0 tasks)...');
  snapshots.push(captureMemorySnapshot('baseline-0'));

  // Load tasks at each scale
  for (const scale of scales.slice(1)) {
    console.log(`Capturing snapshot at ${scale} tasks...`);
    
    // Force GC before loading
    if (global.gc) {
      global.gc();
    }

    // Load tasks
    const tasks = generateTasks(scale, 42);
    
    // Wait a bit for allocations to settle
    await new Promise(resolve => setTimeout(resolve, 100));
    
    // Capture snapshot
    const snapshot = captureMemorySnapshot(`scale-${scale}`);
    snapshot.taskCount = scale;
    snapshot.topObjects = analyzeTopObjects(tasks);
    snapshots.push(snapshot);
  }

  return snapshots;
}

/**
 * Analyze top object types in memory
 */
function analyzeTopObjects(tasks) {
  // Simple analysis based on task structure
  const objectTypes = {
    'Task Objects': tasks.length,
    'String Properties': tasks.length * 3, // id, name, priority
    'Array Dependencies': tasks.reduce((sum, t) => sum + t.dependencies.length, 0),
    'Number Properties': tasks.length * 1 // progress
  };

  return Object.entries(objectTypes)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([type, count]) => ({ type, count }));
}

/**
 * Calculate memory growth rate
 */
function calculateGrowthRate(snapshots) {
  const analysis = [];
  
  for (let i = 1; i < snapshots.length; i++) {
    const prev = snapshots[i - 1];
    const curr = snapshots[i];
    
    const prevHeap = prev.heapUsed;
    const currHeap = curr.heapUsed;
    const prevScale = prev.taskCount || 0;
    const currScale = curr.taskCount || 0;
    
    const heapGrowth = currHeap - prevHeap;
    const scaleGrowth = currScale - prevScale;
    const growthRatio = scaleGrowth > 0 ? (currHeap / prevHeap) : 1;
    const scaleRatio = scaleGrowth > 0 ? (currScale / prevScale) : 1;
    
    analysis.push({
      from: prev.label,
      to: curr.label,
      heapGrowthMB: (heapGrowth / (1024 * 1024)).toFixed(2),
      growthRatio: growthRatio.toFixed(2),
      scaleRatio: scaleRatio.toFixed(2),
      linearityCheck: growthRatio < (scaleRatio * 2) ? 'PASS' : 'FAIL'
    });
  }
  
  return analysis;
}

/**
 * Generate heap analysis report
 */
function generateHeapReport(snapshots, growthAnalysis) {
  console.log('\n📄 Generating heap snapshot report...\n');

  // Ensure reports directory exists
  if (!fs.existsSync(REPORTS_DIR)) {
    fs.mkdirSync(REPORTS_DIR, { recursive: true });
  }

  const report = {
    timestamp: new Date().toISOString(),
    snapshots: snapshots.map(s => ({
      label: s.label,
      taskCount: s.taskCount || 0,
      heapUsedMB: parseFloat(s.heapUsedMB),
      heapTotalMB: parseFloat(s.heapTotalMB),
      topObjects: s.topObjects || []
    })),
    growthAnalysis: growthAnalysis,
    summary: {
      baselineHeapMB: parseFloat(snapshots[0].heapUsedMB),
      scale10kHeapMB: parseFloat(snapshots.find(s => s.taskCount === 10000)?.heapUsedMB || 0),
      linearityCheck: growthAnalysis.every(a => a.linearityCheck === 'PASS') ? 'PASS' : 'FAIL',
      avgGrowthRatio: (growthAnalysis.reduce((sum, a) => sum + parseFloat(a.growthRatio), 0) / growthAnalysis.length).toFixed(2)
    }
  };

  const reportPath = path.join(REPORTS_DIR, 'heap-snapshots-analysis.json');
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
  
  console.log(`✓ Heap analysis saved to: ${reportPath}\n`);

  // Print summary
  console.log('Heap Snapshot Summary:');
  console.log('='.repeat(70));
  console.log(`Baseline (0 tasks): ${snapshots[0].heapUsedMB} MB`);
  for (let i = 1; i < snapshots.length; i++) {
    const s = snapshots[i];
    console.log(`Scale ${s.taskCount}: ${s.heapUsedMB} MB`);
  }
  console.log('='.repeat(70));
  console.log(`Linearity Check: ${report.summary.linearityCheck}`);
  console.log(`Average Growth Ratio: ${report.summary.avgGrowthRatio}x`);
  console.log('='.repeat(70));

  return report;
}

/**
 * Main execution
 */
async function main() {
  try {
    console.log('🚀 Baseline Heap Snapshot Profiling\n');
    
    if (!global.gc) {
      console.warn('⚠ GC not exposed. Run with: node --expose-gc\n');
    }

    // Step 1: Capture heap snapshots
    const snapshots = await captureHeapSnapshots();

    // Step 2: Analyze growth rate
    const growthAnalysis = calculateGrowthRate(snapshots);

    // Step 3: Generate report
    const report = generateHeapReport(snapshots, growthAnalysis);

    console.log('\n✅ Heap snapshot profiling complete!');
    
    // Exit with appropriate code
    const passed = report.summary.linearityCheck === 'PASS';
    process.exit(passed ? 0 : 1);
  } catch (error) {
    console.error('\n❌ Heap snapshot profiling failed:', error);
    process.exit(1);
  }
}

// Run if executed directly
if (require.main === module) {
  main();
}

module.exports = { captureHeapSnapshots, calculateGrowthRate, generateHeapReport };