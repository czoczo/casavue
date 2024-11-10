#!/bin/bash
#
# Script updates static DB JSON file. This file is usef, for Vue development without running Golang backend.
# Nevertheless, Golang backend must be running when using the script for scrape to happen.

echo '{"apiv1":' $(curl -s http://127.0.0.1:8080/api/v1) '}' > db.json
