// RestrictionDetailPanel - Webview panel for detailed restriction viewing (Impeccable UI Optimized)
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
                        vscode.window.showInformationMessage('✓ 已复制到剪贴板');
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
            const categoryClass = this._getCategoryTypeClass(category.name);
            restrictionsHtml += '<div class="category-section ' + categoryClass + '">';
            restrictionsHtml += '<div class="category-header"><h3>' + category.icon + ' ' + category.name + '</h3><span class="category-count">' + category.count + '</span></div>';

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

        const filterButtons = '<button class="filter-btn' + (this._currentFilter === 'all' ? ' active' : '') + '" onclick="filterCategory(\'all\')">全部</button>' +
            categories.map(c => 
                '<button class="filter-btn' + (this._currentFilter === c.name ? ' active' : '') + '" onclick="filterCategory(\'' + c.name + '\')">' + c.icon + ' ' + c.name + '</button>'
            ).join('');

        return '<!DOCTYPE html><html lang="zh-CN"><head><meta charset="UTF-8"><title>解析限制详情</title>' +
            '<style>' + this._getOptimizedStyles() + '</style>' +
            '</head><body><div class="container">' +
            '<div class="header"><h2>📋 解析限制详情</h2><div class="header-stats"><span class="stat-badge">' + totalCount + '</span><span> 个问题</span></div></div>' +
            '<div class="filter-bar"><span class="filter-label">筛选：</span>' + filterButtons + '</div>' +
            restrictionsHtml +
            '</div><script>const vscode=acquireVsCodeApi();function filterCategory(c){vscode.postMessage({command:"filter",category:c})}function copyText(t){vscode.postMessage({command:"copy",text:t})}function navigateTo(f,l){vscode.postMessage({command:"navigate",filePath:f,line:l})}</script>' +
            '</body></html>';
    }

    _getCategoryTypeClass(name) {
        if (name.includes('Missing') || name.includes('missing')) return 'type-missing-import';
        if (name.includes('Ambiguous') || name.includes('ambiguous')) return 'type-ambiguous-call';
        if (name.includes('Type') || name.includes('type')) return 'type-type-check';
        return 'type-default';
    }

    _getOptimizedStyles() {
        return `*{box-sizing:border-box}body{font-family:var(--vscode-font-family);background:var(--vscode-editor-background);color:var(--vscode-editor-foreground);padding:0;margin:0}.container{max-width:1200px;margin:0 auto;padding:24px}.header{margin-bottom:32px;padding-bottom:20px;border-bottom:2px solid var(--vscode-panel-border)}.header h2{margin:0 0 8px 0;font-size:20px;font-weight:600}.header-stats{display:flex;gap:12px;align-items:center;margin-top:12px;font-size:13px}.stat-badge{background:var(--vscode-badge-background);color:var(--vscode-badge-foreground);padding:4px 12px;border-radius:12px;font-weight:600;min-width:40px;text-align:center}.filter-bar{display:flex;gap:8px;margin-bottom:24px;flex-wrap:wrap;align-items:center}.filter-label{font-size:12px;font-weight:600;color:var(--vscode-descriptionForeground);text-transform:uppercase;letter-spacing:0.5px}.filter-btn{padding:8px 14px;background:var(--vscode-button-secondaryBackground);color:var(--vscode-button-secondaryForeground);border:1px solid var(--vscode-focusBorder);cursor:pointer;border-radius:4px;font-size:12px;font-weight:500;transition:all 200ms cubic-bezier(0.4,0,0.2,1)}.filter-btn:hover{background:var(--vscode-button-secondaryHoverBackground);transform:translateY(-2px);box-shadow:0 2px 8px rgba(0,0,0,0.15)}.filter-btn.active{background:var(--vscode-button-background);color:var(--vscode-button-foreground);border-color:var(--vscode-button-background)}.category-section{margin-bottom:24px;padding:20px;background:var(--vscode-sideBar-background);border-radius:6px;border-left:4px solid var(--vscode-textLink-foreground);transition:all 250ms ease}.category-section:hover{box-shadow:0 2px 8px rgba(0,0,0,0.1)}.category-section.type-missing-import{border-left-color:#f78c6b;background:rgba(247,140,107,0.05)}.category-section.type-ambiguous-call{border-left-color:#6cb6ff;background:rgba(108,182,255,0.05)}.category-section.type-type-check{border-left-color:#f5d547;background:rgba(245,213,71,0.05)}.category-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:16px;padding-bottom:12px;border-bottom:1px solid var(--vscode-panel-border)}.category-header h3{margin:0;font-size:15px;font-weight:600}.category-count{background:var(--vscode-badge-background);color:var(--vscode-badge-foreground);padding:3px 12px;border-radius:12px;font-size:12px;font-weight:600}.restriction-item{background:var(--vscode-editor-background);padding:16px;margin-bottom:12px;border-left:3px solid var(--vscode-textLink-foreground);border-radius:4px;display:flex;flex-direction:column;gap:12px;transition:all 200ms ease}.restriction-item:hover{background:var(--vscode-list-hoverBackground);border-left-width:4px}.restriction-header{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}.restriction-location{font-weight:600;color:var(--vscode-textLink-foreground);cursor:pointer;padding:4px 8px;border-radius:2px;transition:background 150ms ease;user-select:none}.restriction-location:hover{background:var(--vscode-editor-selectionBackground);text-decoration:underline}.restriction-message{font-size:13px;line-height:1.5}.copy-btn{padding:6px 12px;font-size:12px;background:var(--vscode-button-secondaryBackground);color:var(--vscode-button-secondaryForeground);border:1px solid var(--vscode-focusBorder);cursor:pointer;border-radius:3px;font-weight:500;transition:all 150ms ease;white-space:nowrap}.copy-btn:hover{background:var(--vscode-button-secondaryHoverBackground);transform:translateY(-2px);box-shadow:0 2px 4px rgba(0,0,0,0.1)}.copy-btn:active{transform:scale(0.98)}.summary-item{background:linear-gradient(135deg,var(--vscode-editor-background),var(--vscode-sideBar-background));padding:16px;margin-bottom:12px;border-left:4px solid var(--vscode-textLink-foreground);border-radius:4px;border-top:1px solid var(--vscode-panel-border)}.summary-title{font-weight:700;margin-bottom:8px;font-size:14px;display:flex;justify-content:space-between;align-items:center;gap:12px}.summary-list{font-size:12px;color:var(--vscode-descriptionForeground);line-height:1.6;margin-top:8px;display:flex;gap:6px;flex-wrap:wrap}.summary-item-tag{display:inline-block;background:var(--vscode-badge-background);color:var(--vscode-badge-foreground);padding:2px 8px;border-radius:3px;font-family:monospace;font-size:11px}.empty-state{text-align:center;padding:60px 20px;color:var(--vscode-descriptionForeground)}.empty-state h2{margin:0 0 12px 0;font-size:18px;font-weight:600}.empty-state p{margin:0;font-size:14px}`;
    }

    _renderSummary(summary) {
        const items = summary.fullList || [];
        const sampleItems = items.slice(0, 5).map(item => '<span class="summary-item-tag">' + item + '</span>').join('');
        const hasMore = items.length > 5;
        const fullList = items.join(', ').replace(/'/g, "\\'");
        
        return '<div class="summary-item"><div class="summary-title"><strong>' + summary.message + '</strong><button class="copy-btn" onclick="copyText(\'' + fullList + '\')">📋 复制全部</button></div><div class="summary-list">' + sampleItems + (hasMore ? '<span class="summary-item-tag">...</span>' : '') + '</div></div>';
    }

    _renderRestriction(restriction) {
        const fileName = restriction.filePath ? restriction.filePath.split(/[\\/]/).pop() : '未知';
        const location = fileName + (restriction.lineNumber > 0 ? ':' + restriction.lineNumber : '');
        const locationHtml = restriction.filePath && restriction.lineNumber > 0
            ? '<span class="restriction-location" onclick="navigateTo(\'' + restriction.filePath.replace(/'/g, "\\'") + '\',' + restriction.lineNumber + ')">📂 ' + location + '</span>'
            : '<span class="restriction-location">📂 ' + location + '</span>';

        const messageEscaped = restriction.message.replace(/'/g, "\\'");

        return '<div class="restriction-item"><div class="restriction-header"><div>' + locationHtml + '</div><button class="copy-btn" onclick="copyText(\'' + messageEscaped + '\')">📋 复制</button></div><div class="restriction-message">' + restriction.message + '</div></div>';
    }

    _getNoRestrictionsHtml() {
        return '<!DOCTYPE html><html><head><meta charset="UTF-8"><style>*{box-sizing:border-box}body{font-family:var(--vscode-font-family);background:var(--vscode-editor-background);color:var(--vscode-editor-foreground);margin:0;padding:0;display:flex;align-items:center;justify-content:center;min-height:100vh}.empty-state{text-align:center;color:var(--vscode-descriptionForeground)}.empty-state h2{margin:0 0 12px 0;font-size:24px;font-weight:600;color:var(--vscode-editor-foreground)}.empty-state p{margin:0;font-size:14px}</style></head><body><div class="empty-state"><h2>✅ 未发现问题</h2><p>代码解析未发现任何限制或错误</p></div></body></html>';
    }

    dispose() {
        RestrictionDetailPanel.currentPanel = undefined;
        this._panel.dispose();
    }
}

module.exports = { RestrictionDetailPanel };
