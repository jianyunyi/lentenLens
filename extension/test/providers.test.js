const { test } = require('node:test');
const assert = require('node:assert/strict');
const { configureModel, presets, verificationModes, validateBase, listModels, workspaceSecretKey, SECURE_API_KEY_ENV } = require('../src/providers');

function jsonResponse(value, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { 'content-type': 'application/json' } });
}

function wizard({ query = true, accept = true, key = 'fixture-api-key', selectedModel, previousKey, configureError, onStore } = {}) {
  const baseURL = 'https://api.fixture/v1';
  const inputs = [], picks = [], warnings = [], calls = [], requests = [];
  const secrets = new Map();
  const secretName = workspaceSecretKey('/fixture/project');
  if (previousKey !== undefined) secrets.set(secretName, previousKey);
  const selectedProvider = presets.find(item => item.provider === 'openai-compatible' && !item.base_url);
  const vscode = { window: {
    showQuickPick: async (items, options = {}) => {
      picks.push({ items, options });
      if (options.title.includes('厂商')) return selectedProvider;
      if (options.title.includes('API 返回')) return selectedModel === 'forged' ? { returnedModelId: 'not-returned' } : items.find(item => item.returnedModelId === (selectedModel || 'model-from-api'));
      return items.find(item => item.value === 'light');
    },
    showInputBox: async options => {
      inputs.push(options);
      if (options.title.includes('基础地址')) return baseURL;
      if (options.title.includes('API Key')) return key;
      if (options.title.includes('token 上限')) return '4096';
      throw new Error(`unexpected input ${options.title}`);
    },
    showWarningMessage: async (text, _options, action) => {
      warnings.push({ text, action });
      if (action === '查询此 API 的模型列表') return query ? action : undefined;
      return accept ? '同意并启用' : 'cancel';
    },
    showInformationMessage() {},
  } };
  const secure = {
    get: async name => secrets.get(name),
    store: async (name, value) => {
      assert.equal(calls.at(-1)?.command, 'configure', 'configuration must be saved before storing a key');
      secrets.set(name, value);
      await onStore?.({ secrets, name, value, saved });
    },
    delete: async name => secrets.delete(name),
  };
  const cli = async (_folder, command, request) => {
    if (command === 'status') return { settings: structuredClone(saved.settings) };
    calls.push({ command, request });
    if (configureError) throw configureError;
    saved.settings = structuredClone(request.settings);
    return { saved: true };
  };
  const fetchImpl = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse({ data: [{ id: 'model-from-api' }, { id: 'another-returned-model' }] });
  };
  const folder = { name: 'fixture', uri: { fsPath: '/fixture/project' } };
  const status = { settings: { model: { token_limit: 200000 } }, usage: { actual_tokens: 42, uncertain_tokens: 10 } };
  const saved = { settings: structuredClone(status.settings) };
  return { vscode, secure, cli, fetchImpl, folder, status, baseURL, inputs, picks, warnings, calls, requests, secrets, secretName, saved };
}

test('OpenAI-compatible model discovery uses the selected /models endpoint and API key only in Authorization header', async () => {
  let request;
  const models = await listModels('openai-compatible', 'https://api.fixture/v1', 'fixture-secret', async (url, options) => {
    request = { url, options };
    return jsonResponse({ data: [{ id: 'model-a' }, { id: 'model-b' }] });
  });
  assert.deepEqual(models.map(model => model.id), ['model-a', 'model-b']);
  assert.equal(request.url, 'https://api.fixture/v1/models');
  assert.equal(request.options.method, 'GET');
  assert.equal(request.options.redirect, 'error');
  assert.equal(request.options.headers.authorization, 'Bearer fixture-secret');
  assert.ok(!request.url.includes('fixture-secret'));
  assert.ok(!new URL(request.url).search);
});

