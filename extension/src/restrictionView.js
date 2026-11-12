// RestrictionTreeView - VS Code tree view for displaying aggregated restrictions
const vscode = require('vscode');

class RestrictionTreeProvider {
    constructor() {
        this._onDidChangeTreeData = new vscode.EventEmitter();
        this.onDidChangeTreeData = this._onDidChangeTreeData.event;
        this.aggregatedData = null;
    }

    refresh(aggregatedData) {
        this.aggregatedData = aggregatedData;
        this._onDidChangeTreeData.fire();
    }

    clear() {
        this.aggregatedData = null;
        this._onDidChangeTreeData.fire();
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
            const summaryItem = new vscode.TreeItem(
                📋 解析限制汇总 (类，共个问题),
                vscode.TreeItemCollapsibleState.Expanded
            );
            summaryItem.contextValue = 'summary';
            items.push(summaryItem);

            // Category items
            for (const category of this.aggregatedData.categories) {
                const categoryItem = new vscode.TreeItem(
                    ${category.icon}  (),
                    category.count > 0 ? vscode.TreeItemCollapsibleState.Collapsed : vscode.TreeItemCollapsibleState.None
                );
                categoryItem.contextValue = 'category';
                categoryItem.tooltip = ${category.count} issues in ;
                categoryItem.category = category;
                items.push(categoryItem);
            }

            return items;
        }

        // Category level: show items (restrictions or summaries)
        if (element.contextValue === 'category') {
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
                    summaryItem.tooltip = Full list: ;
                    summaryItem.command = {
                        command: 'intentlens.showRestrictionDetails',
                        title: 'Show Details',
                        arguments: [item]
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
                    if (item.filePath && item.lineNumber > 0) {
                        restrictionItem.command = {
                            command: 'intentlens.navigateToRestriction',
                            title: 'Go to Location',
                            arguments: [item.filePath, item.lineNumber]
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
            label = ${fileName};
            if (restriction.lineNumber > 0) {
                label += :;
            }
            label +=  - ;
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
