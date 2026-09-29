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

// Command user manages integration_users rows (e.g. webhook-integration-user): create/rotate, list, enable, disable. Uses the same CASSANDRA_* env vars as the server.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gocql/gocql"

	"alert-core-service/internal/auth"
	"alert-core-service/internal/cassandra"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	session, err := connect()
	if err != nil {
		log.Fatalf("user: %v", err)
	}
	defer session.Close()
	repo := auth.NewUserRepo(session)

	switch os.Args[1] {
	case "create":
		runCreate(repo, os.Args[2:])
	case "list":
		runList(repo, os.Args[2:])
	case "enable":
		runSetEnabled(repo, os.Args[2:], true)
	case "disable":
		runSetEnabled(repo, os.Args[2:], false)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: user <create|list|enable|disable> [flags]")
	fmt.Fprintln(os.Stderr, "  create  -username <name> [-secret <value>]   create or rotate a user")
	fmt.Fprintln(os.Stderr, "  list                                         list all users")
	fmt.Fprintln(os.Stderr, "  enable  -username <name>                     re-enable a user")
	fmt.Fprintln(os.Stderr, "  disable -username <name>                     disable a user")
}

// connect reads CASSANDRA_* env vars (same ones the server uses) and opens a session.
func connect() (*gocql.Session, error) {
	cfg, err := cassandra.ConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("read cassandra config: %w", err)
	}
	session, err := cassandra.Connect(cfg, 10*time.Second, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to cassandra: %w", err)
	}
	return session, nil
}

func runCreate(repo *auth.UserRepo, args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	username := fs.String("username", "", "internal user to create, e.g. webhook-integration-user (required)")
	secret := fs.String("secret", "", "secret to set; if omitted, a random secret is generated and printed once")
	fs.Parse(args)

	if *username == "" {
		log.Fatal("user create: -username is required")
	}

	plainSecret := *secret
	generated := false
	if plainSecret == "" {
		s, err := generateSecret()
		if err != nil {
			log.Fatalf("user create: generate secret: %v", err)
		}
		plainSecret = s
		generated = true
	}

	salt, err := auth.GenerateSalt()
	if err != nil {
		log.Fatalf("user create: generate salt: %v", err)
	}
	hash := auth.HashSecret(plainSecret, salt, auth.Iterations)

	u := auth.User{
		Username:   *username,
		SecretHash: base64.StdEncoding.EncodeToString(hash),
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Iterations: auth.Iterations,
		Enabled:    true,
		CreatedAt:  time.Now().UTC(),
	}
	if err := repo.Upsert(context.Background(), u); err != nil {
		log.Fatalf("user create: upsert user: %v", err)
	}

	fmt.Printf("created internal user %q\n", *username)
	if generated {
		fmt.Printf("secret (shown once, store securely): %s\n", plainSecret)
	}
}

func runList(repo *auth.UserRepo, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Parse(args)

	users, err := repo.List(context.Background())
	if err != nil {
		log.Fatalf("user list: %v", err)
	}
	if len(users) == 0 {
		fmt.Println("no internal users found")
		return
	}
	fmt.Printf("%-30s %-8s %-11s %s\n", "USERNAME", "ENABLED", "ITERATIONS", "CREATED_AT")
	for _, u := range users {
		fmt.Printf("%-30s %-8t %-11d %s\n", u.Username, u.Enabled, u.Iterations, u.CreatedAt.Format(time.RFC3339))
	}
}

func runSetEnabled(repo *auth.UserRepo, args []string, enabled bool) {
	name := "enable"
	if !enabled {
		name = "disable"
	}
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	username := fs.String("username", "", "internal user to "+name+" (required)")
	fs.Parse(args)

	if *username == "" {
		log.Fatalf("user %s: -username is required", name)
	}

	if _, err := repo.Get(context.Background(), *username); err != nil {
		log.Fatalf("user %s: %v", name, err)
	}
	if err := repo.SetEnabled(context.Background(), *username, enabled); err != nil {
		log.Fatalf("user %s: %v", name, err)
	}
	fmt.Printf("%sd internal user %q\n", name, *username)
}

// generateSecret returns a random, URL-safe base64-encoded 32-byte secret.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
