-- Allow Slice 2 imports to persist before classification assigns a category.
PRAGMA foreign_keys = OFF;

CREATE TABLE expenses_v2 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date DATE NOT NULL,
    description TEXT NOT NULL,
    amount REAL NOT NULL,
    balance REAL NOT NULL,
    category_id INTEGER,
    confidence_level TEXT DEFAULT 'low',
    imported_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    corrected_at TIMESTAMP,
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

INSERT INTO expenses_v2 (id, date, description, amount, balance, category_id, confidence_level, imported_at, corrected_at)
SELECT id, date, description, amount, balance, category_id, confidence_level, imported_at, corrected_at
FROM expenses;

DROP TABLE expenses;
ALTER TABLE expenses_v2 RENAME TO expenses;

CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date DESC);
CREATE INDEX IF NOT EXISTS idx_expenses_category ON expenses(category_id);
CREATE INDEX IF NOT EXISTS idx_expenses_date_category ON expenses(date DESC, category_id);
CREATE INDEX IF NOT EXISTS idx_expenses_confidence ON expenses(confidence_level);

PRAGMA foreign_keys = ON;