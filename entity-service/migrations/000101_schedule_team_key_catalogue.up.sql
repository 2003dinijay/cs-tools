-- team_key had no catalogue anywhere in the database: a free VARCHAR(64) on
-- both rota fact tables, with no lookup, no foreign key and no check. A typo'd
-- or renamed key returned an empty rota rather than an error -- the same drift
-- risk that turned schedule_absence_kind from an enum into a table in 000090,
-- except here there was not even a row to add.
--
-- The catalogue already exists: it is the team table. What was missing is a
-- stable key to join on. Teams are referred to by a lowercase slug throughout
-- this feature (the CSM_TEAM_REGISTRY entries, the seed, the importer), while
-- team.name carries the display form, so the slug becomes a column of its own
-- rather than an expression nothing can key off.
ALTER TABLE team ADD COLUMN IF NOT EXISTS key VARCHAR(64);

UPDATE team SET key = lower(name) WHERE key IS NULL;

-- Two teams whose names differ only by case would collide here. None do; this
-- fails loudly at migration time rather than silently later if that changes.
ALTER TABLE team ALTER COLUMN key SET NOT NULL;
ALTER TABLE team ADD CONSTRAINT team_key_unique UNIQUE (key);

-- Lets the composite foreign key below reference (id, key) as a pair.
ALTER TABLE team ADD CONSTRAINT team_id_key_unique UNIQUE (id, key);

-- ── The fact tables now point at a real row ───────────────────────────────
ALTER TABLE schedule_assignment
    ADD CONSTRAINT schedule_assignment_team_key_fkey
    FOREIGN KEY (team_key) REFERENCES team (key) ON UPDATE CASCADE;

ALTER TABLE schedule_absence
    ADD CONSTRAINT schedule_absence_team_key_fkey
    FOREIGN KEY (team_key) REFERENCES team (key) ON UPDATE CASCADE;

-- ── and team_id can no longer disagree with team_key ──────────────────────
--
-- schedule_assignment carries both: team_id for a direct join, team_key for
-- the registry-only teams that have no row of their own. Nothing tied them
-- together, so a row could name one team by id and a different one by key.
--
-- MATCH SIMPLE is what is wanted rather than a gap: when team_id is null the
-- composite constraint stands down, which is exactly the registry-only case,
-- and team_key is still checked on its own by the foreign key above.
ALTER TABLE schedule_assignment
    ADD CONSTRAINT schedule_assignment_team_agrees
    FOREIGN KEY (team_id, team_key) REFERENCES team (id, key) ON UPDATE CASCADE;

-- schedule_absence deliberately carries no team_id. It is written and read by
-- key alone -- no query joins an absence to a team row -- and a second way to
-- say the same thing is a second thing to keep in step. The key is now
-- constrained, which is what the id was providing here.
COMMENT ON COLUMN schedule_absence.team_key IS
    'The team this absence belongs to, by catalogue key. No team_id column on '
    'purpose: nothing joins an absence to a team row, and the foreign key on '
    'this column is what integrity the id would have added.';
