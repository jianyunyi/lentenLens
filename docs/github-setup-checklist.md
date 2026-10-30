# GitHub 仓库设置清单

完成这些步骤以充分利用 IntentLens 的 CI/CD 流程。

## ✅ 必需步骤

### 1. 启用 GitHub Actions

- [ ] 访问 **Settings** → **Actions** → **General**
- [ ] 确保 "Allow all actions and reusable workflows" 已启用
- [ ] 设置 Workflow permissions 为 "Read and write permissions"
- [ ] 勾选 "Allow GitHub Actions to create and approve pull requests"

### 2. 启用 GitHub Security Features

- [ ] 访问 **Settings** → **Security** → **Code security and analysis**
- [ ] 启用 **Dependency graph**
- [ ] 启用 **Dependabot alerts**
- [ ] 启用 **Dependabot security updates**
- [ ] 启用 **Code scanning** (CodeQL)
- [ ] 启用 **Secret scanning**

### 3. 保护主分支

- [ ] 访问 **Settings** → **Branches**
- [ ] 点击 **Add branch protection rule**
- [ ] Branch name pattern: `main`
- [ ] 勾选以下选项:
  - [ ] Require a pull request before merging
  - [ ] Require approvals (建议至少 1 个)
  - [ ] Require status checks to pass before merging
    - [ ] 添加: `lint`, `test (ubuntu-latest)`, `test (windows-latest)`
  - [ ] Require conversation resolution before merging
  - [ ] Do not allow bypassing the above settings

### 4. 配置 GitHub Discussions (可选)

- [ ] 访问 **Settings** → **Features**
- [ ] 启用 **Discussions**
- [ ] 创建分类:
  - General
  - Ideas
  - Q&A
  - Show and tell
  - Announcements

---

## 🔧 可选但推荐的设置

### 5. 配置外部服务集成

#### Codecov (测试覆盖率)

