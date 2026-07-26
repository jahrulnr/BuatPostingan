# API service notes

HTTP/SSE shapes for the FE real driver are implemented under `/api/webchat`
(see [`../architecture/README.md`](../architecture/README.md) ports table and
[`../architecture/turn-loop.md`](../architecture/turn-loop.md)).

Presenter helpers live in `delivery/presenter/`. Mount without static FE via
`MountWebchatAPI(mux, uc)` — see [`../architecture/portable-ai-kit.md`](../architecture/portable-ai-kit.md).

Draft static-page preview is a separate read-only mount:

```go
httpdelivery.MountPagePreview(mux, pagesRoot)
```

It serves `GET /api/pages/{page-id}/` and page-local assets with no-cache and
symlink/traversal protection. Contract: [`../architecture/static-pages.md`](../architecture/static-pages.md).
