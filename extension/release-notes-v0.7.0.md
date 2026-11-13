## v0.7.0 - API Configuration & Token Usage Tracking (2026-11-12)

### 🎉 Major Features

#### 1. API Configuration Panel
Configure and manage AI API credentials directly from VS Code.

**Features:**
- **Multiple Provider Support**: OpenAI, Anthropic Claude, Custom/Other
- **Secure API Key Storage**: Encrypted in VS Code Secrets
- **Model Selection**: Select or enter model IDs
- **Model List Fetching**: Automatically fetch available models from provider
- **Connection Testing**: Validate API credentials with one click
- **Advanced Options**: Custom Base URL for self-hosted endpoints
- **Field Validation**: Real-time validation with error messages
- **Status Indicators**: Visual feedback for configuration state

**Access:** Command Palette → "IntentLens: 启用或调整模型分析"

#### 2. Token Usage Tracking Panel
Monitor API token consumption, costs, and budgets in real-time.

**Features:**
- **Session Statistics**: Input/Output/Total tokens in current session
- **Cumulative Tracking**: Statistics by period (Today/Week/Month/All-Time)
- **Cost Calculation**: Automatic cost estimation (configurable pricing)
- **Monthly Budget**: Set and monitor monthly spending limit
- **Budget Alerts**: Color-coded indicators (Green/Yellow/Red)
- **Auto-Refresh**: Updates automatically every 5 minutes
- **Data Export**: Export statistics as CSV or JSON
- **Statistics Reset**: Clear data with confirmation

**Access:** Command Palette → "IntentLens: 查看 Token 消耗"

### 🔒 Security Enhancements
- ✓ API keys stored in VS Code Secrets (encrypted at rest)
- ✓ Workspace-scoped credential storage
- ✓ Input validation and sanitization
- ✓ HTTPS enforcement for custom endpoints
- ✓ No credentials in logs or messages

### 🎨 UI/UX Improvements
- ✓ Impeccable design system integration
- ✓ VS Code theme awareness
- ✓ Responsive layouts
- ✓ Smooth animations and transitions
- ✓ Clear error messages and status indicators
- ✓ Keyboard navigation support
- ✓ WCAG AA accessibility compliance

### 📊 Implementation Details

**Code Changes:**
- New file: extension/src/aiConfigPanel.js (~700 lines)
- New file: extension/src/tokenUsagePanel.js (~550 lines)
- Modified: extension/src/extension.js (command registration)

**Testing:**
- 15+ unit tests for API Configuration Panel
- Comprehensive test coverage
- All tests passing ✅

**Documentation:**
- Complete implementation guides
- Configuration examples
- API reference

### 🚀 Usage

#### Configure API
1. Open Command Palette (Ctrl+Shift+P / Cmd+Shift+P)
2. Search for "IntentLens: 启用或调整模型分析"
3. Fill in API provider details
4. Click "Test Connection" to validate
5. Save configuration

#### View Token Usage
1. Open Command Palette
2. Search for "IntentLens: 查看 Token 消耗"
3. Monitor real-time token consumption
4. Set monthly budget
5. Export data as needed

### 📦 Installation
Download intentlens-0.7.0.vsix and install via VS Code Extensions panel

### ⚙️ Configuration
Settings available in VS Code Preferences:
- intentlens.apiProvider - Select API provider
- intentlens.apiBaseUrl - Custom endpoint (optional)
- intentlens.monthlyBudget - Monthly spending limit

### 🐛 Known Limitations
- Token count accuracy depends on API provider
- Cost calculation uses approximate pricing
- Export is manual (copy-paste friendly)

### 📝 Release Notes
- Resolves: Missing API configuration entry point
- Resolves: No token consumption visibility
- Feature complete for Phase 5 foundation

### 🙏 Contributors
IntentLens Team

---

**Version:** 0.7.0  
**Release Date:** 2026-11-12  
**Compatibility:** VS Code 1.90.0+
