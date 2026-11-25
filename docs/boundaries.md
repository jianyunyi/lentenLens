# IntentLens 第 0 步（含 0a）：边界决策

状态：**已批准（2026-11-25，用户亲口“可以继续进行”，13 条决策 D01–D13 按草稿原文全部通过）。** 本文只提出文档契约；所有“建议决策”、预算新值、状态名和协议字段均为拟议内容。批准后先做 7a/8a 契约，再进入 1–9 的实现与验收；本轮没有运行测试、模型、构建或联网。

核对基线：按任务输入使用 HEAD `8e733554f561834f565afea788a79e9cccd364cb`，未独立验证 Git 对象；文件内版本为 0.7.1（`repo/CHANGELOG.md:1`；`repo/extension/package.json:5`）。事实使用 `repo/路径:行` 引用；引用是现有落点，**不是建议功能已经存在的证明**。附录补齐评审当时缺失的正文，并区分静态核对与运行验证。

## 决策总览（每条一句话）

以下复选框供逐条批准；未勾选均为建议，不以沉默代替批准。

- [x] D01：将工作区的选区工作流定位为“选区 + 预扫关联”，热路径只采实时选区，并约束同工作区扩展后台扫描。
- [x] D02：分别约束本地读取、磁盘保留和模型接收；跨文件仅允许获准的结构化关系与短引用，禁止未选文件全文外发。
- [x] D03：纯选区默认开启，预扫保留与关联外发分别授权，旧模型授权不迁移为新授权。
- [x] D04：显式预扫/显式刷新；复用索引时承认新鲜度未知，已知过期则降级，热路径不全库 hash 或重扫。
- [x] D05：新增独立 manifest v1 与可协商的关联协议，旧 Go index 与未协商的选区契约不变。
- [x] D06：对最终完整输入预算，必需选区/诊断先占位，fast 的 8 条/8 KiB 适用于所有证据。
- [x] D07：所有派生记录保留来源全集，撤销贯穿 answer、review、面板打开与跳转，清除缓存后代。
- [x] D08：确定边、同名候选、文档提及、文档明确决策陈述分四档，不将候选洗成事实。
- [x] D09：首版不实时读取当前文件未选内容，完整文件 hash 只能在独立预扫/核验操作中取得。
- [x] D10：首版 Go + 获准 Markdown，不做全库 LLM 摘要、监听增量或共享 Inspect/CallPath 缓存改造。
- [x] D11：采用本文定稿的隐私表述及本地出处映射，默认不向模型披露路径。
- [x] D12：选区解释结果和诊断继续不保存，获准预扫缓存另行保留且可清除。
- [x] D13：以每条边界的正反契约、实际文件访问和各阶段请求体作为后续验收依据。

## 三张材料表

### D01：读取承诺的作用域

**决策（建议）**：限制覆盖“该工作区选区工作流及其引起的扩展后台读取”，不只覆盖一个 CLI handler。进入选区工作流时，暂停该工作区自动 CodeLens `index` 和侧栏 `inspect`；已有请求须取消或等待结束后才能显示“受限读取已生效”。没有可复用安全数据时展示占位，不触发现场扫描。显式预扫、显式刷新、用户另行执行的函数 Inspect/CallPath/轮次捕获是单独的操作，执行前说明会读取其他文件；不得因刷新 UI 暗中启动它们。约束不代表控制 VS Code 核心或其他扩展的文件访问。

**理由**：单命令不扫库不足以让用户感知的选区操作不扫库。

**代码依据**：CodeLens 调 `index`（`repo/extension/src/extension.js:595`、`:605`）；选区变化触发侧栏 refresh（`repo/extension/src/extension.js:918`），refresh 自身只失效并通知树，根节点读取经 150 ms 防抖才调用 target（`repo/extension/src/sidebar.js:20`、`:37`、`:63`、`:149`），target 调 `inspect`（`repo/extension/src/extension.js:633`）；二者进入 `buildIndex → TakeGo`（`repo/internal/app/inspect.go:91`、`:96`、`:148`）。

**考虑过的替代方案**：只限制 `explain-selection`，保留后台扫库，改为声明“仅此命令不扫库”；实现较小，但容易与用户原意不符。全面缓存化 Inspect/CallPath 则扩大本轮后的改造范围。

**需用户拍板的问题**：批准上述工作区级后台限制，还是只批准命令级限制？倾向前者；无需因此改动共享缓存算法，先抑制自动入口。若选择后者，D01 的 UI、I/O 验收与对外读取承诺必须一起改稿，不能两种口径都算通过。

### D02(a)：本地读取范围表

下表全部为建议契约；“读取”包括从编辑器缓冲区取得文本，不以“已在内存”规避边界。

| 材料 | 纯选区热路径 | 获准关联热路径 | 禁止或例外 | 现有代码依据/接入点 |
|---|---|---|---|---|
| 当前文件选区、语言、UTF-16 范围、保存版本 | 可读 | 可读 | 仅已保存、非空、≤16 KiB；不读取完整缓冲区求 hash | `repo/extension/src/extension.js:717`、`:722`、`:727`；`repo/internal/app/inspect.go:264`、`:327` |
| 相交 Error 诊断 | 可读 | 可读 | 不读其他文件诊断，不纳入 Warning；返回前复核 | `repo/extension/src/core.js:92`；`repo/extension/src/extension.js:741`、`:750` |
| 当前文件实时未选内容 | 不可读 | 不可读 | import、类型、相邻函数及全文 hash 均不作为隐含例外；见 D09 | `repo/extension/src/extension.js:727`、`:744`（当前只取选区） |
| 已授权、指定代际的预扫 manifest、符号/段落索引、来源映射 | 不为检索而读 | 可有界读取 | 不调用 Inspect 取关联；从缓存取同文件关系也须属于获准类型 | `repo/internal/app/inspect.go:96`、`:194`；`repo/internal/codeindex/index.go:20`（拟新增独立查询） |
| 最新 `.intentignore`、默认规则、预扫许可/外发许可策略、模型配置与用量 | 可读 | 必须读 | 来源列表逐项查策略；不能把“只读源码”解释为禁读安全策略 | `repo/internal/capture/policy.go:22`；`repo/internal/app/model_connection.go:58`；`repo/internal/app/inspect.go:445`、`:449` |
| 来源路径的安全元数据 | 可查当前路径 | 可查全部来源路径 | 允许有界 Lstat/路径安全检查；不枚举项目树，不打开来源正文、不用 mtime 冒充 hash | `repo/internal/capture/capture.go:413`、`:418`、`:397` |
| 私有状态恢复/撤销所需记录与锁 | 可读、可清理 | 可读、可清理 | 此为本地状态，不是项目源码；纯模式也可能经 Open 清理旧记录，不承诺“仅打开两个磁盘文件” | `repo/internal/app/store.go:88`；`repo/internal/app/recovery.go:58`；`repo/internal/app/privacy.go:14` |
| 其他项目源码/Markdown、目录全量枚举、依赖下载、工具执行结果 | 不可读 | 不可读 | 只有独立显式 prescan/刷新可以读取获准 Go/Markdown；用户点击跳转另见 D07 | `repo/internal/app/inspect.go:97`（必须避开的全库捕获）；`repo/internal/codeindex/index.go:106` |

**决策（建议）**：以此表为 I/O allowlist；预扫读取前按语言/文档范围和策略筛选，不能先全库 `Take` 再过滤。

**理由**：hash 由文件内容计算，热路径全 hash 与本表冲突；最新规则及路径元数据检查不需要读取来源正文。

**代码依据**：`repo/internal/capture/capture.go:54`、`:275`、`:296`；策略与安全路径分离于源码捕获（`repo/internal/capture/capture.go:405`、`:413`）。

**考虑过的替代方案**：每次全库 hash 只省解析但不省读取；stat-only 无法证明内容未变。

**需用户拍板的问题**：与 D01、D09 一并批准读取例外清单；无额外默认例外。

### D02(b)：磁盘保留范围表

| 材料 | 建议保留形态 | 不保留内容 | 期限/清除 | 现有代码依据/接入点 |
|---|---|---|---|---|
| Go 符号表 | 符号身份、范围、同包 resolved/ambiguous/unresolved 关系；本地可保留有界函数片段与调用片段用于锚定/出处 | 不另存全项目通用文件快照；不直接把公开 Index JSON 当内部缓存 | 本代际最多 7×24 小时，非滑动续期 | `repo/internal/codeindex/index.go:20`、`:29`、`:162`、`:220`（Source 是原文，不是只有名字） |
| Markdown 段落表 | 获准段落原文、标题路径、文件:行、可见日期/版本、检索词；长段按上限切分 | 不保留范围外文档，不生成 LLM 全库摘要 | 同代际期限；撤销即删 | `repo/internal/capture/capture.go:128`（文本捕获落点）；文档分段为建议新增 |
| manifest 与 hash 清单 | 文件相对路径、内容 hash、来源范围全集、coverage/遗漏、策略/版本/生成代际 | 不收录排除正文，不存 API Key、模型输出或诊断 | 与本代际一起删除，不无限留旧 hash 文件 | `repo/internal/capture/capture.go:54`；`repo/internal/codeindex/index.go:57`；`repo/internal/app/store.go:77` |
| 派生关系记录及出处映射 | 不透明证据 ID → 来源集合（路径、hash、行/字符范围）及上游 ID | 不把单一“展示路径”冒充完整来源 | 任一来源撤销即删该记录及后代；保守删除整个代际亦可 | `repo/internal/explain/explain.go:79`（现 DTO 只有单 File）；`repo/internal/app/privacy.go:140`（依赖清除范式） |
| 解释结果、诊断、发送请求体 | 仅当前请求/面板内存；用量仍单独记账 | 不写 explanation、诊断正文日志、eval 报告或发送清单正文文件 | 关闭/失效清内存 | `repo/internal/app/inspect.go:321`、`:473`；`repo/internal/app/selection_test.go:75` |
| 授权与清除状态 | 无正文的许可版本、范围、接收方、撤销代际、删除失败状态 | 不将撤销正文复制到日志或隔离报告 | 授权状态随项目保留；内容删除后可保留无正文撤销标识 | `repo/internal/app/round.go:351`；`repo/internal/app/recovery.go:194`（授权/用量损坏不重置） |

