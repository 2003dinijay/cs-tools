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

import { useMemo, useState } from "react";
import { useAsgardeo as useSdkAsgardeo } from "@asgardeo/react";

/**
 * Drop-in replacement for the SDK's `useAsgardeo` whose `isLoading` stays
 * `false` once the session has been ready, for as long as the user remains
 * signed in.
 *
 * Why: the Asgardeo provider mirrors its client's busy flag into
 * `isLoading` (polled every 100 ms), and that flag goes true whenever the
 * client touches the token, including every `getIdToken()` an API call makes
 * and the periodic background refresh. Nearly every data hook gates on
 * `isSignedIn && !isLoading` with `staleTime: 0`, so each of those blips
 * switched every active query off and on again, and React Query refetches stale
 * data when a query is re-enabled. The refetch calls `getIdToken()`, which
 * can raise the flag again: an open page then repeated its user, project and
 * list requests about once a second, indefinitely.
 *
 * Only the first load needs the gate (nothing can be fetched before the
 * session exists); after that a token refresh in the background is not a
 * reason to throw away a query's enabled state. Signing out clears it, so the
 * next sign-in waits for the SDK again. Everything else the SDK returns is
 * passed through unchanged.
 *
 * Import this instead of `@asgardeo/react`'s hook for anything that reads
 * `isLoading` to gate a request or a render.
 *
 * @returns {ReturnType<typeof useSdkAsgardeo>} The SDK's auth context with a
 * latched `isLoading`.
 */
export function useAsgardeo(): ReturnType<typeof useSdkAsgardeo> {
  const auth = useSdkAsgardeo();
  const [hasBeenReady, setHasBeenReady] = useState(false);

  // Derived state, adjusted while rendering (React's documented alternative to
  // an effect for this): set once the session is up, cleared on sign-out. Each
  // branch only fires when it changes the value, so it settles in one extra
  // render.
  if (!auth.isSignedIn && hasBeenReady) {
    setHasBeenReady(false);
  } else if (auth.isSignedIn && !auth.isLoading && !hasBeenReady) {
    setHasBeenReady(true);
  }

  const hideLoading = hasBeenReady && auth.isSignedIn && auth.isLoading;

  return useMemo(
    () => (hideLoading ? { ...auth, isLoading: false } : auth),
    [auth, hideLoading],
  );
}
