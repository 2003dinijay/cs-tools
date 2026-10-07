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

import ballerina/test;
import ballerina/time;

function stripCases() returns map<[string, string]> {
    return {
        "leading block and its gap are removed": [
            "<thinking> The user asks about Widget 2.\n- Dev: Widget 1\nI should ask.\n</thinking>\n\nI don't see Widget 2 in your environments.",
            "I don't see Widget 2 in your environments."
        ],
        "every block, in any letter case": ["A<thinking>x</thinking>B<Thinking>y</THINKING>C", "ABC"],
        "block that is never closed": ["<thinking>The user wants", ""],
        "unclosed block after some answer": ["Done.\n<thinking>more reasoning", "Done.\n"],
        "half-written opening tag": ["Hello <t", "Hello "],
        "longer half-written opening tag": ["Hello <thin", "Hello "],
        "opening tag without its bracket": ["Hello <thinking", "Hello "],
        "indentation kept when a later block is removed": ["    code\n<thinking>reason</thinking>", "    code\n"],
        "first real line keeps its indentation": ["<thinking>x</thinking>\n\n    code", "    code"],
        "inline gap after a leading block is dropped": ["<thinking>x</thinking>   Answer", "Answer"],
        "whitespace before a leading block is dropped": ["\n <thinking>x</thinking>\nAnswer", "Answer"],
        "non-ASCII text survives": ["é<thinking>x</thinking>ü — héllo ✓", "éü — héllo ✓"],
        "only reasoning": ["<thinking>just this</thinking>", ""],
        "markup that only looks similar is untouched": [
            "Use <table><thead><tr><th>A</th></tr></thead></table> markup",
            "Use <table><thead><tr><th>A</th></tr></thead></table> markup"
        ],
        "a comparison is untouched": ["if (a <t) {}", "if (a <t) {}"],
        "a similarly named tag is untouched": [
            "<thinking-time> custom tag</thinking-time>",
            "<thinking-time> custom tag</thinking-time>"
        ],
        "title tag is untouched": ["<title>Doc</title>", "<title>Doc</title>"],
        "no angle bracket at all": ["plain answer", "plain answer"],
        "empty": ["", ""]
    };
}

@test:Config {dataProvider: stripCases}
function testStripThinkingBlocks(string input, string expected) {
    test:assertEquals(stripThinkingBlocks(input), expected);
}

// A rescan of the rest of the text for every unclosed opener takes several seconds
// on this input; a single pass takes well under a millisecond. The bound is ~1000x
// the real cost so load cannot flake it.
@test:Config {}
function testStripThinkingBlocksIsLinearInTheNumberOfOpeners() {
    string[] openers = [];
    foreach int _ in 0 ..< 50000 {
        openers.push("<thinking>x");
    }
    string input = string:'join("", ...openers);

    time:Utc started = time:utcNow();
    string result = stripThinkingBlocks(input);
    decimal elapsed = time:utcDiffSeconds(time:utcNow(), started);

    test:assertEquals(result, "");
    test:assertTrue(elapsed < 1.0d, string `took ${elapsed}s, want a single linear pass`);
}

@test:Config {}
function testScrubFinalEventCleansTheNestedPayload() returns error? {
    map<json> event = {
        "type": "final",
        "payload": {
            "message": "<thinking>internal notes</thinking>\n\nWhich <b>gateway</b>?",
            "conversationId": "c1",
            "resolved": false
        }
    };
    string raw = event.toJsonString();

    [string, map<json>] result = scrubFinalEvent(event.clone(), raw);

    map<json> forwarded = <map<json>>check result[0].fromJsonString();
    map<json> forwardedPayload = <map<json>>forwarded["payload"];
    test:assertEquals(forwarded["type"], "final");
    test:assertEquals(forwardedPayload["message"], "Which <b>gateway</b>?");
    test:assertEquals(forwardedPayload["conversationId"], "c1");
    test:assertEquals(forwardedPayload["resolved"], false);
    // What the portal persists as the conversation comment.
    test:assertEquals(result[1]["message"], "Which <b>gateway</b>?");
    test:assertEquals(result[1]["conversationId"], "c1");
}

@test:Config {}
function testScrubFinalEventForwardsAnOrdinaryAnswerByteForByte() {
    map<json> event = {"type": "final", "payload": {"message": "Which <b>gateway</b>?", "conversationId": "c1"}};
    string raw = "{ \"type\": \"final\", \"payload\": { \"message\": \"Which <b>gateway</b>?\" } }";

    [string, map<json>] result = scrubFinalEvent(event.clone(), raw);

    test:assertEquals(result[0], raw);
    test:assertEquals(result[1]["message"], "Which <b>gateway</b>?");
}

@test:Config {}
function testScrubFinalEventHandlesTheOlderFlatShape() {
    map<json> event = {"type": "final", "message": "<thinking>x</thinking>Hello", "conversationId": "c1"};

    [string, map<json>] result = scrubFinalEvent(event.clone(), event.toJsonString());

    test:assertEquals(result[1]["message"], "Hello");
    test:assertTrue(result[0].includes("\"message\":\"Hello\""));
}

@test:Config {}
function testScrubFinalEventLeavesUnexpectedShapesAlone() {
    map<json>[] events = [
        {"type": "final", "payload": {"message": {"message": "<thinking>x</thinking>Hi"}}},
        {"type": "final", "payload": {"conversationId": "c1"}},
        {"type": "final", "payload": "<thinking>x</thinking>Hi"}
    ];
    foreach map<json> event in events {
        string raw = event.toJsonString();
        [string, map<json>] result = scrubFinalEvent(event.clone(), raw);
        test:assertEquals(result[0], raw);
    }
}
