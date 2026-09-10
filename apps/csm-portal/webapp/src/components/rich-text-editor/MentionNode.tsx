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
  TextNode,
  type DOMConversionMap,
  type DOMConversionOutput,
  type DOMExportOutput,
  type EditorConfig,
  type LexicalNode,
  type NodeKey,
  type SerializedTextNode,
  type Spread,
} from "lexical";

/** The attribute this editor's HTML pipeline round-trips a mention through
 * (see `@lexical/html`'s `$generateHtmlFromNodes`/`$generateNodesFromDOM` in
 * Editor.tsx) — present on export, and the signal `importDOM` below looks
 * for to reconstruct the node on paste/initial-value load. */
const MENTION_USER_ID_ATTRIBUTE = "data-mention-user-id";

export type SerializedMentionNode = Spread<
  {
    mentionName: string;
    userId: string;
  },
  SerializedTextNode
>;

function convertMentionElement(domNode: HTMLElement): DOMConversionOutput | null {
  const userId = domNode.getAttribute(MENTION_USER_ID_ATTRIBUTE);
  if (!userId) return null;
  // Display text is "@Name" — strip the leading "@" back out for mentionName.
  const text = domNode.textContent ?? "";
  const mentionName = text.startsWith("@") ? text.slice(1) : text;
  return { node: $createMentionNode(mentionName, userId) };
}

/**
 * An inline, atomic `@mention` — extends `TextNode` (per Lexical's own
 * documented mention pattern) so it behaves as a single unit within text
 * flow: selectable/deletable as a whole, not editable letter-by-letter
 * (`isTextEntity`/`canInsertTextBefore`/`canInsertTextAfter` below).
 */
export class MentionNode extends TextNode {
  __mention: string;
  __userId: string;

  static getType(): string {
    return "mention";
  }

  static clone(node: MentionNode): MentionNode {
    return new MentionNode(node.__mention, node.__userId, node.__text, node.__key);
  }

  static importJSON(serializedNode: SerializedMentionNode): MentionNode {
    const node = $createMentionNode(
      serializedNode.mentionName,
      serializedNode.userId,
    );
    node.setTextContent(serializedNode.text);
    node.setFormat(serializedNode.format);
    node.setDetail(serializedNode.detail);
    node.setMode(serializedNode.mode);
    node.setStyle(serializedNode.style);
    return node;
  }

  constructor(mentionName: string, userId: string, text?: string, key?: NodeKey) {
    super(text ?? `@${mentionName}`, key);
    this.__mention = mentionName;
    this.__userId = userId;
  }

  exportJSON(): SerializedMentionNode {
    return {
      ...super.exportJSON(),
      mentionName: this.__mention,
      userId: this.__userId,
      type: "mention",
      version: 1,
    };
  }

  createDOM(config: EditorConfig): HTMLElement {
    const dom = super.createDOM(config);
    dom.className = "editor-mention";
    return dom;
  }

  exportDOM(): DOMExportOutput {
    const element = document.createElement("span");
    element.setAttribute(MENTION_USER_ID_ATTRIBUTE, this.__userId);
    element.textContent = this.__text;
    return { element };
  }

  static importDOM(): DOMConversionMap<HTMLElement> | null {
    return {
      span: (domNode: HTMLElement) => {
        if (!domNode.hasAttribute(MENTION_USER_ID_ATTRIBUTE)) return null;
        return {
          conversion: convertMentionElement,
          priority: 1,
        };
      },
    };
  }

  isTextEntity(): true {
    return true;
  }

  canInsertTextBefore(): boolean {
    return false;
  }

  canInsertTextAfter(): boolean {
    return false;
  }
}

export function $createMentionNode(mentionName: string, userId: string): MentionNode {
  const node = new MentionNode(mentionName, userId);
  node.setMode("segmented").toggleDirectionless();
  return node;
}

export function $isMentionNode(
  node: LexicalNode | null | undefined,
): node is MentionNode {
  return node instanceof MentionNode;
}
