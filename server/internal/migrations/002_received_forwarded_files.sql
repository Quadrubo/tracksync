-- A payload from a client. completed_at is set once all derived files forward.
CREATE TABLE received_files (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id    TEXT NOT NULL,
    client_id    TEXT NOT NULL,
    sha256       TEXT NOT NULL,
    filename     TEXT NOT NULL,
    received_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    UNIQUE (device_id, sha256)
);

-- One file sent to the target; a received file can produce several. Identity is
-- (received file, filename), so a retry is deduped but distinct output files are
-- not, even with identical bytes.
CREATE TABLE forwarded_files (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    received_file_id INTEGER NOT NULL REFERENCES received_files(id) ON DELETE CASCADE,
    sha256           TEXT NOT NULL,
    filename         TEXT NOT NULL,
    forwarded_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (received_file_id, filename)
);

-- Carry over prior uploads, marked completed and seeded with the source file as
-- their single forwarded file.
INSERT INTO received_files (device_id, client_id, sha256, filename, received_at, completed_at)
SELECT device_id, client_id, sha256, filename, uploaded_at, uploaded_at FROM uploads;

INSERT INTO forwarded_files (received_file_id, sha256, filename, forwarded_at)
SELECT rf.id, u.sha256, u.filename, u.uploaded_at
FROM uploads u
JOIN received_files rf ON rf.device_id = u.device_id AND rf.sha256 = u.sha256;

DROP TABLE uploads;
