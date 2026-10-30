'use strict';
const crypto = require('node:crypto');
const path = require('node:path');
const SECURE_API_KEY_ENV = 'INTENTLENS_API_KEY';
const MAX_MODEL_RESPONSE_BYTES = 1024 * 1024;
const MAX_MODEL_PAGES = 50;
const MAX_MODELS = 5000;
const presets = [
  { label: 'OpenAI', provider: 'openai', base_url: 'https://api.openai.com/v1' },
  { label: 'Anthropic / Claude', provider: 'anthropic', base_url: 'https://api.anthropic.com/v1' },
  { label: 'Google Gemini', provider: 'gemini', base_url: 'https://generativelanguage.googleapis.com/v1beta' },
  { label: 'DeepSeek', provider: 'openai-compatible', base_url: 'https://api.deepseek.com/v1' },
  { label: '通义千问 / 百炼（北京，可修改地域）', provider: 'openai-compatible', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { label: '其他 OpenAI 兼容服务 / 本地服务', provider: 'openai-compatible', base_url: '' },
];
const verificationModes = [
  { label: '轻量复核（推荐）', value: 'light', description: '默认：两阶段；精简提示与复核上下文，低输出上限' },
  { label: '快速模式（仅小范围）', value: 'fast', description: '一次请求；不做独立复核，所有说法待确认；超出小范围自动升级严格复核' },
  { label: '严格复核', value: 'strict', description: '完整提示；两阶段逐条核对所有说法' },
];
function workspaceSecretKey(root) {
  const windowsPath = path.win32.isAbsolute(root);
  const normalized = (windowsPath ? path.win32.normalize(root) : path.resolve(root)).replace(/\\/g, '/');
  const identity = windowsPath || process.platform === 'win32' ? normalized.toLowerCase() : normalized;
  return `intentlens.model.api-key.v1.${crypto.createHash('sha256').update(identity).digest('hex').slice(0, 32)}`;
}
function validateBase(value) {
  try {
    const u = new URL(value);
    if (u.username || u.password || u.search || u.forceQuery || value.includes('?') || value.includes('#')) return '地址不能包含密钥、用户名、查询参数或片段';
    const local = u.hostname === 'localhost' || u.hostname === '[::1]' || /^127\./.test(u.hostname);
    if (u.protocol !== 'https:' && !(u.protocol === 'http:' && local)) return '远程服务使用 HTTPS；HTTP 仅限本机地址';
    if (/\/(chat\/completions|messages|models)$/.test(u.pathname.replace(/\/$/, ''))) return '请填写 API 基础地址，不包含 /models、/chat/completions 或 /messages';
    return null;
  } catch { return '请输入完整的 API 基础地址'; }
}
function modelListHeaders(provider, apiKey) {
  if (provider === 'anthropic') return { 'x-api-key': apiKey, 'anthropic-version': '2023-06-01', accept: 'application/json' };
  if (provider === 'gemini') return { 'x-goog-api-key': apiKey, accept: 'application/json' };
  if (provider === 'openai' || provider === 'openai-compatible') return { authorization: `Bearer ${apiKey}`, accept: 'application/json' };
  throw new Error('不支持该模型 provider 的模型列表协议');
}
async function responseJSON(response) {
  const declaredSize = Number(response.headers?.get?.('content-length') || 0);
  if (declaredSize > MAX_MODEL_RESPONSE_BYTES) throw new Error('模型列表响应超过 1 MiB');
  let raw;
  if (response.body?.getReader) {
    const reader = response.body.getReader();
    const chunks = [];
    let size = 0;
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > MAX_MODEL_RESPONSE_BYTES) {
        await reader.cancel().catch(() => {});
        throw new Error('模型列表响应超过 1 MiB');
      }
      chunks.push(Buffer.from(value));
    }
    raw = Buffer.concat(chunks).toString('utf8');
  } else {
    raw = await response.text();
    if (Buffer.byteLength(raw, 'utf8') > MAX_MODEL_RESPONSE_BYTES) throw new Error('模型列表响应超过 1 MiB');
  }
  try { return JSON.parse(raw); } catch { throw new Error('模型列表响应不是有效 JSON'); }
}
function unpackModels(provider, payload) {
  let entries;
  if (provider === 'gemini') entries = payload?.models;
  else entries = payload?.data;
  if (!Array.isArray(entries)) throw new Error('模型列表响应格式不受支持');
  const models = [];
  for (const entry of entries) {
    let id;
    if (provider === 'gemini') {
      const resourceName = typeof entry?.name === 'string' ? entry.name : '';
      const match = /^models\/([A-Za-z0-9][A-Za-z0-9._-]*)$/.exec(resourceName);
      if (!match) continue;
      if (Array.isArray(entry.supportedGenerationMethods) && !entry.supportedGenerationMethods.includes('generateContent')) continue;
      id = resourceName; // Preserve the exact resource ID returned by the provider.
    } else {
      id = typeof entry?.id === 'string' ? entry.id.trim() : '';
      if (!id || id.length > 256 || /[\r\n\0]/.test(id)) continue;
    }
    if (id && !models.some(model => model.id === id)) models.push({ id, displayName: typeof entry.displayName === 'string' ? entry.displayName : '' });
    if (models.length > MAX_MODELS) throw new Error('模型数量超过安全上限，未展示不完整列表');
  }
  return models;
}
function nextPage(provider, payload) {
  if (provider === 'gemini') return typeof payload?.nextPageToken === 'string' && payload.nextPageToken ? ['pageToken', payload.nextPageToken] : undefined;
  if (payload?.has_more !== true) return undefined;
  const cursor = typeof payload.last_id === 'string' ? payload.last_id : '';
  if (!cursor) throw new Error('模型列表声明仍有下一页，但未返回有效游标');
  return [provider === 'anthropic' ? 'after_id' : 'after', cursor];
}
async function listModels(provider, baseURL, apiKey, fetchImpl = globalThis.fetch) {
  if (!['openai', 'openai-compatible', 'anthropic', 'gemini'].includes(provider)) throw new Error('必须明确选择受支持的 provider');
  const baseError = validateBase(baseURL);
  if (baseError) throw new Error(baseError);
  if (typeof fetchImpl !== 'function') throw new Error('当前运行环境不支持安全的 HTTPS 模型列表请求');
  if (typeof apiKey !== 'string' || !apiKey.trim()) throw new Error('请输入 API Key 后才能查询模型列表');
  const endpoint = `${baseURL.trim().replace(/\/+$/, '')}/models`;
  const headers = modelListHeaders(provider, apiKey.trim());
  const models = [];
  const seenCursors = new Set();
  let cursor;
  for (let page = 0; page < MAX_MODEL_PAGES; page++) {
    const url = new URL(endpoint);
    if (provider === 'gemini') url.searchParams.set('pageSize', '1000');
    if (cursor) url.searchParams.set(cursor[0], cursor[1]);
    let response;
    try {
      response = await fetchImpl(url.toString(), {
        method: 'GET', headers, redirect: 'error', signal: AbortSignal.timeout(15000),
      });
    } catch {
      throw new Error('模型列表请求失败；未改用其他 provider 或地址');
    }
    if (!response?.ok) throw new Error(`GET /models 失败（HTTP ${Number(response?.status) || '未知'}）；未改用其他 provider 或地址`);
    const payload = await responseJSON(response);
    models.push(...unpackModels(provider, payload));
    if (models.length > MAX_MODELS) throw new Error('模型数量超过安全上限，未展示不完整列表');
    const following = nextPage(provider, payload);
    if (!following) return models;
    const cursorKey = `${following[0]}:${following[1]}`;
    if (seenCursors.has(cursorKey)) throw new Error('模型列表分页游标重复，未展示不完整列表');
    seenCursors.add(cursorKey);
    cursor = following;
  }
  throw new Error('模型列表页数超过安全上限，未展示不完整列表');
}
async function configureModel(vscode, folder, status, cli, secrets, fetchImpl = globalThis.fetch) {
  if (!secrets || typeof secrets.get !== 'function' || typeof secrets.store !== 'function') throw new Error('当前编辑器不支持安全密钥存储，未保存配置');
  const current = status.settings?.model || {};
  const options = current.model ? [{ label: '继续修改当前配置', ...current }, ...presets] : presets;
  const chosen = await vscode.window.showQuickPick(options, { title: '选择模型厂商或 API 协议', placeHolder: '必须查询此 provider 的实际模型列表；不会手动填写模型名称或切换服务' });
  if (!chosen) return false;
  if (!chosen.provider) throw new Error('必须明确选择模型 provider；不会使用默认或回退服务。');
  const base = await vscode.window.showInputBox({ title: 'API 基础地址（Base URL）', value: chosen.base_url || '', prompt: '填写服务文档中的 API 基础地址；远程必须 HTTPS。模型列表固定查询此地址下的 /models。', validateInput: validateBase });
  if (!base) return false;
  const normalizedBase = base.trim().replace(/\/+$/, '');
  const apiKey = await vscode.window.showInputBox({
    title: 'API Key（仅存入 VS Code 安全密钥存储）',
    prompt: `粘贴 ${chosen.provider} 的 API Key。查询前会再次确认；Key 只通过认证请求头发送，不会放入 URL。`,
    placeHolder: '必填；不会读取环境变量中的其他 Key，也不会回退到其他 provider。',
    password: true,
    ignoreFocusOut: true,
    validateInput: value => String(value || '').trim() ? null : '请输入 API Key；模型查询必须使用本次输入的 Key',
  });
  if (apiKey === undefined) return false;
  if (typeof apiKey !== 'string' || !apiKey.trim()) throw new Error('必须输入 API Key 才能查询模型列表；未保存配置');
  const query = await vscode.window.showWarningMessage(
    `将只向所选地址发送一次模型列表 GET 请求：\nProvider：${chosen.provider}\nEndpoint：${normalizedBase}/models\n\nAPI Key 仅放在该 provider 所需的认证请求头中，不进入 URL；不会发送项目代码、文件路径、用户问题或模型生成 prompt。服务方可能记录该连接请求。查询失败不会回退，也不会保存配置。`,
    { modal: true }, '查询此 API 的模型列表');
  if (query !== '查询此 API 的模型列表') return false;
  const models = await listModels(chosen.provider, normalizedBase, apiKey, fetchImpl);
  if (!models.length) throw new Error('所选 API 没有返回可选模型 ID；未保存配置');
  const choices = models.map(model => ({
    label: model.id,
    description: model.displayName && model.displayName !== model.id ? model.displayName : undefined,
    returnedModelId: model.id,
  }));
  const selection = await vscode.window.showQuickPick(choices, { title: '选择 API 返回的模型 ID', placeHolder: '只能从刚才查询到的模型中选择；不支持手动输入或跳过查询' });
  if (!selection) return false;
  const returnedIDs = new Set(models.map(model => model.id));
  if (!returnedIDs.has(selection.returnedModelId)) throw new Error('所选模型不在此 API 返回的模型列表中；未保存配置');
  const model = selection.returnedModelId;
  const limit = await vscode.window.showInputBox({ title: '项目累计 token 上限（切换不清空用量）', value: String(current.token_limit || 200000), validateInput: value => /^\d+$/.test(value) && Number.isSafeInteger(Number(value)) && Number(value) > 0 ? null : '输入正整数' });
  if (!limit) return false;
  const currentMode = current.verification_mode || 'light';
  const orderedModes = [...verificationModes].sort((a, b) => Number(b.value === currentMode) - Number(a.value === currentMode)).map(mode => mode.value === currentMode ? { ...mode, label: `${mode.label}（当前）` } : mode);
  const selectedMode = await vscode.window.showQuickPick(orderedModes, { title: '选择解释复核策略', placeHolder: 'light 为默认；可按任务选择 fast 或 strict' });
  if (!selectedMode) return false;
  const configuration = { enabled: true, provider: chosen.provider, base_url: normalizedBase, model, api_key_env: SECURE_API_KEY_ENV, token_limit: Number(limit), verification_mode: selectedMode.value };
  const modeDetails = {
    light: '轻量复核：两阶段独立检查，精简提示/复核上下文并降低输出上限。',
    fast: '快速模式：小范围仅发一次请求，不做独立复核，所有说法保持待确认；超出小范围时自动升级为严格双阶段复核。',
    strict: '严格复核：完整提示并在第二阶段逐条检查所有说法。',
  }[selectedMode.value];
  const answer = await vscode.window.showWarningMessage(`启用 ${folder.name} 的模型分析？接收地址：${configuration.base_url}；协议：${configuration.provider}；API 返回的模型 ID：${configuration.model}。复核策略：${selectedMode.label}。${modeDetails} 获准代码、轮次前后文本、问题会发送到该服务；练习反馈还发送答案与说明。Hook 可自动生成轮次简述。项目累计上限 ${limit} tokens，不是费用上限；当前实际 ${status.usage?.actual_tokens || 0}、未决 ${status.usage?.uncertain_tokens || 0}。Key 仅保存到 VS Code SecretStorage，不进入设置文件或 CLI 参数。`, { modal: true }, '同意并启用');
  if (answer !== '同意并启用') return false;
  const secretName = workspaceSecretKey(folder.uri.fsPath);
  const oldSecret = await secrets.get(secretName);
  await secrets.store(secretName, apiKey.trim());
  try {
    await cli(folder, 'configure', { settings: { ...status.settings, model: configuration }, consent: true });
  } catch (error) {
    try { if (oldSecret === undefined) await secrets.delete(secretName); else await secrets.store(secretName, oldSecret); } catch { /* Never expose credential recovery details in UI or logs. */ }
    throw error;
  }
  vscode.window.showInformationMessage('IntentLens：已按所选 provider 的实际模型列表配置并安全保存；尚未发送模型生成请求。');
  return true;
}
module.exports = { configureModel, presets, verificationModes, validateBase, listModels, workspaceSecretKey, SECURE_API_KEY_ENV };
