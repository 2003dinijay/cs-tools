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

package main

import "testing"

// TestExitCode verifies the process reports failure to its caller (a
// scheduled Choreo task) whenever any project failed during the sweep, and
// success only when none did.
func TestExitCode(t *testing.T) {
	tests := []struct {
		name         string
		failureCount int
		wantExitCode int
	}{
		{name: "no failures", failureCount: 0, wantExitCode: 0},
		{name: "one failure", failureCount: 1, wantExitCode: 1},
		{name: "many failures", failureCount: 7, wantExitCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exitCode(tt.failureCount)
			if got != tt.wantExitCode {
				t.Errorf("exitCode(%d) = %d, want %d", tt.failureCount, got, tt.wantExitCode)
			}
		})
	}
}
