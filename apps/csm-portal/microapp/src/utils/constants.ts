// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

export const ErrorMessages = {
  NATIVE_BRIDGE_NOT_AVAILABLE: "Native bridge is not available",
};

// Same superapp webview also shares its storage partition across dev/staging/prod
// builds of this microapp (e.g. a device that had the staging build installed, then
// gets the prod build). VITE_APP_ENV must be set distinctly per build so a token
// cached under one environment is never reused by another — parsing the env out of
// VITE_BACKEND_URL instead isn't safe, since Choreo's own proxy domain contains
// "prod." even for staging URLs. Without this, refreshToken()'s isTokenExpiringSoon()
// short-circuit could reuse a still-unexpired token issued by a different
// environment's IdP, which only surfaced as "the app doesn't work" until the
// superapp was uninstalled and reinstalled to clear the stale cached value.
// Falls back to "prod" (rather than throwing) when unset, since prod is the default
// build target and this must not hard-fail a build that hasn't been updated yet.
const APP_ENV = import.meta.env.VITE_APP_ENV || "prod";

// Prefixed per-app so a token cached here can never be picked up by a sibling
// microapp (e.g. customer-portal) sharing the same storage partition. Each
// microapp is registered as its own OAuth client in the superapp, so the two
// apps' tokens carry different audiences — they were never interchangeable.
// Before this, both apps used the bare "accessToken"/"idToken" keys, so
// refreshToken()'s isTokenExpiringSoon() short-circuit (added in 1644c9cf3)
// could reuse whatever token was cached under this key regardless of which
// app it was actually issued for, causing 403s against the CSM backend after
// visiting Customer Portal first (confirmed live).
export const LocalStorageKeys = {
  accessToken: `csm_portal_accessToken_${APP_ENV}`,
  idToken: `csm_portal_idToken_${APP_ENV}`,
};