**决策（建议）**：缓存存于仓库外按项目根隔离的私有 State；只留一个已发布代际，刷新可临时有一个新代际，成功切换即删旧代际。到期即禁止使用，运行中最迟 60 秒启动清除；应用离线无法定时删除，下一次 Open 在任何内容展示/发送前清除。关闭关联建议同时撤销保留许可并删除该项目全部预扫内容、派生物、临时/隔离副本和内存引用；不删除既有独立轮次/历史，后者仍按来源撤销规则处理。

**理由**：缓存包含原文，结果不保存不能替代保留策略；避免每次 policy hash 另存一份无限增长。

**代码依据**：State 要求在项目外、按根 hash 分目录（`repo/internal/app/store.go:60`、`:77`）；写入用 state 锁、0600 原子替换（`repo/internal/app/store.go:178`、`:193`）；现恢复通用上限为 256 MiB，不能当作新缓存的合理默认（`repo/internal/app/recovery.go:16`）。

**考虑过的替代方案**：只存 hash 不存可检索内容无法离线提供文档引用；永久保留原文风险更大；仅关闭发送而保留缓存可作为另一个明确产品动作，首版不隐含采用。

**需用户拍板的问题**：建议 7 天期限、关闭即清除、单已发布代际；这些是产品保留政策，需明确批准。建议新增缓存编码上限 64 MiB（含索引和 manifest），超限报告遗漏而非扩大旧通用上限；另以 D10 的捕获限额限制原文体积。

### D02(c)：模型接收范围表

| 材料 | 纯选区 | 获准关联 | 建议限制 | 代码依据/接入点 |
|---|---|---|---|---|
| 选区原文、语言、坐标 | 发送 | 发送 | 必需证据；文本中路径/密钥仍按 D11 拦截或脱敏预览 | `repo/internal/app/inspect.go:304` |
| 相交 Error 诊断 | 有则发送 | 有则发送 | 仅“VS Code 报告了什么”，不是代码事实；不执行嵌入指令 | `repo/internal/app/inspect.go:310`；`repo/internal/explain/explain.go:338` |
| 静态代码关系 | 不发送 | 可发送 | 确定性结构记录（关系方向、符号显示名、支持等级、快照时态），可选有界调用表达式原句；不含函数全文 | `repo/internal/codeindex/index.go:215`；`repo/internal/explain/explain.go:489`（须加强支持分类） |
| 文档依据 | 不发送 | 可发送获准短原句 | 明确是提及或决策陈述，不把索引词命中当依据；每条≤1 KiB 且≤12 行、合计受 D06 控制 | `repo/internal/explain/explain.go:89`、`:287`（现有 Source 直序列化，须投影） |
| review 输入 | 不超纯选区边界 | 不超本次获准发送集合 | 生成 claim + 被引证据再次外发，全部原始来源仍过 guard；文档同样不可信 | `repo/internal/explain/explain.go:378`；`repo/internal/app/inspect.go:467` |
| 路径、来源 hash、manifest 全清单 | File 为空 | 默认不发送路径/全清单 | 仅不透明 ID 与必要状态元数据；本地拼出处；相对路径须另授权，绝对路径永不发 | `repo/internal/explain/explain.go:85`、`:287`；`repo/extension/src/core.js:141` |
| 未选文件全文、排除文件内容、密钥类、运行时资料 | 禁止 | 禁止 | 也禁止拆片拼成全文；小文件的引用若等于全文，省略正文仅留关系元数据 | `repo/internal/capture/policy.go:23`；`repo/internal/app/model_connection.go:73`（策略是路径控制，不是内容脱敏） |

**决策（建议）**：选择“**未选文件全文不发送 + 按类型显式授权有限原句**”，放弃“任何未选文件原文都不发送”的绝对句式。调用点短引用、Markdown 原句都属于未选文件原文，不能称为摘要来绕过授权；同文件未选片段若来自预扫，同样只许作为获准关系证据，不能当实时补充上下文。

**理由**：需要可复查的设计依据；确定性关系与原句比模型全库摘要更易核查。

**代码依据**：Function/Call 的 Source 是源码片段（`repo/internal/codeindex/index.go:162`、`:218`）；Evidence.Source 直接进入请求（`repo/internal/explain/explain.go:89`、`:287`）。

**考虑过的替代方案**：只发结构记录、绝不发文档原句，隐私范围更窄但“文档为什么”的证据显著减弱；只发 LLM 摘要仍有跨文件披露和幻觉，不能免授权。

**需用户拍板的问题**：分别批准“代码关系/短调用引用”和“文档短引用”的发送许可；任一未批准类型都不发送，关联模式可仅用另一类。

## 模式与授权迁移

### D03：默认、开启、关闭与撤销

**决策（建议）**：纯选区为默认，已有 `model.enabled=true` 只代表既有模型使用授权。新增项目级、可撤销的两类许可：本地预扫/保留许可，以及关联发送许可（代码关系、调用引用、文档引用分型），后者绑定 provider/base URL 与授权版本。预扫本身不需要模型，也不发模型请求。

| 用户动作 | 建议可见内容与行为 | 缓存处理 |
|---|---|---|
| 旧项目升级/首次进入 | 标明“纯选区；关联未授权”；展示升级说明，不自动扫描文档 | 发现无有效新许可的预扫记录不使用，按无授权内容清除 |
| 授权本地预扫 | 预览根、Go/Markdown 范围、排除/遗漏、原文保留种类、7 天期限和清除入口；确认后执行 | 发布 manifest；不顺带启用外发 |
| 开启关联发送 | 展示已配置接收方、分型许可、索引版本/时间及未知状态；本次发送清单可预览；可取消 | 不创建新索引、不延长保留期；模型未启用时仅提示配置 |
| 关闭关联 | 立即切纯选区，说明“预扫正在清除”；取消待发/复核，拒绝返回旧面板 | 建议同时撤销本地保留并清除该项目预扫全代际及后代 |
| 撤销某文档/代码来源或 `.intentignore` 变化 | 立即关闭/遮蔽相关面板，停止新外发，提示清除完成或失败；不能承诺收回已发数据 | 删除来源与全部后代；保守清除整代际；放宽规则也须显式刷新才重新纳入 |
| 只暂停模型 | 说明此操作不等同清除本地预扫；提供独立“关闭关联并清除”入口 | 若保留许可仍有效可保留至到期；不产生新外发 |
| 切换接收方/工作区 | 新接收方重新确认发送许可；不同根互不继承 | 不自动复制缓存或许可；失去工作区时清内存与动作 ID |

**理由**：模型启用与预扫落盘/文档发送是不同数据行为。

**代码依据**：启用入口委托 configure（`repo/extension/src/extension.js:847`、`:867`）；现有配置提示发获准代码/轮次/问题，保存为 enabled 模型配置，没有预扫分型许可（`repo/extension/src/providers.js:265`、`:271`；`repo/internal/app/round.go:351`）。暂停只禁新分析并提示在途请求可能完成（`repo/extension/src/extension.js:883`）。

**考虑过的替代方案**：沿用 enabled 自动授权文档不接受；每次都重做全部授权负担高，建议项目范围许可 + 每次可查看清单，范围/接收方变化才重新授权。

**需用户拍板的问题**：批准分型许可、关闭即清除及接收方绑定。删除时限建议：在线检测撤销即失效，目标 60 秒内完成删除；失败时持续禁发、显示待清除并在下次 Open/访问重试，离线不承诺后台 SLA。磁盘删除指应用可访问副本的逻辑删除，不承诺对备份/存储介质安全擦除或对服务端已收数据追溯删除。

## 缓存有效性状态机

### D04：版本、新鲜度、选区身份与刷新分离

**决策（建议）**：manifest 是有时点的捕获结果。读取缓存和“已证明全库现在没变”是两件事；新请求不能从旧 hash、stat 相同或没有 watcher 事件推出实时命中。

建议 manifest v1 包含：schema、生成器/Go 解析器/Markdown 分段器/分词器版本、generation ID、生成起止时间、工作区身份、完整 policy hash（默认规则 + `.intentignore` + 预扫范围/预算/分段配置）、授权版本、每文件相对路径/hash/捕获大小、Go/docs 独立 coverage 与完整遗漏原因、索引体完整性摘要、每条派生记录的 sources 全集。模型只收必要的不透明版本/状态，不收 manifest。

