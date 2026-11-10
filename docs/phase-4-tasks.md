# Phase 4 Performance Testing: Task Breakdown

**Version**: 1.0  
**Date**: 2026-11-06  
**Derived From**: phase-4-requirements.md  
**Status**: Ready for Execution

---

# Task List

## Task 1: Set Up Performance Monitoring Infrastructure

**Description**: Implement PerformanceMonitor class and benchmarking harness. This is the foundation for all subsequent performance measurements.

**Sub-tasks**:
1. Create PerformanceMonitor class in `/extension/test/profiling/PerformanceMonitor.js`
   - Methods: start(name), end(name), reset(), report()
   - Records timing, memory delta, GC pauses per operation
   - Calculates p50, p95, p99 percentiles
2. Create benchmark runner in `/extension/test/profiling/run-benchmarks.js`
   - Loads all benchmark files from `/extension/test/benchmarks/`
   - Executes benchmarks in sequence (1k, 5k, 10k, 50k load levels)
   - Aggregates results and outputs JSON + console summary
3. Create test fixtures in `/extension/test/fixtures/`
   - generateTasks(count, seed) - deterministic task generation
   - generateCallTree(depth, branching) - nested call structure
   - Mock VS Code API for testing (window, languages, commands)

**Acceptance Criteria**:
- ✅ PerformanceMonitor accurately measures latency (±10ms at 100ms scale)
- ✅ Percentiles calculated correctly from 100+ samples
- ✅ Memory measurement works (GC.collectGarbage() test passes)
- ✅ All fixtures pass unit tests (npm run test)
- ✅ Benchmark runner executes without errors

**Dependencies**: None (foundational task)

**Estimated Hours**: 12

---

## Task 2: Implement Task Panel Benchmarks

**Description**: Create benchmarks for TaskTreeProvider at 1k, 5k, 10k, 50k task scales.

**Sub-tasks**:
1. Create `/extension/test/benchmarks/task-panel-bench.js`
   - Benchmark: initialization (create TaskTreeProvider with N tasks)
   - Benchmark: getChildren() root expansion
   - Benchmark: applyFilter() with priorities
   - Benchmark: applySortBy() with multiple keys
   - Benchmark: tree traversal (enumerate all tasks)
2. Measure against targets:
   - Load 10k: ≤500ms (FR2.2)
   - Filter 10k: ≤100ms (FR2.3)
   - Peak memory: ≤150MB at 10k (FR2.5)
3. Generate before/after results for optimization comparison

**Acceptance Criteria**:
- ✅ All 5 benchmarks run successfully
- ✅ Latency results logged (p50, p95, p99)
- ✅ Memory delta recorded for each benchmark
- ✅ Results output as JSON
- ✅ 10k load ≤500ms baseline established

**Dependencies**: Task 1 (PerformanceMonitor, fixtures)

**Estimated Hours**: 8

---

## Task 3: Implement Boundary Decoration Benchmarks

**Description**: Create benchmarks for TaskBoundaryDecorator rendering performance.

**Sub-tasks**:
1. Create `/extension/test/benchmarks/boundary-bench.js`
   - Benchmark: applyDecorations() for 100, 500, 1k tasks
   - Benchmark: VS Code setDecorations() API call
   - Benchmark: hover provider registration
   - Benchmark: click handler registration
   - Benchmark: decoration memory footprint
2. Mock VS Code TextEditor for testing
3. Measure against targets:
   - Render 100 decorations: ≤300ms (FR3.1)
   - Cache memory: ≤30MB (FR3.5)

**Acceptance Criteria**:
- ✅ All 5 benchmarks run successfully
- ✅ Render time per boundary measured (<3ms per decoration at baseline)
- ✅ VS Code API mocking accurate
- ✅ Results show memory growth with task count
- ✅ Baseline established for optimization

**Dependencies**: Task 1

**Estimated Hours**: 8

---

## Task 4: Implement Call Path Benchmarks

**Description**: Create benchmarks for EnhancedCallPathProvider tree processing.

**Sub-tasks**:
1. Create `/extension/test/benchmarks/callpath-bench.js`
   - Benchmark: getChildren() with depth 10, 15, 20, 30
   - Benchmark: sortChildren() with 100, 500, 1k children
   - Benchmark: recursive expansion (3 levels deep)
   - Benchmark: search filtering
   - Benchmark: tree structure memory
2. Generate call trees with realistic structure (branching factor 3-5)
3. Measure against targets:
   - Expand (100 children): ≤200ms (FR4.1)
   - Sort (1k children): ≤50ms (FR4.2)

**Acceptance Criteria**:
- ✅ All 5 benchmarks run successfully
- ✅ Tree generation deterministic and realistic
- ✅ Latency scales predictably with depth/children
- ✅ Memory consumption tracked
- ✅ Baseline established for virtual scrolling optimization

