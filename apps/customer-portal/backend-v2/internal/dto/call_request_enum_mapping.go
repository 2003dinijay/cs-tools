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

package dto

// callRequestStateKeyToEnum mirrors entity-service's private
// callRequestKeyToState (internal/service/sn_call_request_service.go) —
// ServiceNow's own numeric choice-list key for each call-request state. The
// frontend was built against the old Ballerina backend, which forwarded
// this exact key as stateKey; it still sends it today
// (PATCH .../call-requests/{id}'s body, POST .../call-requests/search's
// filters.stateKeys) even though entity-service's own contract is the
// plain string enum (see domain.CallRequestStateType). If entity-service's
// copy ever changes, this one must be updated too.
var callRequestStateKeyToEnum = map[int]string{
	1: "pending_on_customer",
	2: "pending_on_wso2",
	3: "scheduled",
	4: "customer_rejected",
	5: "wso2_rejected",
	6: "canceled",
	7: "notes_pending",
	8: "concluded",
}
