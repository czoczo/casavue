# CasaVue Changelog

[ Application Changelog ] | [ [Helm Chart Changelog](./deployment/helm/CHANGELOG.md) ]

## 0.1.4
<WiP> - added default favicon path search
- updated container building docs
- App:
  - Fixed process crash when the dashboard API read items while they were updated.
  - Fixed crash on Ingress without rules and on delete events after a watch reconnect.
  - Fixed items coming back after delete or rename when a late icon crawl wrote them back.
  - Items are keyed by namespace and object name, so the same name in two namespaces no longer overwrites itself. API `/api/v1` keys and generated avatar URLs changed accordingly, item display name is now a `name` field.
  - `/statusCheck/` no longer accepts an arbitrary URL, it checks a dashboard item by key.
  - Icon crawls are limited to 8 at a time and to 2 MB per page, so a big site cannot exhaust container memory.
  - Dashboard items are no longer re-crawled on Kubernetes watch events which change nothing on the dashboard (e.g. status only updates).
  - Fixed static mode (`-static`) crashing when the avatars folder does not exist.
  - Fixed static mode saving an error page or a truncated file as an icon; the original icon URL is kept when the download fails.
  - Fixed `casavue.app/enable=false` not hiding an item in `ingressAnnotation` mode.
  - One HTTP request less per item when looking up icons.
  - Icon size attributes like `32x32` are read as 32 instead of 3232, so the largest icon of a site is picked correctly.
  - Customization values containing `$` (title, app name) no longer break the generated `index.html`.
  - Generated avatars are cached by the browser for a day instead of being generated again on every view.
  - Removed unused dependency `PerformLine/go-stockutil`; 6 modules less in `go.mod` and in image scans.
- WebUI:
  - Fixed dashboard breaking after an item was removed while its status was being checked.
  - Status of removed items is no longer kept and re-checked.
- Docker:
  - The Vue build stage ran on the end-of-life `node:18` image, whose `corepack` cannot start the pinned pnpm 12 (`Cannot find module .../pnpm.cjs`): the image build failed. It now uses `node:24`.
  - Added `.dockerignore`. Every build sent 280 MB to the Docker daemon (220 MB of it the local `node_modules`) and the Vue stage installed from that copy instead of from `pnpm-lock.yaml`.
  - Without a Kubernetes configuration (plain `docker compose up`) the app exited immediately and the container answered nothing. It now serves the items of the config files and logs a warning.
- CI: Go vet and tests run before a container image is published.
- CI: `GITHUB_TOKEN` permissions narrowed from `write-all` to what each workflow uses; stale `.trivyignore` entry removed.

## 0.1.3
- WebUI: Fixed items not showing on dashboard when no labels on Ingress resource.
- App:
  - Updated icon source from "https://github.com/walkxcode/dashboard-icons" to "https://github.com/homarr-labs/dashboard-icons".
  - Added support for overriding icons in static item entries.
  - Added library version locks.


## 0.1.2
- WebUI: Fixed dashboard items update - prevented caching of API endpoint by ServiceWorker.

## 0.1.1
- App: added self skip on Kubernetes Ingress crawl.
- WebUI: added auto refresh after PWA update button click.

## 0.1.0
- Initial release.
