# CasaVue Changelog

[ Application Changelog ] | [ [Helm Chart Changelog](./deployment/helm/CHANGELOG.md) ]

## 0.1.4
- App:
  - Fixed process crash when the dashboard API read items while they were updated.
  - Fixed crash on Ingress without rules and on delete events after a watch reconnect.
  - Fixed items coming back after delete or rename when a late icon crawl wrote them back.
  - Items are keyed by namespace and object name, so the same name in two namespaces no longer overwrites itself.
  - Icon crawls are limited to 8 at a time and to 2 MB per page, so a big site cannot exhaust container memory.
  - Dashboard items are no longer re-crawled on Kubernetes watch events which change nothing on the dashboard (e.g. status only updates).
  - HTTPRoute links are `https://` when a listener of the Gateway they attach to terminates TLS for their hostname, like an Ingress with a `tls` section.
  - Fixed static mode (`-static`) crashing when the avatars folder does not exist.
  - Fixed static mode saving an error page or a truncated file as an icon; the original icon URL is kept when the download fails.
  - Fixed `casavue.app/enable=false` not hiding an item in `ingressAnnotation` mode.
  - Fixed icons which the crawling found but the browser never showed: the dashboard now loads every icon through CasaVue (`/icons/`) instead of from the application itself. Icons of `http://` items are no longer dropped as mixed content by an `https://` dashboard.
  - A vector icon is preferred over the raster icons of a page even when its file name does not end in `.svg` and only the `type` attribute of the link or of the manifest entry says it is one.
  - `customization.logo` in `main.yaml` replaces the CasaVue logo with an own SVG (base64 encoded, e.g. `base64 -w0 logo.svg`); it is the source of the favicon, the mask icon and all the PWA icons.
  - Generated avatars are cached by the browser for a day instead of being generated again on every view.
  - Configuration is reloaded when `main.yaml` or `items.yaml` change, so a new ConfigMap applied to Kubernetes is used without restarting CasaVue.
  - Newlines and spaces are removed from the `content_filters` patterns when `main.yaml` is read, so a long regular expression can be written over several lines for readability.
  - An item answering `403 Forbidden` is reported as up, like the ones answering 200 or 401.
  - `/statusCheck/` answers `424 Failed Dependency` when an HTTPS item presents a TLS certificate which cannot be trusted (unknown issuer, expired, for another host).
- WebUI:
  - Fixed dashboard breaking after an item was removed while its status was being checked.
  - Status of removed items is no longer kept and re-checked.
  - An HTTPS item with an untrusted certificate shows `Invalid TLS certificate` instead of `Site unavailable`.

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
