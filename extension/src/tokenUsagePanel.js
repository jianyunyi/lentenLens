// TokenUsagePanel - WebView panel for tracking API token consumption and costs
const vscode = require('vscode');

class TokenUsagePanel {
    static currentPanel = undefined;

    static show(context) {
        const column = vscode.ViewColumn.Two;

        if (TokenUsagePanel.currentPanel) {
            TokenUsagePanel.currentPanel._panel.reveal(column);
            TokenUsagePanel.currentPanel._update();
            return;
        }

        const panel = vscode.window.createWebviewPanel(
            'tokenUsage',
            '📊 Token 消耗',
            column,
            {
                enableScripts: true,
                retainContextWhenHidden: true
            }
        );

        TokenUsagePanel.currentPanel = new TokenUsagePanel(panel, context);
    }

    constructor(panel, context) {
        this._panel = panel;
        this._context = context;
        this._usageData = {
            sessionStart: Date.now(),
            inputTokens: 0,
            outputTokens: 0,
            dailyStats: {},
            monthlyBudget: 100,
            lastSyncTime: Date.now(),
            historicalData: []
        };

        this._autoRefreshInterval = null;

        this._load();
        this._update();
        this._scheduleAutoRefresh();

        this._panel.onDidDispose(() => this.dispose(), null, context.subscriptions);

        this._panel.webview.onDidReceiveMessage(
            message => {
                switch (message.command) {
                    case 'refresh':
                        this._update();
                        break;
                    case 'updateBudget':
                        this._updateBudget(message.data.budget);
                        break;
                    case 'export':
                        this._export(message.data.format);
                        break;
                    case 'reset':
                        this._resetStatistics();
                        break;
                }
            },
            null,
            context.subscriptions
        );

        this._panel.onDidChangeViewState(e => {
            if (e.webviewPanel.visible) {
                this._scheduleAutoRefresh();
            } else {
                this._clearAutoRefresh();
            }
        });
    }

    async _load() {
        try {
            const saved = await this._context.globalState.get('intentlens.tokenUsage');
            if (saved) {
                this._usageData = { ...this._usageData, ...saved };
            }
        } catch (error) {
            console.error('Failed to load token usage data:', error);
        }
    }

    async _save() {
        try {
            await this._context.globalState.update('intentlens.tokenUsage', this._usageData);
        } catch (error) {
            console.error('Failed to save token usage data:', error);
        }
    }

    async _update() {
        this._panel.webview.html = this._getHtmlContent();
    }

    _scheduleAutoRefresh() {
        if (this._autoRefreshInterval) {
            return;
        }
        this._autoRefreshInterval = setInterval(() => {
            if (this._panel.visible) {
                this._update();
            }
        }, 5 * 60 * 1000); // 5 minutes
    }

    _clearAutoRefresh() {
        if (this._autoRefreshInterval) {
            clearInterval(this._autoRefreshInterval);
            this._autoRefreshInterval = null;
        }
    }

    _updateBudget(budget) {
        if (budget > 0) {
            this._usageData.monthlyBudget = budget;
            this._save();
            this._update();
        }
    }

    async _export(format) {
        const timestamp = new Date().toISOString().replace(/:/g, '-').split('.')[0];
        
        if (format === 'csv') {
            const csv = this._generateCSV();
            await this._downloadFile(`token-usage-${timestamp}.csv`, csv, 'text/csv');
        } else if (format === 'json') {
            const json = JSON.stringify(this._usageData, null, 2);
            await this._downloadFile(`token-usage-${timestamp}.json`, json, 'application/json');
        }
    }

    _generateCSV() {
        let csv = 'Date,InputTokens,OutputTokens,TotalTokens,EstimatedCost\n';
        
        for (const [date, stats] of Object.entries(this._usageData.dailyStats)) {
            const total = stats.input + stats.output;
            const cost = this._calculateCost(stats.input, stats.output);
            csv += `${date},${stats.input},${stats.output},${total},${cost.toFixed(4)}\n`;
        }
        
        return csv;
    }

    async _downloadFile(filename, content, mimeType) {
        const message = `要导出为 ${filename}，请手动复制并保存以下内容。`;
        vscode.window.showInformationMessage(message);
    }

    async _resetStatistics() {
        const confirm = await vscode.window.showWarningMessage(
            '确定要重置所有统计数据吗？此操作无法撤销。',
            '重置',
            '取消'
        );
        
        if (confirm === '重置') {
            this._usageData = {
                sessionStart: Date.now(),
                inputTokens: 0,
                outputTokens: 0,
                dailyStats: {},
                monthlyBudget: this._usageData.monthlyBudget,
                lastSyncTime: Date.now(),
                historicalData: []
            };
            await this._save();
            this._update();
            vscode.window.showInformationMessage('✓ 统计数据已重置');
        }
    }

