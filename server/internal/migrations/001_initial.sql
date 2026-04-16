CREATE TABLE uploads (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    sha256      TEXT UNIQUE NOT NULL,
    device_id   TEXT NOT NULL,
    client_id   TEXT NOT NULL,
    filename    TEXT NOT NULL,
    uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
