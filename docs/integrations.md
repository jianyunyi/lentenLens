# IntentLens：IDE 和 Skill 两种入口

IntentLens 的核心是本地 Go CLI，不依赖某个编码代理。IDE 扩展和 Skill 使用同一套捕获、证据、隐私与模型预算规则；Claude Code hooks 只是可选适配器。

## 在 IDE 中使用

VS Code 扩展直接提供函数阅读、追问、共享和练习。无需安装或登录 Claude。

要记录某次修改：先保存文件，执行 **IntentLens: 开始记录本轮修改**；使用任何编码工具或手工修改后保存，再执行 **IntentLens: 结束记录本轮修改**。在 Explorer 的“IntentLens · 轮次”中查看开始到结束的净变化。扩展不会替你保存未保存内容；会话按工作区保存，不能覆盖已经存在的基线。

轮次结束本身不自动请求模型。可直接在轮次面板点击“生成轮次解释 / 重新生成轮次解释”，核对发送提示后确认；失败保留原解释，重复点击不会重复发出请求。也可进入函数阅读入口主动生成函数解释。通用 CLI 可显式调用 `analyze-round --root <项目绝对路径> --round <实际轮次ID>`；未传 `--round` 时从 stdin 接收 `{"round_id":"实际轮次ID"}`。这两种方式都受项目模型配置和 token 预算约束；显式 `--round` 路径不会等待 stdin。

中断或切换入口后，执行 **IntentLens: 恢复未结束的轮次**，确认当前基线的 ID 和时间后再结束记录。恢复只关联已有会话，不重建基线、不确认作者。若决定放弃本轮，执行 **IntentLens: 取消当前轮次（保留源码）**，确认后只丢弃活动基线，不撤销源码，也不删除已完成历史；先确认其它工具已停止使用该轮次。轮次视图标题菜单也包含这些命令，不会自动恢复或自动取消。

其它 IDE 若能启动本地进程，可使用下面的 JSON CLI 协议接入。当前交付的是 VS Code 扩展和通用协议，**不是为每个 IDE 都实现了原生扩展或自动生命周期回调**。兼容编辑器能否运行 VS Code 扩展需在对应宿主验证。

## 通过 Skill 调用

项目包含 [IntentLens Skill](../.cursor/skills/intentlens/SKILL.md)。该项目级目录供支持相应 Skill 发现方式的宿主使用；其它宿主可将整个 `intentlens` Skill 目录复制到其支持的项目级目录，或显式加载此文件。这里不会自动修改你的个人 Skill 目录或其它项目配置。调用名称和语法由宿主决定，典型方式为显式调用 `intentlens`。

Skill 指导代理调用 CLI 读取函数证据、捕获本轮修改，并保留引用、未知和过期提示。仅阅读函数时无需先开轮次，也不需要 Claude 或独立模型 key。若由宿主根据返回证据组织回答，那是**宿主回答**，不是已保存或通过 IntentLens 支持检查的解释。代码进入当前宿主会话仍须遵守用户允许的代码处理范围。

要产生 IntentLens 的保存版模型解释、修订、共享生成或练习，仍须启用项目模型配置及相应发送范围。宿主订阅或登录态不自动变成 IntentLens API 凭据；不复用其它工具的密钥。

## 通用工具协议

运行 `intentlens integration-info` 获取机器可读的入口与命令清单。这个命令不读取项目、不要求 stdin。工具应以本地进程调用可执行文件，参数作为数组传入，不拼接未经处理的 shell 字符串。

其它命令指定 `--root <本地项目绝对路径>`；测试可另指定仓库外的 `--state <目录>`。带输入的命令从 stdin 接收单个 UTF-8 JSON 对象，stdout 返回 JSON；失败非零退出并返回 `error`。解析返回值，不通过模糊文本猜测成功。

`index` 与 `inspect` 都返回 `coverage`。通用宿主应检查 `coverage.complete`、`omitted_count`、`excluded_count` 和 `truncated`：范围不完整时明确报告，不要把未列出的文件/函数说成不存在；`omitted` 最多提供 20 条样例，排除路径只报告汇总数。即使覆盖完整，也要逐项保留 `calls[].status`（`resolved`、`unresolved` 或 `ambiguous`），不能假设每个调用目标都已唯一解析。`integration-info` 的 `index_coverage` 描述该机器契约。

