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

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Project SLA status values, spelled exactly as ServiceNow's
// PROJECT_SLA_CONFIG -- the portal matches on the string.
const (
	projectSLAStatusAllGood        = "All Good"
	projectSLAStatusNeedsAttention = "Needs Attention"
)

// caseStatsOutstandingStates are the states that count as outstanding for a
// case-like work item: every one except CLOSED. Identical to the active set,
// which is ServiceNow's own "ACTIVE === OUTSTANDING per business
// requirement" (see projectCaseStatsService).
var caseStatsOutstandingStates = []string{
	"OPEN", "WORK_IN_PROGRESS", "AWAITING_INFO", "WAITING_ON_WSO2", "REOPENED", "SOLUTION_PROPOSED",
}

// Change-request state groupings, mirroring ServiceNow's CR_* constants.
// change_request_state_enum's labels (migration 000047) match them one for
// one, so no key translation is needed -- unlike case severity.
//
// Unlike cases, a change request's active and outstanding sets genuinely
// differ: the three earliest states (NEW/ASSESS/AUTHORIZE) are active but
// not yet outstanding.
var (
	crActiveStates         = []string{"NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW"}
	crOutstandingStates    = []string{"CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW"}
	crActionRequiredStates = []string{"CUSTOMER_APPROVAL", "CUSTOMER_REVIEW"}
)

const crResolvedState = "CLOSED"

// conversationActiveStates mirrors ServiceNow's CHAT_ACTIVE_STATE_VALUES.
// conversation_state_enum (migration 000057) carries the same vocabulary.
var conversationActiveStates = []string{"OPEN", "ACTIVE"}

// projectStatsService is the Postgres-backed ProjectStatsService: the whole
// seven-method interface, so routes.go can pick one implementation for both
// data sources instead of registering a subset.
//
// GetProjectMetadata and GetProjectCaseStats already had their own Postgres
// implementations (they were split out when only they were portable); this
// composes them rather than duplicating either.
//
// Two ServiceNow behaviours have no Postgres equivalent and are documented at
// the code that would otherwise reproduce them: the project-type-derived
// allowed-severity narrowing (no feature-entitlement table exists), and the
// units question on logged time (see GetProjectStats).
type projectStatsService struct {
	repo      repository.ProjectStatsRepository
	refRepo   repository.ReferenceDataRepository
	metadata  ProjectMetadataService
	caseStats ProjectCaseStatsService
}

// NewProjectStatsService constructs a Postgres-backed ProjectStatsService.
func NewProjectStatsService(
	repo repository.ProjectStatsRepository,
	refRepo repository.ReferenceDataRepository,
	metadata ProjectMetadataService,
	caseStats ProjectCaseStatsService,
) ProjectStatsService {
	return &projectStatsService{repo: repo, refRepo: refRepo, metadata: metadata, caseStats: caseStats}
}

// GetProjectMetadata implements ProjectStatsService by delegation.
func (s *projectStatsService) GetProjectMetadata(ctx context.Context, projectID string) (domain.ProjectMetadataResponse, error) {
	return s.metadata.GetProjectMetadata(ctx, projectID)
}

// GetProjectCaseStats implements ProjectStatsService by delegation.
func (s *projectStatsService) GetProjectCaseStats(ctx context.Context, projectID string, req domain.ProjectCaseStatsRequest) (domain.ProjectCaseStatsResponse, error) {
	return s.caseStats.GetProjectCaseStats(ctx, projectID, req)
}

// requireProject validates the id and confirms the project exists, so every
// method below reports a missing project as a 404 rather than empty stats.
func (s *projectStatsService) requireProject(ctx context.Context, projectID string) error {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return err
	}
	found, _, err := s.refRepo.GetProjectByID(ctx, projectID)
	if err != nil {
		return err
	}
	if !found {
		return &apierror.NotFoundError{Msg: "project not found"}
	}
	return nil
}

