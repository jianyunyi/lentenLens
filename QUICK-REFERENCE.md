# IntentLens v0.6.0 - 快速参考

## 🚀 性能测试命令

### 运行基准测试
```bash
npm run bench:all           # 运行所有基准测试
npm run bench:task-panel    # Task Panel基准测试
npm run bench:boundary      # Boundary装饰基准测试
npm run bench:callpath      # Call Path基准测试
```

### CPU与内存分析
```bash
npm run profile:cpu         # CPU profiling
npm run profile:memory      # 内存profiling（堆快照）
npm run profile:baseline    # 生成统一基线报告
npm run profile:leak        # 内存泄漏检测（1小时模拟）
```

### 负载与报告
```bash
npm run test:load          # 负载稳定性测试
npm run report             # 生成HTML性能报告
npm test                   # 运行单元测试
```

---

## 📊 当前性能指标 (10k任务规模)

| 指标 | 目标 | 实际 | 状态 |
|------|------|------|------|
| Filter/Sort | ≤100ms | **0.60ms** | ✅ 167x |
| Task Panel Load | ≤500ms | **4.64ms** | ✅ 108x |
| Boundary Render | ≤300ms | **1.21ms** | ✅ 248x |
| Call Path Expand | ≤200ms | **12.07ms** | ✅ 17x |
| Baseline Heap | ≤50MB | **3.88MB** | ✅ 13x |
| Peak Memory | ≤150MB | **6.53MB** | ✅ 23x |
| Memory Growth | <2.0x | **1.55x** | ✅ |
| Memory Leak | ≤10MB/h | **0.84MB/h** | ✅ 12x |

**所有目标达成** ✅ (超预期 50-100x)

---

## 📁 关键文件位置

### 源代码
- `extension/src/taskpanel.js` - Task Panel组件
- `extension/src/boundaries.js` - Boundary装饰（已优化）
- `extension/src/callpath-enhanced.js` - Call Path组件
- `extension/src/taskmanagement.js` - Task Management组件

### 测试
- `extension/test/*.test.js` - 单元测试（91个通过）
- `extension/test/profiling/` - Profiling工具和基准测试

### 文档
- `docs/profiling-guide.md` - Profiling使用指南
- `docs/optimization-log.md` - 优化变更记录
- `docs/PHASE-4-COMPLETE.md` - Phase 4完成总结
- `docs/performance-reports/` - HTML性能报告

### 配置
- `extension/package.json` - npm脚本和扩展配置
- `tasks.md` - Phase 4任务定义

---

## 🔧 常用工作流

### 1. 验证性能
```bash
cd extension
npm run bench:all          # 运行基准测试
npm run report             # 生成报告
# 打开: docs/performance-reports/phase-4-latency-report.html
```

### 2. 内存分析
```bash
cd extension
npm run profile:memory     # 堆快照
npm run profile:leak       # 泄漏检测
npm run report             # 生成报告
# 查看: docs/performance-reports/phase-4-memory-report.html
```

### 3. CPU分析
```bash
cd extension
npm run profile:cpu        # CPU profiling
# 结果: docs/performance-reports/cpu-profile-analysis.json
```

### 4. 完整测试
```bash
cd extension
npm test                   # 单元测试
npm run test:load          # 负载测试
npm run bench:all          # 基准测试
npm run report             # 生成报告
```

---

## 🎯 Phase 5 下一步

### Option A: 创建Phase 5 Spec
```
用户说: "创建 Phase 5 spec"
→ 生成design-first或requirements-first规格
```

### Option B: 修复UI测试
```
用户说: "修复Phase 3的测试"
→ 修复5个UI测试文件的语法问题
```

### Option C: 准备Beta发布
```
用户说: "准备beta发布"
→ 创建CHANGELOG、发布说明、用户指南
```

---

## 📞 问题排查

### 测试失败
```bash
# 检查Node版本（需要v16+）
node --version

# 重新运行测试
npm test

# 查看详细错误
npm test 2>&1 | more
```

### 内存问题
```bash
# 增加堆大小
node --max-old-space-size=4096 extension/test/profiling/baseline-heap-snapshot.js

# 确保使用--expose-gc
npm run profile:memory  # 已包含--expose-gc
```

### 报告生成失败
```bash
# 确保先运行基准测试
npm run bench:all

# 然后生成报告
npm run report

# 检查报告目录
ls docs/performance-reports/
```

---

## 📚 更多资源

- **完整文档**: `docs/profiling-guide.md`
- **优化详情**: `docs/optimization-log.md`
- **项目状态**: `PROJECT-STATUS.md`
- **Phase 4总结**: `docs/PHASE-4-COMPLETE.md`
- **Phase 5规划**: `docs/PHASE-5-OVERVIEW.md`

---

**当前状态**: ✅ Phase 4 完成，准备进入 Phase 5  
**版本**: v0.6.0  
**最后更新**: 2026-11-10
