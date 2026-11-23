# CasaVue Helm Chart Changelog

[ [Application Changelog](../../CHANGELOG.md) ] | [ Helm Chart Changelog ]

## 0.1.5
- Both configuration ConfigMaps are mounted as one directory (`projected` volume) instead of two single file `subPath` mounts. A `subPath` mount is never updated by the kubelet, so CasaVue could never notice a `helm upgrade`; with the directory mount its configuration reload picks the new ConfigMap up without restarting the pod.
- Role and RoleBinding for reading ConfigMaps in the namespace of the release: CasaVue watches `casavue-cfg-main` and `casavue-cfg-items` over the API, so a `helm upgrade` of the configuration is applied within seconds instead of after the kubelet volume sync of up to a minute. The mount stays as it is, it is what CasaVue runs on where the ConfigMaps cannot be read. The rules are not added to the existing ClusterRole: that would allow reading every ConfigMap of the cluster.
- App version bump to 0.1.4.
- Pod-level `securityContext` set to `runAsNonRoot: true` instead of the empty `{}` default: the kubelet now refuses to start the pod as root, and the chart passes the `HIGH` gate of the Trivy config scan in CI.

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
