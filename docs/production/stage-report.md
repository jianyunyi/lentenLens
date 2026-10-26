# 可靠性阶段报告

当前阶段：S5 代码与本地验收交付；状态：通过；未关闭阻断项：无；延期风险：真实团队试用、目标机器长期测量、完整历史线性增长、OS 沙箱；待决定：无；下一步：关联 PR 远程 CI，然后由参与者执行真实试用。远程持续门禁结果以关联 PR checks 为准。

基线 main 53d1720；对应本地源码快照 HEAD 00487c4，102 个文本文件经上一轮逐文件 blob 对照；发布二进制不纳入本地源码快照。

## 历史记录（以下 S1–S4 与上传段落保留当时结果，不代表当前未关闭状态）

## S1 基线
故障探针已确认遗留 capture.lock 和坏解释 JSON 均阻断 Open；源码 O_TRUNC 风险经静态确认。当前先冻结 AC-001–006，不以删减测试调整验收。
检查点：基线快照 + git diff；告警：无；需要决定：无；未做：真实状态迁移、公开发行、团队联系。

## 2026-10-26 · S1 · 异常恢复与安全应用
结论：通过；独立复核：review_reliability（未参与实现）通过。需要决定：无。
AC-001/002/003 smoke：Linux 全量 go test ./... -count=1、go vet ./...、go build ./... PASS；针对性 -race PASS；Windows amd64 build PASS（运行 NOT RUN）。
故障验证：真实子进程强退后锁释放；替换后和 Rename 前退出后重开恢复；外部修改拒绝覆盖；写失败回滚；隔离与策略撤销；critical null/shape 拒绝；umask 权限保留。
独立复核发现并关闭四项：空白/null/缺字段用量接受、源码暂存残留、私有暂存残留、umask 权限改变；补充进程崩溃与安全形状回归。返工：5 轮修改→复验（复核修复后字段缺失编译失败、补字段通过、暂存/权限补齐通过、critical shape 修复后函数重名编译失败、改名通过）；阻塞 0 次。
度量（对 HEAD 的本阶段 diff，报告追加前）：{'docs': [6, 72, 3], 'implementation': [13, 748, 78], 'tests': [2, 423, 0]}
检查点：基线 HEAD + S1 工作区，文件摘要由阶段前后 git diff/本地检查点确定；未部署、未合并。未做：Windows 实跑、断电/文件系统故障注入。风险：比较到替换没有跨编辑器 CAS；应用前关闭并发编辑，不能承诺任意不合作 writer 的绝对保护。
新增状态恢复处理的代价：启动扫描记录和 OS 锁；性能影响由 S4 基准记录。

## 2026-10-26 · S2 · 预算与新鲜度一致性
结论：通过；需要决定：无；独立复核：不适用（复用 S1 已复核恢复路径）。
实现：PrepareExerciseApply 使用 TakeGo；轮次统一 freshness；Inspect 复用现有 Go snapshot 调用同一 freshnessFromSnapshot，避免重复扫描；缺少证据不再显示新鲜。
AC-004 + 前序 smoke：新增三项及函数/练习预算、非 Go 证据、锁、崩溃、暂存清理回归共 9 个顶层测试 PASS；全量 app 测试 PASS。
度量（S1 检查点到 S2，报告自身更新另计）：{'internal/app/privacy.go': [1, 12], 'internal/app/consistency_test.go': [108, 0], 'docs/production/stage-report.md': [9, 0], 'internal/app/exercise_execution.go': [1, 1]}
返工 0 轮；阻塞 0 次；告警/新错误：本轮检查未发现。未做：大仓库端到端 UI 时延测量，归 S4。检查点：基线 + S1/S2 diff，回退需先处理残留应用日志。

