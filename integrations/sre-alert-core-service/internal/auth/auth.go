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

// Package auth manages internal service-account users (e.g. webhook-integration-user) authenticated via Authorization: Bearer <username>.<secret>, with secrets stored as PBKDF2-SHA256 hashes.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// Iterations is the PBKDF2 round count used when provisioning new users.
const Iterations = 10000

// KeyLen is the derived key length in bytes.
const KeyLen = 32

// SaltLen is the random salt length in bytes.
const SaltLen = 16

// User is one row of alertintegration.integration_users; SecretHash and Salt are base64-encoded.
type User struct {
	Username   string    `db:"username"`
	SecretHash string    `db:"secret_hash"`
	Salt       string    `db:"salt"`
	Iterations int       `db:"iterations"`
	Enabled    bool      `db:"enabled"`
	CreatedAt  time.Time `db:"created_at"`
}

// GenerateSalt returns SaltLen random bytes for a new user.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

// HashSecret derives a key from secret using PBKDF2-HMAC-SHA256.
func HashSecret(secret string, salt []byte, iterations int) []byte {
	return pbkdf2.Key([]byte(secret), salt, iterations, KeyLen, sha256.New)
}

// VerifySecret recomputes the PBKDF2 hash for secret against the stored base64 salt/hash and compares in constant time, so response timing can't leak how much of the hash matched.
func VerifySecret(secret, saltB64, hashB64 string, iterations int) bool {
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(hashB64)
	if err != nil {
		return false
	}
	got := HashSecret(secret, salt, iterations)
	return subtle.ConstantTimeCompare(got, want) == 1
}
