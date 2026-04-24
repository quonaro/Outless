# Contributing to Outless Backend

Thanks for considering a contribution.

## Before you start

- Open an issue for large changes (architecture, runtime model, schema changes)
- Keep pull requests focused and small
- Add tests for behavior changes

## Development flow

1. Create a branch from `main`
2. Implement the change
3. Run checks locally:

   ```bash
   go test ./...
   ```

4. Update docs/config examples if needed
5. Open a PR with:
   - clear problem statement
   - solution summary
   - test evidence

## Code conventions

- Follow clean architecture boundaries (`cmd` -> `internal/app` -> `internal/domain`)
- Prefer constructor injection, avoid global mutable state
- Keep logs structured and avoid secret leakage

## Xray-specific changes

Because Outless relies on Xray technology, any PR touching Xray behavior must include:

- runtime mode impact (`embedded` vs `external`)
- config compatibility notes (`xray.edge` / `xray.probe`)
- migration or rollback steps if behavior changes
- tests covering failure and startup paths where applicable

## Commit style

Use Conventional Commits:

- `feat: ...`
- `fix: ...`
- `docs: ...`
- `refactor: ...`
- `test: ...`

## Security

Never commit secrets, private keys, or production credentials.
