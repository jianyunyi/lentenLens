// Offline discovery only: never execute a discovered tool or inspect user projects.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

const ENTRIES = new Set(['ide', 'skill', 'claude']);
const ENV_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;

function fileExists(file) {
  try { return fs.statSync(file).isFile(); } catch { return false; }
}

function parseArgs(args) {
  const result = { entry: 'ide', model: false, keyEnv: 'ANTHROPIC_API_KEY', help: false };
  const seen = new Set();
  for (let i = 0; i < args.length; i += 1) {
    const arg = args[i];
    if (arg === '--help' || arg === '-h') {
      if (result.help) throw new Error('Duplicate --help');
      result.help = true;
    } else if (arg === '--model') {
      if (result.model) throw new Error('Duplicate --model');
      result.model = true;
    } else if (arg === '--entry' || arg === '--key-env') {
      const field = arg === '--entry' ? 'entry' : 'keyEnv';
      if (seen.has(arg)) throw new Error(`Duplicate ${arg}`);
      seen.add(arg);
      const value = args[++i];
      if (!value || value.startsWith('-')) throw new Error(`${arg} requires a value`);
      if (field === 'entry' && !ENTRIES.has(value)) throw new Error('Invalid --entry (expected ide, skill, or claude)');
      if (field === 'keyEnv' && !ENV_NAME.test(value)) throw new Error('Invalid environment variable name');
      result[field] = value;
    } else {
      throw new Error(`Unknown argument: ${arg}`);
    }
  }
  if (result.help && args.length !== 1) throw new Error('--help cannot be combined with other arguments');
  return result;
}

function collect(root, env = process.env, platform = process.platform, arch = process.arch, keyEnv = 'ANTHROPIC_API_KEY', options = {}) {
  if (!ENV_NAME.test(keyEnv)) throw new Error('Invalid environment variable name');
  const entry = options.entry || 'ide';
  const model = Boolean(options.model);
  if (!ENTRIES.has(entry)) throw new Error('Invalid entry');
  const get = name => env[Object.keys(env).find(key => platform === 'win32'
    ? key.toLowerCase() === name.toLowerCase() : key === name)] || '';
  const dirs = get('PATH').split(platform === 'win32' ? ';' : ':')
    .map(dir => dir.trim().replace(/^"(.*)"$/, '$1')).filter(dir => dir && path.isAbsolute(dir));
  function find(name) {
    const suffixes = platform === 'win32' ? ['.exe', '.cmd', '.bat'] : [''];
    for (const dir of dirs) for (const suffix of suffixes) {
      const candidate = path.join(dir, name + suffix);
      if (fileExists(candidate)) return candidate;
    }
    return null;
  }
  function artifact(relative) {
    const file = path.join(root, relative);
    if (!fileExists(file)) return null;
    return { path: file, sha256: crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') };
  }
  const manifest = JSON.parse(fs.readFileSync(path.join(root, 'extension/package.json'), 'utf8'));
  if (!/^\d+\.\d+\.\d+$/.test(manifest.version)) throw new Error('Unsupported package version');
  const tools = { claude: find('claude'), code: find('code'), go: find('go') };
  const home = get(platform === 'win32' ? 'USERPROFILE' : 'HOME');
  if (!tools.claude && home && path.isAbsolute(home)) {
    const native = path.join(home, '.local', 'bin', platform === 'win32' ? 'claude.exe' : 'claude');
    if (fileExists(native)) tools.claude = native;
  }
  const artifacts = {
    cli: artifact('bin/intentlens.exe'),
    bundled_cli: artifact('extension/bin/intentlens.exe'),
    vsix: artifact(`intentlens-${manifest.version}-win32-x64.vsix`),
  };
  const key_present = Boolean(get(keyEnv).trim());
  const bundled_cli_matches = artifacts.cli && artifacts.bundled_cli
    ? artifacts.cli.sha256 === artifacts.bundled_cli.sha256 : null;
  const missing = [];
  const required = {
    ide: ['code', 'cli', 'bundled_cli', 'vsix', 'matching_cli', 'windows_x64_package_platform'],
    skill: ['cli'],
    claude: ['claude', 'cli'],
  }[entry].slice();
  if (model) required.push('api_key');
  for (const name of required) {
    const present = name === 'api_key' ? key_present
      : name === 'matching_cli' ? bundled_cli_matches === true
      : name === 'windows_x64_package_platform' ? platform === 'win32' && arch === 'x64'
      : name === 'cli' || name === 'bundled_cli' || name === 'vsix' ? Boolean(artifacts[name])
      : Boolean(tools[name]);
    if (!present) missing.push(name);
  }
  return {
    schema: 1, version: manifest.version, platform, arch, entry, model,
    requirements: { entry, model, required }, tools, artifacts,
    key_env: keyEnv, key_present, bundled_cli_matches, missing,
    live_integration: 'NOT RUN',
    limitations: [
      'Paths are candidates only; versions, executability and login are unverified.',
      'Key presence does not verify credentials, model access or budget.',
      'Hashes identify local files; VSIX contents and installed extension are not verified here.',
      'No network calls, tool execution, project source reads or credential file reads.',
      'Report contains local paths; review before sharing.',
    ],
  };
}

const HELP = 'Usage: node scripts/preflight.cjs [--entry ide|skill|claude] [--model] [--key-env NAME]';

if (require.main === module) {
  try {
    const args = parseArgs(process.argv.slice(2));
    if (args.help) { process.stdout.write(`${HELP}\n`); process.exitCode = 0; }
    else {
      const report = collect(path.resolve(__dirname, '..'), process.env, process.platform, process.arch, args.keyEnv, args);
      process.stdout.write(JSON.stringify(report, null, 2) + '\n');
      process.exitCode = report.missing.length ? 2 : 0;
    }
  } catch (error) {
    process.stderr.write(`Preflight argument/error: ${error.message}\n${HELP}\n`);
    process.exitCode = 1;
  }
}
module.exports = { collect, parseArgs };
