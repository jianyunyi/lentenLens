# 优化与修复设计

更新：2026-10-26；未关闭阻断项：ISSUE-005（待远程授权/Windows 运行）；延期风险：Windows 实测、试用、沙箱；待决策：无；下一步：审阅差异后明确授权远程上传。

| 问题 | 验收 | 级别/类型 | 根因与复现证据 | 方案 | 状态 |
| --- | --- | --- | --- | --- | --- |
| ISSUE-001 | AC-001 | S0 错误 | O_EXCL 文件锁不会随进程退出释放，遗留 capture.lock 阻断 Open | OS 锁；持有者异常退出测试 | 已关闭 |
| ISSUE-002 | AC-002 | S1 错误 | scrubExcluded 解析一条坏解释时直接返回，Open 全部失败 | 锁内按记录类型隔离，配置/用量/日志失败关闭 | 已关闭 |
| ISSUE-003 | AC-003 | S0 风险 | O_TRUNC 原地写入；回滚错误丢弃；无进程崩溃恢复 | 先持久化日志；临时文件替换；幂等恢复和冲突拒绝 | 已关闭 |
| ISSUE-004 | AC-004 | S1 错误 | Apply 仍用 Take；attachRoundAnalysis 与详情有两套规则 | Go-only 应用；统一 freshness | 已关闭 |
| ISSUE-005 | AC-005 | S1 限制 | Windows 运行证据缺失 | 已实现并静态/独立检查门禁；远程上传遭自动审批拒绝 | 阻塞 |
| ISSUE-006 | AC-006 | S2 风险 | 1000 轮合成数据中 rounds P95 496 ms，历史扫描增长明显 | 保留原始数据，团队基线后决定索引化/分页；不宣称长期性能通过 | 延期 |

同类路径排查：rg 检查 internal/app 的 lock/read/write、scrubExcluded、scrubExercises、PrepareExerciseApply、ConfirmExerciseApply、attachRoundAnalysis、Inspect 及 CLI/扩展调用；原 state.write 已有临时文件+Sync，但改为 rooted 替换。settings/usage 不隔离，避免绕过授权/预算；apply-journal 损坏不隔离，避免失去恢复依据。既有模型与共享逻辑保持原约束。

替代方案：PID 检测删除锁存在 PID 重用/检查删除竞态，弃用；直接覆盖并尽力回滚无法处理进程终止，弃用；Go 1.24 自建跨平台 renameat/Windows NT 封装维护成本高，采用 Go 1.25 标准库。

回退：代码可回退；升级前停止旧进程。已开始的应用必须先恢复，不能删除日志强行继续。隔离内容不作为活动证据自动恢复。

修复轮次：首次实现不计返工；后续失败/独立审查驱动的修改在 stage-report 中追加，原始失败证据保留。


独立复核发现的四项修复均经新增/独立测试关闭：空白 null 和字段缺失形状、事务前暂存路径持久化与回收、state 写锁协调私有暂存回收、Chmod 对抗 umask。关键读取也执行授权/用量形状校验。读取上限 256 MiB，避免合法 before/after 文本 JSON 转义接近 192 MiB 时被较小上限误隔离。
