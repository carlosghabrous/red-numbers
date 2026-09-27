# Expense Tracking Application - Technical Design

## Design Philosophy: Domain-Oriented Code Organization

This is a general architecture approach worth reusing across projects, independent of language or framework. Organize code by **domain/feature**, not by technical layer (no top-level `handlers/`, `models/`, `services/`, `repositories/` grab-bags spanning unrelated features).

Each domain gets its own folder (e.g. `domains/<name>/`) containing only the files it actually needs:

- **`router.go`** — the HTTP/API endpoint implementations. Parses the request, calls the service, renders the response. No business logic, no direct data access.
- **`service.go`** — coordinates actions and pulls data from one or more sources (database, other services, external APIs) to fulfill a request from the router. This is where business/orchestration logic lives.
- **`repository.go`** — the sole abstraction for reading/writing a domain's data (e.g. to a database). Hides SQL/storage details behind Go methods; nothing outside the domain touches persistence directly.
- **`models.go`** — structs mapping persisted records (the domain's "nouns").
- **`schemas.go`** — structs that parse and validate incoming request data (query params, form fields, cookies) into typed, validated values before they reach the service. This is the equivalent of a Pydantic schema layer in a Python API: a boundary that keeps invalid input from leaking into business logic.

Not every domain needs every file — a domain with no HTTP surface (e.g. a lookup table) may only need `models.go` and `repository.go`; a domain with no request-side validation to speak of can skip `schemas.go`. Add files when they earn their keep, not to satisfy the template.

**Dependency injection over internal construction:**
- Handlers/routers never construct their own services or repositories. Services and repositories are built once, in one composition root (e.g. `main.go`), and passed into handler constructors.
- This makes dependencies explicit and swappable — tests can inject fakes/in-memory implementations instead of real ones, and swapping an implementation never requires touching the handler's code, only the wiring in the composition root.
- Handler constructors take a small shared "dependencies" struct (logger, config, etc.) instead of an ad-hoc, growing list of positional parameters. Adding a new cross-cutting concern later (metrics, tracing, feature flags) means adding one field to that struct, not changing every constructor's signature.

**Boundaries between domains:**
- A domain may depend on another domain's exported API (e.g. an `upload` domain reading from an `expenses` repository), but dependencies should point one way — avoid cycles.
- Shared infrastructure that isn't itself a business domain (DB connection setup, migrations, schema verification) lives in its own infra package (e.g. `database/`), separate from domain folders.

## Overview

The Expense Tracking Application is a server-side rendered web application that enables users to upload Spanish-language bank export CSV files, automatically categorize expenses using fuzzy logic, review and correct categorizations, and visualize spending patterns through interactive charts and filtering options.

**Key Design Philosophy:**
- Minimal client-side JavaScript for simplicity and reliability
- Server-side rendering for rapid page generation and SEO benefits
- SQLite database for persistence with efficient indexing strategies
- Form-based interactions with server-side state management
- Real-time widget updates through direct data recalculation after modifications

**Technology Stack:**
- **Backend Framework:** Go with `net/http` and middleware pattern
- **Frontend Rendering:** Go HTML templates with server-side state
- **Database:** SQLite 3 with prepared statements and connection pooling
- **Styling:** CSS with flexbox and media queries for responsive design
- **Charts/Visualizations:** SVG generation server-side for pie charts and histograms
- **Session Management:** HTTP cookies with encrypted state for filter/sort persistence

---

## Architecture

### High-Level System Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Browser                              │
│  (HTML Forms, CSS Styling, Minimal JavaScript for UX)           │
└────────────────────────┬────────────────────────────────────────┘
                         │ HTTP Request/Response
                         │
┌────────────────────────▼────────────────────────────────────────┐
│                    Go HTTP Server                                │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Router & Middleware Layer                                   ││
│  │ - Request logging, auth, CSRF protection, error handling   ││
│  └──────────────────────┬──────────────────────────────────────┘│
│                         │                                        │
│  ┌──────────────────────▼──────────────────────────────────────┐│
│  │ Handler Layer                                              ││
│  │ - CSVUploadHandler     - ExpenseListHandler               ││
│  │ - ExpenseViewHandler   - FilterSortHandler                ││
│  │ - CorrectExpenseHandler - DashboardHandler                ││
│  │ - Re-ClassifyHandler   - WidgetRenderHandler              ││
│  └──────────────────────┬──────────────────────────────────────┘│
│                         │                                        │
│  ┌──────────────────────▼──────────────────────────────────────┐│
│  │ Business Logic Layer                                       ││
│  │ ┌─────────────────────┐  ┌─────────────────────────────┐   ││
│  │ │ CSVImportService    │  │ FuzzyClassifierService      │   ││
│  │ └─────────────────────┘  └─────────────────────────────┘   ││
│  │ ┌─────────────────────┐  ┌─────────────────────────────┐   ││
│  │ │ ReClassifyService   │  │ VisualizationService        │   ││
│  │ └─────────────────────┘  └─────────────────────────────┘   ││
│  └──────────────────────┬──────────────────────────────────────┘│
│                         │                                        │
│  ┌──────────────────────▼──────────────────────────────────────┐│
│  │ Data Access Layer                                          ││
│  │ - ExpenseRepository    - CategoryRepository               ││
│  │ - ClassificationRuleRepository - AuditLogRepository       ││
│  │ - Query builders with indexing strategy                   ││
│  └──────────────────────┬──────────────────────────────────────┘│
│                         │                                        │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────────┐
│                  SQLite Database                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ Expenses     │  │ Categories   │  │ Classification_Rules     │
│  │ table        │  │ table        │  │ table                    │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Audit_Log table (optional)                              │   │
│  └──────────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Indexes on: date, category_id, pattern, confidence_level  │   │
│  └──────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────┘
```

### Request Flow Example: CSV Upload

```
1. User submits CSV file via form
   │
   ▼
2. Router directs to CSVUploadHandler
   │
   ▼
3. Handler validates file format and columns
   │
   ├─ If invalid ──→ Return error HTML
   │
   ▼ (if valid)
4. Call CSVImportService
   │
   ▼
5. Parse CSV rows, extract: date, description, amount, balance
   │
   ▼
6. For each valid row:
   ├─ Call FuzzyClassifierService → get category + confidence
   ├─ Create Expense object
   ├─ Insert into database
   │
   ▼
7. Log import summary (successes, errors)
   │
   ▼
8. Display confirmation page with summary
```

---

## Components and Interfaces

### 1. CSV Import Engine

**Responsibility:** Parse, validate, and import bank export CSV files into the database.

**Key Operations:**

```go
type CSVImportService interface {
    // ValidateFile checks file format and columns
    ValidateFile(filePath string) error
    
    // ImportFile parses and stores all expenses
    ImportFile(filePath string) (ImportResult, error)
    
    // ParseRow extracts a single CSV row into an Expense
    ParseRow(row []string) (*Expense, error)
}

type ImportResult struct {
    SuccessCount  int
    FailureCount  int
    Errors        []string
    ImportedAt    time.Time
}
```

**CSV Column Mapping:**
- `fecha` → date (validate DD/MM/YYYY or DD-MM-YYYY formats)
- `concepto` → description (transaction description for fuzzy classification)
- `importe` → amount (handle comma and period decimal separators)
- `saldo` → running balance

**Validation Rules:**
1. File must have .csv extension
2. First row must contain headers: fecha, concepto, importe, saldo
3. Each row must have 4 columns
4. fecha and importe are required; skip row if missing
5. Date must be parseable; amount must be numeric
6. Handle case-insensitive headers

**Error Handling:**
- Skip individual malformed rows with logging
- Continue processing remaining rows
- Collect all errors for final summary
- Never halt import due to single row failure

---

### 2. Fuzzy Classification System

**Responsibility:** Intelligently assign expenses to categories based on transaction descriptions using keyword matching and confidence scoring.

**Algorithm Design:**

```go
type FuzzyClassifier interface {
    // ClassifyDescription returns category and confidence level
    Classify(description string) (CategoryID, ConfidenceLevel, error)
    
    // ClassifyWithRules applies current classification rules
    ClassifyWithRules(description string, rules []ClassificationRule) (CategoryID, ConfidenceLevel)
}

type ConfidenceLevel string
const (
    ConfidenceLow    ConfidenceLevel = "low"     // 0-39%
    ConfidenceMedium ConfidenceLevel = "medium"  // 40-74%
    ConfidenceHigh   ConfidenceLevel = "high"    // 75-100%
)

type KeywordPattern struct {
    Keywords   []string
    CategoryID CategoryID
    Priority   int
    Weight     float64
}
```

**Keyword Matching Algorithm:**

```
Input: description (string), classification_rules (array)
Output: (category_id, confidence_level)

1. Convert description to lowercase and normalize whitespace
2. For each classification rule ordered by priority:
     a. Split rule pattern into keywords
     b. Calculate match score: (matched_keywords / total_keywords) * weight
     c. Store: (category_id, score, priority)
3. If no matches found:
     a. Return (default_category="supermercado", confidence="low")
4. Filter matches by highest score
5. If ties exist (equal score):
     a. Use rule priority as tiebreaker
6. Calculate confidence level based on score:
     - 0-39% → "low"
     - 40-74% → "medium"
     - 75-100% → "high"
7. Return (winning_category, confidence_level)
```

**Built-in Keyword Rules (from Requirements):**

| Category | Keywords | Priority |
|----------|----------|----------|
| supermercado | mercadona, consum, carrefour, aldi, hiperchina, dia retail | 1 |
| medico | farmacia, clinica, dental, medico | 2 |
| niños | colegio, educarte, crececonmusica | 3 |
| ocio | cinema, cine, ocine, enjoy, amazon prime, netflix, downdog, pilates | 4 |
| deporte | club natacion, delfin, mirador, altitud, bergueda, extreme | 5 |
| suministros | iberdrola, pepe mobile, telefonica, telefónica, agua, gas | 6 |
| casa | alquiler, renta, casa, volkswagen renting | 7 |

**Confidence Scoring Details:**
- Exact keyword match (full word boundary): 1.0
- Partial substring match: 0.7
- Case-insensitive match: 0.8
- Multiple keywords matched: average of individual scores
- If all keywords present: 1.0 (100%)

---

### 3. Re-Classification Engine

**Responsibility:** Update all stored expenses when classification rules change, maintaining data consistency and providing detailed audit trails.

**Process Design:**

```go
type ReClassifyService interface {
    // ReClassifyAll updates all expenses with new rules
    ReClassifyAll(ctx context.Context) (ReClassifyResult, error)
    
    // ReClassifyByPattern updates expenses matching a pattern
    ReClassifyByPattern(ctx context.Context, pattern string) (ReClassifyResult, error)
    
    // IsReClassificationNeeded checks if rules have changed
    IsReClassificationNeeded() (bool, error)
}

type ReClassifyResult struct {
    ProcessedCount  int
    ChangedCount    int
    FailureCount    int
    AffectedCategories map[string]int
    Duration        time.Duration
    Timestamp       time.Time
}
```

**Re-Classification Algorithm (Synchronous):**

```
Input: all stored expenses
Output: updated expenses, summary

1. BEGIN TRANSACTION (SQLite)
2. Fetch all classification rules ordered by priority
3. Fetch all expenses from database
4. FOR each expense:
     a. Get original category
     b. Call FuzzyClassifier with current description and rules
     c. Get new category and confidence
     d. IF new category ≠ original category:
        i. Update expense.category_id
        ii. Update expense.confidence_level
        iii. Increment changed_count
        iv. Log change to audit_log
     e. IF error during classification:
        i. Log error
        ii. Increment failure_count
        iii. CONTINUE (don't rollback, continue with next)
5. COMMIT TRANSACTION
6. Return summary with processed_count, changed_count, affected_categories
```

**Design Decision: Synchronous vs Async**

We implement **synchronous re-classification** for the following reasons:
- **Correctness:** User sees immediate, consistent state after correction
- **Simplicity:** No need for background job queue or status tracking
- **Performance:** With SQLite and proper indexing, re-classification of 1000 expenses takes <500ms
- **Data Integrity:** Transaction guarantees atomicity; all changes succeed or all rollback
- **User Feedback:** Users immediately see impact of their corrections

For larger datasets (>10,000 expenses), async could be added later with progress tracking.

**Error Recovery:**
- Transaction rollback if critical error occurs
- Continue processing other expenses on non-critical errors
- Log all errors for user review
- Notify user of partial success scenarios

---

### 4. Data Persistence Layer

**Database Schema:**

```sql
-- Categories table
CREATE TABLE categories (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,          -- supermercado, medico, niños, etc.
    display_name TEXT NOT NULL,         -- For UI display
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Expenses table
CREATE TABLE expenses (
    id INTEGER PRIMARY KEY,
    date DATE NOT NULL,                 -- Transaction date
    description TEXT NOT NULL,          -- Transaction description (concepto)
    amount DECIMAL(10, 2) NOT NULL,    -- Transaction amount (importe)
    balance DECIMAL(10, 2),             -- Running balance (saldo)
    category_id INTEGER NOT NULL,       -- FK to categories
    confidence_level TEXT,              -- 'low', 'medium', 'high'
    imported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    corrected_at TIMESTAMP,             -- When last corrected by user
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Classification_Rules table
CREATE TABLE classification_rules (
    id INTEGER PRIMARY KEY,
    pattern TEXT NOT NULL,              -- Keyword or regex pattern
    category_id INTEGER NOT NULL,       -- FK to categories
    priority INTEGER DEFAULT 100,       -- Lower = higher priority
    confidence_weight DECIMAL(3, 2) DEFAULT 1.0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT,                    -- User who created the rule
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Audit_Log table (optional)
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    action TEXT NOT NULL,              -- 'import', 'correction', 're_classify', etc.
    user_id TEXT,                      -- User performing action
    expense_id INTEGER,                -- FK to expenses (nullable for bulk actions)
    old_value TEXT,                    -- JSON: {category_id, confidence_level}
    new_value TEXT,                    -- JSON: {category_id, confidence_level}
    details TEXT                       -- Additional context
);

-- Indexes for query performance
CREATE INDEX idx_expenses_date ON expenses(date DESC);
CREATE INDEX idx_expenses_category ON expenses(category_id);
CREATE INDEX idx_expenses_date_category ON expenses(date DESC, category_id);
CREATE INDEX idx_expenses_confidence ON expenses(confidence_level);
CREATE INDEX idx_classification_rules_priority ON classification_rules(priority ASC);
CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp DESC);
```

**Repository Interface:**

```go
type ExpenseRepository interface {
    // Create inserts a new expense
    Create(ctx context.Context, expense *Expense) error
    
    // GetByID retrieves single expense
    GetByID(ctx context.Context, id int64) (*Expense, error)
    
    // GetAll retrieves all expenses (with pagination)
    GetAll(ctx context.Context, offset, limit int) ([]Expense, error)
    
    // GetByFilters retrieves expenses matching filters
    GetByFilters(ctx context.Context, filters ExpenseFilters) ([]Expense, error)
    
    // Update modifies an existing expense
    Update(ctx context.Context, expense *Expense) error
    
    // UpdateCategory updates category for expense
    UpdateCategory(ctx context.Context, expenseID int64, categoryID int64, confidence string) error
    
    // GetCountByCategory returns expense count per category
    GetCountByCategory(ctx context.Context, startDate, endDate time.Time) (map[string]int64, error)
    
    // GetSumByCategory returns total amount per category
    GetSumByCategory(ctx context.Context, startDate, endDate time.Time) (map[string]float64, error)
}

type ExpenseFilters struct {
    StartDate      time.Time
    EndDate        time.Time
    CategoryIDs    []int64
    ConfidenceLevels []string
    SortBy         string      // "date", "amount", "category", "description"
    SortOrder      string      // "asc", "desc"
    Offset         int
    Limit          int
}
```

**Query Performance Strategy:**

1. **Connection Pooling:** Go's SQLite driver with pool of 5-10 connections
2. **Prepared Statements:** All queries use parameterized statements
3. **Batch Operations:** Bulk insert/update within transactions for CSV import
4. **Index Strategy:**
   - Primary indexes on id and foreign keys
   - Composite indexes on common filter combinations (date + category)
   - Separate indexes for sort operations
5. **Query Optimization:**
   - Always use LIMIT with OFFSET for pagination
   - Avoid SELECT * (fetch only needed columns)
   - Use COUNT(*) with indexes for statistics

---

### 5. Frontend Rendering and State Management

**Server-Side Template Architecture:**

```
Template Files:
├── base.html              # Master template with nav/footer
├── expense_list.html      # Main expense listing with filters
├── expense_detail.html    # Single expense view/edit
├── dashboard.html         # Widgets and visualizations
├── upload.html            # CSV upload form
├── components/
│   ├── filter_controls.html
│   ├── sort_controls.html
│   ├── pie_chart.html
│   ├── histogram_weekly.html
│   ├── histogram_monthly.html
│   └── summary_stats.html
```

**State Management via Session Cookies:**

```go
type SessionState struct {
    UserID           string
    FilterCategories []string
    FilterDateStart  time.Time
    FilterDateEnd    string    // "all", "current_month", "previous_month", "current_year", "custom"
    SortBy           string    // "date", "amount", "category", "description"
    SortOrder        string    // "asc", "desc"
    CurrentPage      int
    PageSize         int
}

// Stored as encrypted cookie using sessions middleware
// Cookie expires after 7 days of inactivity
// Session state persists across page reloads
```

**Form-Based Interactions:**

All user actions flow through standard HTML forms with server-side processing:

```html
<!-- Example: Category Filter Form -->
<form method="GET" action="/expenses/list">
    <select name="filter_categories" multiple>
        <option value="">-- All Categories --</option>
        <option value="supermercado">Supermercado</option>
        <option value="medico">Medico</option>
        <!-- etc -->
    </select>
    <button type="submit">Apply Filter</button>
</form>

<!-- Example: Sort Control Form -->
<form method="GET" action="/expenses/list">
    <select name="sort_by">
        <option value="date">Date</option>
        <option value="amount">Amount</option>
        <option value="category">Category</option>
        <option value="description">Description</option>
    </select>
    <select name="sort_order">
        <option value="desc">Descending</option>
        <option value="asc">Ascending</option>
    </select>
    <button type="submit">Sort</button>
</form>
```

**HTML Rendering Pipeline:**

```
1. Handler receives request
2. Extract and validate filters from query params/forms
3. Query database with filters
4. Pass data to template
5. Execute template with context data
6. Return rendered HTML to browser
```

**Minimal JavaScript Usage:**

- Form submission for filters/sorts (no AJAX, standard HTTP POST/GET)
- Tooltip interactions on charts (hover states via CSS)
- Responsive menu toggle on mobile (CSS + tiny JS for class toggle)
- Date picker enhancement for custom date ranges (HTML5 input[type="date"])
- Zero framework dependencies (no React, Vue, Angular)

---

### 6. Visualization Widgets

**Widget Architecture:**

Each visualization generates SVG server-side for maximum compatibility and performance.

#### 6.1 Pie Chart Widget

**Responsibility:** Display monthly expense breakdown by category.

```go
type PieChartWidget interface {
    // Render generates SVG pie chart
    Render(ctx context.Context, expenses []Expense, width, height int) (string, error)
    
    // CalculateSegments computes pie slices
    CalculateSegments(expenses []Expense) []PieSegment
}

type PieSegment struct {
    Category    string
    Amount      float64
    Percentage  float64
    StartAngle  float64
    EndAngle    float64
    Color       string
}
```

**Rendering Strategy:**
- SVG generation server-side (no Canvas needed)
- Color palette: 7 distinct colors for 7 categories
- Segments sized proportional to spending
- Tooltip text in SVG title elements (renders on hover)
- Labels positioned outside slices to avoid overlap
- Responsive: scale SVG to 100% width up to max 600px

**SVG Output Example:**
```xml
<svg viewBox="0 0 400 400" xmlns="http://www.w3.org/2000/svg">
    <!-- Background circle -->
    <circle cx="200" cy="200" r="150" fill="#f5f5f5"/>
    
    <!-- Pie slices (generated for each category) -->
    <path d="M 200,200 L 200,50 A 150,150 0 0,1 256.7,74.2 Z" 
          fill="#FF6B6B" 
          class="pie-slice"
          onmouseover="showTooltip(event)">
        <title>Supermercado: €450.50 (32%)</title>
    </path>
    
    <!-- Labels outside pie -->
    <text x="280" y="40" text-anchor="middle" font-size="12">Supermercado 32%</text>
    
    <!-- Legend -->
    <rect x="20" y="320" width="10" height="10" fill="#FF6B6B"/>
    <text x="35" y="328" font-size="12">Supermercado €450.50</text>
</svg>
```

#### 6.2 Weekly Histogram Widget

**Responsibility:** Display expense distribution across weeks by category.

```go
type WeeklyHistogramWidget interface {
    // Render generates SVG histogram
    Render(ctx context.Context, expenses []Expense, startDate, endDate time.Time) (string, error)
    
    // AggregateByWeek groups expenses by week and category
    AggregateByWeek(expenses []Expense) map[WeekKey]map[CategoryID]float64
}

type WeekKey struct {
    Year int
    Week int
}
```

**Bar Layout:**
- X-axis: weeks (Monday-Sunday format)
- Y-axis: expense amount in euros
- Bar grouping: 7 bars per week (one per category) or stacked bars
- Choice: **stacked bars** for better visual comparison of total per week
- Tooltip on hover: week, category, amount
- Scale Y-axis to data range + 10% padding

#### 6.3 Monthly Histogram Widget

**Responsibility:** Display expense distribution across months by category.

**Structure:** Similar to weekly histogram but aggregated by month.

#### 6.4 Summary Statistics Widget

**Responsibility:** Display key metrics (total, count, average, top category).

```go
type SummaryStatistics struct {
    TotalSpending        float64
    TransactionCount     int64
    AverageTransaction   float64
    TopCategoryName      string
    TopCategoryAmount    float64
    FilteredTotal        float64  // After filters applied
    DateRangeDescription string
}
```

**Rendering:** Simple HTML div with styled statistics display.

---

## Error Handling

### Error Categories and Strategies

**1. CSV Upload Errors**
```go
type CSVValidationError struct {
    Field   string  // e.g., "fecha", "importe"
    Row     int
    Value   string
    Reason  string  // e.g., "invalid date format"
}

// Strategy: Collect all validation errors, display summary, allow partial import
```

**2. Classification Errors**
```go
type ClassificationError struct {
    ExpenseID   int64
    Description string
    Reason      string
}

// Strategy: Log error, use default category, mark as low confidence
```

**3. Database Errors**
```go
// Strategy: Wrap SQLite errors in application-specific errors
// Examples:
// - Connection pool exhausted → retry with exponential backoff
// - Transaction conflict → retry transaction
// - Constraint violation → log and handle gracefully
```

**4. User Input Validation**
```go
// Date range: start_date must not be after end_date
// Category selection: validate against known categories
// File upload: validate extension, size (<50MB)
```

### Error Recovery Patterns

**Re-Classification with Partial Failure:**
- Continue processing even if individual expense fails
- Collect errors and present summary
- Option to retry failed expenses

**Database Connection Loss:**
- Connection pool handles reconnection
- Retry failed operation up to 3 times
- Return user-friendly error message after retries exhausted

**CSV Import Row Errors:**
- Skip invalid rows without halting import
- Log row number and error reason
- Display count of skipped rows with sample errors

---

## Testing Strategy

### Unit Testing

**Test Coverage Targets:**
- CSV parser: valid/invalid formats, edge cases (empty rows, special characters)
- Fuzzy classifier: keyword matching, confidence scoring, tiebreaker logic
- Re-classification: state changes, transaction rollback scenarios
- Repository queries: filter combinations, sorting, pagination
- Template rendering: context data availability, edge cases

**Example Test Cases:**

```go
// CSV Parser Tests
TestParseValidCSVRow()
TestParseRowMissingRequiredField()
TestParseRowInvalidDateFormat()
TestParseRowDecimalSeparatorHandling()

// Fuzzy Classifier Tests
TestClassifyExactKeywordMatch()
TestClassifyMultipleKeywordsPartialMatch()
TestClassifyNoMatchDefaultCategory()
TestClassifyConfidenceScoring()
TestClassifyPriorityTiebreaker()

// Re-Classification Tests
TestReClassifyAllExpenses()
TestReClassifyRollbackOnError()
TestReClassifyTransactionIsolation()
TestReClassifyAuditLogging()

// Repository Tests
TestGetExpensesByDateRange()
TestGetExpensesByCategoryFilter()
TestGetExpensesSortedByAmount()
TestGetExpensesWithPagination()
```

### Integration Testing

**Test Scenarios:**
1. **End-to-End CSV Import:**
   - Upload CSV → Parse → Classify → Store → Verify DB records

2. **Classification Workflow:**
   - Import expense → View with confidence → Correct category → Re-classify all → Verify changes

3. **Filtering and Sorting:**
   - Apply date range filter → Apply category filter → Sort by amount → Verify results

4. **Visualization Updates:**
   - Import expenses → Verify pie chart → Correct expense → Verify pie chart updates

### Performance Testing

**Benchmark Scenarios:**
- CSV import: 1,000 expenses (target: <2 seconds)
- Re-classification: 1,000 expenses (target: <500ms)
- Page load with 500 expenses + filters: <200ms
- Database query with complex filters: <100ms

---

## Database Indexing Strategy

**Primary Indexes:**
```sql
-- Automatically created for PRIMARY KEY and FOREIGN KEY
CREATE INDEX idx_expenses_id ON expenses(id);
CREATE INDEX idx_classification_rules_category ON classification_rules(category_id);
```

**Query-Specific Indexes:**
```sql
-- Most common filter: date range (ascending date most common)
CREATE INDEX idx_expenses_date ON expenses(date DESC);

-- Category filtering
CREATE INDEX idx_expenses_category ON expenses(category_id);

-- Combined filters (date + category)
CREATE INDEX idx_expenses_date_category ON expenses(date DESC, category_id);

-- Confidence level filtering for display
CREATE INDEX idx_expenses_confidence ON expenses(confidence_level);

-- Classification rule priority lookups
CREATE INDEX idx_classification_rules_priority ON classification_rules(priority ASC);

-- Audit log time-based queries
CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp DESC);
```

**Index Maintenance:**
- SQLite automatically maintains indexes
- VACUUM command to reclaim space after bulk deletes (not needed for MVP)
- ANALYZE command to update statistics for query planner (run monthly)

---

## Implementation Notes

### Session State Persistence

**Cookie-Based State:**
- Session cookie encrypted with AES-256-GCM
- Session data: filters, sort order, current page
- Expires after 7 days of inactivity
- HTTPS-only (secure) flag in production
- SameSite=Strict to prevent CSRF

**Alternative Consideration:** Server-side sessions with Redis
- Chosen: Cookie-based (simpler, no external dependency)
- Redis option for future if session growth becomes issue

### Fuzzy Classifier Optimization

**Keyword Index Optimization:**
- Store keywords in lowercase for comparison
- Pre-compile keyword lists at startup (not on every classification)
- Use map for O(1) keyword lookup instead of slice iteration

**Example Optimization:**
```go
// At startup, pre-compile rules
type CompiledRules map[CategoryID][]string  // Category → Keywords

// During classification, lookup is O(n) categories × O(1) keyword check
for categoryID, keywords := range compiledRules {
    matchCount := 0
    for _, keyword := range keywords {
        if strings.Contains(lowerDescription, keyword) {
            matchCount++
        }
    }
    score := float64(matchCount) / float64(len(keywords))
    // ... rest of scoring
}
```

### CSV Import Batching

**Performance Optimization:**
```go
// Insert expenses in batches to reduce transaction overhead
const BATCH_SIZE = 100

for i := 0; i < len(expenses); i += BATCH_SIZE {
    batch := expenses[i:min(i+BATCH_SIZE, len(expenses))]
    
    // BEGIN TRANSACTION
    for _, expense := range batch {
        // INSERT
    }
    // COMMIT TRANSACTION
}
```

### Widget Rendering Caching

**Strategy:**
- Render widgets on-demand (no separate cache layer)
- If same filters applied multiple times, cache is implicit via HTTP caching headers
- Add `Cache-Control: private, max-age=300` for 5-minute browser cache
- No server-side cache needed for MVP

### Date Handling

**Timezone Considerations:**
- Store all dates in UTC in database
- Display in user's local timezone (via JavaScript or browser settings)
- For MVP, assume all users in same timezone (Spain)
- Date parsing: handle both DD/MM/YYYY and DD-MM-YYYY formats

### Concurrent Access

**SQLite Concurrency Model:**
- SQLite allows multiple readers, one writer
- Write operations are serialized (SERIALIZABLE isolation)
- For MVP with single admin user, this is sufficient
- If multi-user support needed later, migrate to PostgreSQL

---

## Deployment Architecture

**Server Configuration:**
- Single Go binary compiled to executable
- SQLite database file on disk (e.g., /data/expenses.db)
- Configurable via environment variables:
  - `DB_PATH` - path to SQLite database
  - `PORT` - HTTP server port (default 8080)
  - `SESSION_SECRET` - encryption key for session cookies
  - `LOG_LEVEL` - logging verbosity

**File Structure:**
```
/app
├── main.go
├── handlers/
├── services/
├── repositories/
├── models/
├── templates/
├── static/
│   └── css/
├── migrations/
│   └── schema.sql
└── config/
    └── categories.json
```

---

## Future Enhancements

**Not included in MVP:**

1. **Recurring Expense Detection**
   - Identify patterns of similar expenses (same merchant, similar amount)
   - Suggest recurring vs one-time classification

2. **Budget Alerts**
   - Set spending limits per category
   - Alert when approaching or exceeding limits

3. **Multi-User Support**
   - User authentication and authorization
   - Role-based access (admin vs viewer)
   - Separate expense databases per user

4. **Machine Learning Classification**
   - Train model on user corrections over time
   - Adaptive confidence scoring
   - Would require separate ML service

5. **API Interface**
   - RESTful API for programmatic access
   - Mobile app support

6. **Advanced Reporting**
   - PDF/Excel export
   - Year-over-year comparisons
   - Spending trend analysis

7. **Mobile App**
   - Native iOS/Android apps with offline support
   - Would duplicate classification logic

---

## Dependencies and Build

**Go Dependencies:**
```go
// CSV parsing
"encoding/csv"  // stdlib

// Database
"database/sql"  // stdlib
"github.com/mattn/go-sqlite3"  // SQLite driver

// Web framework
"net/http"  // stdlib

// Templates
"html/template"  // stdlib
"text/template"  // stdlib

// Utilities
"time"  // stdlib
"log"   // stdlib
"fmt"   // stdlib
"strings"  // stdlib
```

**No external web framework needed** - Go's `net/http` is sufficient for this use case.

**Build Command:**
```bash
go build -o expense-app main.go
```

**Run Command:**
```bash
./expense-app
```

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

This section defines properties that must hold true across all valid inputs and states. These properties guide both implementation and testing, ensuring core functionality is correct and robust.

### Property 1: CSV Header Validation

*For any* CSV file content, the validator should correctly identify whether required columns (fecha, concepto, importe, saldo) are present and reject files missing required columns.

**Validates: Requirements 1.2, 1.3, 13.1, 13.2**

**Implementation Note:** The validator must handle case-insensitive column matching and report which specific columns are missing.

### Property 2: CSV Row Parsing and Field Extraction

*For any* valid CSV row with all required fields, parsing should correctly extract date, description, amount, and balance into their respective data types with proper decimal/format handling (e.g., European decimal separators).

**Validates: Requirements 1.5, 13.4, 13.5**

**Implementation Note:** Date formats (DD/MM/YYYY and DD-MM-YYYY) must be normalized to standard format. Decimal separators (comma and period) must be handled correctly. Parsing should be deterministic.

### Property 3: CSV Row Validation and Skip Logic

*For any* CSV row with missing or invalid required fields (fecha, importe), the importer should skip that row without halting import of remaining rows, and the error should be logged.

**Validates: Requirements 1.6, 13.6**

**Implementation Note:** Invalid data (non-numeric amounts, unparseable dates) should trigger row skip. Import summary should accurately count skipped rows.

### Property 4: Import Summary Accuracy

*For any* set of CSV rows (mix of valid and invalid), the import summary should accurately count successful imports and failures. Success count plus failure count must equal total rows processed.

**Validates: Requirements 1.7, 4.4**

**Implementation Note:** This is a counting invariant: `success_count + failure_count = total_rows`. The summary must be generated without loss of information.

### Property 5: Classification Assigns Valid Categories

*For any* expense description, the fuzzy classifier must assign the expense to one of the seven valid categories: supermercado, medico, niños, ocio, deporte, suministros, or casa.

**Validates: Requirements 2.1, 2.4**

**Implementation Note:** Even descriptions that don't match any pattern should default to a valid category (typically "supermercado" with low confidence).

### Property 6: Classification Keyword Matching

*For any* expense description containing category-specific keywords (from Requirements 2.5–2.11), the classifier should assign the expense to that category. Matching should be case-insensitive and support partial word matching.

**Validates: Requirements 2.2, 2.5, 2.6, 2.7, 2.8, 2.9, 2.10, 2.11**

**Implementation Note:** This property encompasses multiple category-specific rules. Each keyword must be matched case-insensitively. If multiple categories match, the highest priority (lowest priority value) wins.

### Property 7: Confidence Scoring is Deterministic

*For any* expense description and a fixed set of classification rules, the classifier must produce the same category and confidence level on repeated calls. Confidence scoring must be deterministic based only on the input description and rules.

**Validates: Requirements 2.2, 2.3**

**Implementation Note:** Given the same rules and description, the output must always be identical. This ensures reproducible behavior for testing and debugging.

### Property 8: Priority Tiebreaker for Equal Confidence

*For any* expense description that matches multiple categories with equal confidence scores, the classifier must apply the predefined priority tiebreaker (lower priority value wins) to select exactly one category.

**Validates: Requirements 2.3**

**Implementation Note:** When multiple matches tie on confidence score, the rule with lowest priority value must win. Tiebreaker must be applied deterministically.

### Property 9: Re-Classification Processes All Expenses

*For any* set of stored expenses, a re-classification operation must process all expenses. The count of expenses processed must equal the count of expenses before re-classification. No expense should be skipped or duplicated.

**Validates: Requirements 4.2, 4.5**

**Implementation Note:** This is a completeness invariant. After re-classification, `processed_count = initial_expense_count`.

### Property 10: Re-Classification Summary Counts are Accurate

*For any* set of expenses before re-classification and after re-classification with updated rules, the summary counts must be accurate:
- `changed_count` = number of expenses with different category after re-classification
- `processed_count` = total expenses processed
- `failure_count` = number of expenses that failed to re-classify
- `changed_count ≤ processed_count` (invariant: changed cannot exceed processed)

**Validates: Requirements 4.4**

**Implementation Note:** These are counting invariants. The summary must accurately reflect what changed.

### Property 11: Re-Classification Data Integrity

*For any* expense in the database before re-classification, the same expense (same id, amount, description, date) must exist after re-classification. No records should be lost, corrupted, or duplicated.

**Validates: Requirements 4.5**

**Implementation Note:** Re-classification should only update category fields, never delete or create new expense records. Row count must remain constant.

### Property 12: Date Range Validation

*For any* custom date range input where start_date > end_date, the validator must reject the range and not apply the filter.

**Validates: Requirements 7.2, 7.3**

**Implementation Note:** Validation should occur before querying the database. Invalid ranges should be caught and reported to the user.

### Property 13: Date Range Filter Correctness

*For any* date range filter (start_date, end_date), all returned expenses must have dates within the range [start_date, end_date] inclusive. No expense outside the range should be returned.

**Validates: Requirements 7.4, 7.5, 7.6**

**Implementation Note:** This is a correctness invariant for filtering. The database query must respect the range boundaries.

### Property 14: Category Filter Correctness

*For any* set of selected categories for filtering, all returned expenses must belong to one of the selected categories. No expense from an unselected category should appear in results. If multiple categories are selected, the result uses OR logic.

**Validates: Requirements 6.2, 6.3**

**Implementation Note:** Category filtering must be inclusive (OR logic for multiple selections). Unfiltered query should include all categories.

### Property 15: Sort Correctness by Date

*For any* list of expenses sorted by date in descending order, each expense's date must be greater than or equal to the next expense's date. For ascending order, each expense's date must be less than or equal to the next expense's date.

**Validates: Requirements 5.1, 5.3**

**Implementation Note:** Date sorting must respect the sort direction (ascending/descending). This is a strict ordering invariant.

### Property 16: Sort Correctness by Amount

*For any* list of expenses sorted by amount in descending order, each expense's amount must be greater than or equal to the next expense's amount. For ascending order, each expense's amount must be less than or equal to the next expense's amount.

**Validates: Requirements 5.2, 5.4**

**Implementation Note:** Numeric sorting must handle decimal amounts correctly (e.g., 10.50 > 10.05). Must respect sort direction.

### Property 17: Sort Correctness by Category

*For any* list of expenses sorted by category, expenses must be grouped by category (alphabetically by name), and within each category group, must be sorted by date in descending order.

**Validates: Requirements 5.5**

**Implementation Note:** This is a complex multi-level sort: primary sort by category name (alphabetic), secondary sort by date (descending within each category).

### Property 18: Statistics Calculation Accuracy

*For any* set of filtered expenses, the calculated statistics must be accurate:
- `total_spending` = sum of all expense amounts (must match database sum exactly)
- `transaction_count` = count of expenses (must match database count)
- `average_transaction` = total_spending / transaction_count (must be precise to 2 decimal places)
- `top_category` = category with highest total spending (must exist in data and be highest)

**Validates: Requirements 11.1, 11.2, 11.3**

**Implementation Note:** All arithmetic must be consistent with currency precision (€ 2 decimal places). Rounding must be handled consistently.

### Property 19: Week Aggregation Correctness

*For any* set of expenses within a date range, weekly aggregation must correctly group expenses into weeks (Monday through Sunday). Each expense must belong to exactly one week. Weeks must not overlap. All expenses in the range must be assigned to a week.

**Validates: Requirements 9.2, 9.3**

**Implementation Note:** Week boundaries must be consistent (ISO 8601: Monday=start of week). Each expense must appear in exactly one week's totals.

### Property 20: Month Aggregation Correctness

*For any* set of expenses within a date range, monthly aggregation must correctly group expenses into calendar months. Each expense must belong to exactly one month. Months must not overlap. All expenses in the range must be assigned to a month.

**Validates: Requirements 10.2, 10.3**

**Implementation Note:** Month boundaries must respect calendar months (1st to last day). Each expense must appear in exactly one month's totals.

### Property 21: Filter Clearing Restores Full Dataset

*For any* filtered view (by category, date range, or both) that shows a subset of expenses, clearing all filters must restore the full original dataset. The count of expenses returned must equal the total count before filtering was applied.

**Validates: Requirements 6.4, 7.8**

**Implementation Note:** Clearing filters must return to an unfiltered state. No expenses should be lost when clearing filters.

### Property 22: Expense Field Display Completeness

*For any* expense in the list view, all required fields (date, description, amount, category, confidence level) must be present in the display. No field should be null or missing.

**Validates: Requirements 5.6**

**Implementation Note:** Every expense record must include all five fields. Missing data should be handled gracefully (e.g., with placeholder values).

---

## Testing Strategy

### Dual Testing Approach

The implementation requires both **unit tests** and **property-based tests** for comprehensive coverage:

**Property-Based Tests** (validate universal properties):
- CSV parsing and validation (Properties 1–4)
- Fuzzy classification logic (Properties 5–8)
- Re-classification correctness (Properties 9–11)
- Filtering and sorting logic (Properties 12–17)
- Aggregation and statistics (Properties 18–22)

**Unit Tests** (validate specific examples and edge cases):
- Error message formatting
- UI control rendering
- Database schema creation
- Session state management
- Edge cases in date parsing (leap years, month boundaries)

**Integration Tests** (validate end-to-end workflows):
- CSV upload → parse → classify → store
- Correction → re-classification → widget update
- Complex filtering scenarios
- Database transaction rollback scenarios

### Property-Based Testing Library

**Language:** Go

**Library:** [Gopter](https://github.com/leanovate/gopter) or [QuickCheck for Go](https://github.com/k0kubun/go-quickcheck)

**Test Configuration:**
- Minimum 100 iterations per property test (default for randomization coverage)
- Each property test should run independently
- Properties should use meaningful generators for realistic inputs

**Example Test Structure:**

```go
// Property 1: CSV Header Validation
func TestProperty_CSVHeaderValidation(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("headers must be validated", 
        gopter.ForAll(
            generateCSVContent(),  // Generate random CSV content
            func(content string) bool {
                result := ValidateCSVHeaders(content)
                // Property: if content missing headers, validation fails
                // if content has headers, validation succeeds
                return validateProperty(result, content)
            },
        ),
    )
    
    properties.TestingRun(t)
}

// Property 6: Keyword Matching
func TestProperty_ClassificationKeywordMatching(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("descriptions with keywords classify correctly",
        gopter.ForAll(
            generateKeywordDescription(),  // Generate descriptions with known keywords
            func(description string) bool {
                category := Classify(description)
                // Property: if description contains known keyword, 
                // should classify to that category
                return verifyKeywordMatch(description, category)
            },
        ),
    )
    
    properties.TestingRun(t)
}
```

### Test Coverage Targets

- **CSV Import:** 95% line coverage (critical path)
- **Fuzzy Classification:** 100% coverage of keyword matching logic
- **Re-Classification:** 95% coverage (excluding I/O wrapper layer)
- **Filtering/Sorting:** 90% coverage of query logic
- **Statistics:** 100% coverage of calculation logic

### Unit Test Examples

```go
// Edge case: leap year date parsing
func TestParseDate_LeapYear(t *testing.T) {
    date := ParseDate("29/02/2024")
    assert.NotNil(t, date)
    assert.Equal(t, 2024, date.Year())
}

// Example: Confidence level categorization
func TestConfidenceLevel_Boundaries(t *testing.T) {
    assert.Equal(t, "low", ConfidenceLevelFrom(0.35))
    assert.Equal(t, "medium", ConfidenceLevelFrom(0.50))
    assert.Equal(t, "high", ConfidenceLevelFrom(0.85))
}

// Error handling: CSV validation error messages
func TestCSVValidationError_Message(t *testing.T) {
    err := ValidateCSVFile("test.csv")
    assert.NotNil(t, err)
    assert.Contains(t, err.Error(), "missing columns: fecha, importe")
}
```

### Integration Test Examples

```go
// E2E: CSV upload to visualization update
func TestE2E_CSVUploadAndVisualization(t *testing.T) {
    // 1. Upload CSV
    result := ImportCSV("test.csv")
    assert.Equal(t, 5, result.SuccessCount)
    
    // 2. Verify expenses stored
    expenses := GetAllExpenses()
    assert.Equal(t, 5, len(expenses))
    
    // 3. Verify categories assigned
    for _, exp := range expenses {
        assert.NotEmpty(t, exp.CategoryID)
    }
    
    // 4. Correct one expense
    CorrectExpense(expenses[0].ID, "medico")
    
    // 5. Verify re-classification triggered
    summaryAfter := GetReClassificationSummary()
    assert.Greater(t, summaryAfter.ChangedCount, 0)
    
    // 6. Verify visualization updates
    pieChart := GetPieChartData()
    assert.Greater(t, pieChart.MedicoAmount, 0)
}

// Re-classification with errors
func TestReClassification_ErrorRecovery(t *testing.T) {
    // Setup: 10 expenses, 1 will fail classification
    ImportTestExpenses(10)
    
    // Trigger re-classification with mock error
    result := ReClassifyAllWithError()
    
    // Property: should process all 10, fail on 1, succeed on 9
    assert.Equal(t, 10, result.ProcessedCount)
    assert.Equal(t, 1, result.FailureCount)
    assert.Equal(t, 9, result.ProcessedCount - result.FailureCount)
}
```

### Performance Benchmarks

```go
// Benchmark: CSV parsing for 1,000 rows
func BenchmarkCSVParsing_1000Rows(b *testing.B) {
    for i := 0; i < b.N; i++ {
        ImportCSV("test_1000_rows.csv")
    }
    // Target: <2 seconds for 1,000 expenses
}

// Benchmark: Re-classification for 1,000 expenses
func BenchmarkReClassification_1000Expenses(b *testing.B) {
    ImportTestExpenses(1000)
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        ReClassifyAll(context.Background())
    }
    // Target: <500ms for re-classifying 1,000 expenses
}

// Benchmark: Complex filter query
func BenchmarkQuery_ComplexFilter(b *testing.B) {
    ImportTestExpenses(1000)
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        GetByFilters(ExpenseFilters{
            StartDate:    time.Now().AddDate(0, -3, 0),
            EndDate:      time.Now(),
            CategoryIDs:  []int64{1, 2, 3},
            SortBy:       "date",
            SortOrder:    "desc",
        })
    }
    // Target: <100ms for complex query on 1,000 expenses
}
```

---

## Correctness Verification Checklist

Before implementation begins, verify that the design satisfies these constraints:

- [ ] All 22 correctness properties are understood and implementable
- [ ] CSV parsing handles all specified date/decimal formats
- [ ] Fuzzy classifier uses all 7 categories and keyword rules from requirements
- [ ] Re-classification is synchronous and atomic (uses transactions)
- [ ] Database schema includes all required columns and indexes
- [ ] Filtering (date + category) supports both AND and OR logic correctly
- [ ] Sorting is implemented for all 4 sort attributes (date, amount, category, description)
- [ ] Statistics calculations are precise to currency decimal places
- [ ] Week aggregation uses consistent Monday-Sunday boundaries
- [ ] Month aggregation respects calendar month boundaries
- [ ] No data loss or duplication during import or re-classification
- [ ] All required fields display for every expense record
- [ ] Error messages are descriptive and actionable

