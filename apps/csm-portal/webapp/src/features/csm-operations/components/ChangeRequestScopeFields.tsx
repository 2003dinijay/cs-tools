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

import { Autocomplete, Box, Button, Chip, TextField } from "@wso2/oxygen-ui";
import { Lock } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import AsyncProjectSelect from "@features/csm-cases/components/AsyncProjectSelect";
import type { ScopeOption } from "@features/csm-operations/api/useChangeRequestScopeLookups";
import type { ChangeRequestScope } from "@features/csm-operations/hooks/useChangeRequestScope";

interface ChangeRequestScopeFieldsProps {
  scope: ChangeRequestScope;
  disabled?: boolean;
  /** Prefix for element ids (`cr` on the create page, `cr-edit` in the dialog). */
  idPrefix: string;
  /** Whether the Customer Project can be cleared once picked (default true). */
  projectClearable?: boolean;
}

function selectedOptions(
  ids: string[],
  labels: Record<string, string>,
): ScopeOption[] {
  return ids.map((id) => ({ id, label: labels[id] ?? id }));
}

/**
 * The Customer Project / Deployments / Environments / Deployment products
 * block, in the order the ServiceNow change-request form lays them out.
 * Deployments and Environments stay disabled until their parent is chosen;
 * Deployment products is read-only (lock icon) because it is derived.
 */
export default function ChangeRequestScopeFields({
  scope,
  disabled = false,
  idPrefix,
  projectClearable = true,
}: ChangeRequestScopeFieldsProps): JSX.Element {
  const hasProject = !!scope.projectId;
  const hasDeployments = scope.deploymentIds.length > 0;

  const deploymentValue = selectedOptions(scope.deploymentIds, scope.deploymentLabels);
  const environmentValue = selectedOptions(scope.environmentIds, scope.environmentLabels);
  const productValue = selectedOptions(scope.deploymentProductIds, scope.deploymentProductLabels);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <AsyncProjectSelect
        id={`${idPrefix}-project`}
        label="Customer Project"
        value={scope.projectId}
        knownLabel={scope.projectLabel || undefined}
        onChange={scope.setProject}
        disabled={disabled}
        disableClearable={!projectClearable}
      />

      <Autocomplete<ScopeOption, true>
        multiple
        fullWidth
        size="small"
        id={`${idPrefix}-deployments`}
        options={scope.deploymentOptions}
        value={deploymentValue}
        disabled={disabled || !hasProject}
        loading={scope.lookups.isLoading}
        disableCloseOnSelect
        getOptionLabel={(o) => o.label}
        isOptionEqualToValue={(o, v) => o.id === v.id}
        onChange={(_e, next) => scope.setDeployments(next.map((o) => o.id))}
        noOptionsText={
          scope.lookups.isError
            ? "Couldn't load deployments. Try again."
            : scope.lookups.isLoading
              ? "Loading deployments…"
              : "This project has no deployments"
        }
        renderInput={(params) => (
          <TextField
            {...params}
            label="Deployments"
            placeholder={deploymentValue.length ? undefined : "Select deployments…"}
            error={scope.lookups.isError}
            helperText={
              scope.lookups.isError ? (
                <>
                  Couldn&apos;t load deployments.{" "}
                  <Button size="small" variant="text" onClick={scope.lookups.refetch} sx={{ minWidth: 0, p: 0 }}>
                    Retry
                  </Button>
                </>
              ) : !hasProject ? (
                "Select a Customer Project first."
              ) : undefined
            }
          />
        )}
      />

      <Autocomplete<ScopeOption, true>
        multiple
        fullWidth
        size="small"
        id={`${idPrefix}-environments`}
        options={scope.environmentOptions}
        value={environmentValue}
        disabled={disabled || !hasDeployments}
        disableCloseOnSelect
        getOptionLabel={(o) => o.label}
        isOptionEqualToValue={(o, v) => o.id === v.id}
        onChange={(_e, next) => scope.setEnvironments(next.map((o) => o.id))}
        noOptionsText="The chosen deployments provide no environments"
        renderInput={(params) => (
          <TextField
            {...params}
            label="Environments"
            placeholder={environmentValue.length ? undefined : "Select environments…"}
            helperText={
              !hasDeployments
                ? "Select deployments first."
                : "Limited to the environments of the selected deployments."
            }
          />
        )}
      />

      <TextField
        id={`${idPrefix}-deployment-products`}
        label="Deployment products"
        size="small"
        fullWidth
        value=""
        placeholder={productValue.length ? undefined : "—"}
        helperText="Derived from the selected deployments"
        slotProps={{
          inputLabel: { shrink: true },
          input: {
            readOnly: true,
            startAdornment: productValue.length ? (
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, mr: 0.5 }}>
                {productValue.map((p) => (
                  <Chip key={p.id} size="small" variant="outlined" label={p.label} />
                ))}
              </Box>
            ) : undefined,
            endAdornment: <Lock size={16} aria-hidden style={{ opacity: 0.6 }} />,
          },
          htmlInput: { "aria-readonly": true },
        }}
      />
    </Box>
  );
}
