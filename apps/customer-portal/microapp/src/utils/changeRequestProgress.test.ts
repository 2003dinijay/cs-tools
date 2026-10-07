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

import { describe, expect, it } from "vitest";
import { CHANGE_REQUEST_STATUS_IDS_PAST_CUSTOMER_APPROVAL, isProposedTimeAccepted } from "./changeRequestProgress";

// The API state ids of a change request (the customer portal's own): New -5, Assess -4, Authorize -3, Customer Approval 5,
// Scheduled -2, Implement -1, Review 0, Customer Review 1, Rollback 2, Closed 3, Canceled 4.
const BEFORE_OR_AT_CUSTOMER_APPROVAL = ["-5", "-4", "-3", "5"];
const PAST_CUSTOMER_APPROVAL = ["-2", "-1", "0", "1", "2", "3", "4"];

describe("isProposedTimeAccepted", () => {
  it("is true when WSO2 agreed and the change request has moved on from Customer Approval, in each state after it", () => {
    expect([...CHANGE_REQUEST_STATUS_IDS_PAST_CUSTOMER_APPROVAL]).toEqual(PAST_CUSTOMER_APPROVAL);
    for (const stateId of PAST_CUSTOMER_APPROVAL) expect(isProposedTimeAccepted("agreed", stateId), stateId).toBe(true);
  });

  // An `agreed` answer is not cleared when the customers are asked again: back in Customer Approval, Approve and Reject are live.
  it("is false while the change request is in Customer Approval, or before it, whatever answer stands", () => {
    for (const stateId of BEFORE_OR_AT_CUSTOMER_APPROVAL)
      expect(isProposedTimeAccepted("agreed", stateId), stateId).toBe(false);
  });

  it("is false for a state that is not known, or none at all", () => {
    for (const stateId of ["", "99", "abc", null, undefined])
      expect(isProposedTimeAccepted("agreed", stateId), String(stateId)).toBe(false);
  });

  it("is false for every other answer, or no proposal, in any state", () => {
    for (const answer of ["pending", "disagreed", "unanswered", "", "AGREED", null, undefined]) {
      for (const stateId of [...BEFORE_OR_AT_CUSTOMER_APPROVAL, ...PAST_CUSTOMER_APPROVAL]) {
        expect(isProposedTimeAccepted(answer, stateId), `${String(answer)} in ${stateId}`).toBe(false);
      }
    }
  });
});
