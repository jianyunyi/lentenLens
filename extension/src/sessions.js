'use strict';

const crypto = require('node:crypto');

const KEY_PREFIX = 'intentlens.session.';

function rootKey(root) {
  return KEY_PREFIX + crypto.createHash('sha256').update(String(root)).digest('hex').slice(0, 32);
}

function createSessions({ workspaceState, cli, notify = {}, random = () => crypto.randomBytes(16).toString('hex') }) {
  if (!workspaceState || typeof workspaceState.get !== 'function' || typeof workspaceState.update !== 'function') throw new TypeError('workspaceState is required');
  if (typeof cli !== 'function') throw new TypeError('cli is required');
  const busy = new Set();
  const completed = new Map();
  const info = typeof notify.info === 'function' ? notify.info : () => {};
  const error = typeof notify.error === 'function' ? notify.error : () => {};

  async function read(root) {
    const value = await workspaceState.get(rootKey(root));
    return typeof value === 'string' && value.trim() ? value : undefined;
  }
  async function update(root, value) {
    await workspaceState.update(rootKey(root), value || undefined);
  }
  async function run(root, operation) {
    if (busy.has(root)) return undefined;
    busy.add(root);
    try { return await operation(); } finally { busy.delete(root); }
  }
  async function finished(root, session) {
    if (completed.get(root)?.session === session) return completed.get(root).result;
    const rounds = await cli(root, 'rounds');
    return Array.isArray(rounds) ? rounds.find(round => round.session === session) : undefined;
  }
  async function start(root) {
    return run(root, async () => {
      let session = await read(root);
      // A prior stop may have succeeded even though clearing local state failed.
      // Never reuse that session for a new baseline after a reload.
      if (session && await finished(root, session)) {
        await update(root, undefined);
        completed.delete(root);
        session = undefined;
      }
      if (!session) {
        session = random();
        try {
          await update(root, session);
        } catch (cause) {
          const message = `无法保存轮次会话；未执行开始操作。请检查 VS Code 工作区状态后重试（${cause.message}）。`;
          error(message);
          throw new Error(message, { cause });
        }
      }
      try {
        const result = await cli(root, 'start', { session });
        info('IntentLens：轮次已开始。请记得保存文件；扩展不会自动分析或请求模型。');
        return { session, result };
      } catch (cause) {
        const message = `开始轮次失败，会话已保留，可直接重试；未丢失会话（${cause.message}）。`;
        error(message);
        throw new Error(message, { cause });
      }
    });
  }
  async function stop(root) {
    return run(root, async () => {
      const session = await read(root);
      if (!session) {
        const message = '没有可结束的轮次会话；请先开始轮次。';
        error(message);
        throw new Error(message);
      }
      let result = completed.get(root)?.session === session ? completed.get(root).result : undefined;
      try {
        if (!result) result = await cli(root, 'stop', { session });
      } catch (cause) {
        // Reconcile only a completed round for our exact persisted random session.
        // This also recovers a response lost after the CLI already saved the round.
        try { result = await finished(root, session); } catch { /* retain original failure */ }
        if (!result) {
          const message = `结束轮次失败，会话仍保留，可重试；未丢失会话（${cause.message}）。`;
          error(message);
          throw new Error(message, { cause });
        }
      }
      completed.set(root, { session, result });
      try {
        await update(root, undefined);
      } catch (cause) {
        const message = `轮次已由 CLI 成功结束，但无法清除本地会话；请再次执行结束命令重试清理（${cause.message}）。`;
        error(message);
        throw new Error(message, { cause });
      }
      completed.delete(root);
      info('IntentLens：轮次已结束，已记录当时磁盘上的内容。可在轮次视图中查看；不会自动分析或请求模型。');
      return { session, result };
    });
  }
  async function adopt(root, session) {
    if (typeof session !== 'string' || !session.trim()) throw new Error('恢复需要有效会话。');
    return run(root, async () => {
      await update(root, session);
      completed.delete(root);
      return { session };
    });
  }
  async function release(root, session) {
    if (typeof session !== 'string' || !session.trim()) throw new Error('清理需要有效会话。');
    if (busy.has(root)) throw new Error('已有轮次操作正在执行，请完成后重试清理。');
    return run(root, async () => {
      if (await read(root) !== session) return { released: false };
      await update(root, undefined);
      if (completed.get(root)?.session === session) completed.delete(root);
      return { released: true };
    });
  }
  return { start, stop, adopt, release, get: read, key: rootKey };
}

module.exports = { createSessions, rootKey };
