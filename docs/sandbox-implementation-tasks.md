# 沙箱实现任务清单

**状态**: 历史跨平台任务清单；现由 [S6 Windows 优先设计](production/sandbox-execution-design.md) 取代，尚未实现
**优先级**: P0（阻断题目验证与评分）

本清单原定 Linux MVP 优先级已取消。当前发行物是 Windows x64 VSIX，先实现并验证 Windows AppContainer；Linux/macOS 在另行设计和验证前保持 fail-closed。预计日期不再有效。

---

## 📋 Epics 和任务

### Epic 1: 沙箱框架和测试基础设施

**描述**: 建立跨平台沙箱抽象层和测试工具  
**工作量**: 1-2 周

#### 任务 1.1: 完善 `internal/sandbox/common.go`

- [ ] 完成 `Config` 结构体验证
- [ ] 实现 `Result` JSON marshaling
- [ ] 添加错误类型定义
- [ ] 编写单元测试

**工作量**: 2-3 天  
**依赖**: 无

#### 任务 1.2: 创建测试基础设施

**新增文件**:
```
internal/sandbox/
├── test/
│   ├── fixtures.go         # 测试代码片段
│   ├── runner.go           # 运行测试函数
│   └── verify.go           # 验证隔离结果
├── common_test.go          # 通用测试
├── testdata/
│   ├── legitimate/         # 合法代码示例
│   ├── malicious/          # 恶意代码示例
│   └── edge_cases/         # 边界情况
```

- [ ] 创建测试 fixture 存储
- [ ] 实现测试代码运行器
- [ ] 创建验证函数库
- [ ] 编写 10+ 个测试用例

**工作量**: 3-4 天  
**依赖**: 1.1

#### 任务 1.3: 平台抽象层和工厂函数

- [ ] 完成 `sandbox.New()` 工厂函数
- [ ] 实现平台检测逻辑
- [ ] 添加功能检测函数
- [ ] 编写集成测试

**工作量**: 2-3 天  
**依赖**: 1.1

---

### Epic 2: Linux 沙箱实现

**描述**: 使用 seccomp + cgroup v2 实现 Linux 隔离  
**工作量**: 3-4 周  
**优先级**: 1 (MVP 优先)

#### 任务 2.1: Cgroup v2 集成

**新增文件**:
```
internal/sandbox/linux/
├── cgroup.go               # cgroup 设置
├── cgroup_test.go          # cgroup 测试
└── cgroup_limits.go        # 资源限制参数
```

**子任务**:
- [ ] 实现 cgroup 目录创建
- [ ] 实现内存限制设置 (500 MiB)
- [ ] 实现 CPU 限制设置 (10s)
- [ ] 实现进程数限制 (1)
- [ ] 实现文件描述符限制 (256)
- [ ] 编写验证测试

**工作量**: 1 周  
**依赖**: 1.3

#### 任务 2.2: seccomp-bpf 集成

**新增文件**:
```
internal/sandbox/linux/
├── seccomp.go              # seccomp 过滤器
├── seccomp_test.go         # seccomp 测试
└── syscalls.go             # 系统调用白名单/黑名单
```

**子任务**:
- [ ] 定义允许的系统调用白名单 (read, write, open, etc.)
- [ ] 定义禁止的系统调用黑名单 (socket, connect, execve, fork, etc.)
- [ ] 实现 seccomp-bpf 过滤器生成
- [ ] 集成 libseccomp Go 绑定
- [ ] 编写验证测试

**工作量**: 1.5 周  
**依赖**: 1.3

#### 任务 2.3: Linux 沙箱完整实现

**修改文件**: `internal/sandbox/linux/sandbox.go`

- [ ] 完成 `Run()` 方法实现
- [ ] 完成 `setupCgroup()` 方法
- [ ] 完成 seccomp 过滤器应用
- [ ] 实现进程监控和输出捕获
- [ ] 实现超时处理
- [ ] 实现资源超限检测

**工作量**: 1 周  
**依赖**: 2.1, 2.2

#### 任务 2.4: Linux 安全测试

**新增文件**:
```
internal/sandbox/linux/
└── security_test.go        # 安全测试套件
```

**测试场景**:
- [ ] 文件系统逃逸尝试 (../../ 穿越)
- [ ] 目录穿越尝试 (../../../etc/passwd)
- [ ] 网络访问尝试 (socket, connect)
- [ ] 子进程启动尝试 (fork, clone)
- [ ] 资源炸弹 (内存无限分配)
- [ ] CPU 炸弹 (无限循环)
- [ ] 环境变量泄露 (读取 API 密钥)
- [ ] 符号链接攻击

