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

package teamschedule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

// ReminderWindow is a rotation's hours on its own clock: "08:00" to "18:00"
// in America/Los_Angeles.
type ReminderWindow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	TimeZone string `json:"timeZone"`
}

// ReminderSettings is what the rota admins set for a family's reminders on
// the Team Schedule's Rota Access tab (entity-service's
// team_schedule_reminder_setting).
type ReminderSettings struct {
	WeekdayEnabled  bool                      `json:"weekdayEnabled"`
	WeekendEnabled  bool                      `json:"weekendEnabled"`
	InvitesEnabled  bool                      `json:"invitesEnabled"`
	WeekendRequired map[string]int            `json:"weekendRequired"`
	Windows         map[string]ReminderWindow `json:"windows"`
	CopyTo          []string                  `json:"copyTo"`
}

// ReminderSettingsFor is the family's saved reminder settings, or nil when
// nothing has been saved (the task then keeps its own configuration).
func (c *Client) ReminderSettingsFor(ctx context.Context, family string) (*ReminderSettings, error) {
	body, err := c.do(ctx, http.MethodGet, "/team-schedule/reminder-settings/"+url.PathEscape(family), nil)
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s ReminderSettings
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("teamschedule: decode reminder settings: %w", err)
	}
	return &s, nil
}
