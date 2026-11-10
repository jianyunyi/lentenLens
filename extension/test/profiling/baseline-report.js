/**
 * Baseline Report Generator
 * Aggregates CPU and memory profiling results into unified baseline
 */

const fs = require('fs');
const path = require('path');

const REPORTS_DIR = path.join(__dirname, '..', '..', '..', 'docs', 'performance-reports');

/**
 * Load existing profiling reports
 */
function loadProfilingReports() {
  const cpuReportPath = path.join(REPORTS_DIR, 'cpu-profile-analysis.json');
  const heapReportPath = path.join(REPORTS_DIR, 'heap-snapshots-analysis.json');

  let cpuData = null;
  let heapData = null;

  if (fs.existsSync(cpuReportPath)) {
    cpuData = JSON.parse(fs.readFileSync(cpuReportPath, 'utf8'));
    console.log('✓ Loaded CPU profile data');
  } else {
    console.warn('⚠ CPU profile not found. Run baseline-cpu-profile.js first.');
  }

  if (fs.existsSync(heapReportPath)) {
    heapData = JSON.parse(fs.readFileSync(heapReportPath, 'utf8'));
    console.log('✓ Loaded heap snapshot data');
  } else {
    console.warn('⚠ Heap snapshot not found. Run baseline-heap-snapshot.js first.');
  }

  return { cpuData, heapData };
}

/**
 * Generate unified baseline report
 */
function generateBaselineReport(cpuData, heapData) {
  console.log('\n📊 Generating unified baseline report...\n');

  const report = {
    timestamp: new Date().toISOString(),
    version: '0.6.0-baseline',
    baselines: {}
  };

  // Add CPU data
  if (cpuData) {
    report.baselines.cpu = {
      hotFunctions: cpuData.cpu.hotFunctions.slice(0, 5).map(f => ({
        name: f.name,
        samples: f.samples,
        percentage: f.percentage
      })),
      totalSamples: cpuData.cpu.totalSamples
    };
  }

  // Add memory data
  if (heapData) {
    report.baselines.memory = {
      scales: heapData.snapshots.map(s => ({
        scale: s.taskCount,
        heapUsedMB: s.heapUsedMB,
        topObjects: s.topObjects
      })),
      growthRate: heapData.summary.avgGrowthRatio,
      linearityCheck: heapData.summary.linearityCheck,
      baselineHeapMB: heapData.summary.baselineHeapMB,
      scale10kHeapMB: heapData.summary.scale10kHeapMB
    };
  }

  // Add GC metrics placeholder (would come from PerformanceMonitor)
  report.baselines.gc = {
    note: 'GC metrics collected via PerformanceMonitor during benchmark runs',
    avgPauseCount: 'See individual benchmark reports',
    avgPauseDuration: 'See individual benchmark reports'
  };

  // Save report
  const reportPath = path.join(REPORTS_DIR, 'baseline-profile.json');
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
  
  console.log(`✓ Baseline report saved to: ${reportPath}\n`);

  return report;
}

/**
 * Print baseline summary
 */
function printSummary(report) {
  console.log('╔═══════════════════════════════════════════════════════════════╗');
  console.log('║           PHASE 4 BASELINE PROFILING SUMMARY                 ║');
  console.log('╚═══════════════════════════════════════════════════════════════╝\n');

  // CPU Summary
  if (report.baselines.cpu) {
    console.log('📊 CPU Profile:');
    console.log('  Top 5 Hot Functions:');
    report.baselines.cpu.hotFunctions.forEach((fn, i) => {
      console.log(`    ${i + 1}. ${fn.name.substring(0, 50)}`);
      console.log(`       ${fn.percentage.toFixed(2)}% (${fn.samples} samples)`);
    });
    console.log(`  Total Samples: ${report.baselines.cpu.totalSamples}\n`);
  }

  // Memory Summary
  if (report.baselines.memory) {
    console.log('💾 Memory Profile:');
    console.log(`  Baseline Heap: ${report.baselines.memory.baselineHeapMB} MB`);
    console.log(`  10k Tasks Heap: ${report.baselines.memory.scale10kHeapMB} MB`);
    console.log(`  Growth Rate: ${report.baselines.memory.growthRate}x avg`);
    console.log(`  Linearity Check: ${report.baselines.memory.linearityCheck}\n`);
  }

  // Status
  const cpuOk = report.baselines.cpu !== undefined;
  const memOk = report.baselines.memory && report.baselines.memory.linearityCheck === 'PASS';
  
  console.log('Status:');
  console.log(`  CPU Profiling: ${cpuOk ? '✅ Complete' : '❌ Missing'}`);
  console.log(`  Memory Profiling: ${memOk ? '✅ Pass' : '⚠ Review Required'}`);
  
  console.log('\n' + '═'.repeat(65));
  console.log('Next Steps: Proceed to Task 7 (Memory Leak Detection)');
  console.log('═'.repeat(65) + '\n');
}

/**
 * Main execution
 */
async function main() {
  try {
    console.log('🚀 Baseline Report Generation\n');

    // Ensure reports directory exists
    if (!fs.existsSync(REPORTS_DIR)) {
      fs.mkdirSync(REPORTS_DIR, { recursive: true });
    }

    // Step 1: Load profiling data
    const { cpuData, heapData } = loadProfilingReports();

    if (!cpuData && !heapData) {
      console.error('\n❌ No profiling data found. Run CPU and heap profiling first.');
      process.exit(1);
    }

    // Step 2: Generate unified baseline
    const report = generateBaselineReport(cpuData, heapData);

    // Step 3: Print summary
    printSummary(report);

    console.log('✅ Baseline report generation complete!\n');
    process.exit(0);
  } catch (error) {
    console.error('\n❌ Baseline report generation failed:', error);
    process.exit(1);
  }
}

// Run if executed directly
if (require.main === module) {
  main();
}

module.exports = { generateBaselineReport, loadProfilingReports, printSummary };