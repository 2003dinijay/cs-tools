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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import EditProjectOnboardingDialog from "@features/csm-projects/components/EditProjectOnboardingDialog";

// The backend client reads runtime config at module load, absent under vitest.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ get: vi.fn(), post: vi.fn(), patch: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
// The owner picker's directory search is covered by its own hook test.
vi.mock("@features/csm-projects/api/useSearchInternalUsers", () => ({
  useSearchInternalUsers: () => ({ data: [], isFetching: false, isError: false }),
}));

const DATES = {
  onboardingStartDate: "2026-03-01T00:00:00Z",
  goLivePlanDate: "2026-04-01T00:00:00Z",
  goLiveDate: null,
  onboardingExpiryDate: "2026-12-31T00:00:00Z",
};

function renderDialog(onSave = vi.fn()) {
  render(
    <EditProjectOnboardingDialog
      currentStatus="In-Progress"
      currentOwner={{ id: "00000000-0000-0000-0000-000000000000", name: "Jane Doe" }}
      currentDates={DATES}
      isSaving={false}
      onClose={() => {}}
      onSave={onSave}
    />,
  );
  return onSave;
}

/** The picker's field is a group of sections; its value lives in a hidden
 *  input beside it, which is what accepts a typed/pasted full date. */
function dateInput(label: string): HTMLInputElement {
  const group = screen.getByRole("group", { name: label });
  const input = group.parentElement?.querySelector("input");
  if (!input) throw new Error(`no input for ${label}`);
  return input;
}

function typeDate(label: string, value: string) {
  fireEvent.change(dateInput(label), { target: { value } });
}

describe("EditProjectOnboardingDialog dates", () => {
  it("seeds each date picker from the UTC calendar day of the stored value", () => {
    renderDialog();
    expect(dateInput("Onboarding start date")).toHaveValue(
      "03/01/2026",
    );
    expect(dateInput("Go-live plan date")).toHaveValue("04/01/2026");
    expect(dateInput("Go-live date")).toHaveValue("");
    expect(dateInput("Onboarding expiry date")).toHaveValue(
      "12/31/2026",
    );
  });

  it("keeps Save disabled until something changes", () => {
    renderDialog();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("sends only the changed date, under the request field name", () => {
    const onSave = renderDialog();
    typeDate("Go-live date", "05/15/2026");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith({ onboardingGoLiveDate: "2026-05-15" });
  });

  it("blocks Save when a changed date is earlier than the stored start date", () => {
    renderDialog();
    typeDate("Go-live date", "02/15/2026");
    expect(screen.getByText(/earlier than the onboarding start date/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("blocks Save when the start date is moved after a changed go-live date", () => {
    renderDialog();
    typeDate("Onboarding start date", "06/01/2026");
    typeDate("Go-live plan date", "05/01/2026");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    typeDate("Go-live plan date", "06/10/2026");
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("treats an emptied date as unchanged and never sends it", () => {
    const onSave = renderDialog();
    typeDate("Go-live plan date", "");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    typeDate("Go-live date", "05/15/2026");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSave).toHaveBeenCalledWith({ onboardingGoLiveDate: "2026-05-15" });
  });

  it("offers no clear buttons", () => {
    renderDialog();
    expect(screen.queryByRole("button", { name: /clear/i })).not.toBeInTheDocument();
  });
});