## 2026-10-26 · S3 · Windows CI 与候选包检查
结论：实现与本地检查通过，远程运行阻塞；独立复核：review_reliability。
已完成：只读权限、不可变 action SHA、Windows/Linux 测试与 vet/build、Windows 冒烟、最低 Go 版本检查、候选 VSIX 内容校验及 core.js 修改/清单缺失拒绝测试。未加入自动发布。
本地 actionlint、JS 语法、61 个扩展测试、7 个预检测试 PASS。独立故障探针 5 类（正常、失败事件、非零退出、无测试、坏 JSON）正确传递退出状态；真实 CLI 三轮 soak 集成 PASS。
AC-005 Windows 实跑/PowerShell 打包/候选包验证：NOT RUN。Linux 不代替 Windows；尚未修改 main 必需检查规则。候选包自带哈希不等于任意恶意包签名认证。
阻塞事件 B-001：GitHub create_tree 批量上传 32 个源码/项目文件被 automatic approval review 拒绝，理由为本轮未明确授权该载荷的远程导出及目标信任；未创建远程分支/PR，未用其他通道绕过。此前仓库 PR 创建授权未被此轮自动审批接受，需要用户针对本批差异明确确认。
返工 0 轮；阻塞 1 次（授权）；需要决定：允许将可审阅的完整差异上传到 jianyunyi/lentenLens 新分支并创建草稿 PR。

## 2026-10-26 · S4 · 试用准备与合成性能基线
结论：本地基线和试用协议完成；真实团队与长期验收 NOT RUN，不宣称生产就绪。
产物：team-trial.md、recovery-runbook.md、performance_test.go、soak.cjs/soak.test.cjs；原始数据位于 evidence/。
Linux/AMD EPYC 9V74/Go 1.25.0：1000 个 Go 文件索引 21.5–23.6 ms/op；Inspect 25.1–35.4 ms/op。20 条历史 List 0.925–0.950 ms/op，200 条 22.7–26.3 ms/op。每组 1 次迭代、3 次重复，是初始测量，不是稳定统计或目标 Windows SLA。
100 轮合成场景 PASS，3.34 秒；1000 轮 PASS，299.83 秒，0 模型调用，最终状态 780525 字节。1000 轮的命令 P95：start 40.0 ms、stop 40.0 ms、index 38.0 ms、inspect 55.0 ms、rounds 496.2 ms。每轮启动独立 CLI 进程，含逐次增长的历史；不能解释为固定规模下的延迟分布。报告的 RSS 是 Node harness，不能当 CLI 峰值内存。
发现历史扩展成本增长，新增 ISSUE-006；尚未在真实目标机器冻结 SLA，没有以宽松阈值伪造通过。
试用方案建议 3–5 位用户、2 周观察，具体人员/日期/阈值未确定；本任务未联系任何人或开启真实源码/模型试用。AC-006 合成检查 PASS，真实验收 NOT RUN。返工 1 轮（基准输出行尾空白导致 diff --check 失败后规范化复验）；依赖阻塞沿用 B-001，不重复计数。

S2 度量补充校正：原检查点字典仅含 S1 已改文件，漏列 S2 首次修改的 inspect/revisions；完整源码/测试差异为 {'internal/app/consistency_test.go': [108, 0], 'internal/app/inspect.go': [2, 14], 'internal/app/privacy.go': [1, 12], 'internal/app/exercise_execution.go': [1, 1], 'internal/app/revisions.go': [7, 0]}。该校正只补齐度量，不改变测试与验收标准。

