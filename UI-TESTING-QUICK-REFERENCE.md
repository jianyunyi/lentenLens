# IntentLens UI 优化测试指南

## 测试环境要求

- VS Code 最新版本
- IntentLens v0.6.5 VSIX
- Go 项目（包含限制问题）

## 快速测试清单

### 1. 色彩编码 ✓
- [ ] Missing Imports：橙色背景（#f78c6b）
- [ ] Ambiguous Calls：蓝色背景（#6cb6ff）
- [ ] Type Check：黄色背景（#f5d547）

### 2. 信息精简 ✓
- [ ] Summary 显示前 5 个样本 + "..."
- [ ] "📋 复制全部" 按钮可用
- [ ] 完整列表可复制

### 3. 交互动画 ✓
- [ ] Filter 按钮 hover 时上升 + 阴影
- [ ] Restriction 项 hover 时背景变浅
- [ ] Copy 按钮 hover 有缩放和阴影

### 4. 空状态 ✓
- [ ] 无问题时显示"✅ 未发现问题"
- [ ] 文本居中显示
- [ ] 布局整洁

### 5. 响应式 ✓
- [ ] 调整窗口宽度时自动换行
- [ ] 文本不溢出
- [ ] 始终可读

## 性能验证

打开 VS Code DevTools (F12)：
- [ ] 60fps 无卡顿
- [ ] 切换 filter 响应快
- [ ] 大列表（100+ 项）无延迟

## 反馈

发现问题？提交 Issue：
https://github.com/jianyunyi/lentenLens/issues
