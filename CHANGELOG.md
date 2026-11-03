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
  - HTTPRoute links are `https://` when a listener of the Gateway they attach to terminates TLS for their hostname, like an Ingress with a `tls` section; plain `http://` is only the fallback. A route with several hostnames is linked to one the Gateway serves. Needs read access to `gateways` (`gateway.networking.k8s.io`), added to the Helm ClusterRole; without it the routes are still shown, with `http://`.
  - Fixed static mode (`-static`) crashing when the avatars folder does not exist.
  - Fixed static mode saving an error page or a truncated file as an icon; the original icon URL is kept when the download fails.
  - Fixed `casavue.app/enable=false` not hiding an item in `ingressAnnotation` mode.
  - Icons are picked from every icon a site announces (HTML `<link>`, manifest, well-known paths, social cards) plus the dashboard-icons pack, each downloaded and scored by its real format and dimensions, instead of a fixed order of lookups. The site HTML is fetched once per crawl.
  - Fixed icons which the crawling found but the browser never showed: the dashboard now loads every icon through CasaVue (`/icons/`) instead of from the application itself. Zot answers `Cross-Origin-Resource-Policy: same-origin` and the qBittorrent WebUI answers 401 to any request with the `Referer` of another site, so their pictures were refused in the browser; icons of `http://` items are no longer dropped as mixed content by an `https://` dashboard.
  - A vector icon is preferred over the raster icons of a page even when its file name does not end in `.svg` and only the `type` attribute of the link or of the manifest entry says it is one.
  - Icon size attributes like `32x32` are read as 32 instead of 3232, so the largest icon of a site is picked correctly.
  - Customization values containing `$` (title, app name) no longer break the generated `index.html`.
  - `customization.logo` and `customization.logo_small` in `main.yaml` replace the CasaVue logos with own SVGs (base64 encoded, e.g. `base64 -w0 logo.svg`); they are the source of the favicon, the mask icon and all the PWA icons. Overriding only `logo` generates the small logo out of it, overriding only `logo_small` changes the favicons alone. A value which is not a base64 encoded SVG refuses the whole configuration file. The color `#000000` of the SVGs (also of the CasaVue ones) is repainted with the theme contrast color, as `#e0f3ff` was before. An override is drawn 30% smaller than its own `viewBox` before the icons are generated out of it, since an own SVG usually fills the canvas edge to edge; an override without a `viewBox` keeps its size. Overriding `logo` also replaces the logo in the header of the dashboard, and taking the override away draws the CasaVue logo there again.
  - Generated avatars are cached by the browser for a day instead of being generated again on every view.
  - Configuration is reloaded when `main.yaml` or `items.yaml` change, so a new ConfigMap applied to Kubernetes is used without restarting CasaVue: static items are added, changed and removed, the content filters are re-applied to the already known Ingresses and HTTPRoutes, and a changed name or theme color is regenerated into the served files. A `main.yaml` CasaVue could not work with (unparsable YAML, unknown filter mode, invalid log level, invalid theme color) is refused with an error and the configuration in use is kept.
  - A read-only configuration volume (a mounted ConfigMap) no longer ends the process: a configuration file it does not provide is replaced by the defaults built into the binary.
  - Removed unused dependency `PerformLine/go-stockutil`; 6 modules less in `go.mod` and in image scans.
  - Removed dependency `go.deanishe.net/favicon`; 5 modules less in `go.mod` and in image scans.
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
