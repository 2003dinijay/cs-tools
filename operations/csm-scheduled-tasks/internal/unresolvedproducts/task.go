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
package unresolvedproducts

import (
	"context"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

// Lister is the subset of *Client this package depends on.
type Lister interface {
	ListSince(ctx context.Context, since time.Time) (Listing, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// SendDigest returns a registry.Task.Handler that emails to/cc the nodes seen in the last
// window whose reported product or version is not in the product catalogue.
//
// Like the other report sub-crons: with emailsEnabled false (ALERTS_ENABLED) or no
// recipients configured it succeeds without contacting entity-service. Unlike them, a run
// that finds nothing sends nothing, since the point is to prompt someone to add a
// catalogue entry, and an empty digest every day would train its recipients to ignore it.
//
// A node stays in the digest every day for as long as it keeps reporting a product the
// catalogue lacks: the digest is the list of entries still to add, not a one-off alert.
// Each row shows how long it has been unresolved, and a list entity-service cut off at
// its limit says so.
func SendDigest(list Lister, email EmailSender, window time.Duration, to, cc []string, emailsEnabled bool, now func() time.Time) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}
		listing, err := list.ListSince(ctx, now().Add(-window))
		if err != nil {
			return fmt.Errorf("unresolvedproducts: list unresolved products: %w", err)
		}
		nodes := listing.Nodes
		if len(nodes) == 0 {
			return nil
		}

		rows := make([]notify.UnresolvedProductNode, 0, len(nodes))
		products := map[string]struct{}{}
		for _, n := range nodes {
			rows = append(rows, notify.UnresolvedProductNode{
				NodeID:           n.NodeID,
				ProjectKey:       deref(n.ProjectKey),
				DeploymentNumber: deref(n.DeploymentNumber),
				ReportedProduct:  n.ReportedProduct,
				ReportedVersion:  deref(n.ReportedVersion),
				UnresolvedSince:  n.UnresolvedSince,
				LastSeenOn:       n.LastSeenOn,
			})
			products[n.ReportedProduct+" "+deref(n.ReportedVersion)] = struct{}{}
		}
		subject := fmt.Sprintf("[Action-Required][Report] %d product version(s) missing from the product catalogue", len(products))
		body := notify.RenderUnresolvedProductsReport(notify.UnresolvedProductsReportData{
			WindowHours: int(window.Hours()),
			Nodes:       rows,
			Truncated:   listing.Truncated,
		})
		if err := email.SendEmail(ctx, to, cc, subject, body); err != nil {
			return fmt.Errorf("unresolvedproducts: send digest email: %w", err)
		}
		return nil
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
