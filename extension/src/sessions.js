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
    if (typeof value !== 'string' || !value.trim()) return undefined;
    try {
      const parsed = JSON.parse(value);
      if (parsed && typeof parsed.session === 'string' && parsed.session.trim()) return { session: parsed.session, roundID: typeof parsed.round_id === 'string' ? parsed.round_id : undefined };
    } catch { /* Migrate the earlier session-only value below. */ }
    return { session: value, roundID: undefined };
  }
  async function update(root, value) {
    await workspaceState.update(rootKey(root), value ? JSON.stringify({ session: value.session, round_id: value.roundID || undefined }) : undefined);
  }
  async function run(root, operation) {
    if (busy.has(root)) return undefined;
    busy.add(root);
    try { return await operation(); } finally { busy.delete(root); }
  }
  async function allRoundSummaries(root) {
    const items = [];
    let cursor;
    do {
      const page = await cli(root, 'rounds', { cursor, limit: 50 });
      if (!page || !Array.isArray(page.items)) return undefined;
      items.push(...page.items);
      cursor = typeof page.next_cursor === 'string' && page.next_cursor ? page.next_cursor : undefined;
    } while (cursor);
    return items;
  }
  async function finished(root, session, roundID) {
    const known = completed.get(root);
    if (known?.session === session && (!roundID || known.roundID === roundID)) return known.result;
    const rounds = await allRoundSummaries(root);
    if (!rounds) return undefined;
    const candidates = rounds.filter(round => round.session === session && (!roundID || round.id === roundID));
    return candidates.length === 1 ? candidates[0] : undefined;
  }
  function validReceipt(result, session, roundID) {
    return !!result && result.id === roundID && result.session === session &&
      typeof result.started === 'string' && typeof result.ended === 'string' &&
      Number.isInteger(result.change_count) && result.change_count >= 0 &&
      Number.isInteger(result.omitted_count) && result.omitted_count >= 0;
  }
  async function start(root) {
    return run(root, async () => {
      let state = await read(root);
      let isNew = false;
      // A prior stop may have succeeded even though clearing local state failed.
      // Never reuse that session for a new baseline after a reload.
      if (state && await finished(root, state.session, state.roundID)) {
        await update(root, undefined);
        completed.delete(root);
        state = undefined;
      }
      if (!state) {
        state = { session: random() };
        isNew = true;
        try {
          await update(root, state);
        } catch (cause) {
          const message = `无法保存轮次会话；未执行开始操作。请检查 VS Code 工作区状态后重试（${cause.message}）。`;
          error(message);
          throw new Error(message, { cause });
        }
      }
      try {
        // Recover a start whose response or round-ID persistence was lost.
        let active;
        if (!isNew) {
          try { active = await cli(root, 'capture-status'); } catch { /* A fresh start can still proceed. */ }
        }
        let result;
        if (active?.session === state.session && typeof active.id === 'string' && active.id) {
          result = active;
        } else {
          result = await cli(root, 'start', { session: state.session });
        }
        if (typeof result?.id !== 'string' || !result.id) throw new Error('开始响应缺少轮次 ID');
        state = { session: state.session, roundID: result.id };
        await update(root, state);
        info('IntentLens：轮次已开始。请记得保存文件；扩展不会自动分析或请求模型。');
        return { session: state.session, roundID: state.roundID, result };
      } catch (cause) {
        const message = `开始轮次失败，会话已保留，可直接重试；未丢失会话（${cause.message}）。`;
        error(message);
        throw new Error(message, { cause });
      }
    });
  }
  async function stop(root) {
    return run(root, async () => {
      let state = await read(root);
      if (!state) {
        const message = '没有可结束的轮次会话；请先开始轮次。';
        error(message);
        throw new Error(message);
      }
      if (!state.roundID) {
        const active = await cli(root, 'capture-status');
        if (active?.session === state.session && typeof active.id === 'string' && active.id) {
          state = { session: state.session, roundID: active.id };
          await update(root, state);
        } else {
          const rounds = await allRoundSummaries(root);
          const candidates = rounds ? rounds.filter(round => round.session === state.session) : [];
          if (candidates.length !== 1) throw new Error('旧轮次会话缺少轮次 ID，且无法唯一恢复；请从轮次视图确认后再恢复。');
          state = { session: state.session, roundID: candidates[0].id };
          await update(root, state);
        }
      }
      const { session, roundID } = state;
      let result = completed.get(root)?.session === session && completed.get(root)?.roundID === roundID ? completed.get(root).result : undefined;
      try {
        if (!result) result = await cli(root, 'stop', { session, round_id: roundID });
        if (!validReceipt(result, session, roundID)) throw new Error('结束回执无效或与当前轮次不匹配');
      } catch (cause) {
        // Reconcile only a completed round for our exact persisted random session.
        // This also recovers a response lost after the CLI already saved the round.
        try { result = await finished(root, session, roundID); } catch { /* retain original failure */ }
        if (!validReceipt(result, session, roundID)) {
          const message = `结束轮次失败，会话仍保留，可重试；未丢失会话（${cause.message}）。`;
          error(message);
          throw new Error(message, { cause });
        }
      }
      completed.set(root, { session, roundID, result });
      try {
        await update(root, undefined);
      } catch (cause) {
        const message = `轮次已由 CLI 成功结束，但无法清除本地会话；请再次执行结束命令重试清理（${cause.message}）。`;
        error(message);
        throw new Error(message, { cause });
      }
      completed.delete(root);
      info('IntentLens：轮次已结束，已记录当时磁盘上的内容。可在轮次视图中查看；不会自动分析或请求模型。');
      return { session, roundID, result };
    });
  }
  async function adopt(root, session, roundID) {
    if (typeof session !== 'string' || !session.trim()) throw new Error('恢复需要有效会话。');
    return run(root, async () => {
      await update(root, { session, roundID });
      completed.delete(root);
      return { session };
    });
  }
  async function release(root, session) {
    if (typeof session !== 'string' || !session.trim()) throw new Error('清理需要有效会话。');
    if (busy.has(root)) throw new Error('已有轮次操作正在执行，请完成后重试清理。');
    return run(root, async () => {
      if ((await read(root))?.session !== session) return { released: false };
      await update(root, undefined);
      if (completed.get(root)?.session === session) completed.delete(root);
      return { released: true };
    });
  }
  return { start, stop, adopt, release, get: async root => (await read(root))?.session, key: rootKey };
}

module.exports = { createSessions, rootKey };
