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

package util

import (
	"testing"
)

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     string
	}{
		{
			name:     "Email with @ and dot",
			username: "user@example.com",
			want:     "user_40example_2ecom",
		},
		{
			name:     "Email with subdomain",
			username: "john.doe@mail.example.com",
			want:     "john_2edoe_40mail_2eexample_2ecom",
		},
		{
			name:     "Username with forward slash",
			username: "DEFAULT/user@example.com",
			want:     "DEFAULT_2fuser_40example_2ecom",
		},
		{
			name:     "Username with plus",
			username: "user+tag@example.com",
			want:     "user_2btag_40example_2ecom",
		},
		{
			name:     "Literal underscore is escaped too (it is the escape marker)",
			username: "user_example_com",
			want:     "user_5fexample_5fcom",
		},
		{
			name:     "Empty string",
			username: "",
			want:     "",
		},
		{
			name:     "Multiple special chars",
			username: "user@name.with+slash/test",
			want:     "user_40name_2ewith_2bslash_2ftest",
		},
		{
			name:     "Hyphen passes through unescaped",
			username: "user-name",
			want:     "user-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeUsername(tt.username)
			if got != tt.want {
				t.Errorf("SanitizeUsername(%q) = %q, want %q", tt.username, got, tt.want)
			}
		})
	}
}

// TestSanitizeUsername_NoCollision proves fix #9: two distinct valid usernames
// that collided under the old lossy substitution ("a.b@example.com" and
// "a_b@example.com" both sanitized to "a_b@example.com") now sanitize to
// different strings, since a literal '_' is itself escaped.
func TestSanitizeUsername_NoCollision(t *testing.T) {
	inputs := []string{
		"a.b@example.com",
		"a_b@example.com",
		"a/b@example.com",
		"a+b@example.com",
	}

	seen := make(map[string]string, len(inputs))
	for _, in := range inputs {
		out := SanitizeUsername(in)
		if prior, ok := seen[out]; ok {
			t.Errorf("collision: SanitizeUsername(%q) and SanitizeUsername(%q) both produced %q", prior, in, out)
		}
		seen[out] = in
	}
}

func TestIsLikelyEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Valid email",
			input: "user@example.com",
			want:  true,
		},
		{
			name:  "Valid email with subdomain",
			input: "john.doe@mail.example.com",
			want:  true,
		},
		{
			name:  "Valid email with plus",
			input: "user+tag@example.com",
			want:  true,
		},
		{
			name:  "Valid email with special chars",
			input: "user!#$@example.com",
			want:  true,
		},
		{
			name:  "Valid email with accented letters",
			input: "josé@example.com",
			want:  true,
		},
		{
			name:  "No @ symbol",
			input: "username",
			want:  false,
		},
		{
			name:  "No domain",
			input: "user@",
			want:  false,
		},
		{
			name:  "No TLD",
			input: "user@example",
			want:  false,
		},
		{
			name:  "Empty string",
			input: "",
			want:  false,
		},
		{
			name:  "Multiple @ symbols",
			input: "user@@example.com",
			want:  false,
		},
		{
			name:  "Spaces in email",
			input: "user @example.com",
			want:  false,
		},
		{
			name:  "Valid with numbers",
			input: "user123@example123.com",
			want:  true,
		},
		{
			name:  "TLD too long (>10 chars)",
			input: "user@example.verylongtld",
			want:  false, // Regex restricts TLD length
		},
		{
			name:  "Starting with dot",
			input: ".user@example.com",
			want:  false, // Regex starts with non-dot atom
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLikelyEmail(tt.input)
			if got != tt.want {
				t.Errorf("IsLikelyEmail(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateFolderName(t *testing.T) {
	tests := []struct {
		name       string
		folderName string
		wantError  bool
	}{
		{
			name:       "Valid folder name",
			folderName: "project1",
			wantError:  false,
		},
		{
			name:       "Valid folder with underscore",
			folderName: "project_folder_1",
			wantError:  false,
		},
		{
			name:       "Valid folder with hyphen",
			folderName: "project-folder-1",
			wantError:  false,
		},
		{
			name:       "Empty folder name",
			folderName: "",
			wantError:  true,
		},
		{
			name:       "Folder with parent directory traversal",
			folderName: "../etc",
			wantError:  true,
		},
		{
			name:       "Folder with double dots in middle",
			folderName: "folder..name",
			wantError:  true,
		},
		{
			name:       "Folder with forward slash",
			folderName: "folder/name",
			wantError:  true,
		},
		{
			name:       "Folder with backslash",
			folderName: "folder\\name",
			wantError:  true,
		},
		{
			name:       "Folder with leading slash",
			folderName: "/folder",
			wantError:  true,
		},
		{
			name:       "Folder with trailing slash",
			folderName: "folder/",
			wantError:  true,
		},
		{
			// filepath.Join(basePath, ".") resolves to basePath itself, so a
			// bare "." must be rejected explicitly, not just ".." and slashes.
			name:       "Single dot",
			folderName: ".",
			wantError:  true,
		},
		{
			name:       "Double dot",
			folderName: "..",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFolderName(tt.folderName)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateFolderName(%q) error = %v, wantError %v", tt.folderName, err, tt.wantError)
			}
		})
	}
}

func TestInitEmailRegex_Invalid(t *testing.T) {
	// Savely backup and restore
	oldRegex := emailRegex
	defer func() { emailRegex = oldRegex }()

	// Reset to nil to simulate failure at start if needed,
	// though InitEmailRegex should preserve previous value on error.
	emailRegex = nil

	err := InitEmailRegex("[invalid")
	if err == nil {
		t.Error("InitEmailRegex with invalid pattern should return error")
	}

	// Should not panic and should return false
	if IsLikelyEmail("test@example.com") {
		t.Error("IsLikelyEmail should return false when emailRegex is nil")
	}

	// Initialize with valid pattern
	err = InitEmailRegex("^[a-z]+$")
	if err != nil {
		t.Errorf("InitEmailRegex failed with valid pattern: %v", err)
	}

	if !IsLikelyEmail("abc") {
		t.Error("IsLikelyEmail should return true for valid match")
	}
	if IsLikelyEmail("123") {
		t.Error("IsLikelyEmail should return false for non-match")
	}
}
