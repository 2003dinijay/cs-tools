"""MySQL access for til-backend.

Mirrors the PAR Legacy Migration Tool's own db.py (operations/peoplehr-
data-migration/par/backend/db.py in digiops-hr) -- same connection-pool
shape, same fail-loud posture: init_db() only ever VERIFIES the
til_submissions table already exists (SHOW TABLES LIKE), it never creates
anything itself. An operator runs sql/create_til_tables.sql once against
the target database before this app is ever started there -- see that
file for the exact command.
"""
from __future__ import annotations

import os
import threading
import uuid
from contextlib import contextmanager
from datetime import datetime, timezone

import pymysql
import pymysql.cursors

DB_HOST = os.environ.get("DB_HOST", "localhost")
DB_PORT = int(os.environ.get("DB_PORT", "3306"))
DB_USER = os.environ.get("DB_USER", "root")
DB_PASSWORD = os.environ.get("DB_PASSWORD")
if DB_PASSWORD is None:
    # No default -- this connects to a real database in any real
    # deployment, so a missing value must fail loudly rather than silently
    # connecting as root with an empty/guessable password.
    raise RuntimeError("DB_PASSWORD is required and has no default. Set it in the environment.")
DB_NAME = os.environ.get("DB_NAME", "til")

POOL_SIZE = 10


class ConnectionPool:
    """Simple thread-safe connection pool for one database. Checks a
    connection out of a list, pings it (reconnecting if the server dropped
    it), and returns it to the pool when done."""

    def __init__(self, host, port, user, password, database):
        self.host = host
        self.port = port
        self.user = user
        self.password = password
        self.database = database
        self._lock = threading.Lock()
        self._connections = []

    def _new_connection(self):
        return pymysql.connect(
            host=self.host,
            port=self.port,
            user=self.user,
            password=self.password,
            database=self.database,
            charset="utf8mb4",
            cursorclass=pymysql.cursors.DictCursor,
            autocommit=True,
            connect_timeout=10,
            read_timeout=30,
            write_timeout=30,
        )

    @contextmanager
    def get_conn(self):
        with self._lock:
            conn = self._connections.pop() if self._connections else None
        if conn is None:
            conn = self._new_connection()
        else:
            try:
                conn.ping(reconnect=True)
            except Exception:
                conn = self._new_connection()
        try:
            yield conn
        finally:
            with self._lock:
                if len(self._connections) < POOL_SIZE:
                    self._connections.append(conn)
                else:
                    conn.close()


pool = ConnectionPool(DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME)


def init_db():
    """Verifies til_submissions already exists -- does NOT create it. See
    sql/create_til_tables.sql and the module docstring above."""
    with pool.get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("SHOW TABLES LIKE %s", ("til_submissions",))
            if cur.fetchone() is None:
                raise RuntimeError(
                    "Required table `til_submissions` does not exist in database "
                    f"`{DB_NAME}`. Run sql/create_til_tables.sql against this "
                    "database before starting this app."
                )


def create_submission(
    who: str, where: str, what: str, submitted_by_email: str, where_detail: str | None = None
) -> dict:
    submission_id = str(uuid.uuid4())
    created_at = datetime.now(timezone.utc).isoformat()
    with pool.get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(
                "INSERT INTO til_submissions "
                "(id, who, where_, where_detail, what, submitted_by_email, created_at) "
                "VALUES (%s, %s, %s, %s, %s, %s, %s)",
                (submission_id, who, where, where_detail, what, submitted_by_email, created_at),
            )
    return {
        "id": submission_id,
        "who": who,
        "where": where,
        "whereDetail": where_detail,
        "what": what,
        "submittedByEmail": submitted_by_email,
        "createdAt": created_at,
    }


def _row_to_dict(row: dict) -> dict:
    return {
        "id": row["id"],
        "who": row["who"],
        "where": row["where_"],
        "whereDetail": row["where_detail"],
        "what": row["what"],
        "submittedByEmail": row["submitted_by_email"],
        "createdAt": row["created_at"],
    }


def list_submissions(limit: int = 100, cursor: str | None = None) -> dict:
    # Cursor = "<created_at>|<id>" of the last row the caller already has;
    # keyset pagination, newest first. created_at ALONE used to be the whole
    # cursor, but created_at is microsecond-precision text, not a guaranteed-
    # unique key -- two rows landing in the same microsecond would tie, and
    # a strict `created_at < %s` silently drops whichever of them falls on
    # the far side of that boundary. id (the primary key) breaks the tie.
    with pool.get_conn() as conn:
        with conn.cursor() as cur:
            if cursor:
                cursor_created_at, _, cursor_id = cursor.rpartition("|")
                cur.execute(
                    "SELECT * FROM til_submissions "
                    "WHERE (created_at, id) < (%s, %s) "
                    "ORDER BY created_at DESC, id DESC LIMIT %s",
                    (cursor_created_at, cursor_id, limit + 1),
                )
            else:
                cur.execute(
                    "SELECT * FROM til_submissions ORDER BY created_at DESC, id DESC LIMIT %s",
                    (limit + 1,),
                )
            rows = cur.fetchall()

    has_more = len(rows) > limit
    rows = rows[:limit]
    items = [_row_to_dict(r) for r in rows]
    next_cursor = f'{items[-1]["createdAt"]}|{items[-1]["id"]}' if has_more and items else None
    return {"items": items, "nextCursor": next_cursor}


def get_submission(submission_id: str) -> dict | None:
    with pool.get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT * FROM til_submissions WHERE id = %s", (submission_id,))
            row = cur.fetchone()
    if row is None:
        return None
    return _row_to_dict(row)


def delete_submission(submission_id: str) -> bool:
    with pool.get_conn() as conn:
        with conn.cursor() as cur:
            cur.execute("DELETE FROM til_submissions WHERE id = %s", (submission_id,))
            return cur.rowcount > 0
