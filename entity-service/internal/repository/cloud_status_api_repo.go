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

package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The reads behind the public cloud status dashboard.
//
// Postgres equivalents of the ServiceNow Scripted REST APIs in
// wso2-enterprise/uptime-dashboard under servicenow/scripted-rest-api/. Those
// scripts were read directly; each query below notes what it reproduces and
// where it deliberately differs.

// MonitorRow is one active cloud monitor for a cloud.
type MonitorRow struct {
	Region            string
	Group             string
	Name              string
	Description       *string
	Status            string
	ServiceOfferingID string
}

// AvailabilityRow is one uptime figure for one service offering.
type AvailabilityRow struct {
	ServiceOfferingID string
	// Duration is the dashboard's label, already translated from the enum.
	Duration string
	// Availability is carried as text because the dashboard renders it
	// verbatim; see domain.CloudStatusAvailability.
	Availability string
}

// OngoingOutageRow is an outage still in progress against one service
// offering, used to attach a message to a monitor.
type OngoingOutageRow struct {
	ServiceOfferingID string
	ShortDescription  string
	Type              string
}

// IncidentRow is one outage as the incident history shows it.
type IncidentRow struct {
	ID               string
	Begin            string
	End              string
	Type             string
	ShortDescription string
}

// CloudStatusDashboardRepository reads what the public status dashboard
// renders. Reads only: this data is published to customers, and the service
// that shows it must never be able to change it.
type CloudStatusDashboardRepository interface {
	Monitors(ctx context.Context, cloud string) ([]MonitorRow, error)
	Availabilities(ctx context.Context, offeringIDs []string) ([]AvailabilityRow, error)
	OngoingOutages(ctx context.Context, offeringIDs []string) ([]OngoingOutageRow, error)
	Incidents(ctx context.Context, cloud string, since time.Time) ([]IncidentRow, error)
}

type cloudStatusDashboardRepository struct {
	db *pgxpool.Pool
}

// NewCloudStatusDashboardRepository constructs the dashboard reader.
func NewCloudStatusDashboardRepository(db *pgxpool.Pool) CloudStatusDashboardRepository {
	return &cloudStatusDashboardRepository{db: db}
}

// monitorsSQL reproduces monitors.js's own query.
//
//	gr.addEncodedQuery('u_cloud_offering=' + cloud + '^u_active=true');
//	gr.orderByDesc('u_group_priority');
//	gr.orderBy('u_group');
//	gr.orderBy('u_name');
//
// THE ORDER IS LOAD-BEARING, not cosmetic. The script builds its response by
// appending to whichever group it last saw, so the row order decides both the
// order of groups on the page and which monitors land together. Sorting
// differently here would silently reshuffle a customer-facing page.
//
// NULLS LAST on group_priority because Postgres sorts NULLs first on DESC and
// ServiceNow treats an empty priority as the lowest.
const monitorsSQL = `
    SELECT LOWER(COALESCE(cm.region, '')),
           COALESCE(cm."group", ''),
           COALESCE(cm.name, ''),
           cm.description,
           COALESCE(cm.status::text, ''),
           COALESCE(cm.service_offering_id::text, '')
      FROM cloud_monitor cm
     WHERE cm.cloud_offering = $1::cloud_monitor_cloud_offering_enum
       AND cm.is_active IS TRUE
     ORDER BY cm.group_priority DESC NULLS LAST, cm."group", cm.name
`

