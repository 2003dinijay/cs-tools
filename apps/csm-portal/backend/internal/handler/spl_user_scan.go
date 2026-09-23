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

package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

// salesEntityClient abstracts the sales-side entity GraphQL operations used
// by SplUserScanHandler.
type salesEntityClient interface {
	GetContactByEmail(ctx context.Context, email string) (*entity.Contact, error)
	GetSubscriptionByKey(ctx context.Context, subscriptionKey string) (*entity.Subscription, error)
}

// csEntityClient abstracts the CS-side entity GraphQL operations used by
// SplUserScanHandler.
type csEntityClient interface {
	GetUserByEmail(ctx context.Context, email string) (*entity.User, error)
	GetProjectByProjectKey(ctx context.Context, projectKey string) (*entity.Project, error)
	GetProjectContactByEmail(ctx context.Context, email, projectID string) (*entity.ProjectContact, error)
}

// The membership-type values a sales-side Contact's Memberships[i].Type may
// carry — mirrors Ballerina modules/types.bal's MembershipType enum
// (CUSTOMER = "OWN CONTACT", PARTNER = "PARTNER CONTACT").
const (
	splMembershipTypeCustomer = "OWN CONTACT"
	splMembershipTypePartner  = "PARTNER CONTACT"
)

// splAccountClassificationPartner mirrors Ballerina constants:PARTNER, an
// Account.Classification value.
const splAccountClassificationPartner = "Partner"

// splProjectStateOpen mirrors Ballerina modules/types.bal's ProjectState
// enum's OPEN value, compared against Project.WSO2ClosureState.
const splProjectStateOpen = "Open"

// splScanSystem* mirror Ballerina modules/types.bal's System enum.
const (
	splScanSystemSalesforce = "Salesforce"
	splScanSystemServicenow = "Servicenow"
)

// SplUserScanRequest is the request body for POST /spl/scan-user — mirrors
// Ballerina modules/types.bal's UserScanPayload.
type SplUserScanRequest struct {
	Email           string `json:"email"`
	SubscriptionKey string `json:"subscriptionKey"`
	IsPartner       bool   `json:"isPartner"`
}

