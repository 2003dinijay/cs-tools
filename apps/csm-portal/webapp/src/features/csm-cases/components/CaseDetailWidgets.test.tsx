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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useState, type ComponentProps, type JSX, type ReactElement } from "react";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx). `UserRefLink`
// (used for the watcher chip and the attachment uploader) resolves an
// unknown id through `useResolvedUserId`, which calls this client.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn().mockResolvedValue({ users: [] }) }),
}));

import {
  AttachmentsWidget,
  TagsWidget,
  WatchersWidget,
} from "@features/csm-cases/components/CaseDetailWidgets";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import type {
  CaseAttachment,
  CaseTag,
  CaseWatcher,
} from "@features/csm-cases/types/csmCases";

// `previewTarget`/`onPreviewTargetChange` (part of the widget's `preview`
// prop) are lifted to the parent page (see CsmCaseDetailPage) so the preview
// dialog resets on case-to-case navigation. This harness owns that bit of
// state locally, standing in for the parent, and keeps the flat
// `onGetPreviewContent` shape for individual tests below so only this
// harness needs to know about the grouped `preview` prop.
function AttachmentsWidgetHarness({
  onGetPreviewContent,
  ...props
}: Omit<ComponentProps<typeof AttachmentsWidget>, "preview"> & {
  onGetPreviewContent?: (attachment: CaseAttachment) => Promise<Blob>;
}): JSX.Element {
  const [previewTarget, setPreviewTarget] = useState<CaseAttachment | null>(
    null,
  );
  return (
    <AttachmentsWidget
      {...props}
      preview={
        onGetPreviewContent
          ? { onGetPreviewContent, previewTarget, onPreviewTargetChange: setPreviewTarget }
          : undefined
      }
    />
  );
}

vi.mock("@features/csm-users/api/useSearchUsers", () => ({
  useSearchUsers: vi.fn(),
}));

const mockUseSearchUsers = vi.mocked(useSearchUsers);

function mockCandidates(): void {
  mockUseSearchUsers.mockReturnValue({
    data: {
      users: [
        {
          id: "u-2",
          userName: "jsmith",
          name: "Jane Smith",
          email: "jane.smith@example.com",
          timezone: null,
          active: true,
        },
      ],
      total: 1,
      limit: 8,
      offset: 0,
      hasMore: false,
    },
    isFetching: false,
    isError: false,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
  } as any);
}

const TAGS: CaseTag[] = [
  { id: "tag-1", label: "micro-gw" },
  { id: "tag-2", label: "ws-policy" },
];

const WATCHERS: CaseWatcher[] = [
  { id: "w-1", name: "Jane Doe", email: "jane.doe@example.com" },
  { id: "w-2", name: "John Smith", isMe: true },
];

// `WatchersWidget` links each watcher's name to their profile page via
// `UserRefLink`, which renders a `react-router` `Link` and resolves its id
// through react-query — needs both a Router and a QueryClient context even
// outside a full app render.
function renderWithRouter(ui: ReactElement): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TagsWidget", () => {
  it("renders an empty state when there are no tags", () => {
    render(<TagsWidget tags={[]} />);
    expect(screen.getByText("No tags applied.")).toBeInTheDocument();
  });

  it("renders every tag as a chip", () => {
    render(<TagsWidget tags={TAGS} />);
    expect(screen.getByText("micro-gw")).toBeInTheDocument();
    expect(screen.getByText("ws-policy")).toBeInTheDocument();
  });

  it("calls onAdd when the Tag button is clicked", () => {
    const onAdd = vi.fn();
    render(<TagsWidget tags={TAGS} onAdd={onAdd} />);
    fireEvent.click(screen.getByRole("button", { name: /^tag$/i }));
    expect(onAdd).toHaveBeenCalled();
  });

  it("calls onRemove with the tag when its chip delete icon is clicked", () => {
    const onRemove = vi.fn();
    render(<TagsWidget tags={TAGS} onRemove={onRemove} />);
    const chip = screen.getByText("micro-gw").closest(".MuiChip-root");
    const deleteIcon = chip?.querySelector(".MuiChip-deleteIcon");
    expect(deleteIcon).toBeTruthy();
    fireEvent.click(deleteIcon as Element);
    expect(onRemove).toHaveBeenCalledWith(TAGS[0]);
  });

  it("omits the delete affordance when onRemove is not provided", () => {
    render(<TagsWidget tags={TAGS} />);
    const chip = screen.getByText("micro-gw").closest(".MuiChip-root");
    expect(chip?.querySelector(".MuiChip-deleteIcon")).toBeFalsy();
  });
});

