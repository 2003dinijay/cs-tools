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
	"net/url"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// TeamService defines the operations available on the team entity. Postgres
// only -- there is no ServiceNow-backed counterpart of this service in this
// codebase; SPL's own ABT-team-members feature has always called ServiceNow
// directly from csm-portal-backend rather than through entity-service, so
// this is a new capability, not a migrated one.
type TeamService interface {
	// GetTeamMembers returns the roster of the given team. A NotFoundError is
	// returned when no team with that id exists; an empty roster on an
	// existing team is not an error.
	GetTeamMembers(ctx context.Context, teamID string) (domain.GetTeamMembersResponse, error)
	// SearchTeams returns a paginated list of teams from the `team` table,
	// filtered by an optional name search.
	SearchTeams(ctx context.Context, req domain.SearchTeamsRequest) (domain.SearchTeamsResponse, error)
	// ListChatWebhooks returns every team's own configured Chat webhook URL
	// (team.chat_webhook_url, migration 0240) -- internal-caller-only, since
	// a webhook URL is itself a bearer credential (see RequireInternalCaller
	// below). Reference data for a backend service (csm-notification-service,
	// fetched periodically), not something any portal caller has a
	// legitimate use for -- same reasoning as SLADurationPolicyService.
	ListChatWebhooks(ctx context.Context) (domain.ListTeamChatWebhooksResponse, error)
	// GetChatWebhook returns one team's own configured Chat webhook URL (nil
	// when unset) -- internal-caller-only, same reasoning as ListChatWebhooks.
	// A NotFoundError is returned when no team has this id.
	GetChatWebhook(ctx context.Context, teamID string) (domain.TeamChatWebhookDetail, error)
	// SetChatWebhook sets (non-empty url) or clears (empty url) the team's
	// own Chat webhook URL -- internal-caller-only. A ValidationError is
	// returned for a malformed id or a non-empty url that is not a
	// well-formed https:// URL; a NotFoundError when no team has this id.
	SetChatWebhook(ctx context.Context, teamID, url string) (domain.TeamChatWebhookDetail, error)
	// AssignTeamLead makes userID the given team's one 'lead' -- see
	// repository.TeamRepository.AssignTeamLead's own doc comment for what
	// this does and why it matters beyond this feature (the incident
	// call-escalation ladder reads the same role). Internal-caller-only.
	AssignTeamLead(ctx context.Context, teamID, userID string) error
}

type teamService struct {
	repo   repository.TeamRepository
	access AccessService
}

// NewTeamService constructs a TeamService backed by the given repository.
// access gates ListChatWebhooks to internal callers only.
func NewTeamService(repo repository.TeamRepository, access AccessService) TeamService {
	return &teamService{repo: repo, access: access}
}

func (s *teamService) GetTeamMembers(ctx context.Context, teamID string) (domain.GetTeamMembersResponse, error) {
	// The roster includes member names and emails, so an anonymous caller must
	// not reach it -- auth.Middleware itself lets a tokenless request through
	// (per-endpoint decides), so this check is the only thing standing between
	// the roster and an unauthenticated caller.
	if middleware.UserIDTokenFromContext(ctx) == "" {
		return domain.GetTeamMembersResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	if err := validateUUIDs("id", []string{teamID}); err != nil {
		return domain.GetTeamMembersResponse{}, err
	}
	exists, err := s.repo.TeamExists(ctx, teamID)
	if err != nil {
		return domain.GetTeamMembersResponse{}, err
	}
	if !exists {
		return domain.GetTeamMembersResponse{}, &apierror.NotFoundError{Msg: "team not found"}
	}
	rows, err := s.repo.GetTeamMembers(ctx, teamID)
	if err != nil {
		return domain.GetTeamMembersResponse{}, err
	}

	members := make([]domain.TeamMember, 0, len(rows))
	for _, row := range rows {
		role := row.Role
		members = append(members, domain.TeamMember{
			ID:    row.UserID,
			Name:  row.Name,
			Email: row.Email,
			Role:  &role,
		})
	}
	return domain.GetTeamMembersResponse{Members: members}, nil
}

func (s *teamService) SearchTeams(ctx context.Context, req domain.SearchTeamsRequest) (domain.SearchTeamsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchTeamsResponse{}, err
	}
	searchQuery := ""
	if req.Filters != nil {
		if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
			return domain.SearchTeamsResponse{}, err
		}
		searchQuery = req.Filters.SearchQuery
	}

	teams, total, err := s.repo.SearchTeams(ctx, searchQuery, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchTeamsResponse{}, err
	}
	return domain.SearchTeamsResponse{
		Teams:  teams,
		Total:  total,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}

