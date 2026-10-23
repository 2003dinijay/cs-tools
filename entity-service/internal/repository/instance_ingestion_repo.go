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

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// InstanceIngestionRepository defines the persistence operations for the
// telemetry product-consumption-tracking sends: node registration, host
// snapshots, live usage counts and offline (ZIP) daily usages.
//
// Every write runs in one transaction that first takes a transaction-scoped
// advisory lock on each node id it touches. deployment_node.node_id is not
// unique (the table is synced from ServiceNow, whose own identity is not
// consistent upstream, so a unique index is not an option), and every write
// here is a read-then-insert: without the lock two first heartbeats from one
// node would create two deployment_node rows, and two counts for one day two
// daily_usage_summary rows.
//
// The caller's identity must already be on ctx (the routes are internal-only);
// there is no fallback to the system identity.
type InstanceIngestionRepository interface {
	UpsertDeploymentNode(ctx context.Context, nodeID string, meta domain.InstanceNodeMeta) (string, error)
	InsertDeploymentInformation(ctx context.Context, req domain.InstanceInfoRequest, reportedOn time.Time) (string, error)
	InsertUsageStats(ctx context.Context, req domain.InstanceUsageIngestRequest, countedOn time.Time) error
	ImportDailyUsagesBatch(ctx context.Context, req domain.DailyUsageBatchRequest) (int, error)
	// RollupDailyUsage recomputes the API_CALL daily usage rows of a UTC day (YYYY-MM-DD)
	// from its hourly rows, inside the database, and returns how many rows it wrote.
	RollupDailyUsage(ctx context.Context, day string) (int, error)
	// RollupMonthlyUsage recomputes a calendar month's monthly usage rows from the daily
	// ones, inside the database, and returns how many rows it wrote.
	RollupMonthlyUsage(ctx context.Context, year, month int) (int, error)
	// ListUnresolvedProductNodes lists nodes seen since the given time whose reported
	// product and version did not match the catalogue, at most limit of them.
	ListUnresolvedProductNodes(ctx context.Context, since time.Time, limit int) ([]domain.UnresolvedProductNode, error)
	// ListUsageReportImports lists imported reports for a project key and/or deployment
	// number, newest first, at most limit of them.
	ListUsageReportImports(ctx context.Context, projectKey, deploymentNumber string, limit int) ([]domain.UsageReportImport, error)
	GetProjectSecretKey(ctx context.Context, projectKey string) (string, error)
	GetDeploymentConsumptionSecret(ctx context.Context, deploymentNumber string) (domain.DeploymentConsumptionSecretResponse, error)
}

type instanceIngestionRepo struct {
	db                    *Scoped
	monthlyRollupOnImport bool
}

// InstanceIngestionOption configures NewInstanceIngestionRepository.
type InstanceIngestionOption func(*instanceIngestionRepo)

// WithMonthlyRollupOnImport makes ImportDailyUsagesBatch rebuild the monthly rows of
// every month its rows fall in, in the import's own transaction. An offline report
// arrives late by nature, often for a month the scheduled monthly rollup no longer
// rebuilds, so without this its days would never reach monthly_usage_summary.
func WithMonthlyRollupOnImport(enabled bool) InstanceIngestionOption {
	return func(r *instanceIngestionRepo) { r.monthlyRollupOnImport = enabled }
}

// NewInstanceIngestionRepository constructs an InstanceIngestionRepository backed by *Scoped.
func NewInstanceIngestionRepository(db *Scoped, opts ...InstanceIngestionOption) InstanceIngestionRepository {
	r := &instanceIngestionRepo{db: db}
	for _, o := range opts {
		o(r)
	}
	return r
}

// deploymentNodeLockKey is the advisory-lock key every ingestion write takes
// for a node before reading or writing anything about it.
func deploymentNodeLockKey(nodeID string) string {
	return "deployment-node:" + nodeID
}