**Dependencies**: Task 1

**Estimated Hours**: 8

---

## Task 5: Implement Task Management Panel Benchmarks

**Description**: Create benchmarks for TaskManagementPanel interactions.

**Sub-tasks**:
1. Create `/extension/test/benchmarks/taskpanel-bench.js`
   - Benchmark: Webview HTML generation (form with N tasks listed)
   - Benchmark: Form rendering time (panel.show())
   - Benchmark: Cyclic dependency detection (DFS with M dependencies)
   - Benchmark: Form validation latency
   - Benchmark: Webview memory consumption
2. Mock VS Code Webview API
3. Measure against targets:
   - Panel open: ≤400ms (FR5.2)
   - Cyclic dep detection (100 deps): ≤10ms (FR5.4)

**Acceptance Criteria**:
- ✅ All 5 benchmarks run successfully
- ✅ Webview mocking realistic (DOM generation)
- ✅ Cyclic detection algorithm correct (catches cycles)
- ✅ Results show validation latency <50ms
- ✅ Baseline established

**Dependencies**: Task 1

**Estimated Hours**: 8

---

## Task 6: Baseline Profiling (CPU & Memory)

**Description**: Run CPU profiling (v8) and memory profiling (heap snapshots) to establish baseline.

**Sub-tasks**:
1. Create `/extension/test/profiling/baseline-cpu-profile.js`
   - Run all benchmarks under node --prof
   - Process output with v8 profiler
   - Generate flamegraph (identify hotspots)
   - Output: profile.txt (for analysis)
2. Create `/extension/test/profiling/baseline-heap-snapshot.js`
   - Load 0 tasks (baseline)
   - Load 1k, 5k, 10k, 50k tasks
   - Capture heap snapshot at each level
   - Use node --inspect + DevTools or heapdump module
3. Analyze results:
   - CPU: Identify top 5 hot functions per operation
   - Memory: Calculate heap size at each level, growth rate
   - Output: baseline-profile.json (for comparison)

**Acceptance Criteria**:
- ✅ v8 profiling runs successfully
- ✅ Heap snapshots captured at all levels
- ✅ Baseline metrics documented
- ✅ Profile files archived (for future comparison)
- ✅ Memory growth rate calculated (<2x per 10x load)

**Dependencies**: Tasks 2-5 (benchmarks must exist)

**Estimated Hours**: 12

---

## Task 7: Memory Profiling & Leak Detection

**Description**: Analyze memory usage in detail and detect memory leaks.

**Sub-tasks**:
1. Create `/extension/test/profiling/allocation-analysis.js`
   - Use node --expose-gc to force GC and analyze allocation
   - Record allocation timeline while running benchmarks
   - Identify top 10 allocation hotspots (by function)
   - Detect objects retained after operation (memory leak indicators)
2. Create `/extension/test/profiling/leak-detector.js`
   - Run 1-hour session with 10k tasks
   - Perform periodic interactions (filter, expand, navigate)
   - Capture heap snapshots at 0, 30, 60 minutes
   - Analyze retained object count and size over time
   - Detect objects that should have been freed
3. Generate report:
   - Memory growth rate (/hour)
   - Retained objects by type
   - Suspected leaks (with object traces)

**Acceptance Criteria**:
- ✅ Allocation hotspots identified
- ✅ 1-hour session completes without crash
- ✅ Memory growth ≤10MB/hour (<10% baseline)
- ✅ No suspected memory leaks detected
- ✅ Report includes stack traces for investigation

**Dependencies**: Task 6 (baseline established)

**Estimated Hours**: 12

---

## Task 8: Rendering Optimization (Lazy Decoration)

**Description**: Optimize boundary decoration rendering by only decorating visible tasks.

**Sub-tasks**:
1. Modify `/extension/src/boundaries.js` TaskBoundaryDecorator class:
   - Add method: getVisibleRange(editor) - returns visible line range
   - Add method: filterVisibleTasks(tasks, editor) - tasks in visible range
   - Modify applyDecorations(): only decorate visible tasks
   - Add debouncing (batch updates every 100ms)
2. Test optimization:
   - Run task-panel-bench again (with lazy decoration)
   - Compare against baseline
   - Expected improvement: 40%+ faster rendering
3. Verify no regressions:
   - Scrolling still shows boundaries
   - Decorations update as editor scrolls
   - All existing tests still pass

**Acceptance Criteria**:
- ✅ Lazy decoration implemented
- ✅ Render time for 1k tasks: ≤180ms (was 300ms)
- ✅ Memory: no increase (same cache size)
- ✅ Scroll performance smooth (60 FPS)
- ✅ All unit tests pass

