# CasaVue Helm Chart Changelog

[ [Application Changelog](../../CHANGELOG.md) ] | [ Helm Chart Changelog ]

## 0.1.7
- Both configuration ConfigMaps are mounted as one directory (`projected` volume) instead of two single file `subPath` mounts. A `subPath` mount is never updated by the kubelet, so CasaVue could never notice a `helm upgrade`; with the directory mount its configuration reload picks the new ConfigMap up without restarting the pod.

## 0.1.4
- App version bump to 0.1.2.

## 0.1.3
- Fixed configuration volume mounts in deployment.
- Added tag overriding.
- Added IngressClassName handling.
- Improvements in Chart readability.
  
credits and thanks to [@spirilis](https://github.com/spirilis)

## 0.1.2
- Added tls example configuration in default `values.yaml` file.

## 0.1.1
- Fixed passing empty static items list array to CasaVue via ConfigMap.

## 0.1.0
- Initial release.
