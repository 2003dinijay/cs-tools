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
  Box,
  Button,
  Card,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useNavigate } from "react-router";
import AsyncProjectSelect from "@features/csm-cases/components/AsyncProjectSelect";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";

const SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];

// Submission is disabled until the backend exposes a service-request create
// endpoint. SR is currently only a case *type* (`service_request`) in the
// contract; there is no SR entity/endpoint and `POST /cases` accepts only
// `type: "support"`. The form is built and validated so it can be wired the
// moment the BE adds the endpoint (and its catalog/category model) — flip this
// flag and add the mutation in `handleSubmit`.
const SR_CREATE_ENABLED = false;

export default function CreateServiceRequestPage(): JSX.Element {
  const navigate = useNavigate();

  const [projectId, setProjectId] = useState("");
  const [priority, setPriority] = useState<Severity | "">("");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");

  const isValid = useMemo(
    () =>
      !!projectId &&
      !!priority &&
      subject.trim().length > 0 &&
      description.trim().length > 0,
    [projectId, priority, subject, description],
  );

  // Wire the real mutation here when SR_CREATE_ENABLED flips on.
  const handleSubmit = (): void => {
    if (!SR_CREATE_ENABLED || !isValid) return;
  };

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate("/operations")}
        sx={{ mb: 1 }}
      >
        Back to operations
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New service request
      </Typography>

      <Card variant="outlined" sx={{ p: 3 }}>
        <Alert severity="info" sx={{ mb: 2.5 }}>
          Service request creation isn&apos;t available yet — the backend
          endpoint (and the request catalog) are still to come. You can review
          the form below; submitting is disabled for now.
        </Alert>

        <Grid container spacing={2.5}>
          <Grid size={{ xs: 12, md: 8 }}>
            <AsyncProjectSelect
              value={projectId}
              onChange={setProjectId}
              required
            />
          </Grid>

          <Grid size={{ xs: 12, sm: 6, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="sr-priority-label">Priority</InputLabel>
              <Select
                labelId="sr-priority-label"
                label="Priority"
                value={priority}
                onChange={(e) => setPriority(e.target.value as Severity)}
              >
                {SEVERITIES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {s} · {SEVERITY_LABEL[s]}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <TextField
              label="Subject"
              size="small"
              fullWidth
              required
              value={subject}
              onChange={(e) => setSubject(e.target.value.slice(0, 200))}
              helperText={
                subject.length >= 160 ? `${subject.length}/200` : undefined
              }
            />
          </Grid>

          <Grid size={{ xs: 12 }}>
            <TextField
              label="Description"
              size="small"
              fullWidth
              required
              multiline
              minRows={5}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Describe what the customer is requesting…"
            />
          </Grid>
        </Grid>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={() => navigate("/operations")}>
            Cancel
          </Button>
          <Tooltip
            title={
              SR_CREATE_ENABLED
                ? ""
                : "Service request creation will be enabled once the backend supports it."
            }
          >
            <Box component="span">
              <Button
                variant="contained"
                onClick={handleSubmit}
                disabled={!SR_CREATE_ENABLED || !isValid}
              >
                Create service request
              </Button>
            </Box>
          </Tooltip>
        </Box>
      </Card>
    </Box>
  );
}
