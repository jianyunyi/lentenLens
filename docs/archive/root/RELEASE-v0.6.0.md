# ✅ IntentLens v0.6.0 Release Complete

## 📦 Release Information

**Version**: 0.6.0  
**Release Date**: 2026-11-11  
**GitHub Release**: https://github.com/jianyunyi/lentenLens/releases/tag/v0.6.0  
**Git Tag**: v0.6.0

---

## 🐛 Fixed Issues

### 1. Duplicate Notification Messages ✅
**Commits**: 38e5f24  
**Files**: extension/src/extension.js  

**Before**:
```
IntentLens: 开始记录本轮修改
IntentLens: 开始记录本轮修改  ← 重复
IntentLens: 结束记录本轮修改
IntentLens: 结束记录本轮修改  ← 重复
```

**After**:
```
IntentLens: 开始记录本轮修改
IntentLens: 结束记录本轮修改
```

**Implementation**: Message deduplication cache with 1-second TTL

---

### 2. Verbose Parse Restrictions ✅
**Commits**: 9ca4317, 41e4047  
**Files**: internal/codeindex/index.go  

**Before**:
```
解析限制:
- type check: distributed-task/config/database.go:4:2: could not import fmt
- type check: distributed-task/config/database.go:5:2: could not import time
- type check: distributed-task/config/database.go:7:2: could not import gorm.io/driver/mysql
- type check: distributed-task/config/database.go:8:2: could not import gorm.io/gorm
- type check: distributed-task/config/database.go:9:2: could not import gorm.io/gorm/logger
... (10+ lines)
```

**After**:
```
解析限制:
- type check: 7 missing imports (e.g., fmt, gorm.io/driver/mysql, gorm.io/gorm)
```

**Implementation**: Collect import errors separately and summarize

---

### 3. UI Test Failures ✅
**Commits**: b3926ac  
**Files**: extension/test/*.test.js (5 files)  

**Before**: 5 tests failing with "ReferenceError: describe is not defined"  
**After**: All 124 tests passing (119 core + 5 UI)  

**Implementation**: Convert from Mocha to Node.js native test runner

---

## 📊 Git History

```bash
$ git log --oneline -6
29e4303 (HEAD -> main, tag: v0.6.0, origin/main) release: version 0.6.0
41e4047 fix: ensure CLI binary is updated in extension/bin
3e2bf1e (origin/release/bug-fixes-2026-11-11) release: bug fixes v0.6.1
8544d9f docs: add testing guide for bug fixes
9ca4317 fix: summarize type check import errors to reduce UI clutter
38e5f24 fix: prevent duplicate IntentLens notification messages
```

---

## 🚀 Deployment Status

- ✅ All commits merged to main
- ✅ Version bumped to 0.6.0 in package.json
- ✅ CLI rebuilt with fixes (12.8 MB)
- ✅ Git tag v0.6.0 created
- ✅ GitHub Release published
- ✅ CHANGELOG.md updated

---

## ⚠️ Important: Cache Clearing Required

**The fixes are in place, but you may still see old errors if using cached data.**

### To see the fixes take effect:

1. **Reload VS Code**
   ```
   Ctrl+Shift+P → Developer: Reload Window
   ```

2. **Clear IntentLens Cache** (if issues persist)
   - Close VS Code
   - Delete `.intent` directory in your project
   - Reopen VS Code

3. **Trigger Reindex**
   ```
   Ctrl+Shift+P → IntentLens: 刷新轮次与 CodeLens
   ```
   Or open a Go file and trigger IntentLens analysis

---

## 🧪 Verification Steps

### Test 1: Duplicate Messages
1. Execute: IntentLens: 开始记录本轮修改
2. Wait 2 seconds
3. Execute again
4. ✅ Should only appear once per execution

### Test 2: Parse Restrictions
1. Open a Go file with import errors
2. Trigger IntentLens hover/analysis
3. Look at "解析限制" section
4. ✅ Import errors should be summarized in one line

### Test 3: UI Tests
```bash
cd extension
npm test
```
✅ Should show: 124 passing tests

---

## 📝 Documentation

Created/Updated files:
- ✅ CHANGELOG.md - Version 0.6.0 changelog
- ✅ TEST-GUIDE.md - Testing instructions
- ✅ BUGFIX-SUMMARY.md - Complete bug fix details
- ✅ REINDEX-GUIDE.md - Cache clearing guide
- ✅ RELEASE-NOTES.md - Release notes

---

## 🔗 Links

- GitHub Release: https://github.com/jianyunyi/lentenLens/releases/tag/v0.6.0
- Repository: https://github.com/jianyunyi/lentenLens
- Branch: main
- Tag: v0.6.0

---

## 💡 Next Steps

1. **Test the release** in VS Code
2. **Clear .intent cache** in your projects
3. **Verify** fixes are working
4. **Package VSIX** (optional) if needed for distribution
5. **Update marketplace** (if applicable)

---

## 📞 Troubleshooting

### If messages still duplicate:
- Ensure VS Code is fully reloaded
- Check extension is using v0.6.0
- Try completely closing and reopening VS Code

### If errors still verbose:
- Delete `.intent` directory
- Trigger manual reindex
- Verify CLI is updated: check extension/bin/intentlens.exe timestamp

### If tests fail:
- Run: npm install (in extension directory)
- Clear node_modules and reinstall
- Check Node.js version (should be 18+)

---

✨ **Release v0.6.0 is complete and ready for testing!**
