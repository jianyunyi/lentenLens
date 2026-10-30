# Privacy Policy

**Effective Date**: 2026-10-30  
**Version**: 0.5.2

IntentLens ("we", "us", or "the extension") is committed to protecting your privacy. This policy explains what data IntentLens collects, how it is used, stored, and shared.

## Summary

- ✅ **No telemetry**: IntentLens does not collect usage analytics
- ✅ **Local-first**: All analysis data is stored locally on your machine
- ✅ **Explicit consent**: Code is sent to AI providers only when you explicitly enable and configure them
- ✅ **User control**: You control what files are analyzed via `.intentignore`
- ✅ **No tracking**: No cookies, trackers, or user profiling

## Data Collection and Usage

### 1. Local Data Storage

IntentLens stores analysis data locally in the following locations:

**Windows**: `%APPDATA%\IntentLens\projects\<project-hash>\`  
**Linux/macOS**: `~/.config/IntentLens/projects/<project-hash>/` (planned)

This local storage includes:

| Data Type | Purpose | Retention |
|-----------|---------|-----------|
| Source code snapshots | Round-based change tracking | Until manually deleted or excluded via `.intentignore` |
| Function evidence | Code analysis and explanation dependencies | Until source files change or are excluded |
| AI-generated explanations | Function and round explanations | Until manually deleted |
| Correction history | User-flagged incorrect explanations | Until manually deleted |
| Exercise attempts | Practice exercise answers and scores | Until manually deleted |
| Model usage logs | Token consumption tracking for budget control | Until manually deleted |

**Data Controller**: You are the data controller. All data remains on your local machine under your control.

### 2. Workspace Data

IntentLens reads files from your VS Code workspace:

- **Go Source Files**: `.go` files for function indexing and evidence collection
- **Configuration**: `.intentignore` for exclusion rules
- **Shared Data**: `.intent/shared/` for team-shared explanations (when confirmed by user)

**Excluded by Default**:
- `.git/` directories
- `.env*` files
- `*.pem`, `*.key` files (private keys)
- `node_modules/`, `vendor/` directories
- Files matching `.intentignore` patterns
- Binary files
- Symbolic links and directory junctions

**Single File Limit**: 1 MiB per file  
**Round Snapshot Limit**: 16 MiB per round

### 3. Data Sent to External Services

IntentLens sends data to external AI model providers **only when**:

1. You explicitly run the **"IntentLens: Enable or Configure Model Analysis"** command
2. You select a provider (OpenAI, Anthropic, Gemini, DeepSeek, Alibaba Qwen, or custom)
3. You review the configuration showing what data will be sent
4. You confirm the configuration

**What is Sent**:
- Go function source code and its direct dependencies (for function explanations)
- Round change snapshots (before/after file contents, for round explanations)
- Your questions (when using Q&A features)
- Tool call results (function signatures, identifier locations) when using OpenAI-compatible providers with tools enabled

**What is NOT Sent**:
- Files excluded by `.intentignore`
- Files outside your workspace
- API keys or environment variables
- Local correction reasons or private notes
- Unconfirmed shared explanation drafts
- VS Code settings or system information
- User identification or contact information

**Model Provider Privacy Policies**:

You should review the privacy policy of your chosen provider:
- [OpenAI Privacy Policy](https://openai.com/privacy/)
- [Anthropic Privacy Policy](https://www.anthropic.com/privacy)
- [Google AI Privacy Policy](https://policies.google.com/privacy)
- [DeepSeek Privacy Policy](https://www.deepseek.com/privacy)
- [Alibaba Cloud Privacy Policy](https://www.alibabacloud.com/help/en/legal/latest/privacy-policy)

IntentLens is not responsible for how third-party AI providers process your data.

### 4. API Keys

API keys are handled as follows:

- **Storage**: API keys are **never** stored by IntentLens
- **Configuration**: You specify an environment variable name (e.g., `OPENAI_API_KEY`)
- **Access**: IntentLens reads the key from the environment variable at runtime
- **Transmission**: Keys are sent in HTTP headers to the configured API endpoint (HTTPS only)
- **Logs**: API keys are **never** written to logs or shared data

**Recommendation**: Use separate API keys for IntentLens to enable easy rotation and usage tracking.

### 5. Team Shared Data

When you use the **"Prepare for Sharing"** feature:

- A new explanation is generated using only code evidence approved for sharing
- You preview the complete JSON before it's written to `.intent/shared/`
- Only after explicit confirmation is the file written to your workspace
- This file can be committed to Git for team sharing

**Shared files include**:
- Code evidence (file paths, line ranges, source snippets)
- AI-generated explanation text
- Verification status

**Shared files do NOT include**:
- Your local correction reasons
- Your questions or prompts
- API keys or model configuration
- Personal notes or annotations
- Exercise answers or scores

**Team members** who pull these shared files can:
- Read the explanations and evidence
- Flag issues locally (flags stay local)
- Generate revisions (which create new shared records when confirmed)

### 6. No Telemetry or Analytics

IntentLens does **not** collect:

- Usage statistics
- Error reports
- Performance metrics
- Feature usage patterns
- User demographics
- Crash dumps
- Network analytics
- A/B test data

We do not use any analytics services (e.g., Google Analytics, Mixpanel, Sentry).

### 7. No Cookies or Tracking

IntentLens does not:
- Set cookies
- Use browser fingerprinting
- Track your browsing behavior
- Create user profiles
- Share data with advertisers
- Use tracking pixels

## Data Security

### Local Data Protection

- **Access Control**: Local data inherits your operating system's file permissions
- **Encryption**: Data is stored unencrypted on your local file system (use OS-level encryption like BitLocker/FileVault if needed)
- **Backups**: You are responsible for backing up IntentLens data if desired

### Network Security

- **HTTPS Only**: All external API requests use HTTPS
- **No HTTP Fallback**: IntentLens refuses to send data over unencrypted HTTP
- **Certificate Validation**: SSL/TLS certificates are validated
- **Redirect Policy**: Only same-origin redirects are followed automatically

### Credential Security

- **Environment Variables**: API keys stored in environment variables (not in files)
- **No Hardcoding**: No credentials are hardcoded in the extension
- **No Logging**: Credentials are never logged or displayed in UI

## Data Retention and Deletion

### Local Data

You can delete IntentLens data at any time:

1. **Per-Project**: Delete the project folder in `%APPDATA%\IntentLens\projects\<hash>\`
2. **All Projects**: Delete the entire `%APPDATA%\IntentLens\` directory
3. **Shared Data**: Delete `.intent/` directory in your workspace

Uninstalling the extension does not automatically delete local data.

### Exclusion Rules

Adding patterns to `.intentignore` will:
- Prevent future capture of matching files
- Trigger deletion of historical snapshots and dependent explanations on next access
- Cannot retroactively delete already-committed Git history

### Model Provider Data

IntentLens cannot delete data stored by AI providers. Refer to each provider's data retention policy:
- **OpenAI**: 30 days for API data (as of policy date)
- **Anthropic**: Refer to their privacy policy
- **Others**: Check respective provider policies

## Your Rights

Depending on your jurisdiction (GDPR, CCPA, etc.), you may have rights regarding your personal data:

### Right to Access
All IntentLens data is stored locally on your machine and is directly accessible at any time.

### Right to Rectification
You can modify or correct any local data by:
- Editing files in the state directory
- Using the correction/flagging features in the UI
- Regenerating explanations

### Right to Erasure
You can delete IntentLens data as described in the "Data Retention and Deletion" section.

### Right to Data Portability
All IntentLens data is stored in JSON format and can be exported or migrated at any time.

### Right to Object
You can object to data processing by:
- Not enabling model analysis features
- Using `.intentignore` to exclude specific files
- Pausing model analysis via **"IntentLens: Pause Model Analysis"**

### Right to Withdraw Consent
You can withdraw consent at any time by:
- Pausing model analysis
- Removing API keys from environment variables
- Uninstalling the extension

## Children's Privacy

IntentLens is not directed at children under 13 (or 16 in the EU). We do not knowingly collect data from children. If you believe a child has used IntentLens and provided data to an AI provider, please contact the relevant provider.

## Changes to Workspace Files

IntentLens may modify workspace files only when:

1. **Shared Explanations**: After you confirm "Write Shared Record", a file is created in `.intent/shared/`
2. **Shared Exercises**: After you confirm "Write Shared Exercise", a file is created in `.intent/exercises/`
3. **Exercise Application**: After you confirm "Apply to Original Project", modified Go files are written

All write operations require explicit user confirmation with a preview.

## Third-Party Services

IntentLens may interact with:

| Service | Purpose | Data Sent | Your Action Required |
|---------|---------|-----------|---------------------|
| AI Model Provider | Generate explanations | Source code, questions | Enable & configure |
| VS Code Extension Host | Extension runtime | None | Install extension |
| Git (optional) | Version control | `.intent/` files | Manual commit |

IntentLens does not embed third-party analytics, advertising, or tracking services.

## International Data Transfers

If you configure an AI provider with servers outside your country:
- Your code will be transmitted internationally
- Data protection may vary by jurisdiction
- Review your provider's data residency options (e.g., OpenAI's EU region support)

IntentLens itself does not transfer data internationally.

## Data Breach Notification

In the event of a data breach affecting IntentLens:
- We will investigate and assess the scope
- Notify affected users via GitHub repository, email (if available), or extension update notification
- Provide guidance on mitigation steps

Since IntentLens stores data locally, breaches would typically involve:
- Compromise of your local machine (outside IntentLens control)
- Compromise of AI provider systems (provider's responsibility)

## Compliance

IntentLens is designed to support compliance with:
- **GDPR** (General Data Protection Regulation, EU)
- **CCPA** (California Consumer Privacy Act)
- **PIPEDA** (Personal Information Protection and Electronic Documents Act, Canada)

However, compliance ultimately depends on how you configure and use IntentLens.

## Enterprise and Team Use

For enterprise deployments:
- **Data Residency**: All analysis data stays on developers' machines or designated state directories
- **Access Logs**: Not currently implemented (planned future feature)
- **SSO/Authentication**: Not applicable (IntentLens is a local-first tool)
- **Audit Trails**: Round capture history is available in local state
- **Data Loss Prevention**: Use `.intentignore` to enforce organizational policies

## Contact and Questions

For privacy-related questions or concerns:

📧 **Email**: 16671100373@163.com  
🐙 **GitHub Issues**: [lentenLens Repository](https://github.com/jianyunyi/lentenLens/issues) (for non-sensitive inquiries)

For security vulnerabilities, see [SECURITY.md](SECURITY.md).

## Updates to This Policy

We may update this Privacy Policy to reflect:
- Changes in IntentLens features
- Changes in applicable laws
- User feedback and best practices

**Notification of Changes**:
- Updated policy will be published in the repository
- Version number and effective date will be updated
- Major changes will be announced in release notes

**Your Continued Use**: Continued use of IntentLens after policy updates constitutes acceptance of the revised policy.

## Legal Basis for Processing (GDPR)

For users in the EU/EEA, our legal basis for processing is:
- **Consent**: You explicitly enable model analysis features
- **Legitimate Interest**: Local storage for tool functionality
- **Contract**: Providing the service you've chosen to use

You can withdraw consent or object to processing at any time without affecting prior processing.

---

## Appendix: Data Flow Diagram

```
┌─────────────────────────────────────────────────────┐
│  Your VS Code Workspace                             │
│  ├── *.go files (source code)                       │
│  ├── .intentignore (exclusion rules)                │
│  └── .intent/shared/ (confirmed team data)          │
└─────────────────────────────────────────────────────┘
                    │
                    │ Reads (respects .intentignore)
                    ▼
┌─────────────────────────────────────────────────────┐
│  IntentLens Extension (local process)               │
│  ├── Indexes Go functions (local AST)               │
│  ├── Captures round snapshots (local)               │
│  └── Reads API key from environment (runtime only)  │
└─────────────────────────────────────────────────────┘
                    │
      ┌─────────────┴─────────────┐
      │                           │
      ▼                           ▼
┌─────────────────┐    ┌──────────────────────────────┐
│  Local Storage  │    │  AI Provider (if enabled)    │
│  %APPDATA%\     │    │  ├── OpenAI                  │
│  IntentLens\    │    │  ├── Anthropic               │
│  projects\...   │    │  ├── Google Gemini           │
│                 │    │  ├── DeepSeek                │
│  [No upload]    │    │  └── Alibaba Qwen            │
└─────────────────┘    │                              │
                       │  Receives: code + questions  │
                       │  Returns: explanations       │
                       └──────────────────────────────┘
                                   │
                                   │ HTTPS only
                                   │ User explicitly enabled
                                   │ API key from environment
                                   │
                              [Your Data]
```

---

**Last Updated**: 2026-10-30  
**Version**: 0.5.2  
**Language**: 中文版本请见 PRIVACY.zh-CN.md (planned)
