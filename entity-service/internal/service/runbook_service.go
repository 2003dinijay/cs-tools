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
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	maxRunbookAlertNameLength = 1000
	maxRunbookURLLength       = 2000
	maxRunbookImportEntries   = 5000
)

type runbookService struct {
	repo repository.RunbookRepository
}

// NewRunbookService constructs a RunbookService.
func NewRunbookService(repo repository.RunbookRepository) RunbookService {
	return &runbookService{repo: repo}
}

func (s *runbookService) List(ctx context.Context) (domain.RunbookList, error) {
	rbs, err := s.repo.List(ctx)
	if err != nil {
		return domain.RunbookList{}, err
	}
	return domain.RunbookList{Runbooks: rbs}, nil
}

func (s *runbookService) Create(ctx context.Context, req domain.SaveRunbookRequest) (domain.Runbook, error) {
	clean, err := normalizeRunbook(req)
	if err != nil {
		return domain.Runbook{}, err
	}
	return s.repo.Create(ctx, actorOf(ctx), clean)
}

func (s *runbookService) Update(ctx context.Context, id string, req domain.SaveRunbookRequest) (domain.Runbook, error) {
	if err := validateRunbookID(id); err != nil {
		return domain.Runbook{}, err
	}
	clean, err := normalizeRunbook(req)
	if err != nil {
		return domain.Runbook{}, err
	}
	rb, err := s.repo.Update(ctx, actorOf(ctx), id, clean)
	if err != nil {
		return domain.Runbook{}, err
	}
	if rb == nil {
		return domain.Runbook{}, &apierror.NotFoundError{Msg: "runbook not found"}
	}
	return *rb, nil
}

func (s *runbookService) Delete(ctx context.Context, id string) error {
	if err := validateRunbookID(id); err != nil {
		return err
	}
	ok, err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return &apierror.NotFoundError{Msg: "runbook not found"}
	}
	return nil
}

func (s *runbookService) Import(ctx context.Context, req domain.ImportRunbooksRequest) (domain.ImportRunbooksResponse, error) {
	if len(req.Runbooks) == 0 {
		return domain.ImportRunbooksResponse{}, &apierror.ValidationError{Msg: "runbooks must have at least one entry"}
	}
	if len(req.Runbooks) > maxRunbookImportEntries {
		return domain.ImportRunbooksResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("runbooks must have at most %d entries", maxRunbookImportEntries)}
	}
	// Later entries win over earlier ones with the same alert name, so one statement never touches a row twice.
	order := []string{}
	byName := map[string]domain.SaveRunbookRequest{}
	for i, r := range req.Runbooks {
		clean, err := normalizeRunbook(r)
		if err != nil {
			return domain.ImportRunbooksResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("runbooks[%d]: %s", i, err.Error())}
		}
		key := strings.ToLower(clean.AlertName)
		if _, seen := byName[key]; !seen {
			order = append(order, key)
		}
		byName[key] = clean
	}
	entries := make([]domain.SaveRunbookRequest, 0, len(order))
	for _, key := range order {
		entries = append(entries, byName[key])
	}
	return s.repo.Import(ctx, actorOf(ctx), entries)
}

func validateRunbookID(id string) error {
	if !uuidRe.MatchString(id) {
		return &apierror.ValidationError{Msg: "id must be a UUID"}
	}
	return nil
}

// normalizeRunbook trims both fields, turns a blank link into nil and checks what is left.
func normalizeRunbook(req domain.SaveRunbookRequest) (domain.SaveRunbookRequest, error) {
	name := strings.TrimSpace(req.AlertName)
	if name == "" {
		return req, &apierror.ValidationError{Msg: "alertName is required"}
	}
	if utf8.RuneCountInString(name) > maxRunbookAlertNameLength {
		return req, &apierror.ValidationError{Msg: fmt.Sprintf("alertName must be at most %d characters", maxRunbookAlertNameLength)}
	}
	out := domain.SaveRunbookRequest{AlertName: name}
	if req.RunbookURL != nil {
		link := strings.TrimSpace(*req.RunbookURL)
		if link != "" {
			if err := validateRunbookURL(link); err != nil {
				return req, err
			}
			out.RunbookURL = &link
		}
	}
	return out, nil
}

func validateRunbookURL(link string) error {
	if len(link) > maxRunbookURLLength {
		return &apierror.ValidationError{Msg: fmt.Sprintf("runbookUrl must be at most %d characters", maxRunbookURLLength)}
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return &apierror.ValidationError{Msg: "runbookUrl must be a full http(s) link"}
	}
	return nil
}
