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

package domain

// InstanceMetaRequest carries metadata to register or update a deployment node.
type InstanceMetaRequest struct {
	NodeID           string `json:"nodeId"`
	ProjectKey       string `json:"projectKey"`
	DeploymentNumber string `json:"deploymentNumber"`
	Product          string `json:"product"`
	Version          string `json:"version"`
}

// InstanceMetaResponse carries the UUID of the registered deployment node.
type InstanceMetaResponse struct {
	ID string `json:"id"`
}

// InstanceInfoRequest carries JDK and host environment details for a deployment node.
type InstanceInfoRequest struct {
	NodeID         string `json:"nodeId"`
	JDKVersion     string `json:"jdkVersion"`
	CoreCount      int    `json:"coreCount"`
	DeploymentInfo any    `json:"deploymentInfo"`
	Hash           string `json:"hash"`
	CreatedTime    string `json:"createdTime"`
}

// InstanceInfoResponse carries the UUID of the inserted deployment_information record.
type InstanceInfoResponse struct {
	ID string `json:"id"`
}

// InstanceUsageIngestRequest carries live hourly and daily telemetry for a deployment node.
type InstanceUsageIngestRequest struct {
	NodeID      string `json:"nodeId"`
	CountType   string `json:"countType"`
	Count       int    `json:"count"`
	CreatedTime string `json:"createdTime"`
}

// InstanceUsageIngestResponse carries status of usage stats insertion.
type InstanceUsageIngestResponse struct {
	Status string `json:"status"`
}

// DailyUsageBatchItem represents a single daily metric value in an offline import.
type DailyUsageBatchItem struct {
	Date  string `json:"date"`
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// DailyUsageBatchRequest carries parsed daily usage entries from an offline ZIP file.
type DailyUsageBatchRequest struct {
	NodeID           string                `json:"nodeId"`
	ProjectKey       string                `json:"projectKey"`
	DeploymentNumber string                `json:"deploymentNumber"`
	Product          string                `json:"product"`
	Version          string                `json:"version"`
	UserEmail        string                `json:"userEmail"`
	Usages           []DailyUsageBatchItem `json:"usages"`
}

// DailyUsageBatchResponse carries the number of imported daily usage entries.
type DailyUsageBatchResponse struct {
	ImportedCount int    `json:"importedCount"`
	Status        string `json:"status"`
}

// ProjectSecretKeyResponse carries the HMAC primary secret key for a project.
type ProjectSecretKeyResponse struct {
	SecretKey string `json:"secretKey"`
}
