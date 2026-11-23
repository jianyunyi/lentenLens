# IntentLens 解析限制优化 - 测试指南

## 已完成的修复

### 1. 重复通知消息修复 ✅
**文件**: extension/src/extension.js
**提交**: 71ff1f8
**功能**: 添加消息去重机制，防止1秒内重复显示相同消息

### 2. 解析限制错误汇总 ✅
**文件**: internal/codeindex/index.go
**提交**: 143c373
**功能**: 将多个import错误合并为一行汇总显示

### 3. CLI重新构建 ✅
**文件**: extension/bin/intentlens.exe
**大小**: 12.8 MB
**时间**: 2026/11/11 13:02

## 测试步骤

### 在 VS Code 中测试

1. **重新加载扩展**
   - 按 `Ctrl+Shift+P` (Mac: `Cmd+Shift+P`)
   - 输入并选择: `Developer: Reload Window` 或 `开发人员: 重新加载窗口`
   - 等待VS Code重新加载

2. **测试重复消息修复**
   测试以下命令，每条消息应该只出现一次：
   - `IntentLens: 开始记录本轮修改`
   - `IntentLens: 结束记录本轮修改`
   - `IntentLens: 恢复未结束的轮次`
   - `IntentLens: 取消当前轮次（保留源码）`

3. **测试解析限制优化**
   - 打开一个Go文件
   - 触发IntentLens分析（在函数上查看CodeLens）
   - 点击查看"分析限制"部分
   - **期望结果**: 
     - Import错误应该合并为一行
     - 例如: `type check: 7 missing imports (e.g., fmt, time, gorm.io/gorm)`
     - 而不是每个包一行

### 预期效果对比

#### 修复前（冗余）
```
分析限制：
- type check: could not import fmt (Config.Importer not installed)
- type check: could not import time (Config.Importer not installed)  
- type check: could not import gorm.io/driver/mysql (Config.Importer not installed)
- type check: could not import gorm.io/gorm (Config.Importer not installed)
- type check: could not import gorm.io/gorm/logger (Config.Importer not installed)
- type check: could not import gorm.io/datatypes (Config.Importer not installed)
... (每个import一行，非常冗余)
```

#### 修复后（简洁）
```
分析限制：
- type check: 7 missing imports (e.g., fmt, gorm.io/driver/mysql, gorm.io/gorm)
```

## 如果遇到问题

### "spawn intentlens ENOENT" 错误
**原因**: VS Code未找到intentlens.exe
**解决**: 
1. 确认文件存在: `extension/bin/intentlens.exe`
2. 重新加载VS Code窗口
3. 检查VS Code设置中的 `intentlens.binaryPath` 配置

### 消息仍然重复
**原因**: 扩展未重新加载
**解决**: 
1. 再次执行 `Developer: Reload Window`
2. 或者完全关闭并重新打开VS Code

### 解析限制仍然冗余
**原因**: 使用了旧的CLI二进制
**解决**:
1. 确认 `extension/bin/intentlens.exe` 时间戳是最新的
2. 重新加载VS Code
3. 如果还不行，完全关闭VS Code后重新打开

## 技术细节

### 消息去重机制
- 使用 Map 缓存最近1秒内的消息
- 相同类型+内容的消息会被跳过
- 自动清理超过10秒的旧缓存

### 错误汇总逻辑
- 分离import错误和其他类型错误
- Import错误提取包名并去重
- ≤3个包: 显示所有包名
- >3个包: 显示总数和前3个示例

## 下一步

测试确认无误后：
1. 推送到GitHub: `git push origin main`
2. 如果需要，可以创建新的VSIX扩展包
3. 关闭相关issue

## Git历史

```bash
git log --oneline -3
# 143c373 fix: summarize type check import errors to reduce UI clutter
# 71ff1f8 fix: prevent duplicate IntentLens notification messages  
# b3926ac fix: convert 5 Phase 3 UI tests from Mocha to Node.js test runner
```
