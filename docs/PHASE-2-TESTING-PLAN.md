# 🚀 Phase 2 - 单元测试

**目标**: 为 UI 组件编写完整的单元测试

**状态**: Phase 1 ✅ 完成 → Phase 2 开始

---

## 📋 Phase 2 任务

### Task 2.1: 创建测试框架 (30 分钟)

创建 \extension/test/ui.test.js\:

\\\javascript
const assert = require('assert');
const vscode = require('vscode');

describe('IntentLens UI Enhancement', () => {
  
  describe('TaskTreeProvider', () => {
    it('应该过滤任务');
    it('应该排序任务');
    it('应该刷新树视图');
  });
  
  describe('TaskBoundaryDecorator', () => {
    it('应该为编辑器设置边框');
    it('应该根据优先级应用颜色');
    it('应该处理悬停事件');
  });
  
  describe('EnhancedCallPathProvider', () => {
    it('应该排序调用节点');
    it('应该搜索函数');
    it('应该导出为文本');
  });
  
  describe('TaskManagementPanel', () => {
    it('应该创建任务');
    it('应该检测循环依赖');
    it('应该导出/导入任务');
  });
});
\\\

### Task 2.2: TaskTreeProvider 测试 (45 分钟)

编写过滤、排序、刷新的测试

### Task 2.3: TaskBoundaryDecorator 测试 (45 分钟)

编写装饰、颜色、交互的测试

### Task 2.4: EnhancedCallPathProvider 测试 (45 分钟)

编写排序、搜索、导出的测试

### Task 2.5: TaskManagementPanel 测试 (60 分钟)

编写表单验证、依赖检测、导入导出的测试

---

## 📊 预计耗时

| 任务 | 时间 |
|------|------|
| 2.1 框架 | 30 分钟 |
| 2.2 TaskTreeProvider | 45 分钟 |
| 2.3 TaskBoundaryDecorator | 45 分钟 |
| 2.4 EnhancedCallPathProvider | 45 分钟 |
| 2.5 TaskManagementPanel | 60 分钟 |
| **总计** | **225 分钟 (3.75 小时)** |

---

## ✅ Phase 2 完成条件

- [ ] 所有测试文件创建
- [ ] 覆盖所有 4 个核心模块
- [ ] \
pm test\ 通过所有测试
- [ ] 代码覆盖率 ≥ 70%
- [ ] 提交 'feat(test): Phase 2 - UI component tests'

---

## 下一步

执行 Task 2.1: 创建测试框架

