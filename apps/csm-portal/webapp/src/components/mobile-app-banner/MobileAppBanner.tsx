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

import {
  Alert,
  AlertTitle,
  Box,
  Button,
  Collapse,
  IconButton,
  Stack,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX } from "react";
import {
  getMobileAppConfig,
  getMobileAppStoreUrl,
} from "@config/mobileAppConfig";
import { MobileOs, type MobileDeviceInfo } from "@/types/mobileDevice";
import { detectMobileDevice } from "@utils/deviceDetection";

const OS_LABELS: Record<MobileOs, string> = {
  [MobileOs.Ios]: "iOS",
  [MobileOs.Android]: "Android",
};

/**
 * Validates a configured store URL and returns its normalized `http(s)` form,
 * or `undefined` when it's missing or unsupported (e.g. a `javascript:`/
 * `data:` URI, or a string that doesn't parse as a URL at all). Used both to
 * decide whether the banner should show at all and as the actual value
 * `window.open` navigates to -- a config value that fails this check must
 * never render a Download button that silently does nothing on click.
 */
function resolveDownloadUrl(storeUrl: string | undefined): string | undefined {
  if (!storeUrl) return undefined;
  try {
    const parsed = new URL(storeUrl, window.location.origin);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return undefined;
    return parsed.toString();
  } catch {
    return undefined;
  }
}

/**
 * MobileAppBanner component.
 *
 * Dismissible banner nudging CS engineers on a detected mobile phone (and
 * optionally tablet) browser toward the WSO2 Super App micro-app, without
 * blocking access -- engineers can dismiss it and keep using the web portal
 * on a phone, unlike the customer portal's full-page mobile gate.
 *
 * Built directly on `Alert` (not the higher-level `NotificationBanner`):
 * `NotificationBanner`/MUI `Alert` only auto-renders its own close icon when
 * no custom `action` node is supplied, so a banner that also needs a
 * "Download" action button must pack both into `action` itself -- see
 * `ErrorBanner.tsx` for the same pattern already established in this app.
 *
 * @returns {JSX.Element | null} The MobileAppBanner component.
 */
export default function MobileAppBanner(): JSX.Element | null {
  const mobileAppConfig = useMemo(() => getMobileAppConfig(), []);
  const device = useMemo<MobileDeviceInfo | null>(
    () =>
      detectMobileDevice({ includeTablets: mobileAppConfig.includeTablets }),
    [mobileAppConfig.includeTablets],
  );

  const storeUrl = device
    ? getMobileAppStoreUrl(device.os, mobileAppConfig)
    : undefined;
  const downloadUrl = resolveDownloadUrl(storeUrl);

  const visible = mobileAppConfig.enabled && device !== null && !!downloadUrl;

  // State for the banner dismissal.
  const [dismissed, setDismissed] = useState<boolean>(false);

  // Reset the dismissed state when the visibility configuration changes to true.
  useEffect(() => {
    if (visible) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- reset dismissal when banner is re-shown
      setDismissed(false);
    }
  }, [visible]);

  if (!visible || dismissed || !device || !downloadUrl) {
    return null;
  }

  const osLabel = OS_LABELS[device.os];

  const handleDownload = (): void => {
    window.open(downloadUrl, "_blank", "noopener,noreferrer");
  };

  return (
    <Collapse in>
      <Alert
        severity="info"
        variant="filled"
        action={
          <Stack direction="row" spacing={0.5} alignItems="center">
            <Button
              color="inherit"
              size="small"
              onClick={handleDownload}
              sx={{ fontWeight: 600, textDecoration: "underline" }}
            >
              Download
            </Button>
            <IconButton
              size="small"
              color="inherit"
              onClick={() => setDismissed(true)}
              aria-label="Close"
            >
              <X size={16} />
            </IconButton>
          </Stack>
        }
      >
        <AlertTitle sx={{ mb: 0 }}>Get the WSO2 Super App</AlertTitle>
        <Box component="span">
          {`This portal isn't optimized for ${osLabel} browsers. CSM Portal is also available as a micro-app inside the WSO2 Super App for a better mobile experience.`}
        </Box>
      </Alert>
    </Collapse>
  );
}
