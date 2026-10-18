# TRON frontend

The landing page uses React, TypeScript, and Tailwind CSS. It opens the existing
TRON workspace and submits questions through its existing API integration.
The workspace keeps its source inspector, graph, history, and draft workflows.

## Build

From the repository root, with Node.js 22.12+ installed:

```sh
npm ci --prefix frontend
npm run build --prefix frontend
go run ./cmd/tron serve
```

Configure FalkorDB and optional Ollama as described in the root README first.
Open the server URL, or append `#workspace` to go directly to the workspace.

The build writes `internal/ui/hero-assets/hero.js` and `hero.css`. Commit these
generated files with source changes: Go embeds them, so normal Go/Docker builds
do not require Node.js. Rebuild and restart Go after changing frontend assets.

The background video loads from the supplied CloudFront URL; Google Fonts loads
Readex Pro, Geist, and Silkscreen. Offline, system fonts and a neutral background
remain available. Reduced-motion preferences disable autoplay; a visible control
also pauses motion. The video is decorative and does not affect graph operations.

The displayed workflow count describes implemented features; no customer counts
or testimonials are fabricated. The demo continues to use synthetic company data.
