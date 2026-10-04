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
import type { BeOutageDetail } from "@api/backend/types";

vi.mock("@features/csm-operations/api/useOutages", () => ({
  useGetOutageMetadata: () => ({ data: undefined }),
}));
vi.mock("@api/useSearchConfigurationItems", () => ({ useSearchConfigurationItems: vi.fn() }));
vi.mock("@features/csm-operations/api/useSearchIncidentsForSelect", () => ({
  useSearchIncidentsForSelect: vi.fn(),
}));
vi.mock("@components/AsyncEntitySelect", () => ({ default: () => null }));
vi.mock("@features/csm-operations/components/OutagePublicationNotice", () => ({ default: () => null }));

import EditOutageDialog from "@features/csm-operations/components/EditOutageDialog";

const outage = {
  id: "3ca9cf3d-566a-4345-93b2-86845772be45",
  number: "OUT0010000",
  type: "outage",
  status: "in_progress",
  begin: "2026-10-04 15:38:00",
  end: null,
  duration: null,
  shortDescription: "Login errors",
  configurationItem: null,
  incident: null,
  affectedConfigurationItems: [],
  publishesToStatusPage: false,
  statusPageCloud: null,
  notifyInternalStakeholders: false,
  outageCommunication: false,
  impact: "2 - High",
  state: null,
  createdOn: "2026-10-04 15:38:00",
  createdBy: "sasmitha@wso2.com",
  updatedOn: "2026-10-04 15:38:00",
  updatedBy: "sasmitha@wso2.com",
} as unknown as BeOutageDetail;

describe("EditOutageDialog — email opt-ins", () => {
  it("starts from the outage's stored values", () => {
    render(<EditOutageDialog outage={outage} isSaving={false} onClose={vi.fn()} onSave={vi.fn()} />);
    expect(screen.getByLabelText(/Notify internal stakeholders/)).not.toBeChecked();
    expect(screen.getByLabelText(/Outage communication/)).not.toBeChecked();
    expect(screen.getByLabelText("Impact")).toHaveValue("2 - High");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("sends only what changed, and an emptied label as a clear", () => {
    const onSave = vi.fn();
    render(<EditOutageDialog outage={outage} isSaving={false} onClose={vi.fn()} onSave={onSave} />);
    fireEvent.click(screen.getByLabelText(/Outage communication/));
    fireEvent.change(screen.getByLabelText("Impact"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledWith({ outageCommunication: true, impact: "" });
  });
});
