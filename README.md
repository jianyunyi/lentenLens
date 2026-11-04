# IntentLens

理解一轮代码修改，而不只是读一段 AI 总结。

IntentLens 面向需要接手陌生 Go 代码的开发者：记录修改前后的本地净变化，定位函数及直接依赖，把可选的模型解释与具体源码证据关联起来，并提供追问、纠错、团队共享和有限单包练习。

核心是本地 Go CLI。当前提供 VS Code 原生扩展、项目级 Skill 和通用 JSON CLI 协议；Claude Code hooks 是可选适配器，不是必需依赖。

> 当前源码版本为 **0.5.2，开发预览**。尚未完成真实用户验收，不应视为生产就绪产品。
>
> 当前 publisher 为 `intentlens-local`，许可证字段为 `UNLICENSED`，尚无正式发行授权。本仓库不提供 Marketplace 正式发行承诺；源码可见不等于获得开源许可证授权。

## 可以做什么

- **按轮次理解修改：** 在修改前建立基线，修改并保存后记录净变化，查看历史 before/after。支持中断恢复，以及保留源码的取消操作。
- **阅读函数及证据：** 查看 Go 函数、同包内能够唯一解析的直接调用及其源码位置；明确显示未知、歧义和索引范围限制。
- **可选模型解释：** 生成函数或轮次解释、自由提问，并按具体说法展示证据引用及支持检查状态。
- **纠错与共享：** 本地标错、生成保留历史的修订版；完整预览并确认后，才写入可由团队提交的共享 JSON。
- **有限练习：** 从行为说法生成小修改题，预览执行范围后验证、作答和评分；应用回原项目仍需单独预览确认。

本地 CLI 阅读和轮次捕获不要求模型密钥，也不会自动请求模型。通过 Skill 把证据交给宿主时，仍须遵守该宿主和用户允许的代码处理范围；宿主回答不等于 IntentLens 保存并检查过的模型解释。

## 从源码开始

### 环境要求

- Go 1.25+：构建 CLI 或运行练习需要。
- Node.js 22：扩展开发和测试需要。
- VS Code 1.90+：使用当前原生扩展需要。
- 已保存、受信任的本地文件工作区；不支持虚拟工作区。

Go 代码仅使用标准库，扩展没有 npm 运行时依赖。

在仓库根目录构建 Windows CLI：

```powershell
go build -trimpath -o bin/intentlens.exe ./cmd/intentlens
```

### 在 VS Code 中试用

按[本地打包说明](docs/release.md)生成并校验新 VSIX，再执行 **Extensions: Install from VSIX…** 手动安装。

当前打包目标仅为 Windows x64。可复现打包要求 Go 1.25.0、Node.js 22+，并使用锁文件固定的 `@vscode/vsce` 4.0.0：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File scripts/package-release.ps1 -OutputDirectory .\dist
```

脚本不会自动安装、发布、提交或推送。打包成功及哈希一致不代表已经满足公开发行条件。

打开并信任一个本地 Go 项目，保存文件后：

1. 点击函数上方 **IntentLens · 在函数下方展开解释**，查看函数证据和已有解释。
2. 修改前执行 **IntentLens: 开始记录本轮修改**。
3. 使用任何编码工具或手工修改，保存后执行 **IntentLens: 结束记录本轮修改**。
4. 在 Explorer 的 **IntentLens · 轮次** 中查看净变化。
5. 需要保存版模型解释时，先启用项目模型，再主动点击生成并确认发送。

未保存内容不会被分析。`intentlens.binaryPath` 可指定另一 CLI 可执行文件，但不能附带 shell 参数。

中断后可执行“恢复未结束的轮次”，关联已有基线；决定放弃时使用“取消当前轮次（保留源码）”。操作前应确认其它工具没有继续使用该轮次。取消不撤销源码，也不删除已完成历史。

### 只用 CLI

```powershell
$project = 'D:\projects\my-go-project'
$session = [guid]::NewGuid().ToString('N')

