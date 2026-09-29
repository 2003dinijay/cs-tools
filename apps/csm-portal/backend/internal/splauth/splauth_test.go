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

package splauth

import "testing"

func TestIsAuthorized(t *testing.T) {
	tests := []struct {
		name          string
		userGroups    []string
		allowedGroups []string
		want          bool
	}{
		{
			name:          "user in one of the allowed groups",
			userGroups:    []string{"sales-team", "everyone"},
			allowedGroups: []string{"sales-team", "sa-team"},
			want:          true,
		},
		{
			name:          "user in none of the allowed groups",
			userGroups:    []string{"marketing-team"},
			allowedGroups: []string{"sales-team", "sa-team"},
			want:          false,
		},
		{
			name:          "empty user groups",
			userGroups:    nil,
			allowedGroups: []string{"sales-team"},
			want:          false,
		},
		{
			name:          "empty allowed groups denies everyone",
			userGroups:    []string{"sales-team"},
			allowedGroups: nil,
			want:          false,
		},
		{
			name:          "both empty",
			userGroups:    nil,
			allowedGroups: nil,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthorized(tt.userGroups, tt.allowedGroups); got != tt.want {
				t.Errorf("IsAuthorized(%v, %v) = %v, want %v", tt.userGroups, tt.allowedGroups, got, tt.want)
			}
		})
	}
}