// SplScanInformation is additional detail attached to a SplScanResult —
// mirrors Ballerina modules/types.bal's Information. Fields are omitted from
// the JSON response when unset, matching Ballerina's optional (?) fields.
type SplScanInformation struct {
	Issue         string `json:"issue,omitempty"`
	Solution      string `json:"solution,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	InvitationURL string `json:"invitationUrl,omitempty"`
}

// SplScanResult is one row of a scan-user system's results — mirrors
// Ballerina modules/types.bal's ScanResult.
type SplScanResult struct {
	Order       int                `json:"order"`
	Label       string             `json:"label"`
	State       bool               `json:"state"`
	Information SplScanInformation `json:"information"`
}

// SplScanResponse groups one system's SplScanResult rows — mirrors Ballerina
// modules/types.bal's Response (renamed to avoid colliding with this
// package's own response.go helpers).
type SplScanResponse struct {
	System       string          `json:"system"`
	SystemResult []SplScanResult `json:"systemResult"`
}

// splScanErrorBody mirrors Ballerina types:AppServerErrorResponse's body
// shape, used only for the upstream-lookup-failure branches of scan-user
// (distinct from every other /spl/* endpoint's ErrMsg* fallback, since this
// endpoint's Ballerina source returns its own bespoke message per lookup
// rather than a generic one).
type splScanErrorBody struct {
	Message string `json:"message"`
}

func writeSplScanError(w http.ResponseWriter, message string) {
	writeJSONValue(w, http.StatusInternalServerError, splScanErrorBody{Message: message})
}

// Static Information values ported verbatim from
// modules/constants/constants.bal — copy text, including the linked
// documentation URLs, must not be paraphrased.
var (
	splInfoContactNotFound = SplScanInformation{
		Issue:    "The contact not found in Salesforce.",
		Solution: "We need to add the contact in Salesforce.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"fw9vc1atxoc5",
	}
	splInfoSubscriptionNotFound = SplScanInformation{
		Issue:    "The subscription not found in the salesforce.",
		Solution: "We need to add the subscription in salesforce.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"b9o03r3icco2",
	}
	splInfoMembershipNotFound = SplScanInformation{
		Issue:    "The contact is not associated with any subscription.",
		Solution: "We need to connect the project contact with the subscription.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"rwkq1c6yzv6o",
	}
	splInfoMembershipNotFoundInSubscription = SplScanInformation{
		Issue:    "The contact is not associated with provided subscription.",
		Solution: "We need to check and bind the provided subscription with the contact.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"rwkq1c6yzv6o",
	}
	splInfoInvalidMembership = SplScanInformation{
		Issue: "Partner is not added in the customer subscription.",
		Solution: "Partners should be in the partner account. The partner account should have a partner relationship with " +
			"the customer account. After that, partners should be added as partner contacts in the customer subscription.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"rwkq1c6yzv6o",
	}
	splInfoInvalidCustomerMembership = SplScanInformation{
		Issue:    "Customer is added in the partner account.",
		Solution: "Customer should never be in the partner account. Customer should be added as a customer contact in the customer project.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"lm8ke89ptede",
	}
	splInfoSubscriptionNotFoundInAccount = SplScanInformation{
		Issue:    "The contact is not added in the correct account.",
		Solution: "The contact and the subscription are in two different accounts. It's important to verify that the project is associated with the correct account.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"b9o03r3icco2",
	}
	splInfoUserNotFound = SplScanInformation{
		Issue: "The user is not in the Servicenow.",
		Solution: "First check the relevant contact is there in the Salesforce or not. If not exists add the contact to " +
			"the Salesforce. Otherwise talk to Digi-Ops team.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h.rwkq1c6yzv6o",
	}
	splInfoUserLockedOutDocumentation = "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h.rwkq1c6yzv6o"
	splInfoProjectNotFound            = SplScanInformation{
		Issue: "The project is not in the Servicenow.",
		Solution: "First check the relevant subscription is there in the Salesforce or not. If not exist add the " +
			"subscription the Salesforce. Otherwise talk to Digi-Ops team.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h.b9o03r3icco2",
	}
	splInfoUserNotFoundInProject = SplScanInformation{
		Issue:    "The project is not in the Servicenow. Due to that, Project contact is not in the Servicenow.",
		Solution: "First add the project and after that add the project contact.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"rwkq1c6yzv6o",
	}
)

// SplUserScanHandler handles HTTP requests for the user-scan diagnostic
// tool, cross-referencing the sales-side (Salesforce) and CS-side
// (ServiceNow) entity services.
type SplUserScanHandler struct {
	sales         salesEntityClient
	cs            csEntityClient
	allowedGroups []string
}

// NewSplUserScanHandler creates a SplUserScanHandler backed by the given
// sales-side and CS-side entity clients. allowedGroups is SupportPortalLite's
// blanket access-gate group list (SPL_ALLOWED_GROUPS).
func NewSplUserScanHandler(sales salesEntityClient, cs csEntityClient, allowedGroups []string) *SplUserScanHandler {
	return &SplUserScanHandler{sales: sales, cs: cs, allowedGroups: allowedGroups}
}

// ScanUser handles POST /spl/scan-user — ported verbatim (business logic,
// copy text and documentation links included) from Ballerina service.bal's
// `post scan\-user` resource function. See that function for the
// authoritative behavior; comments below reference its structure.
func (h *SplUserScanHandler) ScanUser(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var payload SplUserScanRequest
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	ctx := r.Context()

	// ----- Salesforce-side: contact / subscription / membership -----

	contactResult := SplScanResult{Order: 1, Label: "Contact Details"}
	subscriptionResult := SplScanResult{Order: 2, Label: "Subscription Contact Details"}
	membershipResult := SplScanResult{Order: 3, Label: "Membership Details"}

	contact, err := h.sales.GetContactByEmail(ctx, payload.Email)
	if err != nil {
		slog.ErrorContext(ctx, "sales entity GetContactByEmail failed", "userID", user.UserID, "err", err)
		writeSplScanError(w, "Error occurred when retrieving contact information")
		return
	}

	subscription, err := h.sales.GetSubscriptionByKey(ctx, payload.SubscriptionKey)
	if err != nil {
		slog.ErrorContext(ctx, "sales entity GetSubscriptionByKey failed", "userID", user.UserID, "err", err)
		writeSplScanError(w, "Error occurred when retrieving subscription information")
		return
	}

	if contact == nil {
		contactResult.Information = splInfoContactNotFound
		membershipResult.Information = splInfoMembershipNotFound
		if subscription == nil {
			subscriptionResult.Information = splInfoSubscriptionNotFound
		} else {
			subscriptionResult.State = true
		}
	} else {
		contactResult.State = true

		if len(contact.Memberships) == 0 {
			membershipResult.Information = splInfoMembershipNotFound
		}
		if subscription == nil {
			subscriptionResult.Information = splInfoSubscriptionNotFound
			membershipResult.Information = splInfoMembershipNotFound
		} else {
			subscriptionResult.State = true
			subscriptionID := subscription.ID

			var subscriptionMembership []entity.ContactMembership
			for _, m := range contact.Memberships {
				if m.SubscriptionID == subscriptionID {
					subscriptionMembership = append(subscriptionMembership, m)
				}
			}

			if len(subscriptionMembership) == 0 {
				membershipResult.Information = splInfoMembershipNotFoundInSubscription
			}
			contactAccountID := contact.Account.ID

			if contactAccountID != subscription.CustomerID {
				if !payload.IsPartner {
					subscriptionResult.Information = splInfoSubscriptionNotFoundInAccount
				}
			}

			if subscription.ID != "" && len(subscriptionMembership) > 0 {
				if contact.Account.Classification == splAccountClassificationPartner {
					if (!payload.IsPartner && subscriptionMembership[0].Type == splMembershipTypeCustomer) ||
						(payload.IsPartner && subscriptionMembership[0].Type == splMembershipTypePartner) {
						membershipResult.State = true
					} else {
						membershipResult.Information = splInfoInvalidMembership
					}
				} else {
					if !payload.IsPartner && subscriptionMembership[0].Type == splMembershipTypeCustomer {
						membershipResult.State = true
					} else {
						membershipResult.Information = splInfoInvalidCustomerMembership
					}
				}
			}
		}
	}

	salesResponse := []SplScanResult{contactResult, subscriptionResult, membershipResult}

	// ----- ServiceNow-side: user lock state / project closure state -----

	userStateResult := SplScanResult{Order: 1, Label: "Accept the invitation"}
	projectStateResult := SplScanResult{Order: 2, Label: "Project closure state"}

	snUser, err := h.cs.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		slog.ErrorContext(ctx, "cs entity GetUserByEmail failed", "userID", user.UserID, "err", err)
		writeSplScanError(w, "Error occurred when retrieving user information")
		return
	}

	project, err := h.cs.GetProjectByProjectKey(ctx, payload.SubscriptionKey)
	if err != nil {
		slog.ErrorContext(ctx, "cs entity GetProjectByProjectKey failed", "userID", user.UserID, "err", err)
		writeSplScanError(w, "Error occurred when retrieving project information")
		return
	}

	var projectID string
	if project == nil {
		projectStateResult.Information = splInfoProjectNotFound
		userStateResult.State = false
		userStateResult.Information = splInfoUserNotFoundInProject
	} else {
		projectID = project.ProjectID
		if project.WSO2ClosureState != splProjectStateOpen {
			projectStateResult.Information = SplScanInformation{
				Issue: "The project is not in open state. The project is in " + project.WSO2ClosureState +
					" state. The project should be in the Open state.",
				Solution: "Need to reopen this project for getting the uninterpreted support.",
			}
		} else {
			projectStateResult.State = true
		}

		if snUser == nil {
			userStateResult.Information = splInfoUserNotFound
		} else if snUser.LockedOut {
			// getProjectContactByEmail is called unconditionally here, mirroring
			// the Ballerina source, which does not guard it on projectID being set.
			var userInvitationURL string
			projectContact, err := h.cs.GetProjectContactByEmail(ctx, payload.Email, projectID)
			if err != nil {
				slog.ErrorContext(ctx, "cs entity GetProjectContactByEmail failed", "userID", user.UserID, "err", err)
				writeSplScanError(w, "Error occurred when retrieving project contact information")
				return
			}
			if projectContact == nil {
				userInvitationURL = "No invitation url found for the given email"
			} else if projectContact.InvitationURL != nil {
				userInvitationURL = *projectContact.InvitationURL
			}
			// else: InvitationURL is nil — userInvitationURL stays "", matching
			// Ballerina's userInvitationUrl = projectContact?.invitationUrl (also nil
			// in that case). NOTE: Ballerina's subsequent `if userInvitationUrl == ""`
			// check is technically false when the value is nil rather than the
			// literal empty string, so it falls into the "else" (non-empty) branch
			// even though there is no URL to show — this looks like an unintentional
			// quirk in the original rather than deliberate behavior. This port
			// instead treats a nil/absent InvitationURL as equivalent to empty,
			// taking the "empty" branch below, since that is what a reader would
			// reasonably expect "no invitation URL" to do. Flagging this as a
			// deliberate behavioral deviation from the literal Ballerina source.
			info := SplScanInformation{Issue: "The user didn't accept the invitation."}
			if userInvitationURL == "" {
				info.Solution = "The user invitation is empty. You need to resend the invitation."
			} else {
				info.Solution = "You need to inform the user to accept the invitation."
				info.InvitationURL = userInvitationURL
			}
			info.Documentation = splInfoUserLockedOutDocumentation
			userStateResult.Information = info
		} else {
			userStateResult.State = true
		}
	}

	csResponse := []SplScanResult{userStateResult, projectStateResult}

	writeJSONValue(w, http.StatusOK, []SplScanResponse{
		{System: splScanSystemSalesforce, SystemResult: salesResponse},
		{System: splScanSystemServicenow, SystemResult: csResponse},
	})
}
