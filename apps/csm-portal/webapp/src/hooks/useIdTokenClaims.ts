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

import { useEffect, useState } from "react";
import { useAsgardeo } from "@asgardeo/react";
import type { IdTokenClaims } from "@utils/userClaims";

// Decodes and caches the ID token payload once the user is signed in. The IdP
// SDK exposes a `user` field on the auth context, but only when
// `preferences.user.fetchUserProfile` is true, which triggers an additional
// SCIM call we don't need. The ID token is already in storage; just decode it.
export function useIdTokenClaims(): IdTokenClaims | undefined {
  const { getDecodedIdToken, isSignedIn } = useAsgardeo();
  const [claims, setClaims] = useState<IdTokenClaims | undefined>(undefined);

  useEffect(() => {
    if (!isSignedIn) {
      setClaims(undefined);
      return;
    }

    let cancelled = false;
    void (async () => {
      try {
        const decoded = (await getDecodedIdToken()) as unknown as IdTokenClaims;
        if (!cancelled) setClaims(decoded);
      } catch {
        if (!cancelled) setClaims(undefined);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isSignedIn, getDecodedIdToken]);

  return claims;
}
