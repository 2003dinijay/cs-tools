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

import { useLayoutEffect, useRef } from "react";
import { Link, useLocation } from "react-router-dom";
import { BottomNavigation, BottomNavigationAction, Box } from "@wso2/oxygen-ui";
import { Cog, Headset, House, LayoutGrid } from "@wso2/oxygen-ui-icons-react";

// Mirrors a subset of the webapp's CSM_NAV_ITEMS (apps/csm-portal/webapp/src/config/csmNavItems.ts):
// Home (Dashboard), Support (Cases), Operations get their own tab; Time Cards/Security
// Center/Updates/Engagements/Settings/Profile are grouped under More; Customers isn't in the
// mobile nav.
function activeTabFor(pathname: string): string | false {
  if (pathname === "/") return "home";
  if (pathname.startsWith("/support") || pathname.startsWith("/cases/")) return "support";
  if (pathname.startsWith("/operations")) return "operations";
  if (pathname.startsWith("/more") || pathname.startsWith("/profile")) return "more";
  return false;
}

export function TabBar() {
  const ref = useRef<HTMLDivElement>(null);
  const location = useLocation();
  const activeTab = activeTabFor(location.pathname);

  useLayoutEffect(() => {
    if (!ref.current) return;

    const observer = new ResizeObserver(([entry]) => {
      document.documentElement.style.setProperty("--tab-bar-height", `${entry.contentRect.height}px`);
    });

    observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);

  return (
    <Box
      ref={ref}
      position="fixed"
      bottom={0}
      left={0}
      right={0}
      pt={1}
      // Fixed pb (not derived from --safe-bottom), matching the customer-portal microapp's own
      // TabBar (apps/customer-portal/microapp/src/components/core/TabBar.tsx) for the same
      // reason as TopBar.tsx's fixed pt.
      pb={4}
      sx={{
        // Solid, not the theme's `background.paper` token — that token is deliberately
        // semi-transparent in the shared Acrylic theme (#ffffffe1 / #000000b8, for glass-style
        // Paper/Card surfaces), which reads as visibly translucent on a fixed bar with page
        // content scrolling underneath it. Matches the customer-portal microapp's own TabBar,
        // which hardcodes solid black/white for the same reason. Reads `theme.vars.palette.*`
        // (a CSS var, e.g. var(--oxygen-palette-background-default)), NOT `theme.palette.mode` —
        // this theme switches light/dark purely via CSS variables reacting to the OS color scheme,
        // so `theme.palette.mode` is a static JS value that never actually flips at runtime; using
        // it here left this bar always white regardless of dark mode.
        // No border — matches the customer-portal microapp's own TabBar exactly, which has no
        // sx at all. A divider line here reads as a visible seam separating the bar from the
        // page content above it, rather than the two reading as one continuous surface.
        backgroundColor: (theme) => theme.vars?.palette.background.default ?? theme.palette.background.default,
      }}
    >
      <BottomNavigation value={activeTab} showLabels>
        <BottomNavigationAction component={Link} to="/" value="home" label="Home" icon={<House />} disableRipple />
        <BottomNavigationAction
          component={Link}
          to="/support"
          value="support"
          label="Support"
          icon={<Headset />}
          disableRipple
        />
        <BottomNavigationAction
          component={Link}
          to="/operations"
          value="operations"
          label="Operations"
          icon={<Cog />}
          disableRipple
        />
        <BottomNavigationAction
          component={Link}
          to="/more"
          value="more"
          label="More"
          icon={<LayoutGrid />}
          disableRipple
        />
      </BottomNavigation>
    </Box>
  );
}
