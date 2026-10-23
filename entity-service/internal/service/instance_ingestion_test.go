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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// mockInstanceIngestionRepo records what the service handed it.
type mockInstanceIngestionRepo struct {
	calls int

	nodeID     string
	meta       domain.InstanceNodeMeta
	info       domain.InstanceInfoRequest
	usage      domain.InstanceUsageIngestRequest
	batch      domain.DailyUsageBatchRequest
	reportedOn time.Time
	countedOn  time.Time

	secret    string
	secretErr error

	rolledDay                         string
	rolledYear                        int
	rolledMonth                       int
	unresolvedSince                   time.Time
	unresolvedLimit                   int
	unresolvedNodes                   []domain.UnresolvedProductNode
	importsProject, importsDeployment string
	importsLimit                      int
}

func (m *mockInstanceIngestionRepo) UpsertDeploymentNode(_ context.Context, nodeID string, meta domain.InstanceNodeMeta) (string, error) {
	m.calls++
	m.nodeID, m.meta = nodeID, meta
	return "node-uuid", nil
}

func (m *mockInstanceIngestionRepo) InsertDeploymentInformation(_ context.Context, req domain.InstanceInfoRequest, reportedOn time.Time) (string, error) {
	m.calls++
	m.info, m.reportedOn = req, reportedOn
	return "info-uuid", nil
}

func (m *mockInstanceIngestionRepo) InsertUsageStats(_ context.Context, req domain.InstanceUsageIngestRequest, countedOn time.Time) error {
	m.calls++
	m.usage, m.countedOn = req, countedOn
	return nil
}

func (m *mockInstanceIngestionRepo) ImportDailyUsagesBatch(_ context.Context, req domain.DailyUsageBatchRequest) (int, error) {
	m.calls++
	m.batch = req
	return len(req.Usages), nil
}

func (m *mockInstanceIngestionRepo) RollupDailyUsage(_ context.Context, day string) (int, error) {
	m.calls++
	m.rolledDay = day
	return 7, nil
}

func (m *mockInstanceIngestionRepo) RollupMonthlyUsage(_ context.Context, year, month int) (int, error) {
	m.calls++
	m.rolledYear, m.rolledMonth = year, month
	return 9, nil
}

func (m *mockInstanceIngestionRepo) ListUnresolvedProductNodes(_ context.Context, since time.Time, limit int) ([]domain.UnresolvedProductNode, error) {
	m.calls++
	m.unresolvedSince, m.unresolvedLimit = since, limit
	if m.unresolvedNodes != nil {
		return m.unresolvedNodes, nil
	}
	return []domain.UnresolvedProductNode{{NodeID: "n1", ReportedProduct: "wso2xx"}}, nil
}

func (m *mockInstanceIngestionRepo) ListUsageReportImports(_ context.Context, projectKey, deploymentNumber string, limit int) ([]domain.UsageReportImport, error) {
	m.calls++
	m.importsProject, m.importsDeployment, m.importsLimit = projectKey, deploymentNumber, limit
	return []domain.UsageReportImport{{ID: "i1", ImportedCount: 3}}, nil
}

func (m *mockInstanceIngestionRepo) GetProjectSecretKey(_ context.Context, _ string) (string, error) {
	m.calls++
	return m.secret, m.secretErr
}

func (m *mockInstanceIngestionRepo) GetDeploymentConsumptionSecret(_ context.Context, number string) (domain.DeploymentConsumptionSecretResponse, error) {
	m.calls++
	return domain.DeploymentConsumptionSecretResponse{DeploymentNumber: number}, m.secretErr
}

func newTestIngestionService(repo *mockInstanceIngestionRepo) *instanceIngestionService {
	return &instanceIngestionService{
		repo: repo,
		now:  func() time.Time { return time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC) },
	}
}

func wantValidation(t *testing.T, err error, contains string) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *apierror.ValidationError, got %T (%v)", err, err)
	}
	if !strings.Contains(ve.Msg, contains) {
		t.Errorf("message %q does not mention %q", ve.Msg, contains)
	}
}