**代码依据**：现 policy hash 只有 capture 配置及 patterns，不含拟议分段器配置（`repo/internal/capture/policy.go:160`）；公开 coverage 的 OmissionReasons 不序列化（`repo/internal/codeindex/index.go:57`）；源码 hash 是读取之后由 NewFile 计算（`repo/internal/capture/capture.go:54`、`:275`、`:296`）；捕获只做读取时的稳定性核对（`repo/internal/capture/capture.go:310`），不是整个项目的事务快照。

| 状态（建议） | 可观测判据 | 本次选区解释的唯一行为 | UI |
|---|---|---|---|
| 命中 hit | 本次显式预扫/核验刚完成，版本兼容、完整性/策略/许可通过，请求固定该生成代际及验证时点 | 返回预算内关联，所有关系表述限定“该次预扫中”；不保证生成期间全库原子一致 | 代际、生成/核验时间、分项 coverage；“按预扫时点”，不写“实时全库已验证” |
| 未知 unknown | 索引结构/策略/许可有效，但复用到之后独立请求，没有本次全内容核验；进程重启亦如此 | **允许返回有明确快照时态的获准关联，并标新鲜度未验证**；代码锚定不足则不发该代码关联，仅保留选区/诊断及安全的文档提及 | 醒目“新鲜度未验证”；旧行号仅为快照出处；显式刷新按钮 |
| 过期 stale | 显式核验发现任一 hash 或文件集合变化；已观察到编辑版本/选区锚点不符；policy/schema/解析器/范围版本变化；超保留期限 | **不返回旧关联**，仅选区/相交诊断并提示显式刷新；策略撤销先清除，当前文件也撤销则整个请求拒绝 | “索引过期：原因”；不暗中刷新，不把旧关联混入 unknown 回退 |
| 缺失 missing | 没有获准可用 manifest，或缓存损坏被禁用/清除 | 继续纯选区解释并提示“关联需先预扫”；不要因缺索引禁用原有解释 | “未预扫/缓存不可用”，提供独立操作 |

建议状态优先级：拒绝当前文件/许可无效优先于一切；已知过期 > 缺失/损坏 > 未知；不能以“其他文件没检查”覆盖已经知道的过期。完整性损坏不是普通新鲜度未知。仅点击显式刷新才枚举、读获准文件并比较全 hash/增删；成功发布新代际，失败保留错误状态，不把旧版伪装成新版。新代际返回后，下一次独立选区请求通常为 unknown；**没有 N 分钟免核验的“精确实时”有效期**。

建议转移：missing → 显式预扫成功 → hit；hit → 后续独立请求/失去核验上下文 → unknown；hit/unknown → 已知变更 → stale；stale → 显式刷新成功 → hit；任意状态 → 撤销/到期清除完成 → missing。未观测的 Markdown 外部修改只能保留 unknown，不能声称已检测为 stale。首版不新增内容监听增量；已有策略 watcher 负责撤销，编辑器已知保存/版本变化只能失效，不能触发重扫。

**选区身份（建议）**：预扫保留本地文件 hash、声明原文和精确字节/UTF-16 范围用于快照锚定；请求携带保存版本、字符范围与选区 hash，校验它与缓存同范围片段一致，行/字符边界必须正确（含 CRLF/多字节）。这只证明片段相合，**不能证明当前文件整 hash 未变**。当前整文件 hash 若无独立核验凭证便标未验证，不读未选内容补算。unknown 下只可说“预扫中的此处/所属函数背景”，不能声称确定为当前函数；无法唯一锚定、多函数/跨声明或同片段歧义时省略代码关联并提示刷新。需要保证“当前精确 caller”的产品要求应另授权完整当前文件核验，不能悄悄纳入本轮边界。

**理由**：解决“实时全 hash”与受限读取的阻断冲突，也避免仅凭行号给错函数配证据。

**代码依据**：现保存版本/诊断仅检查当前编辑器（`repo/extension/src/extension.js:734`、`:750`）；服务端只验传入 source/数值坐标，没有磁盘片段同一性核验（`repo/internal/app/inspect.go:327`、`:340`）；Go 索引现仅声明行区间，调用起点列，无完整调用末端契约（`repo/internal/codeindex/index.go:20`、`:29`、`:215`）。

**考虑过的替代方案**：unknown 一律只发选区最保守但缓存关联使用率低；unknown 自动重扫违反 I/O；缓存时间短或 stat 校验均不等于实时验证。

**需用户拍板的问题**：建议批准表中 **unknown 可发标注的快照关联、stale 必须降级** 的单一政策，并接受无法保证请求时全库新鲜；如不接受，改为 unknown 也只发选区，不得同时承诺零跨文件读取与实时关联。建议锚点不充分的代码关联始终省略。

## Schema 与能力版本

### D05：内部缓存与宿主传输分别版本化

**决策（建议）**：新增独立 `prescan`/显式刷新能力；内部 `prescan_manifest_schema=1`，关联请求/响应另设 `selection_context_schema=1` 与能力 `prescan-v1`、`selection-context-v1`、`opaque-evidence-v1`、`provenance-revocation-v1`。名字是拟定契约，7a 冻结最终字段；不从软件版本字符串推断能力。

建议在现 `integration-info` 扩展可选 capabilities 声明，扩展读取并求支持交集；请求明确选纯选区或关联协议和许可版本。无字段/能力查询不支持时只用旧 explain-selection；用户显式要关联则提示升级，不尝试新命令后偷偷退为别的扫描路径。新版 CLI 对未协商请求仍返回旧两类证据；新版扩展须拒绝未知 schema/类型、伪造/重复 ID、引用不闭合、越界来源，不能靠前缀放行。替换 binaryPath、工作区/二进制身份变化后重新协商，不复用旧能力缓存。

| 组合 | 建议结果 |
|---|---|
| 新扩展 + 旧 CLI（无新能力） | 纯选区正常；关联按钮解释“CLI 不支持，请升级”；不发送新许可/证据命令 |
| 旧扩展 + 新 CLI | 未 opt-in 时旧选区响应；不得按服务端项目开关给旧宿主推新证据 |
| 旧扩展意外收到新证据 | 仍应拒绝；升级错误要明确，不能删去校验硬凑成功 |
| 新新组合但 schema 不匹配 | 明确提示升级/禁用关联，纯选区兼容路径仍可用 |
| 旧/坏 index 或 prescan 缓存 | prescan 版本不兼容视为不可用、提示显式重建；不改坏既有函数历史 |

旧 `index` 的 Go functions/issues/coverage 语义与结果契约保持不变；新产物独立报告 `coverage.go` / `coverage.docs`、遗漏及预算，不能给旧 Go complete 悄悄附加文档含义。内部缓存必须保存完整遗漏原因，不仅是公开 JSON。

**理由**：外置二进制与任意宿主组合是可见入口，版本号相近不保证证据协议相容。

**代码依据**：binaryPath 选择（`repo/extension/src/extension.js:81`）；现 integration-info schema=1、列 commands/coverage 但没有预扫能力协商（`repo/cmd/intentlens/integrations.go:3`、`:10`、`:27`）；未知命令报错（`repo/cmd/intentlens/main.go:260`）；旧扩展只接受两个 ID/空 File（`repo/extension/src/core.js:115`）；旧 Index/Coverage 定义（`repo/internal/codeindex/index.go:38`、`:48`）。

**考虑过的替代方案**：扩充旧 index 加 docs 或全局放宽 selection schema，容易使旧宿主误解 complete 或拒绝结果。

**需用户拍板的问题**：建议批准独立协议和“未协商不启用关联”；具体软件发行号可在第 9 步确定，不作为能力凭证。

## 完整输入预算表

### D06：全证据、完整序列化与两阶段预算

**决策（建议）**：保留 **JSON 编码 evidence 数组≤60,000 字节** 的现有硬门槛；这不是“整个 HTTP body≤60,000”，两阶段 prompt/包装/输出还须另外计入总预算。先预留完整选区和所有合法相交 Error，再从剩余容量选关联，绝不为 fast 删除关键诊断。以下新增条数/片段限额是建议，须在 7a 固化边界值。

