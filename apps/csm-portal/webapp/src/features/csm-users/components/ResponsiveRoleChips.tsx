// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
// Licensed under the Apache License, Version 2.0.

import { Box, Chip, Stack } from "@wso2/oxygen-ui";
import {
  useCallback,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
} from "react";
import { INTERNAL_USER_ROLES } from "@features/csm-users/types/csmUsers";

const ROLE_CATALOGUE_ALIASES: Record<string, string> = {
  "sn_customerservice.admin": "admin",
  "wso2_agent": "agent",
  "sn_customerservice.commenter": "commenter",
  "sn_customerservice.customer": "customer",
  "sn_customerservice.customer_admin": "customer_admin",
  "snc_external": "external",
  "snc_internal": "internal",
  "sn_customerservice.partner": "partner",
  "sn_customerservice.partner_admin": "partner_admin",
  "sn_customerservice.timecard_approver": "timecard_approver",
};

function roleDisplayName(role: string, roleNameById: Map<string, string>): string {
  const catalogueKey = ROLE_CATALOGUE_ALIASES[role] ?? role;
  const catalogueName = roleNameById.get(role) ?? roleNameById.get(catalogueKey);
  if (catalogueName) return catalogueName;

  const readableKey = catalogueKey.includes(".")
    ? catalogueKey.slice(catalogueKey.lastIndexOf(".") + 1)
    : catalogueKey;
  return readableKey
    .split("_")
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

interface ResponsiveRoleChipsProps {
  roleIds: string[];
  roleNameById: Map<string, string>;
  userLabel: string;
  onViewAll: () => void;
}

/** One-line, width-aware role summary shared by every user directory table. */
export default function ResponsiveRoleChips({
  roleIds,
  roleNameById,
  userLabel,
  onViewAll,
}: ResponsiveRoleChipsProps): JSX.Element {
  const containerRef = useRef<HTMLDivElement>(null);
  const chipMeasureRefs = useRef<Array<HTMLDivElement | null>>([]);
  const moreMeasureRef = useRef<HTMLDivElement>(null);
  const [visibleCount, setVisibleCount] = useState(Math.min(roleIds.length, 1));
  const labels = useMemo(
    () => roleIds.map((role) => roleDisplayName(role, roleNameById)),
    [roleIds, roleNameById],
  );

  const calculateVisibleCount = useCallback((): void => {
    const availableWidth = containerRef.current?.clientWidth ?? 0;
    if (availableWidth <= 0) return;
    const chipWidths = roleIds.map(
      (_, index) => chipMeasureRefs.current[index]?.offsetWidth ?? 0,
    );
    const moreWidth = moreMeasureRef.current?.offsetWidth ?? 0;
    const gap = 4;
    const allRolesWidth = chipWidths.reduce(
      (total, width, index) => total + width + (index === 0 ? 0 : gap),
      0,
    );
    if (allRolesWidth <= availableWidth) {
      setVisibleCount(chipWidths.length);
      return;
    }

    let usedWidth = 0;
    let nextVisibleCount = 0;
    for (let i = 0; i < chipWidths.length; i += 1) {
      const widthBeforeRole = i === 0 ? 0 : gap;
      if (usedWidth + widthBeforeRole + chipWidths[i] + gap + moreWidth > availableWidth) break;
      usedWidth += widthBeforeRole + chipWidths[i];
      nextVisibleCount += 1;
    }
    setVisibleCount(nextVisibleCount);
  }, [roleIds]);

  useLayoutEffect(() => {
    calculateVisibleCount();
    const container = containerRef.current;
    if (!container || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(calculateVisibleCount);
    observer.observe(container);
    return () => observer.disconnect();
  }, [calculateVisibleCount, labels]);

  const hiddenRoleCount = roleIds.length - visibleCount;
  return (
    <Box ref={containerRef} sx={{ position: "relative", width: "100%", minWidth: 0 }}>
      <Stack direction="row" spacing={0.5} sx={{ flexWrap: "nowrap", overflow: "hidden" }}>
        {roleIds.slice(0, visibleCount).map((role, index) => (
          <Chip
            key={role}
            size="small"
            label={labels[index]}
            variant="outlined"
            color={(INTERNAL_USER_ROLES as string[]).includes(role) ? "primary" : "default"}
            sx={{ flexShrink: 0 }}
          />
        ))}
        {hiddenRoleCount > 0 && (
          <Chip
            size="small"
            variant="outlined"
            label={`+${hiddenRoleCount} more`}
            clickable
            sx={{ flexShrink: 0 }}
            onClick={(e) => {
              e.stopPropagation();
              onViewAll();
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") e.stopPropagation();
            }}
            aria-label={`View all ${roleIds.length} roles for ${userLabel}`}
          />
        )}
      </Stack>
      <Stack
        aria-hidden
        direction="row"
        spacing={0.5}
        sx={{ position: "absolute", visibility: "hidden", pointerEvents: "none" }}
      >
        {labels.map((label, index) => (
          <Chip
            key={`${roleIds[index]}-measure`}
            ref={(element) => {
              chipMeasureRefs.current[index] = element;
            }}
            size="small"
            variant="outlined"
            label={label}
          />
        ))}
        <Chip ref={moreMeasureRef} size="small" variant="outlined" label={`+${roleIds.length} more`} />
      </Stack>
    </Box>
  );
}
