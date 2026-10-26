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
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// RotaAccessRepository stores who may change the rotas beyond the team leads:
// the rota admin roles (one per family, in user_role -- migrations 0156 and
// 0200) and the rota markers (one person, one team -- migration 0225).
//
// Plain data operations. Who may grant what is the service's decision.
type RotaAccessRepository interface {
	// ListRotaAdmins is every holder of a rota admin role.
	ListRotaAdmins(ctx context.Context) ([]domain.RotaAdminGrant, error)
	// ListRotaMarkers is every rota marker grant in the given families, or in
	// every family when families is empty.
	ListRotaMarkers(ctx context.Context, families []string) ([]domain.RotaMarkerGrant, error)

	// GrantRotaAdmin gives the user the named rota admin role, reporting
	// whether it was new. RevokeRotaAdmin removes it, reporting whether
	// there was one.
	GrantRotaAdmin(ctx context.Context, userID, roleName, actorEmail string) (bool, error)
	RevokeRotaAdmin(ctx context.Context, userID, roleName, actorEmail string) (bool, error)
	// GrantRotaMarker and RevokeRotaMarker are the same for one team.
	GrantRotaMarker(ctx context.Context, userID, teamKey, actorEmail string) (bool, error)
	RevokeRotaMarker(ctx context.Context, userID, teamKey, actorEmail string) (bool, error)

	// AddTeamMember puts the user on the team as an engineer, reporting
	// whether they were not on it already.
	AddTeamMember(ctx context.Context, userID, teamKey, actorEmail string) (bool, error)
	// RemoveTeamMember takes the user off the team, and their rota marker
	// grant for it with them, unless something keeps them on it: a lead
	// role (when allowLead is false), a Case Alerts responder tier, or turns
	// on the team that have not ended. Nothing is written when one does;
	// the result says which.
	RemoveTeamMember(ctx context.Context, userID, teamKey, actorEmail string, allowLead bool) (domain.TeamMemberRemoval, error)

	// UserIsInternal is whether the user exists and is internal staff.
	UserIsInternal(ctx context.Context, userID string) (bool, error)
	// RosteredTeamFamily is the family of a team on the schedule, and false
	// for a team that is not on it (or does not exist).
	RosteredTeamFamily(ctx context.Context, teamKey string) (string, bool, error)
}

type rotaAccessRepository struct{ db *pgxpool.Pool }

// NewRotaAccessRepository constructs a RotaAccessRepository over the pool.
func NewRotaAccessRepository(db *pgxpool.Pool) RotaAccessRepository {
	return &rotaAccessRepository{db: db}
}

