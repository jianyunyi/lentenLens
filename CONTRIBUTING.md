# Contributing to IntentLens

Thank you for your interest in contributing to IntentLens! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Pull Request Process](#pull-request-process)
- [Testing Guidelines](#testing-guidelines)
- [Coding Standards](#coding-standards)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Reporting Issues](#reporting-issues)
- [Security Vulnerabilities](#security-vulnerabilities)

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for all contributors. We expect:

- **Be respectful**: Treat all contributors with respect and professionalism
- **Be constructive**: Provide helpful feedback and suggestions
- **Be inclusive**: Welcome contributors of all backgrounds and experience levels
- **Be patient**: Help newcomers and answer questions kindly

Unacceptable behavior includes harassment, discriminatory comments, personal attacks, or any conduct that creates an unwelcoming environment.

## Getting Started

### Prerequisites

Before contributing, ensure you have:

- **Go 1.24+** (Go 1.26.3 recommended for reproducible builds)
- **Node.js 22+** (for extension development)
- **VS Code** (for extension testing)
- **Git** (for version control)
- **Windows, Linux, or macOS** (Windows has the most complete test coverage)

### First-Time Contributors

If this is your first contribution:

1. Fork the repository on GitHub
2. Clone your fork locally
3. Create a feature branch
4. Make your changes
5. Test thoroughly
6. Submit a pull request

Good first issues are labeled with `good first issue` on GitHub.

## Development Setup

### 1. Clone the Repository

```bash
git clone https://github.com/jianyunyi/lentenLens.git
cd lentenLens
```

### 2. Install Dependencies

```bash
# Go dependencies (none external - uses standard library only)
go mod download

# Node dependencies for extension
cd extension
npm install
cd ..
```

### 3. Build the CLI

```bash
# Windows
go build -trimpath -o bin/intentlens.exe ./cmd/intentlens

# Linux/macOS
go build -trimpath -o bin/intentlens ./cmd/intentlens
```

### 4. Verify Installation

```bash
# Run tests
go test ./...
go vet ./...

# Run extension tests
cd extension
npm test
npm run check
cd ..

# Run offline preflight
node scripts/preflight.cjs --entry ide
```

## Development Workflow

### Branch Strategy

- `main` - Stable release branch
- `develop` - Active development branch (if used)
- `feature/*` - New features
- `fix/*` - Bug fixes
- `docs/*` - Documentation updates
- `security/*` - Security fixes

### Making Changes

1. **Create a branch** from `main`:
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. **Make your changes**:
   - Write clean, well-documented code
   - Follow coding standards (see below)
   - Add tests for new functionality
   - Update documentation as needed

3. **Test your changes**:
   ```bash
   go test ./...
   cd extension && npm test && cd ..
   ```

4. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat: add new feature description"
   ```

5. **Push to your fork**:
   ```bash
   git push origin feature/my-new-feature
   ```

6. **Open a Pull Request** on GitHub

## Pull Request Process

### Before Submitting

- [ ] All tests pass locally
- [ ] Code follows project style guidelines
- [ ] Documentation is updated (if applicable)
- [ ] CHANGELOG.md is updated (under `[Unreleased]`)
- [ ] Commit messages follow conventions
- [ ] No sensitive data (API keys, credentials) in commits
- [ ] Branch is up-to-date with `main`

### PR Title Format

Use conventional commit format:

```
<type>(<scope>): <description>

Examples:
feat(cli): add new command for exercise validation
fix(extension): resolve CodeLens performance issue
docs(readme): update installation instructions
security(capture): improve file exclusion validation
```

### PR Description Template

```markdown
## Description
Brief description of changes

## Motivation
Why is this change necessary?

## Changes
- Change 1
- Change 2

## Testing
How was this tested?

## Screenshots (if applicable)

## Breaking Changes
Any breaking changes? How to migrate?

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
```

### Review Process

1. **Automated Checks**: CI must pass (Go tests, extension tests, linting)
2. **Code Review**: At least one maintainer approval required
3. **Security Review**: Required for changes affecting data handling, API calls, or file operations
4. **Testing**: Maintainers may request additional testing or evidence

### Merge Strategy

- **Squash and Merge**: Small PRs with single logical change
- **Rebase and Merge**: Multi-commit PRs with clean commit history
- **Regular Merge**: Large feature branches (rare)

## Testing Guidelines

### Go Tests

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -run TestName ./internal/package

# Run benchmarks
go test -bench . ./internal/capture
```

### Extension Tests

```bash
cd extension
npm test                # Run all tests
npm run check          # Syntax checks
```

### Integration Tests

```bash
# Smoke tests (creates isolated fixture)
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke.ps1 -WorkDirectory 'D:\test-work'

# VS Code integration (requires display)
node scripts/vscode-integration.cjs
```

### Test Requirements

- **Unit tests** for all new functions
- **Integration tests** for CLI commands
- **Regression tests** for fixed bugs
- **Performance tests** for capture/indexing changes
- **Edge cases**: empty inputs, large files, malformed data, concurrent access

### Test Coverage Goals

- **Core packages**: >80% coverage
- **CLI commands**: >70% coverage
- **Critical paths**: 100% coverage (capture, model calls, file writes)

## Coding Standards

### Go Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Use `go vet` for static analysis
- Avoid external dependencies (standard library only)
- Maximum function length: ~50 lines (exceptions allowed)
- Maximum file length: ~1000 lines

**Example**:
```go
// Good
func processFile(path string) (*Result, error) {
    if path == "" {
        return nil, fmt.Errorf("path cannot be empty")
    }
    
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading file: %w", err)
    }
    
    return &Result{Data: data}, nil
}

// Bad (no error wrapping, no validation)
func processFile(path string) *Result {
    data, _ := os.ReadFile(path)
    return &Result{Data: data}
}
```

### JavaScript Code Style (Extension)

- Use Node.js 22+ features
- No external runtime dependencies (dev dependencies OK)
- Async/await for asynchronous operations
- Descriptive variable names
- JSDoc comments for public APIs

**Example**:
```javascript
// Good
/**
 * Executes IntentLens CLI command
 * @param {string} command - Command name
 * @param {object} input - JSON input object
 * @returns {Promise<object>} Parsed JSON response
 */
async function executeCommand(command, input) {
    const result = await execCLI(command, JSON.stringify(input));
    return JSON.parse(result.stdout);
}

// Bad (no types, no error handling)
async function exec(cmd, inp) {
    return JSON.parse((await execCLI(cmd, inp)).stdout);
}
```

### Error Handling

- Always handle errors explicitly
- Use `fmt.Errorf` with `%w` for error wrapping
- Return errors, don't panic (except for truly unrecoverable situations)
- Provide context in error messages

### Security Practices

- **Never** log sensitive data (API keys, file contents with potential secrets)
- Validate all user inputs
- Use absolute paths to prevent directory traversal
- Sanitize file paths before operations
- Check file permissions before reading/writing
- Use HTTPS for all external requests

## Commit Message Guidelines

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Build process, dependency updates
- `security`: Security fixes

### Scope

Package or component affected: `cli`, `extension`, `capture`, `explain`, `exercise`, `codeindex`, `gonav`

### Examples

```
feat(cli): add integration-info command for generic IDE support

Implements machine-readable contract discovery for IDE integrations.
Returns JSON schema describing available commands, data locations,
and feature capabilities.

Closes #123
```

```
fix(capture): prevent race condition in concurrent file reads

Added mutex protection around shared state in capture round operations.
Fixes panic when multiple goroutines access the same baseline.

Fixes #456
```

```
security(capture): validate symlink targets before traversal

Added check to prevent following symlinks outside workspace.
Mitigates potential directory traversal attack.

CVE-XXXX-XXXXX
```

## Reporting Issues

### Before Opening an Issue

- Search existing issues (open and closed)
- Check documentation and FAQs
- Try the latest version
- Verify it's not a user configuration issue

### Bug Report Template

```markdown
**Describe the bug**
Clear description of what went wrong

**To Reproduce**
Steps to reproduce:
1. Go to '...'
2. Click on '...'
3. See error

**Expected behavior**
What you expected to happen

**Actual behavior**
What actually happened

**Environment**
- IntentLens version: [e.g., 0.5.2]
- VS Code version: [e.g., 1.90.0]
- OS: [e.g., Windows 11, Ubuntu 22.04]
- Go version: [e.g., 1.24.0]

**Logs**
Relevant error messages or logs

**Additional context**
Any other relevant information
```

### Feature Request Template

```markdown
**Problem Statement**
What problem does this solve?

**Proposed Solution**
How should it work?

**Alternatives Considered**
Other approaches you've thought of

**Additional Context**
Examples, mockups, or references
```

## Security Vulnerabilities

**Do NOT open public issues for security vulnerabilities.**

See [SECURITY.md](SECURITY.md) for reporting process.

## Documentation

### What Needs Documentation

- New features
- CLI commands
- Configuration options
- API changes
- Breaking changes
- Migration guides

### Where to Update

- `README.md` - User-facing features and quick start
- `docs/*.md` - Detailed documentation
- `CHANGELOG.md` - All changes in `[Unreleased]` section
- `extension/package.json` - Command descriptions
- Code comments - JSDoc/GoDoc for public APIs

### Documentation Style

- Use clear, concise language
- Provide examples
- Include screenshots for UI changes
- Write in present tense
- Use gender-neutral pronouns

## License

By contributing to IntentLens, you agree that your contributions will be licensed under the MIT License.

All submitted code must be compatible with the MIT License and cannot introduce dependencies with incompatible licenses.

## Questions?

- **General questions**: Open a [GitHub Discussion](https://github.com/jianyunyi/lentenLens/discussions)
- **Bug reports**: Open a [GitHub Issue](https://github.com/jianyunyi/lentenLens/issues)
- **Security issues**: Email 16671100373@163.com (see SECURITY.md)
- **Feature proposals**: Open a [GitHub Issue](https://github.com/jianyunyi/lentenLens/issues) with `enhancement` label

## Recognition

Contributors will be recognized in:
- GitHub contributor list
- Release notes (for significant contributions)
- CONTRIBUTORS.md file (planned)

Thank you for contributing to IntentLens! 🎉

---

**Last Updated**: 2026-10-30
