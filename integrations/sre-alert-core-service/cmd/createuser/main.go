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

// Command createuser provisions an integration_users row (e.g. webhook-integration-user) with a
// PBKDF2-hashed secret, using the same CASSANDRA_* env vars as the server.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"time"

	"alert-core-service/internal/auth"
	"alert-core-service/internal/cassandra"
)

func main() {
	username := flag.String("username", "", "internal user to create, e.g. webhook-integration-user (required)")
	secret := flag.String("secret", "", "secret to set; if omitted, a random secret is generated and printed once")
	flag.Parse()

	if *username == "" {
		log.Fatal("createuser: -username is required")
	}

	plainSecret := *secret
	generated := false
	if plainSecret == "" {
		s, err := generateSecret()
		if err != nil {
			log.Fatalf("createuser: generate secret: %v", err)
		}
		plainSecret = s
		generated = true
	}

	cfg, err := cassandra.ConfigFromEnv()
	if err != nil {
		log.Fatalf("createuser: read cassandra config: %v", err)
	}
	session, err := cassandra.Connect(cfg, 10*time.Second, 10*time.Second)
	if err != nil {
		log.Fatalf("createuser: connect to cassandra: %v", err)
	}
	defer session.Close()

	salt, err := auth.GenerateSalt()
	if err != nil {
		log.Fatalf("createuser: generate salt: %v", err)
	}
	hash := auth.HashSecret(plainSecret, salt, auth.Iterations)

	repo := auth.NewUserRepo(session)
	u := auth.User{
		Username:   *username,
		SecretHash: base64.StdEncoding.EncodeToString(hash),
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Iterations: auth.Iterations,
		Enabled:    true,
		CreatedAt:  time.Now().UTC(),
	}
	if err := repo.Upsert(context.Background(), u); err != nil {
		log.Fatalf("createuser: upsert user: %v", err)
	}

	fmt.Printf("created internal user %q\n", *username)
	if generated {
		fmt.Printf("secret (shown once, store securely): %s\n", plainSecret)
	}
}

// generateSecret returns a random, URL-safe base64-encoded 32-byte secret.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