**工作量**: 1 周  
**依赖**: 2.3

#### 任务 2.5: Linux 功能测试

**新增文件**:
```
internal/sandbox/linux/
└── functional_test.go      # 功能测试套件
```

**测试场景**:
- [ ] 基本代码执行
- [ ] 文件 I/O (工作目录内)
- [ ] 标准库使用
- [ ] 测试输出捕获
- [ ] 测试通过/失败判定
- [ ] 超时处理
- [ ] 错误处理

**工作量**: 1 周  
**依赖**: 2.3

---

### Epic 3: Windows 沙箱实现

**描述**: 使用 AppContainer + Job Object 实现 Windows 隔离  
**工作量**: 4-5 周  
**优先级**: 2 (Beta 版本)

#### 任务 3.1: Windows API 绑定

**新增文件**:
```
internal/sandbox/windows/
├── appcontainer.go         # AppContainer WinAPI 绑定
├── token.go                # Token 管理
├── acl.go                  # ACL 配置
└── job.go                  # Job Object 管理
```

**子任务**:
- [ ] 实现 `CreateAppContainerProfile()` 绑定
- [ ] 实现 `CreateRestrictedToken()` 绑定
- [ ] 实现 `CreateProcessAsUserW()` 绑定
- [ ] 实现 `CreateJobObject()` 和 `SetInformationJobObject()` 绑定
- [ ] 实现 `SetFileSecurity()` for ACL 配置
- [ ] 编写 WinAPI 测试

**工作量**: 1.5 周  
**依赖**: 1.3

#### 任务 3.2: AppContainer 配置管理

**新增文件**: 修改 `internal/sandbox/windows/sandbox.go`

- [ ] 实现 AppContainer SID 生成
- [ ] 实现 AppContainer Profile 创建
- [ ] 实现 AppContainer Profile 清理
- [ ] 实现 ACL 配置 (工作目录、Go 二进制、系统库)
- [ ] 实现错误恢复和清理

**工作量**: 1 周  
**依赖**: 3.1

#### 任务 3.3: Job Object 资源限制

**新增文件**: 修改 `internal/sandbox/windows/job.go`

- [ ] 实现内存限制设置 (500 MiB)
- [ ] 实现进程限制设置 (1)
- [ ] 实现 I/O 限制设置
- [ ] 实现 CPU 时间限制 (10s)
- [ ] 实现资源超限检测

**工作量**: 1 周  
**依赖**: 3.1

#### 任务 3.4: Windows 沙箱完整实现

**修改文件**: `internal/sandbox/windows/sandbox.go`

- [ ] 完成 `Run()` 方法实现
- [ ] 实现受限 Token 创建
- [ ] 实现进程创建和监控
- [ ] 实现输出捕获
- [ ] 实现超时处理
- [ ] 实现资源超限检测

**工作量**: 1.5 周  
**依赖**: 3.2, 3.3

#### 任务 3.5: Windows 安全测试

**新增文件**:
```
internal/sandbox/windows/
└── security_test.go        # 安全测试套件
```

**测试场景**: (同 Linux，针对 Windows 调整)

**工作量**: 1.5 周  
**依赖**: 3.4

#### 任务 3.6: Windows 功能测试

**新增文件**:
```
internal/sandbox/windows/
└── functional_test.go      # 功能测试套件
```

**测试场景**: (同 Linux)

**工作量**: 1 周  
**依赖**: 3.4

---

### Epic 4: macOS 沙箱实现

**描述**: 使用 macOS Sandbox (launchd) 实现隔离  
**工作量**: 2-3 周  
**优先级**: 3 (长期)

#### 任务 4.1: Sandbox DSL 配置

**新增文件**:
```
internal/sandbox/macos/
├── sandbox.sb              # Sandbox 配置 DSL
├── profile.go              # 配置生成
└── profile_test.go         # 配置测试
```

- [ ] 编写 Sandbox DSL 配置 (sandbox.sb)
- [ ] 实现配置验证
- [ ] 实现动态路径替换
- [ ] 编写配置测试

**工作量**: 1 周  
**依赖**: 1.3

#### 任务 4.2: macOS Sandbox 集成

**新增文件**: 修改 `internal/sandbox/macos/sandbox.go`

- [ ] 实现 `sandbox_init()` cgo 绑定
- [ ] 实现配置加载
- [ ] 实现进程执行
- [ ] 实现错误处理

**工作量**: 1 周  
**依赖**: 4.1

#### 任务 4.3: macOS 测试和验证

- [ ] 编写安全测试
- [ ] 编写功能测试
- [ ] 验证隔离有效性

**工作量**: 1 周  
**依赖**: 4.2

---