// Monitors returns every active monitor for one cloud, in the dashboard's order.
func (r *cloudStatusDashboardRepository) Monitors(ctx context.Context, cloud string) ([]MonitorRow, error) {
	rows, err := r.db.Query(ctx, monitorsSQL, cloud)
	if err != nil {
		return nil, fmt.Errorf("query cloud status monitors: %w", err)
	}
	defer rows.Close()

	out := make([]MonitorRow, 0)
	for rows.Next() {
		var m MonitorRow
		if err := rows.Scan(&m.Region, &m.Group, &m.Name, &m.Description, &m.Status, &m.ServiceOfferingID); err != nil {
			return nil, fmt.Errorf("scan cloud status monitor: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// availabilitiesSQL replaces the per-monitor lookup the script does inside its
// loop:
//
//	'service_offering.sys_id=' + serviceOffering + '^typeINlast30days,last90days'
//
// Batched over every offering in one round trip instead of one query per
// monitor. With 36 monitors on the busiest cloud that is 36 queries collapsed
// into one, and the result is identical because the script's inner query has
// no ordering or limit to preserve.
//
// DISTINCT ON keeps the most recent row per (offering, window). The table
// holds 213k rows across many windows and re-computations; the script relied
// on there being exactly one match per type, which is not guaranteed.
const availabilitiesSQL = `
    SELECT DISTINCT ON (sa.service_offering_id, sa.type)
           sa.service_offering_id::text,
           sa.type::text,
           COALESCE(sa.absolute_availability::text, '')
      FROM service_availability sa
     WHERE sa.service_offering_id = ANY($1::uuid[])
       AND sa.type IN ('LAST_30_DAYS', 'LAST_90_DAYS')
     ORDER BY sa.service_offering_id, sa.type, sa.end_on DESC NULLS LAST
`

// availabilityDuration maps the stored enum to the dashboard's own label.
// The strings are the script's, and the frontend renders them directly.
var availabilityDuration = map[string]string{
	"LAST_30_DAYS": "Last 30 days",
	"LAST_90_DAYS": "Last 90 days",
}

// Availabilities returns the 30- and 90-day uptime for each offering.
func (r *cloudStatusDashboardRepository) Availabilities(ctx context.Context, offeringIDs []string) ([]AvailabilityRow, error) {
	if len(offeringIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, availabilitiesSQL, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("query cloud status availabilities: %w", err)
	}
	defer rows.Close()

	out := make([]AvailabilityRow, 0)
	for rows.Next() {
		var a AvailabilityRow
		var enumType string
		if err := rows.Scan(&a.ServiceOfferingID, &enumType, &a.Availability); err != nil {
			return nil, fmt.Errorf("scan cloud status availability: %w", err)
		}
		a.Duration = availabilityDuration[enumType]
		out = append(out, a)
	}
	return out, rows.Err()
}

// ongoingOutagesSQL replaces the script's nested lookup for a monitor's
// message:
//
//	'ci_item.sys_id=' + serviceOffering + '^outage.endISEMPTY'
//
// then reading short_description and type off each outage. Batched the same
// way, and for the same reason.
//
// Note this joins outage_affected_ci, which digiops-cs #3187 provides. Until
// that lands the query returns nothing and monitors simply carry no message --
// degrading to the dashboard's own empty-message case rather than failing.
const ongoingOutagesSQL = `
    SELECT ac.ci_id::text,
           COALESCE(o.name, ''),
           COALESCE(o.type::text, '')
      FROM outage_affected_ci ac
      JOIN outage o ON o.id = ac.outage_id
     WHERE ac.ci_id = ANY($1::uuid[])
       AND o.end_on IS NULL
`

// OngoingOutages returns outages still in progress against the given offerings.
func (r *cloudStatusDashboardRepository) OngoingOutages(ctx context.Context, offeringIDs []string) ([]OngoingOutageRow, error) {
	if len(offeringIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, ongoingOutagesSQL, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("query ongoing outages: %w", err)
	}
	defer rows.Close()

	out := make([]OngoingOutageRow, 0)
	for rows.Next() {
		var o OngoingOutageRow
		if err := rows.Scan(&o.ServiceOfferingID, &o.ShortDescription, &o.Type); err != nil {
			return nil, fmt.Errorf("scan ongoing outage: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// incidentsSQL reproduces incidents.js's query:
//
//	'cmdb_ci.ref_service_offering.parent.nameSTARTSWITH' + cloud +
//	'^task_numberISNOTEMPTY' +
//	'^beginBETWEENgs.beginningOfLast2Quarters()@gs.endOfToday()'
//	orderByDesc('sys_created_on')
//
// *** THE CLOUD FILTER HERE IS BY NAME, NOT BY THE FOURTEEN SYSIDS. *** The
// notification flow filters on a hardcoded list of service sys_ids; this
// script matches on the parent service's NAME starting with the cloud. They
// are different populations and both are reproduced faithfully, each in its
// own place. Substituting one for the other would change which incidents the
// public page lists.
//
// task_number IS NOT EMPTY becomes work_item_id IS NOT NULL: the sync maps
// that reference across. It matters -- only 188 of 636 outages carry one, so
// the filter removes two thirds of the table.
//
// The date window is passed in rather than computed here, so the caller owns
// "last two quarters" and it can be tested without freezing a clock.
const incidentsSQL = `
    SELECT o.id::text,
           to_char(o.start_on AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS'),
           COALESCE(to_char(o.end_on AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS'), ''),
           COALESCE(o.type::text, ''),
           COALESCE(o.name, '')
      FROM outage o
      JOIN service_offering so ON so.id = o.service_offering_id
      JOIN service s ON s.id = so.parent_id
     WHERE s.name ILIKE $1 || '%'
       AND o.work_item_id IS NOT NULL
       AND o.start_on >= $2
       AND o.start_on IS NOT NULL
     ORDER BY o.created_on DESC
`

// Incidents returns the outages the dashboard lists for one cloud since the
// given instant.
func (r *cloudStatusDashboardRepository) Incidents(ctx context.Context, cloud string, since time.Time) ([]IncidentRow, error) {
	rows, err := r.db.Query(ctx, incidentsSQL, cloud, since)
	if err != nil {
		return nil, fmt.Errorf("query cloud status incidents: %w", err)
	}
	defer rows.Close()

	out := make([]IncidentRow, 0)
	for rows.Next() {
		var i IncidentRow
		if err := rows.Scan(&i.ID, &i.Begin, &i.End, &i.Type, &i.ShortDescription); err != nil {
			return nil, fmt.Errorf("scan cloud status incident: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
