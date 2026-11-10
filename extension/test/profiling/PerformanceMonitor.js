/**
 * PerformanceMonitor - Records and analyzes operation performance metrics.
 * Measures latency, memory delta, and GC pauses for benchmarking.
 */

const { performance, PerformanceObserver } = require('node:perf_hooks');

class PerformanceMonitor {
  constructor() {
    // Store samples: { name -> [{ latency, memory, gcEvents }] }
    this.samples = new Map();
    
    // Store active measurements
    this.active = new Map();
    
    // GC observer for tracking GC pauses
    this.gcObserver = null;
    this.gcEvents = [];
    
    this._initGCObserver();
  }

  /**
   * Initialize GC observer to track garbage collection pauses
   * @private
   */
  _initGCObserver() {
    try {
      this.gcObserver = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (entry.kind !== undefined) {
            // entry.kind: 1=Scavenge, 2=MarkSweepCompact, 4=IncrementalMarking
            this.gcEvents.push({
              type: entry.kind,
              duration: entry.duration,
              startTime: entry.startTime
            });
          }
        }
      });
      
      // Observe PerformanceObserver with 'gc' entry type
      this.gcObserver.observe({ entryTypes: ['gc'], buffered: true });
    } catch (e) {
      // GC observation not available, continue without it
      this.gcObserver = null;
    }
  }

  /**
   * Start measuring an operation
   * @param {string} name - Operation name
   */
  start(name) {
    if (this.active.has(name)) {
      throw new Error(`Measurement '${name}' is already active`);
    }

    const memBefore = process.memoryUsage();
    const startTime = performance.now();
    const gcCountBefore = this.gcEvents.length;

    this.active.set(name, {
      startTime,
      memBefore,
      gcCountBefore
    });
  }

  /**
   * End measuring an operation and record the sample
   * @param {string} name - Operation name
   */
  end(name) {
    if (!this.active.has(name)) {
      throw new Error(`Measurement '${name}' is not active`);
    }

    const endTime = performance.now();
    const memAfter = process.memoryUsage();
    const gcCountAfter = this.gcEvents.length;

    const measurement = this.active.get(name);
    this.active.delete(name);

    const latency = endTime - measurement.startTime;
    
    // Calculate memory delta in bytes
    const memoryDelta = {
      heapUsed: memAfter.heapUsed - measurement.memBefore.heapUsed,
      heapTotal: memAfter.heapTotal - measurement.memBefore.heapTotal,
      external: memAfter.external - measurement.memBefore.external,
      rss: memAfter.rss - measurement.memBefore.rss
    };

    // Calculate GC metrics for this operation
    const operationGCEvents = this.gcEvents.slice(measurement.gcCountBefore, gcCountAfter);
    const gcMetrics = {
      count: operationGCEvents.length,
      totalDuration: operationGCEvents.reduce((sum, e) => sum + e.duration, 0)
    };

    if (!this.samples.has(name)) {
      this.samples.set(name, []);
    }

    this.samples.get(name).push({
      latency,
      memoryDelta,
      gcMetrics
    });
  }

  /**
   * Reset all samples for a given operation (or all if not specified)
   * @param {string} [name] - Operation name to reset, or undefined for all
   */
  reset(name) {
    if (name !== undefined) {
      this.samples.delete(name);
    } else {
      this.samples.clear();
    }
    this.gcEvents = [];
  }

  /**
   * Calculate percentiles from sorted array of values
   * @param {number[]} sorted - Sorted array of values
   * @param {number} percentile - Percentile (0-100)
   * @returns {number} Percentile value
   * @private
   */
  _percentile(sorted, percentile) {
    if (sorted.length === 0) return 0;
    const index = (percentile / 100) * (sorted.length - 1);
    const lower = Math.floor(index);
    const upper = Math.ceil(index);
    const weight = index % 1;

    if (lower === upper) {
      return sorted[lower];
    }
    return sorted[lower] * (1 - weight) + sorted[upper] * weight;
  }

  /**
   * Generate a performance report for an operation
   * @param {string} name - Operation name
   * @returns {Object} Report object with latency stats, memory, and GC metrics
   */
  report(name) {
    if (!this.samples.has(name) || this.samples.get(name).length === 0) {
      return {
        name,
        iterations: 0,
        latency: { p50: 0, p95: 0, p99: 0, min: 0, max: 0 },
        memoryDelta: { heapUsed: 0, heapTotal: 0, external: 0, rss: 0 },
        gcCount: 0,
        gcTotalTime: 0
      };
    }

    const samples = this.samples.get(name);
    const latencies = samples.map(s => s.latency).sort((a, b) => a - b);
    
    // Calculate latency percentiles
    const report = {
      name,
      iterations: samples.length,
      latency: {
        p50: this._percentile(latencies, 50),
        p95: this._percentile(latencies, 95),
        p99: this._percentile(latencies, 99),
        min: latencies[0],
        max: latencies[latencies.length - 1],
        mean: latencies.reduce((a, b) => a + b, 0) / latencies.length
      },
      memoryDelta: {
        heapUsed: samples.reduce((sum, s) => sum + s.memoryDelta.heapUsed, 0) / samples.length,
        heapTotal: samples.reduce((sum, s) => sum + s.memoryDelta.heapTotal, 0) / samples.length,
        external: samples.reduce((sum, s) => sum + s.memoryDelta.external, 0) / samples.length,
        rss: samples.reduce((sum, s) => sum + s.memoryDelta.rss, 0) / samples.length
      },
      gcCount: samples.reduce((sum, s) => sum + s.gcMetrics.count, 0),
      gcTotalTime: samples.reduce((sum, s) => sum + s.gcMetrics.totalDuration, 0)
    };

    return report;
  }

  /**
   * Get all reports for all measured operations
   * @returns {Object[]} Array of report objects
   */
  reportAll() {
    const reports = [];
    for (const name of this.samples.keys()) {
      reports.push(this.report(name));
    }
    return reports;
  }

  /**
   * Get raw samples for an operation (for advanced analysis)
   * @param {string} name - Operation name
   * @returns {Object[]} Array of sample objects
   */
  getSamples(name) {
    return this.samples.get(name) || [];
  }

  /**
   * Cleanup resources
   */
  destroy() {
    if (this.gcObserver) {
      this.gcObserver.disconnect();
      this.gcObserver = null;
    }
    this.samples.clear();
    this.active.clear();
    this.gcEvents = [];
  }
}

module.exports = PerformanceMonitor;
