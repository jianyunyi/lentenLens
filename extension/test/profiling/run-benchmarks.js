#!/usr/bin/env node

/**
 * Benchmark Runner - Discovers, loads, and executes all benchmarks
 * Usage: node run-benchmarks.js
 */

const fs = require('node:fs');
const path = require('node:path');

/**
 * Discover all benchmark files in the benchmarks directory
 * @returns {string[]} Array of benchmark file paths
 */
function discoverBenchmarks() {
  const benchmarksDir = path.join(__dirname, '..', 'benchmarks');
  
  if (!fs.existsSync(benchmarksDir)) {
    console.warn(`Benchmarks directory not found: ${benchmarksDir}`);
    return [];
  }

  const files = fs.readdirSync(benchmarksDir);
  return files
    .filter(file => file.endsWith('.js') && file !== 'run-benchmarks.js')
    .map(file => path.join(benchmarksDir, file));
}

/**
 * Load and validate a benchmark module
 * @param {string} filePath - Path to benchmark file
 * @returns {Object|null} Benchmark object or null if invalid
 */
function loadBenchmark(filePath) {
  try {
    const benchmark = require(filePath);
    
    if (!benchmark.name || typeof benchmark.name !== 'string') {
      console.error(`Invalid benchmark: ${filePath} - missing 'name' property`);
      return null;
    }
    
    if (!benchmark.run || typeof benchmark.run !== 'function') {
      console.error(`Invalid benchmark: ${filePath} - missing 'run' function`);
      return null;
    }

    return benchmark;
  } catch (error) {
    console.error(`Failed to load benchmark: ${filePath}`);
    console.error(error.message);
    return null;
  }
}

/**
 * Execute a single benchmark
 * @param {Object} benchmark - Benchmark object
 * @returns {Promise<Object>} Result from benchmark
 */
async function executeBenchmark(benchmark) {
  try {
    console.log(`\n▶ Running: ${benchmark.name}`);
    const result = await benchmark.run();
    console.log(`✓ Completed: ${benchmark.name}`);
    return result;
  } catch (error) {
    console.error(`✗ Failed: ${benchmark.name}`);
    console.error(error.message);
    return { error: error.message };
  }
}

/**
 * Format benchmark results as a table for console output
 * @param {Object[]} results - Array of benchmark results
 */
function printResultsTable(results) {
  console.log('\n' + '='.repeat(80));
  console.log('BENCHMARK RESULTS');
  console.log('='.repeat(80));

  // Table headers
  const headers = ['Operation', 'Iterations', 'p50 (ms)', 'p95 (ms)', 'p99 (ms)', 'Memory Δ (MB)'];
  console.log(headers.join('\t'));
  console.log('-'.repeat(80));

  // Table rows
  for (const result of results) {
    if (result.error) {
      console.log(`${result.name || 'Unknown'}\tERROR: ${result.error}`);
      continue;
    }

    const name = result.name || 'Unknown';
    const iterations = result.iterations || 0;
    const p50 = result.latency?.p50?.toFixed(2) || 'N/A';
    const p95 = result.latency?.p95?.toFixed(2) || 'N/A';
    const p99 = result.latency?.p99?.toFixed(2) || 'N/A';
    const memMB = (result.memoryDelta?.heapUsed || 0) / (1024 * 1024);

    console.log(`${name}\t${iterations}\t${p50}\t${p95}\t${p99}\t${memMB.toFixed(2)}`);
  }

  console.log('='.repeat(80));
}

/**
 * Aggregate results from multiple benchmarks
 * @param {Object[]} benchmarkResults - Raw results from each benchmark
 * @returns {Object} Aggregated results
 */
function aggregateResults(benchmarkResults) {
  const aggregated = {
    timestamp: new Date().toISOString(),
    totalBenchmarks: benchmarkResults.length,
    successCount: 0,
    failureCount: 0,
    results: []
  };

  for (const result of benchmarkResults) {
    if (result.error) {
      aggregated.failureCount++;
      aggregated.results.push({
        name: result.name || 'Unknown',
        error: result.error
      });
    } else {
      aggregated.successCount++;
      aggregated.results.push(result);
    }
  }

  return aggregated;
}

/**
 * Main runner function
 */
async function main() {
  console.log('🏃 IntentLens Benchmark Runner');
  console.log('==============================\n');

  // Discover benchmarks
  const benchmarkFiles = discoverBenchmarks();
  
  if (benchmarkFiles.length === 0) {
    console.log('No benchmarks found in extension/test/benchmarks/');
    process.exit(0);
  }

  console.log(`Found ${benchmarkFiles.length} benchmark(s)\n`);

  // Load benchmarks
  const benchmarks = [];
  for (const file of benchmarkFiles) {
    const benchmark = loadBenchmark(file);
    if (benchmark) {
      benchmarks.push(benchmark);
    }
  }

  if (benchmarks.length === 0) {
    console.error('No valid benchmarks could be loaded');
    process.exit(1);
  }

  // Execute benchmarks sequentially
  const results = [];
  for (const benchmark of benchmarks) {
    const result = await executeBenchmark(benchmark);
    results.push(result);
  }

  // Aggregate and display results
  const aggregated = aggregateResults(results);
  printResultsTable(results);

  // Output JSON report
  const reportPath = path.join(__dirname, '..', '..', '..', 'docs', 'performance-reports', 'baseline-benchmarks.json');
  const reportDir = path.dirname(reportPath);

  try {
    if (!fs.existsSync(reportDir)) {
      fs.mkdirSync(reportDir, { recursive: true });
    }

    fs.writeFileSync(reportPath, JSON.stringify(aggregated, null, 2));
    console.log(`\n📊 Results saved to: ${reportPath}`);
  } catch (error) {
    console.error(`Failed to write report: ${error.message}`);
    process.exit(1);
  }

  // Exit with appropriate code
  const exitCode = aggregated.failureCount > 0 ? 1 : 0;
  process.exit(exitCode);
}

// Run the benchmark suite
main().catch(error => {
  console.error('Benchmark runner failed:', error);
  process.exit(1);
});