@{session=$session} | ConvertTo-Json -Compress |
  .\bin\intentlens.exe start --root $project

# 修改并保存项目文件

@{session=$session} | ConvertTo-Json -Compress |
  .\bin\intentlens.exe stop --root $project

.\bin\intentlens.exe rounds --root $project

'{"file":"main.go","line":12}' |
  .\bin\intentlens.exe inspect --root $project
```

数据命令使用 UTF-8 JSON，普通错误非零退出。开始失败时不能声称已捕获本轮；没有前置基线时不能事后还原完整净变化。

## Skill、其它 IDE 与 Claude Code

项目级 Skill 位于 [`.cursor/skills/intentlens/SKILL.md`](.cursor/skills/intentlens/SKILL.md)。其它 Skill 宿主可按自己的发现约定加载同一目录；具体调用语法由宿主决定。

`intentlens integration-info` 返回机器可读契约。通用捕获入口为：

```text
intentlens capture-event --root <项目绝对路径>
```

stdin 接收：

```json
{"event":"start","session":"同一唯一ID"}
```

结束时使用相同 session，将 event 改为 `stop`。当前交付通用协议，不代表每个 IDE 已有原生扩展。

Claude Code 用户可主动预览 `hooks-config`，再手动合并到项目设置；工具不会自动覆盖已有 hooks。模型启用后，Claude Stop hook 可以触发后台轮次解释。不要为同一轮同时开启手动捕获与自动 hooks。

详细流程、恢复协议和预检见 [IDE / Skill 接入说明](docs/integrations.md)。

## 可选模型分析

默认不启用 IntentLens 外部模型请求。执行 **IntentLens: 启用或调整模型分析**，核对：

- 服务商、API 基础地址及准确模型 ID；
- 密钥环境变量名称；
- 项目累计 token 上限；
- 接收地址、发送范围和复核策略。

密钥由启动 VS Code 或 CLI 的环境提供，不写入项目配置或共享记录，不自动读取其它工具的凭据。

实现支持 Anthropic Messages、OpenAI Chat Completions / 兼容服务和 Gemini generateContent；预设包含 OpenAI、Anthropic、Gemini、DeepSeek、通义千问。协议适配不等于所有服务及模型均已实测兼容。

默认 `light` 使用生成和独立支持检查两个阶段；`fast` 在小范围内不做独立复核，结果保持待确认；超出阈值会升级为 `strict`。**任何模式的模型检查都不是正确性证明。**

请求可能包含获准源码、轮次前后文本和主动输入的问题；练习反馈还包含答案与理由。暂停只阻止新请求，已经发出的请求可能继续完成。累计 token 上限不是货币费用上限；未知用量保留保守未决占用，不自动重试。

详见[模型接入](docs/providers-setup.md)和[复核模式](docs/verification-modes.md)。

## 纠错与团队共享

可对具体说法标记“有误 / 证据不支持”。原因保存在本地；生成修订会创建新版本并保留旧版本，不保证模型已经纠正原问题。

“准备共享”依据获准证据重新生成，计入项目模型预算。完整预览全部 JSON 后，只有明确确认才写入：

```text
.intent/shared/<id>.json
```

取消或关闭不写入；预览后证据、排除规则或纠错状态变化会拒绝确认。工具不会执行 Git 提交或推送。

团队成员可读取共享解释并建立自己的本地纠错副本。个人标错不会自动通知其他成员，新的修订仍需再次预览共享。已经提交到 Git 历史的内容无法由本地排除规则远程收回。

## 小修改练习及执行风险

练习限于可独立构建的 Go 单包：最多 20 个 Go 文件、60,000 字节，仅支持允许的标准库导入。不支持外部模块、cgo、TestMain、编译指令或嵌入/外部测试资源。

**当前版本的 `exercise-run`、题目验证和评分均已关闭。** 仓库没有经过验证的 OS 沙箱，因此 CLI、旧授权记录或 UI 确认都不会启动 Go 测试代码；题目查看、本地答案编辑及其他各自确认的功能仍按原流程工作。限时、模块离线、目录隔离或人工确认都不等同于操作系统沙箱。

源码答案应用也只接受单个 Go 文件变更。多文件计划会在写恢复日志或替换任何源码前拒绝；无法在普通文件系统上承诺多文件同时可见。单文件替换的应用级崩溃回滚不构成断电或文件系统损坏恢复保证。

共享题目不包含参考解、个人答案、成绩或执行授权。接收者没有参考解，本地验证和评分会保留不完整或暂定标记。应用答案到原项目仍需预览确认，冲突时停止。

## 隐私与本地数据

默认状态保存在仓库外的系统用户配置目录：

```text
IntentLens/projects/<项目路径摘要>
```

Windows 通常位于 `%APPDATA%\IntentLens\projects\…`，`status` 返回实际位置。历史源码、解释、纠错、草稿、题目和个人作答默认不写入源码仓库；明确确认的共享文件除外。

默认跳过 `.git`、`.intent`、`node_modules`、`vendor`、`.env*`、`*.pem`、`*.key` 等路径，以及文件符号链接和目录联接。嵌套 `.worktrees`、`.cache`、`.pnpm-store` 也默认排除。

项目根 `.intentignore` 可进一步排除：

```text
private/
*.log
internal/secrets/**
```

支持 `*`、`?`、字符组及独立 `**` 段；不支持 `!` 重新包含、绝对路径、反斜杠或 `..`。错误规则使捕获失败，不会静默放行。

更新排除规则后，后续访问会清理相应本地历史及依赖内容；已打开、另存或备份的副本不能远程收回。当前不直接采集原始 prompt/transcript。路径过滤不能自动识别允许文件内的硬编码秘密，发送前仍需人工检查。

## 已知限制

- Go 导航主要覆盖同目录、同包内可唯一解析的直接函数调用；不完整支持构建标签、跨包加载、接口及动态调用。
- 历史证据不等于当前代码，工作区净变化也不能证明作者身份或设计动机。
- 索引完整不代表每个调用均已解析；缺失、歧义和过期信息必须保留。
- CodeLens 会重新索引，大型仓库性能尚未完成验收。
- 自动化 fixture 与宿主回归不等于真实模型、Claude 会话和所有 Skill 宿主均已验证。
- 尚未完成目标用户持续使用与解释质量验收。

## 可靠性验收

本轮改造的故障恢复、预算一致性、CI 和试用状态见 [阶段报告](docs/production/stage-report.md)。升级前停止所有旧版进程；恢复规则见 [恢复操作](docs/production/recovery-runbook.md)。源码要求 Go 1.25+；CI 仅生成候选 VSIX，不自动公开发布。Windows CI 与真实团队试用是否通过以阶段报告的实际证据为准。

## 开发检查

```powershell
go test ./...
go vet ./...
go build -trimpath -o bin/intentlens.exe ./cmd/intentlens

Push-Location extension
npm test
npm run check
Pop-Location

node --test scripts/preflight.test.cjs
```

受控冒烟检查：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File scripts/smoke.ps1 -WorkDirectory 'D:\intentlens-test-work'
```

冒烟脚本使用新建的独立 fixture，不修改真实项目，不调用模型。VS Code 宿主集成入口位于 `scripts/vscode-*-integration.cjs`；应使用独立测试工作区、user-data-dir 和 extensions-dir，不能把普通单元测试通过当作宿主集成通过。

代码主要分布在 `internal/capture`、`internal/codeindex`、`internal/explain`、`internal/app`、`internal/exercise`、`cmd/intentlens` 与 `extension`。较早的 `internal/project`、`internal/gonav` 实现仍保留，当前 CLI 不依赖它们.
