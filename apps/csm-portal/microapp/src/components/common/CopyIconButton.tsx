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

import { useEffect, useRef, useState } from "react";
import { IconButton, Tooltip, pxToRem } from "@wso2/oxygen-ui";
import { Check, Copy } from "@wso2/oxygen-ui-icons-react";

// Mirrors the customer-portal microapp's OverlineSlot copy affordance
// (components/features/detail/OverlineSlot.tsx): tap copies `value`, the icon swaps to a
// checkmark and the tooltip reads "Copied!" for 2s, then reverts.
export function CopyIconButton({ value, "aria-label": ariaLabel }: { value: string; "aria-label": string }) {
  const [copied, setCopied] = useState(false);
  const [hovered, setHovered] = useState(false);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    return () => {
      if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    };
  }, []);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
      copyTimeoutRef.current = setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access denied or unavailable — nothing actionable to do here.
    }
  };

  return (
    <Tooltip
      title={copied ? "Copied!" : "Copy"}
      open={copied || hovered}
      onOpen={() => setHovered(true)}
      onClose={() => setHovered(false)}
    >
      <IconButton size="small" color={copied ? "success" : "default"} onClick={handleCopy} aria-label={ariaLabel}>
        {copied ? <Check size={pxToRem(16)} /> : <Copy size={pxToRem(16)} />}
      </IconButton>
    </Tooltip>
  );
}
