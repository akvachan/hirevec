# Hirevec

## About Hirevec

Hirevec is a modern job recommendation engine that finds jobs and suitable candidates.

## Quick start

Start server with:

```sh
go run cmd/hvserver/main.go
```

Open `http://localhost:6969/health`.

## Recommendation Engine

The recommendation engine currently only employs a content-based approach. There are currently two stages:

1. Filter

   Either via database-powered text search or vector embeddings, compare position text to a candidate profile.

2. Rerank (Optional)

   One can enable a reranker to reduce the pool of candidates further and establish a better ranking, this can be done regradless of which database you use.

## Documentation

The documentation for the server is available at `./openapi.yaml`. Both the `openpapi.yaml` and the
SwagerUI docs are served (SwagerUI requires working internet connection to download the bundle):

- SwagerUI: `http://localhost:8888/docs`
- OpenAPI YAML: `http://localhost:8888/openapi.yaml`

## Evaluation

### Pulling evaluation dataset

```sh
git lfs pull data/evaluation_dataset/1/
```

### Synthesizing evaluation dataset

```sh
go run cmd/genevals/main.go
```

or

```sh
go run cmd/genevals/main.go \
    --out data/recommendations/evals/1 \
    --seed 42 \
    --positions 30000 \
    --candidates 100 \
    --top 10
```

## Server Features

All currently available server settings are demonstrated in [.example.env](./.example.env).
You can configure them in a `.env` file of a current working directory or via environment variables.

## Development

### Passkeys for local development / testing

The server enforces the full WebAuthn ceremony validation in every environment - authentication is
never bypassed during development. A passkey page is served at `/passkeys`: create an account
with a passkey (email + full name, secured by Touch ID or any platform authenticator) and sign back
in with a single click - no details needed. The page displays the resulting access and refresh
tokens; use them in Bruno or any HTTP client as a Bearer token.

Open `http://localhost:6969/passkeys`. The relying party ID and origins default to the host of
`HIREVEC_URL` - test against the same origin the server is configured with.

### Hot-reload

```sh
air \
  --build.cmd "go build -o bin/app cmd/hvserver/main.go" \
  --build.entrypoint "./bin/app" \
  --build.include_ext "go,sql,env,json" \
  --build.kill_delay "50000000000" \
  --build.log "server.log" \
  --build.send_interrupt true \
  --color never \
  --log.main_only true \
  --log.silent true \
  --misc.clean_on_exit true \
  --screen.clear_on_rebuild true \
  --screen.keep_scroll false \
  --tmp_dir "bin"
```
