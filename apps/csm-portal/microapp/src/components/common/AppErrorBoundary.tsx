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

import { Component, type ErrorInfo, type ReactNode } from "react";
import { Box, Button, Stack, Typography } from "@wso2/oxygen-ui";
import { TriangleAlert } from "@wso2/oxygen-ui-icons-react";
import { Logger } from "@utils/logger";

interface AppErrorBoundaryProps {
  children: ReactNode;
}

interface AppErrorBoundaryState {
  hasError: boolean;
}

/**
 * Last-resort boundary at the app root. Every other ErrorBoundary in this app is scoped to a
 * single tab/section (AttachmentsTab, CaseActivityFeed, etc.) — but most pages (HomePage,
 * AnnouncementsPage, MorePage, ...) and shared chrome (MainLayout, TopBar) have no boundary of
 * their own. Without this one, an uncaught render error in any of those unmounts the *entire*
 * React tree with nothing left to catch it — the WebView goes to a blank white screen with no
 * way back short of force-quitting. That's not hypothetical: the createdBy UserReference bug
 * that crashed the Announcements page this session threw at render time, inside a page with no
 * boundary of its own and, at the time, no root boundary either — the "blank page" bug report was
 * this exact failure mode. Mirrors the webapp's AppErrorBoundary, trimmed to this app's simpler
 * recovery options (a WebView has no address bar to retype a URL into, so "reload" is the whole
 * story).
 */
export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    Logger.error("Unhandled render error crashed the app", {
      message: error.message,
      componentStack: info.componentStack,
    });
  }

  private handleReload = (): void => {
    window.location.reload();
  };

  render(): ReactNode {
    if (!this.state.hasError) return this.props.children;

    return (
      <Box sx={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", px: 3 }}>
        <Stack alignItems="center" gap={1.5} sx={{ maxWidth: 320, textAlign: "center" }}>
          <TriangleAlert size={32} color="currentColor" />
          <Typography variant="h6">Something went wrong</Typography>
          <Typography variant="body2" color="text.secondary">
            Please try reloading. If this keeps happening, let support know.
          </Typography>
          <Button variant="contained" onClick={this.handleReload} sx={{ mt: 1 }}>
            Reload
          </Button>
        </Stack>
      </Box>
    );
  }
}
