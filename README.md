# IntentLens · 首个可运行版本

IntentLens 帮助开发者理解 AI 对陌生 Go 模块的修改。本次交付实现 v0.3 的首个 M0/M1 流程：Claude Code 轮次捕获 → Go 函数及直接依赖证据 → 可选模型解释与证据检查 → VS Code 阅读与追问。

**这不是完整 MVP。** 共享版本、逐条纠错、练习生成与验证、评分及将答案应用到原项目尚未实现。完整产品范围仍以同级目录中的 `intentlens-design-v0.3.md` 为准。首次集成已确定为 Claude Code + VS Code。

## 快速试用（Windows x64）

1. 在 VS Code 执行 **Extensions: Install from VSIX…**，选择项目根目录中的 `intentlens-0.1.0-win32-x64.vsix`。安装包已包含 Go 可执行文件，不需要安装 Go 才能使用。
2. 打开并信任一个本地 Go 项目，保存文件。点击函数上方 **IntentLens · 解释与证据**，可浏览静态代码证据、直接依赖及解析限制。
3. 默认不启用模型，不发送代码。要生成解释，先让启动 VS Code 的环境提供 `ANTHROPIC_API_KEY`，再执行 **IntentLens: 启用或调整模型分析**，输入账户可用的模型 ID、累计 token 预算，并确认该项目的数据发送范围。
4. 点击“生成解释”或“自由提问”。回答按具体说法展示证据与检查状态；未知和失败明确显示。历史证据与当前文件分别打开。
5. Explorer 中的 **IntentLens · 轮次** 用于查看 Claude Code hooks 已捕获的变化；手动刷新读取最新轮次。

设置中的 `intentlens.binaryPath` 可指定另一个 CLI 可执行文件路径，不能附带 shell 参数。默认优先使用扩展捆绑的程序。扩展仅支持本地文件工作区，未保存内容不会被分析。

## 接入 Claude Code

将本项目 `bin` 加到运行 Claude Code 的终端 PATH；仅设置 VS Code 的 `intentlens.binaryPath` 不会更改 Claude 的 PATH。

在扩展中执行 **IntentLens: 预览 Claude Code Hook 配置**，或运行：

```powershell
.\bin\intentlens.exe hooks-config
```

将输出的 `hooks` 内容合并到目标项目的 `.claude/settings.json`，保留已有配置：

```json
{
  "hooks": {
    "UserPromptSubmit": [
      { "hooks": [{ "type": "command", "command": "intentlens hook", "timeout": 10 }] }
    ],
    "Stop": [
      { "hooks": [{ "type": "command", "command": "intentlens hook", "timeout": 10 }] }
    ]
  }
}
```

`UserPromptSubmit` 捕获起始基线，`Stop` 保存净变化。修改后又恢复的内容不会计入最终变化；不以 Git HEAD 为基线。来源始终标为未确认，不声称全部修改均由 AI 完成。文件捕获不执行项目代码。

模型已启用时，Stop 启动一个后台进程，为本轮生成一份有证据引用的简述。捕获与模型失败不阻止 Claude 继续工作；诊断见 **查看状态与预算**。捕获扫描仍是同步的，不承诺毫秒级 hook 响应；超时或丢失开始事件时不会伪造完整基线。

