# Expense Tracking App - Vertical Slices Implementation Guide

## Overview

This project is organized into 11 **vertical deliverable slices** instead of traditional layer-by-layer architecture. Each slice delivers complete, end-to-end functionality that you can test in the browser immediately.

## Why Vertical Slices?

✅ **Immediate Testing** - After each slice, features work and are visible in the UI  
✅ **Rapid Feedback** - You can validate each increment before moving to the next  
✅ **Incremental Value** - Each slice adds real functionality, not just infrastructure  
✅ **Easier Debugging** - Issues are isolated to recent changes  
✅ **Flexible Scope** - Easy to adjust or defer features at slice boundaries  

## The 11 Slices

### 🟢 Slice 1: CSV Upload and Raw Display (3-4 hrs)
Upload CSV → Parse with validation → Display in table  
**Test:** Upload your bank CSV → see all rows displayed

### 🟢 Slice 2: Database Storage (3-4 hrs)
Store parsed data in SQLite → Verify persistence  
**Test:** Upload CSV → refresh page → data still there

### 🟡 Slice 3: Basic Classification (4-5 hrs)
Auto-classify expenses → Show category + confidence  
**Test:** Upload CSV → each expense shows category (e.g., "supermercado")

### 🟡 Slice 4: Sorting (2-3 hrs)
Sort by date, amount, category, description  
**Test:** Click "Sort by Amount" → table reorders

### 🟠 Slice 5: Category Filtering (2-3 hrs)
Filter by one or more categories  
**Test:** Select "supermercado" → see only supermarket expenses

### 🟠 Slice 6: Date Range Filtering (3-4 hrs)
Filter by date ranges (current month, custom dates)  
**Test:** Select "current month" → see only this month's expenses

### 🔵 Slice 7: Expense Correction UI (3-4 hrs)
View expense details → Change category → Auto-update in list  
**Test:** Click expense → change category → save → verify change

### 🔵 Slice 8: Summary Statistics Widget (2-3 hrs)
Show total spending, count, average, top category  
**Test:** Dashboard shows "Total: €1,234.56"

### 🟣 Slice 9: Pie Chart Visualization (4-5 hrs)
Monthly breakdown by category as pie chart  
**Test:** Dashboard shows pie chart with 7 segments

### ⚫ Slice 10: Re-Classification & Polish (6-7 hrs)
Auto-update when corrections made + full styling + tests  
**Test:** Correct one expense → all similar ones update

### 🟣 Slice 11: Histograms (5-6 hrs)
Weekly and monthly bar charts showing spending trends  
**Test:** Dashboard shows weekly and monthly charts

## Getting Started

### Prerequisites
- Go 1.22+
- SQLite 3
- A bank export CSV file (semicolon-delimited, with fecha de operación, concepto, fecha valor, importe, and saldo columns)

### Setup
```bash
cd /Users/ghab/Code/github/red-numbers

# Install dependencies (if any)
go mod download

# Set environment variables
export DB_PATH=./expenses.db
export PORT=8080
export LOG_LEVEL=info

# Run the app
go run main.go

# Visit http://localhost:8080
```

### Current Status

**Completed:**
- ✅ Project setup (Go module, directories, main.go)
- ✅ Database schema and migrations
- ✅ Initial data models created
- ✅ Slice 1: CSV Upload and Raw Display
- ✅ Slice 2: Database Storage and Persistence
- ✅ Slice 3: Basic Classification
- ✅ Slice 4: Sorting
- ✅ Slice 5: Category Filtering
- ✅ Slice 6: Date Range Filtering
- ✅ Slice 7: Expense Correction UI
- ✅ Slice 8: Summary Statistics Widget
- ✅ Slice 9: Pie Chart Visualization
- ✅ Slice 10: Re-Classification & Polish
- ✅ Slice 11: Weekly and Monthly Histograms

**Next to implement:**
- 🔲 None — all 11 slices are implemented. Final checkpoint (task 11.12: full
  regression pass + user approval) is the only remaining item.

## What to Implement Next

All planned slices are complete. What's left is the final checkpoint task
(11.12 in `tasks.md`): a last full-suite regression run and sign-off from the
project owner. If new work is wanted beyond the original 11 slices (e.g.
bar-click filtering, CSV export, multi-user support), scope it as a new slice
following the same vertical-slice pattern used throughout this guide.

## Development Flow

For each slice:

1. **Read the tasks** in `tasks.md` for that slice
2. **Implement the code** following the task descriptions
3. **Test manually** - Upload CSV, use UI, verify results in browser
4. **Run unit tests** - Verify business logic
5. **Run integration tests** - Verify end-to-end workflows
6. **Move to next slice** - Build incrementally

## Folder Structure

```
red-numbers/
├── main.go                  # Entry point, config, HTTP server
├── go.mod, go.sum          # Go dependencies
├── handlers/               # HTTP request handlers (handlers.go, etc.)
├── services/               # Business logic (csv_import.go, classifier.go, etc.)
├── repositories/           # Data access layer (expense_repo.go, etc.)
├── models/                 # Data structures (expense.go, category.go, etc.)
├── templates/              # HTML templates (upload.html, dashboard.html, etc.)
├── static/                 # CSS, JavaScript, images
├── migrations/             # Database migration SQL files
└── .kiro/specs/            # Specification documents
    └── expense-tracking-app/
        ├── requirements.md # What to build
        ├── design.md       # How to build it
        └── tasks.md        # Implementation tasks
```

## Testing Strategy

- **Unit Tests**: Test business logic in isolation (services, repositories)
- **Integration Tests**: Test workflows end-to-end (CSV import, filtering, correction)
- **Manual Testing**: Use the web UI to verify features work

Tests marked with `*` in tasks.md are optional for MVP but recommended.

## Key Design Principles

1. **Server-Side Rendering** - Minimal JavaScript, generate HTML on server
2. **SQLite** - Single file database, no external dependencies
3. **Incremental** - Each slice adds value; can stop at any point
4. **Testable** - Each feature can be tested independently
5. **Data Integrity** - All changes logged; database transactions ensure consistency

## Questions?

Refer to the specification documents:
- `requirements.md` - What each feature should do
- `design.md` - Technical architecture and algorithms
- `tasks.md` - Specific implementation tasks

Each requirement and design section includes examples and acceptance criteria.

---

**Next Step:** Start with Slice 7 tasks in `tasks.md` and add expense correction.
