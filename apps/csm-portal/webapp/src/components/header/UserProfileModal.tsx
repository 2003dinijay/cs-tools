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

import {
  Alert,
  Autocomplete,
  Avatar,
  Box,
  Button,
  Chip,
  Dialog,
  IconButton,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { PencilLine, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useCallback, useEffect, useMemo, useState } from "react";
import { useGetUsersMe } from "@features/settings/api/useGetUsersMe";
import {
  usePatchUsersMe,
  type UpdateUserPayload,
} from "@features/settings/api/usePatchUsersMe";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { initialsOf, resolveUserInfo } from "@utils/userClaims";
import {
  listSupportedTimeZones,
  normalizeUserTimeZone,
} from "@utils/dateTime";
import {
  E164,
  platformPhoneState,
  savingRepairsPlatform,
} from "./profilePhone";

// Show at most this many role chips inline before collapsing with "+N more".
const ROLES_PREVIEW_LIMIT = 4;

export interface UserProfileModalProps {
  open: boolean;
  onClose: () => void;
}

export default function UserProfileModal({
  open,
  onClose,
}: UserProfileModalProps): JSX.Element {
  const claims = useIdTokenClaims();
  const { data: userMe, isLoading } = useGetUsersMe();
  const patchUserMe = usePatchUsersMe();
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();

  const [phoneDraft, setPhoneDraft] = useState("");
  const [isPhoneEditing, setIsPhoneEditing] = useState(false);
  const [tzDraft, setTzDraft] = useState<string | null>(null);
  const [isTzEditing, setIsTzEditing] = useState(false);

  const info = resolveUserInfo(claims);
  const email = userMe?.email || info.email || "—";
  const initials = initialsOf(info.fullName);
  const timeZoneOptions = useMemo(() => listSupportedTimeZones(), []);

  useEffect(() => {
    if (open && userMe) {
      // Seed the editable drafts from the latest server values each time the
      // dialog opens (the accepted "reset form state on open" pattern).
      /* eslint-disable react-hooks/set-state-in-effect -- form reset on dialog open */
      setPhoneDraft(userMe.phoneNumber ?? "");
      setTzDraft(normalizeUserTimeZone(userMe.timeZone));
      setIsPhoneEditing(false);
      setIsTzEditing(false);
      /* eslint-enable react-hooks/set-state-in-effect */
    }
  }, [open, userMe]);

  const handleEditCancel = useCallback(() => {
    setPhoneDraft(userMe?.phoneNumber ?? "");
    setTzDraft(normalizeUserTimeZone(userMe?.timeZone));
    setIsPhoneEditing(false);
    setIsTzEditing(false);
  }, [userMe?.phoneNumber, userMe?.timeZone]);

  const phoneState = platformPhoneState(
    userMe?.phoneNumber,
    userMe?.platformPhone,
  );

  // Saves the profile's own number again so the BFF writes it to the platform
  // record too (PATCH /users/me updates both). The shown number has not
  // changed, which is why a plain Save used to send nothing.
  const handleUseProfileNumberForAlerts = useCallback(() => {
    const profile = (userMe?.phoneNumber ?? "").trim();
    if (!profile) return;
    patchUserMe.mutate(
      { phoneNumber: profile },
      {
        onSuccess: () => {
          showSuccess("Case Alerts calls now use your profile number.");
        },
        onError: () => {
          showError("Could not update the number Case Alerts calls. Please try again.");
        },
      },
    );
  }, [userMe?.phoneNumber, patchUserMe, showError, showSuccess]);

  const handleSave = useCallback(() => {
    const currentPhone = userMe?.phoneNumber ?? "";
    const nextPhone = phoneDraft.trim();
    // A number equal to the shown one is still sent when the platform record
    // (what Case Alerts dial) holds another: saving is how it gets repaired.
    const phoneChanged =
      nextPhone !== currentPhone ||
      (isPhoneEditing && savingRepairsPlatform(nextPhone, userMe?.platformPhone));

    const currentTz = normalizeUserTimeZone(userMe?.timeZone) ?? "";
    const nextTz = tzDraft ?? "";
    const tzChanged = nextTz !== currentTz;

    if (!phoneChanged && !tzChanged) {
      onClose();
      return;
    }

    if (phoneChanged && nextPhone && !E164.test(nextPhone)) {
      showError("Phone number must be in E.164 format, e.g. +14155552671.");
      return;
    }

    // Time zone has no "unset" on the backend; clearing the picker (empty
    // draft) must not PATCH `timeZone: ""`. Require a valid zone when changed.
    if (tzChanged && (!nextTz || !normalizeUserTimeZone(nextTz))) {
      showError("Select a valid time zone.");
      return;
    }

    const payload: UpdateUserPayload = {
      ...(phoneChanged && { phoneNumber: nextPhone }),
      ...(tzChanged && { timeZone: nextTz }),
    };

    patchUserMe.mutate(payload, {
      onSuccess: () => {
        // The mutation invalidates `users-me`; CurrentUserProvider re-seeds the
        // preferred-timezone store from the refetched profile.
        showSuccess("Profile updated.");
        setIsPhoneEditing(false);
        setIsTzEditing(false);
        onClose();
      },
      onError: () => {
        showError("Could not update profile. Please try again.");
      },
    });
  }, [
    phoneDraft,
    tzDraft,
    isPhoneEditing,
    userMe?.phoneNumber,
    userMe?.platformPhone,
    userMe?.timeZone,
    patchUserMe,
    showError,
    showSuccess,
    onClose,
  ]);

  // The platform authorizes off its own data, not IdP claims (Asgardeo groups
  // are auth-only, not a role list) — so this shows `/users/me`'s `roles`
  // rather than `claims.groups`.
  const roles = userMe?.roles ?? [];
  const visibleRoles = roles.slice(0, ROLES_PREVIEW_LIMIT);
  const hiddenRoleCount = roles.length - visibleRoles.length;

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <Box sx={{ p: 3, display: "flex", flexDirection: "column", gap: 3 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <Typography variant="h6">Profile</Typography>
          <IconButton onClick={onClose} size="small" aria-label="Close">
            <X size={18} />
          </IconButton>
        </Box>

        <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
          <Avatar
            src={info.avatarUrl}
            imgProps={{ referrerPolicy: "no-referrer" }}
            sx={{ width: 64, height: 64, fontSize: 24 }}
          >
            {initials}
          </Avatar>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography variant="subtitle1" noWrap>
              {info.fullName}
            </Typography>
            <Typography variant="body2" color="text.secondary" noWrap>
              {email}
            </Typography>
            {info.orgName && (
              <Typography variant="caption" color="text.secondary">
                {info.orgName}
                {info.orgHandle && info.orgHandle !== info.orgName
                  ? ` (${info.orgHandle})`
                  : ""}
              </Typography>
            )}
          </Box>
        </Box>

        {(isLoading || roles.length > 0) && (
          <Box>
            <Typography variant="caption" color="text.secondary">
              Roles
            </Typography>
            <Box
              sx={{
                display: "flex",
                flexWrap: "wrap",
                gap: 0.5,
                mt: 0.5,
              }}
            >
              {isLoading ? (
                <Skeleton width={160} />
              ) : (
                <>
                  {visibleRoles.map((r) => (
                    <Chip key={r} size="small" label={r} variant="outlined" />
                  ))}
                  {hiddenRoleCount > 0 && (
                    <Chip
                      size="small"
                      label={`+${hiddenRoleCount} more`}
                      variant="outlined"
                      color="default"
                    />
                  )}
                </>
              )}
            </Box>
          </Box>
        )}

        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box>
            <Typography variant="caption" color="text.secondary">
              Phone number
            </Typography>
            {isPhoneEditing ? (
              <Box sx={{ display: "flex", gap: 1, alignItems: "center", mt: 0.5 }}>
                <TextField
                  size="small"
                  fullWidth
                  placeholder="+14155552671"
                  value={phoneDraft}
                  onChange={(e) => setPhoneDraft(e.target.value)}
                  helperText="E.164 format: + followed by country code and number"
                />
              </Box>
            ) : (
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  mt: 0.5,
                }}
              >
                <Typography variant="body2">
                  {isLoading ? (
                    <Skeleton width={160} />
                  ) : (
                    userMe?.phoneNumber || "Not set"
                  )}
                </Typography>
                <IconButton
                  size="small"
                  onClick={() => setIsPhoneEditing(true)}
                  aria-label="Edit phone number"
                >
                  <PencilLine size={16} />
                </IconButton>
              </Box>
            )}
            {!isLoading && (
              <PlatformPhoneNote
                state={phoneState}
                editing={isPhoneEditing}
                pending={patchUserMe.isPending}
                onUseProfileNumber={handleUseProfileNumberForAlerts}
              />
            )}
          </Box>

          <Box>
            <Typography variant="caption" color="text.secondary">
              Time zone
            </Typography>
            {isTzEditing ? (
              <Box sx={{ display: "flex", gap: 1, alignItems: "center", mt: 0.5 }}>
                <Autocomplete
                  size="small"
                  fullWidth
                  options={timeZoneOptions}
                  value={tzDraft}
                  onChange={(_, next) => setTzDraft(next)}
                  renderInput={(params) => (
                    <TextField
                      {...params}
                      placeholder="Select a time zone"
                      helperText="Used to display dates and times in your zone"
                    />
                  )}
                />
              </Box>
            ) : (
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  mt: 0.5,
                }}
              >
                <Typography variant="body2">
                  {isLoading ? (
                    <Skeleton width={160} />
                  ) : (
                    normalizeUserTimeZone(userMe?.timeZone) || "Not set"
                  )}
                </Typography>
                <IconButton
                  size="small"
                  onClick={() => setIsTzEditing(true)}
                  aria-label="Edit time zone"
                >
                  <PencilLine size={16} />
                </IconButton>
              </Box>
            )}
          </Box>
        </Box>

        <Stack direction="row" justifyContent="flex-end" spacing={1}>
          {(isPhoneEditing || isTzEditing) && (
            <Button variant="text" onClick={handleEditCancel}>
              Cancel
            </Button>
          )}
          <Button
            variant="contained"
            onClick={handleSave}
            disabled={patchUserMe.isPending}
          >
            {isPhoneEditing || isTzEditing ? "Save" : "Close"}
          </Button>
        </Stack>
      </Box>
    </Dialog>
  );
}

