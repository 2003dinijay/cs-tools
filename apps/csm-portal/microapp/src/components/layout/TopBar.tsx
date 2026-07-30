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

import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { Box, IconButton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Grip } from "@wso2/oxygen-ui-icons-react";
import { goToMyAppsScreen } from "@components/microapp-bridge";
import { ConfirmDialog } from "@components/common/ConfirmDialog";

const ROOT_PATHS = ["/", "/support", "/operations", "/more"];

export function TopBar() {
  const location = useLocation();
  const navigate = useNavigate();
  const isRootPath = ROOT_PATHS.includes(location.pathname);

  return (
    <Box
      position="sticky"
      top={0}
      display="flex"
      alignItems="center"
      px={2}
      pb={2}
      sx={{
        // Ties directly to the device's actual safe-area inset (--safe-top, set from the native
        // bridge) plus just enough clearance for the ExitButton pill's own height (~28px: a 20px
        // icon plus a body2 label, p:0), rather than a fixed value — a fixed pt (the
        // customer-portal microapp's own AppBar.tsx uses one, pt:7/56px) is only correct for
        // whatever inset it was tuned against; on a Dynamic Island phone (~59px inset) a flat 56px
        // undershoots the inset itself and the pill overflows into the page content below it. On
        // root pages, this Box's flowed content row is otherwise empty (ExitButton is absolutely
        // positioned, not part of the flow), so this pt is effectively this bar's entire visible
        // height — keep the buffer tight rather than a larger round number, or the bar reads as a
        // lot of empty space above the pill.
        pt: "calc(var(--safe-top, 44px) + 28px)",
        // Solid, not the theme's `background.paper` token — see TabBar.tsx's identical comment,
        // including why this reads `theme.vars.palette.*` rather than `theme.palette.mode`.
        backgroundColor: (theme) => theme.vars?.palette.background.default ?? theme.palette.background.default,
        borderBottom: "1px solid",
        borderColor: "divider",
        zIndex: (theme) => theme.zIndex.appBar,
      }}
    >
      <Box sx={{ minWidth: 32 }}>
        {isRootPath ? (
          <ExitButton />
        ) : (
          <IconButton onClick={() => navigate(-1)} aria-label="Go back" size="small">
            <ArrowLeft size={22} />
          </IconButton>
        )}
      </Box>
    </Box>
  );
}

// Leaves the microapp WebView and returns to the native shell's app switcher — mirrors the
// customer-portal microapp's own ExitButton (components/core/AppBar.tsx), shown on root pages in
// place of the back button. Floats in the safe-area strip above the main row (same technique the
// customer-portal version uses: absolutely positioned at `var(--safe-top)`), so it doesn't compete
// with page content for width on narrow phones.
function ExitButton() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <IconButton
        disableRipple
        color="error"
        sx={{
          gap: 1,
          position: "absolute",
          top: "var(--safe-top, 44px)",
          left: 10,
          p: 0,
        }}
        onClick={() => setOpen(true)}
      >
        <Grip size={20} />
        <Typography variant="body2" fontWeight={600}>
          Go to Apps
        </Typography>
      </IconButton>

      <ConfirmDialog
        open={open}
        title="Return to Apps"
        description="Are you sure you want to leave this application?"
        confirmColor="error"
        confirmLabel="Leave"
        onClose={() => setOpen(false)}
        onConfirm={goToMyAppsScreen}
      />
    </>
  );
}
