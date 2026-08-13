SQL migrations for the `kids_subjects`-successor schema, managed by [goose](https://github.com/pressly/goose).

Create a new migration:

    go run ./cmd/migrate create <name> sql

Apply all pending migrations (requires `DATABASE_URL`, e.g. from `.env`):

    DATABASE_URL=postgres://coglab:changeme@localhost:5432/coglab?sslmode=disable \
      go run ./cmd/migrate up

Check status:

    go run ./cmd/migrate status
