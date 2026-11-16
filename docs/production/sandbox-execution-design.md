# 练习验证与评分执行边界设计

日期：2026-11-16。状态：设计冻结，待实现与隔离验收。依赖用户已确认的产品规则：练习使用独立工作区；运行前展示命令、工作目录和范围并要求确认；同一题版本、命令和范围确认后可重复运行；题目版本绑定代码基线、要求与验收测试；行为要求、测试结果、理由反馈分开保存；模型不能覆盖测试失败。

## 当前证据

- `internal/exercise/run.go` 的 `Run` 永久失败关闭，`verifiedOSSandboxAvailable` 为 false。
- `internal/sandbox/windows/sandbox.go`、`linux/sandbox.go`、`macos/sandbox.go` 均有未完成的隔离步骤。它们不能提供安全边界；不得接入执行入口。
- 当前发行 target 为 Windows x64；此主机为 Windows 10 Home。Linux 和 macOS 暂不开放可执行练习。

## 冻结实现

Windows 执行器使用 AppContainer 低权限进程和 Job Object。AppContainer 不授予网络等能力；文件 ACL 仅授予一次性练习目录读写和 Go 工具链只读权限。进程树置于有内存、累计 CPU 时间、活动进程数及 kill-on-close 约束的 Job Object。超时、取消或任一隔离创建步骤失败时，关闭 Job、清理临时 profile/ACL 并返回未执行或执行中断；不得回退到普通 `exec.Command`。

`go test -json -count=1 -timeout=10s .` 在隔离目录内执行。只允许已经验证为可独立构建的 Go 单包和标准库；禁用 cgo，设置 `GOTOOLCHAIN=local`、`GOPROXY=off`、`GOSUMDB=off`，清除所有不必要环境变量。Go 工具链只读挂载/授权，缓存与临时文件留在本次隔离目录。固定运行时输出上限，解析不完整、退出异常或证据无法关联时均不能通过。

执行前 UI 显示题目版本、命令、独立目录、可读写范围、进程/内存/CPU/时限和网络状态，并要求本次明确确认。执行摘要与原始测试状态绑定题目版本、答案摘要和验证时间；用户答案、个人分数与阅读状态仍只在本地。行为要求匹配结果、测试通过情况、理由解释反馈各自展示。没有执行结果只能给暂定反馈；模型反馈不能改变测试结果。

Linux/macOS 和 Windows 不支持 AppContainer/Job Object 的环境一律返回 sandbox unavailable。无自动降级、无普通进程执行、无升级权限或开启 Hyper-V 的要求。

## 验收门槛

1. 功能：原测试及练习测试按预期运行；基线中每条新增验收测试以指定缺失行为失败，参考解通过；完整题目验证状态和个人作答评分分开持久化并绑定版本。
2. 隔离：恶意 fixture 无法读取或修改练习目录以外的哨兵文件、连接 loopback/外网、读取环境密钥或在限制外启动进程；内存/CPU/墙钟超限时整个进程树终止；拒绝场景保持源码与持久化题目状态完整。
3. 清理：成功、失败、取消、超时和父进程异常退出后，不残留活动 AppContainer 授权、运行进程或可写宿主路径。
4. 平台：Windows 10 Home x64 实机运行隔离攻击与功能检查；任何未验证 Windows 版本及 Linux/macOS 均明确失败关闭。
5. 代码审查：独立复核 Win32 API 参数、ACL 恢复、profile 生命周期、Job 关闭路径和所有普通进程旁路。只有安全审查和攻击验收全部通过后，才可把执行能力门闩设为可用。

## 风险与限制

这是用于受控练习的本机 OS 边界，不承诺抵御 Windows 内核、hypervisor 或受信任 Go 工具链漏洞。AppContainer 权限配置错误可能暴露宿主文件或无法运行 Go 工具链；这两类情况都必须使执行失败关闭。真实用户试用、真实模型质量和产品生产就绪仍需分别验收。

官方依据：Windows AppContainer 进程通过 `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES` 创建，且只能访问显式授予资源：[UpdateProcThreadAttribute](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute)、[AppContainer isolation](https://learn.microsoft.com/en-us/windows/win32/secauthz/appcontainer-isolation)。
