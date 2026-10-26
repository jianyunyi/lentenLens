# 优化与修复设计

更新：2026-10-26；未关闭阻断项：无；当前：S5 本地验证通过，远程门禁见关联 PR checks；延期风险：真实目标机器长期测量、团队试用、OS 沙箱；待决策：无；下一步：真实目标机器与团队试用。

| 问题 | 验收 | 级别/类型 | 根因与复现证据 | 方案 | 状态 |
| --- | --- | --- | --- | --- | --- |
| ISSUE-001 | AC-001 | S0 错误 | O_EXCL 文件锁不会随进程退出释放，遗留 capture.lock 阻断 Open | OS 锁；持有者异常退出测试 | 已关闭 |
| ISSUE-002 | AC-002 | S1 错误 | scrubExcluded 解析一条坏解释时直接返回，Open 全部失败 | 锁内按记录类型隔离，配置/用量/日志失败关闭 | 已关闭 |
| ISSUE-003 | AC-003 | S0 风险 | O_TRUNC 原地写入；回滚错误丢弃；无进程崩溃恢复 | 先持久化日志；临时文件替换；幂等恢复和冲突拒绝 | 已关闭 |
| ISSUE-004 | AC-004 | S1 错误 | Apply 仍用 Take；attachRoundAnalysis 与详情有两套规则 | Go-only 应用；统一 freshness | 已关闭 |
| ISSUE-005 | AC-005 | S1 限制 | Windows 运行证据曾缺失 | main 3475922 / CI 40316638711：Windows 253 次通过、0 跳过，候选包及篡改拒绝通过 | 已关闭 |
| ISSUE-006 | AC-006、AC-007/008 | S2 风险 | List 逐轮扫描全部解释、纠错和工作区；同机 1000 轮含解释基线中位 53.142 秒 | 请求内批量关联；不新增持久索引；原始前后数据及规模回归 | 重复关联根因已修复（本地验证）；远程门禁见 PR |

同类路径排查：rg 检查 internal/app 的 lock/read/write、scrubExcluded、scrubExercises、PrepareExerciseApply、ConfirmExerciseApply、attachRoundAnalysis、Inspect 及 CLI/扩展调用；原 state.write 已有临时文件+Sync，但改为 rooted 替换。settings/usage 不隔离，避免绕过授权/预算；apply-journal 损坏不隔离，避免失去恢复依据。既有模型与共享逻辑保持原约束。

替代方案：PID 检测删除锁存在 PID 重用/检查删除竞态，弃用；直接覆盖并尽力回滚无法处理进程终止，弃用；Go 1.24 自建跨平台 renameat/Windows NT 封装维护成本高，采用 Go 1.25 标准库。

回退：代码可回退；升级前停止旧进程。已开始的应用必须先恢复，不能删除日志强行继续。隔离内容不作为活动证据自动恢复。

修复轮次：首次实现不计返工；后续失败/独立审查驱动的修改在 stage-report 中追加，原始失败证据保留。


独立复核发现的四项修复均经新增/独立测试关闭：空白 null 和字段缺失形状、事务前暂存路径持久化与回收、state 写锁协调私有暂存回收、Chmod 对抗 umask。关键读取也执行授权/用量形状校验。读取上限 256 MiB，避免合法 before/after 文本 JSON 转义接近 192 MiB 时被较小上限误隔离。

B-001 已解除：2026-10-26 用户明确批准“上传”，35 个文件已上传至 fix/production-reliability，草稿 PR #5 已创建。先前审批拒绝保留为历史，不再视为当前授权阻塞。

## S5 / ISSUE-006 修复方案（进入实现前冻结）
授权：用户“继续未完成的”，本次处理已有历史扩展性能项。调用方检查：rg attachRoundAnalysis 只有 List 一处；List 对每个 round 重读目录、所有 explanation，再单独扫描 correction 和捕获 Go 证据，造成 R×E 文件读取。scrubExcluded/CLI Open 也有扫描，暂保留其安全行为。改为单次请求内的 round→最新解释、explanation→纠错关联；诊断读取一次，Go 快照一次，未使用持久缓存，不跳过撤销清理。替代方案：SQLite/持久索引需迁移和一致性治理，本轮不引入；直接缓存上次结果无法保证源码/策略实时性，弃用。验证 AC-007–009；未测量前只记录预期收益。

历史说明：本文件前文中“待验/上传阻塞/草稿”叙述是旧检查点，不代表当前状态；当前问题总表为权威状态。

ISSUE-006 范围说明：本轮关闭的是逐轮全目录/解释/纠错重复扫描；完整历史返回体、Open/recover/scrub 的线性工作及真实长期预算仍是延期限制，不据此宣称所有规模均达标。