// ListRotaAdmins implements RotaAccessRepository.
//
// user_role has no unique (user_id, role_id) and duplicates exist, so one row
// per person and role, the earliest grant first.
func (r *rotaAccessRepository) ListRotaAdmins(ctx context.Context) ([]domain.RotaAdminGrant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (u.id, ro.name)
		       `+rotaAdminFamilyOfRole+`, u.id::text, COALESCE(u.name, ''), COALESCE(u.email, ''),
		       COALESCE(ur.created_by, ''), ur.created_on,
		       u.user_type = 'INTERNAL'::user_type_enum
		  FROM user_role ur
		  JOIN role ro   ON ro.id = ur.role_id
		  JOIN "user" u  ON u.id = ur.user_id
		 WHERE ro.name IN ('cre_rota_admin', 'sre_rota_admin', 'sme_rota_admin')
		 ORDER BY u.id, ro.name, ur.created_on`)
	if err != nil {
		return nil, fmt.Errorf("list rota admins: %w", err)
	}
	defer rows.Close()
	out := []domain.RotaAdminGrant{}
	for rows.Next() {
		var g domain.RotaAdminGrant
		if err := rows.Scan(&g.Family, &g.UserID, &g.Name, &g.Email, &g.GrantedBy, &g.GrantedOn, &g.Active); err != nil {
			return nil, fmt.Errorf("scan rota admin: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ListRotaMarkers implements RotaAccessRepository.
func (r *rotaAccessRepository) ListRotaMarkers(ctx context.Context, families []string) ([]domain.RotaMarkerGrant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.team_key, `+teamFamilyExpr+`, u.id::text, COALESCE(u.name, ''), COALESCE(u.email, ''),
		       m.created_by, m.created_on,
		       u.user_type = 'INTERNAL'::user_type_enum
		         AND EXISTS (SELECT 1 FROM team_member tm WHERE tm.team_id = t.id AND tm.user_id = u.id)
		  FROM team_schedule_rota_marker m
		  JOIN team t   ON t.key = m.team_key
		  JOIN "user" u ON u.id = m.user_id
		 WHERE cardinality($1::text[]) = 0 OR `+teamFamilyExpr+` = ANY($1::text[])
		 ORDER BY m.team_key, lower(COALESCE(u.name, '')), u.id`, families)
	if err != nil {
		return nil, fmt.Errorf("list rota markers: %w", err)
	}
	defer rows.Close()
	out := []domain.RotaMarkerGrant{}
	for rows.Next() {
		var g domain.RotaMarkerGrant
		if err := rows.Scan(&g.TeamKey, &g.Family, &g.UserID, &g.Name, &g.Email, &g.GrantedBy, &g.GrantedOn, &g.Active); err != nil {
			return nil, fmt.Errorf("scan rota marker: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// inTx runs fn in a transaction that names the actor for the audit trail.
func (r *rotaAccessRepository) inTx(ctx context.Context, actorEmail string, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameTheActor(ctx, tx, actorEmail); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GrantRotaAdmin implements RotaAccessRepository.
//
// "Not already held" is asked in the insert itself, since user_role has no
// unique key to conflict on. The role row is looked up by name, as the
// migrations that seed it do: the ServiceNow sync may have created it under
// another id.
func (r *rotaAccessRepository) GrantRotaAdmin(ctx context.Context, userID, roleName, actorEmail string) (bool, error) {
	var added bool
	err := r.inTx(ctx, actorEmail, func(tx pgx.Tx) error {
		var roleID string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM role WHERE name = $1`, roleName).Scan(&roleID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("role %s is not seeded", roleName)
			}
			return fmt.Errorf("look up role %s: %w", roleName, err)
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
			SELECT gen_random_uuid(), now(), now(), $3, $3, $1::uuid, $2::uuid
			 WHERE NOT EXISTS (SELECT 1 FROM user_role WHERE user_id = $1::uuid AND role_id = $2::uuid)`,
			userID, roleID, actorEmail)
		if err != nil {
			return fmt.Errorf("grant %s: %w", roleName, err)
		}
		added = tag.RowsAffected() > 0
		return nil
	})
	return added, err
}

// RevokeRotaAdmin implements RotaAccessRepository. Every copy goes, since
// duplicates exist and any one left behind would keep the role.
func (r *rotaAccessRepository) RevokeRotaAdmin(ctx context.Context, userID, roleName, actorEmail string) (bool, error) {
	var removed bool
	err := r.inTx(ctx, actorEmail, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			DELETE FROM user_role ur USING role ro
			 WHERE ur.role_id = ro.id AND ur.user_id = $1::uuid AND ro.name = $2`, userID, roleName)
		if err != nil {
			return fmt.Errorf("revoke %s: %w", roleName, err)
		}
		removed = tag.RowsAffected() > 0
		return nil
	})
	return removed, err
}

// GrantRotaMarker implements RotaAccessRepository.
func (r *rotaAccessRepository) GrantRotaMarker(ctx context.Context, userID, teamKey, actorEmail string) (bool, error) {
	var added bool
	err := r.inTx(ctx, actorEmail, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO team_schedule_rota_marker (created_by, updated_by, user_id, team_key)
			VALUES ($3, $3, $1::uuid, lower($2))
			ON CONFLICT (user_id, team_key) DO NOTHING`, userID, teamKey, actorEmail)
		if err != nil {
			return fmt.Errorf("grant rota marker: %w", err)
		}
		added = tag.RowsAffected() > 0
		return nil
	})
	return added, err
}

// RevokeRotaMarker implements RotaAccessRepository.
func (r *rotaAccessRepository) RevokeRotaMarker(ctx context.Context, userID, teamKey, actorEmail string) (bool, error) {
	var removed bool
	err := r.inTx(ctx, actorEmail, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			DELETE FROM team_schedule_rota_marker
			 WHERE user_id = $1::uuid AND team_key = lower($2)`, userID, teamKey)
		if err != nil {
			return fmt.Errorf("revoke rota marker: %w", err)
		}
		removed = tag.RowsAffected() > 0
		return nil
	})
	return removed, err
}

// AddTeamMember implements RotaAccessRepository.
//
// team_member has no unique key on (team, user), so "not already on it" is
// asked in the insert itself, under a lock on the team row so two adds of
// the same person cannot both see "not there". The id is given, not left
// to the column default, which not every database has (migration 0179).
func (r *rotaAccessRepository) AddTeamMember(ctx context.Context, userID, teamKey, actorEmail string) (bool, error) {
	var added bool
	err := r.inTx(ctx, actorEmail, func(tx pgx.Tx) error {
		var teamID string
		err := tx.QueryRow(ctx, `SELECT id::text FROM team WHERE key = lower($1) FOR UPDATE`, teamKey).Scan(&teamID)
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.NotFoundError{Msg: fmt.Sprintf("%s is not a team on the schedule", teamKey)}
		}
		if err != nil {
			return fmt.Errorf("lock team: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role)
			SELECT gen_random_uuid(), NOW(), NOW(), $3, $3, $2::uuid, $1::uuid, 'engineer'
			 WHERE NOT EXISTS (SELECT 1 FROM team_member m WHERE m.team_id = $2::uuid AND m.user_id = $1::uuid)`,
			userID, teamID, actorEmail)
		if err != nil {
			return fmt.Errorf("add team member: %w", err)
		}
		added = tag.RowsAffected() > 0
		return nil
	})
	return added, err
}

