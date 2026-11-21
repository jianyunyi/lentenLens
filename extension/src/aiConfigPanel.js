// Configuration status and entry points. The shared wizard owns the only form.
'use strict';
const vscode = require('vscode');
const crypto = require('node:crypto');
const { page, escapeHTML } = require('./core');
const { readModelConfiguration } = require('./providers');

class AIConfigPanel {
    static currentPanel;

    static async show(context, folder, actions) {
        const current = AIConfigPanel.currentPanel;
        if (current && current._folder.uri.fsPath === folder.uri.fsPath) {
            current._panel.reveal(vscode.ViewColumn.Two);
            await current._handleMessage({ action: 'refresh' });
            return current;
        }
        current?._panel.dispose();
        const view = vscode.window.createWebviewPanel(
            'aiConfigPanel', 'IntentLens: API 配置', vscode.ViewColumn.Two,
            { enableScripts: true }
        );
        const panel = new AIConfigPanel(view, context, folder, actions);
        AIConfigPanel.currentPanel = panel;
        await panel._handleMessage({ action: 'refresh' });
        return panel;
    }

    constructor(panel, context, folder, actions) {
        this._panel = panel;
        this._context = context;
        this._folder = folder;
        this._actions = actions;
        this._busy = false;
        this._render('正在读取后端配置…');
        panel.onDidDispose(() => {
            this._disposed = true;
            if (AIConfigPanel.currentPanel === this) AIConfigPanel.currentPanel = undefined;
        }, null, context.subscriptions);
        panel.webview.onDidReceiveMessage(message => this._handleMessage(message), null, context.subscriptions);
    }

    async _loadConfiguration() {
        this._configurationData = undefined;
        const { model, key } = await readModelConfiguration(this._folder, this._actions.cli, this._context.secrets);
        this._configurationData = { model, credentialBound: !!key };
    }

    async _handleMessage(message) {
        const action = message?.action;
        if (this._busy || this._disposed || !['save', 'testConnection', 'refresh'].includes(action)) return;
        this._busy = true;
        this._render('正在处理…');
        let notice = '';
        try {
            let result;
            if (action === 'save') result = await this._actions.configure(this._folder);
            if (action === 'testConnection') result = await this._actions.testConnection(this._folder);
            await this._loadConfiguration();
            if (action === 'save' && result === true) {
                if (!this._configurationData.credentialBound || !this._configurationData.model.enabled) {
                    throw new Error('后端配置或密钥绑定未就绪，请重新配置。');
                }
                notice = '配置与密钥绑定已保存。';
            }
            if (action === 'testConnection' && result) {
                notice = `连接测试成功。固定探针使用了 ${result.actual_tokens} tokens。`;
            }
        } catch (error) {
            if (action !== 'refresh') {
                // A failed save may have rolled back; show the actual state.
                try { await this._loadConfiguration(); } catch { /* Status remains unknown. */ }
            }
            notice = `操作失败：${error.message}`;
        } finally {
            this._busy = false;
            this._render(notice);
        }
    }

    _render(notice = '') {
        if (this._disposed) return;
        const data = this._configurationData;
        const model = data?.model;
        const status = !data ? '配置状态未知' : !data.credentialBound ? '需要配置并绑定 API Key'
            : model.enabled ? '已配置并启用' : '已配置，模型分析已暂停';
        const disabled = this._busy ? ' disabled' : '';
        this._panel.webview.html = page('API 配置', `
            <p>工作区：${escapeHTML(this._folder.name)}</p>
            <p>${escapeHTML(status)}</p>
            ${model ? `<p>Provider：${escapeHTML(model.provider || '未配置')}<br>
                Base URL：${escapeHTML(model.base_url || '未配置')}<br>
                模型 ID：${escapeHTML(model.model || '未配置')}</p>` : ''}
            <p>配置向导将查询所选 API 的实际模型列表，选择模型并安全保存。测试连接会确认当前 Provider、Base URL 和模型 ID。</p>
            <button data-action="save"${disabled}>配置 API / 查询并选择模型</button>
            <button data-action="testConnection"${disabled}>测试已保存的连接</button>
            <button data-action="refresh"${disabled}>刷新配置状态</button>
            <p role="status">${escapeHTML(notice)}</p>
        `, crypto.randomBytes(18).toString('hex'));
    }
}

module.exports = { AIConfigPanel };
