/**
 * Example Benchmark - Demonstrates benchmark structure and PerformanceMonitor usage
 */

const PerformanceMonitor = require('../profiling/PerformanceMonitor');
const { generateTasks } = require('../fixtures/task-fixtures');

/**
 * Example benchmark that measures task generation performance
 */
async function runExampleBenchmark() {
  const monitor = new PerformanceMonitor();
  
  // Measure task generation at different scales
  const scales = [100, 500, 1000];
  
  for (const scale of scales) {
    const opName = `task-generation-${scale}`;
    
    for (let i = 0; i < 10; i++) {
      monitor.start(opName);
      generateTasks(scale, 42 + i);
      monitor.end(opName);
    }
  }
  
  // Collect results - return a single result per benchmark
  // In practice, benchmarks should aggregate their results
  const aggregated = {
    name: 'Example: Task Generation Performance',
    iterations: 0,
    latency: { p50: 0, p95: 0, p99: 0, min: 0, max: 0 },
    memoryDelta: { heapUsed: 0 },
    gcCount: 0,
    gcTotalTime: 0
  };
  
  // Aggregate metrics from all scales
  let totalIterations = 0;
  let allLatencies = [];
  
  for (const scale of scales) {
    const report = monitor.report(`task-generation-${scale}`);
    totalIterations += report.iterations;
    
    for (const sample of monitor.getSamples(`task-generation-${scale}`)) {
      allLatencies.push(sample.latency);
    }
  }
  
  if (allLatencies.length > 0) {
    allLatencies.sort((a, b) => a - b);
    aggregated.iterations = totalIterations;
    aggregated.latency.min = allLatencies[0];
    aggregated.latency.max = allLatencies[allLatencies.length - 1];
    aggregated.latency.p50 = allLatencies[Math.floor(allLatencies.length * 0.5)];
    aggregated.latency.p95 = allLatencies[Math.floor(allLatencies.length * 0.95)];
    aggregated.latency.p99 = allLatencies[Math.floor(allLatencies.length * 0.99)];
  }
  
  monitor.destroy();
  return aggregated;
}

module.exports = {
  name: 'Example: Task Generation Performance',
  run: runExampleBenchmark
};
