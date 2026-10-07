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

// SkipTotal on the five searches global search uses (cases, incidents, change
// requests, problems, conversations): the COUNT is not run, the total is
// reported as TotalNotComputed, and the page is exactly the one the same search
// returns without it. Runs the real repositories on a real database and records
// the statements they send. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run SearchSkipTotal

package repository_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// statementRecorder is a pgx tracer that remembers every statement sent, except
// the identity-setting ones Scoped queues ahead of each query.
type statementRecorder struct {
	mu   sync.Mutex
	sqls []string
}

func (r *statementRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	r.add(d.SQL)
	return ctx
}
func (r *statementRecorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (r *statementRecorder) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	return ctx
}
func (r *statementRecorder) TraceBatchQuery(_ context.Context, _ *pgx.Conn, d pgx.TraceBatchQueryData) {
	r.add(d.SQL)
}
func (r *statementRecorder) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}

func (r *statementRecorder) add(sql string) {
	if strings.Contains(sql, "set_config") {
		return
	}
	r.mu.Lock()
	r.sqls = append(r.sqls, sql)
	r.mu.Unlock()
}

func (r *statementRecorder) reset() {
	r.mu.Lock()
	r.sqls = nil
	r.mu.Unlock()
}

// counts returns how many COUNT statements were sent.
func (r *statementRecorder) counts() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, s := range r.sqls {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "SELECT COUNT(*)") {
			n++
		}
	}
	return n
}

func tracedPool(t *testing.T) (*pgxpool.Pool, *statementRecorder) {
	t.Helper()
	dsn := os.Getenv("CASE_STATS_TEST_DSN")
	if dsn == "" {
		t.Skip("CASE_STATS_TEST_DSN not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	rec := &statementRecorder{}
	cfg.ConnConfig.Tracer = rec
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, rec
}

func TestSearchSkipTotalIntegration(t *testing.T) {
	pool, rec := tracedPool(t)
	scoped := repository.NewScoped(pool)
	scope := repository.SearchScope{Unrestricted: true, ViewerEmail: "skip-total-test@wso2.com"}
	ctx := repository.WithCallerIdentity(context.Background(), scope)
	page := domain.Pagination{Limit: 5}

	cases := repository.NewCaseRepository(scoped)
	incidents := repository.NewIncidentRepository(scoped)
	changeRequests := repository.NewChangeRequestRepository(scoped)
	problems := repository.NewProblemRepository(scoped)
	conversations := repository.NewConversationRepository(scoped)

	searches := []struct {
		name string
		run  func(skip bool) (page any, total int, err error)
	}{
		{"cases", func(skip bool) (any, int, error) {
			return cases.SearchCases(ctx, domain.SearchCasesRequest{
				SortBy:     domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc},
				Pagination: page, SkipTotal: skip,
			}, scope)
		}},
		{"incidents", func(skip bool) (any, int, error) {
			return incidents.SearchIncidents(ctx, domain.SearchIncidentsRequest{
				SortBy:     domain.IncidentSort{Field: domain.IncidentSortFieldUpdatedOn, Order: domain.IncidentSortOrderDesc},
				Pagination: page, SkipTotal: skip,
			}, nil, nil, nil, nil, nil, nil, nil, nil)
		}},
		{"change requests", func(skip bool) (any, int, error) {
			return changeRequests.SearchChangeRequests(ctx, domain.SearchChangeRequestsRequest{
				SortBy:     domain.ChangeRequestSort{Field: domain.ChangeRequestSortFieldUpdatedOn, Order: domain.ChangeRequestSortOrderDesc},
				Pagination: page, SkipTotal: skip,
			}, nil, nil, nil, nil)
		}},
		{"problems", func(skip bool) (any, int, error) {
			return problems.SearchProblems(ctx, domain.SearchProblemsRequest{Pagination: page, SkipTotal: skip}, nil, nil, nil)
		}},
		{"conversations", func(skip bool) (any, int, error) {
			return conversations.SearchConversations(ctx, domain.SearchConversationsRequest{
				SortBy:     domain.ConversationSort{Field: domain.ConversationSortFieldCreatedOn, Order: domain.ConversationSortOrderDesc},
				Pagination: page, SkipTotal: skip,
			}, "")
		}},
	}

	for _, s := range searches {
		t.Run(s.name, func(t *testing.T) {
			rec.reset()
			withTotal, total, err := s.run(false)
			if err != nil {
				t.Fatalf("search with the count: %v", err)
			}
			if n := rec.counts(); n != 1 {
				t.Errorf("a normal search sent %d COUNT statements, want 1", n)
			}
			if total < 0 {
				t.Errorf("a normal search reported total %d", total)
			}

			rec.reset()
			skipped, skippedTotal, err := s.run(true)
			if err != nil {
				t.Fatalf("search without the count: %v", err)
			}
			if n := rec.counts(); n != 0 {
				t.Errorf("a SkipTotal search sent %d COUNT statements, want 0", n)
			}
			if skippedTotal != domain.TotalNotComputed {
				t.Errorf("a SkipTotal search reported total %d, want %d", skippedTotal, domain.TotalNotComputed)
			}

			a, _ := json.Marshal(withTotal)
			b, _ := json.Marshal(skipped)
			if string(a) != string(b) {
				t.Errorf("the page differs with SkipTotal:\n  with count:    %s\n  without count: %s", a, b)
			}
		})
	}
}
