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

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// CreateToken calls POST /robot-accounts.
func (c *Client) CreateToken(ctx context.Context, req TokenCreatePayload) (TokenCreationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return TokenCreationResponse{}, fmt.Errorf("registry: encode create token request: %w", err)
	}
	raw, err := c.do(ctx, http.MethodPost, "/robot-accounts", body)
	if err != nil {
		return TokenCreationResponse{}, err
	}
	var out TokenCreationResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return TokenCreationResponse{}, fmt.Errorf("registry: decode create token response: %w", err)
	}
	return out, nil
}

// SearchTokens calls POST /robot-accounts/search.
func (c *Client) SearchTokens(ctx context.Context, req TokenSearchPayload) ([]Token, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("registry: encode search tokens request: %w", err)
	}
	raw, err := c.do(ctx, http.MethodPost, "/robot-accounts/search", body)
	if err != nil {
		return nil, err
	}
	var out []Token
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("registry: decode search tokens response: %w", err)
	}
	return out, nil
}

// GetTokenByID calls GET /robot-accounts/{tokenId}.
func (c *Client) GetTokenByID(ctx context.Context, tokenID string) (Token, error) {
	raw, err := c.do(ctx, http.MethodGet, "/robot-accounts/"+url.PathEscape(tokenID), nil)
	if err != nil {
		return Token{}, err
	}
	var out Token
	if err := json.Unmarshal(raw, &out); err != nil {
		return Token{}, fmt.Errorf("registry: decode token response: %w", err)
	}
	return out, nil
}

// DeleteToken calls DELETE /robot-accounts/{tokenId}.
func (c *Client) DeleteToken(ctx context.Context, tokenID string) error {
	_, err := c.do(ctx, http.MethodDelete, "/robot-accounts/"+url.PathEscape(tokenID), nil)
	return err
}

// RegenerateToken calls POST /robot-accounts/{tokenId}/regenerate-token.
func (c *Client) RegenerateToken(ctx context.Context, tokenID string) (TokenCreationResponse, error) {
	raw, err := c.do(ctx, http.MethodPost, "/robot-accounts/"+url.PathEscape(tokenID)+"/regenerate-token", []byte("{}"))
	if err != nil {
		return TokenCreationResponse{}, err
	}
	var out TokenCreationResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return TokenCreationResponse{}, fmt.Errorf("registry: decode regenerate token response: %w", err)
	}
	return out, nil
}

// GetIntegrationUsersByProjectID calls GET /projects/{projectId}/integration-users.
func (c *Client) GetIntegrationUsersByProjectID(ctx context.Context, projectID string) ([]IntegrationUser, error) {
	raw, err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectID)+"/integration-users", nil)
	if err != nil {
		return nil, err
	}
	var out []IntegrationUser
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("registry: decode integration users response: %w", err)
	}
	return out, nil
}

// descriptionPartCount is the number of "##"-delimited parts a valid token
// description must have — see DeriveTokenInfoFromDescription.
const descriptionPartCount = 5

// DeriveTokenInfoFromDescription parses a Token's opaque Description field,
// formatted as "<snAccountId>##<snProjectId>##<TokenType>##<createdFor>##<createdBy>".
// This is the only way to recover a token's owning project/type/owner for
// authorization purposes — the registry service itself has no concept of
// the caller's project membership.
func DeriveTokenInfoFromDescription(description string) (TokenDescriptionInfo, error) {
	parts := strings.Split(description, "##")
	if len(parts) != descriptionPartCount {
		return TokenDescriptionInfo{}, fmt.Errorf("registry: invalid token description format")
	}
	tokenType := TokenType(parts[2])
	if tokenType != TokenTypeService && tokenType != TokenTypeUser {
		return TokenDescriptionInfo{}, fmt.Errorf("registry: invalid token type in description: %q", parts[2])
	}
	return TokenDescriptionInfo{
		SnAccountID: parts[0],
		SnProjectID: parts[1],
		TokenType:   tokenType,
		CreatedFor:  parts[3],
		CreatedBy:   parts[4],
	}, nil
}
