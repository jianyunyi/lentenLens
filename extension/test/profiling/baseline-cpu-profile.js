/**
 * Baseline CPU Profiling
 * Runs all benchmarks and captures CPU profile data
 * Identifies hot functions and performance bottlenecks
 */

const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

const REPORTS_DIR = path.join(__dirname, '..', '..', '..', 'docs', 'performance-reports');

/**
 * Run benchmarks under CPU profiling
 */
async function runCPUProfile() {
  console.log('🔍 Running CPU profiling...\n');

  // Ensure reports directory exists
  if (!fs.existsSync(REPORTS_DIR)) {
    fs.mkdirSync(REPORTS_DIR, { recursive: true });
  }

  // Run benchmarks with --prof flag
  const benchmarkRunner = path.join(__dirname, 'run-benchmarks.js');
  
  return new Promise((resolve, reject) => {
    const proc = spawn('node', ['--prof', benchmarkRunner], {
      cwd: path.dirname(benchmarkRunner),
      stdio: 'inherit'
    });

    proc.on('close', (code) => {
      if (code === 0) {
        console.log('\n✓ CPU profiling complete');
        resolve();
      } else {
        reject(new Error(`Profiling failed with code ${code}`));
      }
    });

    proc.on('error', (err) => {
      reject(err);
    });
  });
}

/**
 * Analyze CPU profile output
 */
function analyzeCPUProfile() {
  console.log('\n📊 Analyzing CPU profile...\n');

  // Find isolate log files
  const cwd = process.cwd();
  const files = fs.readdirSync(cwd);
  const isolateLog = files.find(f => f.startsWith('isolate-') && f.endsWith('.log'));

  if (!isolateLog) {
    console.warn('⚠ No isolate log found. CPU profiling may not have run.');
    return { hotFunctions: [], totalSamples: 0 };
  }

  // Process the log file using node --prof-process
  const { execSync } = require('child_process');
  try {
    const output = execSync(`node --prof-process ${isolateLog}`, {
      encoding: 'utf8',
      maxBuffer: 10 * 1024 * 1024 // 10MB buffer
    });

    // Save raw output
    const rawOutputPath = path.join(REPORTS_DIR, 'cpu-profile-raw.txt');
    fs.writeFileSync(rawOutputPath, output);
    console.log(`✓ Raw CPU profile saved to: ${rawOutputPath}`);

    // Parse hot functions
    const hotFunctions = parseHotFunctions(output);

    // Clean up isolate log
    fs.unlinkSync(isolateLog);

    return { hotFunctions, totalSamples: hotFunctions.reduce((sum, f) => sum + f.samples, 0) };
  } catch (error) {
    console.error('Error processing CPU profile:', error.message);
    return { hotFunctions: [], totalSamples: 0 };
  }
}

/**
 * Parse hot functions from prof-process output
 */
function parseHotFunctions(output) {
  const lines = output.split('\n');
  const hotFunctions = [];
  
  // Look for the statistical profiling result section
  let inStatsSection = false;
  
  for (const line of lines) {
    if (line.includes('[Summary]:') || line.includes('Statistical profiling result')) {
      inStatsSection = true;
      continue;
    }

    if (inStatsSection && line.trim().match(/^\d+\s+\d+\.\d+%/)) {
      // Parse line like: "  123   45.6%   LazyCompile: *functionName"
      const match = line.trim().match(/^(\d+)\s+(\d+\.\d+)%\s+(.+)/);
      if (match) {
        const [, samples, percentage, name] = match;
        hotFunctions.push({
          name: name.trim(),
          samples: parseInt(samples, 10),
          percentage: parseFloat(percentage),
          timePerCall: 0 // Not available from summary
        });
      }
    }

    // Stop after collecting top functions
    if (hotFunctions.length >= 20) break;
  }

  return hotFunctions.slice(0, 10); // Top 10
}

/**
 * Generate CPU profile analysis report
 */
function generateCPUReport(cpuData) {
  console.log('\n📄 Generating CPU profile report...\n');

  const report = {
    timestamp: new Date().toISOString(),
    cpu: {
      hotFunctions: cpuData.hotFunctions.map(f => ({
        name: f.name,
        samples: f.samples,
        percentage: f.percentage,
        timePerCall: f.timePerCall
      })),
      totalSamples: cpuData.totalSamples
    }
  };

  const reportPath = path.join(REPORTS_DIR, 'cpu-profile-analysis.json');
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
  
  console.log(`✓ CPU analysis saved to: ${reportPath}\n`);

  // Print summary
  console.log('Top 5 Hot Functions:');
  console.log('='.repeat(70));
  for (let i = 0; i < Math.min(5, cpuData.hotFunctions.length); i++) {
    const fn = cpuData.hotFunctions[i];
    console.log(`${i + 1}. ${fn.name}`);
    console.log(`   Samples: ${fn.samples} (${fn.percentage.toFixed(2)}%)`);
  }
  console.log('='.repeat(70));

  return report;
}

/**
 * Main execution
 */
async function main() {
  try {
    console.log('🚀 Baseline CPU Profiling\n');
    console.log('This will run all benchmarks with CPU profiling enabled.\n');

    // Step 1: Run benchmarks with profiling
    await runCPUProfile();

    // Step 2: Analyze profile output
    const cpuData = analyzeCPUProfile();

    // Step 3: Generate report
    const report = generateCPUReport(cpuData);

    console.log('\n✅ CPU profiling complete!');
    process.exit(0);
  } catch (error) {
    console.error('\n❌ CPU profiling failed:', error);
    process.exit(1);
  }
}

// Run if executed directly
if (require.main === module) {
  main();
}

module.exports = { runCPUProfile, analyzeCPUProfile, generateCPUReport };