## 最终本地复验与交付
Go 1.25.0/Linux：129 个顶层测试 PASS、1 个 Windows 专属用例 SKIP；计入子测试 246 次 PASS。go vet/build PASS；Windows app 测试二进制交叉编译 PASS（实际运行 NOT RUN）。扩展 61/61、预检 7/7、真实 CLI soak 集成 1/1 PASS；actionlint 与 git diff --check PASS。
后续小范围加固：授权/用量形状校验同时用于关键 read；单条读取上限调整为 256 MiB，并运行 critical/quarantine/crash 定向测试及上述全量复验。
基准原始输出仅规范化了行尾空白，数值未改变。soak 报告记录所测二进制 SHA-256；长短两次跑相同二进制，后续关键 read 加固另经定向和全量复验，不把它们冒充不同版本的性能对比。
S3/S4 按交付物统计：CI 工作流/执行器/PowerShell 测试、Go 基准、soak 执行器及集成测试；具体文件增删行见交付包 diffstat.json。阶段隔离提交 N/A（采用连续工作区和 S1 源码检查点，未创建虚假提交）；最终完整差异与 SHA-256 清单均在交付包，排除工具下载/二进制/私有 fixture。
告警：npm 对运行环境 http-proxy 配置发出弃用提示，测试未失败；Windows/候选包/真实试用状态未虚报。未完成项：B-001 上传授权、ISSUE-005 Windows 实跑、真实长期验收、ISSUE-006 扩展性能；不对生产就绪作承诺。

## 2026-10-26 · 上传检查点
用户明确回复“上传”，授权本批载荷写入 jianyunyi/lentenLens 新分支并创建草稿 PR，B-001 已解除。已建立 fix/production-reliability，源码提交 84ae463c550d066a248c224268bd51639c24cbee，草稿 PR https://github.com/jianyunyi/lentenLens/pull/5 。远程比较：35 个文件、1977 行增加/107 行删除，与已审阅交付范围一致；未删除额外路径，未合并 main。
本补充仅更新三份阶段文档的当前授权状态，代码及本地测试结论不变。首次查询尚无 workflow run 记录，Windows/PowerShell/候选包与真实团队试用仍不标 PASS。

## 2026-10-26 · S3 远程验收关闭
PR #5 已合并，main 3475922f8fc60f4d52250e679fa1e6a756ffe134，push CI https://github.com/jianyunyi/lentenLens/actions/runs/40316638711 全部成功。Windows 253 次通过、0 跳过；vet/build、扩展 61 项、预检、soak、CLI 冒烟、最低 Go 和 Linux/race 均 PASS。候选 VSIX 校验 11 项包内容、71 项 Go 源文件，正常包通过/源码篡改拒绝/缺失清单拒绝全部 PASS。ISSUE-005 已关闭；B-001 已解除，均不再阻塞。

## 2026-10-26 · S5 · 历史查询批量关联
结论：本地实现与语义回归通过；1000 轮合成结果追加于下文；远程状态以关联 PR checks 为准。独立复核：未独立复核（本轮 Standard 自审）；需要决定：无。
基线 main 3475922；本地重建源码检查点 09e288c，仅用于本地版本定位，不冒充 GitHub 提交。before 基准为该源码加同一 BenchmarkRoundHistoryJoin；after 的代码树摘要见 evidence/history-environment.json。
实现：List 单次目录快照内关联最新解释和纠错，单读分析诊断，Go 捕获在本次请求内复用。每次调用仍 scrubExcluded，读取所有当前记录，非 Go 证据保留既有保守规则；无状态格式迁移、无跨请求缓存。代价：请求内关联 map；返回的完整历史仍随记录量增长，scrub/recover 的扫描未跳过，非 Go 证据仍可能各次捕获通用快照。
验收映射：AC-008 → TestRoundHistoryBatchPreservesLatestCorrectionsAndDiagnostics、TestRoundHistoryDoesNotCacheUnreadableExplanation；前序 smoke → 原有函数/轮次预算、删除依赖、非 Go 证据、撤销、恢复、应用与分享测试。语义覆盖最新版本、排序、纠错标记及隔离、诊断归属、跨调用源码修改与撤销、坏记录拒绝。
本地 go test ./... -count=1、go vet ./...、go build ./...、定向 race 均 PASS；扩展和压力结果追加于下文。无新增依赖。非法压力轮数 0 的 CLI 入口检查正确拒绝。
测量：Go 1.26.3/Linux/Xeon Platinum 8573C，同一 benchmark 每组 1 次迭代、3 次重复。含解释/纠错 1000 轮时延中位 53.142 秒 → 0.250 秒；累计分配中位 5,434,673,816 → 20,398,952 字节。无解释 1000 轮 897.2 → 37.6 ms，153,384,016 → 5,076,504 字节。200 轮含解释 1.949 秒 → 65.3 ms。20 轮含解释 14.4 → 15.6 ms，存在小规模样本波动。共享宿主机、未隔离系统负载，样本不能证明统计显著性，更不是 Windows 目标机器 SLA。完整原始数据保留，未隐藏小规模回退样本。
CI 增加 Windows/Linux 的 history-acceptance 入口，三次规模基准 + 100 轮子进程压力检查，上传源码摘要、环境与原始结果。默认本地入口 1000 轮；只能操作合成临时项目，不收集用户源码/密钥。
范围限制：真实团队、真实目标 Windows 机器和两周长期观察仍 NOT RUN，不以 CI 虚拟机替代。下一步由所有者选定参与者、目标机器与观测日期后冻结预算。

