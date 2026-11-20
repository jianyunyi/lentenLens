// RestrictionTreeView - VS Code tree view for displaying aggregated restrictions
const vscode = require('vscode');

class RestrictionTreeProvider {
    constructor() {
        this._onDidChangeTreeData = new vscode.EventEmitter();
        this.onDidChangeTreeData = this._onDidChangeTreeData.event;
        this.aggregatedData = null;
    }

    refresh(aggregatedData, resultId, idsByItem) {
        this.aggregatedData = aggregatedData;
        this.resultId = resultId;
        this.idsByItem = idsByItem;
        this._onDidChangeTreeData.fire();
    }

    clear() {
        this.refresh(null);
    }

    dispose() {
        this._onDidChangeTreeData.dispose();
    }

    getTreeItem(element) {
        return element;
    }

    getChildren(element) {
        if (!this.aggregatedData) {
            return [];
        }

        // Root level: show summary and categories
        if (!element) {
            const items = [];
            
            // Summary header
            const categoryCount = this.aggregatedData.categories.length;
            const totalCount = this.aggregatedData.categories.reduce((sum, cat) => sum + cat.count, 0);
            const summaryItem = new vscode.TreeItem(
                `📋 解析限制汇总 (${categoryCount}类，共${totalCount}个问题)`,
                vscode.TreeItemCollapsibleState.Expanded
            );
            summaryItem.contextValue = 'summary';
            items.push(summaryItem);

            // Category items
            for (const category of this.aggregatedData.categories) {
                const categoryItem = new vscode.TreeItem(
                    `${category.icon} ${category.name} (${category.count})`,
                    category.count > 0 ? vscode.TreeItemCollapsibleState.Collapsed : vscode.TreeItemCollapsibleState.None
                );
                categoryItem.contextValue = 'category';
                categoryItem.tooltip = `${category.count} issues in ${category.name}`;
                categoryItem.category = category;
                categoryItem.resultId = this.resultId;
                items.push(categoryItem);
            }

            return items;
        }

        // Category level: show items (restrictions or summaries)
        if (element.contextValue === 'category' && element.resultId === this.resultId) {
            const category = element.category;
            const items = [];

            for (const item of category.items) {
                if (item.type === 'summary') {
                    // ItemSummary (e.g., Missing Imports summary)
                    const summaryItem = new vscode.TreeItem(
                        item.message,
                        vscode.TreeItemCollapsibleState.None
                    );
                    summaryItem.contextValue = 'itemSummary';
                    summaryItem.tooltip = `Full list: ${item.details || item.message}`;
                    summaryItem.command = {
                        command: 'intentlens.showRestrictionDetails',
                        title: 'Show Details',
                        arguments: [this.resultId]
                    };
                    items.push(summaryItem);
                } else {
                    // Individual Restriction
                    const restrictionItem = new vscode.TreeItem(
                        this._formatRestrictionLabel(item),
                        vscode.TreeItemCollapsibleState.None
                    );
                    restrictionItem.contextValue = 'restriction';
                    restrictionItem.tooltip = item.message;
                    
                    // Add command to navigate to source location
                    const recordId = this.idsByItem.get(item);
                    if (recordId) {
                        restrictionItem.command = {
                            command: 'intentlens.navigateToRestriction',
                            title: 'Go to Location',
                            arguments: [recordId]
                        };
                    }
                    
                    items.push(restrictionItem);
                }
            }

            return items;
        }

        return [];
    }

    _formatRestrictionLabel(restriction) {
        let label = '';
        
        if (restriction.filePath) {
            const fileName = restriction.filePath.split(/[/\\]/).pop();
            label = `${fileName}`;
            if (restriction.lineNumber > 0) {
                label += `:${restriction.lineNumber}`;
            }
            label += ` - `;
        }
        
        // Truncate long messages
        const message = restriction.message.length > 60 
            ? restriction.message.substring(0, 57) + '...'
            : restriction.message;
        
        label += message;
        
        return label;
    }
}

module.exports = { RestrictionTreeProvider };
