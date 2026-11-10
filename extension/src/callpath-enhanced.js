'use strict';

/**
 * Enhanced Call Path Provider
 * 显示函数调用链及其性能指标
 */

class CallPathNode {
  constructor(vscode, data, parentExecutionTime = 0) {
    this.vscode = vscode;
    this.name = data.name || 'unknown';
    this.file = data.file;
    this.line = data.line;
    this.column = data.column;
    this.executionTime = data.executionTime || 0;
    this.callCount = data.callCount || 1;
    this.memory = data.memory || 0;
    this.cpu = data.cpu || 0;
    this.children = (data.children || []).map(
      child => new CallPathNode(vscode, child, this.executionTime)
    );
    this.parentExecutionTime = parentExecutionTime;
  }

  getTreeItem() {
    const timePercent = this.parentExecutionTime > 0 ? 
      ((this.executionTime / this.parentExecutionTime) * 100).toFixed(1) : 
      '0.0';

    const label = `${this.name} [${this.executionTime}ms · ${timePercent}%]`;
    
    const item = new this.vscode.TreeItem(
      label,
      this.children.length > 0 ?
        this.vscode.TreeItemCollapsibleState.Collapsed :
        this.vscode.TreeItemCollapsibleState.None
    );

    // 根据执行时间选择图标
    item.iconPath = this.getIconByPerformance();
    
    // 详细信息
    item.description = `${this.callCount} calls · ${this.memory}MB · ${this.cpu}%CPU`;
    
    // 快速链接到源代码
    item.command = {
      command: 'intentlens.callpath.goto',
      title: 'Go to Code',
      arguments: [this.file, this.line, this.column],
    };

    // 上下文菜单
    item.contextValue = 'intentlens.callpath.node';
    
    // 悬停提示
    item.tooltip = this.buildTooltip();

    return item;
  }

  /**
   * 根据性能选择图标
   */
  getIconByPerformance() {
    if (this.executionTime > 1000) {
      return new this.vscode.ThemeIcon('error', 
        new this.vscode.ThemeColor('errorForeground'));
    }
    if (this.executionTime > 500) {
      return new this.vscode.ThemeIcon('warning',
        new this.vscode.ThemeColor('warningForeground'));
    }
    if (this.executionTime > 200) {
      return new this.vscode.ThemeIcon('info',
        new this.vscode.ThemeColor('infoPoreground'));
    }
    return new this.vscode.ThemeIcon('pass',
      new this.vscode.ThemeColor('testing.iconPassed'));
  }

  /**
   * 构建悬停提示
   */
  buildTooltip() {
    const parts = [
      `## 📞 ${this.name}`,
      '',
      `**文件**: ${this.file}:${this.line}`,
      `**执行时间**: ${this.executionTime}ms`,
      `**调用次数**: ${this.callCount}`,
      `**内存分配**: ${this.memory}MB`,
      `**CPU占用**: ${this.cpu}%`,
    ];

    if (this.children.length > 0) {
      parts.push('');
      parts.push('### 子函数调用:');
      for (const child of this.children) {
        parts.push(`- ${child.name} (${child.executionTime}ms)`);
      }
    }

    return new this.vscode.MarkdownString(parts.join('\n'));
  }
}

class EnhancedCallPathProvider {
  constructor(vscode, onDidChangeTreeData) {
    this.vscode = vscode;
    this._onDidChangeTreeData = new vscode.EventEmitter();
    this.onDidChangeTreeData = this._onDidChangeTreeData.event;
    this._callTree = null;
    this._sortBy = 'executionTime'; // executionTime, name, callCount
    this._filterThreshold = 0; // 最小执行时间 (ms)
    this._flattenedNodes = [];
  }

  /**
   * 设置调用树数据
   */
  setCallTree(callTreeData) {
    this._callTree = new CallPathNode(this.vscode, callTreeData);
    this._flattenTree();
    this.refresh();
  }

  /**
   * 刷新视图
   */
  refresh() {
    this._onDidChangeTreeData.fire(undefined);
  }

  /**
   * 获取树项
   */
  getTreeItem(element) {
    return element.getTreeItem();
  }

  /**
   * 获取子项
   */
  getChildren(element) {
    if (!element) {
      return this._callTree ? [this._callTree] : [];
    }
    
    return this.sortNodes(element.children);
  }

  /**
   * 排序节点
   */
  sortNodes(nodes) {
    const sorted = [...nodes];
    
    switch (this._sortBy) {
      case 'executionTime':
        sorted.sort((a, b) => b.executionTime - a.executionTime);
        break;
      case 'name':
        sorted.sort((a, b) => a.name.localeCompare(b.name));
        break;
      case 'callCount':
        sorted.sort((a, b) => b.callCount - a.callCount);
        break;
    }
    
    return sorted;
  }

  /**
   * 展平树为列表 (用于快速搜索)
   */
  _flattenTree(node = this._callTree, result = []) {
    if (!node) return result;
    
    result.push(node);
    for (const child of node.children) {
      this._flattenTree(child, result);
    }
    
    return result;
  }

  /**
   * 搜索函数
   */
  search(query) {
    const searchLower = query.toLowerCase();
    return this._flattenedNodes.filter(node =>
      node.name.toLowerCase().includes(searchLower) ||
      node.file?.toLowerCase().includes(searchLower)
    );
  }

  /**
   * 设置排序方式
   */
  setSortBy(sortBy) {
    this._sortBy = sortBy;
    this.refresh();
  }

  /**
   * 设置性能过滤阈值
   */
  setFilterThreshold(ms) {
    this._filterThreshold = ms;
    this.refresh();
  }

  /**
   * 导出调用图
   */
  exportAsText(node = this._callTree, indent = 0) {
    if (!node) return '';
    
    let result = '';
    result += ' '.repeat(indent) + 
      `└─ ${node.name} [${node.executionTime}ms]\n`;
    
    for (const child of node.children) {
      result += this.exportAsText(child, indent + 3);
    }
    
    return result;
  }

  /**
   * 注册命令
   */
  registerCommands(context) {
    context.subscriptions.push(
      this.vscode.commands.registerCommand(
        'intentlens.callpath.goto',
        async (file, line, column) => {
          const uri = this.vscode.Uri.file(file);
          const doc = await this.vscode.workspace.openTextDocument(uri);
          const editor = await this.vscode.window.showTextDocument(doc);
          
          const position = new this.vscode.Position(line - 1, column - 1);
          editor.selection = new this.vscode.Selection(position, position);
          editor.revealRange(
            new this.vscode.Range(position, position),
            this.vscode.TextEditorRevealType.InCenter
          );
        }
      )
    );

    context.subscriptions.push(
      this.vscode.commands.registerCommand(
        'intentlens.callpath.export',
        () => {
          const exported = this.exportAsText();
          this.vscode.window.showInformationMessage('调用图已导出');
          // 复制到剪贴板
          this.vscode.env.clipboard.writeText(exported);
        }
      )
    );

    context.subscriptions.push(
      this.vscode.commands.registerCommand(
        'intentlens.callpath.sort',
        async () => {
          const sortBy = await this.vscode.window.showQuickPick([
            { label: '⏱ 按执行时间', value: 'executionTime' },
            { label: '📝 按名称', value: 'name' },
            { label: '📊 按调用次数', value: 'callCount' },
          ], { placeHolder: '选择排序方式' });
          
          if (sortBy) {
            this.setSortBy(sortBy.value);
          }
        }
      )
    );
  }
}

module.exports = {
  EnhancedCallPathProvider,
  CallPathNode,
};
