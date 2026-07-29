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

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import UserRefLink from "@components/UserRefLink";

describe("UserRefLink", () => {
  it("renders a link to the profile page when an email is present", () => {
    render(
      <MemoryRouter>
        <UserRefLink name="Jane Doe" email="jane.doe@example.com" />
      </MemoryRouter>,
    );
    const link = screen.getByRole("link", { name: "Jane Doe" });
    expect(link).toHaveAttribute(
      "href",
      `/people/${encodeURIComponent("jane.doe@example.com")}`,
    );
  });

  it("URL-encodes the email in the href", () => {
    render(
      <MemoryRouter>
        <UserRefLink name="Jane Doe" email="jane+test@example.com" />
      </MemoryRouter>,
    );
    const link = screen.getByRole("link", { name: "Jane Doe" });
    expect(link).toHaveAttribute(
      "href",
      "/people/jane%2Btest%40example.com",
    );
  });

  it("renders plain text (no link) when there's no email", () => {
    render(
      <MemoryRouter>
        <UserRefLink name="Jane Doe" />
      </MemoryRouter>,
    );
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("renders plain text when the email is an empty/whitespace string", () => {
    render(
      <MemoryRouter>
        <UserRefLink name="Jane Doe" email="   " />
      </MemoryRouter>,
    );
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