| 输入项 | 建议材料限额/计量 | fast 占位及最终编码/两阶段处理 | 现有依据 |
|---|---|---|---|
| 选区 | 1 条、UTF-8≤16 KiB；非空且合法 | 完整保留，算入全部 Source 字节；仅选区>8 KiB 就不能 fast | `repo/internal/app/inspect.go:264`、`:304`；`repo/internal/explain/explain.go:186` |
| 诊断 | 最多100个 Error，合为有诊断时的1条 evidence | JSON 文本本身计入 Source，外层再次转义也计量；超必需容量拒绝并请缩小选区，不静默少发诊断 | `repo/internal/app/inspect.go:265`、`:292`、`:310` |
| 当前文件实时补充 | **0 条、0 字节** | 不为 import/类型留隐含窗口；预扫里的同文件关系属于下一行 | `repo/extension/src/extension.js:727`（现状）；D09 建议 |
| 关联 | 最多6条、合计 Source≤6 KiB；单条原句≤1 KiB且≤12行；结构记录也计入合计 | 仍只能用剩余预算；先确定 resolved 关系，再明确文档决策，再文档提及；首版不产跨包同名候选 | `repo/internal/codeindex/index.go:220`；`repo/internal/explain/explain.go:154`（新增限额不是现状） |
| 元数据与 JSON 转义 | ID、类型、等级、语言、坐标、版本/时态、括号键名、双层诊断转义全部实测 | 不用“Source 合计”代替编码量；本地路径/hash 清单不投影给模型 | `repo/internal/explain/explain.go:79`、`:287`、`:326` |
| question + answer system prompt | fast question≤2,000字节；一般 question≤8,000字节；完整实际提示含诊断/文档不可信边界 | 三套 answer 分别测实际 body/预算；新增提示也必须计量 | `repo/internal/explain/explain.go:156`、`:162`、`:323`、`:338` |
| review prompt + claims + 被引 evidence | light/strict 第二阶段实际重新编码，claim 可能回显证据；同一来源再次发送仍受许可 | 两阶段合计输入/输出额度，不能只算 answer；review 动态剩余不足保持 uncertain 并提示，不自动追加预算 | `repo/internal/explain/explain.go:378`、`:388` |
| 模型输出额度 | 现 light answer/review为1024/384、fast为1024/0、strict为2048/1024 token | 建议先保留，验证完整请求预留足够；不能把输出额度当输入字节 | `repo/internal/explain/explain.go:149` |
| provider 包装及累计用量 | 实际 prompt/JSON/provider 包装开销和两次输出；预估保守，真实计费独立 | 发送前保守 reserve，发送后结算；撤销后也记已发生用量 | `repo/internal/explain/explain.go:246`、`:258`；`repo/internal/app/inspect.go:469`、`:476` |

建议具体算法顺序：

1. 校验/构造必需证据；仅必需证据编码已>60,000 或诊断>100 即拒绝，不能截断选区/诊断强行发送。
2. 确定性排序关联（相关性/类型优先，其次本地规范路径及范围稳定排序），在6条/6 KiB以及最终60,000上限内加入；片段在 UTF-8/句子边界裁切，并记录 `truncated`、省略条数及理由；不够完整语义的引用直接省略。
3. 用**全体 evidence**计算 fast：≤8条、全部 Source 合计≤8 KiB、question≤2,000字节；有诊断时必需占2条，关联条数余量至多6，字节余量为 `8192−选区字节−诊断Source字节`。例如选区7 KiB、诊断2 KiB即已不能 fast，关联0条也必须升级 strict。
4. 建议不为了维持 fast 另裁必需输入；可选关联按产品上限裁后仍超过 fast 则自动升级 strict，UI 说明两阶段/成本变化；项目预算不足则在发送前拒绝。
5. 两阶段请求按各自实际序列化内容计量，包含 profileForRun 追加的边界文字及 review 的 claims。现 reservation 有保守估算基础，后续先验证不低估再决定是否调整算法；不能仅以旧公式存在断言新输入已覆盖。
6. 第二阶段预算不足/普通复核故障遵循 uncertain 提示；策略撤销错误则丢弃整个结果，不能以 uncertain 返回已撤销内容。

**理由**：fast 是全输入小范围资格，不是关联材料的独立额度。

**代码依据**：fast 全 evidence 判定与自动 strict（`repo/internal/explain/explain.go:186`、`:203`）；60000 检查（`repo/internal/app/inspect.go:312`）；review 预算退化及策略阻断特殊处理（`repo/internal/explain/explain.go:388`、`:395`）；预留入口用 ReservationForSelectionMode，而实际运行再追加诊断提示（`repo/internal/app/inspect.go:487`；`repo/internal/explain/explain.go:241`、`:336`）。

**考虑过的替代方案**：给关联独立8条/8 KiB会忽略必需证据；删诊断保 fast 不接受；把60000当整个两阶段上限会误述既有契约。

**需用户拍板的问题**：建议批准6条/6 KiB、每条引用1 KiB/12行及排序/裁剪政策；60,000与fast全体门槛作为必须保留的边界。

## 派生证据来源与撤销

### D07：来源全集与四处守卫

**决策（建议）**：每个符号、关系、引用、未来摘要都保留 `sources[]`：文件相对路径、捕获全文 hash、行/字符范围、原文片段 hash、generation，以及派生父 ID。一个“caller A→B”的来源至少含调用点 A 与被解析声明 B；分组解析/消歧若依赖其他包文件，这些也进入来源全集，首版可保守把整同包解析输入列为来源，不能为减清单遗漏。跨文件汇总取来源并集；未来摘要即使不含原句也不能摆脱来源撤销。

任一来源撤销立即禁止记录与全部后代发送/展示；首版建议 policy hash 变化时直接失效并清除全部预扫代际，降低漏删风险，重新放行仍需显式预扫。授权 epoch 与索引 generation 必须参与并发发布：正在构建的旧授权结果不能在撤销之后重新发布；清除与刷新采用明确锁顺序和原子切换。模型无需这些路径，但本地 guard 必须拿得到完整集合。

| 检查位置 | 建议检查与失败行为 | 现有落点与差距 |
|---|---|---|
| answer 前 | 最新许可、策略、代际、全部来源路径安全、DTO/ID/预算；不读来源正文 | `repo/internal/app/inspect.go:463`；配置闭包实际在 `repo/internal/app/model_connection.go:52`；HTTP发送前 `repo/internal/explain/providers.go:132` |
| review 前及返回前 | review 即使只引一条也检查第一阶段全部来源；返回再次验证许可/策略，撤销丢结果，已用token记账 | `repo/internal/app/inspect.go:467`、`:481`；`repo/internal/explain/explain.go:393`、`:395`；当前 guard 只覆盖返回的 paths，尚无 sources[] |
| 面板打开/恢复可见 | 主机复核工作区、许可 epoch、manifest、来源全集；失效遮蔽/关闭面板，不能仅信 webview 内数据 | 现策略 watcher 关闭该根面板并调用 status 清理（`repo/extension/src/extension.js:922`）；现选区面板 handler 为空（`:752`） |
| 跳转/查看引用 | 只接主机发的有界 record ID；重取本代际合法证据、重查所有来源；不接受 webview 路径；先展示快照出处 | 现本地动作ID范式（`repo/extension/src/sidebar.js:129`）；现 openCurrent 只负责路径限制（`repo/extension/src/extension.js:138`），不足以核验新血缘 |

**导航例外（建议）**：用户主动点击“核验并打开当前目标”才允许单独读取该目标文件全文求 hash（先过路径/策略/许可检查），此是独立本地导航动作，不是选区热路径，也不授权再发送目标全文。hash相同才按快照范围精确定位；hash不同或目标脏则不按旧行号精确跳转，提示“目标已变化”并保留仅缓存快照视图。路径失效/链接/撤销则拒绝；未知状态按钮清楚注明需要读取目标，不能打开就扫描全库重建 call-path。

**理由**：只检查显示文件，无法撤销多来源派生物；复核会回显第一阶段信息。

**代码依据**：现 `analysisSourcePaths` 选区只允许两个 Kind/ID 与空 File（`repo/internal/app/inspect.go:512`）；现清除会删含不允许 evidence 的整个 explanation，再删缺源的 share-draft/correction（`repo/internal/app/privacy.go:118`、`:140`）；当前 recover 对未知记录直接跳过（`repo/internal/app/recovery.go:155`）。

**考虑过的替代方案**：仅换 policy hash 缓存文件名会留旧正文；只删原文不删摘要/关系后代不接受；跳转照搬现 call-path 重建会破坏读取边界。

**需用户拍板的问题**：建议批准整代际清除和上述单目标导航例外/60秒删除目标；若不批准导航读取，首版只提供缓存快照，不承诺定位当前文件精确行。

## 支持等级

### D08：四档证据，支持状态另判

**决策（建议）**：以下四档作为显式类型与显示分组；证据等级与模型的 supported/uncertain 是两个维度。supported 只表示该 claim 被提供的证据支持，不保证项目实时、完整或测试通过；unknown 索引下的关系要限定预扫时态，不能当当前事实。

| 档位 | 能说什么 | 不能说什么/支持限制 | 代码依据 |
|---|---|---|---|
| 确定边 resolved direct | “该版预扫中同目录同包 A 直接调用 B”，带方向和出处；字符范围证实位于选区内才能称选区内调用，否则标所属函数背景 | 不外推动态/接口/跨包；同包不是充分条件；不从整个函数的边推断选中那一行执行 | `repo/internal/codeindex/index.go:91`、`:183`、`:221`、`:275` |
| 同名候选 name_candidate | “同名、尚未解析”，只许 uncertain；首版建议不生产跨包同名候选，保留此拒绝契约防未来误用 | 不能标 caller/call_site/usage，不作为 strict 用途门槛证据；即使 review 说 supported 也不得升级 | `repo/internal/codeindex/index.go:226`；`repo/internal/explain/explain.go:489`（现仅标签门槛需修改） |
| 文档提及 doc_mention | “文档此段提到某名字/概念”，附原句；与当前代码的关联仍 uncertain | 单纯命中不证明调用、用途或设计原因，不得混用 documentation 通用标签绕过用途门槛 | `repo/internal/explain/explain.go:499`；文档提及分类为拟新增 |
| 文档明确决策陈述 doc_decision | “某版文档明确记载理由 X”，有明确对象与原句，经过语义复核可支持这项归属陈述 | 不证明作者个人动机/实现已经遵循；只靠标题、关键词或自动标签不算明确陈述 | `repo/internal/explain/explain.go:170`、`:175`、`:437`；`repo/internal/app/inspect.go:316` |