// resolveProductVersionIDTx maps a product name or code and a version to a
// product_version id. An exact code match wins over an exact name match, which
// wins over a name containing the product (the reported value is sometimes a
// shortened name); among the contained-name matches the shortest name wins, so
// "API Manager" picks "WSO2 API Manager" over "WSO2 API Manager Analytics".
// The remaining ties are broken by id, so the answer never depends on the plan.
func resolveProductVersionIDTx(ctx context.Context, q querier, productName, version string) (*string, error) {
	product := strings.TrimSpace(productName)
	ver := strings.TrimSpace(version)
	if product == "" || ver == "" {
		return nil, nil
	}

	const query = `
		SELECT pv.id::text
		FROM product_version pv
		JOIN product p ON p.id = pv.product_id
		WHERE pv.version = $2
		  AND (LOWER(p.code) = LOWER($1)
		       OR LOWER(p.name) = LOWER($1)
		       OR LOWER(p.name) LIKE '%' || LOWER($3) || '%' ESCAPE '\')
		ORDER BY CASE
		           WHEN LOWER(p.code) = LOWER($1) THEN 0
		           WHEN LOWER(p.name) = LOWER($1) THEN 1
		           ELSE 2
		         END,
		         LENGTH(p.name), p.name, p.id, pv.id
		LIMIT 1`

	var id string
	err := q.QueryRow(ctx, query, product, ver, escapeLikeWildcards(product)).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve product version: %w", err)
	}
	return &id, nil
}

