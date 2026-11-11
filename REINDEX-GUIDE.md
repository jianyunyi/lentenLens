# 重新索引说明

## 问题分析

VS Code可能在使用缓存的索引结果。需要：

1. 确保使用最新的CLI
2. 清除IntentLens的工作缓存
3. 重新触发索引

## 步骤

### 1. 确认CLI已更新
```bash
ls -lh extension/bin/intentlens.exe
# 应该显示: 12.8MB, 2026/11/11 13:35
```

### 2. 在VS Code中清除缓存并重新索引

方法A - 使用命令面板:
1. Ctrl+Shift+P
2. 输入: IntentLens: 刷新轮次与 CodeLens
3. 或者: IntentLens: 查看索引范围与遗漏原因

方法B - 手动清除 .intent 目录:
1. 关闭VS Code
2. 删除项目中的 .intent 目录
3. 重新打开VS Code
4. 触发IntentLens分析

### 3. 验证修复

打开Go文件，触发IntentLens hover，检查"解析限制"部分：

期望结果:
```
解析限制:
- type check: 7 missing imports (e.g., fmt, time, gorm.io/gorm)
```

而不是每个包一行。

## 调试命令

如果还是不行，可以手动测试CLI:
```bash
cd your-go-project
../intentlens/bin/intentlens.exe index
```

查看输出中的issues部分是否已经合并。