test('provider-specific auth and pagination return actual Anthropic and Gemini IDs', async () => {
  const anthropicRequests = [];
  const anthropic = await listModels('anthropic', 'https://api.fixture/v1', 'anthropic-key', async (url, options) => {
    anthropicRequests.push({ url: new URL(url), options });
    if (!new URL(url).searchParams.has('after_id')) return jsonResponse({ data: [{ id: 'claude-a' }], has_more: true, last_id: 'claude-a' });
    return jsonResponse({ data: [{ id: 'claude-b' }], has_more: false });
  });
  assert.deepEqual(anthropic.map(model => model.id), ['claude-a', 'claude-b']);
  assert.equal(anthropicRequests.length, 2);
  assert.equal(anthropicRequests[0].options.headers['x-api-key'], 'anthropic-key');
  assert.equal(anthropicRequests[0].options.headers['anthropic-version'], '2023-06-01');
  assert.equal(anthropicRequests[1].url.searchParams.get('after_id'), 'claude-a');
  assert.ok(!anthropicRequests[1].url.toString().includes('anthropic-key'));

  const geminiRequests = [];
  const gemini = await listModels('gemini', 'https://api.fixture/v1beta', 'gemini-key', async (url, options) => {
    const parsed = new URL(url);
    geminiRequests.push({ url: parsed, options });
    if (!parsed.searchParams.has('pageToken')) return jsonResponse({ models: [
      { name: 'models/gemini-flash', displayName: 'Gemini Flash', supportedGenerationMethods: ['generateContent'] },
      { name: 'models/gemini-embed', supportedGenerationMethods: ['embedContent'] },
    ], nextPageToken: 'page-2' });
    return jsonResponse({ models: [{ name: 'models/gemini-pro' }] });
  });
  assert.deepEqual(gemini.map(model => model.id), ['models/gemini-flash', 'models/gemini-pro']);
  assert.equal(geminiRequests.length, 2);
  assert.equal(geminiRequests[0].options.headers['x-goog-api-key'], 'gemini-key');
  assert.equal(geminiRequests[1].url.searchParams.get('pageToken'), 'page-2');
  assert.ok(!geminiRequests[0].url.toString().includes('gemini-key'));
});

test('model pagination deduplicates before applying the limit and retains first occurrence order', async () => {
  const firstPage = Array.from({ length: 4999 }, (_, i) => ({ id: `model-${i}`, displayName: 'first' }));
  const overlap = [{ id: 'model-0', displayName: 'second' }, { id: 'last' }];
  const fetchPages = extra => async url => new URL(url).searchParams.has('after')
    ? jsonResponse({ data: [...overlap, ...extra] })
    : jsonResponse({ data: firstPage, has_more: true, last_id: 'model-4998' });
  const models = await listModels('openai', 'https://api.fixture/v1', 'key', fetchPages([]));
  assert.deepEqual(models.map(model => model.id), [...firstPage.map(model => model.id), 'last']);
  assert.equal(models[0].displayName, 'first');
  await assert.rejects(listModels('openai', 'https://api.fixture/v1', 'key', fetchPages([{ id: 'overflow' }])), /模型数量超过安全上限/);
});

for (const { status, headers, error } of [
  { status: 401, error: /HTTP 401/ },
  { status: 200, headers: { 'content-length': String(1024 * 1024 + 1) }, error: /响应超过 1 MiB/ },
]) {
  test(`model discovery cancels the response body on early rejection (HTTP ${status})`, async () => {
    let cancelled = false;
    const body = new ReadableStream({
      cancel() { cancelled = true; throw new Error('fixture cancellation failed'); },
    });
    const fetchImpl = async () => new Response(body, { status, headers });
    await assert.rejects(listModels('openai', 'https://api.fixture/v1', 'key', fetchImpl), error);
    assert.equal(cancelled, true);
  });
}

