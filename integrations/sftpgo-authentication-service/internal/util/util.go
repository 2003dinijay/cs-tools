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
	"fmt"
	"io"
	"regexp"
	"strings"
)

var (
	// defaultEmailRegexPattern is the default pattern used for broad email address validation.
	// Supports accented letters (\p{L}), digits, and allowed special characters.
	defaultEmailRegexPattern = `^[\p{L}0-9!#$'%*+=?^_{|}~&-]+(?:\.[\p{L}0-9!#$'%*+=?^_{|}~&-]+)*@[\p{L}0-9.\-_]+\.[a-zA-Z]{2,10}$`

	// emailRegex is the compiled regular expression used for email validation.
	emailRegex *regexp.Regexp
)

// InitEmailRegex initializes the email regex with a custom pattern or uses the default.
func InitEmailRegex(pattern string) error {
	if pattern == "" {
		pattern = defaultEmailRegexPattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	emailRegex = re
	return nil
}

func init() {
	// Initialize with default pattern
	_ = InitEmailRegex("")
}

// usernameEscape is the marker byte SanitizeUsername uses to introduce an
// escape sequence. It is deliberately excluded from isSafeUsernameByte so
// that a literal occurrence of it is always escaped too -- see SanitizeUsername.
const usernameEscape = '_'

// SanitizeUsername converts u into a string containing only ASCII letters,
// digits, and hyphens, safe to use as a single filesystem path segment (an
// SFTPGo HomeDir).
//
// Every byte that is not in that safe set -- including a literal usernameEscape
// byte itself -- is replaced with a 3-character escape sequence: usernameEscape
// followed by the two lowercase hex digits of the byte's value (e.g. "@"
// becomes "_40"). Because the escape marker is always escaped when it occurs
// literally, every usernameEscape byte in the output is guaranteed to begin
// an escape sequence, never a passed-through literal. That makes the mapping
// injective: no two distinct inputs can ever sanitize to the same output.
//
// The previous implementation replaced every disallowed character with a
// fixed placeholder ('_'), which was lossy: "a.b@example.com" and
// "a_b@example.com" both sanitized to "a_b@example.com", so two distinct,
// valid usernames could be granted the same HomeDir (a real IDOR risk). This
// encoding closes that.
func SanitizeUsername(u string) string {
	var b strings.Builder
	b.Grow(len(u))
	for i := 0; i < len(u); i++ {
		c := u[i]
		if isSafeUsernameByte(c) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%c%02x", usernameEscape, c)
		}
	}
	return b.String()
}

// isSafeUsernameByte reports whether c may appear unescaped in a sanitized
// username. usernameEscape itself is intentionally excluded so it always
// signals the start of an escape sequence.
func isSafeUsernameByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-'
}

// IsLikelyEmail checks if a string broadly resembles an email address.
func IsLikelyEmail(s string) bool {
	if emailRegex == nil {
		return false
	}
	return emailRegex.MatchString(s)
}

// IsInternalUser checks if a username belongs to an internal user based on the configured suffix.
func IsInternalUser(username, suffix string) bool {
	if suffix == "" {
		return false
	}
	return strings.HasSuffix(username, suffix)
}

// ValidateFolderName checks if a folder name is valid (not empty and no illegal characters).
func ValidateFolderName(name string) error {
	if name == "" {
		return io.ErrShortBuffer // Reusing generic error for empty
	}
	// "." resolves via filepath.Join(basePath, ".") to basePath itself, so it
	// must be rejected explicitly alongside ".." and path separators.
	if name == "." || name == ".." {
		return fmt.Errorf("invalid folder name '%s': must not be '.' or '..'", name)
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid folder name '%s': contains illegal characters (.., /, or \\)", name)
	}
	return nil
}
