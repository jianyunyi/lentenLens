# IntentLens for VS Code

本地 Go 函数证据、按轮次保存的工作区净变化，以及可选的 Anthropic 解释。需要 VS Code 1.90+ 和 IntentLens CLI。没有 npm 运行时依赖。

## 启动

1. 在扩展 `bin/` 放置对应平台的 `intentlens.exe` / `intentlens`，或将 CLI 加入 PATH；也可配置 `intentlens.binaryPath` 为可执行文件路径（不接受 shell 参数）。
2. 打开并信任本地 Go 工作区。多根工作区按文件所属根目录分析；项目级命令会让你选择根目录。
3. 保存 Go 文件，点击函数上方 `IntentLens · 解释与证据`，或执行同名命令。当前分析依据磁盘版本；未保存文件不展示 CodeLens。
4. Explorer 中展开 `IntentLens · 轮次`。点击轮次查看净变化、before/after 快照与已保存分析；使用刷新命令读取新的 hook 结果。

## 可选模型分析

默认只有本地证据。执行 `IntentLens: 启用或调整模型分析`，输入准确的 Anthropic 模型 ID 和项目累计 token 上限，明确同意代码证据与问题外发后启用。API 密钥通过启动 VS Code 时继承的 `ANTHROPIC_API_KEY` 提供；扩展不读取输入、不存储密钥。修改环境变量后可能需要重新启动 VS Code。

函数面板支持生成解释和自由提问。解释按 claim 展示证据引用与 supported / unsupported / uncertain 状态，保留模型未能确认的内容。模型支持检查也可能出错。查看状态命令显示实际 token、保守未决占用与 CLI 返回的费用说明；token 上限不是货币费用上限。暂停命令阻止新的分析，已经发出的请求可能仍完成。

## Claude Code hooks

执行 `IntentLens: 预览 Claude Code Hook 配置`，将生成的 UserPromptSubmit / Stop 配置手动合并到 Claude Code 设置，保留已有 hooks。扩展不会修改设置文件。CLI 必须在 Claude Code 进程的 PATH 中；扩展的 binaryPath 设置不会配置该 PATH。启用模型后 Stop hook 可触发远程轮次分析。

## 证据语义与限制

- 历史证据原文打开已捕获的文本，当前文件按钮打开现在的文件，两者可能不同。所有当前文件导航验证路径位于真实工作区内，包括符号链接目标。
- 静态 Go 索引保守处理调用关系，不执行构建，不下载依赖，不能解析所有动态调用。缺失证据、无法唯一解析的目标会显示为分析限制。
- 轮次表示工作区变化，不能单凭结果确认变更作者或作者动机；测试源码不能证明测试已执行。
- CLI 遵循其排除策略。监视到 `.intentignore` 新建、修改或删除（含外部编辑）时，关闭受影响工作区的证据/轮次面板、刷新列表并应用持久化隐私清理。每次证据导航和分析前重新读取策略允许的证据；内容变化时先刷新面板，请重新选择操作。已经手动打开的历史快照文本是独立编辑器文档，需要自行关闭。
- 本扩展只支持本地文件工作区。CodeLens 每次请求重新调用本地索引，大型仓库可能有延迟。
- 自动化测试覆盖 CLI JSON 桥接、HTML/CSP、路径 confinement 和注册契约；真实 VS Code Extension Host 交互仍需手动验收。

运行 `npm test` 与 `npm run check` 验证，不需要 `npm install`。