// lockAndUpsertNodeTx takes the node's advisory lock, then registers the node
// or updates it from meta, returning deployment_node.id. A blank project key or
// deployment number leaves the stored value alone; the product version is only
// moved when both product and version are sent (to NULL when that pair is not
// in the catalogue, so a node never keeps reporting a version it has left).
// When several rows already share the node id, the oldest one is the node.
func lockAndUpsertNodeTx(ctx context.Context, tx pgx.Tx, nodeID string, meta domain.InstanceNodeMeta) (string, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0::bigint))`, deploymentNodeLockKey(nodeID)); err != nil {
		return "", fmt.Errorf("lock deployment node: %w", err)
	}

	moveVersion := strings.TrimSpace(meta.Product) != "" && strings.TrimSpace(meta.Version) != ""
	var versionID *string
	if moveVersion {
		var err error
		versionID, err = resolveProductVersionIDTx(ctx, tx, meta.Product, meta.Version)
		if err != nil {
			return "", err
		}
		if versionID == nil {
			// ServiceNow emailed a fixed address on this; it is logged here and listed by
			// GET /instances/unresolved-products, from the reported_* columns kept below.
			slog.WarnContext(ctx, "instance ingestion: reported product is not in the product catalogue",
				"nodeId", nodeID, "product", meta.Product, "version", meta.Version,
				"projectKey", meta.ProjectKey, "deploymentNumber", meta.DeploymentNumber)
		}
	}

	var existingID string
	err := tx.QueryRow(ctx, `
		SELECT id::text FROM deployment_node
		WHERE node_id = $1
		ORDER BY created_on, id
		LIMIT 1`, nodeID).Scan(&existingID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("find deployment node: %w", err)
	}

	if existingID != "" {
		const updateSQL = `
			UPDATE deployment_node
			SET project_key        = COALESCE(NULLIF($2, ''), project_key),
			    deployment_number  = COALESCE(NULLIF($3, ''), deployment_number),
			    product_version_id = CASE WHEN $5 THEN $4::uuid ELSE product_version_id END,
			    reported_product   = CASE WHEN $5 THEN $7 ELSE reported_product END,
			    reported_version   = CASE WHEN $5 THEN $8 ELSE reported_version END,
			    unresolved_since   = CASE
			        WHEN NOT $5 THEN unresolved_since
			        WHEN $4::uuid IS NOT NULL THEN NULL
			        WHEN product_version_id IS NULL THEN COALESCE(unresolved_since, NOW())
			        ELSE NOW() END,
			    updated_on = NOW(),
			    updated_by = $6
			WHERE id = $1::uuid`
		if _, err := tx.Exec(ctx, updateSQL, existingID, meta.ProjectKey, meta.DeploymentNumber, versionID, moveVersion, domain.InstanceIngestionActor,
			nonBlank(meta.Product), nonBlank(meta.Version)); err != nil {
			return "", fmt.Errorf("update deployment node: %w", err)
		}
		return existingID, nil
	}

	const insertSQL = `
		INSERT INTO deployment_node (
			id, created_on, updated_on, created_by, updated_by,
			node_id, project_key, deployment_number, product_version_id,
			reported_product, reported_version, unresolved_since
		) VALUES (
			gen_random_uuid(), NOW(), NOW(), $5, $5,
			$1, $2, NULLIF($3, ''), $4::uuid,
			$6, $7, CASE WHEN $8 THEN NOW() END
		)
		RETURNING id::text`
	var reportedProduct, reportedVersion *string
	if moveVersion {
		reportedProduct, reportedVersion = nonBlank(meta.Product), nonBlank(meta.Version)
	}
	var insertedID string
	if err := tx.QueryRow(ctx, insertSQL, nodeID, meta.ProjectKey, meta.DeploymentNumber, versionID, domain.InstanceIngestionActor,
		reportedProduct, reportedVersion, moveVersion && versionID == nil).Scan(&insertedID); err != nil {
		return "", fmt.Errorf("insert deployment node: %w", err)
	}
	return insertedID, nil
}

// storedDeploymentNumberTx reads the node's own deployment number, the value a
// daily row gets when the request names none.
func storedDeploymentNumberTx(ctx context.Context, tx pgx.Tx, nodeUUID string) (*string, error) {
	var dn *string
	if err := tx.QueryRow(ctx, `SELECT deployment_number FROM deployment_node WHERE id = $1::uuid`, nodeUUID).Scan(&dn); err != nil {
		return nil, fmt.Errorf("read deployment number: %w", err)
	}
	return dn, nil
}

// upsertDailyUsageTx writes one daily_usage_summary row per (node, day, count
// type, data source): the latest value wins, as product-consumption-tracking's
// own SQL did. The caller holds the node's advisory lock.
func upsertDailyUsageTx(ctx context.Context, tx pgx.Tx, nodeUUID string, deploymentNumber *string, summaryDate, countType string, value int, dataSource, actor string) error {
	var existingID string
	err := tx.QueryRow(ctx, `
		SELECT id::text FROM daily_usage_summary
		WHERE deployment_node_id = $1::uuid
		  AND summary_date = $2::date
		  AND count_type = $3
		  AND data_source = $4::usage_data_source_enum
		ORDER BY created_on, id
		LIMIT 1`, nodeUUID, summaryDate, countType, dataSource).Scan(&existingID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("find daily usage: %w", err)
	}

	if existingID != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE daily_usage_summary
			SET value = $2,
			    deployment_number = COALESCE($3, deployment_number),
			    updated_by = $4,
			    updated_on = NOW()
			WHERE id = $1::uuid`, existingID, value, deploymentNumber, actor); err != nil {
			return fmt.Errorf("update daily usage: %w", err)
		}
		return nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO daily_usage_summary (
			id, created_on, updated_on, created_by, updated_by,
			deployment_node_id, deployment_number, summary_date, count_type, value, data_source
		) VALUES (
			gen_random_uuid(), NOW(), NOW(), $7, $7,
			$1::uuid, $2, $3::date, $4, $5, $6::usage_data_source_enum
		)`, nodeUUID, deploymentNumber, summaryDate, countType, value, dataSource, actor); err != nil {
		return fmt.Errorf("insert daily usage: %w", err)
	}
	return nil
}

func nonBlank(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

// positiveOrNil stores an unreported (zero) core count as NULL, as
// product-consumption-tracking did when the snapshot carried none.
func positiveOrNil(n int) *int {
	if n <= 0 {
		return nil
	}
	return &n
}

// UpsertDeploymentNode registers or updates a node in deployment_node.
func (r *instanceIngestionRepo) UpsertDeploymentNode(ctx context.Context, nodeID string, meta domain.InstanceNodeMeta) (string, error) {
	if strings.TrimSpace(nodeID) == "" {
		return "", &apierror.ValidationError{Msg: "nodeId is required"}
	}
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
		return lockAndUpsertNodeTx(ctx, tx, nodeID, meta)
	})
}

// InsertDeploymentInformation registers the node from the request's meta, then
// records the host snapshot. A snapshot already stored for the node with the
// same hash is not stored again: its payload_updated_on is moved instead and
// its id returned, so a node that reports the same environment every hour keeps
// one row, as product-consumption-tracking's own SQL did.
func (r *instanceIngestionRepo) InsertDeploymentInformation(ctx context.Context, req domain.InstanceInfoRequest, reportedOn time.Time) (string, error) {
	if strings.TrimSpace(req.NodeID) == "" {
		return "", &apierror.ValidationError{Msg: "nodeId is required"}
	}

	infoJSON := []byte("{}")
	if req.DeploymentInfo != nil {
		b, err := json.Marshal(req.DeploymentInfo)
		if err != nil {
			return "", &apierror.ValidationError{Msg: "deploymentInfo must be valid JSON"}
		}
		infoJSON = b
	}

	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
		if _, err := lockAndUpsertNodeTx(ctx, tx, req.NodeID, req.InstanceNodeMeta); err != nil {
			return "", err
		}

		var existingID string
		err := tx.QueryRow(ctx, `
			SELECT id::text FROM deployment_information
			WHERE node_id = $1 AND deployment_info_hash = $2
			ORDER BY created_on, id
			LIMIT 1`, req.NodeID, req.Hash).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("find deployment information: %w", err)
		}
		if existingID != "" {
			if _, err := tx.Exec(ctx, `
				UPDATE deployment_information
				SET payload_updated_on = GREATEST(payload_updated_on, $2),
				    updated_on = NOW(),
				    updated_by = $3
				WHERE id = $1::uuid`, existingID, reportedOn, domain.InstanceIngestionActor); err != nil {
				return "", fmt.Errorf("update deployment information: %w", err)
			}
			return existingID, nil
		}

		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO deployment_information (
				id, created_on, updated_on, created_by, updated_by,
				node_id, jdk_version, core_count, deployment_info, deployment_info_hash,
				payload_created_on, payload_updated_on
			) VALUES (
				gen_random_uuid(), NOW(), NOW(), $7, $7,
				$1, $2, $3, $4::jsonb, $5,
				$6, $6
			)
			RETURNING id::text`,
			req.NodeID, nonBlank(req.JDKVersion), positiveOrNil(req.CoreCount), string(infoJSON), req.Hash, reportedOn, domain.InstanceIngestionActor,
		).Scan(&id); err != nil {
			return "", fmt.Errorf("insert deployment information: %w", err)
		}
		return id, nil
	})
}