建议同名消歧结合路径、包、接收者及标题上下文；无法确定时保留提及档。文档与选区冲突时并列“文档记载 X；选区可见 Y”，不强行调和，隐藏/未选实现保持未知。无结果固定写“**本次范围未检出**”，同时显示覆盖、语言、预算/遗漏；不能证明“没有调用者”，更不能要求无 caller 时 callee/文档也为空。fast 仍全部 uncertain，不能以确定边等级冒称独立复核。

**理由**：现用途标签检查并不自动验证语义，新增类型不应把候选洗成支持事实。

**代码依据**：hasPurposeEvidence 只认被引用 Kind/Role 标签（`repo/internal/explain/explain.go:489`）；strict 检查用途门槛（`:437`），review 要求证据蕴含而非仅引用存在（`:175`）；fast单阶段（`:303`）；motive 被过滤、dependency 本就允许（`:357`、`:366`）。

**考虑过的替代方案**：全部复用 caller/documentation 标签会混淆；一律 uncertain 则丢掉明确快照边/文档归属的可核查价值。

**需用户拍板的问题**：建议批准四档与首版不生产跨包同名候选；“明确决策”自动检测只能产生待复核候选，不能靠索引器直接认定 supported。

## 首版范围与不做事项

### D09：当前文件未选内容

**决策（建议）**：采纳评审首版不实时读取当前文件未选部分的建议，名称明确为“选区 + 预扫关联”。不在 CLI 现场读取 import、类型定义、相邻函数或全文；不让扩展 `getText()` 无范围读取全文来算当前 hash。预扫阶段本地可采这些内容，仅可经 D02 获准的关系投影进入请求，不能借缓存把所有当前文件未选部分送出。

**理由**：保持现有输入承诺、降低新取证面，且避免“为验证 hash”悄悄打破不读未选部分的约束。代价是 D04 的全文件身份/当前性不能每次验证，必须显式呈现。

**代码依据**：扩展 `getText(selection)`（`repo/extension/src/extension.js:727`）；CLI source/坐标验证不读磁盘（`repo/internal/app/inspect.go:327`、`:340`）。

**考虑过的替代方案**：受限同文件窗口、类型/import专用上下文或全文本地 hash；均须新证据类型、来源hash、窗口上限、预算和单独授权。只发hash虽不发正文，仍属于读取未选内容。

**需用户拍板的问题**：建议批准不读；若需要这些上下文，另立边界修订再开工，不能由实施者自行加窗口。

### D10：Go + Markdown 与明确延期事项

**决策（建议）**：代码预扫仅获准 `.go`；文档默认只列根目录 `*.md` 和 `docs/**/*.md` 供用户选择，不自动捕获隐藏目录或全部 Markdown，策略排除始终优先。非 Go 的已保存非 plaintext 选区仍可解释；无该语言符号/调用预扫，最多从获准文档得到明确标注的提及，不承诺多语言调用图。

建议预扫 Go/docs 各自总文本≤16 MiB、单文件≤1 MiB，不能共享挤占；Go最多10,000文件，docs最多2,000文件/20,000段，单段≤4 KiB（按行/句拆分保留标题与出处），总缓存编码≤64 MiB、构建上限120秒；达到上限明确 coverage 不完整/构建失败，不静默扩大范围。数值为首版待批准工程边界，后续基准可提案调整，不能自动增长。读取前过滤、UTF-8/无NUL、普通文件/无链接、读取中变化检查均保留。

**Markdown 形态（建议）**：按 ATX/Setext 标题层级分段，标题前序言也成段；记录标题路径、物理文件起止行、原文/hash、段落ID与拆分序号，CRLF 与围栏代码内伪标题须正确处理，围栏内容作不可信文本，不执行代码/指令。不追随 Markdown 链接读范围外文件。中文采用确定性字符 n-gram（建议二元）结合标识符完整词检索，声明不保证中文语义分词/召回；同名靠包/路径/标题上下文消歧，无法确定降级提及。日期/版本只保留文档可见信息；没有就标“文档未声明”，不把 mtime 或预扫时间当文档版本。历史报告/过期文档与现代码冲突并列展示，不调和、不把历史测试数当当前验收。

**首版明确不做（建议）**：LLM 全库摘要或全库文档外发；文件监听增量构建（现有 watcher 可继续撤销/失效）；跨包完整类型加载/依赖下载；非 Go 符号调用分析；全面改造共享 `buildIndex`、Inspect、CallPath 和历史 freshness 缓存；结果自动保存/分享。先在新独立预扫路径验证正确性、文件读量、冷/热时延、峰值内存、编码大小及并发收益，再讨论共享缓存。D01 的后台抑制属于入口调度，不要求改造共享缓存。

**理由**：可控首版范围与可测收益优先，不能把“全项目预扫”宣传成所有语言完整语义。

**代码依据**：TakeGo `.go`过滤、16 MiB/1 MiB边界（`repo/internal/capture/capture.go:19`、`:240`）；无 importer/同目录同包（`repo/internal/codeindex/index.go:91`、`:106`）；匿名函数跳过、方法非完整解析（`:183`、`:199`）；CallPath调用共享 buildIndex（`repo/internal/app/callpath.go:57`）；历史验证文档明确不代表当前发行验收（`repo/docs/verification.md:1`、`:5`）；非Go source验证不限定Go（`repo/internal/app/inspect.go:334`）。

**考虑过的替代方案**：所有 Markdown 默认扫、完整多语言或 LLM 摘要均扩大授权/性能/语义责任；监听增量需要另行一致性设计，不用于掩盖本版 unknown 状态。

**需用户拍板的问题**：批准 Go + Markdown 收窄、文档默认范围、预算及延期事项；若多语言完整关联是硬需求，需扩计划而不能只改产品文案。

## 隐私对外表述

### D11：定稿文本与 File 最小披露

**决策（建议定稿，待批准后同步发布）**：

> 纯选区模式保持现有范围；启用关联模式后，将按本次清单向已配置服务发送选区、相交错误，以及来自获准预扫的有限代码关系和文档依据；预扫内容在本地另行保留。

建议紧随说明：

> 预扫与刷新是单独的显式操作。选区工作流不读取当前文件未选正文或其他源码来全量核验 hash；关联以所示预扫版本为准，复用时标记“新鲜度未验证”。引用可能包含获准的未选代码调用原句或文档短原句，但不发送未选文件全文。结果与诊断默认不保存；预扫原文按授权期限保留，可关闭并清除。

本地保持 `证据ID → 工作区/代际/路径/行与字符范围/hash/来源全集` 映射；模型只收不透明随机 ID 与证据内容/必要类型状态。ID 不编码路径，不把 ID hash 误称脱敏。File 在默认 transport 投影为空（或新schema省略），本地证据保留路径；输出按 ID 拼出处，不能把模型编造的路径用于导航。相对路径仅在另行明示许可时授予；建议首版不提供相对路径外发开关，保留未来能力，不从“开启关联”自动推导路径许可。绝对路径在任何模式永不发送。

**内容中嵌路径的处理（建议）**：检查 Source、诊断、结构记录、文档原句、标题、question 及 review claim 内嵌的 POSIX/Windows/UNC/文件URI绝对路径；可确定的路径替换为本次不透明占位符，模型仍以原证据 ID 引用，本地保留映射。秘密/凭据疑似值不发；可选证据省略并标注，必需选区/诊断若无法安全转换则拒绝发送并给出本地预览，要求用户修改/缩小选区，绝不静默丢关键诊断。转换若破坏可解释语义同样拒绝/省略，不把脱敏后的文字当原句逐字证明；请求清单显示“已遮蔽”。检测能力有限，不能承诺任意秘密自动识别；未实施有效内容检查前不能宣称实现了“绝对路径永不发送”。

**理由**：禁 File 绝对路径与内容脱敏是两个控制；预扫派生内容外发才是实质扩边界，不能只改一个 File 字段。

**代码依据**：现 File/Source 一起序列化（`repo/internal/explain/explain.go:79`、`:287`）；诊断原消息保留（`repo/internal/app/inspect.go:414`）；旧对外承诺（`repo/extension/package.json:98`；`repo/extension/src/core.js:177`）；本地拼引用位置范式（`repo/extension/src/core.js:141`）。这些位置并未提供自动文本脱敏保证。

**考虑过的替代方案**：把相对路径视为关联必需不成立；只禁 File 而原样发 Source 会漏路径；声称“摘要已匿名化”无代码依据。

**需用户拍板的问题**：批准定稿文本、不透明ID、首版不外发相对路径，以及必需材料不能安全处理则拒发的政策。后续 description、面板、启用界面、CLI/Skill说明须一起更新；本轮不改这些文件。

## 保存策略

### D12：临时解释与持久预扫分开

