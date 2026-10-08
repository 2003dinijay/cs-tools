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

package paging

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeLookup struct {
	mu      sync.Mutex
	numbers map[string]string
	err     error
	delay   time.Duration
	calls   map[string]int
}

func (f *fakeLookup) MobileNumber(ctx context.Context, email string) (string, error) {
	f.mu.Lock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[email]++
	f.mu.Unlock()
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return f.numbers[email], f.err
}

func phones(rs []Recipient) map[string]string {
	out := map[string]string{}
	for _, r := range rs {
		out[r.Email] = r.Phone
	}
	return out
}

func TestProfilePhoneResolver(t *testing.T) {
	ctx := context.Background()
	rc := RoutingContext{Shift: ShiftLK}

	t.Run("fills a missing number and keeps one already named", func(t *testing.T) {
		inner := StaticResolver{ByLevel: map[Level][]Recipient{Level0: {
			{Name: "A", Email: "a@example.com"},
			{Name: "Head", Email: "head@example.com", Phone: "+94770000099"},
		}}}
		lookup := &fakeLookup{numbers: map[string]string{
			"a@example.com": "+94770000001", "head@example.com": "+94770000002"}}
		got, err := NewProfilePhoneResolver(inner, lookup).Resolve(ctx, Level0, rc)
		if err != nil {
			t.Fatal(err)
		}
		p := phones(got)
		if p["a@example.com"] != "+94770000001" {
			t.Errorf("profile number not filled in: %q", p["a@example.com"])
		}
		if p["head@example.com"] != "+94770000099" {
			t.Errorf("a number named in configuration was replaced: %q", p["head@example.com"])
		}
		if lookup.calls["head@example.com"] != 0 {
			t.Error("looked up someone who already had a number")
		}
	})

	t.Run("no number, a bad number and a failed lookup each leave just that person without one", func(t *testing.T) {
		inner := StaticResolver{ByLevel: map[Level][]Recipient{Level0: {
			{Name: "Unset", Email: "unset@example.com"},
			{Name: "Local", Email: "local@example.com"},
			{Name: "Ok", Email: "ok@example.com"},
		}}}
		lookup := &fakeLookup{numbers: map[string]string{
			"local@example.com": "0770000001", "ok@example.com": "+94770000003"}}
		got, err := NewProfilePhoneResolver(inner, lookup).Resolve(ctx, Level0, rc)
		if err != nil {
			t.Fatal(err)
		}
		p := phones(got)
		if p["unset@example.com"] != "" || p["local@example.com"] != "" {
			t.Errorf("unset/non-E.164 numbers must stay empty: %v", p)
		}
		if p["ok@example.com"] != "+94770000003" {
			t.Errorf("one bad number must not stop the others: %v", p)
		}

		failing := &fakeLookup{err: errors.New("directory down")}
		got, err = NewProfilePhoneResolver(inner, failing).Resolve(ctx, Level0, rc)
		if err != nil {
			t.Fatalf("a failing directory must not fail the rung: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("recipients = %d, want all three still returned", len(got))
		}
	})

	t.Run("one lookup per person across tiers, refreshed after the TTL", func(t *testing.T) {
		lead := Recipient{Name: "Lead", Email: "Lead@Example.com"}
		inner := StaticResolver{ByLevel: map[Level][]Recipient{Level1: {lead}, Level2: {lead}}}
		lookup := &fakeLookup{numbers: map[string]string{"lead@example.com": "+94770000004"}}
		r := NewProfilePhoneResolver(inner, lookup)
		now := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
		r.now = func() time.Time { return now }
		for _, lvl := range []Level{Level1, Level2} {
			got, _ := r.Resolve(ctx, lvl, rc)
			if got[0].Phone != "+94770000004" {
				t.Fatalf("%s phone = %q", lvl, got[0].Phone)
			}
		}
		if n := lookup.calls["lead@example.com"]; n != 1 {
			t.Errorf("lookups = %d, want 1 for one person on two tiers", n)
		}
		now = now.Add(profilePhoneTTL + time.Second)
		_, _ = r.Resolve(ctx, Level1, rc)
		if n := lookup.calls["lead@example.com"]; n != 2 {
			t.Errorf("lookups after TTL = %d, want a fresh one", n)
		}
	})

	t.Run("a slow directory is cut off rather than holding the plan", func(t *testing.T) {
		inner := StaticResolver{ByLevel: map[Level][]Recipient{Level0: {{Name: "Slow", Email: "slow@example.com"}}}}
		lookup := &fakeLookup{delay: profilePhoneTimeout + 2*time.Second,
			numbers: map[string]string{"slow@example.com": "+94770000005"}}
		start := time.Now()
		got, err := NewProfilePhoneResolver(inner, lookup).Resolve(ctx, Level0, rc)
		if err != nil {
			t.Fatal(err)
		}
		if el := time.Since(start); el > profilePhoneTimeout+time.Second {
			t.Errorf("took %s; the lookup timeout did not apply", el)
		}
		if got[0].Phone != "" {
			t.Errorf("a timed-out lookup must leave no number, got %q", got[0].Phone)
		}
	})
}

// fakePagingContacts stands in for entity-service's paging contacts.
type fakePagingContacts struct {
	numbers map[string]string
	err     error
	calls   int
	asked   []string
}

func (f *fakePagingContacts) PagingPhones(_ context.Context, emails []string) (map[string]string, error) {
	f.calls++
	f.asked = append(f.asked, emails...)
	return f.numbers, f.err
}

// The paging number is the fallback for a profile with none: profile first,
// one batched lookup, and a failed lookup changes nothing.
func TestProfilePhoneResolver_PagingNumberFallback(t *testing.T) {
	ctx := context.Background()
	inner := StaticResolver{ByLevel: map[Level][]Recipient{Level0: {
		{Name: "Profiled", Email: "p@example.com"},
		{Name: "Paging", Email: "Pg@example.com"},
		{Name: "Bad", Email: "bad@example.com"},
		{Name: "Nothing", Email: "n@example.com"},
		{Name: "Head", Email: "head@example.com", Phone: "+94770000099"},
	}}}
	lookup := &fakeLookup{numbers: map[string]string{"p@example.com": "+94770000001"}}
	contacts := &fakePagingContacts{numbers: map[string]string{
		"p@example.com": "+94770000011", "pg@example.com": "+94770000012",
		"bad@example.com": "0770000013", "head@example.com": "+94770000014",
	}}

	got, err := NewProfilePhoneResolver(inner, lookup).WithPagingContacts(contacts).Resolve(ctx, Level0, RoutingContext{})
	if err != nil {
		t.Fatal(err)
	}
	p := phones(got)
	for email, want := range map[string]string{
		"p@example.com":    "+94770000001", // the profile wins
		"Pg@example.com":   "+94770000012", // no profile number: the paging one
		"bad@example.com":  "",             // not E.164: not dialled
		"n@example.com":    "",             // neither: NO_NUMBER, as before
		"head@example.com": "+94770000099", // named in paging-alert.yaml: wins over both
	} {
		if p[email] != want {
			t.Errorf("%s = %q, want %q", email, p[email], want)
		}
	}
	if contacts.calls != 1 {
		t.Errorf("%d paging lookups; want one batch", contacts.calls)
	}
	if strings.Join(contacts.asked, ",") != "pg@example.com,bad@example.com,n@example.com" {
		t.Errorf("asked for %v; want only those without a profile number", contacts.asked)
	}

	t.Run("a failed lookup leaves today's behaviour", func(t *testing.T) {
		failing := &fakePagingContacts{err: errors.New("entity-service down")}
		got, err := NewProfilePhoneResolver(inner, lookup).WithPagingContacts(failing).Resolve(ctx, Level0, RoutingContext{})
		if err != nil {
			t.Fatalf("a paging lookup failure failed the tier: %v", err)
		}
		p := phones(got)
		if p["p@example.com"] != "+94770000001" || p["Pg@example.com"] != "" {
			t.Errorf("numbers = %v; want the profile one only", p)
		}
	})

	t.Run("without a profile directory the paging number still applies", func(t *testing.T) {
		got, _ := NewProfilePhoneResolver(inner, nil).WithPagingContacts(contacts).Resolve(ctx, Level0, RoutingContext{})
		if p := phones(got); p["p@example.com"] != "+94770000011" {
			t.Errorf("p@example.com = %q; want its paging number", p["p@example.com"])
		}
	})
}
