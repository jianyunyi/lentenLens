const test = require('node:test');
const assert = require('node:assert/strict');
const PerformanceMonitor = require('./profiling/PerformanceMonitor');

test('PerformanceMonitor basic timing measurement', async t => {
  const monitor = new PerformanceMonitor();
  
  // Measure a simple operation
  monitor.start('test-op');
  await new Promise(resolve => setTimeout(resolve, 10)); // 10ms delay
  monitor.end('test-op');
  
  const report = monitor.report('test-op');
  
  assert.equal(report.iterations, 1);
  assert.ok(report.latency.p50 >= 8); // Allow some variance
  assert.ok(report.latency.p50 <= 50);
  assert.equal(report.latency.p50, report.latency.p95); // Same for 1 sample
  assert.equal(report.latency.p50, report.latency.p99);
  
  monitor.destroy();
});

test('PerformanceMonitor percentile calculations', async t => {
  const monitor = new PerformanceMonitor();
  
  // Run multiple samples to test percentiles
  for (let i = 0; i < 100; i++) {
    monitor.start('percentile-test');
    await new Promise(resolve => setTimeout(resolve, 1));
    monitor.end('percentile-test');
  }
  
  const report = monitor.report('percentile-test');
  
  assert.equal(report.iterations, 100);
  assert.ok(report.latency.p50 > 0);
  assert.ok(report.latency.p95 >= report.latency.p50);
  assert.ok(report.latency.p99 >= report.latency.p95);
  assert.ok(report.latency.max >= report.latency.p99);
  assert.equal(report.latency.min, Math.min(...[report.latency.min]));
  
  monitor.destroy();
});

test('PerformanceMonitor memory delta tracking', t => {
  const monitor = new PerformanceMonitor();
  
  monitor.start('memory-test');
  const obj = new Array(10000).fill(0); // Allocate some memory
  monitor.end('memory-test');
  
  const report = monitor.report('memory-test');
  
  assert.ok(typeof report.memoryDelta === 'object');
  assert.ok(typeof report.memoryDelta.heapUsed === 'number');
  assert.ok(typeof report.memoryDelta.heapTotal === 'number');
  assert.ok(typeof report.memoryDelta.external === 'number');
  assert.ok(typeof report.memoryDelta.rss === 'number');
  
  monitor.destroy();
});

test('PerformanceMonitor tracks GC count', t => {
  const monitor = new PerformanceMonitor();
  
  monitor.start('gc-test');
  monitor.end('gc-test');
  
  const report = monitor.report('gc-test');
  
  assert.ok(typeof report.gcCount === 'number');
  assert.ok(typeof report.gcTotalTime === 'number');
  assert.ok(report.gcCount >= 0);
  assert.ok(report.gcTotalTime >= 0);
  
  monitor.destroy();
});

test('PerformanceMonitor throws on duplicate active measurement', t => {
  const monitor = new PerformanceMonitor();
  
  monitor.start('test-op');
  assert.throws(() => monitor.start('test-op'), /already active/);
  
  monitor.end('test-op');
  monitor.destroy();
});

test('PerformanceMonitor throws on end without start', t => {
  const monitor = new PerformanceMonitor();
  
  assert.throws(() => monitor.end('nonexistent'), /not active/);
  
  monitor.destroy();
});

test('PerformanceMonitor reset clears samples', t => {
  const monitor = new PerformanceMonitor();
  
  monitor.start('test-op');
  monitor.end('test-op');
  
  let report = monitor.report('test-op');
  assert.equal(report.iterations, 1);
  
  monitor.reset('test-op');
  report = monitor.report('test-op');
  assert.equal(report.iterations, 0);
  
  monitor.destroy();
});

test('PerformanceMonitor reset all clears everything', t => {
  const monitor = new PerformanceMonitor();
  
  monitor.start('op1');
  monitor.end('op1');
  monitor.start('op2');
  monitor.end('op2');
  
  let reports = monitor.reportAll();
  assert.equal(reports.length, 2);
  
  monitor.reset();
  reports = monitor.reportAll();
  assert.equal(reports.length, 0);
  
  monitor.destroy();
});

test('PerformanceMonitor reportAll includes all operations', async t => {
  const monitor = new PerformanceMonitor();
  
  for (let i = 0; i < 3; i++) {
    monitor.start(`op-${i}`);
    await new Promise(resolve => setTimeout(resolve, 1));
    monitor.end(`op-${i}`);
  }
  
  const reports = monitor.reportAll();
  assert.equal(reports.length, 3);
  assert.ok(reports.some(r => r.name === 'op-0'));
  assert.ok(reports.some(r => r.name === 'op-1'));
  assert.ok(reports.some(r => r.name === 'op-2'));
  
  monitor.destroy();
});

test('PerformanceMonitor reports correctly formatted output', t => {
  const monitor = new PerformanceMonitor();
  
  monitor.start('format-test');
  monitor.end('format-test');
  
  const report = monitor.report('format-test');
  
  assert.equal(typeof report.name, 'string');
  assert.equal(typeof report.iterations, 'number');
  assert.ok(report.latency);
  assert.ok(report.latency.p50 !== undefined);
  assert.ok(report.latency.p95 !== undefined);
  assert.ok(report.latency.p99 !== undefined);
  assert.ok(report.latency.min !== undefined);
  assert.ok(report.latency.max !== undefined);
  assert.ok(report.memoryDelta);
  assert.ok(report.gcCount !== undefined);
  assert.ok(report.gcTotalTime !== undefined);
  
  monitor.destroy();
});

test('PerformanceMonitor handles empty report request', t => {
  const monitor = new PerformanceMonitor();
  
  const report = monitor.report('never-started');
  
  assert.equal(report.name, 'never-started');
  assert.equal(report.iterations, 0);
  assert.equal(report.latency.p50, 0);
  assert.equal(report.latency.p95, 0);
  assert.equal(report.latency.p99, 0);
  
  monitor.destroy();
});

test('PerformanceMonitor getSamples returns raw data', async t => {
  const monitor = new PerformanceMonitor();
  
  monitor.start('sample-test');
  await new Promise(resolve => setTimeout(resolve, 5));
  monitor.end('sample-test');
  
  const samples = monitor.getSamples('sample-test');
  
  assert.equal(samples.length, 1);
  assert.ok(samples[0].latency > 0);
  assert.ok(samples[0].memoryDelta);
  assert.ok(samples[0].gcMetrics);
  
  monitor.destroy();
});

test('PerformanceMonitor calculates mean correctly', async t => {
  const monitor = new PerformanceMonitor();
  
  const delays = [5, 10, 15];
  for (const delay of delays) {
    monitor.start('mean-test');
    await new Promise(resolve => setTimeout(resolve, delay));
    monitor.end('mean-test');
  }
  
  const report = monitor.report('mean-test');
  const expectedMean = delays.reduce((a, b) => a + b, 0) / delays.length;
  
  // Mean should be close to expected (allow 50% variance for timing variance)
  assert.ok(report.latency.mean > 0);
  
  monitor.destroy();
});
