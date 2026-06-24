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

import { Box, Button, Chip, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import { useNavigate } from "react-router";

type OperationsTabId = "service_requests" | "change_requests" | "incidents";

/**
 * Operations landing — the home for the managed-cloud operational entities,
 * split into Service Requests / Change Requests / Incidents tabs. Only the
 * Service Request create entry point is live; the list views and the
 * CR/Incident tabs are awaiting their backend endpoints, so they render
 * "coming soon" placeholders. Replaces the previous blanket coming-soon page.
 */
export default function OperationsPage(): JSX.Element {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<OperationsTabId>("service_requests");

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box>
        <Typography variant="h5">Operations</Typography>
        <Typography variant="body2" color="text.secondary">
          Service requests, change requests, and incidents across customers.
        </Typography>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, v) => setActiveTab(v as OperationsTabId)}
        >
          <Tab value="service_requests" label="Service requests" />
          <Tab value="change_requests" label="Change requests" />
          <Tab value="incidents" label="Incidents" />
        </Tabs>
      </Box>

      {activeTab === "service_requests" && (
        <TabPanel
          description="Catalog-driven requests raised on behalf of a customer (e.g. configuration, access, infrastructure changes)."
          action={
            <Button
              variant="contained"
              color="primary"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => navigate("/operations/service-requests/new")}
            >
              Create service request
            </Button>
          }
        >
          <Typography variant="body2" color="text.secondary">
            The service request list will appear here once the backend search
            endpoint is available.
          </Typography>
        </TabPanel>
      )}

      {activeTab === "change_requests" && (
        <TabPanel
          description="Controlled changes, linked to service requests, with peer / CAB approval."
          comingSoon
        />
      )}

      {activeTab === "incidents" && (
        <TabPanel
          description="SaaS incidents raised by CRE or automation; may link to a case."
          comingSoon
        />
      )}
    </Box>
  );
}

interface TabPanelProps {
  description: string;
  action?: ReactNode;
  comingSoon?: boolean;
  children?: ReactNode;
}

function TabPanel({
  description,
  action,
  comingSoon,
  children,
}: TabPanelProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1.5,
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="body2" color="text.secondary">
            {description}
          </Typography>
          {comingSoon && (
            <Chip
              size="small"
              label="Coming soon"
              color="warning"
              variant="outlined"
            />
          )}
        </Box>
        {action}
      </Box>
      {children}
    </Box>
  );
}
