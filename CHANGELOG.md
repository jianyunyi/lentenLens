# Changelog

## [0.6.0] - 2026-11-11

### Fixed

#### UI Improvements
- **Duplicate Notification Messages**: Added deduplication cache to prevent repeated messages within 1 second
  - Fixed: "开始记录本轮修改", "结束记录本轮修改" appearing twice
  - Implementation: Message cache with 1-second TTL
  - File: `extension/src/extension.js`

- **Verbose Parse Restrictions**: Summarized type-check import errors into single line
  - Before: Each "could not import" error on separate line (10+ lines)
  - After: "type check: 7 missing imports (e.g., fmt, time, gorm.io/gorm)"
  - File: `internal/codeindex/index.go`

#### Testing
- **Phase 3 UI Tests**: Converted 5 test files from Mocha to Node.js test runner
  - Files: boundaries.test.js, callpath-enhanced.test.js, taskmanagement.test.js, taskpanel.test.js, ui-integration.test.js
  - Result: All 124 tests passing (119 core + 5 UI)

### Technical Details

- Rebuilt CLI binary with error summarization (12.8 MB)
- Added message deduplication with automatic cache cleanup
- Import errors now collected and summarized separately
- Non-import type errors still reported individually

### Breaking Changes
None

### Migration Guide
1. Reload VS Code window: `Ctrl+Shift+P` → "Developer: Reload Window"
2. Clear IntentLens cache if needed: Delete `.intent` directory in project
3. Trigger reindex: Open Go file and use IntentLens analysis

### Known Issues
- Cached index results may still show old error format until reindex
- Solution: Use "IntentLens: 刷新轮次与 CodeLens" command

---

## [0.5.2] - Previous Release
(Previous release notes)

