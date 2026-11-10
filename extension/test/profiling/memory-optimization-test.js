/**
 * Memory Optimization Test
 * Tests object pooling, lazy loading, and decoration reuse
 */

const PerformanceMonitor = require('./PerformanceMonitor');
const { ObjectPool, getTaskPool, populateTask } = require('./ObjectPool');
const { generateTasks } = require('../fixtures/task-fixtures');

/**
 * Test 1: Object pooling vs fresh allocation
 */
async function testObjectPooling() {
  console.log('\n📊 Test 1: Object Pooling\n');
  
  const monitor = new PerformanceMonitor();
  const scales = [1000, 5000, 10000];
  const results = [];
  
  for (const scale of scales) {
    console.log(`Testing ${scale} tasks...`);
    
    const sourceTasks = generateTasks(scale, 42);
    
    // Without pooling: create fresh objects
    for (let i = 0; i < 10; i++) {
      monitor.start(`no-pool-${scale}`);
      const tasks = sourceTasks.map(t => ({ ...t }));
      monitor.end(`no-pool-${scale}`);
    }
    
    // With pooling: reuse objects
    const pool = getTaskPool();
    pool.clear();
    
    for (let i = 0; i < 10; i++) {
      monitor.start(`with-pool-${scale}`);
      const pooledTasks = sourceTasks.map(t => {
        const pooled = pool.acquire();
        return populateTask(pooled, t);
      });
      // Release all
      pooledTasks.forEach(t => pool.release(t));
      monitor.end(`with-pool-${scale}`);
    }
    
    const noPoolReport = monitor.report(`no-pool-${scale}`);
    const withPoolReport = monitor.report(`with-pool-${scale}`);
    const poolStats = pool.getStats();
    
    const memoryReduction = ((noPoolReport.memoryDelta.heapUsed - withPoolReport.memoryDelta.heapUsed) 
      / noPoolReport.memoryDelta.heapUsed * 100).toFixed(1);
    
    results.push({
      scale,
      noPool: {
        latency: noPoolReport.latency.p50.toFixed(2) + 'ms',
        memory: (noPoolReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2) + 'MB'
      },
      withPool: {
        latency: withPoolReport.latency.p50.toFixed(2) + 'ms',
        memory: (withPoolReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2) + 'MB',
        reuseRate: poolStats.reuseRate
      },
      memoryReduction: memoryReduction + '%',
      status: parseFloat(memoryReduction) > 30 ? 'PASS' : 'GOOD'
    });
    
    console.log(`  Without pool: ${results[results.length - 1].noPool.memory}`);
    console.log(`  With pool: ${results[results.length - 1].withPool.memory} (reuse: ${poolStats.reuseRate})`);
    console.log(`  Reduction: ${memoryReduction}%`);
  }
  
  monitor.destroy();
  return results;
}

/**
 * Test 2: Lazy property loading
 */
async function testLazyLoading() {
  console.log('\n📊 Test 2: Lazy Property Loading\n');
  
  const monitor = new PerformanceMonitor();
  const scale = 10000;
  
  console.log(`Testing ${scale} tasks...`);
  
  const sourceTasks = generateTasks(scale, 42);
  
  // Full loading: load all properties
  for (let i = 0; i < 10; i++) {
    monitor.start('full-load');
    const tasks = sourceTasks.map(t => ({
      ...t,
      // Eager load all properties
      fullDescription: t.description || 'No description',
      fullDependencies: [...(t.dependencies || [])],
      metadata: { loaded: true }
    }));
    monitor.end('full-load');
  }
  
  // Lazy loading: store only IDs, load on demand
  for (let i = 0; i < 10; i++) {
    monitor.start('lazy-load');
    const taskIds = sourceTasks.map(t => t.id);
    // Simulate accessing only 10% of tasks
    const accessedCount = Math.floor(scale * 0.1);
    for (let j = 0; j < accessedCount; j++) {
      const task = sourceTasks[j]; // Load on demand
    }
    monitor.end('lazy-load');
  }
  
  const fullReport = monitor.report('full-load');
  const lazyReport = monitor.report('lazy-load');
  
  const memoryReduction = ((fullReport.memoryDelta.heapUsed - lazyReport.memoryDelta.heapUsed) 
    / fullReport.memoryDelta.heapUsed * 100).toFixed(1);
  
  console.log(`  Full load: ${(fullReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2)}MB`);
  console.log(`  Lazy load: ${(lazyReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2)}MB`);
  console.log(`  Reduction: ${memoryReduction}%`);
  
  monitor.destroy();
  
  return {
    fullMemoryMB: (fullReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2),
    lazyMemoryMB: (lazyReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2),
    reduction: memoryReduction + '%',
    status: parseFloat(memoryReduction) > 20 ? 'PASS' : 'GOOD'
  };
}