describe("WatchersWidget", () => {
  it("renders an empty state when there are no watchers", () => {
    renderWithRouter(<WatchersWidget watchers={[]} />);
    expect(
      screen.getByText("No one is watching this case."),
    ).toBeInTheDocument();
  });

  it("renders every watcher as a chip, marking the current user", () => {
    renderWithRouter(<WatchersWidget watchers={WATCHERS} />);
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("John Smith (you)")).toBeInTheDocument();
  });

  it("hides the Add watcher action when onAdd is omitted", () => {
    renderWithRouter(<WatchersWidget watchers={WATCHERS} />);
    expect(
      screen.queryByRole("button", { name: /add watcher/i }),
    ).not.toBeInTheDocument();
  });

  it("omits the per-chip remove affordance when onRemove is omitted", () => {
    renderWithRouter(<WatchersWidget watchers={WATCHERS} />);
    const chip = screen.getByText("Jane Doe").closest(".MuiChip-root");
    expect(chip?.querySelector(".MuiChip-deleteIcon")).toBeFalsy();
  });

  it("calls onRemove with the watcher when its chip delete icon is clicked", () => {
    const onRemove = vi.fn();
    renderWithRouter(<WatchersWidget watchers={WATCHERS} onRemove={onRemove} />);
    const chip = screen.getByText("Jane Doe").closest(".MuiChip-root");
    const deleteIcon = chip?.querySelector(".MuiChip-deleteIcon");
    expect(deleteIcon).toBeTruthy();
    fireEvent.click(deleteIcon as Element);
    expect(onRemove).toHaveBeenCalledWith(WATCHERS[0]);
  });

  it("opens an inline search panel on Add watcher and calls onAdd for a picked candidate — no dialog involved", () => {
    mockCandidates();
    const onAdd = vi.fn();
    renderWithRouter(<WatchersWidget watchers={WATCHERS} onAdd={onAdd} />);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /add watcher/i }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /jane smith/i }));
    expect(onAdd).toHaveBeenCalledWith("jane.smith@example.com", "Jane Smith");
  });

  it("closes the inline search panel when its cancel button is clicked", () => {
    mockCandidates();
    renderWithRouter(<WatchersWidget watchers={WATCHERS} onAdd={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: /add watcher/i }));
    expect(
      screen.getByPlaceholderText(/search people to add/i),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /cancel adding a watcher/i }));
    expect(
      screen.queryByPlaceholderText(/search people to add/i),
    ).not.toBeInTheDocument();
  });
});

describe("AttachmentsWidget — preview affordance", () => {
  const IMAGE_ATTACHMENT: CaseAttachment = {
    id: "att-1",
    filename: "screenshot.png",
    size: 2048,
    contentType: "image/png",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-01T00:00:00Z",
  };
  const VIDEO_ATTACHMENT: CaseAttachment = {
    id: "att-2",
    filename: "repro.mp4",
    size: 4096,
    contentType: "video/mp4",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-02T00:00:00Z",
  };
  const ZIP_ATTACHMENT: CaseAttachment = {
    id: "att-3",
    filename: "logs.zip",
    size: 8192,
    contentType: "application/zip",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-03T00:00:00Z",
  };

  beforeEach(() => {
    // jsdom has no object-URL implementation; stub both so the preview
    // dialog's blob -> object URL -> revoke lifecycle can run in tests.
    globalThis.URL.createObjectURL = vi.fn(() => "blob:mock-url");
    globalThis.URL.revokeObjectURL = vi.fn();
  });

  it("shows Preview for an image but not for a video or a zip, when a fetcher is supplied", () => {
    renderWithRouter(
      <AttachmentsWidgetHarness
        attachments={[IMAGE_ATTACHMENT, VIDEO_ATTACHMENT, ZIP_ATTACHMENT]}
        onGetPreviewContent={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: `Preview ${IMAGE_ATTACHMENT.filename}` }),
    ).toBeInTheDocument();
    // Video is not previewable: the backend's safe-content-type allowlist
    // (`safeAttachmentTypes` in case_handler.go) has no video/* entry, so
    // GET /attachments/{id}/content always coerces a video response to
    // application/octet-stream. Offering a preview button here would rely
    // on the uploader-controlled metadata `contentType` instead of the
    // backend-verified one, defeating that allowlist.
    expect(
      screen.queryByRole("button", { name: `Preview ${VIDEO_ATTACHMENT.filename}` }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: `Preview ${ZIP_ATTACHMENT.filename}` }),
    ).not.toBeInTheDocument();
  });

  it("hides every Preview affordance when no fetcher is supplied", () => {
    renderWithRouter(
      <AttachmentsWidgetHarness attachments={[IMAGE_ATTACHMENT, VIDEO_ATTACHMENT]} />,
    );
    expect(screen.queryByRole("button", { name: /^preview /i })).not.toBeInTheDocument();
  });

  it("opens the preview dialog, fetches content, and renders it as an image", async () => {
    const fetchContent = vi
      .fn()
      .mockResolvedValue(new Blob(["fake"], { type: "image/png" }));
    renderWithRouter(
      <AttachmentsWidgetHarness
        attachments={[IMAGE_ATTACHMENT]}
        onGetPreviewContent={fetchContent}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: `Preview ${IMAGE_ATTACHMENT.filename}` }),
    );

    expect(fetchContent).toHaveBeenCalledWith(IMAGE_ATTACHMENT);
    await waitFor(() =>
      expect(screen.getByAltText(IMAGE_ATTACHMENT.filename)).toBeInTheDocument(),
    );
    expect(screen.getByAltText(IMAGE_ATTACHMENT.filename)).toHaveAttribute(
      "src",
      "blob:mock-url",
    );
  });

  it("shows an error message when the preview fetch fails", async () => {
    const fetchContent = vi.fn().mockRejectedValue(new Error("network down"));
    renderWithRouter(
      <AttachmentsWidgetHarness
        attachments={[IMAGE_ATTACHMENT]}
        onGetPreviewContent={fetchContent}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: `Preview ${IMAGE_ATTACHMENT.filename}` }),
    );

    await waitFor(() =>
      expect(screen.getByText("network down")).toBeInTheDocument(),
    );
  });

  it("still shows Download for every attachment regardless of preview support", () => {
    renderWithRouter(
      <AttachmentsWidgetHarness
        attachments={[IMAGE_ATTACHMENT, ZIP_ATTACHMENT]}
        onDownload={vi.fn()}
        onGetPreviewContent={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: `Download ${IMAGE_ATTACHMENT.filename}` }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: `Download ${ZIP_ATTACHMENT.filename}` }),
    ).toBeInTheDocument();
  });
});
