CREATE TABLE uploaded (
    sha256      TEXT UNIQUE NOT NULL,
    filename    TEXT NOT NULL,
    device_id   TEXT NOT NULL,
    uploaded_at DATETIME NOT NULL
);
