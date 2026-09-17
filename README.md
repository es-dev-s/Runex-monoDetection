# Detection Engine

Deterministic stack detector for the deployment platform.

It accepts a GitHub URL or a zip upload, isolates the work in a per-request workspace, and returns a normalized stack with evidence and confidence. Specfy is not the authority. Manifests and rules are.

## Why this shape

A hybrid detector is the right production model. A sequential Specfy → Linguist → parser pipeline is not.

- Specfy is a TypeScript library. Shelling out to Node from a Go service adds a second runtime, slower cold starts, and another failure domain. The catalog layer here covers that job natively.
- GitHub Linguist in-process is `go-enry`, the Go port used in production scanners.
- Manifest parsers and the rule engine are the source of truth for deploy decisions.
- Confidence is evidence-gated. Three independent signals (dependency + config + script) is 100%. A lone language guess is not.

100% means "the framework is proven," not "every repository on Earth is classifiable." Unknown repos return a language-only result and a warning.

## API

```
GET  /healthz
POST /v1/detect/github   {"url":"owner/repo"|"https://github.com/owner/repo","ref":"optional"}
POST /v1/detect/zip      multipart field: file
```

Optional `X-GitHub-Token` or `GITHUB_TOKEN` for private clones. Tokens are never written to logs or results.

```bash
go test -race -count=1 ./...
DETECTOR_ADDR=127.0.0.1:8090 go run ./cmd/server
```
