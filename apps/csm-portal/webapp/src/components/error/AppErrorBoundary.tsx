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
import { Box, Button, Stack, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Check, Copy } from "@wso2/oxygen-ui-icons-react";
import { getErrorReferenceId } from "@utils/correlationId";

interface AppErrorBoundaryProps {
  children: ReactNode;
}

interface AppErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
  componentStack: string | null;
  copied: boolean;
}

/** How long the "Copied!" confirmation stays visible after a successful copy. */
const COPY_CONFIRMATION_MS = 2000;

/**
 * Top-level error boundary so an uncaught render error degrades to a
 * recoverable "something went wrong" screen instead of unmounting the whole
 * app to a blank page. Class component by necessity: error boundaries have no
 * hook equivalent, which also means the context-based logger is unavailable
 * here — console is the only sink that cannot itself fail.
 */
export default class AppErrorBoundary extends Component<
  AppErrorBoundaryProps,
  AppErrorBoundaryState
> {
  state: AppErrorBoundaryState = {
    hasError: false,
    error: null,
    componentStack: null,
    copied: false,
  };

  private copyResetTimer: ReturnType<typeof setTimeout> | null = null;

  /** Flip to the fallback UI when a descendant throws during render. */
  static getDerivedStateFromError(error: Error): Partial<AppErrorBoundaryState> {
    return { hasError: true, error };
  }

  /** Log the captured error and component stack to the console sink. */
  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error("Unhandled render error:", error, errorInfo.componentStack);
    this.setState({ componentStack: errorInfo.componentStack ?? null });
  }

  componentWillUnmount(): void {
    if (this.copyResetTimer) clearTimeout(this.copyResetTimer);
  }

  /** Navigate away from the dead error screen back to the app root. */
  private handleGoHome = (): void => {
    window.location.href = "/";
  };

  /** Build the plain-text error report and copy it to the clipboard. */
  private handleCopyDetails = (): void => {
    const { error, componentStack } = this.state;
    if (!error) return;

    const referenceId = getErrorReferenceId(error);
    const lines = [
      "CSM Portal error report",
      `Time: ${new Date().toISOString()}`,
      `URL: ${window.location.href}`,
      `Error: ${error.name}: ${error.message}`,
      ...(referenceId ? [`Correlation ID: ${referenceId}`] : []),
      "Component stack:",
      componentStack ?? "(unavailable)",
    ];

    void navigator.clipboard.writeText(lines.join("\n")).then(() => {
      this.setState({ copied: true });
      if (this.copyResetTimer) clearTimeout(this.copyResetTimer);
      this.copyResetTimer = setTimeout(() => {
        this.setState({ copied: false });
      }, COPY_CONFIRMATION_MS);
    });
  };

  /** Render the children, or the recovery screen once an error is caught. */
  render(): ReactNode {
    if (!this.state.hasError) {
      return this.props.children;
    }
    return (
      <Box
        sx={{
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          px: 3,
        }}
      >
        <Stack spacing={2} alignItems="center" sx={{ maxWidth: 480 }}>
          <Typography variant="h5">Something went wrong</Typography>
          <Typography variant="body1" color="text.secondary" textAlign="center">
            Something went wrong loading this page. Try reloading — your session
            will be kept.
          </Typography>
          <Stack direction="row" spacing={1.5}>
            <Button variant="contained" onClick={() => window.location.reload()}>
              Reload page
            </Button>
            <Button variant="outlined" onClick={this.handleGoHome}>
              Go to home
            </Button>
          </Stack>
          <Tooltip title={this.state.copied ? "Copied!" : "Copy error details"} placement="top">
            <Button
              size="small"
              variant="text"
              color="inherit"
              onClick={this.handleCopyDetails}
              startIcon={this.state.copied ? <Check size={14} /> : <Copy size={14} />}
              sx={{ color: "text.disabled" }}
              aria-label="Copy error details"
            >
              {this.state.copied ? "Copied" : "Copy error details"}
            </Button>
          </Tooltip>
        </Stack>
      </Box>
    );
  }
}
