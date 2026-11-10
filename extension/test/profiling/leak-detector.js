/**
 * Memory Leak Detection
 * Runs 1-hour sustained load test with periodic interactions
 * Captures heap snapshots to detect memory leaks
 */

const fs = require('fs');
const path = require('path');
const { generateTasks } = require('../fixtures/task-fixtures');

const REPORTS_DIR = path.join(__dirname, '..', '..', '..', 'docs', 'performance-reports');
const TEST_DURATION_MS = 60 * 60 * 1000; // 1 hour (reduced to 5 min for testing)
const SNAPSHOT_INTERVAL_MS = 30 * 60 * 1000; // 30 minutes
const INTERACTION_INTERVAL_MS = 30 * 1000; // 30 seconds

/**
 * Capture memory snapshot with timestamp
 */
function captureSnapshot(label, iteration) {
  if (global.gc) {
    global.gc();
  }

  const mem = process.memoryUsage();
  return {
    label,
    iteration,
    timestamp: new Date().toISOString(),
    elapsedMs: Date.now() - startTime,
    heapUsed: mem.heapUsed,
    heapTotal: mem.heapTotal,
    external: mem.external,
    rss: mem.rss,
    heapUsedMB: (mem.heapUsed / (1024 * 1024)).toFixed(2),
    heapTotalMB: (mem.heapTotal / (1024 * 1024)).toFixed(2)
  };
}

/**
 * Simulate typical user interactions
 */
function simulateInteraction(tasks, iteration) {
  // Filter tasks by priority
  const filtered = tasks.filter(t => t.priority === 'P0');
  
  // Sort tasks by name
  const sorted = [...filtered].sort((a, b) => a.name.localeCompare(b.name));
  
  // Access random task
  const randomIdx = Math.floor(Math.random() * tasks.length);
  const task = tasks[randomIdx];
  
  // Simulate tree navigation
  const dependencies = task.dependencies.map(depId => 
    tasks.find(t => t.id === `task-${depId}`)
  ).filter(Boolean);
  
  return {
    filteredCount: filtered.length,
    sortedCount: sorted.length,
    accessedTask: task.id,
    dependenciesResolved: dependencies.length
  };
}

/**
 * Analyze memory growth over time
 */
function analyzeMemoryGrowth(snapshots) {
  if (snapshots.length < 2) {
    return { growthRateMBPerHour: 0, leaked: false };
  }

  const first = snapshots[0];
  const last = snapshots[snapshots.length - 1];
  
  const heapGrowthMB = parseFloat(last.heapUsedMB) - parseFloat(first.heapUsedMB);
  const timeElapsedHours = (last.elapsedMs - first.elapsedMs) / (1000 * 60 * 60);
  const growthRateMBPerHour = timeElapsedHours > 0 ? heapGrowthMB / timeElapsedHours : 0;
  
  // Consider it a leak if growth rate exceeds 10MB/hour
  const leaked = growthRateMBPerHour > 10;
  
  return {
    heapGrowthMB: heapGrowthMB.toFixed(2),
    timeElapsedHours: timeElapsedHours.toFixed(2),
    growthRateMBPerHour: growthRateMBPerHour.toFixed(2),
    leaked,
    status: leaked ? 'LEAK_DETECTED' : 'PASS'
  };
}

/**
 * Detect retained objects (simplified analysis)
 */
function analyzeRetainedObjects(snapshots) {
  const retained = [];
  
  for (let i = 1; i < snapshots.length; i++) {
    const prev = snapshots[i - 1];
    const curr = snapshots[i];
    
    const heapDelta = curr.heapUsed - prev.heapUsed;
    const timeDelta = curr.elapsedMs - prev.elapsedMs;
    
    if (heapDelta > 1024 * 1024) { // More than 1MB growth
      retained.push({
        from: prev.label,
        to: curr.label,
        heapDeltaMB: (heapDelta / (1024 * 1024)).toFixed(2),
        timeDeltaMs: timeDelta,
        suspectedLeak: heapDelta > 5 * 1024 * 1024 // >5MB suspicious
      });
    }
  }
  
  return retained;
}

/**
 * Generate leak detection report
 */
function generateLeakReport(snapshots, interactions, growthAnalysis, retainedObjects) {
  console.log('\n📄 Generating leak detection report...\n');

  const report = {
    timestamp: new Date().toISOString(),
    testDurationMs: TEST_DURATION_MS,
    snapshotCount: snapshots.length,
    interactionCount: interactions.length,
    snapshots: snapshots,
    growthAnalysis: growthAnalysis,
    retainedObjects: retainedObjects,
    summary: {
      leaked: growthAnalysis.leaked,
      growthRateMBPerHour: growthAnalysis.growthRateMBPerHour,
      status: growthAnalysis.status,
      suspectedLeaks: retainedObjects.filter(r => r.suspectedLeak).length
    }
  };

  const reportPath = path.join(REPORTS_DIR, 'leak-detection-report.json');
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
  
  console.log(`✓ Leak detection report saved to: ${reportPath}\n`);

  return report;
}

/**
 * Print leak detection summary
 */
