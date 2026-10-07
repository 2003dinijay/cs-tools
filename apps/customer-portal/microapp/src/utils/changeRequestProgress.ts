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

/**
 * What the change request pages read of the customer's part of the process, kept free of React and of any
 * component so the rules can be tested on their own (`changeRequestProgress.test.ts`).
 */

/**
 * The API state ids a change request is in once it has moved on from Customer Approval (id 5): Scheduled (-2),
 * Implement (-1), Review (0), Customer Review (1), Rollback (2), Closed (3) and Canceled (4).
 */
export const CHANGE_REQUEST_STATUS_IDS_PAST_CUSTOMER_APPROVAL: readonly string[] = [
  "-2",
  "-1",
  "0",
  "1",
  "2",
  "3",
  "4",
];

/**
 * Whether WSO2 accepted a time the customer proposed AND the change request has moved on because of it
 * (Scheduled or later). Accepting schedules the change, so that is where it stands; the customer's approval flag
 * stays false (no staff action records a customer's approval: the proposal was their own consent).
 *
 * An `agreed` answer is only the answer WSO2 once gave. Nothing clears it when the customers are asked again, so a
 * change that is (back) in Customer Approval, or in a state not known here, with `agreed` standing was NOT
 * scheduled by that acceptance and is waiting for the customer's own answer: reading it as accepted would tell
 * the customer there is nothing left for them to do.
 */
export function isProposedTimeAccepted(answer: string | null | undefined, stateId: string | null | undefined): boolean {
  return answer === "agreed" && !!stateId && CHANGE_REQUEST_STATUS_IDS_PAST_CUSTOMER_APPROVAL.includes(stateId);
}