1. [ ] 访问 [codecov.io](https://codecov.io)
2. [ ] 使用 GitHub 登录
3. [ ] 添加 `jianyunyi/lentenLens` 仓库
4. [ ] 复制 **Upload Token**
5. [ ] 在 GitHub 添加 Secret:
   - Name: `CODECOV_TOKEN`
   - Value: [粘贴 token]

#### Go Report Card (代码质量)

1. [ ] 访问 [goreportcard.com](https://goreportcard.com)
2. [ ] 输入仓库 URL: `github.com/jianyunyi/lentenLens`
3. [ ] 点击 "Generate Report"
4. [ ] 等待分析完成（首次可能需要几分钟）

#### Semgrep (安全扫描) - 可选

1. [ ] 访问 [semgrep.dev](https://semgrep.dev)
2. [ ] 注册账号
3. [ ] 添加 GitHub 仓库
4. [ ] 复制 App Token
5. [ ] 在 GitHub 添加 Secret:
   - Name: `SEMGREP_APP_TOKEN`
   - Value: [粘贴 token]

### 6. 配置 GitHub Pages (文档托管) - 可选

- [ ] 访问 **Settings** → **Pages**
- [ ] Source: 选择 branch (如 `gh-pages`) 或 GitHub Actions
- [ ] 自定义域名（可选）

### 7. 添加仓库主题和描述

- [ ] 访问仓库主页
- [ ] 点击齿轮图标 ⚙️
- [ ] 添加描述: "Go function explanations with AI-powered analysis and evidence tracking"
- [ ] 添加主题标签:
  - `go`
  - `golang`
  - `vscode-extension`
  - `ai`
  - `code-analysis`
  - `documentation`
  - `developer-tools`

### 8. 配置 Issue 标签

访问 **Issues** → **Labels**，创建以下标签:

| 标签 | 颜色 | 描述 |
|------|------|------|
| `bug` | `#d73a4a` | Something isn't working |
| `enhancement` | `#a2eeef` | New feature or request |
| `documentation` | `#0075ca` | Improvements or additions to documentation |
| `good first issue` | `#7057ff` | Good for newcomers |
| `help wanted` | `#008672` | Extra attention is needed |
| `security` | `#ee0701` | Security-related issue |
| `performance` | `#fbca04` | Performance improvement |
| `dependencies` | `#0366d6` | Pull requests that update a dependency |
| `needs-triage` | `#d4c5f9` | Needs initial review |
| `wontfix` | `#ffffff` | This will not be worked on |

### 9. 添加 CODEOWNERS 文件

创建 `.github/CODEOWNERS`:

```
# Default owners for everything in the repo
* @jianyunyi

# Go code
*.go @jianyunyi

# Extension code
/extension/ @jianyunyi

# Documentation
/docs/ @jianyunyi
*.md @jianyunyi

# CI/CD
/.github/ @jianyunyi

# Security and privacy
/SECURITY.md @jianyunyi
/PRIVACY.md @jianyunyi
```

---

## 🚀 发布准备（当需要发布时）

### 10. VS Code Marketplace 准备

仅在准备公开发布到 VS Code Marketplace 时需要:

1. [ ] 访问 [Azure DevOps](https://dev.azure.com)
2. [ ] 创建组织和 Personal Access Token
3. [ ] Token scopes: **Marketplace** (Acquire, Manage)
4. [ ] 在 GitHub 添加 Secret:
   - Name: `VSCE_PAT`
   - Value: [粘贴 token]
5. [ ] 编辑 `.github/workflows/release.yml`
6. [ ] 将 `publish-marketplace` job 的 `if: false` 改为 `if: true`

### 11. 配置 GitHub Release Settings

- [ ] 访问 **Settings** → **Options**
- [ ] 确保 **Releases** 已启用
- [ ] 检查发布权限设置

---

## 🧪 验证设置

### 12. 测试 CI/CD Pipeline

1. [ ] 创建测试分支
   ```bash
   git checkout -b test/ci-setup
   echo "# Test" >> TEST.md
   git add TEST.md
   git commit -m "test: verify CI pipeline"
   git push origin test/ci-setup
   ```

2. [ ] 创建 Pull Request
3. [ ] 检查以下 checks 是否运行:
   - [ ] Lint and Format Check
   - [ ] Test (ubuntu-latest)
   - [ ] Test (windows-latest)
   - [ ] CodeQL Analysis
   - [ ] Dependency Review
   - [ ] Code Quality checks

4. [ ] 确认所有 checks 通过
5. [ ] 合并或关闭测试 PR
6. [ ] 删除测试分支

### 13. 验证 Security Scanning

- [ ] 访问 **Security** → **Code scanning**
- [ ] 确认看到 CodeQL 扫描结果
- [ ] 访问 **Security** → **Dependabot**
- [ ] 确认依赖监控已激活

### 14. 测试 Issue 和 PR 模板

- [ ] 创建一个新 Issue，确认看到模板选项
- [ ] 创建一个新 PR，确认看到 PR 模板
- [ ] 不需要实际提交，可以取消

---

## 📊 监控和维护

### 定期检查（每周）

- [ ] 审查 Dependabot PR
- [ ] 检查 Security alerts
- [ ] 查看 Actions 运行状态
- [ ] 审查新的 Issues 和 PRs

### 定期检查（每月）

- [ ] 审查代码覆盖率趋势
- [ ] 检查性能基准测试结果
- [ ] 更新过时的依赖
- [ ] 审查和更新文档

---

## ❓ 故障排查

### Actions 无法运行

**症状**: Workflows 没有触发

**解决方案**:
1. 检查 Settings → Actions 是否启用
2. 确认 workflow 文件格式正确
3. 查看 Actions tab 的错误信息

### Security scanning 失败

**症状**: CodeQL 或其他扫描失败

**解决方案**:
1. 查看具体错误日志
2. 可能是代码问题，需要修复
3. 如果是误报，可以在 Security tab 关闭

### Dependabot 没有创建 PR

**症状**: 依赖更新没有自动创建

**解决方案**:
1. 确认 Dependabot 已启用
2. 检查 `.github/dependabot.yml` 配置
3. 查看 Insights → Dependency graph → Dependabot

### Coverage 上传失败

**症状**: Codecov 报告 "Commit not found"

**解决方案**:
1. 确认已添加 `CODECOV_TOKEN`
2. 检查 token 是否正确
3. 可能需要等待几分钟同步

---

## 📞 获取帮助

如果遇到问题:

1. 查看 [CI/CD Guide](ci-cd-guide.md)
2. 查看 [GitHub Actions 文档](https://docs.github.com/en/actions)
3. 在 [GitHub Discussions](https://github.com/jianyunyi/lentenLens/discussions) 提问
4. 发送邮件至 16671100373@163.com

---

## ✅ 完成

当所有必需步骤都完成后:

- [ ] 在仓库 README 添加状态徽章
- [ ] 通知团队成员关于新的 CI/CD 流程
- [ ] 更新贡献指南（如有需要）
- [ ] 庆祝 🎉

---

**最后更新**: 2026-10-30  
**维护者**: jianyunyi
