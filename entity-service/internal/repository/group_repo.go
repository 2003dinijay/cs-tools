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

// GroupRepository defines the read operations for the team table (migration
// 000028), which "mirror[s] a hand-curated allow-list of ServiceNow's OOB
// sys_user_group / sys_user_grmember tables" per that migration's own
// comment -- the same concept GroupService's ServiceNow implementation
// searches.
type GroupRepository interface {
	// SearchGroups returns a filtered, paginated slice of teams together
	// with the total count of matching rows before pagination.
	SearchGroups(ctx context.Context, searchQuery string, limit, offset int) ([]domain.Group, int, error)
	// SearchSupportGroups is SearchGroups over the incident support-group set
	// (supportGroupSetSQL) instead of team: "group" rows, whose ids are the
	// ones work_item.assignment_group_id and service.support_group_id hold.
	// It backs POST /groups/search's supportGroupsOnly filter.
	SearchSupportGroups(ctx context.Context, searchQuery string, limit, offset int) ([]domain.Group, int, error)
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
// domain.Group.Active has no backing column on team -- it is hardcoded true
// (every row here is a hand-curated, actively-maintained team, unlike
// ServiceNow's own sys_user_group, which can hold deactivated groups).
// Parent has no backing column either (team is flat, no hierarchy) and is
// always nil.
func (r *groupRepo) SearchGroups(ctx context.Context, searchQuery string, limit, offset int) ([]domain.Group, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if searchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(searchQuery)
		args = append(args, "%"+escaped+"%")
		where += fmt.Sprintf(" AND name ILIKE $%d ESCAPE '\\'", len(args))
	}

	countQuery := "SELECT COUNT(*) FROM team " + where
	dataQuery := fmt.Sprintf(
		`SELECT id, name FROM team %s ORDER BY name, id LIMIT $%d OFFSET $%d`,
		where, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), limit, offset)

	var total int
	var groups []domain.Group

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count teams: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query teams: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Group, 0, limit)
		for rows.Next() {
			var g domain.Group
			if err := rows.Scan(&g.ID, &g.Name); err != nil {
				return fmt.Errorf("scan team: %w", err)
			}
			g.Active = true
			result = append(result, g)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate teams: %w", err)
		}
		groups = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}

// SearchSupportGroups implements GroupRepository.
//
// *** A DIFFERENT TABLE FROM SearchGroups. *** SearchGroups lists the team
// registry (team), whose ids are not "group" ids; an incident's assignment
// group is a "group" row. So this reads "group", limited to the same set
// IncidentRepository.IsSupportGroup checks a create against. Every row it
// returns is active (the set excludes is_active = FALSE); a group with no
// name is listed under its id.
func (r *groupRepo) SearchSupportGroups(ctx context.Context, searchQuery string, limit, offset int) ([]domain.Group, int, error) {
	where := ""
	args := []any{}
	if searchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(searchQuery)
		args = append(args, "%"+escaped+"%")
		where = fmt.Sprintf(" AND g.name ILIKE $%d ESCAPE '\\'", len(args))
	}
	set := supportGroupSetSQL + where

	countQuery := "SELECT COUNT(*) FROM (" + set + ") sg"
	dataQuery := fmt.Sprintf(`
		SELECT sg.id::text, COALESCE(sg.name, sg.id::text), p.id::text, p.name
		FROM (%s) sg
		LEFT JOIN "group" p ON p.id = sg.parent_id
		ORDER BY LOWER(COALESCE(sg.name, '')), sg.id
		LIMIT $%d OFFSET $%d`, set, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	var total int
	var groups []domain.Group

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count support groups: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query support groups: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Group, 0, limit)
		for rows.Next() {
			var g domain.Group
			var parentID, parentName *string
			if err := rows.Scan(&g.ID, &g.Name, &parentID, &parentName); err != nil {
				return fmt.Errorf("scan support group: %w", err)
			}
			g.Active = true
			if parentID != nil {
				g.Parent = &domain.GroupParentRef{ID: *parentID, Name: stringOrEmpty(parentName)}
			}
			result = append(result, g)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate support groups: %w", err)
		}
		groups = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}
