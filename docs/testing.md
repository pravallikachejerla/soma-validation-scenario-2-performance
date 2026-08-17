# Testing

The repository ships with two test suites that share the same Go
toolchain but serve different audiences.

## Public tests

The public suite lives under `tests/public/` and is wired into the
normal `go test ./...` invocation. It exercises the API, the
pricing engine, the promotion resolver and the batch path against
the seeded synthetic dataset.

To run the public suite:

```bash
go test ./tests/public/...
```

Public tests do not require a live database. They use the
in-memory store and the `small` profile fixture that ships with
the repository.

## Private tests

The private suite lives under `private/` and is **not** part of
the normal build cycle. It exists for reviewers who need to
exercise the implementation in isolation against documented
expectations. The private suite uses table-driven tests with
descriptive names and exercises synthetic-sensitive behaviour that
the public suite deliberately avoids.

To run the private suite:

```bash
go test ./private/...
```

The private suite is organised as one test per documented area of
interest. Each test is self-contained and reads only the in-memory
store and the seed fixture. The private suite never reads the
public test sources and never asserts on the public API surface.

## Adding a new test

1. Place the test under `tests/public/` if it represents a normal
   engineering check that should keep passing for every change.
2. Place the test under `private/` if it represents a
   reviewer-only expectation that documents an isolated behaviour.
3. Use `seeddata` and the in-memory store rather than touching the
   PostgreSQL store; the public suite must stay portable.
4. Do not include synthetic-sensitive markers in the test code;
   use the synthetic identifiers from the seed fixtures.
5. Run the relevant `go test` command locally before opening a
   pull request.
