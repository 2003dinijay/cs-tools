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

// This is a plain (non-DB) static check, not an integration test: it needs
// no database and runs as part of every ordinary `go test ./...`, exactly
// the "compile-time-adjacent backstop" the RLS migration series' own plan
// called for -- "a lint rule banning pgxpool imports outside
// internal/db/cmd".
//
// That rule, taken completely literally, does not match this codebase:
// dozens of repository files legitimately still hold a raw *pgxpool.Pool
// (access_repo.go, catalog_repo.go, the plg_* files, and so on) because
// their tables were never in scope for RLS at all -- they carry no
// customer-project data to scope, so forcing them through Scoped would add
// runtime overhead (an identity stamp on every call) for no security
// benefit, and this test would be flagging dozens of unrelated files
// forever. rls.go/scoped.go themselves also legitimately hold the pool --
// they ARE the mechanism.
//
// What the plan's rule actually protects against is a file that touches
// one of the RLS-protected tables reaching Postgres through a raw pool
// instead of Scoped -- exactly the "someone reached the raw pool, bypassing
// the mechanism" gap FORCE ROW LEVEL SECURITY's own fail-closed default
// already backstops at the database layer, but which this test catches
// earlier, at build/test time, without needing a live database at all. So
// the rule this test actually enforces is narrower and more precise than
// the plan's literal wording: any internal/repository/*.go file (other
// than rls.go/scoped.go themselves) that imports pgxpool directly AND
// contains a string literal referencing one of rlsProtectedTables (see
// rls_schema_integration_test.go) is a violation -- it is either a
// not-yet-converted file that needs to move to Scoped, or a regression in
// an already-converted one.
package repository_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// rlsBypassLintExemptFiles are the only internal/repository/*.go files
// allowed to import pgxpool directly regardless of what their string
// literals mention -- the mechanism itself, not a table-specific
// repository. Every other file's exemption comes from simply not
// mentioning a protected table, not from being listed here.
var rlsBypassLintExemptFiles = map[string]bool{
	"rls.go":    true,
	"scoped.go": true,
}

// rlsTableNameMatchers builds one regexp per protected table name from
// rls_schema_integration_test.go's own rlsProtectedTables list, so the two
// can never quietly drift apart. Each requires an actual SQL keyword
// (FROM/JOIN/INTO/UPDATE) immediately before the table name, not a bare
// word-boundary match on the name alone: several of these names (case,
// incident, comment, problem) are also common English words, and a bare
// match flagged human-readable strings like an error message mentioning
// "an incident" or a reference-data label {"announcement", "Announcement"}
// -- neither is a real SQL reference. "case" additionally requires its
// double-quoted SQL form (`"case"`), since every real query in this
// codebase quotes it that way (case is a reserved word).
func rlsTableNameMatchers() map[string]*regexp.Regexp {
	out := make(map[string]*regexp.Regexp, len(rlsProtectedTables))
	for _, table := range rlsProtectedTables {
		name := regexp.QuoteMeta(table)
		if table == "case" {
			name = `"case"`
		}
		out[table] = regexp.MustCompile(`(?i)\b(FROM|JOIN|INTO|UPDATE)\s+` + name + `\b`)
	}
	return out
}

// TestRLSBypassLint_NoRawPoolAgainstAProtectedTable walks every
// internal/repository/*.go source file (not _test.go: test fixtures
// legitimately seed through a raw pool for tables that have no RLS-aware
// helper of their own, e.g. project/account/user in this session's own
// integration test fixtures) and fails if a non-exempt file both imports
// pgxpool directly and contains a string literal mentioning one of
// rlsProtectedTables.
func TestRLSBypassLint_NoRawPoolAgainstAProtectedTable(t *testing.T) {
	const repoDir = "."
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		t.Fatalf("read internal/repository: %v", err)
	}

	matchers := rlsTableNameMatchers()
	fset := token.NewFileSet()

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if rlsBypassLintExemptFiles[name] {
			continue
		}

		path := filepath.Join(repoDir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		if !importsPgxpool(file) {
			continue
		}

		var violations []string
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			text := stringLitValue(lit.Value)
			for table, re := range matchers {
				if re.MatchString(text) {
					violations = append(violations, table)
				}
			}
			return true
		})

		if len(violations) > 0 {
			t.Errorf(
				"%s imports pgxpool directly AND references RLS-protected table(s) %v in a string literal -- "+
					"this file must take a *repository.Scoped instead of a raw *pgxpool.Pool, the same conversion "+
					"already done for every other repository backing one of these tables",
				name, uniqueSorted(violations),
			)
		}
	}
}

func importsPgxpool(file *ast.File) bool {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if path == "github.com/jackc/pgx/v5/pgxpool" {
			return true
		}
	}
	return false
}

// stringLitValue best-effort decodes a Go string literal's source text
// (raw backtick or interpreted double-quoted) into its actual value. A raw
// string never needs escape processing; strconv.Unquote handles the
// interpreted case. Falls back to the literal source text (still good
// enough for a plain substring/word-boundary search) if neither applies.
func stringLitValue(src string) string {
	if strings.HasPrefix(src, "`") {
		return strings.Trim(src, "`")
	}
	if v, err := strconv.Unquote(src); err == nil {
		return v
	}
	return src
}

func uniqueSorted(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
