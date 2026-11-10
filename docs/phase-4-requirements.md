# Phase 4 Performance Testing: Requirements

**Version**: 1.0  
**Date**: 2026-11-06  
**Derived From**: phase-4-design.md  
**Status**: Ready for Implementation

---

## Overview

Phase 4 requires implementing a comprehensive performance testing suite to validate that all 4 UI components meet production readiness criteria. Requirements span benchmarking infrastructure, profiling harness, optimization targets, and reporting.

---

## Functional Requirements

### FR1: Benchmarking Harness

**Requirement**: Build a reusable benchmarking framework that measures operation latency across different load scales.

**Sub-Requirements**:
- FR1.1: Implement PerformanceMonitor class that records operation timing
- FR1.2: Support percentile reporting (p50, p95, p99 latency)
- FR1.3: Measure memory delta before/after each operation
- FR1.4: Record GC pause count and total pause duration
- FR1.5: Output results as JSON for regression tracking

**Acceptance Criteria**:
- ✅ PerformanceMonitor can measure any async operation
- ✅ Percentiles calculated accurately from 100+ samples
- ✅ Memory measurement ±1MB accuracy
- ✅ GC tracking works on Node.js 16+

---

### FR2: Task Panel Performance Tests

**Requirement**: Measure TaskTreeProvider performance at 1k, 5k, 10k, 50k task scales.

**Sub-Requirements**:
- FR2.1: Test provider initialization time (create with N tasks)
- FR2.2: Test getChildren() latency (root expansion)
- FR2.3: Test applyFilter() with common predicates (priority, status)
- FR2.4: Test applySortBy() with multiple sort keys
- FR2.5: Measure memory consumption for N tasks in memory

**Acceptance Criteria**:
- ✅ Load 10k tasks in ≤500ms
- ✅ Filter 10k tasks in ≤100ms (after debounce)
- ✅ Peak memory ≤150MB at 10k scale
- ✅ Results show no regressions vs. baseline

---

### FR3: Boundary Decoration Performance Tests

**Requirement**: Measure TaskBoundaryDecorator rendering performance.

**Sub-Requirements**:
- FR3.1: Test applyDecorations() latency for 100, 500, 1k tasks in single file
- FR3.2: Test VS Code setDecorations() API call timing
- FR3.3: Test hover provider registration overhead
- FR3.4: Test click handler registration for N decorations
- FR3.5: Measure decoration object memory footprint

**Acceptance Criteria**:
- ✅ Render 100 task boundaries in ≤300ms
- ✅ Decoration cache memory ≤30MB
- ✅ Hover/click activation ≤50ms latency
- ✅ Lazy decoration mode reduces render time by 40%+

---

### FR4: Call Path Performance Tests

**Requirement**: Measure EnhancedCallPathProvider tree processing performance.

**Sub-Requirements**:
- FR4.1: Test getChildren() with varying tree depth (10, 15, 20, 30 levels)
- FR4.2: Test sorting children array with 100, 500, 1k children per node
- FR4.3: Test recursive tree expansion (auto-expand 3 levels deep)
- FR4.4: Test search filtering on large trees
- FR4.5: Measure tree structure memory consumption

**Acceptance Criteria**:
- ✅ Expand node with 100 children in ≤200ms
- ✅ Sort 1k children in ≤50ms
- ✅ Search tree (depth 20) in ≤100ms
- ✅ Virtual scrolling reduces memory usage by 60%

---

### FR5: Task Management Panel Performance Tests

**Requirement**: Measure TaskManagementPanel interactions and validation.

**Sub-Requirements**:
- FR5.1: Test Webview HTML generation + CSS inlining time
- FR5.2: Test form rendering in Webview (show command)
- FR5.3: Test cyclic dependency detection (DFS) for task with M dependencies
- FR5.4: Test form validation latency (key press → validation < 50ms)
- FR5.5: Measure Webview memory consumption

**Acceptance Criteria**:
- ✅ Panel opens in ≤400ms
- ✅ Form validation completes in ≤50ms
- ✅ Cyclic dep detection on 100 deps in ≤10ms
- ✅ Webview memory footprint ≤20MB

---

### FR6: Memory Profiling Suite

**Requirement**: Generate detailed memory analysis across load scales.

**Sub-Requirements**:
- FR6.1: Capture baseline heap snapshot (no tasks)
- FR6.2: Capture heap snapshots at 1k, 5k, 10k, 50k task levels
- FR6.3: Analyze allocation timeline (top 10 allocation hotspots)
- FR6.4: Identify retained objects causing memory growth
- FR6.5: Detect memory leaks (objects not freed after 1 hour session)

**Acceptance Criteria**:
- ✅ Baseline heap ≤50MB
- ✅ 10k tasks peak ≤150MB
- ✅ Heap growth rate < 2x per 10x load scale
- ✅ No memory leaks detected in 1-hour session

---

### FR7: Load Testing Suite

**Requirement**: Validate performance under sustained and concurrent load.

**Sub-Requirements**:
- FR7.1: Run 1-hour sustained load test with 10k tasks
- FR7.2: Perform periodic interactions every 30 seconds (filter, expand, navigate)
- FR7.3: Monitor for memory growth over time
- FR7.4: Run concurrent operations (load + expand + render) simultaneously
- FR7.5: Test rapid filter changes (5 changes/sec for 30 seconds)

