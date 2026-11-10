# Phase 4 Profiling Guide

IntentLens Performance Profiling & Optimization Guide for v0.6.0

## Table of Contents

1. [Quick Start](#quick-start)
2. [Running Benchmarks](#running-benchmarks)
3. [Interpreting Results](#interpreting-results)
4. [Profiling Tools](#profiling-tools)
5. [Troubleshooting](#troubleshooting)

## Quick Start

Run all performance tests:

```bash
npm run bench:all      # Run all benchmarks
npm run profile:cpu    # CPU profiling
npm run profile:memory # Memory profiling  
npm run report         # Generate HTML reports
```

## Running Benchmarks

### 1. Component Benchmarks

Test individual UI components at different scales:

```bash
# Task Panel (1k, 5k, 10k, 50k tasks)
node extension/test/profiling/run-benchmarks.js task-panel

# Boundary Decorations  
node extension/test/profiling/run-benchmarks.js boundary

# Call Path View
node extension/test/profiling/run-benchmarks.js callpath

# Task Management Panel
node extension/test/profiling/run-benchmarks.js taskpanel
```

### 2. Baseline Profiling

Establish performance baselines:

```bash
# Heap snapshots at 0/1k/5k/10k/50k scales
node --expose-gc extension/test/profiling/baseline-heap-snapshot.js

# CPU profiling
node extension/test/profiling/baseline-cpu-profile.js

# Generate unified baseline
node extension/test/profiling/baseline-report.js
```

### 3. Memory Leak Detection

Run sustained load tests:

```bash
# 1-hour simulation (5-min for testing)
node --expose-gc extension/test/profiling/leak-detector.js

# Allocation hotspots
node --expose-gc extension/test/profiling/allocation-analysis.js
```

### 4. Optimization Tests

Validate optimization strategies:

```bash
# Lazy decoration (viewport culling, debouncing)
node extension/test/profiling/lazy-decoration-benchmark.js

# Memory pooling & reuse
node --expose-gc extension/test/profiling/memory-optimization-test.js

# CPU debouncing & indexed filtering
node extension/test/profiling/cpu-optimization-test.js

# Load testing (sustained, concurrent, rapid)
node extension/test/profiling/load-testing-report.js
```

### 5. Generate Reports

Create HTML reports from all profiling data:

```bash
node extension/test/profiling/generate-reports-simple.js
```

Reports generated in `docs/performance-reports/`:
- `phase-4-latency-report.html` - Latency metrics vs targets
- `phase-4-memory-report.html` - Heap analysis & leak detection
- `phase-4-optimization-summary.html` - Optimization impact
- `phase-4-summary.json` - Aggregate JSON data

## Interpreting Results

### Latency Metrics

**Percentiles (p50/p95/p99):**
- **p50 (median)**: 50% of operations complete within this time
- **p95**: 95% complete (filters outliers)  
- **p99**: 99% complete (catches worst-case)

**Targets (at 10k task scale):**
- Task Panel Load: ≤500ms (p95)
- Boundary Render (100): ≤300ms (p95)
- Call Path Expand (100): ≤200ms (p95)
- Filter/Sort: ≤100ms (p95)

**Example Output:**
```json
{
  "p50": 0.38,
  "p95": 0.60,  
  "p99": 0.73,
  "status": "PASS"  // p95 < 100ms target
}
```

### Memory Metrics

**Heap Analysis:**
- **Baseline Heap**: Memory at 0 tasks (target: ≤50MB)
- **Peak Memory**: Memory at 10k tasks (target: ≤150MB)
- **Growth Ratio**: Avg memory increase per 10x scale (target: <2x)

**Memory Leak Detection:**
- **Growth Rate**: MB/hour during sustained load (target: ≤10MB/hour)
- **Status PASS**: No leaks detected, memory stable

**Example Output:**
```json
{
  "baselineHeapMB": 3.88,
  "scale10kHeapMB": 6.53,
  "avgGrowthRatio": "1.55",  // 1.55x growth per 10x scale (PASS)
  "growthRateMBPerHour": 0.84  // 0.84 MB/hour (PASS)
}
```

### Optimization Impact

**CPU Optimizations:**
- **Debouncing**: % reduction in function calls (target: >80%)
- **Indexed Filtering**: Speedup vs linear scan (target: >2x at 10k)
- **Combined Filter+Sort**: p95 latency (target: ≤100ms)

**Memory Optimizations:**
- **Decoration Reuse**: % memory reduction (target: >50%)
- **Object Pooling**: Reuse rate (target: >80%)
- **Lazy Loading**: Reduction in initial load

**Example Output:**
```json
{
  "debounceResults": {
    "reductionPercent": "90.0%",  // 10 calls → 1 execution
    "status": "PASS"
  },
  "filterResults": {
    "speedup": "4.27x",  // Indexed 4.27x faster than linear
    "status": "PASS"
  }
}
```

## Profiling Tools

### PerformanceMonitor

Core profiling utility (used by all benchmarks):

```javascript
const { PerformanceMonitor } = require('./PerformanceMonitor');

const monitor = new PerformanceMonitor();
const result = await monitor.measure('operation-name', () => {
  // Your code here
}, { iterations: 10 });

console.log(result.latency.p95);  // 95th percentile
console.log(result.memoryDelta.heapUsed);  // MB delta
```

### Benchmark Runner

Automated benchmark execution:

```javascript
const { runBenchmark } = require('./run-benchmarks');

runBenchmark({
  name: 'My Benchmark',
  fn: async () => { /* ... */ },
  iterations: 10
});
```

### Test Fixtures

Deterministic test data generation:

```javascript
const { generateTasks } = require('../fixtures/task-fixtures');
const { generateCallTree } = require('../fixtures/tree-fixtures');

const tasks = generateTasks(10000);  // 10k tasks
const tree = generateCallTree(20, 100);  // depth 20, 100 children/node
```

## Troubleshooting

### Issue: "Heap out of memory"

**Cause**: Testing at 50k+ task scale without sufficient heap.

**Fix**: Increase Node heap size:
```bash
node --max-old-space-size=4096 extension/test/profiling/baseline-heap-snapshot.js
```

### Issue: "GC not exposed"

**Cause**: Running memory tests without `--expose-gc` flag.

**Fix**: Always use `--expose-gc` for memory tests:
```bash
node --expose-gc extension/test/profiling/leak-detector.js
```

### Issue: Benchmarks show high variance

**Cause**: System load interference or GC pauses.

**Fix**:
1. Close other applications
2. Increase iterations: `{ iterations: 20 }`
3. Use p95 instead of mean to filter outliers

### Issue: CPU profile not generated

**Cause**: V8 profiler logs require post-processing on Windows.

**Fix**: Use alternative profiling:
```bash
node --inspect extension/test/profiling/baseline-cpu-profile.js
# Open chrome://inspect in Chrome DevTools
```

### Issue: Reports show "N/A" data

**Cause**: JSON reports not generated before running report generator.

**Fix**: Run benchmarks first:
```bash
npm run bench:all
npm run report
```

## Performance Targets Summary

| Metric | Target | Phase 4 Result |
|--------|--------|----------------|
| Filter/Sort 10k (p95) | ≤100ms | 0.60ms ✅ |
| Baseline Heap | ≤50MB | 3.88MB ✅ |
| Peak Memory 10k | ≤150MB | 6.53MB ✅ |
| Growth Ratio | <2.0x | 1.55x ✅ |
| Memory Leak | ≤10MB/hour | 0.84MB/hour ✅ |
| Debouncing | >80% reduction | 90% ✅ |
| Indexed Filter | >2x speedup | 4-6x ✅ |

## Next Steps

1. **Phase 5 Beta**: Monitor real-world performance metrics
2. **Regression Testing**: Run benchmarks on each release
3. **Optimization Candidates**: Virtual scrolling for 50k+ scales
4. **Telemetry**: Collect anonymous performance data from users

---

*For issues or questions, see `docs/optimization-log.md` for implementation details.*