func TestInstanceIngestion_MetaIsTrimmedAndPassedOn(t *testing.T) {
	repo := &mockInstanceIngestionRepo{}
	resp, err := newTestIngestionService(repo).UpsertInstanceMeta(context.Background(), domain.InstanceMetaRequest{
		NodeID: " node-1 ",
		InstanceNodeMeta: domain.InstanceNodeMeta{
			ProjectKey: " PROJ ", DeploymentNumber: " DEP001 ", Product: " wso2am ", Version: " 4.2.0 ",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "node-uuid" {
		t.Errorf("id = %q", resp.ID)
	}
	want := domain.InstanceNodeMeta{ProjectKey: "PROJ", DeploymentNumber: "DEP001", Product: "wso2am", Version: "4.2.0"}
	if repo.nodeID != "node-1" || repo.meta != want {
		t.Errorf("repo got node %q meta %+v, want node-1 %+v", repo.nodeID, repo.meta, want)
	}
}

// Products report a node id as a 64-character hash, not a dashed UUID, and the
// column holds 128, so both must be accepted and only the 129th character refused.
func TestInstanceIngestion_AcceptsHashSizedNodeIDs(t *testing.T) {
	for _, n := range []int{64, 128} {
		repo := &mockInstanceIngestionRepo{}
		node := strings.Repeat("a", n)
		if _, err := newTestIngestionService(repo).UpsertInstanceMeta(context.Background(), domain.InstanceMetaRequest{NodeID: node}); err != nil {
			t.Errorf("a %d-character node id was rejected: %v", n, err)
		}
		if repo.nodeID != node {
			t.Errorf("a %d-character node id did not reach the repository", n)
		}
	}
}

func TestInstanceIngestion_RejectsBeforeTheRepository(t *testing.T) {
	long := func(n int) string { return strings.Repeat("x", n) }
	ctx := context.Background()
	cases := []struct {
		name     string
		run      func(InstanceIngestionService) error
		contains string
	}{
		{"meta without node", func(s InstanceIngestionService) error {
			_, err := s.UpsertInstanceMeta(ctx, domain.InstanceMetaRequest{NodeID: "  "})
			return err
		}, "nodeId is required"},
		{"node id too long", func(s InstanceIngestionService) error {
			_, err := s.UpsertInstanceMeta(ctx, domain.InstanceMetaRequest{NodeID: long(129)})
			return err
		}, "nodeId must be at most 128"},
		{"deployment number too long", func(s InstanceIngestionService) error {
			_, err := s.UpsertInstanceMeta(ctx, domain.InstanceMetaRequest{NodeID: "n", InstanceNodeMeta: domain.InstanceNodeMeta{DeploymentNumber: long(51)}})
			return err
		}, "deploymentNumber"},
		{"usage without count type", func(s InstanceIngestionService) error {
			_, err := s.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "n"})
			return err
		}, "countType is required"},
		{"negative count", func(s InstanceIngestionService) error {
			_, err := s.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "n", CountType: "TPS", Count: -1})
			return err
		}, "count must be between"},
		{"count beyond INTEGER", func(s InstanceIngestionService) error {
			_, err := s.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "n", CountType: "TPS", Count: 1 << 31})
			return err
		}, "count must be between"},
		{"unreadable time", func(s InstanceIngestionService) error {
			_, err := s.InsertUsageStats(ctx, domain.InstanceUsageIngestRequest{NodeID: "n", CountType: "TPS", CreatedTime: "tomorrow"})
			return err
		}, "createdTime"},
		{"info time in no known layout", func(s InstanceIngestionService) error {
			_, err := s.InsertInstanceInfo(ctx, domain.InstanceInfoRequest{NodeID: "n", CreatedTime: "15/10/2026 10:00"})
			return err
		}, "createdTime"},
		{"batch without node", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{})
			return err
		}, "nodeId is required"},
		{"batch entry with a bad date", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{NodeID: "n", Usages: []domain.DailyUsageBatchItem{
				{Date: "2026-10-15", Type: "CORES", Value: 1},
				{Date: "2026-13-01", Type: "CORES", Value: 1},
			}})
			return err
		}, "usages[1].date"},
		{"batch entry with no type", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{NodeID: "n", Usages: []domain.DailyUsageBatchItem{{Date: "2026-10-15", Value: 1}}})
			return err
		}, "usages[0].type is required"},
		{"batch entry with a negative value", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{NodeID: "n", Usages: []domain.DailyUsageBatchItem{{Date: "2026-10-15", Type: "CORES", Value: -5}}})
			return err
		}, "usages[0].value"},
		{"batch node without id", func(s InstanceIngestionService) error {
			_, err := s.ImportDailyUsagesBatch(ctx, domain.DailyUsageBatchRequest{NodeID: "n", Nodes: []domain.DailyUsageBatchNode{{NodeID: " "}}})
			return err
		}, "nodes[0].nodeId is required"},
		{"deployment secret without number", func(s InstanceIngestionService) error {
			_, err := s.GetDeploymentConsumptionSecret(ctx, " ")
			return err
		}, "deployment number is required"},
		{"deployment number too long", func(s InstanceIngestionService) error {
			_, err := s.GetDeploymentConsumptionSecret(ctx, long(51))
			return err
		}, "deployment number must be at most 50"},
		{"secret without key", func(s InstanceIngestionService) error {
			_, err := s.GetProjectSecretKey(ctx, " ")
			return err
		}, "project key is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockInstanceIngestionRepo{}
			wantValidation(t, tc.run(newTestIngestionService(repo)), tc.contains)
			if repo.calls != 0 {
				t.Errorf("the repository was called %d times for a refused request", repo.calls)
			}
		})
	}
}

