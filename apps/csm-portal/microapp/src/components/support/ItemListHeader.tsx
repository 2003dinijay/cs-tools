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

import type { ReactNode } from "react";
import { Button, Stack, Typography } from "@wso2/oxygen-ui";
import { ChevronRight } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router-dom";

interface ItemListHeaderProps {
  title: string;
  viewAllPath: string;
  children: ReactNode;
}

// Mirrors the customer-portal microapp's ItemListView
// (apps/customer-portal/microapp/src/components/features/support/ItemListView.tsx): a section
// header row (title left, "View All" link right) above the truncated recent-items list.
export function ItemListHeader({ title, viewAllPath, children }: ItemListHeaderProps) {
  return (
    <Stack gap={1.5}>
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="subtitle1">{title}</Typography>
        <Button
          component={Link}
          to={viewAllPath}
          variant="text"
          size="small"
          endIcon={<ChevronRight size={16} />}
          disableRipple
        >
          View All
        </Button>
      </Stack>

      {children}
    </Stack>
  );
}
