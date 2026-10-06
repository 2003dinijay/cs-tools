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


import { type Locator, type Page } from "../fixtures/test";
import { CHANGE_REQUEST_DECISION } from "../utils/selectors";

/**
 * Page object for a change request's detail page
 * (`/projects/:projectId/operations/change-requests/:changeRequestId`), as far as a
 * customer answering it needs: the decision buttons, the banner they raise, and the
 * workflow stepper's "Current" marker.
 */
export class ChangeRequestDetailPage {
  constructor(private readonly page: Page) {}

  /** Customer Approval: "Approve". */
  approveButton(): Locator {
    return this.page.getByRole("button", {
      name: CHANGE_REQUEST_DECISION.approve,
      exact: true,
    });
  }

  /** Customer Approval: "Reject". */
  rejectButton(): Locator {
    return this.page.getByRole("button", {
      name: CHANGE_REQUEST_DECISION.reject,
      exact: true,
    });
  }

  /** Customer Approval: "Propose New Time". */
  proposeNewTimeButton(): Locator {
    return this.page.getByRole("button", {
      name: CHANGE_REQUEST_DECISION.proposeNewTime,
      exact: true,
    });
  }

  /** Customer Review: "Successful". */
  reviewSuccessfulButton(): Locator {
    return this.page.getByRole("button", {
      name: CHANGE_REQUEST_DECISION.reviewSuccessful,
      exact: true,
    });
  }

  /** Customer Review: "Unsuccessful". */
  reviewUnsuccessfulButton(): Locator {
    return this.page.getByRole("button", {
      name: CHANGE_REQUEST_DECISION.reviewUnsuccessful,
      exact: true,
    });
  }

  /** The success banner a decision raises. */
  banner(message: string): Locator {
    return this.page.getByText(message, { exact: true });
  }

  /**
   * The workflow stepper's row for the state the change request is in now: the
   * stage name next to its "Current" marker.
   */
  currentStage(): Locator {
    return this.page.getByText("Current", { exact: true }).locator("xpath=../..");
  }

  /** "Back to Change Requests". */
  backButton(): Locator {
    return this.page.getByRole("button", {
      name: CHANGE_REQUEST_DECISION.back,
      exact: true,
    });
  }
}
