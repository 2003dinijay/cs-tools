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

package service

import (
	"context"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// cloudStatusClouds are the clouds the dashboard serves, and the only values
// the endpoints accept.
//
// The ServiceNow monitors script validates against five and rejects the rest
// with a 400. This accepts SEVEN: the common backend already serves
// choreo-eu (17 monitors) and agent-manager (14), and has never been able to
// receive anything for them because the flow's URL script has no branch for
// either. Refusing them here would reproduce a limitation that exists nowhere
// except in that one script.
var cloudStatusClouds = map[string]string{
	"asgardeo":      "ASGARDEO",
	"choreo":        "CHOREO",
	"bijira":        "BIJIRA",
	"devant":        "DEVANT",
	"moesif":        "MOESIF",
	"choreo-eu":     "CHOREO_EU",
	"agent-manager": "AGENT_MANAGER",
}

type cloudStatusDashboardService struct {
	repo repository.CloudStatusDashboardRepository
	// now is injectable so the incident window can be tested without
	// freezing a clock.
	now func() time.Time
}

// NewCloudStatusDashboardService constructs the dashboard read service.
func NewCloudStatusDashboardService(repo repository.CloudStatusDashboardRepository) CloudStatusDashboardService {
	return &cloudStatusDashboardService{repo: repo, now: time.Now}
}

// validateCloud maps a dashboard cloud slug to its stored enum value.
func validateCloud(cloud string) (string, error) {
	enum, ok := cloudStatusClouds[strings.ToLower(strings.TrimSpace(cloud))]
	if !ok {
		return "", &apierror.ValidationError{Msg: "cloud must be one of asgardeo, choreo, bijira, devant, moesif, choreo-eu, agent-manager"}
	}
	return enum, nil
}

// Monitors assembles the dashboard's monitor view for one cloud.
//
// Three reads instead of the script's one-plus-two-per-monitor: the monitors,
// then their availability and any ongoing outages batched by offering. On the
// busiest cloud that is 3 queries where ServiceNow issued 73.
func (s *cloudStatusDashboardService) Monitors(ctx context.Context, cloud string) (domain.CloudStatusMonitorsResponse, error) {
	enum, err := validateCloud(cloud)
	if err != nil {
		return nil, err
	}

	monitors, err := s.repo.Monitors(ctx, enum)
	if err != nil {
		return nil, err
	}

	offerings := make([]string, 0, len(monitors))
	seen := map[string]bool{}
	for _, m := range monitors {
		if m.ServiceOfferingID != "" && !seen[m.ServiceOfferingID] {
			seen[m.ServiceOfferingID] = true
			offerings = append(offerings, m.ServiceOfferingID)
		}
	}

	avail, err := s.repo.Availabilities(ctx, offerings)
	if err != nil {
		return nil, err
	}
	byOffering := map[string][]domain.CloudStatusAvailability{}
	for _, a := range avail {
		if a.Duration == "" {
			continue
		}
		byOffering[a.ServiceOfferingID] = append(byOffering[a.ServiceOfferingID],
			domain.CloudStatusAvailability{Availability: a.Availability, Duration: a.Duration})
	}

	outages, err := s.repo.OngoingOutages(ctx, offerings)
	if err != nil {
		return nil, err
	}

	// The response is built by APPENDING in row order, which is why the
	// repository's ORDER BY matters. A map would lose that, so groups are
	// tracked by index within each region.
	resp := domain.CloudStatusMonitorsResponse{}
	groupIndex := map[string]map[string]int{}

	for _, m := range monitors {
		status := domain.DashboardStatus(domain.CloudMonitorStatus(m.Status))

		// The message only attaches when an ongoing outage's type agrees with
		// the status shown -- see domain.MessageAgreesWithStatus for why that
		// is not redundant.
		message := ""
		for _, o := range outages {
			if o.ServiceOfferingID == m.ServiceOfferingID &&
				domain.MessageAgreesWithStatus(status, o.Type) {
				message = o.ShortDescription
				break
			}
		}

		sub := domain.CloudStatusMonitorSubgroup{
			Description:  m.Description,
			DisplayName:  m.Name,
			Status:       status,
			Availability: byOffering[m.ServiceOfferingID],
			Message:      message,
		}
		if sub.Availability == nil {
			// The dashboard iterates this; a null would render as nothing at
			// best and throw at worst. The script always emits an array.
			sub.Availability = []domain.CloudStatusAvailability{}
		}

		if _, ok := groupIndex[m.Region]; !ok {
			groupIndex[m.Region] = map[string]int{}
		}
		if idx, ok := groupIndex[m.Region][m.Group]; ok {
			resp[m.Region][idx].Subgroups = append(resp[m.Region][idx].Subgroups, sub)
			continue
		}
		resp[m.Region] = append(resp[m.Region], domain.CloudStatusMonitorGroup{
			DisplayName: m.Group,
			Subgroups:   []domain.CloudStatusMonitorSubgroup{sub},
		})
		groupIndex[m.Region][m.Group] = len(resp[m.Region]) - 1
	}
	return resp, nil
}

// incidentMonths is how many months of history the dashboard shows.
const incidentMonths = 6

// Incidents assembles the dashboard's incident history for one cloud.
//
// The response always carries SIX month keys, populated or not, because the
// script pre-seeds them and the frontend renders a row per key. Returning only
// the months that have incidents would silently shorten the page.
func (s *cloudStatusDashboardService) Incidents(ctx context.Context, cloud string) (domain.CloudStatusIncidentsResponse, error) {
	if _, err := validateCloud(cloud); err != nil {
		return nil, err
	}

	now := s.now().UTC()
	resp := domain.CloudStatusIncidentsResponse{}
	for i := 0; i < incidentMonths; i++ {
		m := now.AddDate(0, -i, 0)
		resp[incidentMonthKey(m)] = domain.CloudStatusIncidentMonth{
			Incidents: []domain.CloudStatusIncident{},
		}
	}

	// The script's window is gs.beginningOfLast2Quarters(). Six months back
	// from the first of the current month covers the same span and matches
	// the six keys above, which the quarter boundary does not always do.
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).
		AddDate(0, -(incidentMonths - 1), 0)

	rows, err := s.repo.Incidents(ctx, strings.ToLower(strings.TrimSpace(cloud)), since)
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		begin, err := time.Parse("2006-01-02 15:04:05", r.Begin)
		if err != nil {
			continue
		}
		key := incidentMonthKey(begin)
		month, ok := resp[key]
		if !ok {
			// Outside the six seeded months. The script drops these too --
			// its query window and its key seeding can disagree at a quarter
			// boundary, and an unkeyed incident simply vanishes.
			continue
		}
		month.Incidents = append(month.Incidents, domain.CloudStatusIncident{
			ID:               r.ID,
			Begin:            r.Begin,
			End:              r.End,
			Type:             incidentTypeLabel(r.Type),
			Status:           incidentStatus(r.End),
			ShortDescription: r.ShortDescription,
			Expanded:         false,
		})
		resp[key] = month
	}
	return resp, nil
}

// incidentMonthKey renders the "YYYY-M" key. Deliberately not zero-padded:
// the script builds it by concatenation and the frontend matches on it.
func incidentMonthKey(t time.Time) string {
	return t.Format("2006") + "-" + strings.TrimPrefix(t.Format("01"), "0")
}

// incidentTypeLabel title-cases the stored enum for display: the live API
// returns "Outage", not "OUTAGE".
func incidentTypeLabel(t string) string {
	switch strings.ToUpper(t) {
	case "OUTAGE":
		return "Outage"
	case "DEGRADATION":
		return "Degradation"
	case "PLANNED":
		return "Planned"
	default:
		return ""
	}
}

// incidentStatus is derived, not stored: an outage with an end is Resolved.
func incidentStatus(end string) string {
	if end == "" {
		return "Ongoing"
	}
	return "Resolved"
}
