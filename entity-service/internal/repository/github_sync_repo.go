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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepoMapping is a GitHub repository and what it routes to.
type RepoMapping struct {
	ProductID   string
	ProductName string
	// TeamID is who a change request from this repository is assigned to.
	// Empty when nobody has been nominated yet -- a routing gap, not a reason
	// to reject the event.
	TeamID string
}

// GithubChangeRequest is the slice of a change request the sync reads.
type GithubChangeRequest struct {
	ID     string
	Number string
	State  string
}

// ErrDeliverySeen means this webhook delivery has already been handled.
var ErrDeliverySeen = errors.New("github: delivery already processed")

// GithubSyncRepository is the database side of the GitHub change-request sync.
type GithubSyncRepository interface {
	// RepoMapping resolves a repository to its product and team. Not found is
	// a nil mapping and no error: a repository we do not map is one we do not
	// handle, which is ordinary rather than exceptional.
	RepoMapping(ctx context.Context, owner, repository string) (*RepoMapping, error)
	// ChangeRequestByGitReference finds the change request linked to an issue.
	ChangeRequestByGitReference(ctx context.Context, issueURL string) (*GithubChangeRequest, error)
	// ClaimDelivery records a delivery, returning ErrDeliverySeen if another
	// attempt already recorded it.
	ClaimDelivery(ctx context.Context, deliveryID, event, action string) error
	// ReleaseDelivery removes the claim so GitHub's retry can be processed.
	ReleaseDelivery(ctx context.Context, deliveryID string) error
	// LinkDelivery records which change request a delivery resolved to.
	LinkDelivery(ctx context.Context, deliveryID, changeRequestID string) error
}

type githubSyncRepository struct {
	db *pgxpool.Pool
}

// NewGithubSyncRepository constructs the GitHub sync reader/writer.
func NewGithubSyncRepository(db *pgxpool.Pool) GithubSyncRepository {
	return &githubSyncRepository{db: db}
}

func (r *githubSyncRepository) RepoMapping(ctx context.Context, owner, repository string) (*RepoMapping, error) {
	// Lower-cased on both sides: GitHub routes case-insensitively while
	// preserving the case a repository was created with, so "Choreo" and
	// "choreo" are the same repository. ServiceNow compared exactly and every
	// mismatch fell through to a literal "NULL" assignment group.
	const query = `
		SELECT p.id::text,
		       p.name,
		       COALESCE(gr.team_id::text, '')
		FROM product_github_repo gr
		JOIN product p ON p.id = gr.product_id
		WHERE lower(gr.owner) = lower($1)
		  AND lower(gr.repository) = lower($2)
		  AND gr.is_active`

	var m RepoMapping
	err := r.db.QueryRow(ctx, query, owner, repository).Scan(&m.ProductID, &m.ProductName, &m.TeamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("github: repo mapping for %s/%s: %w", owner, repository, err)
	}
	return &m, nil
}

func (r *githubSyncRepository) ChangeRequestByGitReference(ctx context.Context, issueURL string) (*GithubChangeRequest, error) {
	const query = `
		SELECT cr.id::text, wi.number, COALESCE(cr.state::text, '')
		FROM change_request cr
		JOIN work_item wi ON wi.id = cr.id
		WHERE cr.git_reference = $1`

	var cr GithubChangeRequest
	err := r.db.QueryRow(ctx, query, issueURL).Scan(&cr.ID, &cr.Number, &cr.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("github: change request for %s: %w", issueURL, err)
	}
	return &cr, nil
}

func (r *githubSyncRepository) ClaimDelivery(ctx context.Context, deliveryID, event, action string) error {
	const query = `
		INSERT INTO github_webhook_delivery (delivery_id, event, action)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (delivery_id) DO NOTHING`

	tag, err := r.db.Exec(ctx, query, deliveryID, event, action)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDeliverySeen
		}
		return fmt.Errorf("github: claim delivery %s: %w", deliveryID, err)
	}
	// DO NOTHING reports zero rows when the row was already there, which is
	// the ordinary way a redelivery arrives -- not an error condition.
	if tag.RowsAffected() == 0 {
		return ErrDeliverySeen
	}
	return nil
}

func (r *githubSyncRepository) ReleaseDelivery(ctx context.Context, deliveryID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM github_webhook_delivery WHERE delivery_id = $1`, deliveryID)
	if err != nil {
		return fmt.Errorf("github: release delivery %s: %w", deliveryID, err)
	}
	return nil
}

func (r *githubSyncRepository) LinkDelivery(ctx context.Context, deliveryID, changeRequestID string) error {
	const query = `
		UPDATE github_webhook_delivery
		SET change_request_id = $2::uuid
		WHERE delivery_id = $1`
	if _, err := r.db.Exec(ctx, query, deliveryID, changeRequestID); err != nil {
		return fmt.Errorf("github: link delivery %s: %w", deliveryID, err)
	}
	return nil
}
