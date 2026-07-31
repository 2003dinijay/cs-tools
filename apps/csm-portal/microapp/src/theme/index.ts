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

import type { OxygenTheme } from "@wso2/oxygen-ui/styles/Themes/OxygenThemeBase";
import { AcrylicOrangeTheme, extendTheme } from "@wso2/oxygen-ui";
import { typography } from "@theme/typography";

const theme = extendTheme(AcrylicOrangeTheme, {
  typography,
  components: {
    // @wso2/oxygen-ui's own MuiDialog override (dist/index.js) sets `opacity: 0.7` plus a
    // backdrop blur on the dialog surface itself — on top of an already semi-transparent
    // background.paper token, that's a double-transparency effect that makes the whole
    // surface, including its text, read as hazy/unreadable. All of this app's filter sheets
    // (FiltersSheet.tsx and friends) are built on MUI Dialog, so fix it once here rather than
    // overriding sx per sheet. Solid, not background.paper/acrylic, and reads
    // `theme.vars.palette.*` (a CSS var) rather than the static `theme.palette.mode`, since this
    // theme switches light/dark purely via CSS variables reacting to the OS color scheme.
    MuiDialog: {
      styleOverrides: {
        paper: ({ theme }) => ({
          WebkitBackdropFilter: "none",
          backdropFilter: "none",
          background: theme.vars.palette.background.default,
          opacity: 1,
        }),
      },
    },
  },
}) as OxygenTheme;

export default theme;
