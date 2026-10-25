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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SearchEngagementAllocations returns the confirmed allocations of the given
// people whose [start_date, end_date] overlaps [from, to], both ends inclusive.
//
// Only CONFIRMED rows are returned: tentative, cancelled and rejected
// allocations are not work anyone is committed to. A row without a resource,
// an engagement or both dates cannot be drawn on a person's row and is left out.
func (r *scheduleRepository) SearchEngagementAllocations(ctx context.Context, req domain.SearchScheduleEngagementAllocationsRequest) ([]domain.ScheduleEngagementAllocation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ar.id::text, ar.allocation_id, ar.resource_id::text, ar.engagement_id::text,
		       ce.engagement_id, a.name, ce.engagement_type::text, ce.delivery_mode::text,
		       ar.utilization::float8, ar.start_date, ar.end_date
		FROM customer_engagement_allocation_resource ar
		JOIN customer_engagement ce ON ce.id = ar.engagement_id
		LEFT JOIN account a ON a.id = ce.account_id
		WHERE ar.resource_id = ANY($1::uuid[])
		  AND ar.state = 'CONFIRMED'
		  AND ar.start_date IS NOT NULL AND ar.end_date IS NOT NULL
		  AND ar.start_date <= $3::date AND ar.end_date >= $2::date
		ORDER BY ar.resource_id, ar.start_date, ar.end_date, ar.id`,
		req.UserIDs, req.From, req.To)
	if err != nil {
		return nil, fmt.Errorf("query schedule engagement allocations: %w", err)
	}
	defer rows.Close()

	out := []domain.ScheduleEngagementAllocation{}
	for rows.Next() {
		var al domain.ScheduleEngagementAllocation
		var start, end time.Time
		if err := rows.Scan(&al.ID, &al.AllocationID, &al.UserID, &al.EngagementID,
			&al.EngagementDisplayID, &al.AccountName, &al.EngagementType, &al.ModeOfDelivery,
			&al.Utilization, &start, &end); err != nil {
			return nil, fmt.Errorf("scan schedule engagement allocation: %w", err)
		}
		al.StartDate = start.Format("2006-01-02")
		al.EndDate = end.Format("2006-01-02")
		out = append(out, al)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schedule engagement allocations: %w", err)
	}
	return out, nil
}
