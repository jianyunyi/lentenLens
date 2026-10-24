'use strict';

function createCaptureRecovery({ vscode, cli, sessions }) {
  function activeOf(state) {
    // capture-status emits CaptureState directly (or JSON null), not an
    // { active: ... } envelope. Keep this boundary strict so schema drift
    // cannot silently turn an active round into "nothing to recover".
    const active = state;
    if (active == null) return null;
    if (typeof active.id !== 'string' || !active.id || typeof active.session !== 'string' || !active.session.trim() || typeof active.started !== 'string') throw new Error('轮次状态无效；没有更改本地会话或基线。');
    return active;
  }
  async function current(folder) { return activeOf(await cli(folder, 'capture-status')); }
  async function unchanged(folder, expected) {
    const active = await current(folder);
    if (!active || active.id !== expected.id || active.session !== expected.session || active.started !== expected.started) throw new Error('待处理轮次已变化，请重新查看并确认。');
    return active;
  }
  async function resume(folder) {
    const active = await current(folder);
    if (!active) { await vscode.window.showInformationMessage('IntentLens：当前没有未结束的轮次。'); return; }
    const action = '恢复此轮次';
    const answer = await vscode.window.showWarningMessage('恢复当前未结束的轮次？', { modal: true, detail: `轮次 ${active.id}，开始于 ${active.started}。它可能由 IDE、Skill 或其它工具启动；请先确认没有其它操作仍在使用它。恢复只关联本地会话，不修改基线，也不确认变更作者。` }, action);
    if (answer !== action) return;
    await unchanged(folder, active);
    const adopted = await sessions.adopt(folder.uri.fsPath, active.session);
    if (!adopted) throw new Error('已有轮次操作正在执行，请完成后重试恢复。');
    // If another process changed the baseline while local state was saved, do not
    // report success or retain a knowingly invalid association.
    try { await unchanged(folder, active); }
    catch (error) { await sessions.release(folder.uri.fsPath, active.session); throw error; }
    await vscode.window.showInformationMessage('IntentLens：已恢复本地轮次关联。保存修改后可结束记录；未调用模型。');
    return active;
  }
  async function cancel(folder) {
    const active = await current(folder);
    if (!active) { await vscode.window.showInformationMessage('IntentLens：当前没有可取消的轮次。'); return; }
    const action = '丢弃此轮次基线';
    const answer = await vscode.window.showWarningMessage('取消当前轮次并丢弃起始基线？', { modal: true, detail: `轮次 ${active.id}，开始于 ${active.started}。仅删除这条未结束的捕获基线，不撤销源码，不删除已完成的历史，也不取消正在运行的其它工具任务。丢弃后不能补造本轮净变化。请确认没有其它操作仍在使用它。` }, action);
    if (answer !== action) return;
    await unchanged(folder, active);
    await cli(folder, 'capture-abort', { session: active.session, round_id: active.id, confirm: true });
    try { await sessions.release(folder.uri.fsPath, active.session); }
    catch (cause) { throw new Error(`该轮次基线已取消，但本地关联未清理；请重试或重新开始。${cause.message}`); }
    await vscode.window.showInformationMessage('IntentLens：该轮次基线已取消；源码和已完成历史未删除。');
    return active;
  }
  return { resume, cancel };
}

module.exports = { createCaptureRecovery };
