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
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { useNavTransition } from "@hooks/useNavTransition";

export interface CreateFromProjectRequiredDialogProps {
  /** Whether the dialog is open. */
  open: boolean;
  /**
   * Singular noun for the entity being created ("case", "service request",
   * "security report", "engagement") — used to phrase the dialog copy so it
   * reads naturally regardless of which listing page it was opened from.
   */
  entityNoun: string;
  onClose: () => void;
}

/**
 * Shared guardrail dialog for every "Create X" button on a cross-project
 * listing page (Cases, Service Requests, Security Reports, Engagements).
 * Creating directly from a listing risks picking the wrong project — there is
 * no project already in context — so this steers the engineer to open the
 * correct project first and create from there instead, where a locked-to-that-
 * project create action is available.
 */
export default function CreateFromProjectRequiredDialog({
  open,
  entityNoun,
  onClose,
}: CreateFromProjectRequiredDialogProps): JSX.Element {
  const navigate = useNavTransition();

  const handleGoToProjects = (): void => {
    onClose();
    navigate("/customers/projects");
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Create from the project instead</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary">
          Creating a {entityNoun} from this list risks picking the wrong
          project. Open the customer&apos;s project first — from Customers →
          the account → the project — and create the {entityNoun} from its
          Project details page instead, where the project is already locked
          in.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose}>
          Cancel
        </Button>
        <Button variant="contained" onClick={handleGoToProjects}>
          Go to projects
        </Button>
      </DialogActions>
    </Dialog>
  );
}
