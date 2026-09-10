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

import { useCallback, useMemo, useState, type JSX } from "react";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import {
  LexicalTypeaheadMenuPlugin,
  MenuOption,
  useBasicTypeaheadTriggerMatch,
} from "@lexical/react/LexicalTypeaheadMenuPlugin";
import type { TextNode } from "lexical";
import { Box, CircularProgress, MenuItem, MenuList, Paper, Typography } from "@wso2/oxygen-ui";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchUsersByName } from "@api/useSearchUsersByName";
import { $createMentionNode } from "@components/rich-text-editor/MentionNode";

/** Up to this many matches are offered per keystroke — a mention picker is a
 * quick "find the right person," not a full directory browser. */
const MAX_MENTION_RESULTS = 10;
const MENTION_QUERY_DEBOUNCE_MS = 300;

class MentionTypeaheadOption extends MenuOption {
  userId: string;
  name: string;

  constructor(userId: string, name: string) {
    super(userId);
    this.userId = userId;
    this.name = name;
  }
}

/**
 * `@mention` typeahead for the case-comment composer, built on
 * `@lexical/react`'s `LexicalTypeaheadMenuPlugin` (keyboard nav, trigger
 * matching, and popup positioning all come from that shared building
 * block — this plugin only supplies the trigger pattern, the user search,
 * and what happens on selection).
 *
 * Mentionable users are restricted to WSO2/CSM staff (`userType ===
 * "internal"`, as returned by `POST /users/search` — the response has no
 * server-side filter for this, so it's applied client-side here). Customer
 * contacts, also returned by that same endpoint, are never offered.
 */
export default function MentionsPlugin(): JSX.Element | null {
  const [editor] = useLexicalComposerContext();
  const [queryString, setQueryString] = useState<string | null>(null);
  const debouncedQuery = useDebouncedValue(queryString ?? "", MENTION_QUERY_DEBOUNCE_MS);

  const { data: users, isFetching } = useSearchUsersByName(
    debouncedQuery,
    queryString !== null,
  );

  const options = useMemo(() => {
    return (users ?? [])
      .filter((u) => u.userType === "internal" && !!u.id)
      .slice(0, MAX_MENTION_RESULTS)
      .map(
        (u) =>
          new MentionTypeaheadOption(
            u.id as string,
            [u.firstName, u.lastName].filter(Boolean).join(" ") ||
              u.userName ||
              u.email ||
              "Unknown user",
          ),
      );
  }, [users]);

  // "@" followed by word characters, no whitespace — the standard mention
  // trigger shape `useBasicTypeaheadTriggerMatch` is built for.
  const checkForMentionMatch = useBasicTypeaheadTriggerMatch("@", {
    minLength: 0,
  });

  const onSelectOption = useCallback(
    (
      selectedOption: MentionTypeaheadOption,
      nodeToReplace: TextNode | null,
      closeMenu: () => void,
    ) => {
      editor.update(() => {
        const mentionNode = $createMentionNode(
          selectedOption.name,
          selectedOption.userId,
        );
        if (nodeToReplace) {
          nodeToReplace.replace(mentionNode);
        }
        mentionNode.selectNext();
      });
      closeMenu();
    },
    [editor],
  );

  return (
    <LexicalTypeaheadMenuPlugin<MentionTypeaheadOption>
      onQueryChange={setQueryString}
      onSelectOption={onSelectOption}
      triggerFn={checkForMentionMatch}
      options={options}
      menuRenderFn={(anchorElementRef, { selectedIndex, selectOptionAndCleanUp, setHighlightedIndex }) => {
        if (!anchorElementRef.current || queryString === null) return null;

        return (
          <Paper
            variant="outlined"
            sx={{ minWidth: 220, maxWidth: 320, py: 0.5, boxShadow: 3 }}
          >
            {isFetching && options.length === 0 ? (
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, px: 1.5, py: 1 }}>
                <CircularProgress size={14} />
                <Typography variant="caption" color="text.secondary">
                  Searching…
                </Typography>
              </Box>
            ) : options.length === 0 ? (
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ display: "block", px: 1.5, py: 1 }}
              >
                No matching users
              </Typography>
            ) : (
              <MenuList dense sx={{ py: 0 }}>
                {options.map((option, index) => (
                  <MenuItem
                    key={option.key}
                    selected={index === selectedIndex}
                    ref={option.setRefElement}
                    onMouseEnter={() => setHighlightedIndex(index)}
                    onClick={() => selectOptionAndCleanUp(option)}
                  >
                    {option.name}
                  </MenuItem>
                ))}
              </MenuList>
            )}
          </Paper>
        );
      }}
    />
  );
}
