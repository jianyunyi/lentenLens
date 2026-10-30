# CI/CD 配置指南

本文档说明 IntentLens 项目的持续集成和持续部署（CI/CD）配置。

## 📋 目录

- [工作流概览](#工作流概览)
- [CI 工作流](#ci-工作流)
- [安全扫描](#安全扫描)
- [发布流程](#发布流程)
- [依赖管理](#依赖管理)
- [代码质量检查](#代码质量检查)
- [性能测试](#性能测试)
- [必需的 Secrets](#必需的-secrets)
- [故障排查](#故障排查)

---

## 工作流概览

### 自动触发的工作流

| 工作流 | 触发条件 | 目的 |
|--------|---------|------|
| **CI** | Push, PR | 运行测试、检查代码质量 |
| **Security Scanning** | Push, PR, 每日 | 扫描安全漏洞 |
| **Dependency Review** | PR | 审查依赖变更 |
| **Code Quality** | Push, PR, 每周 | 检查代码质量指标 |
| **Performance** | Push to main, PR | 性能基准测试 |

### 手动触发的工作流

| 工作流 | 用途 |
|--------|------|
| **Release** | 创建新版本发布 |

---

## CI 工作流

### 位置
`.github/workflows/ci.yml`

### 功能

1. **Lint and Format Check**
   - Go 代码格式检查 (`gofmt`)
   - Go 静态分析 (`go vet`)
   - 高级静态检查 (`staticcheck`)

2. **Test** (Windows + Linux)
   - Go 单元测试
   - Go 竞态检测
   - 扩展测试 (Node.js)
   - 代码语法检查
   - 性能基准测试
   - 离线预检测试

3. **Coverage**
   - 生成测试覆盖率报告
   - 上传到 Codecov
   - 生成 HTML 覆盖率报告

### 本地运行

```bash
# 格式检查
gofmt -l .

# 静态分析
go vet ./...
staticcheck ./...

# 运行测试
go test ./...
go test -race ./...

# 生成覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## 安全扫描

### 位置
`.github/workflows/security.yml`

### 扫描工具

#### 1. CodeQL
- **语言**: Go, JavaScript
- **功能**: 代码语义分析，发现安全漏洞
- **频率**: Push, PR, 每日

#### 2. Gosec
- **语言**: Go
- **功能**: Go 特定的安全扫描
- **检查项**: SQL 注入、命令注入、弱加密等

#### 3. Nancy
- **功能**: Go 依赖漏洞扫描
- **数据源**: Sonatype OSS Index

#### 4. npm Audit
- **语言**: JavaScript
- **功能**: npm 包漏洞扫描
- **目录**: `extension/`, `scripts/release-tools/`

#### 5. Trivy
- **功能**: 综合漏洞扫描
- **扫描**: 文件系统、依赖、配置

#### 6. Semgrep
- **功能**: 静态应用安全测试 (SAST)
- **规则**: 自动配置 + 社区规则

#### 7. Gitleaks
- **功能**: 密钥泄露检测
- **扫描**: Git 历史中的敏感信息

#### 8. License Check
- **工具**: go-licenses
- **功能**: 检查依赖许可证合规性

### 查看扫描结果

1. 访问 GitHub Security tab
2. 查看 Code scanning alerts
3. 审查 Dependabot alerts

### 本地运行安全扫描

```bash
# Gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec ./...

# Nancy
go install github.com/sonatype-nexus-community/nancy@latest
go list -json -deps ./... | nancy sleuth

# npm audit
cd extension && npm audit

# Gitleaks
docker run -v $(pwd):/path zricethezav/gitleaks:latest detect --source="/path"
```

---

## 发布流程

### 位置
`.github/workflows/release.yml`

### 触发方式

#### 方式 1: 推送版本标签（推荐）

```bash
# 1. 更新版本号
# 编辑 extension/package.json 中的 version 字段

# 2. 更新 CHANGELOG.md
# 将 [Unreleased] 改为新版本号

# 3. 提交并打标签
git add extension/package.json CHANGELOG.md
git commit -m "chore: bump version to 0.5.3"
git tag -a v0.5.3 -m "Release v0.5.3"

# 4. 推送标签（触发发布）
git push origin main
git push origin v0.5.3
```

#### 方式 2: 手动触发

1. 访问 GitHub Actions
2. 选择 "Release" 工作流
3. 点击 "Run workflow"
4. 输入版本号和是否为预发布版

### 发布步骤

1. **Validate**
   - 验证 go.mod 声明 Go 1.24.0
   - 运行所有测试（含竞态检测）
   - 验证扩展测试
   - 检查版本一致性

2. **Build**
   - 使用可复现打包脚本构建
   - 生成 VSIX 包
   - 生成 CLI 可执行文件
   - 创建 manifest.json
   - 生成 SHA256 校验和

3. **Create Release**
   - 从 CHANGELOG.md 提取发布说明
   - 创建 GitHub Release
   - 上传所有构建产物
   - 创建讨论主题（Announcements）
   - 更新 `latest` 标签

4. **Publish Marketplace** (可选，默认禁用)
   - 发布到 VS Code Marketplace
   - 需要配置 `VSCE_PAT` secret

5. **Notify**
   - 通知发布状态

### 启用 Marketplace 发布

1. 访问 [Azure DevOps](https://dev.azure.com)
2. 创建 Personal Access Token (Marketplace: Manage)
3. 在 GitHub 添加 `VSCE_PAT` secret
4. 编辑 `.github/workflows/release.yml`，将 `if: false` 改为 `if: true`

---

## 依赖管理

### Dependabot 配置

位置: `.github/dependabot.yml`

#### 监控范围

- **Go modules** (`/go.mod`)
- **npm** (`/extension`, `/scripts/release-tools`)
- **GitHub Actions** (`.github/workflows`)

#### 更新频率

- Go modules: 每周一
- npm: 每周一
- GitHub Actions: 每月

#### 配置

```yaml
# 每个生态系统的 PR 数量限制
open-pull-requests-limit: 5

# 自动标签
labels: ["dependencies", "go"]

# 审阅者
reviewers: ["jianyunyi"]
```

### Dependency Review 工作流

位置: `.github/workflows/dependency-review.yml`

#### 功能

1. **依赖审查**
   - 检测新增/变更的依赖
   - 识别已知漏洞
   - 检查许可证合规性

2. **Go Mod Tidy 检查**
   - 验证 `go.mod` 和 `go.sum` 是否整洁

3. **Outdated 包检查**
   - 列出过时的 npm 包

4. **SBOM 生成**
   - 软件物料清单 (SPDX + CycloneDX)

### 许可证策略

**允许的许可证**:
- MIT
- Apache-2.0
- BSD-2-Clause
- BSD-3-Clause
- ISC

**拒绝的许可证**:
- GPL-3.0
- AGPL-3.0

---

## 代码质量检查

### 位置
`.github/workflows/code-quality.yml`

### 检查项

#### 1. 复杂度分析 (gocyclo)
- **阈值**: 15
- **指标**: 圈复杂度
- **目的**: 识别过于复杂的函数

#### 2. 代码重复检测 (dupl)
- **阈值**: 100 tokens
- **目的**: 发现重复代码块

#### 3. 无效赋值检查 (ineffassign)
- **目的**: 发现未使用的赋值

#### 4. 拼写检查 (misspell)
- **语言**: 美式英语
- **范围**: 所有文件

#### 5. golangci-lint
- **配置**: `.golangci.yml`
- **linters**: 20+ 个 linters
- **超时**: 10 分钟

#### 6. 测试覆盖率
- **最低要求**: 70%
- **失败条件**: 低于 70%
- **报告**: Codecov

#### 7. 文档质量
- **检查**: README 缺失
- **检查**: CHANGELOG 更新
- **检查**: Markdown 链接有效性

#### 8. 代码度量 (cloc)
- **统计**: 代码行数、注释、空行
- **排除**: vendor, node_modules, .work

### golangci-lint 配置

位置: `.golangci.yml`

启用的 linters:
```yaml
- errcheck      # 未检查的错误
- gosimple      # 简化代码
- govet         # 标准 vet
- ineffassign   # 无效赋值
- staticcheck   # 高级静态分析
- unused        # 未使用代码
- gofmt         # 格式化
- goimports     # import 检查
- misspell      # 拼写
- gocritic      # 综合检查
- gocyclo       # 复杂度
- dupl          # 重复
- gosec         # 安全
```

### 本地运行

```bash
# golangci-lint
golangci-lint run

# 复杂度检查
gocyclo -over 15 .

# 代码重复
dupl -t 100 ./...

# 拼写检查
misspell .

# 代码统计
cloc --exclude-dir=vendor,node_modules .
```

---

## 性能测试

### 位置
`.github/workflows/performance.yml`

### 测试类型

#### 1. Go 基准测试
- 运行所有 `*_test.go` 中的 benchmarks
- 时间: 5 秒每个
- 包含内存分配统计

#### 2. 捕获性能测试
- 专门测试 `internal/capture` 的性能
- 使用合成 1024 个 Go 文件
- 报告 ns/op 和内存使用

#### 3. 内存分析
- 生成内存 profile
- 检测内存泄漏
- 上传 profile 文件

#### 4. CPU 分析
- 生成 CPU profile
- 分析热点函数
- 上传 profile 文件

### PR 性能对比

在 PR 中，工作流会：
1. 运行当前 PR 的 benchmarks
2. Checkout 基准分支
3. 运行基准分支的 benchmarks
4. 在 PR 摘要中对比结果

### 本地性能测试

```bash
# 运行所有 benchmarks
go test -bench=. -benchmem ./...

# 运行特定 benchmark
go test -bench=BenchmarkTakeGoSynthetic -benchtime=5s ./internal/capture

# 生成内存 profile
go test -memprofile=mem.prof -bench=. ./internal/capture
go tool pprof -text mem.prof

# 生成 CPU profile
go test -cpuprofile=cpu.prof -bench=. ./internal/capture
go tool pprof -text cpu.prof

# 可视化 profile (需要 graphviz)
go tool pprof -http=:8080 cpu.prof
```

---

## 必需的 Secrets

在 GitHub repository settings > Secrets and variables > Actions 中配置：

### 可选但推荐

| Secret | 用途 | 获取方式 |
|--------|------|---------|
| `CODECOV_TOKEN` | 上传覆盖率到 Codecov | [codecov.io](https://codecov.io) |
| `SEMGREP_APP_TOKEN` | Semgrep 扫描 | [semgrep.dev](https://semgrep.dev) |
| `GITLEAKS_LICENSE` | Gitleaks 企业版 | [gitleaks.io](https://gitleaks.io) |

### 发布相关（仅在准备发布时需要）

| Secret | 用途 | 获取方式 |
|--------|------|---------|
| `VSCE_PAT` | 发布到 VS Code Marketplace | [Azure DevOps](https://dev.azure.com) |

### 注意

- `GITHUB_TOKEN` 自动提供，无需配置
- 大多数扫描工具在没有 token 时仍可运行（功能受限）

---

## 故障排查

### CI 失败常见原因

#### 1. 测试失败
```bash
# 本地复现
go test -v ./...
go test -race ./...
```

#### 2. 格式检查失败
```bash
# 修复格式
gofmt -w .
git add .
git commit -m "style: format code"
```

#### 3. Lint 失败
```bash
# 本地运行 lint
golangci-lint run

# 查看具体错误
golangci-lint run --verbose
```

#### 4. 覆盖率低于阈值
```bash
# 查看覆盖率
go test -cover ./...

# 查看详细报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

#### 5. 安全扫描发现问题
- 查看 Security tab 的详细报告
- 根据严重性优先修复
- 如果是误报，可以在配置中添加排除

#### 6. 依赖审查失败
```bash
# 检查依赖
go mod tidy
go mod verify

# 更新依赖
go get -u ./...
```

### 工作流调试

#### 启用调试日志

在 repository secrets 中添加：
```
ACTIONS_RUNNER_DEBUG = true
ACTIONS_STEP_DEBUG = true
```

#### 手动触发工作流

大多数工作流支持 `workflow_dispatch`，可以手动触发：
1. 访问 Actions tab
2. 选择工作流
3. 点击 "Run workflow"

#### 查看工作流日志

1. 访问 Actions tab
2. 选择失败的运行
3. 点击失败的作业
4. 查看具体步骤的日志

### 性能问题

如果 CI 运行时间过长：

1. **缓存依赖**
   - Go modules 已缓存
   - npm packages 已缓存

2. **并行执行**
   - 测试使用 matrix 策略并行
   - 可以调整 `max-parallel`

3. **跳过某些检查**
   - 编辑工作流的触发条件
   - 使用 `paths-ignore` 过滤

---

## 最佳实践

### Commit 和 PR

1. **提交前本地测试**
   ```bash
   go test ./...
   golangci-lint run
   ```

2. **保持 PR 小而专注**
   - 每个 PR 解决一个问题
   - 便于审查和回滚

3. **更新 CHANGELOG**
   - 每个 PR 都应更新 CHANGELOG.md
   - 记录在 `[Unreleased]` 部分

### 安全

1. **定期审查 Security alerts**
2. **及时更新依赖**
3. **不要在代码中硬编码密钥**
4. **使用 `.gitignore` 排除敏感文件**

### 发布

1. **语义化版本**
   - MAJOR: 不兼容的 API 变更
   - MINOR: 向后兼容的新功能
   - PATCH: 向后兼容的 bug 修复

2. **发布前检查清单**
   - [ ] 所有测试通过
   - [ ] 文档已更新
   - [ ] CHANGELOG 已更新
   - [ ] 版本号一致
   - [ ] 打包脚本成功运行

3. **发布后验证**
   - 下载 release artifacts
   - 验证 SHA256 校验和
   - 手动测试 VSIX 安装

---

## 参考资源

- [GitHub Actions 文档](https://docs.github.com/en/actions)
- [golangci-lint 文档](https://golangci-lint.run/)
- [Codecov 文档](https://docs.codecov.com/)
- [Dependabot 文档](https://docs.github.com/en/code-security/dependabot)
- [CodeQL 文档](https://codeql.github.com/docs/)
- [Semgrep 文档](https://semgrep.dev/docs/)

---

**最后更新**: 2026-10-30  
**维护者**: jianyunyi