**决策（建议）**：选区结果、诊断、claim与本次发送清单正文默认只在内存，不写解释历史、修订/共享记录或诊断日志。用量/许可/无正文清除状态允许持久化；显式授权的预扫可以包含与选区恰好相同的文本，不能据此宣称结果被保存。测试按记录类别、字段与新增写入核验，不把“State 中任何选区字串都不得存在”直接套到已获准缓存。

用户未来若要“保存本次结果”，作为后续可选动作：预览材料、索引版本、出处/hash、诊断与权限范围，再实现读取、撤销和过期；保存不授权分享，首版不实现此动作。

**理由**：保持现有临时解释语义，避免同时引入稳定选区身份/非Go历史新鲜度/分享生命周期。

**代码依据**：ExplainSelection直接返回而 generate写explanation（`repo/internal/app/inspect.go:321`、`:422`）；不保存测试断言（`repo/internal/app/selection_test.go:75`）；预留/结算用量仍写状态（`repo/internal/app/inspect.go:473`、`:477`）；现共享有准备与确认入口（`repo/extension/src/extension.js:190`）。

**考虑过的替代方案**：自动保存关联解释扩大留存范围；完全禁止一切State写入则连现有用量管理也破坏。

**需用户拍板的问题**：建议批准继续不保存与预扫分类留存；“保存本次结果”明确延期。

## 验收案例

### D13：边界到契约的映射

**决策（建议）**：下表是未来 7a/8a 与 7b/8b/9 的验收要求，**本轮未执行，不能记为通过**。每行至少一个正例和一个反例；反例表示系统应拒绝/降级/标注，不能“测试出现错误就算通过”。所有模式覆盖 answer、存在时的 review、返回、面板/导航；核对实际 HTTP transport 请求体与文件访问，不只看 renderer。现请求 transport 可注入（`repo/internal/app/store.go:98`；`repo/internal/explain/providers.go:129`），可作为后续观测入口。

| 对应边界 | 正例（应成功或正确显示） | 反例（必须防止） | 后续契约落点/依据 |
|---|---|---|---|
| D01 工作区后台限制 | 侧栏可见、快速改变选区、CodeLens请求并发时，受限模式只读允许材料/安全元数据 | 取消前的 inspect 继续扫库、自动CodeLens触发 index；必须等结束才声称限制生效 | `repo/extension/test/sidebar.test.js:132`、`:161`；`repo/extension/src/extension.js:605` |
| D02(a) 本地读取 | 关联热请求读指定缓存、最新策略和选区，访问跟踪符合白名单 | 为hash打开另一源码/Markdown或枚举项目；即使不进prompt也不通过 | `repo/internal/app/inspect.go:97`；`repo/internal/capture/capture.go:296` |
| D02(b) 留存 | 获准预扫按范围持久化、单代际、到期清除；刷新原子替换 | 旧policy缓存、隔离坏副本/临时正文或超期缓存仍留存/可发送 | `repo/internal/app/store.go:178`；`repo/internal/app/recovery.go:129`、`:230` |
| D02(c) 引用边界 | 开启指定文档/调用引用许可后有限短引可发送、ID可本地定位 | 未授权类型、把短小未选文件全文当“短引”、多片拼全文不得发送 | `repo/internal/explain/explain.go:287`；`repo/extension/src/core.js:115` |
| D03 默认/迁移 | 旧已启用模型项目仍仅选区+相交Error；仅本地预扫不发模型 | 升级或只勾预扫就自动发送文档；接收方更换复用原许可 | `repo/extension/src/providers.js:265`、`:271` |
| D03 关闭/撤销 | 关闭即禁新发送并清预扫；暂停模型保留仍获准缓存且说明区别 | 删除失败却显示“已清除”、应用离线承诺准时删除、撤销后新代际重新发布 | `repo/extension/src/extension.js:883`；`repo/internal/app/privacy.go:118` |
| D04 四态 | 显式刷新后的时点hit；之后unknown带“新鲜度未验证”；缺失继续纯选区并提示先预扫 | 旧hash/未触发watcher即标实时hit，缺失/过期静默扫库 | `repo/internal/capture/capture.go:296`；`repo/internal/app/inspect.go:96` |
| D04 外部变更 | 未观测的文档修改显示unknown；显式核验发现后stale只发选区/诊断 | 已知stale仍发旧关联；未知却谎称检测到全部变更或精确实时行号 | `repo/extension/src/extension.js:935`；`repo/internal/capture/capture.go:310` |
| D04 选区绑定 | 多字节/CRLF范围与缓存片段相合时仅给快照关系；不充分时省略代码关联 | 选区hash相同就声称全文件hash相同、同文同行歧义配错当前函数 | `repo/internal/app/inspect.go:340`；`repo/internal/explain/explain.go:104` |
| D05 能力/旧契约 | 新新协商后新schema；旧CLI纯选区仍正常；旧index结果不受docs预算影响 | 旧扩展收到新证据须拒绝并明确升级；伪造schema/重复ID/未知引用不得吞掉 | `repo/extension/src/core.js:115`、`:122`；`repo/cmd/intentlens/integrations.go:3` |
| D06 全体fast阈值 | 选区+诊断+关联合计8条/8192字节及question2000边界按实际输入判断 | 关联自身未超8KiB就保fast；选区7KiB+诊断2KiB不升级；删除诊断凑阈值 | `repo/internal/explain/explain.go:186`、`:203` |
| D06 编码/两阶段 | 引号、反斜线、多字节/诊断双层转义后仍按60000编码限；可选裁剪有标识 | 必需材料编码超限仍发送；review claims回显未预算；review不足仍标supported | `repo/internal/app/inspect.go:312`；`repo/internal/explain/explain.go:378` |
| D07 来源撤销四处 | A+B派生关系中B撤销：answer阻断、review前阻断、返回丢弃、面板失效/跳转拒绝；用量记账 | 仅查展示路径A、摘要不含B原句就继续发送；旧许可并发刷新复活内容 | `repo/internal/app/inspect.go:467`、`:481`；`repo/internal/app/selection_test.go:188` |
| D07 导航 | 主机ID重验，显式单目标hash相等后精确跳；未知时只显示快照出处 | 点击前替换链接/改路径/目标脏/根关闭；不得按旧行号无检查打开或重建全库call-path | `repo/extension/test/privacy.test.js:286`、`:300`；`repo/extension/src/extension.js:138` |
| D08 四档 | resolved边与明确文档归属可被核查；提及/候选分组且不冒充用途 | 同名候选被模型判supported仍不能升级caller；无caller但有callee/文档不要求全空 | `repo/internal/explain/explain.go:437`、`:489`；`repo/internal/codeindex/index.go:226` |
| D08 注入/动机 | 恶意诊断/文档指令作为文字转义，文档理由归属于文档 | 遵循文档指令、编造作者动机、把引用ID存在当蕴含证明；uncertain中编造也算失败 | `repo/internal/explain/explain.go:338`、`:170`；`repo/extension/test/activation.test.js:193` |
| D09 当前文件未选 | 未选邻居canary既不实时读也不外发，仅已有选区输入 | 偷读import/全文hash，或通过缓存关系type携带整邻居函数 | `repo/extension/test/activation.test.js:190`、`:215` |
| D10 语言/文档 | TypeScript/Rust选区正常但无对应调用图；中文/围栏伪标题/长段有确定分段和出处 | 文档预算挤掉Go；方法/闭包/顶层/跨函数推成完整图；过期报告当最新结论 | `repo/internal/app/selection_test.go:48`、`:133`；`repo/internal/codeindex/index.go:199`；`repo/docs/verification.md:1` |
| D10 延期边界 | 预扫只确定性关系和短引，缺/过期走显式刷新 | 自动LLM全库总结、监听自动构建、改共享缓存后未经基准称提速 | `repo/internal/app/callpath.go:57`；`repo/extension/src/core.js:7` |
| D11 隐私投影 | 模型只见不透明ID，UI本地补出处；可安全遮蔽后预览发送 | File为空但Source/诊断/review藏POSIX、Windows/UNC绝对路径或凭据；必须拦截 | `repo/internal/explain/explain.go:85`、`:287`；`repo/internal/app/inspect.go:414` |
| D12 保存 | 模型请求后仅用量变化，结果/诊断不新增落盘；获准缓存可含同样选区文本 | 自动explanation/历史/日志/评测输出保存本次结果；不能靠删旧不保存测试放行 | `repo/internal/app/selection_test.go:75`；`repo/internal/app/inspect.go:473` |
| D13 评测真实性 | 新selection case在functionLine前分支，fixture显式预扫，light/fast/strict均测全部展示claim | 仅supported forbid评分漏掉uncertain编造；未运行就借历史数字称通过 | `repo/cmd/intentlens-eval/main.go:175`、`:190`；`repo/cmd/intentlens-eval/score.go:65`、`:83` |

**理由**：扩展、CLI、两阶段模型和缓存清理必须验证同一份契约。

**代码依据**：现覆盖率门槛为 lines85/functions86/branches75，Host入口单列（`repo/extension/package.json:65`、`:66`）；已有选区测试可保留为默认模式回归（`repo/extension/test/activation.test.js:198`；`repo/internal/explain/selection_test.go:37`），不能整体放宽代替新fixture。