test('wizard queries first, lets the user select only a returned ID, and stores the key outside settings', async () => {
  const h = wizard();
  assert.equal(await configureModel(h.vscode, h.folder, h.status, h.cli, h.secure, h.fetchImpl), true);
  assert.equal(h.requests.length, 1);
  assert.equal(h.requests[0].url, `${h.baseURL}/models`);
  assert.equal(h.requests[0].options.headers.authorization, 'Bearer fixture-api-key');
  assert.equal(h.calls.length, 1);
  assert.equal(h.calls[0].command, 'configure');
  const cfg = h.calls[0].request.settings.model;
  assert.deepEqual({ provider: cfg.provider, base_url: cfg.base_url, model: cfg.model, api_key_env: cfg.api_key_env }, {
    provider: 'openai-compatible', base_url: h.baseURL, model: 'model-from-api', api_key_env: SECURE_API_KEY_ENV,
  });
  assert.equal(cfg.enabled, true);
  assert.equal(cfg.token_limit, 4096);
  assert.equal(cfg.verification_mode, 'light');
  assert.equal('api_key' in cfg, false);
  assert.ok(h.inputs.some(options => options.password === true));
  assert.ok(!h.inputs.some(options => options.title === '模型 ID'));
  assert.deepEqual(h.picks.find(pick => pick.options.title.includes('API 返回')).items.map(item => item.returnedModelId), ['model-from-api', 'another-returned-model']);
  assert.equal(h.secrets.get(h.secretName), 'fixture-api-key');
  assert.ok(h.warnings.some(item => item.action === '查询此 API 的模型列表' && item.text.includes('/models')));
  assert.ok(!JSON.stringify(h.calls).includes('fixture-api-key'));
  assert.ok(!h.warnings.some(item => item.text.includes('fixture-api-key')));
});

test('query consent, API errors, empty results, forged selections, and final consent all fail closed', async () => {
  const denied = wizard({ query: false });
  assert.equal(await configureModel(denied.vscode, denied.folder, denied.status, denied.cli, denied.secure, denied.fetchImpl), false);
  assert.equal(denied.requests.length, 0);
  assert.equal(denied.calls.length, 0);
  assert.equal(denied.secrets.has(denied.secretName), false);

  const failed = wizard();
  failed.fetchImpl = async () => jsonResponse({ error: 'secret response body must not be shown' }, 401);
  await assert.rejects(() => configureModel(failed.vscode, failed.folder, failed.status, failed.cli, failed.secure, failed.fetchImpl), /HTTP 401/);
  assert.equal(failed.calls.length, 0);
  assert.equal(failed.secrets.has(failed.secretName), false);

  const empty = wizard();
  empty.fetchImpl = async () => jsonResponse({ data: [] });
  await assert.rejects(() => configureModel(empty.vscode, empty.folder, empty.status, empty.cli, empty.secure, empty.fetchImpl), /没有返回可选模型/);
  assert.equal(empty.calls.length, 0);
  assert.equal(empty.secrets.has(empty.secretName), false);

  const forged = wizard({ selectedModel: 'forged' });
  await assert.rejects(() => configureModel(forged.vscode, forged.folder, forged.status, forged.cli, forged.secure, forged.fetchImpl), /不在此 API 返回/);
  assert.equal(forged.calls.length, 0);
  assert.equal(forged.secrets.has(forged.secretName), false);

  const declined = wizard({ accept: false });
  assert.equal(await configureModel(declined.vscode, declined.folder, declined.status, declined.cli, declined.secure, declined.fetchImpl), false);
  assert.equal(declined.calls.length, 0);
  assert.equal(declined.secrets.has(declined.secretName), false);
});

