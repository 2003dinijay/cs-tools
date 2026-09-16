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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// DeployedProductRepository defines the persistence operations for the
// deployed_product table (migration 000014).
type DeployedProductRepository interface {
	// SearchDeployedProducts returns a filtered, paginated slice of enriched deployed-product
	// views together with the total count of matching rows before pagination.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error)
}

type deployedProductRepo struct {
	db *pgxpool.Pool
}

// NewDeployedProductRepository constructs a DeployedProductRepository backed by the given connection pool.
func NewDeployedProductRepository(db *pgxpool.Pool) DeployedProductRepository {
	return &deployedProductRepo{db: db}
}

// SearchDeployedProducts implements DeployedProductRepository.
func (r *deployedProductRepo) SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
	filterArgs := []any{}
	argIdx := 1

	// deployed_product.deployment_id/product_id are both nullable (migration
	// 000014 sets them NULL when the parent deployment/product is deleted).
	// The data query below inner-joins both and so can never return such a
	// row; without these predicates the count query would still include it,
	// inflating total relative to what's actually returned.
	// DeployedProductView.Deployment/Product are non-pointer EntityRef
	// values, so switching to LEFT joins isn't a safe alternative -- that
	// would need a response-contract change and nullable scan handling.
	where := "WHERE dp.deployment_id IS NOT NULL AND dp.product_id IS NOT NULL"

	if len(req.DeploymentIDs) > 0 {
		where += fmt.Sprintf(" AND dp.deployment_id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.DeploymentIDs)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM deployed_product dp " + where

	// update_level_info (JSONB) -- domain.DeployedProductView.Updates -- is
	// deliberately not selected here: its actual JSON shape isn't confirmed
	// against any real payload, so it's left unpopulated (nil, the correct
	// "none recorded" value per that field's own doc comment) rather than
	// guessed at. cores/tps/category, in contrast, are plain scalar columns
	// with an unambiguous mapping, so they are selected.
	dataQuery := fmt.Sprintf(
		`SELECT dp.id, dp.created_on, dp.updated_on,
		        dp.core_count, dp.tps_count, dp.product_category::TEXT,
		        d.id, d.name,
		        p.id, p.name,
		        pv.id, pv.version, pv.release_date, pv.support_eol_date
		 FROM deployed_product dp
		 JOIN deployment d ON dp.deployment_id = d.id
		 JOIN product p ON dp.product_id = p.id
		 LEFT JOIN product_version pv ON dp.version_id = pv.id
		 %s
		 ORDER BY dp.created_on DESC, dp.id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var deployedProducts []domain.DeployedProductView

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count deployed products: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query deployed products: %w", err)
		}
		defer rows.Close()

		result := make([]domain.DeployedProductView, 0, req.Pagination.Limit)
		for rows.Next() {
			var dp domain.DeployedProductView
			// Version fields are nullable (LEFT JOIN).
			var pvID, pvName *string
			var pvReleaseDate, pvEoLDate *time.Time
			if err := rows.Scan(
				&dp.ID, &dp.CreatedOn, &dp.UpdatedOn,
				&dp.Cores, &dp.TPS, &dp.Category,
				&dp.Deployment.ID, &dp.Deployment.Name,
				&dp.Product.ID, &dp.Product.Name,
				&pvID, &pvName, &pvReleaseDate, &pvEoLDate,
			); err != nil {
				return fmt.Errorf("scan deployed product: %w", err)
			}
			if pvID != nil {
				dp.Version = &domain.DeployedProductVersionRef{
					ID:             *pvID,
					Name:           *pvName,
					ReleasedDate:   pvReleaseDate,
					SupportEoLDate: pvEoLDate,
				}
			}
			result = append(result, dp)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate deployed products: %w", err)
		}
		deployedProducts = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return deployedProducts, total, nil
}
