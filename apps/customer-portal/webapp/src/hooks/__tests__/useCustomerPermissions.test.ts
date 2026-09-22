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

import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import {
  useCustomerPermissions,
  normalizeCustomerRoles,
  hasCustomerPermission,
  type CanonicalRole,
  type CustomerModule,
  type CustomerAction,
} from "../useCustomerPermissions";

vi.mock("@features/settings/api/useGetUserDetails");

const ALL_MODULES: CustomerModule[] = [
  "cases",
  "time_cards",
  "projects",
  "change_requests",
  "deployments",
  "deployment_products",
  "deployment_resources",
  "security_admin",
];

const ALL_ACTIONS: CustomerAction[] = ["create", "read", "update", "delete"];

describe("useCustomerPermissions & Permission Matrix", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("Role Normalization", () => {
    it("normalizes ServiceNow role strings to canonical names", () => {
      const roles = normalizeCustomerRoles([
        "sn_customerservice.admin",
        "wso2_agent",
        "sn_customerservice.customer_admin",
        "sn_customerservice.customer",
        "sn_customerservice.partner_admin",
        "sn_customerservice.partner",
      ]);

      expect(roles).toEqual([
        "admin",
        "agent",
        "customer_admin",
        "customer_user",
        "partner_admin",
        "partner_user",
      ]);
    });

    it("handles empty and duplicate roles", () => {
      expect(normalizeCustomerRoles([])).toEqual([]);
      expect(normalizeCustomerRoles(undefined)).toEqual([]);
      expect(
        normalizeCustomerRoles(["admin", "sn_customerservice.admin"]),
      ).toEqual(["admin"]);
    });
  });

  describe("Permission Matrix Rules", () => {
    // super_admin and stakeholder were removed: no real role emits either
    // name, so their grants were unreachable.
    it("no role grants security_admin", () => {
      const everyRole: CanonicalRole[] = [
        "admin",
        "agent",
        "customer_admin",
        "customer_user",
        "partner_admin",
        "partner_user",
        "internal",
      ];
      for (const role of everyRole) {
        for (const act of ALL_ACTIONS) {
          expect(hasCustomerPermission([role], "security_admin", act)).toBe(false);
        }
      }
    });

    it("Admin has full CRUD on 7 modules and zero access to Security Admin", () => {
      const roles: CanonicalRole[] = ["admin"];
      for (const mod of ALL_MODULES) {
        if (mod === "security_admin") {
          for (const act of ALL_ACTIONS) {
            expect(hasCustomerPermission(roles, mod, act)).toBe(false);
          }
        } else {
          for (const act of ALL_ACTIONS) {
            expect(hasCustomerPermission(roles, mod, act)).toBe(true);
          }
        }
      }
    });

    it("Agent has CRU on Cases/Timecards/CRs, RU on Projects, CRUD on Deployments, zero on Security Admin", () => {
      const roles: CanonicalRole[] = ["agent"];

      // Cases: CRU, no D
      expect(hasCustomerPermission(roles, "cases", "create")).toBe(true);
      expect(hasCustomerPermission(roles, "cases", "read")).toBe(true);
      expect(hasCustomerPermission(roles, "cases", "update")).toBe(true);
      expect(hasCustomerPermission(roles, "cases", "delete")).toBe(false);

      // Time Cards: CRU, no D
      expect(hasCustomerPermission(roles, "time_cards", "create")).toBe(true);
      expect(hasCustomerPermission(roles, "time_cards", "read")).toBe(true);
      expect(hasCustomerPermission(roles, "time_cards", "update")).toBe(true);
      expect(hasCustomerPermission(roles, "time_cards", "delete")).toBe(false);

      // Projects: RU, no C, no D
      expect(hasCustomerPermission(roles, "projects", "create")).toBe(false);
      expect(hasCustomerPermission(roles, "projects", "read")).toBe(true);
      expect(hasCustomerPermission(roles, "projects", "update")).toBe(true);
      expect(hasCustomerPermission(roles, "projects", "delete")).toBe(false);

      // Change Requests: CRU, no D
      expect(hasCustomerPermission(roles, "change_requests", "create")).toBe(
        true,
      );
      expect(hasCustomerPermission(roles, "change_requests", "read")).toBe(true);
      expect(hasCustomerPermission(roles, "change_requests", "update")).toBe(
        true,
      );
      expect(hasCustomerPermission(roles, "change_requests", "delete")).toBe(
        false,
      );

      // Deployments, Products, Resources: CRUD
      for (const mod of [
        "deployments",
        "deployment_products",
        "deployment_resources",
      ] as const) {
        for (const act of ALL_ACTIONS) {
          expect(hasCustomerPermission(roles, mod, act)).toBe(true);
        }
      }

      // Security Admin: Zero access
      for (const act of ALL_ACTIONS) {
        expect(hasCustomerPermission(roles, "security_admin", act)).toBe(false);
      }
    });

    it("External roles have CRU on Cases, Read on Timecards/Projects/CRs, CRUD on Deployments, zero on Security Admin", () => {
      const externalRoles: CanonicalRole[][] = [
        ["customer_user"],
        ["customer_admin"],
        ["partner_user"],
        ["partner_admin"],
      ];

      for (const roles of externalRoles) {
        // Cases: CRU, no D
        expect(hasCustomerPermission(roles, "cases", "create")).toBe(true);
        expect(hasCustomerPermission(roles, "cases", "read")).toBe(true);
        expect(hasCustomerPermission(roles, "cases", "update")).toBe(true);
        expect(hasCustomerPermission(roles, "cases", "delete")).toBe(false);

        // Timecards: Read Only
        expect(hasCustomerPermission(roles, "time_cards", "read")).toBe(true);
        expect(hasCustomerPermission(roles, "time_cards", "create")).toBe(false);
        expect(hasCustomerPermission(roles, "time_cards", "update")).toBe(false);
        expect(hasCustomerPermission(roles, "time_cards", "delete")).toBe(false);

        // Projects: Read Only
        expect(hasCustomerPermission(roles, "projects", "read")).toBe(true);
        expect(hasCustomerPermission(roles, "projects", "create")).toBe(false);
        expect(hasCustomerPermission(roles, "projects", "update")).toBe(false);
        expect(hasCustomerPermission(roles, "projects", "delete")).toBe(false);

        // Change Requests: Read Only
        expect(hasCustomerPermission(roles, "change_requests", "read")).toBe(
          true,
        );
        expect(hasCustomerPermission(roles, "change_requests", "create")).toBe(
          false,
        );
        expect(hasCustomerPermission(roles, "change_requests", "update")).toBe(
          false,
        );
        expect(hasCustomerPermission(roles, "change_requests", "delete")).toBe(
          false,
        );

        // Deployments, Products, Resources: CRUD
        for (const mod of [
          "deployments",
          "deployment_products",
          "deployment_resources",
        ] as const) {
          for (const act of ALL_ACTIONS) {
            expect(hasCustomerPermission(roles, mod, act)).toBe(true);
          }
        }

        // Security Admin: Zero access
        for (const act of ALL_ACTIONS) {
          expect(hasCustomerPermission(roles, "security_admin", act)).toBe(
            false,
          );
        }
      }
    });

  });

  describe("Hook Functionality", () => {
    // A role name nothing emits normalises to itself and matches no matrix
    // entry, so it grants nothing. super_admin is now such a name.
    it("an unrecognised role grants nothing", () => {
      vi.mocked(useGetUserDetails).mockReturnValue({
        data: {
          id: "usr-1",
          email: "super@wso2.com",
          firstName: "Super",
          lastName: "Admin",
          timeZone: "UTC",
          roles: ["sn_customerservice.super_admin"],
        },
        isLoading: false,
        isError: false,
      } as ReturnType<typeof useGetUserDetails>);

      const { result } = renderHook(() => useCustomerPermissions());

      expect(result.current.isAdmin).toBe(false);
      expect(result.current.canAccessSecurityAdmin).toBe(false);
      expect(result.current.canCreateCase).toBe(false);
      expect(result.current.canManageContacts).toBe(false);
    });

  });
});