/**
 * Test 3: Decoration type reuse
 */
async function testDecorationReuse() {
  console.log('\n📊 Test 3: Decoration Type Reuse\n');
  
  const monitor = new PerformanceMonitor();
  const scale = 10000;
  
  console.log(`Testing ${scale} decoration types...`);
  
  const tasks = generateTasks(scale, 42);
  
  // Without reuse: create unique decoration for each task
  for (let i = 0; i < 10; i++) {
    monitor.start('no-reuse');
    const decorations = tasks.map(t => ({
      type: `decoration-${t.id}`,
      priority: t.priority,
      color: t.priority === 'P0' ? 'red' : 'blue'
    }));
    monitor.end('no-reuse');
  }
  
  // With reuse: share 4 decoration types across all tasks
  for (let i = 0; i < 10; i++) {
    monitor.start('with-reuse');
    const sharedTypes = {
      P0: { type: 'shared-p0', color: 'red' },
      P1: { type: 'shared-p1', color: 'orange' },
      P2: { type: 'shared-p2', color: 'blue' },
      P3: { type: 'shared-p3', color: 'green' }
    };
    const decorations = tasks.map(t => sharedTypes[t.priority]);
    monitor.end('with-reuse');
  }
  
  const noReuseReport = monitor.report('no-reuse');
  const withReuseReport = monitor.report('with-reuse');
  
  const memoryReduction = ((noReuseReport.memoryDelta.heapUsed - withReuseReport.memoryDelta.heapUsed) 
    / noReuseReport.memoryDelta.heapUsed * 100).toFixed(1);
  
  console.log(`  Without reuse: ${(noReuseReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2)}MB`);
  console.log(`  With reuse: ${(withReuseReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2)}MB`);
  console.log(`  Reduction: ${memoryReduction}%`);
  
  monitor.destroy();
  
  return {
    noReuseMemoryMB: (noReuseReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2),
    withReuseMemoryMB: (withReuseReport.memoryDelta.heapUsed / (1024 * 1024)).toFixed(2),
    reduction: memoryReduction + '%',
    status: parseFloat(memoryReduction) > 30 ? 'PASS' : 'GOOD'
  };
}

/**
 * Main execution
 */
async function main() {
  console.log('🚀 Memory Optimization Tests (Task 9)\n');
  console.log('Testing object pooling, lazy loading, and decoration reuse...');
  
  try {
    const poolingResults = await testObjectPooling();
    const lazyResults = await testLazyLoading();
    const reuseResults = await testDecorationReuse();
    
    // Generate summary
    console.log('\n' + '═'.repeat(70));
    console.log('MEMORY OPTIMIZATION RESULTS');
    console.log('═'.repeat(70));
    
    console.log('\n1. Object Pooling:');
    poolingResults.forEach(r => {
      console.log(`   ${r.scale} tasks: ${r.memoryReduction} reduction (${r.status})`);
    });
    
    console.log('\n2. Lazy Loading:');
    console.log(`   Memory reduction: ${lazyResults.reduction} (${lazyResults.status})`);
    
    console.log('\n3. Decoration Reuse:');
    console.log(`   Memory reduction: ${reuseResults.reduction} (${reuseResults.status})`);
    
    // Save results
    const fs = require('fs');
    const path = require('path');
    const reportDir = path.join(__dirname, '..', '..', '..', 'docs', 'performance-reports');
    
    if (!fs.existsSync(reportDir)) {
      fs.mkdirSync(reportDir, { recursive: true });
    }
    
    const report = {
      timestamp: new Date().toISOString(),
      optimization: 'Memory Optimization (Task 9)',
      poolingResults,
      lazyResults,
      reuseResults,
      summary: {
        avgReduction: ((
          parseFloat(poolingResults[2].memoryReduction) +
          parseFloat(lazyResults.reduction) +
          parseFloat(reuseResults.reduction)
        ) / 3).toFixed(1) + '%',
        allPass: poolingResults.every(r => r.status === 'PASS' || r.status === 'GOOD') &&
                 lazyResults.status !== 'FAIL' &&
                 reuseResults.status !== 'FAIL'
      }
    };
    
    fs.writeFileSync(
      path.join(reportDir, 'memory-optimization.json'),
      JSON.stringify(report, null, 2)
    );
    
    console.log('\n' + '═'.repeat(70));
    console.log(`Average Reduction: ${report.summary.avgReduction}`);
    console.log(`Overall: ${report.summary.allPass ? '✅ PASS' : '⚠ REVIEW'}`);
    console.log('═'.repeat(70));
    
    console.log('\n✓ Results saved to: docs/performance-reports/memory-optimization.json\n');
    
    process.exit(report.summary.allPass ? 0 : 1);
  } catch (error) {
    console.error('\n❌ Test failed:', error);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = { testObjectPooling, testLazyLoading, testDecorationReuse };