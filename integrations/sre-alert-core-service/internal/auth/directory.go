// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// ErrDirectoryUnavailable means there is no usable copy of integration_users: none loaded yet, or the last good one is older than MaxStale.
var ErrDirectoryUnavailable = errors.New("integration_users copy unavailable")

// DirectoryConfig tunes the in-memory copy of integration_users.
type DirectoryConfig struct {
	// QueryTimeout bounds one refresh query.
	QueryTimeout time.Duration
	// RefreshInterval is how often the copy is reloaded; a new, disabled or rotated user takes up to this long to apply.
	RefreshInterval time.Duration
	// MaxStale is how long the last good copy keeps serving while refreshes fail.
	MaxStale time.Duration
}

// Directory keeps every integration_users row in memory and reloads it in the background, so /alertz never waits on Postgres.
type Directory struct {
	repo   *UserRepo
	logger *slog.Logger
	cfg    DirectoryConfig
	snap   atomic.Pointer[directorySnapshot]
}

type directorySnapshot struct {
	users    map[string]User
	loadedAt time.Time
}

// NewDirectory returns an empty copy; call Refresh once at startup and Run in a goroutine.
func NewDirectory(repo *UserRepo, logger *slog.Logger, cfg DirectoryConfig) *Directory {
	return &Directory{repo: repo, logger: logger, cfg: cfg}
}

// Refresh reloads every row in one query; on failure the previous copy stays in place.
func (d *Directory) Refresh(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.QueryTimeout)
	defer cancel()
	users, err := d.repo.List(ctx)
	if err != nil {
		return err
	}
	byName := make(map[string]User, len(users))
	for _, u := range users {
		byName[u.Username] = u
	}
	d.store(byName, time.Now())
	return nil
}

func (d *Directory) store(users map[string]User, loadedAt time.Time) {
	d.snap.Store(&directorySnapshot{users: users, loadedAt: loadedAt})
}

// Run refreshes every RefreshInterval until ctx ends.
func (d *Directory) Run(ctx context.Context) {
	ticker := time.NewTicker(d.cfg.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if err := d.Refresh(ctx); err != nil && ctx.Err() == nil {
			d.logger.Warn("integration_users refresh failed; keeping the last good copy", "error", err)
		}
	}
}

// Lookup returns username's row from the copy: ErrUserNotFound when it has no such row, ErrDirectoryUnavailable when the copy can't be trusted.
func (d *Directory) Lookup(username string) (User, error) {
	s := d.snap.Load()
	if s == nil {
		return User{}, fmt.Errorf("%w: not loaded yet", ErrDirectoryUnavailable)
	}
	if age := time.Since(s.loadedAt); age > d.cfg.MaxStale {
		return User{}, fmt.Errorf("%w: copy is %v old", ErrDirectoryUnavailable, age.Round(time.Second))
	}
	u, ok := s.users[username]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}
