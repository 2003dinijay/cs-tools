#!/usr/bin/env sh
# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

# Builds the CSM portal micro app and packages it as a Super App bundle.
#
# The Super App unzips the bundle and expects index.html and microapp.json at
# the zip root (or under build/), so the contents of dist/ are zipped rather
# than the dist/ folder itself. microapp.json carries the OAuth client the
# Super App exchanges its token for; it is generated here so client IDs stay
# out of git and can differ per environment.
#
# Required:  MICROAPP_CLIENT_ID      OAuth client ID with the token-exchange grant
#            VITE_BACKEND_URL        set in the environment or in .env / .env.production
# Optional:  MICROAPP_DISPLAY_MODE   "fullscreen" (default) or "default"
#            MICROAPP_ZIP_NAME       output file name (default csm-portal-microapp.zip)

set -eu

cd "$(dirname "$0")/.."

if [ -z "${MICROAPP_CLIENT_ID:-}" ]; then
  echo "MICROAPP_CLIENT_ID is not set. The Super App cannot exchange a token without it." >&2
  exit 1
fi

if [ -z "${VITE_BACKEND_URL:-}" ] && ! grep -qs '^VITE_BACKEND_URL=.' .env .env.production; then
  echo "VITE_BACKEND_URL is not set in the environment, .env or .env.production." >&2
  exit 1
fi

DISPLAY_MODE="${MICROAPP_DISPLAY_MODE:-fullscreen}"
ZIP_NAME="${MICROAPP_ZIP_NAME:-csm-portal-microapp.zip}"

npm run build

cat > dist/microapp.json <<EOF
{
  "clientId": "${MICROAPP_CLIENT_ID}",
  "displayMode": "${DISPLAY_MODE}"
}
EOF

rm -f "$ZIP_NAME"
(cd dist && zip -qr "../$ZIP_NAME" . -x '.*' -x '__MACOSX/*')

echo "Packaged $(pwd)/$ZIP_NAME"
