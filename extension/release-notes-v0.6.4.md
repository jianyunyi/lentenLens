## v0.6.4 - Ambiguous Calls Summarization (2026-11-12)

### 🎯 Features
- **Intelligent Ambiguous Calls Summarization**: Deduplicates repetitive "调用目标未唯一确定" errors
  - Was: 15 separate lines for each ambiguous call
  - Now: Single line "8 ambiguous calls (e.g., errors.New, len, json.Valid)"
  - Reduces clutter by 90%+ while maintaining visibility via hover/click

### 🔧 Technical Implementation
- Added parseAmbiguousCallMessage() to extract call targets
- Added extractAmbiguousTargets() to deduplicate and sort
- Added summarizeAmbiguousCalls() to generate summary with samples
- Integrated into aggregator pipeline Step 3 (parallel to Missing Imports)
- Comprehensive test coverage in ambiguous_test.go

### ✅ Validation
- All 106 tests passing
- Follows same pattern as existing Missing Imports summarization
- Tested with real-world Go projects

### 📦 Installation
Download intentlens-0.6.4.vsix and install via VS Code Extensions > Install from VSIX
