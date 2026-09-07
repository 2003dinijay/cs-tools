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

package productconsumption

import (
	"context"
	"encoding/base64"
)

// ImportDeploymentUsage calls POST /deployment-usages against the tracking
// service's own base URL, base64-encoding zipFile as the upstream service's
// contract requires.
func (c *Client) ImportDeploymentUsage(ctx context.Context, email string, zipFile []byte) (ImportUsageResponse, error) {
	req := ImportUsageRequest{
		Email: email,
		Zip:   base64.StdEncoding.EncodeToString(zipFile),
	}
	var out ImportUsageResponse
	err := c.postJSONTracking(ctx, "/deployment-usages", req, &out)
	return out, err
}
