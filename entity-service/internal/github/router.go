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

package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrNoToken is returned for a call to a repository that no configured token
// covers: it is not in GITHUB_REPO_TOKENS (by repository or owner) and
// GITHUB_TOKEN is unset.
var ErrNoToken = errors.New("github: no token configured for this repository")

// Router sends each call with the token for the repository it is about.
//
// ONE TOKEN PER REPOSITORY, AS SERVICENOW HAD. Each mapped repository can
// carry its own fine-grained PAT, and a fine-grained PAT reaches the
// repositories of one owner only -- so a single GITHUB_TOKEN cannot serve
// repositories under different owners, and a repository its token cannot see
// answers 404, which FileContent reads as "no config file": the issue is then
// skipped as unmapped, with no error anywhere. Routing per repository closes
// that.
//
// A call is routed by "owner/repository" first, then by "owner" (one token for
// every repository of an organisation), then to the fallback (GITHUB_TOKEN).
// Router has the same methods as *Client, so every consumer's interface takes
// it unchanged.
type Router struct {
	fallback *Client
	byKey    map[string]*Client
}

// ParseRepoTokens reads GITHUB_REPO_TOKENS: a JSON object of "owner/repository"
// (or "owner") to token. Keys match case-insensitively, as GitHub routes. An
// empty value means no per-repository tokens. Errors never quote a token.
func ParseRepoTokens(raw string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	var in map[string]string
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return nil, errors.New(`GITHUB_REPO_TOKENS: not a JSON object of "owner/repository" to token`)
	}
	for key, tok := range in {
		k := strings.ToLower(strings.TrimSpace(key))
		parts := strings.Split(k, "/")
		if k == "" || len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
			return nil, fmt.Errorf(`GITHUB_REPO_TOKENS: key %q is not "owner/repository" or "owner"`, key)
		}
		if strings.TrimSpace(tok) == "" {
			return nil, fmt.Errorf("GITHUB_REPO_TOKENS: %q has no token", key)
		}
		if _, dup := out[k]; dup {
			return nil, fmt.Errorf("GITHUB_REPO_TOKENS: %q is listed twice", key)
		}
		out[k] = strings.TrimSpace(tok)
	}
	return out, nil
}

// NewRouter builds a client per token. base carries everything but the token;
// fallbackToken (GITHUB_TOKEN) may be empty.
func NewRouter(base Config, fallbackToken string, repoTokens map[string]string) *Router {
	r := &Router{byKey: make(map[string]*Client, len(repoTokens))}
	if strings.TrimSpace(fallbackToken) != "" {
		cfg := base
		cfg.Token = fallbackToken
		r.fallback = NewClient(cfg)
	}
	for key, tok := range repoTokens {
		cfg := base
		cfg.Token = tok
		r.byKey[strings.ToLower(key)] = NewClient(cfg)
	}
	return r
}

// For returns the client for owner/repository.
func (r *Router) For(owner, repository string) (*Client, error) {
	o, repo := strings.ToLower(strings.TrimSpace(owner)), strings.ToLower(strings.TrimSpace(repository))
	if c, ok := r.byKey[o+"/"+repo]; ok {
		return c, nil
	}
	if c, ok := r.byKey[o]; ok {
		return c, nil
	}
	if r.fallback != nil {
		return r.fallback, nil
	}
	return nil, fmt.Errorf("%w: %s/%s", ErrNoToken, owner, repository)
}

// CreateIssue implements *Client.CreateIssue with the repository's token.
func (r *Router) CreateIssue(ctx context.Context, owner, repository, title, body string, labels []string) (*CreatedIssue, error) {
	c, err := r.For(owner, repository)
	if err != nil {
		return nil, err
	}
	return c.CreateIssue(ctx, owner, repository, title, body, labels)
}

// CreateComment implements *Client.CreateComment with the issue's repository token.
func (r *Router) CreateComment(ctx context.Context, issue Issue, body string) (*Comment, error) {
	c, err := r.For(issue.Owner, issue.Repository)
	if err != nil {
		return nil, err
	}
	return c.CreateComment(ctx, issue, body)
}

// ListComments implements *Client.ListComments with the issue's repository token.
func (r *Router) ListComments(ctx context.Context, issue Issue) ([]Comment, error) {
	c, err := r.For(issue.Owner, issue.Repository)
	if err != nil {
		return nil, err
	}
	return c.ListComments(ctx, issue)
}

// SetLabels implements *Client.SetLabels with the issue's repository token.
func (r *Router) SetLabels(ctx context.Context, issue Issue, labels []string) error {
	c, err := r.For(issue.Owner, issue.Repository)
	if err != nil {
		return err
	}
	return c.SetLabels(ctx, issue, labels)
}

// AddLabel implements *Client.AddLabel with the issue's repository token.
func (r *Router) AddLabel(ctx context.Context, issue Issue, label string) error {
	c, err := r.For(issue.Owner, issue.Repository)
	if err != nil {
		return err
	}
	return c.AddLabel(ctx, issue, label)
}

// RemoveLabel implements *Client.RemoveLabel with the issue's repository token.
func (r *Router) RemoveLabel(ctx context.Context, issue Issue, label string) error {
	c, err := r.For(issue.Owner, issue.Repository)
	if err != nil {
		return err
	}
	return c.RemoveLabel(ctx, issue, label)
}

// SetState implements *Client.SetState with the issue's repository token.
func (r *Router) SetState(ctx context.Context, issue Issue, state State) error {
	c, err := r.For(issue.Owner, issue.Repository)
	if err != nil {
		return err
	}
	return c.SetState(ctx, issue, state)
}

// Dispatch implements *Client.Dispatch with the repository's token.
func (r *Router) Dispatch(ctx context.Context, owner, repository, eventType string, payload map[string]any) error {
	c, err := r.For(owner, repository)
	if err != nil {
		return err
	}
	return c.Dispatch(ctx, owner, repository, eventType, payload)
}

// FileContent implements *Client.FileContent with the repository's token.
//
// No token for the repository is an error here, never (nil, nil): that would
// read as "the repository has no config file" and skip its issues silently.
func (r *Router) FileContent(ctx context.Context, owner, repository, path string) ([]byte, error) {
	c, err := r.For(owner, repository)
	if err != nil {
		return nil, err
	}
	return c.FileContent(ctx, owner, repository, path)
}