**考虑过的替代方案**：仅模型prompt黑名单、仅UI快照或仅Go测试不能证明读取/撤销/版本组合；仅关键词命中不能证明关联语义。

**需用户拍板的问题**：建议批准为后续验收清单；7a/8a先定测试/eval字段与canary，本轮不写测试。真实模型质量、Extension Host、Windows x64 VSIX和本地fixture运行结果分开记录，历史结果不覆盖。

## 附录：0a 核对记录

以下是对现有正文的静态核对结论；“影响/接入建议”不是现状。测试名称只证明存在断言，不表示本轮执行通过。

### A1. privacy / round / recover

| 文件:行 | 一句话现有行为 | 对第0步边界的影响/接入建议 |
|---|---|---|
| `repo/internal/app/store.go:88` | Open先recoverState，再scrubExcluded，失败不返回App。 | 新预扫必须进入恢复与撤销清理，否则仅新增读写函数不足。 |
| `repo/internal/app/privacy.go:14`、`:28`、`:38` | 清理持capture锁，按policy与State记录名/大小/mtime签名跳过重复扫描，签名相同时仍重查已记录evidence路径安全。 | 这是状态清理优化，不是源码新鲜度验证；须把新记录/来源纳入签名与路径复核。 |
| `repo/internal/app/privacy.go:83`、`:110`、`:175` | active删被规则排除的Files/Omitted；round删对应Changes/Omitted并将Analysis置nil。 | 原基线policy hash保留以供Stop报告范围变化（`:104`）；新缓存也不能只换hash文件名。 |
| `repo/internal/app/privacy.go:118`、`:131`、`:140`、`:163` | 任一explanation证据不允许则删整条解释，再删除源解释不存在的share-draft/correction，最后清理练习。 | 当前有历史与依赖级联清理，但没有新预扫记录分支；D07建议整代际及其后代清除。 |
| `repo/internal/app/round.go:52`、`:125`、`:167` | Start捕获通用基线，Stop捕获after；按当前policy过滤基线和遗漏，policy变化时提示新增允许范围缺可靠基线。 | 轮次操作会读多文件，必须作为D01独立显式操作，不能借用为选区关联热路径。 |
| `repo/internal/app/round.go:83`、`:117`、`:231`、`:302` | Stop重试/已提交轮次返回前scrubCommittedRound，List也先scrub再按Allowed过滤。 | 撤销检查不是只在一次捕获时做，新选区面板/导航也须复查。 |
| `repo/internal/app/recovery.go:19`、`:155` | recordTarget按固定文件名/前缀识别，未知名直接跳过，列表无prescan类型。 | **新增prescan不会自动被识别或隔离，也不会被privacy清掉**；3b必须同时接入recordTarget、版本/语义验证、scrubEntries。 |
| `repo/internal/app/recovery.go:16`、`:58`、`:170`、`:194`、`:197` | recover按exercise→capture锁顺序，清暂存、隔离坏记录、恢复active/apply；通用记录上限256MiB；settings/usage损坏直接失败，其他坏记录隔离。 | 维持锁顺序，预扫需更小上限和schema/血缘校验；不能以“JSON能解码”视为安全。 |
| `repo/internal/app/recovery.go:125`、`:219`、`:230`、`:268` | 隔离目录policy变化会清.bad原文，遗留.intentlens-write-*普通文件删除；非settings/usage现主要做JSON解码/null检查。 | 新缓存坏副本仍含原文，建议撤销/关闭就删而非等待policy变化；临时名须纳入清理，不绕开原子写框架。 |
| `repo/internal/app/revisions.go:208` | 非Go证据freshness路径也会调用capture.Take。 | 与简报“Take只用于round before/after”有出入，以代码为准；不能把旧freshness链路直接复用到受限热路径。 |

### A2. 模型请求守卫：真实定义与覆盖阶段

| 文件:行 | 一句话现有行为 | 对第0步边界的影响/接入建议 |
|---|---|---|
| `repo/internal/app/inspect.go:463`、`:512` | analysisSourcePaths在selection下加入selectionFile，并只放行(selection,selected-code)/(diagnostics,selected-diagnostics)且File必须空；函数模式从File收路径。 | 新类型必须经本次集合/唯一ID/schema校验，来源从本地sources[]收齐，不能只放宽Kind或靠transport.File。 |
| `repo/internal/app/model_connection.go:52`、`:58` | modelRequestConfig与checkModelSourcePaths实际定义在model_connection.go，闭包每次加载最新policy、去重路径并Allowed检查。 | 纠正任务定位：inspect.go是调用处，不是这两个定义；现检查不核验内容hash/新增授权epoch，需新增许可/代际守卫，保留无正文路径安全检查。 |
| `repo/internal/explain/explain.go:24`、`:30`；`repo/internal/explain/providers.go:132` | BeforeRequest不序列化，CheckRequestPolicy失败包装ErrRequestBlocked，client.Do之前执行。 | answer前和实际发生的review前都覆盖；fast没有review，无claim可跳过review，不能说每次总有两次守卫。 |
| `repo/internal/explain/explain.go:291`、`:378`、`:393`、`:395` | answer和review走同发送函数，review被policy阻断时返回空Document与错误，非policy失败可退为待确认。 | 全来源闭包应延续到review，不能只查review被引用子集；文档也需不可信提示。 |
| `repo/internal/app/inspect.go:467`、`:476`、`:481`、`:499` | 全分析共用paths；真实调用结算后返回前再查policy，fake runner替代HTTP时也先CheckRequestPolicy。 | 已覆盖answer前/review前/返回前，但**不自动覆盖新血缘/新许可**；四处控制的面板/导航仍由扩展另做。 |

### A3. 侧栏与整个扩展后台读取

| 文件:行 | 一句话现有行为 | 对第0步边界的影响/接入建议 |
|---|---|---|
| `repo/extension/src/extension.js:444`、`:620`、`:633` | sidebar注入getCurrentModelTarget，要求可信、已保存Go、本地工作区，然后调用inspect并校验编辑器版本/位置epoch。 | inspect会建Go索引；“侧栏不请求模型”不等于不读全库。 |
| `repo/extension/src/sidebar.js:20`、`:37`、`:47`、`:63`、`:149` | refresh只失效/取消/发tree事件；根getChildren才target，经默认150ms防抖并合并同代际pending调用getCurrentTarget。 | **不是选区事件同步立即inspect**；可见树后续取节点时会触发，后台读量仍须纳入D01，隐藏视图是否取节点是宿主行为，未运行核实。 |
| `repo/extension/src/extension.js:917`、`:918`；`repo/extension/src/sidebar.js:103` | 活动编辑器与选区变化均触发refresh，侧栏说明只展示已保存回答且不请求模型。 | 可保留“无模型调用”，不能扩大成“只读取当前文件”。 |
| `repo/extension/src/extension.js:605`；`repo/internal/app/inspect.go:96` | CodeLens也独立调用index，当前buildIndex每次TakeGo。 | D01须约束这个后台入口，不能仅抑制sidebar。 |

### A4. 扩展授权、binaryPath与旧组合

| 文件:行 | 一句话现有行为 | 对第0步边界的影响/接入建议 |
|---|---|---|
| `repo/extension/src/extension.js:847`、`:867`；`repo/extension/src/providers.js:223`、`:243`、`:265`、`:271` | enableModel取status后选择provider/baseURL、输入Key，单独确认GET模型列表、选真实ID/预算/复核模式，再确认启用并保存。 | 现已有接收方确认链，但没有“预扫原文落盘/设计文档分型发送”授权；旧enabled不能自动升级。 |
| `repo/extension/src/extension.js:86`；`repo/internal/app/model_connection.go:31` | 模型命令用SecretStorage凭据注入独立环境变量并带editor target，CLI复核目标provider/baseURL及配置。 | 新关联授权要绑定接收方，继承凭据目标绑定，不能把隐私许可放宽等同换接收方许可。 |
| `repo/extension/src/extension.js:81`、`:97` | 优先自定义binaryPath，否则bundle存在则用bundle，否则PATH的intentlens，每次runCLI。 | 不能假设CLI随扩展同版本；替换二进制需重协商。 |
| `repo/cmd/intentlens/integrations.go:5`、`:27`、`:28`；`repo/cmd/intentlens/main.go:232`、`:260` | integration-info有schema=1和命令描述，无新预扫能力列表；其model_commands未列explain-selection，但main实际支持该命令；未知命令返回错误。 | 现integration-info不是充分能力协商（也不能直接当完整命令真值）；需D05显式可选能力，不按缺少某条旧描述推断实现不存在。 |
| `repo/extension/src/core.js:115`、`:122` | 旧扩展硬拒绝新ID/非空File和无效引用。 | 旧扩展/新CLI必须保持旧默认响应；旧CLI/新扩展应明确升级，当前没有完整版本协商流程。 |

### A5. 现有扩展测试与 Host 集成覆盖