    _calculateCost(inputTokens, outputTokens) {
        // OpenAI pricing: ~$0.5/$1.50 per 1M tokens (approximate)
        const inputCost = inputTokens * 0.0000005;
        const outputCost = outputTokens * 0.0000015;
        return inputCost + outputCost;
    }

    _getStatsByPeriod(period) {
        const now = new Date();
        const today = now.toISOString().split('T')[0];
        let startDate = today;

        if (period === 'week') {
            const weekAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
            startDate = weekAgo.toISOString().split('T')[0];
        } else if (period === 'month') {
            const monthAgo = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);
            startDate = monthAgo.toISOString().split('T')[0];
        }

        let stats = { input: 0, output: 0, cost: 0 };
        
        for (const [date, data] of Object.entries(this._usageData.dailyStats)) {
            if (date >= startDate) {
                stats.input += data.input;
                stats.output += data.output;
            }
        }
        
        stats.cost = this._calculateCost(stats.input, stats.output);
        return stats;
    }

    _getHtmlContent() {
        const today = this._getStatsByPeriod('today');
        const week = this._getStatsByPeriod('week');
        const month = this._getStatsByPeriod('month');
        
        const totalCost = this._calculateCost(this._usageData.inputTokens, this._usageData.outputTokens);
        const budgetUsed = (totalCost / this._usageData.monthlyBudget * 100).toFixed(1);
        const budgetColor = budgetUsed > 80 ? '#f48771' : budgetUsed > 50 ? '#f5d547' : '#10a981';

        return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <title>Token 消耗</title>
    <style>${this._getStyles()}</style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>📊 Token 消耗追踪</h2>
            <button class="refresh-btn" onclick="vscode.postMessage({command:'refresh'})">🔄 刷新</button>
        </div>

        <div class="section">
            <h3>当前会话</h3>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-label">输入 Token</div>
                    <div class="stat-value">${this._usageData.inputTokens.toLocaleString()}</div>
                </div>
                <div class="stat-card">
                    <div class="stat-label">输出 Token</div>
                    <div class="stat-value">${this._usageData.outputTokens.toLocaleString()}</div>
                </div>
                <div class="stat-card">
                    <div class="stat-label">总计 Token</div>
                    <div class="stat-value">${(this._usageData.inputTokens + this._usageData.outputTokens).toLocaleString()}</div>
                </div>
                <div class="stat-card">
                    <div class="stat-label">会话成本</div>
                    <div class="stat-value">$${totalCost.toFixed(4)}</div>
                </div>
            </div>
        </div>

        <div class="section">
            <h3>累计统计</h3>
            <div class="period-stats">
                <div class="period">
                    <div class="period-title">今天</div>
                    <div class="period-value">${today.input.toLocaleString()} / ${today.output.toLocaleString()} / $${today.cost.toFixed(4)}</div>
                </div>
                <div class="period">
                    <div class="period-title">本周</div>
                    <div class="period-value">${week.input.toLocaleString()} / ${week.output.toLocaleString()} / $${week.cost.toFixed(4)}</div>
                </div>
                <div class="period">
                    <div class="period-title">本月</div>
                    <div class="period-value">${month.input.toLocaleString()} / ${month.output.toLocaleString()} / $${month.cost.toFixed(4)}</div>
                </div>
            </div>
        </div>

        <div class="section">
            <h3>月度预算</h3>
            <div class="budget-container">
                <div class="budget-info">
                    <div>已用: <span style="color: ${budgetColor}; font-weight: 600;">$${totalCost.toFixed(2)}</span> / $${this._usageData.monthlyBudget}</div>
                    <div>使用率: <span style="color: ${budgetColor}; font-weight: 600;">${budgetUsed}%</span></div>
                </div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: ${Math.min(budgetUsed, 100)}%; background-color: ${budgetColor};"></div>
                </div>
                <div class="budget-input-group">
                    <label>月度预算 ($)</label>
                    <div class="input-group">
                        <input type="number" id="budgetInput" value="${this._usageData.monthlyBudget}" min="1" step="10">
                        <button onclick="vscode.postMessage({command:'updateBudget', data:{budget: parseFloat(document.getElementById('budgetInput').value)}})">保存</button>
                    </div>
                </div>
            </div>
        </div>

        <div class="section">
            <h3>数据导出</h3>
            <div class="action-buttons">
                <button onclick="vscode.postMessage({command:'export', data:{format:'csv'}})">📥 导出 CSV</button>
                <button onclick="vscode.postMessage({command:'export', data:{format:'json'}})">📥 导出 JSON</button>
                <button class="danger-btn" onclick="vscode.postMessage({command:'reset'})">🗑️ 重置统计</button>
            </div>
        </div>

        <div class="footer">
            <small>最后同步: ${new Date(this._usageData.lastSyncTime).toLocaleString('zh-CN')}</small>
        </div>
    </div>

    <script>
        const vscode = acquireVsCodeApi();
    </script>
</body>
</html>`;
    }

    _getStyles() {
        return `* { box-sizing: border-box; }
body { font-family: var(--vscode-font-family); background: var(--vscode-editor-background); color: var(--vscode-editor-foreground); padding: 0; margin: 0; }
.container { max-width: 1000px; margin: 0 auto; padding: 24px; }
.header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 32px; padding-bottom: 16px; border-bottom: 2px solid var(--vscode-panel-border); }
.header h2 { margin: 0; font-size: 20px; font-weight: 600; }
.refresh-btn { padding: 8px 16px; background: var(--vscode-button-background); color: var(--vscode-button-foreground); border: none; border-radius: 4px; cursor: pointer; font-weight: 500; transition: all 200ms ease; }
.refresh-btn:hover { background: var(--vscode-button-hoverBackground); transform: translateY(-2px); box-shadow: 0 2px 8px rgba(0,0,0,0.15); }
.section { margin-bottom: 32px; padding: 20px; background: var(--vscode-sideBar-background); border-radius: 6px; border-left: 4px solid var(--vscode-textLink-foreground); }
.section h3 { margin: 0 0 16px 0; font-size: 15px; font-weight: 600; }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; }
.stat-card { background: var(--vscode-editor-background); padding: 16px; border-radius: 4px; text-align: center; }
.stat-label { font-size: 12px; color: var(--vscode-descriptionForeground); text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 8px; }
.stat-value { font-size: 20px; font-weight: 700; color: var(--vscode-textLink-foreground); }
.period-stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; }
.period { background: var(--vscode-editor-background); padding: 16px; border-radius: 4px; }
.period-title { font-size: 12px; color: var(--vscode-descriptionForeground); margin-bottom: 8px; }
.period-value { font-size: 13px; font-weight: 500; font-family: monospace; }
.budget-container { background: var(--vscode-editor-background); padding: 16px; border-radius: 4px; }
.budget-info { margin-bottom: 12px; font-size: 13px; }
.progress-bar { height: 8px; background: var(--vscode-panel-border); border-radius: 4px; overflow: hidden; margin-bottom: 16px; }
.progress-fill { height: 100%; transition: width 300ms ease; }
.budget-input-group { margin-top: 16px; }
.budget-input-group label { display: block; font-size: 12px; font-weight: 600; margin-bottom: 8px; }
.input-group { display: flex; gap: 8px; }
.input-group input { flex: 1; padding: 8px 12px; background: var(--vscode-input-background); color: var(--vscode-input-foreground); border: 1px solid var(--vscode-input-border); border-radius: 3px; font-size: 13px; }
.input-group input:focus { outline: none; border-color: var(--vscode-focusBorder); box-shadow: 0 0 0 2px rgba(30, 130, 255, 0.1); }
.input-group button { padding: 8px 16px; background: var(--vscode-button-background); color: var(--vscode-button-foreground); border: none; border-radius: 3px; cursor: pointer; font-weight: 500; transition: all 150ms ease; }
.input-group button:hover { background: var(--vscode-button-hoverBackground); }
.action-buttons { display: flex; gap: 8px; flex-wrap: wrap; }
.action-buttons button { padding: 10px 16px; background: var(--vscode-button-background); color: var(--vscode-button-foreground); border: none; border-radius: 4px; cursor: pointer; font-weight: 500; transition: all 200ms ease; }
.action-buttons button:hover { transform: translateY(-2px); box-shadow: 0 2px 8px rgba(0,0,0,0.15); }
.action-buttons button.danger-btn { background: #f48771; }
.action-buttons button.danger-btn:hover { background: #f26154; }
.footer { text-align: center; color: var(--vscode-descriptionForeground); margin-top: 24px; padding-top: 16px; border-top: 1px solid var(--vscode-panel-border); }`;
    }

    dispose() {
        TokenUsagePanel.currentPanel = undefined;
        this._clearAutoRefresh();
        this._save();
        this._panel.dispose();
    }
}

module.exports = { TokenUsagePanel };