**Dependencies**: Tasks 2, 3, 6

**Estimated Hours**: 10

---

## Task 9: Memory Optimization (Object Pooling & Compression)

**Description**: Optimize memory usage of task objects and decoration cache.

**Sub-tasks**:
1. Implement object pooling in `/extension/test/profiling/ObjectPool.js`:
   - Reuse task objects across filter operations
   - Pool size: max 1000 objects
   - Factory: createTask(), returnTask()
2. Modify `/extension/src/taskpanel.js`:
   - Use ObjectPool for task instances
   - Lazy-load task properties (details on-demand)
   - Store only task IDs in tree until expanded
3. Modify `/extension/src/boundaries.js`:
   - Decoration type reuse (5-10 shared types by priority)
   - Cleanup decorations when tasks removed
4. Test optimization:
   - Run heap snapshot again (10k tasks)
   - Measure memory reduction
   - Expected: 30-40% reduction (150MB → 100MB)

**Acceptance Criteria**:
- ✅ Object pooling implemented and working
- ✅ 10k task memory: ≤110MB (target: ≤150MB met comfortably)
- ✅ Baseline heap: ≤50MB (unchanged)
- ✅ No functionality regression
- ✅ Pool efficiency >80% (reuse rate)

**Dependencies**: Task 7 (memory baseline)

**Estimated Hours**: 14

---

## Task 10: CPU Optimization (Debouncing & Filtering)

**Description**: Optimize CPU usage of filter/sort and event handling.

**Sub-tasks**:
1. Implement debouncing in `/extension/src/taskpanel.js`:
   - User input → wait 300ms → filter (not real-time)
   - Scroll events → throttle to max 1 per 50ms
   - Expand/collapse → batch DOM updates with RAF
2. Implement indexed filtering:
   - Pre-build task indexes (by priority, status)
   - Filter queries use index (O(1) lookup) instead of array scan (O(n))
3. Partial sorting:
   - Sort only top N results (pagination)
   - Avoid O(n log n) sort for full 10k list
4. Test optimization:
   - Run benchmark: filter 10k tasks
   - Expected: ≤100ms (was ~150ms)
   - CPU profile should show filter time dominated by index lookup

**Acceptance Criteria**:
- ✅ Debouncing implemented
- ✅ Filter 10k tasks: ≤100ms
- ✅ Scroll framerate: 60 FPS (no drops)
- ✅ CPU during filter: 15-25% (was 30-40%)
- ✅ All unit tests pass

**Dependencies**: Task 6 (baseline)

**Estimated Hours**: 12

---

## Task 11: Load Testing & Stability Validation

**Description**: Validate performance under sustained and concurrent load.

**Sub-tasks**:
1. Create `/extension/test/loadtest/sustained-load-test.js`:
   - Load 10k tasks
   - Run for 1 hour
   - Every 30 seconds: perform interaction (filter, expand, navigate)
   - Monitor: latency (should remain stable), memory (should not grow), GC (should not pile up)
2. Create `/extension/test/loadtest/concurrent-ops-test.js`:
   - Concurrent: load + expand + render boundaries
   - Measure interference (is operation A slower when B runs?)
   - Verify no deadlocks or race conditions
3. Create `/extension/test/loadtest/rapid-filter-test.js`:
   - User types quickly (5 filter changes/sec for 30 seconds)
   - Debouncing should prevent CPU spike
   - No missed filter results

**Acceptance Criteria**:
- ✅ 1-hour session completes without crash
- ✅ Latency stable: p95 at 60min ≤ 1.5x p95 at 5min
- ✅ Memory growth ≤10MB/hour
- ✅ GC frequency ≤2/sec at full load
- ✅ Concurrent ops show <10% interference
- ✅ Rapid filter completes all changes, no data loss

**Dependencies**: Tasks 8-10 (optimizations in place)

**Estimated Hours**: 12

---

## Task 12: Generate Performance Reports

**Description**: Create automated performance reports comparing results to targets and baseline.

**Sub-tasks**:
1. Create report generator in `/extension/test/profiling/generate-report.js`:
   - Input: benchmark results (JSON), baseline, targets
   - Generate HTML report with:
     - Latency table (all ops, all scales, vs. targets)
     - Memory graph (heap vs. task count)
     - Pass/fail status per target
   - Generate JSON report (for automated regression detection)
2. Compare optimization impact:
   - Before/after latency for each optimization (Task 8, 9, 10)
   - Total improvement vs. baseline
   - Identify which optimization had biggest impact
3. Output reports to `/docs/performance-reports/`:
   - `phase-4-latency-report.html`
   - `phase-4-memory-report.html`
   - `phase-4-regression-analysis.json`
   - `phase-4-optimization-summary.html`

