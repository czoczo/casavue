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
- WebUI:
  - Fixed dashboard breaking after an item was removed while its status was being checked.
  - Status of removed items is no longer kept and re-checked.
- CI: Go vet and tests run before a container image is published.

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
