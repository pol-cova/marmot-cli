# Contributing to Marmot

Thank you for your interest in contributing to Marmot! This document provides guidelines and instructions for contributing.

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating a bug report, please:

1. Check the [existing issues](https://github.com/pol-cova/marmot-cli/issues) to see if the problem has already been reported
2. Try to isolate the problem and provide a minimal reproducible example

When reporting a bug, please include:

- **OS and version**: e.g., Ubuntu 22.04, macOS 14
- **Go version**: `go version`
- **Marmot version**: `marmot --version`
- **Steps to reproduce**: Clear steps to reproduce the issue
- **Expected behavior**: What you expected to happen
- **Actual behavior**: What actually happened
- **Logs**: Any relevant log output (with sensitive data redacted)

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, please include:

- **Clear title**: Describe the enhancement concisely
- **Detailed description**: Explain the feature and why it would be useful
- **Use cases**: Describe specific use cases for the feature
- **Potential implementation**: If you have ideas on how to implement it

### Pull Requests

1. Fork the repository
2. Create a new branch from `main`: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests: `make test`
5. Run linter: `make lint`
6. Commit your changes: `git commit -am 'Add new feature'`
7. Push to your fork: `git push origin feature/my-feature`
8. Create a Pull Request

#### Pull Request Guidelines

- Keep changes focused and atomic
- Update documentation if needed
- Add tests for new functionality
- Ensure all tests pass
- Follow existing code style and conventions
- Reference any related issues

## Development Setup

### Prerequisites

- Go 1.25 or later
- Docker (for testing database discovery)
- Make

### Building

```bash
# Clone the repository
git clone https://github.com/pol-cova/marmot-cli.git
cd marmot

# Build the binary
make build

# Run tests
make test

# Run linter
make lint
```

### Project Structure

```
cmd/marmot/         # CLI entry point and commands
internal/           # Private application code
  ├── agent/        # Backup orchestration
  ├── backup/       # Database dumpers
  ├── client/       # Hub API client
  ├── config/       # Configuration management
  ├── crypto/       # Encryption
  ├── discovery/    # Auto-discovery
  └── storage/      # Local storage & queue
```

### Code Style

We follow standard Go conventions:

- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go) guidelines
- Write clear, concise comments for exported functions
- Use meaningful variable and function names
- Keep functions focused and small
- Handle errors explicitly and appropriately

### Testing

- Write tests for new functionality
- Use table-driven tests where appropriate
- Mock external dependencies
- Aim for good test coverage

Run tests:
```bash
make test
```

Generate coverage report:
```bash
make test-coverage
```

### Commit Messages

Use clear and meaningful commit messages:

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests where appropriate

Example:
```
Add support for PostgreSQL 15

- Update pg_dump detection for version 15
- Add integration test for PostgreSQL 15
- Update documentation

Fixes #123
```

## Release Process

Maintainers will handle releases following this process:

1. Update `CHANGELOG.md`
2. Update version in `cmd/marmot/main.go`
3. Create a new release on GitHub
4. Upload compiled binaries for all platforms

## Questions?

If you have questions or need help, please:

- Open an issue for discussion
- Check existing documentation
- Reach out to maintainers

## License

By contributing to Marmot, you agree that your contributions will be licensed under the project's license.
