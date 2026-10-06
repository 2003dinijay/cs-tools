# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

"""Integration tests for the submission endpoints, with auth mocked out.

auth.require_auth is overridden via FastAPI's dependency_overrides rather
than mocking the JWKS fetch -- these tests are about main.py's own request
handling (the onBehalfOfEmail rule in particular), not about token
verification, which is auth.py's job and not exercised here.
"""
import os

from conftest import TEST_DB_NAME, bootstrap_test_database

os.environ.setdefault("DB_NAME", TEST_DB_NAME)

import pytest
from fastapi.testclient import TestClient

import db
import main
from auth import require_auth

HUMAN_USER = {"email": "jane@wso2.com", "name": "Jane", "groups": [], "is_moderator": False, "is_chat_service_account": False}
OTHER_HUMAN_USER = {"email": "sam@wso2.com", "name": "Sam", "groups": [], "is_moderator": False, "is_chat_service_account": False}
MODERATOR_USER = {"email": "mod@wso2.com", "name": "Mod", "groups": ["til-mods"], "is_moderator": True, "is_chat_service_account": False}
CHAT_SERVICE_ACCOUNT = {"email": "til-chat-sa@wso2.com", "name": "TIL Chat", "groups": [], "is_moderator": False, "is_chat_service_account": True}


@pytest.fixture(autouse=True)
def fresh_db():
    # Swaps the already-imported db module's live pool, rather than
    # importlib.reload(db) -- main.py's handlers call db.create_submission
    # etc. by attribute lookup on this same module object at request time,
    # so mutating .pool in place is enough for them to pick up til_test.
    bootstrap_test_database()
    db.pool = db.ConnectionPool(db.DB_HOST, db.DB_PORT, db.DB_USER, db.DB_PASSWORD, TEST_DB_NAME)
    db.init_db()
    with db.pool.get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("TRUNCATE TABLE til_submissions")
    yield


def client_as(user: dict) -> TestClient:
    main.app.dependency_overrides[require_auth] = lambda: user
    return TestClient(main.app)


def test_create_submission_uses_tokens_own_email():
    client = client_as(HUMAN_USER)
    resp = client.post("/submissions", json={"who": "Jane Doe, CSM", "where": "Internal", "what": "Learned X."})
    assert resp.status_code == 200
    assert resp.json()["submittedByEmail"] == "jane@wso2.com"


def test_create_submission_sanitizes_what_even_if_client_skips_the_editor():
    client = client_as(HUMAN_USER)
    resp = client.post(
        "/submissions",
        json={"who": "Jane", "where": "Internal", "what": '<p>hi</p><script>alert(1)</script>'},
    )
    assert resp.status_code == 200
    assert resp.json()["what"] == "<p>hi</p>"


def test_create_submission_rejects_invalid_payload():
    client = client_as(HUMAN_USER)
    resp = client.post("/submissions", json={"who": "", "where": "Internal", "what": "x"})
    assert resp.status_code == 400


def test_create_submission_requires_where_detail_for_customer():
    client = client_as(HUMAN_USER)
    resp = client.post("/submissions", json={"who": "Jane", "where": "Customer", "what": "x"})
    assert resp.status_code == 400


def test_create_submission_stores_where_detail_for_customer():
    client = client_as(HUMAN_USER)
    resp = client.post(
        "/submissions",
        json={"who": "Jane", "where": "Customer", "whereDetail": "Acme Corp", "what": "x"},
    )
    assert resp.status_code == 200
    assert resp.json()["whereDetail"] == "Acme Corp"


def test_on_behalf_of_rejected_from_a_regular_human():
    client = client_as(HUMAN_USER)
    resp = client.post(
        "/submissions",
        json={"who": "Someone", "where": "Internal", "what": "x", "onBehalfOfEmail": "other@wso2.com"},
    )
    assert resp.status_code == 403


def test_on_behalf_of_accepted_from_the_chat_service_account():
    client = client_as(CHAT_SERVICE_ACCOUNT)
    resp = client.post(
        "/submissions",
        json={"who": "Someone", "where": "Internal", "what": "x", "onBehalfOfEmail": "real-submitter@wso2.com"},
    )
    assert resp.status_code == 200
    assert resp.json()["submittedByEmail"] == "real-submitter@wso2.com"


def test_on_behalf_of_still_validated_as_an_email():
    client = client_as(CHAT_SERVICE_ACCOUNT)
    resp = client.post(
        "/submissions",
        json={"who": "Someone", "where": "Internal", "what": "x", "onBehalfOfEmail": "not-an-email"},
    )
    assert resp.status_code == 400


def test_non_owner_non_moderator_cannot_delete():
    created = client_as(HUMAN_USER).post("/submissions", json={"who": "Jane", "where": "Internal", "what": "x"}).json()
    resp = client_as(OTHER_HUMAN_USER).delete(f"/submissions/{created['id']}")
    assert resp.status_code == 403


def test_submitter_can_delete_their_own_entry():
    client = client_as(HUMAN_USER)
    created = client.post("/submissions", json={"who": "Jane", "where": "Internal", "what": "x"}).json()
    resp = client.delete(f"/submissions/{created['id']}")
    assert resp.status_code == 204


def test_moderator_can_delete_someone_elses_entry():
    created = client_as(HUMAN_USER).post("/submissions", json={"who": "Jane", "where": "Internal", "what": "x"}).json()
    resp = client_as(MODERATOR_USER).delete(f"/submissions/{created['id']}")
    assert resp.status_code == 204


def test_delete_nonexistent_is_404_for_a_moderator():
    resp = client_as(MODERATOR_USER).delete("/submissions/does-not-exist")
    assert resp.status_code == 404


def test_delete_nonexistent_is_404_even_for_a_non_moderator():
    # 404 (does this exist at all) takes precedence over 403 (are you
    # allowed) -- you can't be "unauthorized" to delete nothing.
    resp = client_as(HUMAN_USER).delete("/submissions/does-not-exist")
    assert resp.status_code == 404


def test_get_single_submission_returns_it():
    created = client_as(HUMAN_USER).post("/submissions", json={"who": "Jane", "where": "Internal", "what": "x"}).json()
    # Any signed-in employee, not just the moderator or the submitter -- same
    # "every entry, every employee" rule as the list endpoint.
    resp = client_as(MODERATOR_USER).get(f"/submissions/{created['id']}")
    assert resp.status_code == 200
    assert resp.json()["id"] == created["id"]


def test_get_single_submission_404_for_missing():
    resp = client_as(HUMAN_USER).get("/submissions/does-not-exist")
    assert resp.status_code == 404
