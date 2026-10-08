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
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// GroupRepository defines the read operations for the "group" table (migration
// 0074), mirrored from ServiceNow's sys_user_group -- the same concept
// GroupService's ServiceNow implementation searches. Its ids are the ones
// GET /groups/{id} and the approval stages' assignment_group_id use.
type GroupRepository interface {
	// SearchGroups returns a filtered, paginated slice of teams together
	// with the total count of matching rows before pagination.
	SearchGroups(ctx context.Context, searchQuery string, limit, offset int) ([]domain.Group, int, error)
}

type groupRepo struct {
	db *pgxpool.Pool
}

// NewGroupRepository constructs a GroupRepository backed by the given connection pool.
func NewGroupRepository(db *pgxpool.Pool) GroupRepository {
	return &groupRepo{db: db}
}

// SearchGroups implements GroupRepository.
//
// Active is "group".is_active, NULL counting as active. Parent is resolved
// from parent_id, and is nil for a top-level group.
func (r *groupRepo) SearchGroups(ctx context.Context, searchQuery string, limit, offset int) ([]domain.Group, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if searchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(searchQuery)
		args = append(args, "%"+escaped+"%")
		where += fmt.Sprintf(" AND g.name ILIKE $%d ESCAPE '\\'", len(args))
	}

	countQuery := `SELECT COUNT(*) FROM "group" g ` + where
	dataQuery := fmt.Sprintf(
		`SELECT g.id::text, COALESCE(g.name, ''), COALESCE(g.is_active, TRUE),
		        p.id::text, p.name
		 FROM "group" g
		 LEFT JOIN "group" p ON p.id = g.parent_id
		 %s ORDER BY g.name, g.id LIMIT $%d OFFSET $%d`,
		where, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), limit, offset)

	var total int
	var groups []domain.Group

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count groups: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query groups: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Group, 0, limit)
		for rows.Next() {
			var g domain.Group
			var parentID, parentName *string
			if err := rows.Scan(&g.ID, &g.Name, &g.Active, &parentID, &parentName); err != nil {
				return fmt.Errorf("scan group: %w", err)
			}
			if parentID != nil {
				g.Parent = &domain.GroupParentRef{ID: *parentID}
				if parentName != nil {
					g.Parent.Name = *parentName
				}
			}
			result = append(result, g)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate groups: %w", err)
		}
		groups = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}
