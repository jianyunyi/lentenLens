'use strict';
// Uses disposable synthetic workspaces only. Measurements are environment-specific,
// never a team trial or an SLA, and contain neither user source nor model keys.
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const crypto = require('node:crypto');
const {run} = require('./soak.cjs');
const iterations = Number(process.argv[2] || '1000');
if (process.argv.length > 3 || !Number.isInteger(iterations) || iterations < 1 || iterations > 10000) {
  console.error('Usage: node scripts/history-acceptance.cjs [iterations:1..10000]');
  process.exit(1);
}
const output = path.resolve('history-acceptance');
fs.mkdirSync(output, {recursive: true});
const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-history-'));
function command(exe, args, timeout = 300000) {
  const result = spawnSync(exe, args, {encoding:'utf8', shell:false, timeout, maxBuffer:8*1024*1024});
  if (result.error || result.status !== 0) throw new Error(`${exe} failed: ${result.error?.message || result.stderr || result.stdout}`);
  return result.stdout;
}
try {
  const sourcePaths=command('git',['ls-files','-co','--exclude-standard']).trim().split('\n')
    .filter(p=>p.endsWith('.go') || p==='go.mod' || ['scripts/soak.cjs','scripts/history-acceptance.cjs'].includes(p)).sort();
  const sourceHash=crypto.createHash('sha256');
  for(const file of sourcePaths) {sourceHash.update(file+'\0');sourceHash.update(fs.readFileSync(file));sourceHash.update('\0');}
  const environment = {
    sourceSha256:sourceHash.digest('hex'),
    dirty:command('git',['status','--porcelain']).trim() !== '',
    commit:command('git',['rev-parse','HEAD']).trim(),
    go:command('go',['version']).trim(),
    platform:process.platform, arch:process.arch, osRelease:os.release(),
    cpu:os.cpus()[0]?.model, logicalCpus:os.cpus().length,
    node:process.version, iterations,
    limits:'Synthetic fixture; shared CI measurements are not target-machine or real-team acceptance.',
  };
  fs.writeFileSync(path.join(output,'environment.json'), JSON.stringify(environment,null,2)+'\n');
  const benchmark=command('go',['test','./internal/app','-run','^$','-bench','BenchmarkRoundHistoryJoin','-benchtime=1x','-benchmem','-count=3']);
  fs.writeFileSync(path.join(output,'benchmark.txt'),benchmark);
  const binary=path.join(temporary,process.platform==='win32'?'intentlens.exe':'intentlens');
  command('go',['build','-trimpath','-o',binary,'./cmd/intentlens']);
  const report=run(binary,iterations);
  fs.writeFileSync(path.join(output,'soak.json'),JSON.stringify(report,null,2)+'\n');
  console.log(JSON.stringify({result:report.result,iterations,output,durationMs:report.durationMs,roundsP95Ms:report.latency.rounds.p95Ms}));
} catch(error) {
  console.error(error.message);process.exitCode=1;
} finally { fs.rmSync(temporary,{recursive:true,force:true}); }