**Acceptance Criteria**:
- ✅ All reports generated successfully
- ✅ Latency targets: all met (✅ = green, ❌ = red)
- ✅ Memory targets: all met
- ✅ Regression analysis: compare to v0.5.2
- ✅ Reports include optimization impact breakdown

**Dependencies**: Tasks 6-11 (all measurements complete)

**Estimated Hours**: 8

---

## Task 13: Profiling Guide & Documentation

**Description**: Create documentation for running benchmarks and interpreting results.

**Sub-tasks**:
1. Create `/docs/profiling-guide.md`:
   - How to run benchmarks: `npm run bench:all`
   - How to generate profiles: `npm run profile:cpu` and `npm run profile:memory`
   - How to interpret results
   - How to investigate regressions (spot a 15% slowdown)
   - Troubleshooting (GC pauses, out of memory, etc.)
2. Create `/docs/optimization-log.md`:
   - Summary of Task 8, 9, 10 optimizations
   - What changed and why
   - Performance impact of each change
   - Potential future optimizations (marked as TODOs)
3. Add npm scripts to package.json:
   - `npm run bench:all` - run all benchmarks
   - `npm run profile:cpu` - CPU profiling
   - `npm run profile:memory` - memory profiling
   - `npm run report` - generate reports

**Acceptance Criteria**:
- ✅ Guide is clear and actionable
- ✅ Guide includes examples (sample output)
- ✅ npm scripts work correctly
- ✅ Optimization log complete (each change documented)
- ✅ Future maintainers can run benchmarks without issue

**Dependencies**: Tasks 6-12 (all implementation done)

**Estimated Hours**: 6

---

## Task 14: Final Validation & QA

**Description**: Comprehensive validation that all performance targets are met and no regressions exist.

**Sub-tasks**:
1. Verify all latency targets met (from phase-4-requirements.md):
   - Task Panel Load (10k): ≤500ms ✅
   - Boundary Render (100): ≤300ms ✅
   - Call Path Expand (100): ≤200ms ✅
   - Task Management Open: ≤400ms ✅
   - Filter/Sort (10k): ≤100ms ✅
2. Verify all memory targets met:
   - Baseline heap: ≤50MB ✅
   - Load 3 peak (10k): ≤150MB ✅
   - Decoration cache: ≤30MB ✅
3. Verify no regressions vs. v0.5.2:
   - Compare latency: all operations ≤baseline+10%
   - Compare memory: baseline heap ≤v0.5.2+5MB
4. Run full test suite (npm run test):
   - All 34 unit tests from Phases 1-2 still pass
   - New profiling tests pass
5. Manual spot-check:
   - Load extension with real project (if available)
   - Perform typical operations
   - Verify responsiveness matches targets

**Acceptance Criteria**:
- ✅ All latency targets met
- ✅ All memory targets met
- ✅ No regressions vs. v0.5.2
- ✅ All unit tests pass
- ✅ Manual verification successful
- ✅ Ready for Phase 5 (Beta & User Testing)

**Dependencies**: Tasks 1-13

**Estimated Hours**: 10

---

## Summary

| Task | Title | Hours | Owner | Status |
|------|-------|-------|-------|--------|
| 1 | Set Up Performance Monitoring Infrastructure | 12 | Dev | Ready |
| 2 | Implement Task Panel Benchmarks | 8 | Dev | Ready |
| 3 | Implement Boundary Decoration Benchmarks | 8 | Dev | Ready |
| 4 | Implement Call Path Benchmarks | 8 | Dev | Ready |
| 5 | Implement Task Management Panel Benchmarks | 8 | Dev | Ready |
| 6 | Baseline Profiling (CPU & Memory) | 12 | Dev | Ready |
| 7 | Memory Profiling & Leak Detection | 12 | Dev | Ready |
| 8 | Rendering Optimization (Lazy Decoration) | 10 | Dev | Ready |
| 9 | Memory Optimization (Object Pooling) | 14 | Dev | Ready |
| 10 | CPU Optimization (Debouncing & Filtering) | 12 | Dev | Ready |
| 11 | Load Testing & Stability Validation | 12 | Dev | Ready |
| 12 | Generate Performance Reports | 8 | Dev | Ready |
| 13 | Profiling Guide & Documentation | 6 | Dev | Ready |
| 14 | Final Validation & QA | 10 | Dev | Ready |
| | **TOTAL** | **138 hours** | | |

**Execution Order**: Sequential (some parallelization possible)
- Week 1 (28h): Tasks 1-5 (infrastructure + baselines)
- Week 2 (38h): Tasks 6-9 (profiling + optimization)
- Week 3 (44h): Tasks 10-14 (CPU opt + testing + validation)

---

*End of Task Breakdown*