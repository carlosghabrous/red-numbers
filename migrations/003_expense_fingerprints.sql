ALTER TABLE expenses ADD COLUMN fingerprint TEXT;
CREATE INDEX IF NOT EXISTS idx_expenses_fingerprint ON expenses(fingerprint);