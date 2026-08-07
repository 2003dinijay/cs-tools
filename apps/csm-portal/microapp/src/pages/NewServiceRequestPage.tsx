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

import { useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { Button, FormControl, FormHelperText, InputLabel, MenuItem, Select, Stack, Typography } from "@wso2/oxygen-ui";
import { useMutation, useQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { deployments } from "@src/services/deployments";
import { catalogs } from "@src/services/catalogs";
import { projects } from "@src/services/projects";
import { attachments as attachmentsService } from "@src/services/attachments";
import type { Project } from "@src/types";
import { Logger } from "@utils/logger";
import { AttachmentsField } from "@components/support/AttachmentsField";
import { ProjectSelect } from "@components/support/ProjectSelect";
import { CatalogVariableFields } from "@components/operations/CatalogVariableFields";
import {
  encodeVariableValue,
  getFirstEmptyRequiredField,
  getUserEditableVariables,
  isAttachmentField,
} from "@utils/catalogVariables";
import type { PendingAttachment } from "@utils/attachments";

// Router (`navigate(..., { state })`) payload carried from a case's "Create service request"
// action (case detail's Linked Items tab) to this page, so the form can prefill from the
// originating case and file the new SR as linked to it in one step. Mirrors the webapp's
// CreateServiceRequestFromCaseNavState: `projectId` locks the Project field read-only;
// `deploymentId`/`deployedProductId` are just starting values and stay fully editable.
export interface CreateServiceRequestFromCaseNavState {
  projectId: string;
  relatedCaseId: string;
  relatedCaseNumber?: string;
  deploymentId?: string;
  deployedProductId?: string;
}

// Ports the webapp's CreateServiceRequestPage.tsx (features/csm-operations/pages/): the same
// cascading Project → Deployment → Deployed product → Catalog → Catalog item picker, driving a
// dynamic form built from the chosen catalog item's ServiceNow variables. Layout follows this
// app's own NewCasePage.tsx (single-column Stack) rather than the webapp's Grid/Card, and — like
// NewCasePage — the Description variable renders as a plain multiline field rather than the
// rich-text editor (this app has no rich-text component).
export default function NewServiceRequestPage() {
  const navigate = useNavigate();
  const fromCaseState = useLocation().state as CreateServiceRequestFromCaseNavState | undefined;

  const [project, setProject] = useState<Project | null>(null);
  const [deploymentId, setDeploymentId] = useState(fromCaseState?.deploymentId ?? "");
  const [deployedProductId, setDeployedProductId] = useState(fromCaseState?.deployedProductId ?? "");
  const [catalogId, setCatalogId] = useState("");
  const [catalogItemId, setCatalogItemId] = useState("");
  // Variable answers, keyed by variable id.
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const [submitError, setSubmitError] = useState<string | null>(null);
  // createCase.isPending alone only covers the case-creation call — it flips back to false while
  // the attachment uploads below are still in flight, which would let the button re-enable and
  // fire a duplicate submission. Tracks the whole handleSubmit lifecycle instead, same as
  // NewCasePage.tsx.
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Resolves the full Project (ProjectSelect needs more than the nav state's bare id) once, to
  // seed the locked field below — never re-runs once `project` is set, so it doesn't fight a
  // user's own pick on a page that arrived without a related case.
  const lockedProjectId = fromCaseState?.projectId;
  const lockedProjectQuery = useQuery({ ...projects.get(lockedProjectId ?? ""), enabled: !!lockedProjectId });
  useEffect(() => {
    if (lockedProjectQuery.data && !project) setProject(lockedProjectQuery.data);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lockedProjectQuery.data]);

  const deploymentsQuery = useQuery(deployments.byProject(project?.id ?? ""));
  const productsQuery = useQuery(deployments.productsByDeployment(deploymentId));
  const catalogsQuery = useQuery(catalogs.search(deployedProductId));
  const variablesQuery = useQuery(catalogs.itemVariables(catalogId, catalogItemId));

  const createCase = useMutation({ mutationFn: cases.create });
  const uploadAttachment = useMutation({ mutationFn: attachmentsService.create });

  const catalogItems = useMemo(
    () => catalogsQuery.data?.find((c) => c.id === catalogId)?.catalogItems ?? [],
    [catalogsQuery.data, catalogId],
  );

  // ServiceNow returns context/hidden fields mixed in; only user-editable (non-attachment)
  // variables get a rendered input. Attachments go through the shared AttachmentsField below.
  const allVariables = useMemo(() => variablesQuery.data ?? [], [variablesQuery.data]);
  const renderableVars = useMemo(
    () => getUserEditableVariables(allVariables).filter((v) => !isAttachmentField(v)),
    [allVariables],
  );
  const firstEmptyRequired = useMemo(() => getFirstEmptyRequiredField(allVariables, answers), [allVariables, answers]);

  // A deployed product with no catalogs is the common non-ServiceNow case; tell the engineer
  // rather than leaving an empty dropdown.
  const noCatalogs = !!deployedProductId && catalogsQuery.isSuccess && (catalogsQuery.data?.length ?? 0) === 0;

  // Cascade resets: a parent change invalidates every dependent field below it.
  const handleProjectChange = (next: Project | null) => {
    setProject(next);
    setDeploymentId("");
    setDeployedProductId("");
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };
  const handleDeploymentChange = (next: string) => {
    setDeploymentId(next);
    setDeployedProductId("");
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };
  const handleDeployedProductChange = (next: string) => {
    setDeployedProductId(next);
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };
  const handleCatalogChange = (next: string) => {
    setCatalogId(next);
    setCatalogItemId("");
    setAnswers({});
  };
  const handleCatalogItemChange = (next: string) => {
    setCatalogItemId(next);
    setAnswers({});
  };

  const canSubmit =
    !!project &&
    !!deploymentId &&
    !!deployedProductId &&
    !!catalogId &&
    !!catalogItemId &&
    !variablesQuery.isLoading &&
    !variablesQuery.isError &&
    // Hot fix (mirrors the webapp and, upstream, the customer portal): all typable variables required.
    firstEmptyRequired === null &&
    !createCase.isPending &&
    !isSubmitting;

  const handleSubmit = async () => {
    if (!canSubmit || !project) return;
    setSubmitError(null);
    setIsSubmitting(true);

    // Send user-editable, non-attachment variables, encoded by type, omitting empties.
    // Context/hidden fields are excluded by getUserEditableVariables.
    const variablePayload = getUserEditableVariables(allVariables)
      .filter((v) => !isAttachmentField(v))
      .map((v) => ({ id: v.id, value: encodeVariableValue(v, answers[v.id] ?? "") }))
      .filter((v) => v.value !== "");

    try {
      const created = await createCase.mutateAsync({
        type: "service_request",
        projectId: project.id,
        deploymentId,
        deployedProductId,
        catalogId,
        catalogItemId,
        variables: variablePayload,
        ...(fromCaseState?.relatedCaseId ? { relatedCaseId: fromCaseState.relatedCaseId } : {}),
      });

      // The create endpoint doesn't attach files for service requests, so upload them to the new
      // case afterwards, same as NewCasePage.tsx — a partial failure still lands the case.
      const results = await Promise.allSettled(
        attachments.map((attachment) =>
          uploadAttachment.mutateAsync({
            referenceId: created.id,
            referenceType: "case",
            name: attachment.name,
            type: attachment.type,
            file: attachment.file,
          }),
        ),
      );
      const failedCount = results.filter((r) => r.status === "rejected").length;
      if (failedCount > 0) {
        Logger.warn(`${failedCount} attachment(s) failed to upload to service request ${created.id}`);
      }

      navigate(`/cases/${created.id}`);
    } catch (error) {
      Logger.warn("Could not create the service request", error);
      setSubmitError("Could not create the service request. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Stack gap={2}>
      <Typography variant="h6">New Service Request</Typography>
      {fromCaseState && (
        <Typography variant="caption" color="text.secondary">
          Creating from case {fromCaseState.relatedCaseNumber ?? fromCaseState.relatedCaseId}
        </Typography>
      )}
      {lockedProjectQuery.isError && (
        <Typography variant="caption" color="error.main">
          Could not load the case's project. Pick one below to continue.
        </Typography>
      )}

      <Stack gap={2}>
        <ProjectSelect
          value={project}
          onChange={handleProjectChange}
          // Stays locked only while the case's project is still expected to arrive — a failed
          // lookup falls back to letting the engineer pick one manually instead of leaving the
          // form permanently stuck on a null project with no way to proceed.
          disabled={(!!fromCaseState && !lockedProjectQuery.isError) || createCase.isPending || isSubmitting}
        />

        <FormControl
          size="small"
          fullWidth
          disabled={!project || deploymentsQuery.isLoading || createCase.isPending || isSubmitting}
        >
          <InputLabel id="sr-deployment-label">Deployment</InputLabel>
          <Select
            labelId="sr-deployment-label"
            label="Deployment"
            value={deploymentId}
            onChange={(event) => handleDeploymentChange(event.target.value)}
          >
            {(deploymentsQuery.data ?? []).map((d) => (
              <MenuItem key={d.id} value={d.id}>
                {d.name}
              </MenuItem>
            ))}
          </Select>
          {!project ? (
            <FormHelperText>Select a project first</FormHelperText>
          ) : deploymentsQuery.isLoading ? (
            <FormHelperText>Loading deployments…</FormHelperText>
          ) : (deploymentsQuery.data ?? []).length === 0 ? (
            <FormHelperText>No deployments found for this project.</FormHelperText>
          ) : null}
        </FormControl>

        <FormControl
          size="small"
          fullWidth
          disabled={!deploymentId || productsQuery.isLoading || createCase.isPending || isSubmitting}
        >
          <InputLabel id="sr-product-label">Deployed Product</InputLabel>
          <Select
            labelId="sr-product-label"
            label="Deployed Product"
            value={deployedProductId}
            onChange={(event) => handleDeployedProductChange(event.target.value)}
          >
            {(productsQuery.data ?? []).map((p) => (
              <MenuItem key={p.id} value={p.id}>
                {p.label}
              </MenuItem>
            ))}
          </Select>
          {!deploymentId ? (
            <FormHelperText>Select a deployment first</FormHelperText>
          ) : productsQuery.isLoading ? (
            <FormHelperText>Loading products…</FormHelperText>
          ) : (productsQuery.data ?? []).length === 0 ? (
            <FormHelperText>No deployed products found for this deployment.</FormHelperText>
          ) : null}
        </FormControl>

        <FormControl
          size="small"
          fullWidth
          disabled={!deployedProductId || catalogsQuery.isLoading || noCatalogs || createCase.isPending || isSubmitting}
        >
          <InputLabel id="sr-catalog-label">Catalog</InputLabel>
          <Select
            labelId="sr-catalog-label"
            label="Catalog"
            value={catalogId}
            onChange={(event) => handleCatalogChange(event.target.value)}
          >
            {(catalogsQuery.data ?? []).map((c) => (
              <MenuItem key={c.id} value={c.id}>
                {c.name ?? c.id}
              </MenuItem>
            ))}
          </Select>
          {!deployedProductId ? (
            <FormHelperText>Select a deployed product first</FormHelperText>
          ) : catalogsQuery.isLoading ? (
            <FormHelperText>Loading catalogs…</FormHelperText>
          ) : catalogsQuery.isError ? (
            <FormHelperText error>Couldn't load catalogs. Try again.</FormHelperText>
          ) : noCatalogs ? (
            <FormHelperText>No service catalogs are available for this product.</FormHelperText>
          ) : null}
        </FormControl>

        <FormControl size="small" fullWidth disabled={!catalogId || createCase.isPending || isSubmitting}>
          <InputLabel id="sr-catalog-item-label">Catalog Item</InputLabel>
          <Select
            labelId="sr-catalog-item-label"
            label="Catalog Item"
            value={catalogItemId}
            onChange={(event) => handleCatalogItemChange(event.target.value)}
          >
            {catalogItems.map((ci) => (
              <MenuItem key={ci.id} value={ci.id}>
                {ci.name ?? ci.id}
              </MenuItem>
            ))}
          </Select>
          {!catalogId ? (
            <FormHelperText>Select a catalog first</FormHelperText>
          ) : catalogItems.length === 0 ? (
            <FormHelperText>No items found in this catalog.</FormHelperText>
          ) : null}
        </FormControl>

        {/* Dynamic variable form for the chosen catalog item. */}
        {catalogItemId && (
          <>
            {variablesQuery.isLoading ? (
              <Typography variant="body2" color="text.secondary">
                Loading request form…
              </Typography>
            ) : variablesQuery.isError ? (
              <Typography variant="body2" color="error.main">
                Could not load the request form for this catalog item. Try again.
              </Typography>
            ) : renderableVars.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                This catalog item has no additional fields.
              </Typography>
            ) : (
              <CatalogVariableFields
                variables={renderableVars}
                values={answers}
                disabled={createCase.isPending || isSubmitting}
                onChange={(id, value) => setAnswers((prev) => ({ ...prev, [id]: value }))}
              />
            )}
          </>
        )}

        {catalogItemId && !variablesQuery.isLoading && !variablesQuery.isError && (
          <AttachmentsField
            attachments={attachments}
            onChange={setAttachments}
            disabled={createCase.isPending || isSubmitting}
          />
        )}

        {firstEmptyRequired && !variablesQuery.isLoading && catalogItemId && (
          <Typography variant="caption" color="text.secondary">
            Required field: {firstEmptyRequired}
          </Typography>
        )}

        {submitError && (
          <Typography variant="body2" color="error.main">
            {submitError}
          </Typography>
        )}

        <Stack direction="row" gap={1} justifyContent="flex-end">
          <Button onClick={() => navigate(-1)} disabled={createCase.isPending || isSubmitting}>
            Cancel
          </Button>
          <Button variant="contained" disabled={!canSubmit} onClick={() => void handleSubmit()}>
            {isSubmitting ? "Creating…" : "Create Service Request"}
          </Button>
        </Stack>
      </Stack>
    </Stack>
  );
}
