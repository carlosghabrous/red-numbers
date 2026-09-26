-- Initial SQLite Schema for Expense Tracking Application

-- Categories table
CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Expenses table
CREATE TABLE IF NOT EXISTS expenses (
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

-- Classification rules table
CREATE TABLE IF NOT EXISTS classification_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern TEXT NOT NULL,
    category_id INTEGER NOT NULL,
    priority INTEGER DEFAULT 0,
    confidence_weight REAL DEFAULT 1.0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Audit log table
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    action TEXT NOT NULL,
    user_id TEXT,
    expense_id INTEGER,
    old_value TEXT,
    new_value TEXT,
    details TEXT,
    FOREIGN KEY (expense_id) REFERENCES expenses(id)
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date DESC);
CREATE INDEX IF NOT EXISTS idx_expenses_category ON expenses(category_id);
CREATE INDEX IF NOT EXISTS idx_expenses_date_category ON expenses(date DESC, category_id);
CREATE INDEX IF NOT EXISTS idx_expenses_confidence ON expenses(confidence_level);
CREATE INDEX IF NOT EXISTS idx_classification_rules_priority ON classification_rules(priority ASC);
CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp DESC);
