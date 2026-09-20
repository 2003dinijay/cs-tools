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

import (
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
)

func TestLoadAccessConfig(t *testing.T) {
	envs := []string{
		"AUTH_VIEWER_ROLES", "AUTH_COMMENTER_ROLES", "AUTH_ESCALATOR_ROLES",
		"AUTH_ATTACHMENT_DOWNLOADER_ROLES", "AUTH_USAGE_METRICS_VIEWER_ROLES",
		"AUTH_SUPPORT_ENGINEER_ROLES", "AUTH_ADMIN_ROLES", "AUTH_TIMECARD_APPROVER_ROLES",
		"AUTH_DASHBOARD_DESIGNER_ROLES",
	}
	resetEnv := func(t *testing.T) {
		for _, name := range envs {
			t.Setenv(name, "")
		}
	}

	t.Run("unset falls back to the default role names", func(t *testing.T) {
		resetEnv(t)
		got := loadAccessConfig()
		want := handler.DefaultAccessConfig()
		if !slices.Equal(got.Viewer, want.Viewer) || !slices.Equal(got.Admin, want.Admin) ||
			!slices.Equal(got.DashboardDesigner, want.DashboardDesigner) {
			t.Errorf("loadAccessConfig() = %+v, want defaults %+v", got, want)
		}
	})

	t.Run("a configured value replaces the default and may list several roles", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("AUTH_COMMENTER_ROLES", " corp-notes , corp-interns ,, ")
		got := loadAccessConfig()
		if want := []string{"corp-notes", "corp-interns"}; !slices.Equal(got.Commenter, want) {
			t.Errorf("Commenter = %v, want %v", got.Commenter, want)
		}
		if want := handler.DefaultAccessConfig().SupportEngineer; !slices.Equal(got.SupportEngineer, want) {
			t.Errorf("SupportEngineer = %v, want the untouched default %v", got.SupportEngineer, want)
		}
	})

	t.Run("a value of only commas or spaces falls back to the default", func(t *testing.T) {
		resetEnv(t)
		t.Setenv("AUTH_ADMIN_ROLES", " , ,")
		if got, want := loadAccessConfig().Admin, handler.DefaultAccessConfig().Admin; !slices.Equal(got, want) {
			t.Errorf("Admin = %v, want default %v", got, want)
		}
	})
}