// InsertUsageStats registers the node from the request's meta, records the
// count in hourly_usage_summary and recomputes the day's API_CALL row of
// daily_usage_summary from that day's hourly rows (the day is countedOn's UTC
// date): the sum for a counter, the latest report for anything else. The row
// used to take the reported count as the day's value, so a counter published
// every hour showed only its last hour.
func (r *instanceIngestionRepo) InsertUsageStats(ctx context.Context, req domain.InstanceUsageIngestRequest, countedOn time.Time) error {
	if strings.TrimSpace(req.NodeID) == "" {
		return &apierror.ValidationError{Msg: "nodeId is required"}
	}
	if strings.TrimSpace(req.CountType) == "" {
		return &apierror.ValidationError{Msg: "countType is required"}
	}

	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		nodeUUID, err := lockAndUpsertNodeTx(ctx, tx, req.NodeID, req.InstanceNodeMeta)
		if err != nil {
			return err
		}

		// A report the tracker retries (same node, type and counted_on) is stored once:
		// counters are summed into the day, so a second copy would double-count. The
		// node lock held above serialises every writer of this node, so the check
		// cannot race; hourly_usage_summary has no unique key to lean on (synced rows
		// may already hold duplicates, which the sums skip, see dailyUsageValueTx).
		if _, err := tx.Exec(ctx, `
			INSERT INTO hourly_usage_summary (
				id, created_on, updated_on, created_by, updated_by,
				deployment_node_id, count_type, count, counted_on
			)
			SELECT gen_random_uuid(), NOW(), NOW(), $5::text, $5::text, $1::uuid, $2::text, $3::integer, $4::timestamptz
			WHERE NOT EXISTS (
				SELECT 1 FROM hourly_usage_summary
				WHERE deployment_node_id = $1::uuid AND count_type = $2::text AND counted_on = $4::timestamptz)`,
			nodeUUID, req.CountType, req.Count, countedOn, domain.InstanceIngestionActor); err != nil {
			return fmt.Errorf("insert hourly usage: %w", err)
		}

		deploymentNumber := nonBlank(req.DeploymentNumber)
		if deploymentNumber == nil {
			if deploymentNumber, err = storedDeploymentNumberTx(ctx, tx, nodeUUID); err != nil {
				return err
			}
		}
		dayValue, err := dailyUsageValueTx(ctx, tx, nodeUUID, req.CountType, countedOn)
		if err != nil {
			return err
		}
		summaryDate := countedOn.UTC().Format("2006-01-02")
		return upsertDailyUsageTx(ctx, tx, nodeUUID, deploymentNumber, summaryDate, req.CountType, dayValue, "API_CALL", domain.InstanceIngestionActor)
	})
}