interface PlatformPhoneNoteProps {
  state: ReturnType<typeof platformPhoneState>;
  editing: boolean;
  pending: boolean;
  onUseProfileNumber: () => void;
}

/**
 * Says so when Case Alerts calls a different number from the one shown above
 * (the platform record's copy has drifted), and offers to fix it. Nothing is
 * shown while the two agree or the platform copy is unknown.
 */
function PlatformPhoneNote({
  state,
  editing,
  pending,
  onUseProfileNumber,
}: PlatformPhoneNoteProps): JSX.Element | null {
  if (state.kind === "unknown" || state.kind === "same") return null;

  let message: string;
  let canRepair = false;
  switch (state.kind) {
    case "differs":
      message = `Case Alerts calls go to ${state.platform}, the number on your platform record, not the number above.`;
      canRepair = E164.test(state.profile);
      break;
    case "platformMissing":
      message =
        "Your platform record has no phone number, so Case Alerts calls can't reach you on the number above.";
      canRepair = E164.test(state.profile);
      break;
    case "profileMissing":
      message = `Case Alerts calls go to ${state.platform}, which isn't shown above. Set your phone number above to change it.`;
      break;
  }
  if (editing) {
    message += " Saving updates both.";
  }

  return (
    <Alert
      severity="warning"
      sx={{ mt: 1 }}
      action={
        canRepair && !editing ? (
          <Button
            color="inherit"
            size="small"
            onClick={onUseProfileNumber}
            disabled={pending}
          >
            Use this number for alerts
          </Button>
        ) : undefined
      }
    >
      {message}
    </Alert>
  );
}
