# 性能指标（SLO）

本文件定义 CLI 单次命令的性能预算，由 `internal/app/command_perf_test.go` 中的 `TestPerformanceBudget` 强制执行（`INTENTLENS_PERF_GATE=1`），CI 见 `.github/workflows/performance.yml`。

## 预算

| 场景 | 规模 | Linux 参考测量 | CI 预算 |
|---|---|---|---|
| `index` | 1000 个函数 | 29 ms | 120 ms |
| `inspect` | 1000 个函数 | 30 ms | 120 ms |
| `rounds` 单次命令（Open + 首页 50 条） | 1000 轮，每轮含解释和纠错 | 255–290 ms | 750 ms |

参考测量：Linux x86_64，2 vCPU，Go 1.25.0，`-benchtime=3x`。CI 预算约为参考值的 2.5–4 倍，用于吸收共享 runner 的噪声；超出即视为回归。

## 范围与限制

- 预算只覆盖 Linux 合成数据，**未在 Windows 目标机器上测量**，不构成对 Windows 用户的 SLA。
- `rounds` 的成本仍随历史规模线性增长：每次命令的 Open 会做一次完整的恢复校验和隐私清理，首页分页也需要读取全部轮次以排序。同一进程内的重复隐私清理在策略和记录（名称、大小、修改时间）都未变时会跳过。
- 跨进程的持久索引或清理缓存未引入：隐私清理依赖项目目录的链接状态，持久缓存无法保证实时性。