### S5 本地验收与度量
1000 轮合成压力 PASS：280.71 秒，rounds P95 171.95 ms，状态 780507 字节，模型调用 0；二进制 SHA-256 为 2ed38c532c2263f364378170f492fb2e1a5204c1dd3729f64b45c03fd8b34c83。环境及源码树摘要见 history-environment.json；原始报告 history-soak-1000.json。未在同一环境复跑旧 CLI 的端到端压力，不把这份时延与旧 EPYC/Go 1.25 的 496 ms 直接相除。RSS 仅为 Node harness。

| 检查 | 状态 | 证据/操作 |
| --- | --- | --- |
| AC-007 前后同机规模基准 | PASS | history-before.txt / history-after.txt；1000 轮两类中位时延与累计分配均降低 |
| AC-008 最新解释/纠错/诊断/跨请求撤销与坏记录 | PASS | TestRoundHistoryBatchPreservesLatestCorrectionsAndDiagnostics、TestRoundHistoryDoesNotCacheUnreadableExplanation |
| smoke 全量 Go | PASS | go test ./... -count=1 |
| smoke race | PASS | go test -race ./internal/app -run 'TestRound\|TestFunctionFreshness\|TestNonGoRound\|TestDeletedDependency\|TestIgnoreChange\|TestRevocation' -count=1 |
| smoke vet/build | PASS | go vet ./...；go build ./... |
| 扩展 | PASS | npm test：61/61；npm run check |
| AC-009 合成 1000 轮 | PASS | node scripts/history-acceptance.cjs 1000，原始 JSON 已保留 |
| 入口失败路径 | PASS | 参数 0 返回非零，不创建试用项目 |
| Windows/Linux 持续门禁 | NOT RUN | 此本地检查点尚未运行远程 CI；最终远程状态见关联 PR checks；包含 100 轮合成与原始 artifact |
| 真实团队/目标机器/长期观察 | NOT RUN | 缺少参与者、真实安装与持续观测数据 |

返工：0 轮（无失败检查驱动的修复；请求内 map 在自审中调整为按函数选择最新版本，以保持重复历史 ID 的关联语义）。实施阻塞 0 次；真实团队长期验收有 1 项外部输入缺口，保持 NOT RUN，不阻塞合成阶段。自审覆盖 List→隐私清理→新批量关联、详情纠错复用、CLI 与扩展调用；未独立复核。
告警：npm 环境 http-proxy 弃用提示；本轮已运行的语义/全量/race 检查未发现代码错误。
检查点：main 3475922 + 本批 diff，回退新代码即可，不改持久格式；试用恢复规则仍见 recovery-runbook.md。本轮为 Standard 连续执行，沿用用户的仓库修复/PR 授权，提交新 PR。
度量（git diff --numstat，含新增文件；排除原始测量输出；本行更新不改变行数）：实现 7 个文件 +155/-39；测试 2 个 +150/-0；文档 4 个 +70/-8。排除项：4 份 evidence 原始数据与运行工具/二进制/临时 fixture。