// isCounterCountType reports whether a count type is a per-period delta that
// adds up over a day, as opposed to a level (users, organisations, cores, MAU)
// whose day value is its latest reading. A product publishes each cycle's
// transactions, M2M tokens and agent token or lifecycle events as the amount
// since its previous publish.
func isCounterCountType(countType string) bool {
	return countType == "TRANSACTION_COUNT" || countType == "M2M_TOKEN" || strings.HasPrefix(countType, "AGENT_")
}

// dailyUsageValueTx derives a node's day value for a count type from its
// hourly rows of that UTC day (the day countedOn falls in): the sum for a
// counter (capped at what the INTEGER column holds), the latest report by
// counted_on for anything else, so a late report from an earlier hour does not
// replace a newer one.
func dailyUsageValueTx(ctx context.Context, tx pgx.Tx, nodeUUID, countType string, countedOn time.Time) (int, error) {
	u := countedOn.UTC()
	start := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)

	query := `
		SELECT count FROM hourly_usage_summary
		WHERE deployment_node_id = $1::uuid AND count_type = $2 AND counted_on >= $3 AND counted_on < $4
		ORDER BY counted_on DESC, created_on DESC
		LIMIT 1`
	if isCounterCountType(countType) {
		// One row per counted_on: a duplicate of the same report is counted once.
		query = `
			SELECT LEAST(COALESCE(SUM(count), 0), 2147483647)::int FROM (
				SELECT DISTINCT ON (counted_on) count FROM hourly_usage_summary
				WHERE deployment_node_id = $1::uuid AND count_type = $2 AND counted_on >= $3 AND counted_on < $4
				ORDER BY counted_on, created_on DESC, id) per_report`
	}
	var value int
	if err := tx.QueryRow(ctx, query, nodeUUID, countType, start, end).Scan(&value); err != nil {
		return 0, fmt.Errorf("derive daily usage: %w", err)
	}
	return value, nil
}

