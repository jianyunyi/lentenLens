# 本地打包与发布检查

## 可复现的 Windows x64 包

扩展版本以 `extension/package.json` 为准。当前打包目标固定为 `windows/amd64`（VS Code target `win32-x64`）；Go 工具链固定为 1.26.3，源码 `go.mod` 必须仍声明 Go 1.24.0；Node.js 至少 22；`@vscode/vsce` 固定为 4.0.0，完整传递依赖由 `scripts/release-tools/package-lock.json` 锁定。打包器会核对这些声明，不会静默改用其它 Go 版本或目标。

在项目根目录执行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-release.ps1 -OutputDirectory .\dist
```

脚本把扩展复制到系统临时目录，从当前 Go 源码交叉编译捆绑 CLI，不改写 `bin/` 或 `extension/bin/`，然后生成 VSIX。输出目录必须显式指定；若同名包已存在，脚本拒绝覆盖。首次运行需要根据锁文件安装固定版打包工具；离线环境可加 `-Offline`，前提是 npm 缓存已经包含所有锁定依赖。

## 验证包与源文件一致

打包器会在 VSIX 内写入 `extension/release-manifest.json`，记录扩展包内文件的 SHA-256、Go 源树摘要、Go 工具链和打包工具版本。打包完成后会自动对 VSIX 清单做完整性验证。也可以再次针对保留的包核验：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/verify-vsix.ps1 `
  -Vsix .\dist\intentlens-0.5.2-win32-x64.vsix `
  -SourceExtension .\extension `
  -RepositoryRoot .
```

验证会比较归档中每个扩展文件的哈希与内部清单，校验包内的 `README.md`、`src/core.js`、`src/providers.js` 和 CLI 二进制；同时将可直接对照的扩展源文件、Go 源文件及 vsce 锁文件与当前工作树比较。CLI 在临时目录内从被清点的 Go 源码构建，验证器还检查其 Go 版本和 `windows/amd64` 构建信息。

根目录已有的历史 VSIX 被 Git 忽略，不能作为当前工作树的发布依据。打包时应生成新的临时/指定目录候选包并通过以上检查；不会自动安装、发布、提交或推送。

## 尚待所有者决定

哈希通过只代表包内内容与清单一致，不表示可以公开发布。当前 `publisher` 仍为 `intentlens-local`，`license` 仍为 `UNLICENSED`；打包清单会保留这两个值并列出阻断项。公开发布前，项目所有者须提供正式 Marketplace publisher ID，并选择法律适用的许可证、更新 manifest 且提供匹配的 `LICENSE` 文件。打包脚本不会替所有者决定或虚构这些身份/法律信息。