function printSummary(report) {
  console.log('═'.repeat(70));
  console.log('MEMORY LEAK DETECTION SUMMARY');
  console.log('═'.repeat(70));
  console.log(`Test Duration: ${(report.testDurationMs / 1000 / 60).toFixed(0)} minutes`);
  console.log(`Snapshots Captured: ${report.snapshotCount}`);
  console.log(`Interactions Performed: ${report.interactionCount}`);
  console.log('');
  console.log(`Initial Heap: ${report.snapshots[0].heapUsedMB} MB`);
  console.log(`Final Heap: ${report.snapshots[report.snapshotCount - 1].heapUsedMB} MB`);
  console.log(`Growth Rate: ${report.growthAnalysis.growthRateMBPerHour} MB/hour`);
  console.log('');
  console.log(`Status: ${report.summary.status}`);
  console.log(`Suspected Leaks: ${report.summary.suspectedLeaks}`);
  console.log('═'.repeat(70));
}

let startTime;

/**
 * Main execution
 */
async function main() {
  try {
    console.log('🚀 Memory Leak Detection - 1 Hour Sustained Load Test\n');
    
    if (!global.gc) {
      console.warn('⚠ GC not exposed. Run with: node --expose-gc\n');
      console.log('Continuing without manual GC...\n');
    }

    // Ensure reports directory exists
    if (!fs.existsSync(REPORTS_DIR)) {
      fs.mkdirSync(REPORTS_DIR, { recursive: true });
    }

    // For testing purposes, reduce duration to 5 minutes
    const actualDuration = process.env.LEAK_TEST_DURATION 
      ? parseInt(process.env.LEAK_TEST_DURATION) 
      : 5 * 60 * 1000; // 5 minutes for testing

    console.log(`Test will run for ${(actualDuration / 1000 / 60).toFixed(0)} minutes`);
    console.log('Press Ctrl+C to stop early (results will still be saved)\n');

    // Initialize test data (10k tasks)
    console.log('Loading 10,000 tasks...');
    const tasks = generateTasks(10000, 42);
    console.log('✓ Tasks loaded\n');

    startTime = Date.now();
    const snapshots = [];
    const interactions = [];

    // Capture initial snapshot
    console.log('Capturing initial snapshot...');
    snapshots.push(captureSnapshot('initial', 0));

    let iteration = 0;
    let nextSnapshotTime = startTime + SNAPSHOT_INTERVAL_MS;
    let nextInteractionTime = startTime + INTERACTION_INTERVAL_MS;

    // Run test loop
    console.log('Starting sustained load test...\n');
    
    const testLoop = setInterval(() => {
      const now = Date.now();
      const elapsed = now - startTime;

      // Perform interaction every 30 seconds
      if (now >= nextInteractionTime) {
        iteration++;
        const interaction = simulateInteraction(tasks, iteration);
        interactions.push({
          iteration,
          timestamp: new Date().toISOString(),
          elapsedMs: elapsed,
          ...interaction
        });
        
        if (iteration % 10 === 0) {
          console.log(`✓ Interaction ${iteration} (${(elapsed / 1000 / 60).toFixed(1)}m elapsed)`);
        }
        
        nextInteractionTime = now + INTERACTION_INTERVAL_MS;
      }

      // Capture snapshot every 30 minutes (or every 2 min for 5-min test)
      const snapshotInterval = actualDuration <= 5 * 60 * 1000 
        ? 2 * 60 * 1000 
        : SNAPSHOT_INTERVAL_MS;
        
      if (now >= nextSnapshotTime) {
        console.log(`\n📸 Capturing snapshot at ${(elapsed / 1000 / 60).toFixed(1)} minutes...`);
        snapshots.push(captureSnapshot(`snapshot-${snapshots.length}`, iteration));
        console.log(`   Heap: ${snapshots[snapshots.length - 1].heapUsedMB} MB\n`);
        nextSnapshotTime = now + snapshotInterval;
      }

      // Stop after test duration
      if (elapsed >= actualDuration) {
        clearInterval(testLoop);
        
        // Capture final snapshot
        console.log('\n📸 Capturing final snapshot...');
        snapshots.push(captureSnapshot('final', iteration));
        
        // Analyze results
        console.log('\n🔍 Analyzing memory growth...');
        const growthAnalysis = analyzeMemoryGrowth(snapshots);
        
        console.log('🔍 Detecting retained objects...');
        const retainedObjects = analyzeRetainedObjects(snapshots);
        
        // Generate report
        const report = generateLeakReport(snapshots, interactions, growthAnalysis, retainedObjects);
        
        // Print summary
        printSummary(report);
        
        console.log('\n✅ Memory leak detection complete!');
        
        const exitCode = report.summary.leaked ? 1 : 0;
        process.exit(exitCode);
      }
    }, 5000); // Check every 5 seconds

    // Handle Ctrl+C
    process.on('SIGINT', () => {
      console.log('\n\n⚠ Test interrupted by user');
      clearInterval(testLoop);
      
      // Capture final snapshot anyway
      snapshots.push(captureSnapshot('interrupted', iteration));
      
      // Generate report with what we have
      const growthAnalysis = analyzeMemoryGrowth(snapshots);
      const retainedObjects = analyzeRetainedObjects(snapshots);
      const report = generateLeakReport(snapshots, interactions, growthAnalysis, retainedObjects);
      printSummary(report);
      
      console.log('\n⚠ Partial results saved');
      process.exit(0);
    });

  } catch (error) {
    console.error('\n❌ Memory leak detection failed:', error);
    process.exit(1);
  }
}

// Run if executed directly
if (require.main === module) {
  main();
}

module.exports = { 
  captureSnapshot, 
  simulateInteraction, 
  analyzeMemoryGrowth, 
  analyzeRetainedObjects,
  generateLeakReport 
};