// ImportDailyUsagesBatch writes an offline import's daily usages as FILE_UPLOAD
// rows, all or nothing: one transaction, so a failure part-way leaves nothing
// imported and the upload can simply be retried. An entry's own nodeId and
// deploymentNumber override the batch's. Nodes are locked in id order, so two
// imports touching the same nodes cannot deadlock.
func (r *instanceIngestionRepo) ImportDailyUsagesBatch(ctx context.Context, req domain.DailyUsageBatchRequest) (int, error) {
	if strings.TrimSpace(req.NodeID) == "" {
		return 0, &apierror.ValidationError{Msg: "nodeId is required"}
	}

	actor := strings.TrimSpace(req.UserEmail)
	if actor == "" {
		actor = domain.InstanceIngestionActor
	}

	entryNode := func(e domain.DailyUsageBatchItem) string {
		if strings.TrimSpace(e.NodeID) != "" {
			return e.NodeID
		}
		return req.NodeID
	}
	nodeSet := map[string]domain.InstanceNodeMeta{req.NodeID: req.InstanceNodeMeta}
	// Nodes the report describes are registered (with their own product and
	// version) even when they have no usage rows.
	for _, n := range req.Nodes {
		if strings.TrimSpace(n.NodeID) == "" {
			continue
		}
		meta := req.InstanceNodeMeta
		if strings.TrimSpace(n.Product) != "" && strings.TrimSpace(n.Version) != "" {
			meta.Product, meta.Version = n.Product, n.Version
		}
		nodeSet[n.NodeID] = meta
	}
	for _, e := range req.Usages {
		id := entryNode(e)
		if _, ok := nodeSet[id]; ok {
			continue
		}
		meta := req.InstanceNodeMeta
		if strings.TrimSpace(e.DeploymentNumber) != "" {
			meta.DeploymentNumber = e.DeploymentNumber
		}
		nodeSet[id] = meta
	}
	nodeIDs := make([]string, 0, len(nodeSet))
	for id := range nodeSet {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (int, error) {
		nodeUUIDs := make(map[string]string, len(nodeIDs))
		for _, id := range nodeIDs {
			uuid, err := lockAndUpsertNodeTx(ctx, tx, id, nodeSet[id])
			if err != nil {
				return 0, err
			}
			nodeUUIDs[id] = uuid
		}

		imported := 0
		for _, e := range req.Usages {
			nodeID := entryNode(e)
			deploymentNumber := nonBlank(e.DeploymentNumber)
			if deploymentNumber == nil {
				deploymentNumber = nonBlank(nodeSet[nodeID].DeploymentNumber)
			}
			if deploymentNumber == nil {
				var err error
				if deploymentNumber, err = storedDeploymentNumberTx(ctx, tx, nodeUUIDs[nodeID]); err != nil {
					return 0, err
				}
			}
			if err := upsertDailyUsageTx(ctx, tx, nodeUUIDs[nodeID], deploymentNumber, e.Date, e.Type, e.Value, "FILE_UPLOAD", actor); err != nil {
				return 0, err
			}
			imported++
		}

		if r.monthlyRollupOnImport {
			if err := rollupImportedMonthsTx(ctx, tx, req.Usages); err != nil {
				return 0, err
			}
		}

		// The import's history row, in the same transaction: a failed import leaves none.
		var exportedAt *time.Time
		if v := strings.TrimSpace(req.ExportedAt); v != "" {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				exportedAt = &t
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO usage_report_import (
				created_by, project_key, deployment_number, export_version, exported_at, export_valid,
				node_count, imported_count
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			actor, nonBlank(req.ProjectKey), nonBlank(req.DeploymentNumber), nonBlank(req.ExportVersion), exportedAt,
			req.ExportValid, len(nodeIDs), imported); err != nil {
			return 0, fmt.Errorf("record usage report import: %w", err)
		}
		return imported, nil
	})
}

// rollupImportedMonthsTx rebuilds the monthly rows of every month the imported usages
// fall in, oldest first, so the import's days reach monthly_usage_summary however late
// the report arrived. The monthly function takes a lock per month; taking months in a
// fixed order keeps two imports from taking a pair of them in opposite orders.
func rollupImportedMonthsTx(ctx context.Context, tx pgx.Tx, usages []domain.DailyUsageBatchItem) error {
	months := map[[2]int]struct{}{}
	for _, e := range usages {
		d, err := time.Parse("2006-01-02", strings.TrimSpace(e.Date))
		if err != nil {
			return &apierror.ValidationError{Msg: fmt.Sprintf("usage date %q is not a date (YYYY-MM-DD)", e.Date)}
		}
		months[[2]int{d.Year(), int(d.Month())}] = struct{}{}
	}
	ordered := make([][2]int, 0, len(months))
	for m := range months {
		ordered = append(ordered, m)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i][0] != ordered[j][0] {
			return ordered[i][0] < ordered[j][0]
		}
		return ordered[i][1] < ordered[j][1]
	})
	for _, m := range ordered {
		if _, err := tx.Exec(ctx, `SELECT rollup_monthly_usage($1, $2)`, m[0], m[1]); err != nil {
			return fmt.Errorf("roll up imported month %d-%02d: %w", m[0], m[1], err)
		}
	}
	return nil
}

