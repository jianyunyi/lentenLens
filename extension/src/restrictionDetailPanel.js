// RestrictionDetailPanel - Webview panel for detailed restriction viewing
const vscode = require('vscode');

class RestrictionDetailPanel {
    static currentPanel = undefined;

    static show(context, aggregatedData) {
        const column = vscode.ViewColumn.Two;

        if (RestrictionDetailPanel.currentPanel) {
            RestrictionDetailPanel.currentPanel._panel.reveal(column);
            RestrictionDetailPanel.currentPanel._update(aggregatedData);
            return;
        }

        const panel = vscode.window.createWebviewPanel(
            'restrictionDetails',
            '解析限制详情',
            column,
            {
                enableScripts: true,
                retainContextWhenHidden: true
            }
        );

        RestrictionDetailPanel.currentPanel = new RestrictionDetailPanel(panel, context, aggregatedData);
    }

    constructor(panel, context, aggregatedData) {
        this._panel = panel;
        this._context = context;
        this._aggregatedData = aggregatedData;
        this._currentFilter = 'all';

        this._update(aggregatedData);

        this._panel.onDidDispose(() => this.dispose(), null, context.subscriptions);

        this._panel.webview.onDidReceiveMessage(
            message => {
                switch (message.command) {
                    case 'filter':
                        this._currentFilter = message.category;
                        this._update(this._aggregatedData);
                        break;
                    case 'copy':
                        vscode.env.clipboard.writeText(message.text);
                        vscode.window.showInformationMessage('已复制到剪贴板');
                        break;
                    case 'navigate':
                        this._navigateToLocation(message.filePath, message.line);
                        break;
                }
            },
            null,
            context.subscriptions
        );
    }

    _update(aggregatedData) {
        this._aggregatedData = aggregatedData;
        this._panel.webview.html = this._getHtmlContent();
    }

    _navigateToLocation(filePath, line) {
        vscode.workspace.openTextDocument(filePath).then(doc => {
            vscode.window.showTextDocument(doc).then(editor => {
                const position = new vscode.Position(line - 1, 0);
                editor.selection = new vscode.Selection(position, position);
                editor.revealRange(new vscode.Range(position, position));
            });
        });
    }

    _getHtmlContent() {
        if (!this._aggregatedData) {
            return this._getNoRestrictionsHtml();
        }

        const categories = this._aggregatedData.categories;
        const filteredCategories = this._currentFilter === 'all' 
            ? categories 
            : categories.filter(c => c.name === this._currentFilter);

        let restrictionsHtml = '';
        let totalCount = 0;

        for (const category of filteredCategories) {
            restrictionsHtml += '<div class="category-section">';
            restrictionsHtml += '<h3>' + category.icon + ' ' + category.name + ' (' + category.count + ')</h3>';

            for (const item of category.items) {
                if (item.type === 'summary') {
                    restrictionsHtml += this._renderSummary(item);
                } else {
                    restrictionsHtml += this._renderRestriction(item);
                }
                totalCount++;
            }

            restrictionsHtml += '</div>';
        }

        const filterButtons = categories.map(c => 
            '<button class="filter-btn' + (this._currentFilter === c.name ? ' active' : '') + '" onclick="filterCategory(\'' + c.name + '\')">' + c.icon + ' ' + c.name + ' (' + c.count + ')</button>'
        ).join('');

        return '<!DOCTYPE html><html lang="zh-CN"><head><meta charset="UTF-8"><title>解析限制详情</title>' +
            '<style>body{font-family:var(--vscode-font-family);padding:20px}.header{display:flex;justify-content:space-between;margin-bottom:20px;border-bottom:1px solid var(--vscode-panel-border)}.filter-bar{display:flex;gap:10px;margin-bottom:20px}.filter-btn{padding:5px 10px;background:var(--vscode-button-background);color:var(--vscode-button-foreground);border:none;cursor:pointer;border-radius:3px}.filter-btn.active{background:var(--vscode-button-hoverBackground)}.category-section{margin-bottom:30px}.restriction-item{background:var(--vscode-editor-background);padding:12px;margin-bottom:10px;border-left:3px solid var(--vscode-textLink-foreground);border-radius:3px}.restriction-location{font-weight:bold;color:var(--vscode-textLink-foreground);cursor:pointer}.copy-btn{padding:2px 8px;font-size:11px;background:var(--vscode-button-secondaryBackground);border:none;cursor:pointer}</style>' +
            '</head><body>' +
            '<div class="header"><h2>📋 解析限制详情</h2><div>共 ' + totalCount + ' 个问题</div></div>' +
            '<div class="filter-bar"><button class="filter-btn' + (this._currentFilter === 'all' ? ' active' : '') + '" onclick="filterCategory(\'all\')">全部</button>' + filterButtons + '</div>' +
            restrictionsHtml +
            '<script>const vscode=acquireVsCodeApi();function filterCategory(c){vscode.postMessage({command:\'filter\',category:c})}function copyText(t){vscode.postMessage({command:\'copy\',text:t})}function navigateTo(f,l){vscode.postMessage({command:\'navigate\',filePath:f,line:l})}</script>' +
            '</body></html>';
    }

    _renderSummary(summary) {
        return '<div class="restriction-item"><div><strong>' + summary.message + '</strong> <button class="copy-btn" onclick="copyText(\'' + summary.fullList.join(', ') + '\')">复制</button></div><div>完整列表: ' + summary.fullList.join(', ') + '</div></div>';
    }

    _renderRestriction(restriction) {
        const location = restriction.filePath ? restriction.filePath + ':' + restriction.lineNumber : '未知位置';
        const locationHtml = restriction.filePath && restriction.lineNumber > 0
            ? '<span class="restriction-location" onclick="navigateTo(\'' + restriction.filePath + '\',' + restriction.lineNumber + ')">' + location + '</span>'
            : '<span>' + location + '</span>';

        return '<div class="restriction-item"><div>' + locationHtml + ' <button class="copy-btn" onclick="copyText(\'' + restriction.message.replace(/'/g, "\\'") + '\')">复制</button></div><div>' + restriction.message + '</div></div>';
    }

    _getNoRestrictionsHtml() {
        return '<!DOCTYPE html><html><head><meta charset="UTF-8"><style>body{text-align:center;padding:40px}</style></head><body><h2>✅ 未发现问题</h2><p>代码解析未发现任何限制或错误</p></body></html>';
    }

    dispose() {
        RestrictionDetailPanel.currentPanel = undefined;
        this._panel.dispose();
    }
}

module.exports = { RestrictionDetailPanel };