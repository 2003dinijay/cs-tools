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

// "Today I Learned" Chat App — the "+" / slash-command Dialog.
//
// See README.md in this folder for what this is, why it calls til-backend
// as a service account rather than the human's own token, and the deploy
// steps (which need Google Workspace admin console access this project
// doesn't have).
//
// Script property TIL_BACKEND_URL must be set (Project Settings -> Script
// properties) before this does anything useful.

const WHERE_OPTIONS = ["Customer", "Partner", "Internal", "Other"];
const WHAT_MAX_LENGTH = 5000;

/** Entry point Chat calls for the "/learn" slash command. */
function onMessage(event) {
  return openSubmitDialog();
}

/** Entry point Chat calls for the "+" menu item (same manifest-declared trigger). */
function onAppCommand(event) {
  return openSubmitDialog();
}

function openSubmitDialog() {
  return {
    actionResponse: {
      type: "DIALOG",
      dialogAction: { dialog: { body: buildFormCard() } },
    },
  };
}

function buildFormCard() {
  return {
    sections: [
      {
        header: "Today I Learned",
        widgets: [
          {
            textParagraph: {
              text: "Your name is recorded along with your entry — this isn't anonymous.",
            },
          },
          {
            textInput: {
              label: "Who",
              type: "SINGLE_LINE",
              name: "who",
              hintText: "e.g. Jane Doe, Customer Success Engineer",
            },
          },
          {
            selectionInput: {
              name: "where",
              label: "Where",
              type: "DROPDOWN",
              items: WHERE_OPTIONS.map((opt) => ({
                text: opt,
                value: opt,
                selected: false,
              })),
            },
          },
          {
            textInput: {
              label: `What (max ${WHAT_MAX_LENGTH} characters)`,
              type: "MULTIPLE_LINE",
              name: "what",
            },
          },
        ],
      },
    ],
    fixedFooter: {
      primaryButton: {
        text: "Share",
        onClick: {
          action: { function: "onSubmitForm" },
        },
      },
    },
  };
}

/** Called when the Dialog's Share button is clicked. */
function onSubmitForm(event) {
  const formInputs = event.common.formInputs;
  const who = readTextInput(formInputs, "who");
  const where = readTextInput(formInputs, "where");
  const what = readTextInput(formInputs, "what");

  const error = validate(who, where, what);
  if (error) {
    return statusDialog(error, /* isError= */ true);
  }

  // event.user.email can be absent (Chat user records don't always carry
  // it) -- if it is, submitEntry would send no onBehalfOfEmail at all, and
  // til-backend falls back to attributing the entry to THIS SCRIPT'S OWN
  // service account instead of rejecting it. Fail the submission instead of
  // silently misattributing it.
  if (!event.user.email) {
    return statusDialog(
      "Couldn't identify your account email -- please try again, or submit from the One WSO2 page instead.",
      /* isError= */ true,
    );
  }

  try {
    submitEntry(who, where, what, event.user.email);
  } catch (err) {
    return statusDialog(`Couldn't submit: ${err.message}`, /* isError= */ true);
  }

  return {
    actionResponse: {
      type: "DIALOG",
      dialogAction: {
        actionStatus: { statusCode: "OK", userFacingMessage: "Thanks for sharing!" },
      },
    },
  };
}

function readTextInput(formInputs, name) {
  // Apps Script wraps each widget's value under an extra empty-string key
  // that other Chat API client shapes (HTTP, other languages) don't have --
  // confirmed against Google's own form-data guide: "If your Chat app is
  // built with Apps Script, you must add [""] after the widget name,
  // e.g. formInputs.contactName[""].stringInputs.value[0]."
  const field = formInputs && formInputs[name] && formInputs[name][""];
  const value = field && field.stringInputs && field.stringInputs.value;
  return value && value.length > 0 ? value[0].trim() : "";
}

function validate(who, where, what) {
  if (!who) return "Who is required.";
  if (WHERE_OPTIONS.indexOf(where) === -1) return "Where must be one of: " + WHERE_OPTIONS.join(", ");
  if (!what) return "What is required.";
  if (what.length > WHAT_MAX_LENGTH) return `What must be ${WHAT_MAX_LENGTH} characters or fewer.`;
  return null;
}

/**
 * POSTs to the same til-backend endpoint the One WSO2 page calls.
 *
 * Authenticates as this Chat App's own service account (a Bearer token this
 * function is responsible for obtaining -- left as getServiceAccountToken()
 * below, to be filled in against whatever this org's real service-account
 * auth flow is, same shape as the PAR migration tool's own service-account
 * ROPC call). Separately asserts onBehalfOfEmail = the real human who
 * clicked Share, which til-backend only honors from this exact service
 * account identity -- see til-backend's chat_app/README.md and main.py.
 */
function submitEntry(who, where, what, submitterEmail) {
  const backendUrl = PropertiesService.getScriptProperties().getProperty("TIL_BACKEND_URL");
  if (!backendUrl) {
    throw new Error("TIL_BACKEND_URL script property is not set.");
  }

  const response = UrlFetchApp.fetch(`${backendUrl}/submissions`, {
    method: "post",
    contentType: "application/json",
    headers: { Authorization: `Bearer ${getServiceAccountToken()}` },
    payload: JSON.stringify({ who, where, what, onBehalfOfEmail: submitterEmail }),
    muteHttpExceptions: true,
  });

  if (response.getResponseCode() >= 300) {
    throw new Error(response.getContentText());
  }
}

/** TODO: fill in against this org's real service-account auth flow. */
function getServiceAccountToken() {
  throw new Error(
    "getServiceAccountToken() is not implemented yet -- see chat_app/README.md",
  );
}

function statusDialog(message, isError) {
  return {
    actionResponse: {
      type: "DIALOG",
      dialogAction: {
        actionStatus: {
          statusCode: isError ? "INVALID_ARGUMENT" : "OK",
          userFacingMessage: message,
        },
      },
    },
  };
}