事件约定核对自 [Claude Code Hooks reference](https://code.claude.com/docs/en/hooks)。当前环境未找到 Claude Code 命令，已测试官方形状的 JSON 事件，尚未在真实 Claude 会话中确认版本兼容性。扩展没有自动修改任何现有项目的 hook 配置。

## 本地验证，不使用模型

在项目根目录构建并运行：

```powershell
go build -trimpath -o bin/intentlens.exe ./cmd/intentlens
'{"session":"manual"}' | .\bin\intentlens.exe start --root 'D:\your-go-project'
# 编辑并保存项目中的代码
'{"session":"manual"}' | .\bin\intentlens.exe stop --root 'D:\your-go-project'
.\bin\intentlens.exe rounds --root 'D:\your-go-project'
'{"file":"main.go","line":12}' | .\bin\intentlens.exe inspect --root 'D:\your-go-project'
```

CLI 接受 UTF-8 JSON，也兼容 Windows PowerShell 管道的 UTF-8 BOM。无输入参数的命令不会等待 stdin。所有数据命令返回 JSON；普通错误非零退出。`hook` 特意始终成功退出且不向 Claude 注入输出，以免干扰编码。

未结束的轮次不能被另一会话覆盖。确认不需要该轮基线时，可运行 `intentlens abort --root <目录>` 取消。异常退出遗留的锁应先检查状态目录中的 PID，确认相关进程已结束后再手动清理对应 `.lock`，不要在进程仍运行时删除锁。

## 隐私与数据位置

本地数据默认保存到系统用户配置目录下的 `IntentLens/projects/<项目路径摘要>`，Windows 通常为 `%APPDATA%\IntentLens\projects\…`。`status` 返回实际位置。基线、历史 before/after、函数证据与解释都在仓库外；不写到待提交的 `.intent/` 中。

`--state <目录>` 可指定另一个仓库外目录，主要用于隔离测试。拒绝仓库内目录及指向仓库的状态目录链接。

默认排除 `.git`、`.intent`、`.intentignore`、`node_modules`、`vendor`、`.env*`、`*.pem`、`*.key`。所有文件符号链接和目录联接跳过。当前只捕获 UTF-8 文本，单文件最多 1 MiB、单轮快照最多 16 MiB；二进制、超限与无法捕获项列为 omissions，不伪装成新增或删除。

项目根的 `.intentignore` 支持：

```text
# 注释与空行被忽略
private/
*.log
internal/secrets/**
```

无斜杠规则匹配任一路径分量；包含斜杠的规则相对项目根。支持 `*`、`?`、字符组及独立 `**` 段。目录规则排除后代。不支持 `!` 重新包含、绝对路径、反斜杠或 `..`；错误规则导致捕获失败，不能静默放行。

更新排除规则后，下一次本地访问会不可逆地清除相应历史源内容及依赖它的解释。扩展监视 `.intentignore` 变化，关闭缓存的证据面板、刷新并触发清理。已手动打开或另存的快照文本是独立文档，不能远程收回；历史备份也不由本工具清理。

**第一阶段不采集 prompt 和 transcript 原文。** 路径规则无法可靠净化其中嵌入的私有内容，故不直接摄入。动机缺失会明确保留，后续安全采集是剩余工作。普通允许文件中的硬编码秘密不由当前路径过滤器自动识别，应通过排除规则控制发送范围。

## 模型解释的实际边界

- 使用 Anthropic Messages API，密钥只来自环境变量，代码中没有密钥或默认模型 ID。不会自动读取你其他工具的凭据。
- 启用确认覆盖必要代码证据、轮次 before/after 和主动输入的问题。暂停只阻止新请求，已经发出的请求可能仍结束。
- 函数解释只带当前函数及唯一解析的直接依赖。轮次解释带本轮变化的前后文本，超过 40,000 字节时要求按函数展开，不偷偷截掉关键材料。
- 每项说法必须引用给定证据 ID；无效引用和缺乏动机材料的动机说法被剔除。另一次调用只核对证据是否支持说法。检查失败时显示 `uncertain`，不显示为已核实。
- 支持检查仍是模型判断，不是正确性证明。仅存在测试源码时明确未确认运行结果。
- 项目累计预算覆盖生成和检查，不按日自动重置。调用前持久化保守预留，进程中断或请求结果不明确时保留未决用量；不会为省预算把未知调用当免费。主动追加预算通过配置完成。
- 当前展示实际 token 和未决占用；未配置费率时明确费用估算不可用，不生成虚假价格。
- 目前没有缓存。当前仅一项分析任务占用项目分析锁，后续并发请求返回忙碌信息，不排队覆盖历史。
- 历史解释按实际引用片段判断变化；已引用证据未变不代表所有间接依赖未变。轮次简述按其使用的 before/after 版本与当前变化文件比较。

## Go 导航边界

使用 Go 标准库 AST 和类型绑定，定位函数及同一目录、同一包内的直接函数调用，不构建代码、不下载依赖。函数变量、接口和无法确定的 selector 调用保留未解析；竞争声明提供候选，不能任意选择 Linux 或 Windows 版本。

尚未完整处理构建标签、包加载和跨包静态解析。类型与语法错误会显示为限制。CodeLens 当前会重新索引项目，大仓库性能尚未验收。

## 开发与验证

开发需要 Go 1.24+ 和 Node.js 22。Go 代码仅使用标准库；扩展无 npm 运行时依赖。

```powershell
go test ./...
go vet ./...
go build -trimpath -o bin/intentlens.exe ./cmd/intentlens
Push-Location extension
npm test
npm run check
Pop-Location
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke.ps1 -WorkDirectory 'D:\intentlens-test-work'
```

`scripts/smoke.ps1` 使用新建的独立 fixture，不修改真实项目，不调用模型。`scripts/vscode-integration.cjs` 是 VS Code `--extensionTestsPath` 入口，可验证实际激活、CodeLens、证据面板和命令注册；使用独立的 user-data-dir 和 extensions-dir。

打包前把构建后的程序复制到 `extension/bin/intentlens.exe`，然后在 extension 目录运行：

```powershell
npx --yes @vscode/vsce package --allow-missing-repository --skip-license --target win32-x64 --out ../intentlens-0.1.0-win32-x64.vsix
```

本仓库尚未选择开源许可证，当前包标为 UNLICENSED，没有发布到扩展市场。其他平台需要重新构建对应程序再打包。

代码组织：`internal/capture` 捕获与过滤，`internal/codeindex` Go 导航，`internal/explain` 生成和检查，`internal/app` 本地应用，`cmd/intentlens` CLI 与 hooks，`extension` 编辑器集成。已有的 `internal/project` 与 `internal/gonav` 较早实现保留并适配现有捕获结构，当前 CLI 不依赖它们。

## 后续阶段

1. 实测 Claude Code 会话与真实模型调用，完善捕获中断、范围和性能。
2. 共享版本重新生成、共享预览确认、说法纠错及依赖失效。
3. 版本化小修改题、隔离练习目录、执行授权、固定测试评分、应用差异预览。
4. 3–5 位目标用户的真实任务试用，分别验收内容质量与主动重复使用。

以上保持 v0.3 已确认范围，未在本次交付中以空按钮或假数据代替。