func (s *teamService) ListChatWebhooks(ctx context.Context) (domain.ListTeamChatWebhooksResponse, error) {
	if err := RequireInternalCaller(ctx, s.access, "team chat webhooks are only available to internal services"); err != nil {
		return domain.ListTeamChatWebhooksResponse{}, err
	}
	webhooks, err := s.repo.ListChatWebhooks(ctx)
	if err != nil {
		return domain.ListTeamChatWebhooksResponse{}, err
	}
	return domain.ListTeamChatWebhooksResponse{Teams: webhooks}, nil
}

func (s *teamService) GetChatWebhook(ctx context.Context, teamID string) (domain.TeamChatWebhookDetail, error) {
	if err := RequireInternalCaller(ctx, s.access, "team chat webhooks are only available to internal services"); err != nil {
		return domain.TeamChatWebhookDetail{}, err
	}
	if err := validateUUIDs("id", []string{teamID}); err != nil {
		return domain.TeamChatWebhookDetail{}, err
	}
	url, err := s.repo.GetChatWebhookURL(ctx, teamID)
	if err != nil {
		return domain.TeamChatWebhookDetail{}, err
	}
	return domain.TeamChatWebhookDetail{ChatWebhookURL: url}, nil
}

// maxChatWebhookURLLen bounds team.chat_webhook_url (migration 0240, a plain
// TEXT column with no declared limit) -- a sanity cap, not a column-width
// constraint, so a caller can't accidentally write something absurd.
const maxChatWebhookURLLen = 2048

// allowedChatWebhookHost is the only host SetChatWebhook accepts. A webhook
// URL stored here is later fetched, unattended, by csm-notification-service,
// which POSTs a JSON payload to it on every Chat alert -- so accepting an
// arbitrary https:// URL (the original check here was only a prefix test)
// would let whoever can call this endpoint (the BFF lets a team's own lead,
// not just an Asgardeo admin, do so) turn that service into an open SSRF
// relay: point it at an internal service, a cloud metadata endpoint
// (169.254.169.254), or anything else reachable from that service's network,
// and have it make requests on the caller's behalf. Pinning to the one real
// Google Chat webhook host closes that off without needing a broader
// allowlist.
const allowedChatWebhookHost = "chat.googleapis.com"

// validateChatWebhookURL rejects anything but a well-formed
// https://chat.googleapis.com/... URL with no embedded userinfo (a
// "https://chat.googleapis.com@evil.example/..." trick) -- parsed structurally
// via net/url, not matched by prefix, so neither case-folding nor an
// embedded-credentials trick can smuggle a different host past it.
func validateChatWebhookURL(trimmed string) error {
	u, err := url.Parse(trimmed)
	if err != nil {
		return &apierror.ValidationError{Msg: "chatWebhookUrl must be a well-formed https:// URL"}
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if u.Scheme != "https" || u.User != nil || host != allowedChatWebhookHost {
		return &apierror.ValidationError{Msg: "chatWebhookUrl must be a well-formed https://" + allowedChatWebhookHost + " URL"}
	}
	return nil
}

func (s *teamService) SetChatWebhook(ctx context.Context, teamID, url string) (domain.TeamChatWebhookDetail, error) {
	if err := RequireInternalCaller(ctx, s.access, "team chat webhooks are only available to internal services"); err != nil {
		return domain.TeamChatWebhookDetail{}, err
	}
	if err := validateUUIDs("id", []string{teamID}); err != nil {
		return domain.TeamChatWebhookDetail{}, err
	}
	trimmed := strings.TrimSpace(url)
	var toStore *string
	if trimmed != "" {
		if len(trimmed) > maxChatWebhookURLLen {
			return domain.TeamChatWebhookDetail{}, &apierror.ValidationError{Msg: "chatWebhookUrl is too long"}
		}
		if err := validateChatWebhookURL(trimmed); err != nil {
			return domain.TeamChatWebhookDetail{}, err
		}
		toStore = &trimmed
	}
	if err := s.repo.SetChatWebhookURL(ctx, teamID, toStore); err != nil {
		return domain.TeamChatWebhookDetail{}, err
	}
	return domain.TeamChatWebhookDetail{ChatWebhookURL: toStore}, nil
}

func (s *teamService) AssignTeamLead(ctx context.Context, teamID, userID string) error {
	if err := RequireInternalCaller(ctx, s.access, "assigning a team lead is only available to internal services"); err != nil {
		return err
	}
	if err := validateUUIDs("id", []string{teamID}); err != nil {
		return err
	}
	if err := validateUUIDs("userId", []string{userID}); err != nil {
		return err
	}
	return s.repo.AssignTeamLead(ctx, teamID, userID)
}