test('a previously saved key cannot replace wizard input and a failed CLI save leaves the key alone', async () => {
  const noKey = wizard({ key: '', previousKey: 'old-key' });
  await assert.rejects(() => configureModel(noKey.vscode, noKey.folder, noKey.status, noKey.cli, noKey.secure, noKey.fetchImpl), /必须输入 API Key/);
  assert.equal(noKey.requests.length, 0);
  assert.equal(noKey.secrets.get(noKey.secretName), 'old-key');

  const failedSave = wizard({ previousKey: 'old-key', configureError: new Error('fixture config write failed') });
  await assert.rejects(() => configureModel(failedSave.vscode, failedSave.folder, failedSave.status, failedSave.cli, failedSave.secure, failedSave.fetchImpl), /fixture config write failed/);
  assert.equal(failedSave.secrets.get(failedSave.secretName), 'old-key');
  assert.equal(failedSave.calls.length, 1);
  assert.ok(!failedSave.warnings.some(item => item.text.includes('old-key')));
});

for (const concurrentChange of ['neither value', 'configuration', 'key']) {
  test(`wizard guards rollback after a rejected key write when another writer changes ${concurrentChange}`, async () => {
    const h = wizard({
      previousKey: 'old-key',
      onStore: ({ secrets, name, value, saved }) => {
        if (value !== 'fixture-api-key') return;
        if (concurrentChange === 'configuration') saved.settings.model.model = 'other-model';
        if (concurrentChange !== 'neither value') secrets.set(name, 'other-key');
        throw new Error('fixture key write failed');
      },
    });
    // The UI status was fetched earlier. Rollback must restore the config
    // present just before saving, including any edits made during the wizard.
    h.saved.settings.model.token_limit = 123456;
    const previous = structuredClone(h.saved.settings);
    await assert.rejects(configureModel(h.vscode, h.folder, h.status, h.cli, h.secure, h.fetchImpl), /fixture key write failed/);
    if (concurrentChange === 'configuration') {
      assert.equal(h.saved.settings.model.model, 'other-model');
      assert.equal(h.calls.length, 1);
    } else {
      assert.deepEqual(h.saved.settings, previous);
      assert.equal(h.calls.length, 2);
    }
    assert.equal(h.secrets.get(h.secretName), concurrentChange === 'neither value' ? 'old-key' : 'other-key');
  });
}

test('concurrent wizard saves wait for the preceding rollback before writing their key', async () => {
  let entered, release;
  const writing = new Promise(resolve => { entered = resolve; });
  const resume = new Promise(resolve => { release = resolve; });
  const h = wizard({ previousKey: 'old-key', onStore: async ({ value }) => {
    if (value !== 'fixture-api-key') return;
    entered();
    await resume;
    throw new Error('first save failed');
  } });
  const first = assert.rejects(configureModel(h.vscode, h.folder, h.status, h.cli, h.secure, h.fetchImpl), /first save failed/);
  await writing;
  const secondWizard = wizard({ key: 'second-key' });
  const second = configureModel(secondWizard.vscode, h.folder, h.status, h.cli, h.secure, h.fetchImpl);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(h.calls.length, 1);
  release();
  await first;
  assert.equal(await second, true);
  assert.equal(h.secrets.get(h.secretName), 'second-key');
  assert.equal(h.calls.length, 3);
});

test('secure storage is mandatory and base URLs reject credentials, queries, model endpoints and remote HTTP', async () => {
  const h = wizard();
  await assert.rejects(() => configureModel(h.vscode, h.folder, h.status, h.cli, undefined, h.fetchImpl), /安全密钥存储/);
  assert.equal(h.requests.length, 0);
  for (const value of ['http://remote.test/v1', 'https://a:b@host/v1', 'https://host/v1?key=abc', 'https://host/v1#x', 'https://host/v1/models']) assert.ok(validateBase(value));
  for (const value of ['https://host/v1', 'http://127.0.0.1:8080/v1', 'http://[::1]:8080/v1']) assert.equal(validateBase(value), null);
  for (const preset of presets) assert.ok(preset.provider);
  assert.deepEqual(verificationModes.map(mode => mode.value), ['light', 'fast', 'strict']);
});
