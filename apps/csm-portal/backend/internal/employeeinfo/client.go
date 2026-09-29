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

// Package employeeinfo is a GraphQL client for SupportPortalLite's
// employee-info service, ported from the Ballerina backend's
// modules/userinfo package (client.bal, employee.bal, types.bal).
package employeeinfo

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

// Config holds the configuration for the employee-info service client.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
}

// Client is a GraphQL client for the employee-info service, authenticated
// via the OAuth2 client credentials grant.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client that authenticates against the employee-info
// GraphQL service using the OAuth2 client credentials grant type.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &Client{http: httpClient, baseURL: cfg.BaseURL}
}

// Employee is the employee-info service's representation of an employee —
// mirrors Ballerina modules/userinfo/types.bal's Employee.
type Employee struct {
	FirstName         string  `json:"firstName"`
	LastName          string  `json:"lastName"`
	EmployeeThumbnail *string `json:"employeeThumbnail"`
}

type employeeData struct {
	Employee Employee `json:"employee"`
}

// GetEmployeeData resolves an Employee by work email.
func (c *Client) GetEmployeeData(ctx context.Context, workEmail string) (*Employee, error) {
	const query = `
        query employeeQuery ($workEmail: String!) {
            employee(email: $workEmail) {
                firstName,
                lastName,
                employeeThumbnail
            }
        }
    `

	data, err := doGraphQL[employeeData](ctx, c.http, c.baseURL, query,
		map[string]any{"workEmail": workEmail}, "employeeinfo: getEmployeeData")
	if err != nil {
		return nil, err
	}
	return &data.Employee, nil
}
