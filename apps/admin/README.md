# Ruto Admin

Vue 3 + TypeScript admin SPA for the **ruto** API gateway control plane (`core`).
Dark theme, English only, built with [Naive UI](https://www.naiveui.com/).

## Stack

- Vue 3 (Composition API, `<script setup>`)
- Vue Router 4 (workspace-style navigation, route guards)
- Pinia (session + UI state)
- Naive UI (dark theme via `darkTheme` + theme overrides)
- Vite 7

## Architecture

```
src/
  api/          thin typed client over the core REST API (/api/*)
  composables/  reusable hooks: useEntityForm, useDrawerResource, ...
  stores/       pinia stores: auth, apps, ui, snapshot
  router/       routes + auth/admin guards
  layouts/      DefaultLayout (header nav + app sidebar)
  components/   editors/, display/, common/, app/, endpoint/, usr/, gateway/
  views/        page components
  lib/          datetime/format helpers
  assets/       global styles & design tokens
```

The UI mirrors the domain hierarchy **Root → App → Endpoint**: pick an app in the
left sidebar to open its workspace (config + endpoints); global config, users and
gateways live in the header navigation. Entities are created/edited in drawers and
modals (no full-page form navigation).

## Commands

```bash
pnpm install
pnpm dev          # vite dev server
pnpm build        # type-check + production build → dist/
pnpm preview      # preview the production build
```

`core` serves the built SPA from `apps/admin/dist` (see `internal/app/core/spa.go`).

## Configuration

The admin signs in with core's HttpOnly session cookie (`POST /api/session`), so the
API must be same-origin with the page. In production core serves both. In development
the vite dev server proxies `/api` to core: set `VITE_API_PROXY_TARGET` (default
`http://localhost:80`) and leave `VITE_API_BASE_URL` unset — pointing it at another
origin breaks sign-in, the browser does not send the cookie there.