func TestInstanceIngestion_TimesAreReadAsUTC(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
	}{
		{"2026-10-15 23:30:00", time.Date(2026, 10, 15, 23, 30, 0, 0, time.UTC)},
		{"2026-10-16T05:00:00+05:30", time.Date(2026, 10, 15, 23, 30, 0, 0, time.UTC)},
		{"2026-10-15T23:30:00Z", time.Date(2026, 10, 15, 23, 30, 0, 0, time.UTC)},
		{"", time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		repo := &mockInstanceIngestionRepo{}
		if _, err := newTestIngestionService(repo).InsertUsageStats(context.Background(), domain.InstanceUsageIngestRequest{
			NodeID: "n", CountType: "TPS", Count: 3, CreatedTime: tc.in,
		}); err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !repo.countedOn.Equal(tc.want) || repo.countedOn.Location() != time.UTC {
			t.Errorf("%q: countedOn = %v, want %v (UTC)", tc.in, repo.countedOn, tc.want)
		}
	}
}

func TestInstanceIngestion_InfoWithoutHashIsHashedFromTheSnapshot(t *testing.T) {
	info := map[string]any{"jdkVersion": "17", "resourceProfile": map[string]any{"cpuCores": 8}}
	hashOf := func(v any) string {
		repo := &mockInstanceIngestionRepo{}
		if _, err := newTestIngestionService(repo).InsertInstanceInfo(context.Background(), domain.InstanceInfoRequest{NodeID: "n", DeploymentInfo: v}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return repo.info.Hash
	}
	a, b := hashOf(info), hashOf(info)
	if len(a) != 64 || a != b {
		t.Errorf("hash = %q / %q, want the same 64-character SHA-256 for the same snapshot", a, b)
	}
	if c := hashOf(map[string]any{"jdkVersion": "21"}); c == a {
		t.Error("a different snapshot got the same hash")
	}

	repo := &mockInstanceIngestionRepo{}
	if _, err := newTestIngestionService(repo).InsertInstanceInfo(context.Background(), domain.InstanceInfoRequest{NodeID: "n", Hash: " given ", DeploymentInfo: info}); err != nil {
		t.Fatal(err)
	}
	if repo.info.Hash != "given" {
		t.Errorf("a sent hash must be kept, got %q", repo.info.Hash)
	}
}

func TestInstanceIngestion_BatchIsTrimmedAndCounted(t *testing.T) {
	repo := &mockInstanceIngestionRepo{}
	resp, err := newTestIngestionService(repo).ImportDailyUsagesBatch(context.Background(), domain.DailyUsageBatchRequest{
		NodeID:    "node-1",
		UserEmail: " someone@example.com ",
		Usages: []domain.DailyUsageBatchItem{
			{Date: " 2026-10-15 ", Type: " CORES ", Value: 8, NodeID: " node-2 ", DeploymentNumber: " DEP9 "},
			{Date: "2026-10-15", Type: "TRANSACTION_COUNT", Value: 500},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ImportedCount != 2 || resp.Status != "SUCCESS" {
		t.Errorf("response = %+v", resp)
	}
	got := repo.batch.Usages[0]
	if got.Date != "2026-10-15" || got.Type != "CORES" || got.NodeID != "node-2" || got.DeploymentNumber != "DEP9" {
		t.Errorf("entry not trimmed: %+v", got)
	}
	if repo.batch.UserEmail != "someone@example.com" {
		t.Errorf("userEmail = %q", repo.batch.UserEmail)
	}
}

func TestInstanceIngestion_SecretKeyErrorsPassThrough(t *testing.T) {
	repo := &mockInstanceIngestionRepo{secretErr: &apierror.NotFoundError{Msg: "no key"}}
	_, err := newTestIngestionService(repo).GetProjectSecretKey(context.Background(), "PROJ")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("want NotFoundError, got %T", err)
	}

	repo = &mockInstanceIngestionRepo{secret: "s3cret"}
	resp, err := newTestIngestionService(repo).GetProjectSecretKey(context.Background(), " PROJ ")
	if err != nil || resp.SecretKey != "s3cret" {
		t.Fatalf("resp = %+v, err = %v", resp, err)
	}
}

func TestInstanceIngestion_RollupDailyValidatesTheDay(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ name, date, contains string }{
		{"not a date", "15/10/2026", "date must be a date"},
		{"empty", " ", "date must be a date"},
		{"tomorrow", "2026-10-17", "must not be in the future"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockInstanceIngestionRepo{}
			_, err := newTestIngestionService(repo).RollupDailyUsage(ctx, domain.UsageRollupDailyRequest{Date: tc.date})
			wantValidation(t, err, tc.contains)
			if repo.calls != 0 {
				t.Errorf("a rejected day reached the repository (%d calls)", repo.calls)
			}
		})
	}

	// Today's partial day is allowed, and the day is handed on trimmed.
	repo := &mockInstanceIngestionRepo{}
	resp, err := newTestIngestionService(repo).RollupDailyUsage(ctx, domain.UsageRollupDailyRequest{Date: " 2026-10-16 "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.rolledDay != "2026-10-16" || resp.RowsWritten != 7 || resp.Status != "SUCCESS" {
		t.Errorf("rolled %q, response %+v", repo.rolledDay, resp)
	}
}

func TestInstanceIngestion_RollupMonthlyValidatesTheMonth(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name        string
		year, month int
		contains    string
	}{
		{"month zero", 2026, 0, "month must be between 1 and 12"},
		{"month thirteen", 2026, 13, "month must be between 1 and 12"},
		{"year too early", 1999, 12, "year must be 2000 or later"},
		{"next month", 2026, 11, "must not be in the future"},
		{"next year", 2027, 1, "must not be in the future"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockInstanceIngestionRepo{}
			_, err := newTestIngestionService(repo).RollupMonthlyUsage(ctx, domain.UsageRollupMonthlyRequest{Year: tc.year, Month: tc.month})
			wantValidation(t, err, tc.contains)
			if repo.calls != 0 {
				t.Errorf("a rejected month reached the repository (%d calls)", repo.calls)
			}
		})
	}

	// The current month is allowed.
	repo := &mockInstanceIngestionRepo{}
	resp, err := newTestIngestionService(repo).RollupMonthlyUsage(ctx, domain.UsageRollupMonthlyRequest{Year: 2026, Month: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.rolledYear != 2026 || repo.rolledMonth != 10 || resp.RowsWritten != 9 {
		t.Errorf("rolled %d-%d, response %+v", repo.rolledYear, repo.rolledMonth, resp)
	}
}

func TestInstanceIngestion_ListUnresolvedProductsDefaultsAndValidation(t *testing.T) {
	ctx := context.Background()

	repo := &mockInstanceIngestionRepo{}
	resp, err := newTestIngestionService(repo).ListUnresolvedProducts(ctx, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// newTestIngestionService's clock is 2026-10-16T08:00Z: the default window is the last 24 hours.
	// The repository is asked for one more than the limit, to tell a cut-off list.
	if !repo.unresolvedSince.Equal(time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)) || repo.unresolvedLimit != 501 {
		t.Errorf("defaults: since %v limit %d", repo.unresolvedSince, repo.unresolvedLimit)
	}
	if resp.Count != 1 || resp.Nodes[0].ReportedProduct != "wso2xx" {
		t.Errorf("response %+v", resp)
	}

	repo = &mockInstanceIngestionRepo{}
	if _, err := newTestIngestionService(repo).ListUnresolvedProducts(ctx, "2026-10-01T00:00:00Z", "20"); err != nil {
		t.Fatal(err)
	}
	if !repo.unresolvedSince.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) || repo.unresolvedLimit != 21 {
		t.Errorf("explicit: since %v limit %d", repo.unresolvedSince, repo.unresolvedLimit)
	}

	for _, tc := range []struct{ since, limit, contains string }{
		{"yesterday", "", "since must be an RFC 3339 time"},
		{"", "0", "limit must be between 1 and 5000"},
		{"", "5001", "limit must be between 1 and 5000"},
		{"", "ten", "limit must be between 1 and 5000"},
	} {
		repo := &mockInstanceIngestionRepo{}
		_, err := newTestIngestionService(repo).ListUnresolvedProducts(ctx, tc.since, tc.limit)
		wantValidation(t, err, tc.contains)
		if repo.calls != 0 {
			t.Errorf("a rejected request (%q, %q) reached the repository", tc.since, tc.limit)
		}
	}
}

func TestInstanceIngestion_ListUsageReportImportsValidation(t *testing.T) {
	ctx := context.Background()
	repo := &mockInstanceIngestionRepo{}
	resp, err := newTestIngestionService(repo).ListUsageReportImports(ctx, " SCTPSUB ", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if repo.importsProject != "SCTPSUB" || repo.importsDeployment != "" || repo.importsLimit != 100 || resp.Count != 1 {
		t.Errorf("defaults: %q %q %d, response %+v", repo.importsProject, repo.importsDeployment, repo.importsLimit, resp)
	}
	for _, tc := range []struct{ project, deployment, limit, contains string }{
		{"", "", "", "projectKey or deploymentNumber is required"},
		{"P", "", "0", "limit must be between 1 and 1000"},
		{"", "DEP1", "1001", "limit must be between 1 and 1000"},
	} {
		repo := &mockInstanceIngestionRepo{}
		_, err := newTestIngestionService(repo).ListUsageReportImports(ctx, tc.project, tc.deployment, tc.limit)
		wantValidation(t, err, tc.contains)
		if repo.calls != 0 {
			t.Errorf("a rejected request reached the repository")
		}
	}
}

func TestInstanceIngestion_ReportedProductAndVersionAreLengthChecked(t *testing.T) {
	repo := &mockInstanceIngestionRepo{}
	_, err := newTestIngestionService(repo).UpsertInstanceMeta(context.Background(), domain.InstanceMetaRequest{
		NodeID: "n", InstanceNodeMeta: domain.InstanceNodeMeta{Product: strings.Repeat("p", 256), Version: "1"}})
	wantValidation(t, err, "product")
	_, err = newTestIngestionService(repo).UpsertInstanceMeta(context.Background(), domain.InstanceMetaRequest{
		NodeID: "n", InstanceNodeMeta: domain.InstanceNodeMeta{Product: "wso2is", Version: strings.Repeat("9", 101)}})
	wantValidation(t, err, "version")
	if repo.calls != 0 {
		t.Errorf("an over-long product or version reached the repository")
	}
}

func TestInstanceIngestion_ExportVersionIsLengthChecked(t *testing.T) {
	repo := &mockInstanceIngestionRepo{}
	_, err := newTestIngestionService(repo).ImportDailyUsagesBatch(context.Background(), domain.DailyUsageBatchRequest{
		NodeID: "n", ExportVersion: strings.Repeat("1", 21)})
	wantValidation(t, err, "exportVersion")
	if repo.calls != 0 {
		t.Errorf("an over-long export version reached the repository")
	}
}

func TestInstanceIngestion_BatchNodeProductAndVersionAreLengthChecked(t *testing.T) {
	for _, tc := range []struct {
		node     domain.DailyUsageBatchNode
		contains string
	}{
		{domain.DailyUsageBatchNode{NodeID: "n", Product: strings.Repeat("p", 256), Version: "7.1.0"}, "nodes[0].product"},
		{domain.DailyUsageBatchNode{NodeID: "n", Product: "wso2is", Version: strings.Repeat("9", 101)}, "nodes[0].version"},
	} {
		repo := &mockInstanceIngestionRepo{}
		_, err := newTestIngestionService(repo).ImportDailyUsagesBatch(context.Background(), domain.DailyUsageBatchRequest{
			NodeID: "n", Nodes: []domain.DailyUsageBatchNode{tc.node}})
		wantValidation(t, err, tc.contains)
		if repo.calls != 0 {
			t.Errorf("an over-long %s reached the repository", tc.contains)
		}
	}
}

func TestInstanceIngestion_ListUnresolvedProductsSaysWhenTheListIsCut(t *testing.T) {
	ctx := context.Background()
	three := []domain.UnresolvedProductNode{{NodeID: "a"}, {NodeID: "b"}, {NodeID: "c"}}

	repo := &mockInstanceIngestionRepo{unresolvedNodes: three}
	resp, err := newTestIngestionService(repo).ListUnresolvedProducts(ctx, "", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Truncated || resp.Count != 2 || len(resp.Nodes) != 2 {
		t.Errorf("limit 2 of 3: %+v", resp)
	}

	repo = &mockInstanceIngestionRepo{unresolvedNodes: three}
	if resp, err = newTestIngestionService(repo).ListUnresolvedProducts(ctx, "", "3"); err != nil {
		t.Fatal(err)
	}
	if resp.Truncated || resp.Count != 3 {
		t.Errorf("limit 3 of 3: %+v", resp)
	}
}