**Acceptance Criteria**:
- ✅ 1-hour session completes without crashes
- ✅ Memory growth ≤10MB/hour (< 10% over baseline)
- ✅ Latency remains stable (p95 < 1.5x baseline)
- ✅ GC frequency ≤2/sec at full load

---

### FR8: Performance Reporting

**Requirement**: Generate automated performance reports comparing results to targets and baseline.

**Sub-Requirements**:
- FR8.1: Generate latency table (all operations, all load levels)
- FR8.2: Generate memory graph (heap vs. task count)
- FR8.3: Generate CPU profile flamegraphs from v8 profiler output
- FR8.4: Compare against performance targets (pass/fail)
- FR8.5: Compare against v0.5.2 baseline (regression detection)
- FR8.6: Export reports as HTML + JSON

**Acceptance Criteria**:
- ✅ All metrics vs. targets clearly shown
- ✅ Regression flagged if latency increases >10%
- ✅ Reports include before/after optimization
- ✅ Reports archived in /docs/performance-reports/

---

## Non-Functional Requirements

### NFR1: Zero External Dependencies

**Requirement**: All benchmarking and profiling uses only Node.js built-ins and VS Code Extension API.

**Rationale**: Keep extension lightweight, avoid npm supply chain risk.

**Acceptance Criteria**:
- ✅ No new npm packages added
- ✅ Uses only node built-ins: perf_hooks, v8, fs, etc.
- ✅ Uses only VS Code Extension API (no custom fork)

---

### NFR2: Instrumentation Removability

**Requirement**: All profiling and instrumentation code must be feature-gated and removable.

**Rationale**: Production release must not include profiling overhead.

**Acceptance Criteria**:
- ✅ PerformanceMonitor disabled in production builds
- ✅ All instrumentation in /test/profiling/ directory
- ✅ Zero profiling code in /src/ extension files

---

### NFR3: Reproducibility

**Requirement**: All benchmarks must produce consistent results across multiple runs.

**Rationale**: Regression detection requires stable baseline.

**Acceptance Criteria**:
- ✅ 10 consecutive runs produce p95 latency within ±5% variance
- ✅ Test data generation is deterministic (fixed seed)
- ✅ Hardware fingerprint recorded in reports

---

### NFR4: Code Quality

**Requirement**: All benchmark and profiling code is tested and reviewed.

**Rationale**: Benchmarks are code too; garbage-in = garbage-out.

**Acceptance Criteria**:
- ✅ All benchmark files have unit test coverage >80%
- ✅ Benchmark code passes npm run check
- ✅ Results validated by manual spot-check before merge

---

## Test Strategy

### Test Execution

- Benchmarks run in Node.js directly (not in VS Code process)
- Memory profiling uses node --inspect for precision
- CPU profiling uses node --prof + v8 analysis
- Load testing uses isolated process to avoid VS Code interference

### Test Data

- Generate 1k, 5k, 10k, 50k task objects using fixtures
- Task structure matches real extension data model
- Call trees generated with varying depth and branching

### Success Metrics

- All latency targets met
- All memory targets met
- No regressions from v0.5.2 baseline
- Optimization strategies implemented and measured

---

## Deliverables

1. **Benchmarking Harness** (`extension/test/profiling/`)
   - PerformanceMonitor.js
   - Benchmark runner (load benchmarks, aggregate results)

2. **Benchmark Tests** (`extension/test/benchmarks/`)
   - task-panel-bench.js
   - boundary-bench.js
   - callpath-bench.js
   - taskpanel-bench.js

3. **Profiling Tools** (`extension/test/profiling/`)
   - heap-snapshot.js (capture at each load level)
   - allocation-timeline.js (analyze hotspots)
   - cpu-profile.js (flamegraph generation)

4. **Load Testing** (`extension/test/loadtest/`)
   - sustained-load-test.js (1 hour with interactions)
   - concurrent-ops-test.js (load + expand + render)

5. **Reports** (`docs/performance-reports/`)
   - latency-report.html (all operations vs. targets)
   - memory-report.html (heap graphs)
   - regression-report.json (baseline comparison)
   - optimization-before-after.html (improvements from each strategy)

6. **Documentation** (`docs/`)
   - profiling-guide.md (how to run benchmarks, interpret results)
   - optimization-log.md (changes made and their impact)

---

## Acceptance Criteria Summary

✅ **Performance**: All targets met at 10k task scale  
✅ **Regression**: No regressions vs. v0.5.2 baseline  
✅ **Quality**: Benchmark code tested and reviewed  
✅ **Reporting**: Automated reports for all metrics  
✅ **Optimization**: Strategies implemented and measured  
✅ **Documentation**: Profiling guide for maintainers  

---

## Timeline

- Week 1: Benchmarking harness + baseline (28 hours)
- Week 2: Memory profiling + rendering optimization (32 hours)
- Week 3: CPU optimization + load testing + validation (32 hours)

**Total**: ~92 hours (~3 weeks)

---

*End of Requirements Document*