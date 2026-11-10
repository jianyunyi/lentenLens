#!/usr/bin/env node

/**
 * Boundary Bench Runner
 * Standalone script to run boundary decoration benchmarks
 */

const path = require('path');

async function main() {
  try {
    // Import benchmark module
    const boundaryBench = require('./benchmarks/boundary-bench');
    
    console.log(`\n=== ${boundaryBench.name} ===\n`);
    
    // Run the benchmarks
    const results = await boundaryBench.run();
    
    // Print summary
    console.log('\n=== Summary ===\n');
    console.log(JSON.stringify(results, null, 2));
    
    process.exit(results.all_passed ? 0 : 1);
  } catch (error) {
    console.error('Fatal error:', error);
    process.exit(1);
  }
}

main();
