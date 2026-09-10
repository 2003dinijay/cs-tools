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

package registry

// TokenType identifies whether a registry token belongs to a person or a
// service integration.
type TokenType string

const (
	TokenTypeService TokenType = "Service"
	TokenTypeUser    TokenType = "User"
)

// TokenCreatePayload is the request body for POST /robot-accounts.
type TokenCreatePayload struct {
	AccountName string    `json:"accountName"`
	ProjectKey  string    `json:"projectKey"`
	RobotName   string    `json:"robotName"`
	SnAccountID string    `json:"snAccountId"`
	SnProjectID string    `json:"snProjectId"`
	TokenType   TokenType `json:"tokenType"`
	CreatedFor  string    `json:"createdFor"`
	CreatedBy   string    `json:"createdBy"`
}

// TokenCreationResponse is the response for creating or regenerating a token
// — an open record upstream; Secret is the only field this backend reads
// directly, the rest is passed through as-is.
type TokenCreationResponse struct {
	Secret string  `json:"secret"`
	Name   *string `json:"name,omitempty"`
}

// Permission is one access grant on a registry token.
type Permission struct {
	Namespace string `json:"namespace"`
}

// Token is a registry token as returned by the registry service.
type Token struct {
	ID           *int64       `json:"id,omitempty"`
	Name         string       `json:"name"`
	DisplayName  *string      `json:"displayName,omitempty"`
	Description  string       `json:"description"`
	CreationTime *string      `json:"creationTime,omitempty"`
	TokenType    *TokenType   `json:"tokenType,omitempty"`
	CreatedFor   *string      `json:"createdFor,omitempty"`
	CreatedBy    *string      `json:"createdBy,omitempty"`
	ExpiresAt    *int64       `json:"expiresAt,omitempty"`
	Disable      bool         `json:"disable"`
	Duration     int          `json:"duration"`
	Permissions  []Permission `json:"permissions"`
}

// TokenSearchPayload is the request body for POST /robot-accounts/search.
type TokenSearchPayload struct {
	SnAccountID string  `json:"snAccountId"`
	SnProjectID string  `json:"snProjectId"`
	UserEmail   *string `json:"userEmail,omitempty"`
	IsAdmin     bool    `json:"isAdmin"`
}

// IntegrationUser is a service-account user associated with a project.
type IntegrationUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// TokenDescriptionInfo is derived by parsing a Token's opaque Description
// field — see DeriveTokenInfoFromDescription.
type TokenDescriptionInfo struct {
	SnAccountID string
	SnProjectID string
	TokenType   TokenType
	CreatedFor  string
	CreatedBy   string
}