| 文件:行 | 一句话已有断言/场景 | 对7a/8a的接口建议 |
|---|---|---|
| `repo/extension/test/activation.test.js:174`、`:177`、`:184`、`:198`、`:201` | 选区命令测空选区/脏文件/plaintext拒绝及TypeScript成功payload（语言、UTF-16、诊断、保存版本）。 | 保留默认模式；新关联测试复用mock CLI路由，另加能力/授权/schema fixture。 |
| `repo/extension/test/activation.test.js:190`、`:193`、`:214`、`:217`、`:223`、`:225` | 断言只getText(selection)、过滤Warning/选区外Error、无绝对根/邻文件字串、注入转义、半开范围边界、请求中诊断变化不展示。 | 这些是真实存在的旧契约，不整体放宽；新增全库canary和三模式两阶段请求体观测。 |
| `repo/extension/test/core.test.js:160`、`:169`、`:178` | no_errors与error-only envelope、恶意诊断转义、拒非Error/含路径证据。 | 按协商schema分支新增类型与ID闭包测试，保留旧schema拒绝。 |
| `repo/extension/test/sidebar.test.js:73`、`:97`、`:132`、`:161`、`:244` | 覆盖失效节点、无模型生成、防抖合并、取消在途inspect、dispose抑制迟到。 | 可测D01入口抑制/读量，不只测refresh次数；未证明宿主隐藏树行为。 |
| `repo/extension/test/privacy.test.js:59`、`:92`、`:286`、`:300`、`:318`、`:427` | 覆盖ignore事件关面板/清理、证据动作重新检查、主机recordID、撤销/根关闭/旧ID/迟到结果拒绝。 | 新关联导航不能照搬会inspect扫库的handler，扩新manifest/epoch及多来源撤销契约。 |
| `repo/scripts/vscode-integration.cjs:17`、`:31`、`:60`、`:81` | 实际宿主测Go CodeLens/inspect、三个阅读fixture的内联分节、证据/练习面板及命令；报告model_calls=0。 | **没有explainSelection/关联模式Host场景**；新增真实选区/诊断变化/权限迁移/受限读取fixture。 |
| `repo/scripts/vscode-index-integration.cjs:24`、`:39`、`:60`、`:71` | 实际宿主测嵌套工作树排除、非Go不挤Go预算、Go预算遗漏、ignore后拒绝，model_calls=0。 | 复用预算/排除断言但独立验证docs分项、显式刷新、坏缓存及旧版本组合。 |
| `repo/scripts/vscode-rounds-integration.cjs:19`、`:27`、`:38`、`:45` | 实际宿主测integration-info、手动start/stop两轮及恢复命令注册，无自动模型调用。 | 这不是选区上下文验收，不以轮次通过替代7a/9。 |
| `repo/extension/package.json:64`、`:65`、`:66`、`:67` | test用node test，coverage门槛85/86/75，integration用runner，check是枚举src名单。 | 后续新增模块需进入check及coverage；本轮未运行，当前门槛是否通过**未核实**。 |

### A6. 五个 eval 样例的实际组成与 Case 契约

每目录实际业务fixture均为一个同名 `.go`、`go.mod`、`case.json`，另有快照元数据 `_listing.json`；没有调用方独立文件或设计文档（目录组成清单：`repo/eval/explain/cases/cache/_listing.json:1`；`repo/eval/explain/cases/config/_listing.json:1`；`repo/eval/explain/cases/dedupe/_listing.json:1`；`repo/eval/explain/cases/retry/_listing.json:1`；`repo/eval/explain/cases/withdraw/_listing.json:1`）。下表用各真实文件行号核实业务组成，不把_listing当行为证据；五份go.mod均为module evalcase、go1.25（各目录 `go.mod:1`、`:3`）。

| 目录与文件（路径均相对repo） | case.json 实际内容摘要 | 源码核对/边界影响 |
|---|---|---|
| `eval/explain/cases/cache/`：cache.go、go.mod、case.json（`repo/eval/explain/cases/cache/cache.go:1`；`repo/eval/explain/cases/cache/go.mod:1`；`repo/eval/explain/cases/cache/case.json:1`） | file=cache.go/function=Put；expect锁/FIFO最早插入/更新保序/非正limit不限；forbid LRU/TTL（`repo/eval/explain/cases/cache/case.json:2`、`:5`、`:10`） | Cache字段与NewCache、Put在同文件；Put声明含接收者（`repo/eval/explain/cases/cache/cache.go:5`、`:12`、`:17`），不等于已有resolved caller fixture。 |
| `eval/explain/cases/config/`：config.go、go.mod、case.json（`repo/eval/explain/cases/config/config.go:1`；`repo/eval/explain/cases/config/go.mod:1`；`repo/eval/explain/cases/config/case.json:1`） | file=config.go/function=Parse；expect注释空行/行号错误/首个等号/scanner错误；forbid配置格式臆测/env/引号（`repo/eval/explain/cases/config/case.json:2`、`:5`、`:10`） | Parse按Scanner读key=value并返回s.Err（`repo/eval/explain/cases/config/config.go:11`、`:19`、`:25`）；无文档依据。 |
| `eval/explain/cases/dedupe/`：dedupe.go、go.mod、case.json（`repo/eval/explain/cases/dedupe/dedupe.go:1`；`repo/eval/explain/cases/dedupe/go.mod:1`；`repo/eval/explain/cases/dedupe/case.json:1`） | file=dedupe.go/function=Dedupe；expect首次保序/seen集合/复用底层数组；forbid排序/不改输入或新副本（`repo/eval/explain/cases/dedupe/case.json:2`、`:5`、`:9`） | Dedupe用in[:0]与map（`repo/eval/explain/cases/dedupe/dedupe.go:4`、`:5`、`:6`）；是函数解释基线，不是selection fixture。 |
| `eval/explain/cases/retry/`：retry.go、go.mod、case.json（`repo/eval/explain/cases/retry/retry.go:1`；`repo/eval/explain/cases/retry/go.mod:1`；`repo/eval/explain/cases/retry/case.json:1`） | file=retry.go/function=Retry；expect次数/翻倍/末轮不sleep/Join最后错误；forbid jitter/context/并发安全（`repo/eval/explain/cases/retry/case.json:2`、`:5`、`:10`） | 同文件有sleep与Retry，Retry调用sleep（`repo/eval/explain/cases/retry/retry.go:10`、`:13`、`:24`）；已有callee但无独立caller/docs。 |
| `eval/explain/cases/withdraw/`：withdraw.go、go.mod、case.json（`repo/eval/explain/cases/withdraw/withdraw.go:1`；`repo/eval/explain/cases/withdraw/go.mod:1`；`repo/eval/explain/cases/withdraw/case.json:1`） | file=withdraw.go/function=Withdraw；expect拒非正/余额不足/分单位/错误不改余额；forbid并发安全/日志/透支（`repo/eval/explain/cases/withdraw/case.json:2`、`:5`、`:10`） | Account.Balance cents注释与Withdraw方法（`repo/eval/explain/cases/withdraw/withdraw.go:10`、`:15`），不证明作者设计动机或完整调用图。 |

Case 现有字段仅 File/Function/Question/Expect/Forbid，Rule为ID/Any（`repo/cmd/intentlens-eval/score.go:12`、`:17`）；五份case.json均未指定question（各 `case.json:2` 至末尾）。runner复制整个case目录后先读文件、functionLine，再a.Explain，尚无selection分支（`repo/cmd/intentlens-eval/main.go:165`、`:170`、`:175`、`:190`）。评分expect/forbid都只看supported文本（`repo/cmd/intentlens-eval/score.go:65`、`:69`、`:83`）。与评审“当时未提供”的差异是现可确认Put/Parse/Dedupe/Retry/Withdraw和fixture内容，**不能因此说新关联eval已有覆盖**。

**8a接口建议（尚不实现）**：新增显式case kind/schema，selection分支置于functionLine之前；字段包含language、UTF-16 range、saved_version、诊断、纯/关联模式、预扫授权范围、期望缓存状态/代际、预扫与撤销/变更步骤、预期证据类型与来源/支持等级、全展示claim forbid、预算截断/隐私canary。code从fixture按范围切取并校验，避免另抄一份漂移文本；caller/callee/docs fixture显式齐备并显式执行prescan，不靠放文件暗中扫库。旧score单列保基线，新增出处/类型准确率、相关性、召回误报、新鲜度标识与uncertain编造检查。上述建议依据runner/评分缺口，不是现Case已支持这些字段（`repo/cmd/intentlens-eval/main.go:175`；`repo/cmd/intentlens-eval/score.go:17`、`:83`）。

### A7. 仍未核实与开工前批准点

- **未核实**：本HEAD测试/覆盖率/真实Extension Host、Windows路径及目录联接、VSIX、模型质量、I/O量/时延、离线删除失败恢复的运行结果；本轮仅阅读正文，不能用历史报告代替（历史声明见 `repo/docs/verification.md:1`、`:5`；执行入口见 `repo/extension/package.json:64`）。
- **未核实**：VS Code隐藏侧栏是否主动getChildren及取消信号是否及时停止已启动CLI的所有读取；静态只能确认refresh→树失效、target防抖及取消链（`repo/extension/src/sidebar.js:28`、`:37`、`:63`）。D01实施后必须以实际宿主文件访问观测确认。
- **建议批准顺序**：先D01/D02/D04/D09（读取与新鲜度冲突），再D03/D07/D11（授权撤销和披露），再D05/D06/D08/D10/D12/D13；所有框均待产品负责人逐条批准，存在未决项就先改本文，不以实施代码代替产品选择。