// GetProjectSecretKey returns the project's product-consumption HMAC primary
// secret key, looked up by project key (or project id). A project with no key
// set is a NotFoundError as well: there is nothing to verify a signature with.
func (r *instanceIngestionRepo) GetProjectSecretKey(ctx context.Context, projectKey string) (string, error) {
	trimmed := strings.TrimSpace(projectKey)
	if trimmed == "" {
		return "", &apierror.ValidationError{Msg: "project key is required"}
	}

	var secretKey *string
	err := r.db.QueryRow(ctx, `
		SELECT product_consumption_primary_secret_key
		FROM project
		WHERE key = $1 OR id::text = $1
		ORDER BY (key = $1) DESC
		LIMIT 1`, trimmed).Scan(&secretKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", &apierror.NotFoundError{Msg: fmt.Sprintf("project %q not found", trimmed)}
		}
		return "", fmt.Errorf("get project secret key: %w", err)
	}
	if secretKey == nil || strings.TrimSpace(*secretKey) == "" {
		return "", &apierror.NotFoundError{Msg: fmt.Sprintf("project %q has no product consumption secret key", trimmed)}
	}
	return *secretKey, nil
}

// GetDeploymentConsumptionSecret returns the project a deployment number belongs
// to and that project's primary secret key. An active deployment wins over an
// inactive one with the same number. An unknown deployment, or a project with no
// key, is a NotFoundError.
func (r *instanceIngestionRepo) GetDeploymentConsumptionSecret(ctx context.Context, deploymentNumber string) (domain.DeploymentConsumptionSecretResponse, error) {
	number := strings.TrimSpace(deploymentNumber)
	if number == "" {
		return domain.DeploymentConsumptionSecretResponse{}, &apierror.ValidationError{Msg: "deployment number is required"}
	}

	var projectID string
	var projectKey, secretKey *string
	err := r.db.QueryRow(ctx, `
		SELECT p.id::text, p.key, p.product_consumption_primary_secret_key
		FROM deployment d
		JOIN project p ON p.id = d.project_id
		WHERE d.number = $1
		ORDER BY COALESCE(d.is_active, TRUE) DESC, d.created_on, d.id
		LIMIT 1`, number).Scan(&projectID, &projectKey, &secretKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DeploymentConsumptionSecretResponse{}, &apierror.NotFoundError{Msg: fmt.Sprintf("deployment %q not found", number)}
		}
		return domain.DeploymentConsumptionSecretResponse{}, fmt.Errorf("get deployment consumption secret: %w", err)
	}
	if secretKey == nil || strings.TrimSpace(*secretKey) == "" {
		return domain.DeploymentConsumptionSecretResponse{}, &apierror.NotFoundError{Msg: fmt.Sprintf("the project of deployment %q has no product consumption secret key", number)}
	}
	key := ""
	if projectKey != nil {
		key = *projectKey
	}
	return domain.DeploymentConsumptionSecretResponse{
		DeploymentNumber: number,
		ProjectID:        projectID,
		ProjectKey:       key,
		SecretKey:        *secretKey,
	}, nil
}

// dailyRollupChunkSize is how many nodes one rollup transaction takes. Each node is
// one advisory lock held to the end of the transaction, and those share the lock
// table, so a day with many nodes is rolled up a chunk at a time.
const dailyRollupChunkSize = 100

