# Contributing

Thank you for considering contributing to remotty!

## Code of Conduct

Be respectful, inclusive, and constructive.

## How to Contribute

1. **Fork** the repository
2. **Create a branch** for your feature/fix: `git checkout -b feat/my-feature`
3. **Make your changes**
4. **Run tests**: `make test`
5. **Commit** with clear messages: `git commit -m "feat: add QR pairing support"`
6. **Push** and open a **Pull Request**

## Development Setup

```bash
git clone https://github.com/remotty/remotty.git
cd remotty
go mod download
make build-all
```

## Project Structure

See [README.md](README.md#architecture) for architecture overview.

## Coding Standards

- **Go:** Standard `gofmt`, follow Go conventions
- **TypeScript:** ESLint + Prettier
- **Swift:** Follow Swift API design guidelines
- **Commits:** Conventional commits (feat:, fix:, docs:, etc.)

## Testing

```bash
make test        # All tests
make test-short  # Quick tests
```

## Pull Request Process

1. Update documentation if needed
2. Add tests for new features
3. Ensure CI passes
4. Request review from maintainers

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
