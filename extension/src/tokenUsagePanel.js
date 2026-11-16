// TokenUsagePanel displays the project usage reported by the CLI status command.
const vscode = require('vscode');

class TokenUsagePanel {
    static currentPanel = undefined;

    static show(context, loadStatus) {
        const column = vscode.ViewColumn.Two;
        if (TokenUsagePanel.currentPanel) {
            TokenUsagePanel.currentPanel._loadStatus = loadStatus;
            TokenUsagePanel.currentPanel._panel.reveal(column);
            TokenUsagePanel.currentPanel._update();
            return;
        }

        const panel = vscode.window.createWebviewPanel('tokenUsage', '📊 Token 用量', column, {
            enableScripts: true,
            retainContextWhenHidden: true,
        });
        TokenUsagePanel.currentPanel = new TokenUsagePanel(panel, context, loadStatus);
    }

    constructor(panel, context, loadStatus) {
        this._panel = panel;
        this._loadStatus = loadStatus;
        this._status = undefined;
        this._error = '';
        this._loading = false;
        this._autoRefreshInterval = null;

        this._panel.webview.onDidReceiveMessage(message => {
            if (message?.command === 'refresh') this._update();
        }, null, context.subscriptions);
        this._panel.onDidDispose(() => this.dispose(), null, context.subscriptions);
        this._panel.onDidChangeViewState(event => {
            if (event.webviewPanel.visible) this._scheduleAutoRefresh();
            else this._clearAutoRefresh();
        });
        this._update();
        this._scheduleAutoRefresh();
    }

    async _update() {
        this._loading = true;
        this._panel.webview.html = this._getHtmlContent();
        try {
            this._status = await this._loadStatus();
            this._error = '';
        } catch (error) {
            this._status = undefined;
            this._error = error?.message || '无法读取 CLI 用量状态';
        }
        this._loading = false;
        this._panel.webview.html = this._getHtmlContent();
    }

    _scheduleAutoRefresh() {
        if (this._autoRefreshInterval) return;
        this._autoRefreshInterval = setInterval(() => {
            if (this._panel.visible) this._update();
        }, 5 * 60 * 1000);
    }

    _clearAutoRefresh() {
        if (this._autoRefreshInterval) {
            clearInterval(this._autoRefreshInterval);
            this._autoRefreshInterval = null;
        }
    }

    _getHtmlContent() {
        const status = this._status;
        const usage = status?.usage;
        const model = status?.settings?.model;
        const actual = Number.isSafeInteger(usage?.actual_tokens) && usage.actual_tokens >= 0 ? usage.actual_tokens : undefined;
        const uncertain = Number.isSafeInteger(usage?.uncertain_tokens) && usage.uncertain_tokens >= 0 ? usage.uncertain_tokens : undefined;
        const limit = Number.isSafeInteger(model?.token_limit) && model.token_limit > 0 ? model.token_limit : undefined;
        const used = actual !== undefined && uncertain !== undefined ? actual + uncertain : undefined;
        const remaining = used !== undefined && limit !== undefined ? Math.max(0, limit - used) : undefined;
        const percent = used !== undefined && limit ? Math.min(100, used / limit * 100) : 0;
        const error = this._error ? `<p role="alert">读取失败：${escapeHTML(this._error)}</p>` : '';
        const value = (number) => number === undefined ? '未知' : number.toLocaleString('zh-CN');
        const provider = model?.provider || '未配置';
        const modelID = model?.model || '未配置';
        const costStatus = status?.fees || '未提供可信费率，费用估算不可用';

        return `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body{font-family:var(--vscode-font-family);color:var(--vscode-editor-foreground);background:var(--vscode-editor-background);padding:20px;margin:0}
.container{max-width:760px;margin:auto}.header{display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid var(--vscode-panel-border);padding-bottom:12px}
button{background:var(--vscode-button-background);color:var(--vscode-button-foreground);border:0;border-radius:3px;padding:7px 12px;cursor:pointer}
.section{margin:20px 0;padding:16px;background:var(--vscode-sideBar-background);border-left:3px solid var(--vscode-textLink-foreground)}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px}.card{padding:14px;background:var(--vscode-editor-background);text-align:center}
.label{font-size:12px;color:var(--vscode-descriptionForeground)}.value{font-size:20px;font-weight:600;margin-top:8px;overflow-wrap:anywhere}
.muted{color:var(--vscode-descriptionForeground);font-size:12px}.bar{height:8px;background:var(--vscode-progressBar-background);margin-top:12px}.fill{height:100%;background:var(--vscode-textLink-foreground)}
[role=alert]{color:var(--vscode-errorForeground)}
</style></head><body><main class="container">
<div class="header"><div><h2>模型 Token 用量</h2><div class="muted">数据来源：当前工作区的 IntentLens CLI 状态</div></div>
<button onclick="vscode.postMessage({command:'refresh'})">刷新</button></div>
${error}
${this._loading ? '<p role="status">正在读取 CLI 用量状态…</p>' : ''}
<section class="section"><h3>下一次调用配置</h3><p>Provider：${escapeHTML(provider)}<br>模型：${escapeHTML(modelID)}</p><p class="muted">累计用量按项目共享预算统计；配置变更不会把历史用量归到当前模型。</p></section>
<section class="section"><h3>预算用量</h3><div class="grid">
<div class="card"><div class="label">已确认 Token</div><div class="value">${value(actual)}</div></div>
<div class="card"><div class="label">未确认 Token 上限</div><div class="value">${value(uncertain)}</div></div>
<div class="card"><div class="label">总计（含未确认）</div><div class="value">${value(used)}</div></div>
<div class="card"><div class="label">剩余预算 Token</div><div class="value">${value(remaining)}</div></div>
</div><div class="muted">项目调用上限：${value(limit)} tokens。未确认用量也计入预算保留量。</div>
<div class="bar"><div class="fill" style="width:${percent.toFixed(1)}%"></div></div>
<p class="muted">当前状态仅记录总 token，未提供输入与输出拆分。</p></section>
<section class="section"><h3>费用</h3><p>${escapeHTML(costStatus)}</p><p class="muted">费用估算不能替代服务商账单；未取得适用于当前 provider 和模型的费率时不显示金额。</p></section>
<script>const vscode=acquireVsCodeApi();</script></main></body></html>`;
    }

    dispose() {
        this._clearAutoRefresh();
        if (TokenUsagePanel.currentPanel === this) TokenUsagePanel.currentPanel = undefined;
    }
}

function escapeHTML(value) {
    return String(value ?? '').replace(/[&<>"']/g, character => ({
        '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
    })[character]);
}

module.exports = { TokenUsagePanel };