// RollupDailyUsage runs the rollup_daily_usage SQL function (migration 0245) for a
// day, a chunk of nodes per transaction, in byte order. The function is idempotent
// per node, so a run that fails part-way is completed by running it again.
func (r *instanceIngestionRepo) RollupDailyUsage(ctx context.Context, day string) (int, error) {
	nodeIDs, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) ([]string, error) {
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT dn.node_id COLLATE "C"
			FROM hourly_usage_summary h
			JOIN deployment_node dn ON dn.id = h.deployment_node_id
			WHERE h.counted_on >= ($1::date)::timestamp AT TIME ZONE 'UTC'
			  AND h.counted_on < ($1::date + 1)::timestamp AT TIME ZONE 'UTC'
			ORDER BY 1`, day)
		if err != nil {
			return nil, err
		}
		return pgx.CollectRows(rows, pgx.RowTo[string])
	})
	if err != nil {
		return 0, fmt.Errorf("list nodes to roll up: %w", err)
	}

	written := 0
	for start := 0; start < len(nodeIDs); start += dailyRollupChunkSize {
		chunk := nodeIDs[start:min(start+dailyRollupChunkSize, len(nodeIDs))]
		n, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) (int, error) {
			var n int
			if err := tx.QueryRow(ctx, `SELECT rollup_daily_usage($1::date, $2::text[])`, day, chunk).Scan(&n); err != nil {
				return 0, err
			}
			return n, nil
		})
		if err != nil {
			return written, fmt.Errorf("roll up daily usage: %w", err)
		}
		written += n
	}
	return written, nil
}

// RollupMonthlyUsage runs the rollup_monthly_usage SQL function (migration 0245) for a month.
func (r *instanceIngestionRepo) RollupMonthlyUsage(ctx context.Context, year, month int) (int, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (int, error) {
		var written int
		if err := tx.QueryRow(ctx, `SELECT rollup_monthly_usage($1, $2)`, year, month).Scan(&written); err != nil {
			return 0, fmt.Errorf("roll up monthly usage: %w", err)
		}
		return written, nil
	})
}

// ListUnresolvedProductNodes implements InstanceIngestionRepository. Grouped by what was
// reported, so the catalogue entries to add read as a list, most recently seen first
// within each. A node unresolved since before unresolved_since existed reads as
// unresolved since its last report.
func (r *instanceIngestionRepo) ListUnresolvedProductNodes(ctx context.Context, since time.Time, limit int) ([]domain.UnresolvedProductNode, error) {
	rows, err := r.db.Query(ctx, `
		SELECT node_id, project_key, deployment_number, reported_product, reported_version,
		       COALESCE(unresolved_since, updated_on), updated_on
		FROM deployment_node
		WHERE product_version_id IS NULL
		  AND reported_product IS NOT NULL
		  AND updated_on >= $1
		ORDER BY reported_product, reported_version NULLS FIRST, updated_on DESC, id
		LIMIT $2`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("list unresolved product nodes: %w", err)
	}
	defer rows.Close()

	nodes := []domain.UnresolvedProductNode{}
	for rows.Next() {
		var n domain.UnresolvedProductNode
		if err := rows.Scan(&n.NodeID, &n.ProjectKey, &n.DeploymentNumber, &n.ReportedProduct, &n.ReportedVersion, &n.UnresolvedSince, &n.LastSeenOn); err != nil {
			return nil, fmt.Errorf("scan unresolved product node: %w", err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list unresolved product nodes: %w", err)
	}
	return nodes, nil
}

// ListUsageReportImports implements InstanceIngestionRepository.
func (r *instanceIngestionRepo) ListUsageReportImports(ctx context.Context, projectKey, deploymentNumber string, limit int) ([]domain.UsageReportImport, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, created_on, created_by, project_key, deployment_number, export_version, exported_at,
		       export_valid, node_count, imported_count
		FROM usage_report_import
		WHERE ($1 = '' OR project_key = $1)
		  AND ($2 = '' OR deployment_number = $2)
		ORDER BY created_on DESC, id
		LIMIT $3`, projectKey, deploymentNumber, limit)
	if err != nil {
		return nil, fmt.Errorf("list usage report imports: %w", err)
	}
	defer rows.Close()
	imports := []domain.UsageReportImport{}
	for rows.Next() {
		var i domain.UsageReportImport
		if err := rows.Scan(&i.ID, &i.CreatedOn, &i.CreatedBy, &i.ProjectKey, &i.DeploymentNumber, &i.ExportVersion,
			&i.ExportedAt, &i.ExportValid, &i.NodeCount, &i.ImportedCount); err != nil {
			return nil, fmt.Errorf("scan usage report import: %w", err)
		}
		imports = append(imports, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list usage report imports: %w", err)
	}
	return imports, nil
}