// GetProjectStats implements ProjectStatsService -- ServiceNow's
// getProjectStatistics.
func (s *projectStatsService) GetProjectStats(ctx context.Context, projectID string) (domain.ProjectStatsResponse, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return domain.ProjectStatsResponse{}, err
	}

	billableMinutes, nonBillableMinutes, err := s.repo.TimeLoggedMinutes(ctx, projectID, "", "")
	if err != nil {
		return domain.ProjectStatsResponse{}, err
	}

	deployments, err := s.repo.DeploymentCount(ctx, projectID)
	if err != nil {
		return domain.ProjectStatsResponse{}, err
	}
	deployedProducts, err := s.repo.DeployedProductCount(ctx, projectID)
	if err != nil {
		return domain.ProjectStatsResponse{}, err
	}

	// deployment_node is spelled with the live schema's column names, which a
	// migrations-built database does not have (see the repository's own
	// note). A failure here must not take down the whole dashboard, so the
	// count degrades to zero -- every other figure is still correct.
	instances, err := s.repo.InstanceCount(ctx, projectID)
	if err != nil {
		instances = 0
	}

	outstanding, err := s.repo.OutstandingCounts(ctx, projectID, caseStatsOutstandingStates, crOutstandingStates)
	if err != nil {
		return domain.ProjectStatsResponse{}, err
	}

	slaInputs, err := s.repo.SLAStatusInputs(ctx, projectID)
	if err != nil {
		return domain.ProjectStatsResponse{}, err
	}

	return domain.ProjectStatsResponse{
		// ServiceNow returns raw seconds in these two fields despite their
		// names (getProjectCaseTimeLogged sums time_card.total and assigns
		// it straight to totalHours/billableHours). That is not reproduced:
		// this schema stores per-activity MINUTES, and emitting minutes
		// under a field called hours would be a second, differently-wrong
		// number rather than parity. Real hours are returned; expect this
		// figure to differ from the ServiceNow data source until the units
		// there are confirmed.
		TotalHours:           minutesToHours(billableMinutes + nonBillableMinutes),
		BillableHours:        minutesToHours(billableMinutes),
		SLAStatus:            projectSLAStatus(slaInputs),
		DeploymentCount:      deployments,
		DeployedProductCount: deployedProducts,
		InstanceCount:        instances,
		OutstandingCount: domain.ProjectStatsOutstandingCount{
			CaseCount:           outstanding["case"],
			ServiceRequestCount: outstanding["service_request"],
			EngagementCount:     outstanding["engagement"],
			SraCount:            outstanding["security_report_analysis"],
			AnnouncementCount:   outstanding["announcement"],
			ChangeRequestCount:  outstanding["change_request"],
		},
	}, nil
}

// projectSLAStatus reduces the four conditions to the status string.
// ServiceNow checks them in sequence and returns Needs Attention on the first
// failure; since none has a side effect, evaluating all four and combining
// them is equivalent.
func projectSLAStatus(in repository.ProjectSLAStatusInputs) string {
	if in.HasOutstandingCase || !in.HasDeployedProduct || !in.HasActiveEndDate || !in.HasCustomerAdminContact {
		return projectSLAStatusNeedsAttention
	}
	return projectSLAStatusAllGood
}

// minutesToHours converts logged minutes to hours, rounded to two decimals.
func minutesToHours(minutes int) float64 {
	return roundToTwoDecimals(float64(minutes) / 60)
}

// GetProjectConversationStats implements ProjectStatsService -- ServiceNow's
// getProjectChatStats.
func (s *projectStatsService) GetProjectConversationStats(ctx context.Context, projectID, createdBy string) (domain.ProjectConversationStatsResponse, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return domain.ProjectConversationStatsResponse{}, err
	}

	labels, err := s.refRepo.EnumLabels(ctx, []string{conversationStateEnumType})
	if err != nil {
		return domain.ProjectConversationStatsResponse{}, fmt.Errorf("project conversation stats: %w", err)
	}

	resp := domain.ProjectConversationStatsResponse{
		StateCount: zeroedCounts(labels[conversationStateEnumType]),
	}

	rows, err := s.repo.ConversationStateCounts(ctx, projectID, createdBy)
	if err != nil {
		return domain.ProjectConversationStatsResponse{}, err
	}
	for _, row := range rows {
		resp.TotalCount += row.Count
		incrementCount(resp.StateCount, row.State, row.Count)
		if containsString(conversationActiveStates, row.State) {
			resp.ActiveCount += row.Count
		}
	}
	return resp, nil
}