// leadRoles are the membership roles a rota admin may not take off a team:
// each one is a rung of the Case Alerts ladders, so removing it changes who
// is called.
var leadRoles = []string{"lead", "sub_lead", "americas_team_lead", "cre_head", "cs_head"}

// RemoveTeamMember implements RotaAccessRepository.
//
// The person's membership rows are locked first, then everything that keeps
// them on the team is read under that lock, and the rows are deleted only
// when nothing does -- so a turn or a tier added at the same moment cannot
// slip past the check. Turns are counted by ends_at, so one under way counts.
func (r *rotaAccessRepository) RemoveTeamMember(ctx context.Context, userID, teamKey, actorEmail string, allowLead bool) (domain.TeamMemberRemoval, error) {
	var out domain.TeamMemberRemoval
	err := r.inTx(ctx, actorEmail, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT m.role, COALESCE(m.alert_tier, '')
			  FROM team_member m JOIN team t ON t.id = m.team_id
			 WHERE t.key = lower($2) AND m.user_id = $1::uuid
			 ORDER BY m.id
			   FOR UPDATE OF m`, userID, teamKey)
		if err != nil {
			return fmt.Errorf("lock team membership: %w", err)
		}
		var lead bool
		for rows.Next() {
			var role, tier string
			if err := rows.Scan(&role, &tier); err != nil {
				rows.Close()
				return fmt.Errorf("scan team membership: %w", err)
			}
			out.Member = true
			if out.Role == "" || slices.Contains(leadRoles, role) {
				out.Role = role
			}
			if slices.Contains(leadRoles, role) {
				lead = true
			}
			if tier != "" {
				out.AlertTier = tier
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read team membership: %w", err)
		}
		if !out.Member {
			return nil
		}
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM team_schedule_assignment
			 WHERE user_id = $1::uuid AND team_key = lower($2) AND ends_at > NOW()`,
			userID, teamKey).Scan(&out.UpcomingTurns); err != nil {
			return fmt.Errorf("count upcoming turns: %w", err)
		}
		if (lead && !allowLead) || out.AlertTier != "" || out.UpcomingTurns > 0 {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM team_member m USING team t
			 WHERE t.id = m.team_id AND t.key = lower($2) AND m.user_id = $1::uuid`, userID, teamKey); err != nil {
			return fmt.Errorf("remove team member: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM team_schedule_rota_marker WHERE user_id = $1::uuid AND team_key = lower($2)`,
			userID, teamKey); err != nil {
			return fmt.Errorf("remove the rota marker of a team member: %w", err)
		}
		out.Removed = true
		return nil
	})
	return out, err
}

// UserIsInternal implements RotaAccessRepository.
func (r *rotaAccessRepository) UserIsInternal(ctx context.Context, userID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM "user"
		                WHERE id = $1::uuid
		                  AND user_type = 'INTERNAL'::user_type_enum
		                  AND COALESCE(is_active, TRUE))`, userID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check internal user: %w", err)
	}
	return ok, nil
}

// RosteredTeamFamily implements RotaAccessRepository.
func (r *rotaAccessRepository) RosteredTeamFamily(ctx context.Context, teamKey string) (string, bool, error) {
	var family string
	err := r.db.QueryRow(ctx, `
		SELECT `+teamFamilyExpr+`
		  FROM team t
		 WHERE t.key = lower($1) AND `+rosteredTeamWhere, teamKey).Scan(&family)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("team family: %w", err)
	}
	return family, true, nil
}
