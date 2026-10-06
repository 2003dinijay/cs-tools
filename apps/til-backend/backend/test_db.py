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

"""Tests against the real local til_test MySQL database (conftest.py)."""
import os

import pytest

from conftest import TEST_DB_NAME, bootstrap_test_database


@pytest.fixture
def db_module():
    os.environ["DB_NAME"] = TEST_DB_NAME
    bootstrap_test_database()
    import importlib

    import db as db_mod

    importlib.reload(db_mod)
    db_mod.init_db()
    with db_mod.pool.get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("TRUNCATE TABLE til_submissions")
    yield db_mod


def test_create_and_get_submission(db_module):
    created = db_module.create_submission("Jane", "Internal", "Learned X", "jane@wso2.com")
    fetched = db_module.get_submission(created["id"])
    assert fetched == created


def test_where_detail_round_trips(db_module):
    created = db_module.create_submission(
        "Jane", "Customer", "Learned X", "jane@wso2.com", where_detail="Acme Corp"
    )
    assert created["whereDetail"] == "Acme Corp"
    fetched = db_module.get_submission(created["id"])
    assert fetched["whereDetail"] == "Acme Corp"
    page = db_module.list_submissions(limit=10)
    assert page["items"][0]["whereDetail"] == "Acme Corp"


def test_list_submissions_newest_first(db_module):
    db_module.create_submission("A", "Customer", "first", "a@wso2.com")
    db_module.create_submission("B", "Internal", "second", "b@wso2.com")
    page = db_module.list_submissions(limit=10)
    assert [item["who"] for item in page["items"]] == ["B", "A"]
    assert page["nextCursor"] is None


def test_list_submissions_respects_limit_and_sets_cursor(db_module):
    for i in range(3):
        db_module.create_submission(f"Person {i}", "Internal", "x", f"p{i}@wso2.com")
    page = db_module.list_submissions(limit=2)
    assert len(page["items"]) == 2
    assert page["nextCursor"] is not None


def test_delete_submission_removes_it(db_module):
    created = db_module.create_submission("Jane", "Customer", "x", "jane@wso2.com")
    assert db_module.delete_submission(created["id"]) is True
    assert db_module.get_submission(created["id"]) is None


def test_delete_nonexistent_submission_returns_false(db_module):
    assert db_module.delete_submission("does-not-exist") is False
