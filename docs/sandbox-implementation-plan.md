# IntentLens 沙箱实现计划

**版本**: 1.0 (历史草案)
**日期**: 2026-10-30  
**状态**: 已由 [S6 执行边界设计](production/sandbox-execution-design.md) 取代；当前没有可用的平台沙箱
**优先级**: P0（阻断题目验证与评分）

> 本文中基于 seccomp 按路径限制 `open/openat`、对 `execve` 作绝对路径白名单等表述不是可直接落地的安全设计。执行授权以 S6 文档为准。不要据本历史计划开放任何执行入口。

## 📋 目录

- [执行摘要](#执行摘要)
- [需求分析](#需求分析)
- [平台策略](#平台策略)
- [实现路线图](#实现路线图)
- [验证策略](#验证策略)
- [风险分析](#风险分析)

---

## 执行摘要

IntentLens 的 `exercise-run` 功能需要操作系统级隔离来安全执行用户的 Go 代码。当前实现被禁用（`verifiedOSSandboxAvailable = false`），因为仅有文件系统隔离不足以防止网络访问和资源耗尽。

本计划设计跨平台沙箱实现，在 Windows、Linux 和 macOS 上都能安全执行受限的 Go 测试代码。

### 隔离目标

| 资源 | 隔离需求 | 当前状态 |
|------|---------|--------|
| **文件系统** | 只读项目目录 + 隔离工作目录 | 应用级 ✓ |
| **网络** | 完全禁止 | 源码检查 ✓ |
| **子进程** | 仅允许 go test，10s 超时 | 应用级超时 ~ |
| **资源** | 内存/CPU 限制 | ❌ 缺失 |
| **代码执行** | 白名单标准库 | 检查 ✓ |

### 成功标准

- [ ] Windows: AppContainer 实现通过自动化测试
- [ ] Linux: seccomp + cgroup 实现通过自动化测试
- [ ] macOS: Sandbox 实现通过自动化测试
- [ ] 端到端: 练习代码无法逃逸、访问网络或耗尽资源
- [ ] 无假正/假负: 所有合法测试通过，所有恶意代码被阻止

---

## 需求分析

### 1. 隔离需求

#### A. 文件系统隔离

**目标**: 练习代码只能访问隔离目录

**应用层现有**:
- 练习限于 1-20 个文件，60 KiB 大小
- 所有文件在独立目录中 materialize
- 符号链接和目录联接被检查和拒绝

**OS 层需要**:
- 子进程的根目录 (chroot 或等价)
- 或者白名单访问策略
- 防止 `..` 路径穿越

**Windows 实现**:
```
AppContainer ACL:
  工作目录 → 读/写权限
  GO二进制文件 → 执行权限  
  系统库 (msvcrt等) → 读权限
  其他所有文件系统 → 拒绝
```

**Linux 实现**:
```
seccomp 规则:
  open/openat → 仅允许工作目录路径
  execve → 仅允许 /usr/bin/go
  mount/unmount → 拒绝所有
```

**macOS 实现**:
```
Sandbox 配置:
  (allow file-read* (subpath "/path/to/work"))
  (allow file-write* (subpath "/path/to/work"))
  (deny file-read-data (subpath "/"))
```

#### B. 网络隔离

**目标**: 完全禁止网络访问

**当前机制**:
- `exercise.go` 的 `imports` 白名单不包括 `net`, `net/http` 等
- 源码解析验证拒绝这些导入

**OS 层需要**:
- 阻止所有 socket 操作
- 阻止 DNS 调用
- 防止通过环境变量泄露 API 密钥

**实现方式**:
```
Linux seccomp:
  socket → EPERM
  connect → EPERM
  getenv → 仅允许白名单变量

Windows:
  WinAPI socket 函数 → 返回失败

macOS:
  Sandbox (deny network* (all))
```

#### C. 子进程隔离

**目标**: 禁止 go test 之外的进程启动

**当前机制**:
- 命令固定为 `go test -json -count=1 -timeout=10s .`
- 10 秒应用级超时

**OS 层需要**:
- 禁止 fork/execve (除 go test 二进制)
- 禁止 dlopen (动态加载)
- 防止启动其他进程链

**实现方式**:
```
Linux seccomp:
  execve → 仅允许 /usr/bin/go
  clone/fork → 仅 go test 进程

Windows:
  Job Object 进程树限制
  可能的 AppContainer 限制

macOS:
  Sandbox (deny process* (all))
```

#### D. 资源隔限制

**目标**: 防止 DoS 攻击 (无限内存、CPU、文件描述符)

**需要的限制**:
- **内存**: 500 MiB (go test 合理上界)
- **CPU 时间**: 10 秒 (应用级超时)
- **文件描述符**: 64 (防止打开轰炸)
- **进程数**: 1 (仅 go test)
- **临时文件**: 100 MiB

**实现方式**:
```
Linux cgroup v2:
  memory.max = 524288000  (500 MiB)
  cpu.max = 10000000:1000000  (10秒/1秒)
  pids.max = 1

Windows Job Object:
  JOB_OBJECT_LIMIT_MEMORY = 500 MiB
  JOB_OBJECT_LIMIT_PROCESS_TIME = 10s

macOS launchd:
  <key>SoftResourceLimits</key>
  <dict>
    <key>MemoryLimit</key>
    <integer>524288000</integer>
  </dict>
```

### 2. 平台需求

#### Windows

**最低版本**: Windows 10 1809+ (AppContainer 成熟)

**技术栈**:
- AppContainer 或 Job Object
- SID (Security Identifier)
- ACL (Access Control List)
- WinAPI: `CreateAppContainerProfile`, `CreateRestrictedToken`

**优势**:
- 原生 Windows 安全特性
- 强隔离和资源控制

**劣势**:
- 需要 WinAPI (复杂的 cgo)
- 跨版本兼容性

#### Linux

**最低版本**: 5.8+ (seccomp v2, cgroup v2)

**技术栈**:
- seccomp-bpf (系统调用过滤)
- cgroup v2 (资源限制)
- namespaces (进程/网络隔离)

**优势**:
- 原生内核支持
- 广泛适用

**劣势**:
- 需要 CAP_SYS_ADMIN 权限 (某些环境不支持)
- 配置复杂

#### macOS

**最低版本**: macOS 10.14+ (Sandbox 成熟)

**技术栈**:
- Sandbox (DSL-based)
- `sandbox_init()` / launchd
- Gatekeeper (代码签名)

**优势**:
- 用户友好
- 与系统集成

**劣势**:
- 相对 Docker/VM 较弱
- DSL 不够灵活

---

## 平台策略

### Windows: AppContainer + Job Object

#### 1. 架构

```
IntentLens (Host Process)
    ↓ (创建隔离环境)
AppContainer Profile
    + Job Object (资源限制)
        + IPC (命名管道)
            ↓
        go test (沙箱进程)
```

#### 2. 实现步骤

**A. 创建 AppContainer Profile**

```go
// internal/sandbox/windows/container.go

func CreateContainer(ctx context.Context, workDir string) (*Container, error) {
    // 1. 创建 AppContainer SID
    //    Name: "intentlens-exercise-{random}"
    
    // 2. 设置 ACL
    //    工作目录: 读/写
    //    Go 二进制: 执行
    //    其他: 拒绝
    
    // 3. 创建 Job Object
    //    内存限制: 500 MiB
    //    进程限制: 1
    //    时间限制: 10s
    
    // 4. 返回 container token 用于 CreateProcess
}
```

**B. 在容器中运行进程**

```go
func (c *Container) Run(ctx context.Context, tool string, args []string) (*Result, error) {
    // 1. 准备环境
    //    - 清除所有环境变量
    //    - 仅设置 PATH, TMPDIR, GOPATH (必需的)
    //
    // 2. 创建进程
    //    - 使用 container token
    //    - 工作目录: 隔离目录
    //    - 标准输入输出: 管道
    //
    // 3. 监控执行
    //    - 捕获输出
    //    - 检测超时
    //    - 收集退出码
}
```

#### 3. 关键 WinAPI 调用

```c
// 创建 AppContainer SID
HRESULT CreateAppContainerProfile(
    PCWSTR pszAppContainerName,    // "intentlens-{uuid}"
    PCWSTR pszDisplayName,          // "IntentLens Exercise Sandbox"
    PCWSTR pszDescription,          // ...
    PSID_AND_ATTRIBUTES pCapabilities,
    DWORD dwCapabilityCount,
    PSID *ppSidAppContainerSid
);

// 创建受限 Token
BOOL CreateRestrictedToken(
    HANDLE ExistingTokenHandle,
    DWORD Flags,                    // DISABLE_MAX_PRIVILEGE
    DWORD DisableSidCount,
    PSID_AND_ATTRIBUTES DisableSids,
    DWORD RestrictedSidCount,
    PSID_AND_ATTRIBUTES RestrictedSids,
    DWORD PrivilegeCount,
    PLUID_AND_ATTRIBUTES Privileges,
    PHANDLE NewTokenHandle
);

// 创建进程
BOOL CreateProcessAsUserW(
    HANDLE hToken,                  // Restricted token
    LPCWSTR lpApplicationName,      // "go.exe"
    LPWSTR lpCommandLine,           // "test -json ..."
    PSECURITY_ATTRIBUTES ...
);

// Job Object 资源限制
BOOL SetInformationJobObject(
    HANDLE hJob,
    JOBOBJECTINFOCLASS JobObjectInfoClass,
    LPVOID lpJobObjectInfo,
    DWORD cbJobObjectInfoLength
);
```

#### 4. 限制

- **内核 API 复杂度**: 需要深度了解 Windows 安全模型
- **跨版本支持**: AppContainer 在某些 Windows 10 版本有差异
- **权限要求**: 通常需要管理员权限

---

### Linux: seccomp + cgroup

#### 1. 架构

```
IntentLens (Host Process)
    ↓ (配置 cgroup + seccomp)
cgroup v2
    + seccomp-bpf
        ↓
    go test (隔离进程)
```

#### 2. 实现步骤

**A. 设置 cgroup**

```go
// internal/sandbox/linux/cgroup.go

func SetupCgroup(ctx context.Context, pid int) error {
    // 1. 在 cgroup v2 中创建控制组
    //    路径: /sys/fs/cgroup/intentlens/{uuid}
    
    // 2. 设置资源限制
    cgroupPath := "/sys/fs/cgroup/intentlens/" + uuid
    
    // 内存限制 500 MiB
    os.WriteFile(cgroupPath+"/memory.max", []byte("524288000"), 0644)
    
    // CPU 限制 10s/1s = 10 CPUs 的使用
    os.WriteFile(cgroupPath+"/cpu.max", []byte("10000000 1000000"), 0644)
    
    // 进程限制 1
    os.WriteFile(cgroupPath+"/pids.max", []byte("1"), 0644)
    
    // 3. 将进程加入 cgroup
    os.WriteFile(cgroupPath+"/cgroup.procs", []byte(fmt.Sprintf("%d", pid)), 0644)
    
    return nil
}
```

**B. 配置 seccomp**

```go
// internal/sandbox/linux/seccomp.go

func CreateSeccompFilter() ([]byte, error) {
    // 使用 libseccomp (Go 绑定)
    // 或者加载预编译的 BPF 规则
    
    filter := []syscall.SockFilter{
        // 验证架构
        bpf.ArchLoadAndJump(),
        
        // 允许的系统调用白名单
        bpf.Allow("read"),
        bpf.Allow("write"),
        bpf.Allow("open"),
        bpf.Allow("close"),
        bpf.Allow("stat"),
        bpf.Allow("fstat"),
        bpf.Allow("lstat"),
        bpf.Allow("poll"),
        bpf.Allow("lseek"),
        bpf.Allow("mmap"),
        bpf.Allow("mprotect"),
        bpf.Allow("munmap"),
        bpf.Allow("brk"),
        // ... (测试所需的其他 syscalls)
        
        // 禁止的系统调用
        bpf.Deny("socket"),        // 网络
        bpf.Deny("connect"),       // 网络
        bpf.Deny("execve"),        // 仅允许初始进程
        bpf.Deny("fork"),          // 禁止子进程
        bpf.Deny("clone"),         // 禁止子进程
        bpf.Deny("dlopen"),        // 禁止动态加载
        bpf.Deny("dlsym"),
        bpf.Deny("ptrace"),        // 禁止调试
        bpf.Deny("mount"),         // 禁止挂载
        bpf.Deny("umount"),
        bpf.Deny("ioctl"),         // 限制 I/O
    }
    
    return filter.Bytes(), nil
}
```

#### 3. 系统调用列表

**允许** (Go test 需要):
```
read, write, open, openat, close, stat, fstat, lstat,
poll, lseek, mmap, mprotect, munmap, brk, rt_sigaction,
rt_sigprocmask, rt_sigpending, rt_sigtimedwait, rt_sigaction,
rt_sigprocmask, rt_sigpending, rt_sigtimedwait, sigaltstack,
getitimer, setitimer, pause, nanosleep, getresuid, times,
prlimit64, gettimeofday, clock_gettime, clock_getres,
getpid, getppid, getuid, getgid, geteuid, getegid,
exit_group, rt_sigprocmask, dup, dup2, dup3, fcntl,
getdents, getcwd, chdir, fchdir, rename, mkdir, rmdir,
creat, link, unlink, symlink, readlink, chmod, fchmod,
chown, fchown, lchown, umask, gettimeofday, getrlimit,
getrusage, sysinfo, getpriority, setpriority, setreuid,
setregid, getgroups, setgroups, setresuid, setresgid,
getfsuid, setfsuid, getfsgid, setfsgid, getsid, setsid,
getpgid, setpgid, sethostname, setdomainname, setrlimit,
arch_prctl, adjtimex, clock_adjtime, syncfs, setns
```

**禁止**:
```
socket, bind, listen, accept, connect, sendto, recvfrom,
sendmsg, recvmsg, shutdown, setsockopt, getsockopt,
execve, fork, vfork, clone, clone3, rt_sigaction,
ptrace, process_vm_readv, process_vm_writev,
mount, umount2, sysfs, sysctl, chroot, pivot_root,
quotactl, setfsuid, setfsgid, capset, rt_sigpending,
rt_sigtimedwait, rt_sigqueueinfo, kill, tkill, tgkill
```

#### 4. 限制

- **需要 CAP_SYS_ADMIN**: Docker/OCI 容器内可能不可用
- **seccomp 复杂性**: BPF 编写和调试困难
- **与其他工具冲突**: AppArmor、SELinux 可能冲突

---

### macOS: Sandbox

#### 1. 架构

```
IntentLens (Host Process)
    ↓ (加载 Sandbox 配置)
Sandbox (via launchd or sandbox_init)
    ↓
go test (沙箱进程)
```

#### 2. 实现步骤

**A. Sandbox 配置 (DSL)**

```scheme
;; internal/sandbox/macos/sandbox.sb

(version 1)

;; 允许进程启动
(allow process-exec (literal "/usr/bin/go"))

;; 允许工作目录访问
(allow file-read* (subpath "/var/tmp/intentlens-*"))
(allow file-write* (subpath "/var/tmp/intentlens-*"))

;; 允许必需的系统库
(allow file-read-data (literal "/usr/lib/libSystem.dylib"))
(allow file-read-data (literal "/usr/lib/libobjc.dylib"))
(allow file-read-metadata (subpath "/usr/lib"))
(allow file-read-metadata (subpath "/System/Library"))

;; 允许基本操作
(allow process-info-pidinfo)
(allow process-info-pidfdinfo)
(allow process-info-setcontrol)
(allow process-info-rusage)
(allow process-info-codesignature)

(allow file-read-metadata)
(allow file-read-xattr)
(allow file-write-xattr (only-writes-xattr "com.apple.macl"))

;; 禁止网络
(deny network* (all))

;; 禁止其他进程
(deny process-exec (literal "/bin/sh"))
(deny process-exec (literal "/bin/bash"))
(deny process-exec (literal "/usr/bin/python*"))

;; 禁止系统调用
(deny default (with no-log))
```

**B. 加载 Sandbox**

```go
// internal/sandbox/macos/sandbox.go

import "C"

func ApplySandbox(sbStr string) error {
    // 使用 libsandbox (C 绑定)
    // void sandbox_init(const char *profile, uint64_t flags, char **errorbuf)
    
    var errBuf *C.char
    sbC := C.CString(sbStr)
    defer C.free(unsafe.Pointer(sbC))
    
    result := C.sandbox_init(sbC, C.kSBXProfileNoCache, &errBuf)
    if result != 0 {
        err := C.GoString(errBuf)
        C.sandbox_free_error(errBuf)
        return fmt.Errorf("sandbox_init failed: %s", err)
    }
    return nil
}
```

#### 3. 限制

- **沙箱强度**: 相对 Linux seccomp 较弱 (可能有逃逸向量)
- **代码签名**: 某些操作需要应用签名
- **内核扩展**: 某些隔离需要 KEXT (在 Big Sur+ 中受限)

---

## 实现路线图

### 阶段 1: 设计和基础设施 (1-2 周)

**目标**: 建立沙箱抽象层和测试框架

```
cmd/intentlens/
  └─ sandbox_test.go (端到端测试)

internal/sandbox/
  ├─ common.go         (共享接口)
  ├─ windows/
  │  ├─ container.go   (AppContainer)
  │  └─ sandbox_test.go
  ├─ linux/
  │  ├─ seccomp.go     (seccomp-bpf)
  │  ├─ cgroup.go      (cgroup v2)
  │  └─ sandbox_test.go
  └─ macos/
     ├─ sandbox.sb     (沙箱 DSL)
     ├─ sandbox.go     (加载器)
     └─ sandbox_test.go
```

**任务**:
- [ ] 定义 Sandbox 接口 (Run, Cleanup, GetResult)
- [ ] 创建测试工具和 fixtures
- [ ] 设计错误处理和恢复机制

### 阶段 2: Windows 实现 (2-3 周)

**目标**: 完成 AppContainer 实现并通过所有测试

**任务**:
- [ ] 实现 AppContainer 创建和清理
- [ ] 实现受限环境变量处理
- [ ] 集成 Job Object 资源限制
- [ ] 编写和通过功能测试 (隔离验证、资源限制验证)
- [ ] 编写和通过安全测试 (逃逸尝试被阻止)

### 阶段 3: Linux 实现 (2-3 周)

**目标**: 完成 seccomp + cgroup 实现

**任务**:
- [ ] 编译和测试 seccomp BPF 规则
- [ ] 实现 cgroup v2 设置
- [ ] 处理权限问题 (CAP_SYS_ADMIN)
- [ ] 编写和通过功能测试
- [ ] 处理与 AppArmor/SELinux 的兼容性

### 阶段 4: macOS 实现 (1-2 周)

**目标**: 完成 Sandbox 实现

**任务**:
- [ ] 编写和测试沙箱 DSL 规则
- [ ] 实现代码签名和加载
- [ ] 编写和通过功能测试
- [ ] 处理 Big Sur+ 限制

### 阶段 5: 集成和验证 (2-3 周)

**目标**: 端到端集成和安全验证

**任务**:
- [ ] 启用 `verifiedOSSandboxAvailable = true`
- [ ] 集成到 exercise-run 命令
- [ ] 编写端到端测试 (包括恶意代码)
- [ ] 性能基准测试
- [ ] 文档和发布准备

---

## 验证策略

### 自动化测试

#### A. 功能隔离测试

**文件系统访问**:
```go
// 尝试读取工作目录之外的文件
func TestFilesystemEscape(t *testing.T) {
    code := `
        package main
        import (
            "io/ioutil"
            "os"
        )
        func TestEscape(t *testing.T) {
            data, err := ioutil.ReadFile("/etc/passwd")
            if err == nil {
                t.Fatalf("应该无法读取 /etc/passwd，但读到了: %s", data)
            }
            os.Open("/root")
            // 应该返回权限错误
        }
    `
    // 执行代码，验证失败
}
```

**网络访问**:
```go
func TestNetworkBlocked(t *testing.T) {
    code := `
        package main
        import (
            "net"
            "testing"
        )
        func TestNetwork(t *testing.T) {
            conn, err := net.Dial("tcp", "example.com:80")
            if err == nil {
                conn.Close()
                t.Fatal("应该无法拨号到外网")
            }
        }
    `
}
```

**子进程启动**:
```go
func TestSubprocessBlocked(t *testing.T) {
    code := `
        package main
        import (
            "os/exec"
            "testing"
        )
        func TestExec(t *testing.T) {
            cmd := exec.Command("bash", "-c", "whoami")
            err := cmd.Run()
            if err == nil {
                t.Fatal("应该无法启动子进程")
            }
        }
    `
}
```

#### B. 资源限制测试

**内存炸弹**:
```go
func TestMemoryLimit(t *testing.T) {
    code := `
        package main
        import "testing"
        func TestMemoryBomb(t *testing.T) {
            var data [][]byte
            for {
                data = append(data, make([]byte, 1024*1024)) // 1 MiB 块
            }
        }
    `
    // 预期: 进程被 OOM killer 杀死
}
```

**进程炸弹**:
```go
func TestProcessLimit(t *testing.T) {
    code := `
        package main
        import (
            "os"
            "testing"
        )
        func TestFork(t *testing.T) {
            for i := 0; i < 100; i++ {
                os.StartProcess(...)  // 尝试启动 100 个子进程
            }
        }
    `
    // 预期: 只有 1 个进程存活
}
```

#### C. 正常操作验证

**合法代码执行**:
```go
func TestLegitimateCodeWorks(t *testing.T) {
    code := `
        package main
        import (
            "fmt"
            "testing"
        )
        func TestHelloWorld(t *testing.T) {
            result := fmt.Sprintf("Hello, %s!", "World")
            if result != "Hello, World!" {
                t.Fatalf("Got %s", result)
            }
        }
    `
    result, err := sandbox.Run(code)
    if err != nil {
        t.Fatalf("合法代码不应该失败: %v", err)
    }
    if !result.Passed {
        t.Fatalf("测试应该通过")
    }
}
```

**文件 I/O (工作目录内)**:
```go
func TestWorkdirIOWorks(t *testing.T) {
    code := `
        package main
        import (
            "io/ioutil"
            "os"
            "testing"
        )
        func TestFileIO(t *testing.T) {
            err := ioutil.WriteFile("test.txt", []byte("hello"), 0644)
            if err != nil {
                t.Fatalf("应该能写入工作目录: %v", err)
            }
            data, err := ioutil.ReadFile("test.txt")
            if err != nil || string(data) != "hello" {
                t.Fatalf("应该能读回写入的文件")
            }
        }
    `
}
```

### 手动验证清单

在发布前，需要在真实系统上手动验证：

**Windows**:
- [ ] AppContainer 正确创建和清理
- [ ] 可以访问 Go 二进制文件和工作目录
- [ ] 无法读取 `C:\Windows\System32`
- [ ] 无法创建网络连接
- [ ] 内存用尽时正确报告
- [ ] 超时时正确终止

**Linux**:
- [ ] seccomp 规则正确加载
- [ ] cgroup 资源限制有效
- [ ] 进程无法逃逸
- [ ] 内存和 CPU 限制工作

**macOS**:
- [ ] 沙箱正确加载
- [ ] 网络被阻止
- [ ] 文件访问被限制
- [ ] 代码签名工作

---

## 风险分析

### 1. 技术风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| **AppContainer 复杂性** | Windows 实现延期 | 优先完成 Linux 作为 MVP |
| **seccomp 不完整** | 可能的逃逸 | 广泛的安全测试 |
| **macOS 沙箱弱** | 更多逃逸向量 | 参考已知限制 |
| **权限问题** | 某些环境无法工作 | 优雅降级，提示用户 |

### 2. 性能风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| **沙箱开销** | 测试变慢 | 基准测试，优化 |
| **创建/清理延迟** | UX 变差 | 缓存容器，异步清理 |
| **内存使用** | 资源消耗大 | 设置合理限制 |

### 3. 安全风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| **实现有 bug** | 逃逸漏洞 | 安全审计，外部审查 |
| **TOCTOU 竞态** | 文件被替换 | 使用完全隔离目录 |
| **符号链接攻击** | 目录穿越 | 追踪链接，拒绝 |

---

## 定义完成条件

### 最小可发布版本 (MVP)

- [ ] **Linux 实现完成** (最常见的部署)
- [ ] **功能隔离测试** 全部通过
- [ ] **安全测试** 全部通过
- [ ] **文档** 完整 (用户指南、限制说明、故障排查)
- [ ] **使用 verifiedOSSandboxAvailable = true** (仅 Linux)

### 完整版本

- [ ] 上述 MVP + Windows 实现
- [ ] 上述 MVP + macOS 实现
- [ ] 性能基准 < 2 秒额外延迟
- [ ] 跨平台端到端测试通过
- [ ] 外部安全审计 (可选)

---

## 参考资源

- [Windows AppContainer Security](https://learn.microsoft.com/en-us/windows/win32/secauthz/app-containers)
- [Linux seccomp-bpf](https://man7.org/linux/man-pages/man2/seccomp.2.html)
- [Linux cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
- [macOS Sandbox Design](https://developer.apple.com/library/archive/documentation/Security/Conceptual/AppSandboxDesignGuide/)
- [OWASP Sandbox Evasion](https://owasp.org/www-community/attacks/Sandbox_Evasion)

---

**下一步**: 启动阶段 1 设计和基础设施工作。

**预计完成**: 2026-12-15 (Linux MVP) / 2026-01-15 (完整版本)
