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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";

const navigateMock = vi.fn();
const postChangeRequestMutateMock = vi.fn();
const patchChangeRequestMutateMock = vi.fn();
const showErrorMock = vi.fn();
const postIsPending = false;
const patchIsPending = false;

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-operations/api/usePostChangeRequest", () => ({
  usePostChangeRequest: () => ({
    mutate: postChangeRequestMutateMock,
    get isPending() {
      return postIsPending;
    },
  }),
}));
vi.mock("@features/csm-operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({
    mutate: patchChangeRequestMutateMock,
    get isPending() {
      return patchIsPending;
    },
  }),
}));
vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => ({ data: undefined }),
}));
// CreateChangeRequestPage imports BackendApiError from the real API client
// module, which reads window.config at module load and throws outside a
// configured runtime. Mock it with a real class (so `instanceof` still
// works), mirroring CreateProblemPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));
// Every record-reference field on this form (Service / Service offering /
// Configuration item / Assignment group / Assigned to / Requested by /
// Originating service request) is a generic AsyncEntitySelect — out of scope
// to drive through its real search/dropdown interaction here, so it's stubbed
// as a plain labeled input that reports its id straight through onChange,
// same technique as CreateProblemPage.test.tsx.
vi.mock("@components/AsyncEntitySelect", () => ({
  default: ({
    label,
    value,
    onChange,
  }: {
    label: string;
    value: string;
    onChange: (next: string) => void;
  }) => (
    <input aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));
// The Planning fields use the Lexical rich-text editor; not under test here.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

// Imported after the mocks above so the module picks them up.
import CreateChangeRequestPage from "@features/csm-operations/pages/CreateChangeRequestPage";

function fillSubject(): void {
  fireEvent.change(screen.getByLabelText(/subject/i), {
    target: { value: "Roll out fix to production" },
  });
}

describe("CreateChangeRequestPage", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("renders the originating service request picker inside the optional section", () => {
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/originating service request/i)).toBeInTheDocument();
  });

  it("does not PATCH when no originating service request was picked — navigates straight to the created change request", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    const [, options] = postChangeRequestMutateMock.mock.calls[0];
    options.onSuccess({ changeRequest: { id: "chg-1", number: "CHG0000001" } });

    expect(patchChangeRequestMutateMock).not.toHaveBeenCalled();
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/chg-1");
  });

  it("PATCHes the created change request with caseId when a service request was picked, then navigates on success", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: "sr-123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    // POST /change-requests never carries caseId — it isn't a create field.
    const [postPayload, postOptions] = postChangeRequestMutateMock.mock.calls[0];
    expect(postPayload).not.toHaveProperty("caseId");

    postOptions.onSuccess({ changeRequest: { id: "chg-1", number: "CHG0000001" } });

    expect(patchChangeRequestMutateMock).toHaveBeenCalledWith(
      { id: "chg-1", patch: { caseId: "sr-123" } },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    );

    const [, patchOptions] = patchChangeRequestMutateMock.mock.calls[0];
    patchOptions.onSuccess();
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/chg-1");
  });

  it("still navigates and surfaces a non-silent error when the follow-up PATCH fails", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: "sr-123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    const [, postOptions] = postChangeRequestMutateMock.mock.calls[0];
    postOptions.onSuccess({ changeRequest: { id: "chg-1", number: "CHG0000001" } });

    const [, patchOptions] = patchChangeRequestMutateMock.mock.calls[0];
    patchOptions.onError(new Error("link failed"));

    expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("linking it to the originating service request failed"));
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/chg-1");
  });

  it("surfaces a create-mutation error via the shared error banner", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    const [, options] = postChangeRequestMutateMock.mock.calls[0];
    options.onError(new Error("network down"));
    expect(showErrorMock).toHaveBeenCalledWith(
      "Could not create the change request. Please try again.",
      expect.any(Error),
    );
  });
});