### Epic 5: 集成和启用

**描述**: 集成沙箱到 exercise-run 命令并启用功能  
**工作量**: 2-3 周

#### 任务 5.1: 集成到 exercise-run

**修改文件**: `internal/exercise/run.go`

- [ ] 修改 `verifiedOSSandboxAvailable` 常量
  - [ ] Linux: 设置为 true
  - [ ] Windows: 条件设置 (检查权限)
  - [ ] macOS: 条件设置 (检查版本)
- [ ] 实现 `Run()` 函数使用沙箱
- [ ] 实现错误处理和恢复
- [ ] 编写集成测试

**工作量**: 1 周  
**依赖**: 2.5 (Linux), 或 3.6/4.3 (其他平台)

#### 任务 5.2: 文档更新

**修改文件**: 
- [ ] SECURITY.md - 更新沙箱功能
- [ ] README.md - 文档 exercise-run 支持
- [ ] docs/production-acceptance-checklist.md - 标记完成

**工作量**: 2-3 天  
**依赖**: 5.1

#### 任务 5.3: 发布准备

- [ ] 性能基准测试
- [ ] 端到端测试
- [ ] 文档完整性检查
- [ ] 版本号更新

**工作量**: 1 周  
**依赖**: 5.1, 5.2

---

## 📊 依赖图

```
Epic 1 (框架)
    ├─→ Epic 2 (Linux) ──→ Epic 5 (集成)
    ├─→ Epic 3 (Windows) ──→ Epic 5
    ├─→ Epic 4 (macOS) ──→ Epic 5
    └─→ Epic 5 (启用和发布)
```

**关键路径**:
1. Epic 1 (2 周)
2. Epic 2 (4 周) - MVP
3. Epic 5 (3 周)
4. Epic 3 (5 周) - Beta
5. Epic 4 (3 周) - 长期

**关键路径总时间**: ~11-12 周 (MVP + Windows)

---

## 🎯 里程碑

### 里程碑 1: MVP 基础 (2 周)
- [x] Epic 1 完成
- [ ] 框架验收和代码审查

### 里程碑 2: Linux MVP (4 周后)
- [ ] Epic 2 完成
- [ ] Linux 安全和功能测试通过
- [ ] 准备好启用 Linux 上的 exercise-run

### 里程碑 3: MVP 启用 (7 周后)
- [ ] Epic 5.1/5.2 完成
- [ ] Linux 的 exercise-run 启用
- [ ] 文档更新完成

### 里程碑 4: Windows 支持 (12 周后)
- [ ] Epic 3 完成
- [ ] Windows 安全测试通过
- [ ] Windows 版 exercise-run 启用

### 里程碑 5: macOS 支持 (15 周后)
- [ ] Epic 4 完成
- [ ] macOS 安全测试通过
- [ ] 全平台支持完成

---

## ✅ 定义完成条件

### Linux MVP (可发布)
- [x] 所有 Epic 2 任务完成
- [ ] 通过所有安全测试 (100%)
- [ ] 通过所有功能测试 (100%)
- [ ] 性能基准 < 2s 额外延迟
- [ ] 文档完整
- [ ] Code review 通过
- [ ] verifiedOSSandboxAvailable = true (Linux)

### 完整版本 (可发布)
- [ ] 上述 Linux MVP
- [ ] Windows 实现完成
- [ ] macOS 实现完成
- [ ] 跨平台测试通过
- [ ] 文档完整
- [ ] 外部安全审计 (可选)

---

## 📝 下一步行动

1. **立即** (本周)
   - [ ] 代码审查 `docs/sandbox-implementation-plan.md`
   - [ ] 审查初始代码框架
   - [ ] 分配开发人员到 Epic 1

2. **第 1-2 周**
   - [ ] 完成 Epic 1 所有任务
   - [ ] 建立开发环境和工具链
   - [ ] 开始 Epic 2.1 (Cgroup)

3. **第 3-6 周**
   - [ ] 完成 Linux 实现 (Epic 2)
   - [ ] 大量安全测试
   - [ ] 性能优化

4. **第 7-9 周**
   - [ ] 完成集成 (Epic 5)
   - [ ] 发布 Linux MVP

5. **第 10+ 周**
   - [ ] Windows/macOS 实现
   - [ ] 完整版本发布

---

## 🔗 相关文档

- [沙箱实现计划](sandbox-implementation-plan.md) - 详细的技术设计
- [生产验收清单](production-acceptance-checklist.md) - 发布要求
- [SECURITY.md](../SECURITY.md) - 安全政策

---

**最后更新**: 2026-10-30  
**负责人**: 待分配  
**状态**: 设计完成，待启动
