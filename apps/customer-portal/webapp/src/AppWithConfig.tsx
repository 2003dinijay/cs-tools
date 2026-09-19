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

import { type JSX } from "react";
import { BrowserRouter } from "react-router";
import { GlobalStyles, OxygenUIThemeProvider } from "@wso2/oxygen-ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import App from "./App";
import { AsgardeoProvider } from "@asgardeo/react";
import { themeConfig } from "@config/themeConfig";
import { loggerConfig } from "@config/loggerConfig";
import LoggerProvider from "@context/logger/LoggerProvider";
import { FontSizeProvider } from "@context/font-size/FontSizeContext";
import MobileAppGate from "@providers/MobileAppGate";
import { authConfig } from "@config/authConfig";

/**
 * Custom retry function for React Query.
 * Only retries on 502 (Bad Gateway) and 503 (Service Unavailable) errors.
 *
 * @param {number} failureCount - The number of times the request has failed.
 * @param {Error} error - The error that occurred.
 * @returns {boolean} True if the request should be retried, false otherwise.
 */
function shouldRetry(failureCount: number, error: Error): boolean {
  // Max 3 retries
  if (failureCount >= 2) {
    return false;
  }

  // Check if error has a status code property
  const errorWithStatus = error as Error & {
    response?: { status?: number };
    status?: number;
  };
  const statusCode = errorWithStatus.response?.status || errorWithStatus.status;

  // Only retry on 502 (Bad Gateway) and 503 (Service Unavailable)
  return statusCode === 502 || statusCode === 503;
}

const queryClient: QueryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: shouldRetry,
      retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30000),
      refetchOnWindowFocus: false,
      refetchOnReconnect: false,
      refetchOnMount: true,
    },
    mutations: {
      retry: shouldRetry,
      retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30000),
    },
  },
});

export default function AppWithConfig(): JSX.Element {
  return (
    <AsgardeoProvider
      baseUrl={authConfig.baseUrl}
      clientId={authConfig.clientId}
      afterSignInUrl={authConfig.signInRedirectURL}
      afterSignOutUrl={authConfig.signOutRedirectURL}
      // eslint-disable-next-line @typescript-eslint/ban-ts-comment
      // @ts-ignore
      periodicTokenRefresh
      // eslint-disable-next-line @typescript-eslint/ban-ts-comment
      // @ts-ignore -- `issuer` isn't in @asgardeo/react's typed `endpoints`
      // config (only authorization/endSession/introspection/jwks/token/
      // userInfo/wellKnown are), but the SDK's own fallback endpoint
      // resolution (`resolveEndpointsByBaseURL` in @asgardeo/javascript)
      // merges `endpoints` via `Object.keys(...)` with no allowlist, so this
      // key IS honored at runtime. Without it, the SDK's only default for the
      // ID token's expected issuer is `${baseUrl}/oauth2/token` (the classic
      // Asgardeo/WSO2 Identity Server convention) -- it only trusts the real
      // value from the OIDC discovery document when `platform` is explicitly
      // "AsgardeoV2", which this app deliberately does not set (that also
      // swaps `signOut()`/`signIn()`/`signUp()` to the embedded-flow "Thunder"
      // behavior this app doesn't use). Any spec-compliant OIDC provider whose
      // issuer is the bare `baseUrl` -- RFC 8414's own convention, and what
      // this app's IdP actually issues -- mismatches that hardcoded default
      // and fails ID-token validation with SPA-CRYPTO-UTILS-VJ-IV01 /
      // ERR_JWT_CLAIM_VALIDATION_FAILED ("iss").
      endpoints={{ issuer: authConfig.baseUrl }}
      scopes={["openid", "email", "groups", "profile"]}
      preferences={{
        theme: {
          inheritFromBranding: false,
        },
        user: {
          fetchUserProfile: false,
          fetchOrganizations: false,
        },
      }}
    >
      <BrowserRouter>
        <LoggerProvider config={loggerConfig}>
          <OxygenUIThemeProvider theme={themeConfig}>
            <GlobalStyles
              styles={{
                "html, body, #root": {
                  width: "100%",
                  maxWidth: "100vw",
                  overflowX: "clip",
                },
              }}
            />
            <FontSizeProvider>
              <QueryClientProvider client={queryClient}>
                <MobileAppGate>
                  <App />
                </MobileAppGate>
                <ReactQueryDevtools initialIsOpen={false} />
              </QueryClientProvider>
            </FontSizeProvider>
          </OxygenUIThemeProvider>
        </LoggerProvider>
      </BrowserRouter>
    </AsgardeoProvider>
  );
}
