#!/bin/bash

# This script updates deployment/helm/Chart.yaml file, so it
# reflects version values from VERSION_APP.txt and VERSION_CHART.txt

CHART_FILE_PATH="deployment/helm/Chart.yaml"

APP_VERSION="$(cat VERSION_APP.txt | tr -dc '[0-9].')"
CHART_VERSION="$(cat VERSION_CHART.txt | tr -dc '[0-9].')"

CHART="$(awk \
  -v appVer=$APP_VERSION \
  -v chartVer=$CHART_VERSION \
  '/^version:/ { print "version: " chartVer; next; }
   /^appVersion:/  { print "appVersion: " appVer; next; }
   { print; }' \
${CHART_FILE_PATH})"

echo "$CHART" > "$CHART_FILE_PATH"

cat <<EOF
Chart file $CHART_FILE_PATH was updated with following values:
 - appVersion:	$APP_VERSION
 - version:	$CHART_VERSION
EOF
