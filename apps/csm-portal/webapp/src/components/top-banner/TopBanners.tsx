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

import DOMPurify from "dompurify";
import { useEffect, useMemo, useState, type JSX } from "react";
import { getTopBanners, type TopBannerItem } from "@config/topBannersConfig";
import { useLogger } from "@hooks/useLogger";

const FALLBACK_STORAGE_KEY = "top_banner_fallback_v1";

// Dedicated instance so the hook and the allowed `target` attribute do not
// leak into other DOMPurify usage in the app. Default DOMPurify strips
// `target`, which would turn banner links meant to open in a new tab into
// same-tab navigations; keep it and force a safe `rel`.
const purifier = DOMPurify(window);
purifier.addHook("afterSanitizeAttributes", (node) => {
  if (node.tagName === "A" && node.getAttribute("target") === "_blank") {
    node.setAttribute("rel", "noopener noreferrer");
  }
});

function sanitizeBannerHtml(html: string): string {
  return purifier.sanitize(html, { ADD_ATTR: ["target"] });
}

function isDismissed(storageKey: string): boolean {
  try {
    return localStorage.getItem(storageKey) === "dismissed";
  } catch {
    return false;
  }
}

function persistDismissal(storageKey: string): void {
  try {
    localStorage.setItem(storageKey, "dismissed");
  } catch {
    // ignore storage errors
  }
}

function Banner({ banner }: { banner: TopBannerItem }): JSX.Element | null {
  const { closeable } = banner;
  const resolvedStorageKey = banner.storageKey || FALLBACK_STORAGE_KEY;
  const logger = useLogger();
  const [closed, setClosed] = useState(() =>
    closeable ? isDismissed(resolvedStorageKey) : false,
  );
  const sanitizedHtml = useMemo(() => sanitizeBannerHtml(banner.html), [banner.html]);

  useEffect(() => {
    if (closeable && !banner.storageKey) {
      logger.warn(
        "A top banner has closeable: true but no storageKey set. " +
          "A fallback key is being used; dismiss state may persist incorrectly.",
      );
    }
  }, [closeable, banner.storageKey, logger]);

  if (closed || !sanitizedHtml) return null;

  const handleClose = (): void => {
    persistDismissal(resolvedStorageKey);
    setClosed(true);
  };

  return (
    <div style={{ position: "relative", overflow: "hidden" }}>
      <div
        style={{ display: "block", lineHeight: 0, fontSize: 0, overflow: "hidden" }}
        dangerouslySetInnerHTML={{ __html: sanitizedHtml }}
      />
      {closeable && (
        <div
          style={{
            position: "absolute",
            inset: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "flex-end",
            paddingRight: "12px",
            pointerEvents: "none",
          }}
        >
          <button
            type="button"
            onClick={handleClose}
            aria-label="Close banner"
            style={{
              pointerEvents: "auto",
              background: "rgba(0,0,0,.55)",
              border: "1px solid rgba(255,255,255,.6)",
              color: "#fff",
              width: "24px",
              height: "24px",
              fontSize: "14px",
              lineHeight: "1",
              cursor: "pointer",
              borderRadius: "4px",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
            }}
          >
            &times;
          </button>
        </div>
      )}
    </div>
  );
}

/**
 * Renders all enabled top banners (CSM_PORTAL_TOP_BANNERS, plus the legacy
 * CSM_PORTAL_TOP_BANNER_* keys) top-to-bottom in order. Each banner tracks its
 * own dismiss state via its storageKey. Banner HTML is sanitized.
 */
export default function TopBanners(): JSX.Element | null {
  const banners = getTopBanners();

  if (banners.length === 0) return null;

  return (
    <>
      {banners.map((banner, index) => (
        <Banner key={`${index}:${banner.storageKey}`} banner={banner} />
      ))}
    </>
  );
}
