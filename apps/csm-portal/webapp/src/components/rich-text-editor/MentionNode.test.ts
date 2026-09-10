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

import { describe, expect, it } from "vitest";
import { $generateHtmlFromNodes, $generateNodesFromDOM } from "@lexical/html";
import {
  $getRoot,
  $createParagraphNode,
  $createTextNode,
  $isElementNode,
  createEditor,
  type LexicalEditor,
} from "lexical";
import {
  $createMentionNode,
  $isMentionNode,
  MentionNode,
  type SerializedMentionNode,
} from "@components/rich-text-editor/MentionNode";

/** Minimal headless editor, mirroring this editor's own registered-nodes
 * shape closely enough for MentionNode's own HTML round trip. */
function createTestEditor(): LexicalEditor {
  return createEditor({
    namespace: "MentionNodeTest",
    nodes: [MentionNode],
    onError: (error) => {
      throw error;
    },
  });
}

describe("MentionNode", () => {
  it("exports to the <span data-mention-user-id> shape the editor's HTML pipeline expects", () => {
    const editor = createTestEditor();
    let html = "";

    editor.update(
      () => {
        const root = $getRoot();
        const paragraph = $createParagraphNode();
        paragraph.append(
          $createTextNode("Hey "),
          $createMentionNode("Jane Doe", "00000000-0000-0000-0000-000000000001"),
          $createTextNode(" take a look"),
        );
        root.clear();
        root.append(paragraph);
      },
      { discrete: true },
    );

    editor.getEditorState().read(() => {
      html = $generateHtmlFromNodes(editor);
    });

    expect(html).toContain(
      '<span data-mention-user-id="00000000-0000-0000-0000-000000000001">@Jane Doe</span>',
    );
  });

  it("imports the same <span data-mention-user-id> markup back into a MentionNode", () => {
    const editor = createTestEditor();
    const html =
      '<p>Hey <span data-mention-user-id="00000000-0000-0000-0000-000000000002">@John Smith</span> please review</p>';

    const found: { userId?: string; mentionName?: string; text?: string } = {};

    editor.update(
      () => {
        const dom = new DOMParser().parseFromString(html, "text/html");
        const nodes = $generateNodesFromDOM(editor, dom);
        const root = $getRoot();
        root.clear();
        root.append(...nodes);

        const paragraph = root.getFirstChild();
        if (!paragraph || !$isElementNode(paragraph)) return;
        for (const child of paragraph.getChildren()) {
          if ($isMentionNode(child)) {
            found.userId = child.__userId;
            found.mentionName = child.__mention;
            found.text = child.getTextContent();
          }
        }
      },
      { discrete: true },
    );

    expect(found.userId).toBe("00000000-0000-0000-0000-000000000002");
    expect(found.mentionName).toBe("John Smith");
    expect(found.text).toBe("@John Smith");
  });

  it("round-trips through exportJSON/importJSON", () => {
    const editor = createTestEditor();
    const exported: { json?: SerializedMentionNode } = {};

    editor.update(
      () => {
        const node = $createMentionNode("Ada Lovelace", "00000000-0000-0000-0000-000000000003");
        exported.json = node.exportJSON();
      },
      { discrete: true },
    );

    expect(exported.json?.type).toBe("mention");
    expect(exported.json?.mentionName).toBe("Ada Lovelace");
    expect(exported.json?.userId).toBe("00000000-0000-0000-0000-000000000003");

    const restored: { userId?: string; mentionName?: string; text?: string } = {};
    editor.update(
      () => {
        const node = MentionNode.importJSON(exported.json as SerializedMentionNode);
        restored.userId = node.__userId;
        restored.mentionName = node.__mention;
        restored.text = node.getTextContent();
      },
      { discrete: true },
    );

    expect(restored.mentionName).toBe("Ada Lovelace");
    expect(restored.userId).toBe("00000000-0000-0000-0000-000000000003");
    expect(restored.text).toBe("@Ada Lovelace");
  });
});