PowerShell 示例（把 CLI 和项目路径替换为真实路径）：

```powershell
$cli = 'C:\tools\intentlens.exe'
$project = 'D:\projects\my-go-project'
$session = [guid]::NewGuid().ToString('N')

# 修改前建立基线；失败时不要继续声称已捕获本轮。
@{event='start'; session=$session} | ConvertTo-Json -Compress | & $cli capture-event --root $project
if ($LASTEXITCODE -ne 0) { throw 'IntentLens 起始捕获失败' }

# 由用户已授权的工具修改项目，并保存文件。

# 完成后捕获净变化；必须使用同一 session。
$raw = @{event='stop'; session=$session} | ConvertTo-Json -Compress | & $cli capture-event --root $project
if ($LASTEXITCODE -ne 0) { throw 'IntentLens 结束捕获失败；保留 session 以便重试' }
$round = $raw | ConvertFrom-Json

& $cli rounds --root $project
'{"file":"main.go","line":12}' | & $cli inspect --root $project
```

`start` / `stop` 及其 `{"session":"..."}` 输入继续兼容。`capture-event` 以统一 `event=start|stop` 接入不同 IDE/代理。它不接收工具专属生命周期的隐式假设，不保存原始 prompt/transcript，不自动启动分析。不能给同一工作区建立重叠轮次；开始失败不算有效基线。来源始终保留 unconfirmed，不能因调用者名称就断言 AI 是全部修改的作者。

没有前置基线时只能阅读当前函数，不能事后伪造一轮净变化。中断后保存原 session 并按错误提示恢复；只有用户决定放弃基线后才能显式 `abort`，不得自动清除其它会话。

安全恢复使用 `capture-status`：返回最小对象 `{"id":"...","session":"...","started":"..."}` 或 `null`，不返回源码快照。安全取消使用 `capture-abort`，stdin 为 `{"session":"实际会话","round_id":"实际轮次ID","confirm":true}`；后端锁内校验两项身份后才删除活动基线，确认期间轮次变化会拒绝。旧 `abort` 为兼容入口，不应代替带身份校验的交互式取消。

## 按入口预检

```powershell
node scripts/preflight.cjs --entry ide
node scripts/preflight.cjs --entry skill
node scripts/preflight.cjs --entry claude
# 仅在计划使用 IntentLens 模型服务时检查所选密钥：
node scripts/preflight.cjs --entry skill --model --key-env OPENAI_API_KEY
```

默认入口是 IDE。本地阅读和捕获不要求 API key；Go 工具链仅在构建 CLI 或执行练习时需要。Skill 的 CLI 模式不要求 VS Code 或 VSIX；Claude 模式才要求 Claude 工具。预检仅发现候选和检查文件，不执行工具、不联网、不证明版本、鉴权、宿主兼容性或真实联调成功。

## 可选 Claude 适配

`hooks-config` / `hook` 继续支持 Claude 的 `UserPromptSubmit` 和 `Stop`。手动合并配置并保留已有 hooks；不自动改你的编辑器或代理设置。该适配器保持原先不阻断编码的行为，错误写本地诊断；与之不同，通用 `capture-event` 明确返回失败，调用工具必须检查退出码。

Claude Stop 在模型已启用时仍保留自动后台解释。不要同时为同一轮开启 IDE 手动捕获与 Claude 自动捕获，它们共享基线并会拒绝重叠。

## 仍然保持的限制

只处理已保存的本地文件，排除策略优先；历史证据不等于当前代码。分享和答案应用仍需各自预览确认，不自动提交或推送。当前版本没有经过验证的 OS 沙箱，`exercise-run`、题目验证和评分均失败关闭；旧授权或 UI 确认不能启动 Go 测试代码。新答案应用只接受单文件变更，多文件计划在写日志或修改源码前拒绝。当前验证覆盖 CLI 与模拟 IDE 接口，不等同于所有 Skill 宿主或真实代理联调均已通过。
