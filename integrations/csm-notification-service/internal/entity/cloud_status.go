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

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// RecordCloudStatusDelivery reports the outcome of one status-page webhook
// (outage.status_page_due) -- the call csm-scheduled-tasks makes after its own
// posts. It counts the attempt and ends the row's delivery lease, so a failure
// becomes the scheduled task's to retry. errMsg is required when delivered is
// false.
func (c *CustomerEntityClient) RecordCloudStatusDelivery(ctx context.Context, webhookID string, delivered bool, errMsg string) error {
	if webhookID == "" {
		return fmt.Errorf("entity: webhookId is required to record a cloud status delivery")
	}
	body, err := json.Marshal(struct {
		Delivered bool   `json:"delivered"`
		Error     string `json:"error,omitempty"`
	}{delivered, errMsg})
	if err != nil {
		return fmt.Errorf("entity: encode cloud status delivery: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, "/internal/cloud-status/"+url.PathEscape(webhookID)+"/delivery", body)
	return err
}
