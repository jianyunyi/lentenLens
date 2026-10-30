# Security Policy

## Supported Versions

IntentLens is currently in active development. Security updates are provided for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 0.5.x   | :white_check_mark: |
| < 0.5.0 | :x:                |

## Security Model

### Execution Boundaries

**IMPORTANT**: The exercise execution feature (`exercise-run`) is currently **disabled** due to lack of OS-level sandboxing. IntentLens does not currently provide operating system-enforced isolation for executed code.

- **Local File Access**: IntentLens reads Go source code from your workspace
- **API Keys**: API keys are read from environment variables, never stored in configuration files
- **Model Interactions**: Code is sent to configured AI model providers only when explicitly enabled by the user
- **Workspace Trust**: IntentLens requires workspace trust in VS Code to execute CLI commands

### Data Handling

- **Local Storage**: Analysis data is stored in `%APPDATA%\IntentLens\projects\` (Windows) or equivalent user config directory
- **Shared Data**: Only explicitly confirmed shared explanations are written to `.intent/shared/`
- **Exclusions**: `.git`, `.env*`, `*.pem`, `*.key`, and custom `.intentignore` patterns are excluded from capture
- **No Telemetry**: IntentLens does not collect or transmit usage telemetry

### Network Communication

IntentLens communicates with external services only when:
- User explicitly enables model analysis
- User configures API provider and confirms sending scope
- User initiates explanation generation, Q&A, or exercise features

Supported providers: OpenAI, Anthropic, Google Gemini, DeepSeek, Alibaba Qwen, and OpenAI-compatible services.

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

**Please DO NOT open a public GitHub issue for security vulnerabilities.**

Instead, please report security issues via email to:

📧 **16671100373@163.com**

### What to Include

When reporting a vulnerability, please include:

1. **Description**: Clear description of the vulnerability
2. **Impact**: Potential impact and attack scenario
3. **Reproduction**: Step-by-step instructions to reproduce the issue
4. **Environment**: 
   - IntentLens version
   - VS Code version
   - Operating system
   - Go version (if relevant)
5. **Proof of Concept**: Code, screenshots, or videos demonstrating the issue (if applicable)
6. **Suggested Fix**: If you have ideas on how to fix the issue

### Response Timeline

We aim to respond to security reports according to the following timeline:

- **Initial Response**: Within 72 hours of report submission
- **Triage**: Within 7 days, we will confirm the vulnerability and assess severity
- **Fix Development**: Critical issues will be prioritized for immediate fix
- **Disclosure**: After a fix is available, we will coordinate public disclosure with the reporter

### Severity Assessment

We use the following severity levels:

- **Critical**: Remote code execution, privilege escalation, data exfiltration
- **High**: Authentication bypass, significant data exposure, DOS attacks
- **Medium**: XSS, CSRF, moderate information disclosure
- **Low**: Minor information disclosure, issues requiring significant user interaction

### Security Advisories

Security advisories will be published:
- In the [GitHub repository's Security tab](https://github.com/jianyunyi/lentenLens/security/advisories)
- In release notes for patched versions
- Referenced in CHANGELOG.md

### Bug Bounty

IntentLens does not currently have a bug bounty program. However, we deeply appreciate security researchers' contributions and will publicly acknowledge reporters (with permission) in:
- Security advisories
- Release notes
- CONTRIBUTORS.md file

## Security Best Practices for Users

### For Individual Users

1. **Review API Provider**: Understand what code is sent to your configured AI provider
2. **API Key Security**: Store API keys in environment variables, never in code
3. **Exclude Sensitive Files**: Use `.intentignore` to exclude files containing secrets, credentials, or sensitive business logic
4. **Workspace Trust**: Only open trusted workspaces in VS Code
5. **Review Changes**: Review AI-generated explanations and modifications before applying
6. **Budget Limits**: Configure appropriate token budget limits to prevent unexpected costs

### For Team/Enterprise Use

1. **Shared Explanations**: Review all shared explanations before committing to Git
2. **Access Control**: Use `.intentignore` to prevent sensitive code from being analyzed
3. **API Key Rotation**: Rotate API keys regularly
4. **Network Policies**: Ensure AI provider endpoints are allowed through corporate firewalls
5. **Audit Logs**: Review IntentLens state directories for unexpected activity
6. **Version Control**: Pin to specific IntentLens versions in production environments

## Known Limitations

The following are known security limitations documented for transparency:

1. **No OS Sandbox**: Exercise execution is disabled because file isolation does not prevent network access or resource exhaustion
2. **File Access**: IntentLens CLI can read any file in the workspace (respecting `.intentignore`)
3. **API Key Exposure**: Environment variables are accessible to the extension process
4. **Model Trust**: IntentLens cannot verify the correctness of AI-generated explanations
5. **Symlink Handling**: Symbolic links and directory junctions are skipped to prevent path traversal

## Future Security Enhancements

Planned security improvements:

- [ ] Windows AppContainer/Lowbox implementation for exercise execution
- [ ] Atomic execution logging for crash recovery
- [ ] Enhanced API key handling with OS keychain integration
- [ ] Content Security Policy for webviews
- [ ] Signed releases and binary verification
- [ ] SBOM (Software Bill of Materials) generation

## Security Updates

Security updates will be released as patch versions (e.g., 0.5.3) and clearly marked in release notes. Users are encouraged to:

- Enable automatic updates in VS Code
- Subscribe to GitHub releases
- Review CHANGELOG.md regularly

## Questions?

For security-related questions that are not vulnerability reports, please:
- Open a [GitHub Discussion](https://github.com/jianyunyi/lentenLens/discussions)
- Or contact the maintainers at 16671100373@163.com

---

**Last Updated**: 2026-10-30  
**Version**: 0.5.2
