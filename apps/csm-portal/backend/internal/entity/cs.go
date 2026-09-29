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

package entity

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// CSEntityConfig holds the configuration for the SupportPortalLite CS-side
// entity GraphQL service client — a separate, differently-hosted GraphQL
// endpoint from SalesEntityClient, though today configured with the same
// OAuth2 client-credentials app (ported from Ballerina
// modules/entity/cs.bal and its csClient in clients.bal).
type CSEntityConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
}

// CSEntityClient is a GraphQL client for the SupportPortalLite CS-side
// entity service, authenticated via the OAuth2 client credentials grant.
type CSEntityClient struct {
	http    *http.Client
	baseURL string
}

// NewCSEntityClient constructs a CSEntityClient that authenticates against
// the CS-side entity GraphQL service using the OAuth2 client credentials
// grant type.
func NewCSEntityClient(cfg CSEntityConfig) *CSEntityClient {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &CSEntityClient{http: httpClient, baseURL: cfg.BaseURL}
}

// User is the CS-side entity service's representation of a ServiceNow
// portal user, resolved by email — mirrors Ballerina modules/types.User.
type User struct {
	LockedOut bool   `json:"lockedOut"`
	UserID    string `json:"userId"`
}

type userData struct {
	User *User `json:"user"`
}

// GetUserByEmail resolves a ServiceNow portal User by email. Returns (nil,
// nil) when no user matches.
func (c *CSEntityClient) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	const query = `
		query userDetails($email: String!) {
			user(email: $email) {
				lockedOut
				userId
			}
		}`

	data, err := doGraphQL[userData](ctx, c.http, c.baseURL, query,
		map[string]any{"email": email}, "cs entity: getUserByEmail")
	if err != nil {
		return nil, err
	}
	return data.User, nil
}

// Project is the CS-side entity service's representation of a ServiceNow
// project, resolved by project key — mirrors Ballerina
// modules/types.Project.
type Project struct {
	AccountID                  string `json:"accountId"`
	ProjectID                  string `json:"projectId"`
	WSO2ClosureState           string `json:"wso2ClosureState"`
	EndDateClosureState        string `json:"endDateClosureState"`
	InvoiceDueDateClosureState string `json:"invoiceDueDateClosureState"`
}

type projectData struct {
	Project *Project `json:"project"`
}

// GetProjectByProjectKey resolves a Project by its project key. Returns
// (nil, nil) when no project matches.
func (c *CSEntityClient) GetProjectByProjectKey(ctx context.Context, projectKey string) (*Project, error) {
	const query = `
		query projectQuery($projectKey: String!) {
			project(projectKey: $projectKey){
				accountId
				projectId
				wso2ClosureState
				endDateClosureState
				invoiceDueDateClosureState
			}
		}`

	data, err := doGraphQL[projectData](ctx, c.http, c.baseURL, query,
		map[string]any{"projectKey": projectKey}, "cs entity: getProjectByProjectKey")
	if err != nil {
		return nil, err
	}
	return data.Project, nil
}

// ProjectContact is the CS-side entity service's representation of a
// project's invitation link, resolved by email + project ID — mirrors
// Ballerina modules/types.ProjectContact.
type ProjectContact struct {
	InvitationURL *string `json:"invitationUrl"`
}

type projectContactData struct {
	ProjectContact *ProjectContact `json:"projectContact"`
}

// GetProjectContactByEmail resolves a ProjectContact by email and project
// ID. Returns (nil, nil) when no project contact matches.
func (c *CSEntityClient) GetProjectContactByEmail(ctx context.Context, email, projectID string) (*ProjectContact, error) {
	const query = `
		query projectContact($email: String!, $projectId: String!){
			projectContact(email: $email, projectId: $projectId){
				invitationUrl
			}
		}
	`

	data, err := doGraphQL[projectContactData](ctx, c.http, c.baseURL, query,
		map[string]any{"email": email, "projectId": projectID}, "cs entity: getProjectContactByEmail")
	if err != nil {
		return nil, err
	}
	return data.ProjectContact, nil
}
