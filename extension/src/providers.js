'use strict';
const presets = [
  { label: 'OpenAI', provider: 'openai', base_url: 'https://api.openai.com/v1', api_key_env: 'OPENAI_API_KEY' },
  { label: 'Anthropic / Claude', provider: 'anthropic', base_url: 'https://api.anthropic.com/v1', api_key_env: 'ANTHROPIC_API_KEY' },
  { label: 'Google Gemini', provider: 'gemini', base_url: 'https://generativelanguage.googleapis.com/v1beta', api_key_env: 'GEMINI_API_KEY' },
  { label: 'DeepSeek', provider: 'openai-compatible', base_url: 'https://api.deepseek.com/v1', api_key_env: 'DEEPSEEK_API_KEY' },
  { label: '通义千问 / 百炼（北京，可修改地域）', provider: 'openai-compatible', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1', api_key_env: 'DASHSCOPE_API_KEY' },
  { label: '其他 OpenAI 兼容服务 / 本地服务', provider: 'openai-compatible', base_url: '', api_key_env: 'INTENTLENS_API_KEY' },
];
const verificationModes = [
  { label: '轻量复核（推荐）', value: 'light', description: '默认：两阶段；精简提示与复核上下文，低输出上限' },
  { label: '快速模式（仅小范围）', value: 'fast', description: '一次请求；不做独立复核，所有说法待确认；超出小范围自动升级严格复核' },
  { label: '严格复核', value: 'strict', description: '完整提示；两阶段逐条核对所有说法' },
];
function validateBase(value) {
  try {
    const u = new URL(value);
    if (u.username || u.password || u.search || value.includes('?') || value.includes('#')) return '地址不能包含密钥、用户名、查询参数或片段';
    const local = u.hostname === 'localhost' || u.hostname === '[::1]' || /^127\./.test(u.hostname);
    if (u.protocol !== 'https:' && !(u.protocol === 'http:' && local)) return '远程服务使用 HTTPS；HTTP 仅限本机地址';
    if (/\/(chat\/completions|messages)$/.test(u.pathname.replace(/\/$/, ''))) return '请填写基础地址，不包含 /chat/completions 或 /messages';
    return null;
  } catch { return '请输入完整的 API 基础地址'; }
}
async function configureModel(vscode, folder, status, cli) {
  const current = status.settings?.model || {};
  const options = current.model ? [{ label: '继续修改当前配置', ...current }, ...presets] : presets;
  const chosen = await vscode.window.showQuickPick(options, { title: '选择模型厂商或 API 协议', placeHolder: '用于解释、证据检查、共享生成和练习；与编码代理独立' });
  if (!chosen) return false;
  const base = await vscode.window.showInputBox({ title: 'API 基础地址（Base URL）', value: chosen.base_url || '', prompt: '使用厂商文档中的基础地址，不填写密钥。地域和账户需匹配。', validateInput: validateBase });
  if (!base) return false;
  const model = await vscode.window.showInputBox({ title: '模型 ID', value: chosen.model || '', prompt: '填写该账户可调用的模型名称；Gemini 不带 models/ 前缀。', validateInput: v => v.trim() ? null : '模型 ID 不能为空' });
  if (!model) return false;
  const keyEnv = await vscode.window.showInputBox({ title: '密钥环境变量名称', value: chosen.api_key_env || 'ANTHROPIC_API_KEY', prompt: '只填变量名称（例如 DEEPSEEK_API_KEY），不要填密钥本身。密钥需在启动 VS Code / Claude Code 的环境中设置。', validateInput: v => /^[A-Za-z_][A-Za-z0-9_]*$/.test(v) ? null : '请输入有效环境变量名称，不要输入密钥' });
  if (!keyEnv) return false;
  const limit = await vscode.window.showInputBox({ title: '项目累计 token 上限（切换不清空用量）', value: String(current.token_limit || 200000), validateInput: v => /^\d+$/.test(v) && Number.isSafeInteger(Number(v)) && Number(v) > 0 ? null : '输入正整数' });
  if (!limit) return false;
  const currentMode = current.verification_mode || 'light';
  const orderedModes = [...verificationModes].sort((a, b) => Number(b.value === currentMode) - Number(a.value === currentMode)).map(mode => mode.value === currentMode ? { ...mode, label: `${mode.label}（当前）` } : mode);
  const selectedMode = await vscode.window.showQuickPick(orderedModes, { title: '选择解释复核策略', placeHolder: 'light 为默认；可按任务选择 fast 或 strict' });
  if (!selectedMode) return false;
  const configuration = { enabled: true, provider: chosen.provider || 'anthropic', base_url: base.trim().replace(/\/+$/, ''), model: model.trim(), api_key_env: keyEnv, token_limit: Number(limit), verification_mode: selectedMode.value };
  const modeDetails = {
    light: '轻量复核：两阶段独立检查，精简提示/复核上下文并降低输出上限。',
    fast: '快速模式：小范围仅发一次请求，不做独立复核，所有说法保持待确认；超出小范围时自动升级为严格双阶段复核。',
    strict: '严格复核：完整提示并在第二阶段逐条检查所有说法。',
  }[selectedMode.value];
  const answer = await vscode.window.showWarningMessage(`启用 ${folder.name} 的模型分析？接收地址：${configuration.base_url}；协议：${configuration.provider}；模型：${configuration.model}。复核策略：${selectedMode.label}。${modeDetails} 获准代码、轮次前后文本、问题会发送到该服务；练习反馈还发送答案与说明。Hook 可自动生成轮次简述。项目累计上限 ${limit} tokens，不是费用上限；当前实际 ${status.usage?.actual_tokens || 0}、未决 ${status.usage?.uncertain_tokens || 0}。密钥只从 ${keyEnv} 读取；配置保存在本地，不进入共享。`, { modal: true }, '同意并启用');
  if (answer !== '同意并启用') return false;
  await cli(folder, 'configure', { settings: { ...status.settings, model: configuration }, consent: true });
  vscode.window.showInformationMessage('IntentLens：模型配置已保存。尚未验证密钥和模型可用性；主动生成解释时才发送请求。');
  return true;
}
module.exports = { configureModel, presets, verificationModes, validateBase };
