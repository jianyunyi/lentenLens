# 本地打包与发布检查

## 可复现的 Windows x64 包

扩展版本以 `extension/package.json` 为准。当前打包目标固定为 `windows/amd64`（VS Code target `win32-x64`）；Go 工具链固定为 1.26.3，源码 `go.mod` 必须仍声明 Go 1.25.0；Node.js 至少 22；`@vscode/vsce` 固定为 4.1.0，完整传递依赖由 `scripts/release-tools/package-lock.json` 锁定。打包器会核对这些声明，不会静默改用其它 Go 版本或目标。

在项目根目录执行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-release.ps1 -OutputDirectory .\dist
```

脚本把扩展复制到系统临时目录，从当前 Go 源码交叉编译捆绑 CLI，不改写 `bin/` 或 `extension/bin/`，然后生成 VSIX。输出目录必须显式指定；若同名包已存在，脚本拒绝覆盖。首次运行需要根据锁文件安装固定版打包工具；离线环境可加 `-Offline`，前提是 npm 缓存已经包含所有锁定依赖。

## 验证包与源文件一致

打包器会先核对 manifest 与 npm 锁文件中的名称、版本和许可证，并确认根目录是 MIT 许可证文本；当前本地 VSIX 合约仅接受 MIT，不一致时拒绝生成。包内 `extension/release-manifest.json` 记录 `manual-vsix` 渠道、publisher、许可证、包内文件 SHA-256、Go 源树摘要、Go 工具链和打包工具版本。`marketplaceReady` 单独表示 Marketplace 门槛，不代表本地 VSIX 是否可安装。打包完成后会自动核验 VSIX 清单和元数据。也可以再次针对保留的包核验：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/verify-vsix.ps1 `
  -Vsix .\dist\intentlens-0.7.1-win32-x64.vsix `
  -SourceExtension .\extension `
  -RepositoryRoot .
```

验证会比较归档中每个扩展文件的哈希与内部清单，校验包内的 `README.md`、`src/core.js`、`src/providers.js` 和 CLI 二进制；同时将可直接对照的扩展源文件、Go 源文件及 vsce 锁文件与当前工作树比较。CLI 在临时目录内从被清点的 Go 源码构建，验证器还检查其 Go 版本和 `windows/amd64` 构建信息。

根目录已有的历史 VSIX 被 Git 忽略，不能作为当前工作树的发布依据。打包时应生成新的临时/指定目录候选包并通过以上检查；不会自动安装、发布、提交或推送。

## 当前发行状态与后续公开发布条件

当前 `extension/package.json` 的版本为 `0.7.1`、publisher 为 `intentlens-local`、许可证为 MIT；`extension/package-lock.json` 与扩展清单版本一致，根目录 `LICENSE` 为 MIT。当前发行目标是手动安装 VSIX；Marketplace publisher 尚未选定或验证。所有 `manual-vsix` 清单都会固定记录 `marketplacePublisherVerified=false`、`marketplaceReady=false` 和 `publisher_ownership_unverified` 阻断代码；即使 publisher 字符串改变，打包器也不会推断其所有权。校验器拒绝任何声称已验证或已就绪的手动 VSIX 清单。`.github/workflows/release.yml` 只能由手动触发，生成保留 7 天的候选 VSIX 工作流制品，不创建 GitHub Release、不发布 Marketplace。`intentlens.intentlens` 已被一个功能不同的公开扩展使用，因此不可把 `intentlens` 当作本项目的 Marketplace publisher。若以后公开发布，须新增可独立核验的账户所有权流程；哈希通过只代表包内内容与清单一致，不代表可以公开发布或满足生产验收。

## 自动化候选包门禁

`.github/workflows/reliability.yml` 在 PR、main push 和手动触发时执行 Linux/Windows 检查；保留 Go JSON 事件，Windows junction 测试必须实际 PASS。使用 release-config.json 固定的发行 Go 版本和 Node major，另以 Go 1.25.0 验证最低源码版本。候选包作业依赖全部测试，再执行现有打包/校验与两类损坏拒绝回归，仅保留 7 天制品。

工作流存在不代表某次运行通过，也不自动建立分支保护。仓库所有者应在收到真实运行结果后将相应 test、minimum-go、candidate 检查绑定为 main 必需检查。不能以 Linux 交叉编译替代 Windows 测试。校验保证可信流水线候选包的内容一致性，不是任意恶意包的签名认证。