// GetProjectDeploymentStats implements ProjectStatsService.
func (s *projectStatsService) GetProjectDeploymentStats(ctx context.Context, projectID string) (domain.ProjectDeploymentStatsResponse, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return domain.ProjectDeploymentStatsResponse{}, err
	}

	total, err := s.repo.DeploymentCount(ctx, projectID)
	if err != nil {
		return domain.ProjectDeploymentStatsResponse{}, err
	}

	// ServiceNow reads its last deployment from a separate
	// customer-project-deployment table; this schema has only deployment
	// itself, so the newest row's creation time is the equivalent.
	last, err := s.repo.LastDeploymentOn(ctx, projectID)
	if err != nil {
		return domain.ProjectDeploymentStatsResponse{}, err
	}
	var lastOn *string
	if last != nil {
		formatted := last.UTC().Format(time.RFC3339)
		lastOn = &formatted
	}

	return domain.ProjectDeploymentStatsResponse{TotalCount: total, LastDeploymentOn: lastOn}, nil
}

// GetProjectTimeCardStats implements ProjectStatsService. startDate/endDate
// are optional inclusive bounds on the time card's work date.
func (s *projectStatsService) GetProjectTimeCardStats(ctx context.Context, projectID, startDate, endDate string) (domain.ProjectTimeCardStatsResponse, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return domain.ProjectTimeCardStatsResponse{}, err
	}

	billable, nonBillable, err := s.repo.TimeLoggedMinutes(ctx, projectID, startDate, endDate)
	if err != nil {
		return domain.ProjectTimeCardStatsResponse{}, err
	}

	return domain.ProjectTimeCardStatsResponse{
		TotalHours:       minutesToHours(billable + nonBillable),
		BillableHours:    minutesToHours(billable),
		NonBillableHours: minutesToHours(nonBillable),
	}, nil
}

// GetProjectChangeRequestStats implements ProjectStatsService -- ServiceNow's
// getProjectChangeRequestStats.
func (s *projectStatsService) GetProjectChangeRequestStats(ctx context.Context, projectID string) (domain.ProjectChangeRequestStatsResponse, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, err
	}

	labels, err := s.refRepo.EnumLabels(ctx, []string{changeRequestStateEnumType})
	if err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, fmt.Errorf("project change request stats: %w", err)
	}

	resp := domain.ProjectChangeRequestStatsResponse{
		StateCount: zeroedCounts(labels[changeRequestStateEnumType]),
	}

	rows, err := s.repo.ChangeRequestStateCounts(ctx, projectID)
	if err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, err
	}
	for _, row := range rows {
		resp.TotalCount += row.Count
		incrementCount(resp.StateCount, row.State, row.Count)
		if containsString(crActiveStates, row.State) {
			resp.ActiveCount += row.Count
		}
		if containsString(crOutstandingStates, row.State) {
			resp.OutstandingCount += row.Count
		}
		if containsString(crActionRequiredStates, row.State) {
			resp.ActionRequiredCount += row.Count
		}
		if row.State == crResolvedState {
			resp.ResolvedCount.Total += row.Count
		}
	}

	currentMonth, pastThirtyDays, err := s.repo.ChangeRequestResolvedBuckets(ctx, projectID, crResolvedState)
	if err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, err
	}
	resp.ResolvedCount.CurrentMonth = currentMonth
	resp.ResolvedCount.PastThirtyDays = pastThirtyDays

	return resp, nil
}
