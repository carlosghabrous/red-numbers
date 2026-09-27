# Implementation Plan: Expense Tracking Application

## Overview

This document organizes the expense tracking application implementation into 11 vertical slices, each independently testable and building incrementally. Each slice delivers a complete, usable feature that users can test in the browser.

---

## Slice 1: CSV Upload and Raw Display (Minimum Viable)

**Goal:** Enable users to upload a CSV file and see parsed data in a table on the browser.

**Success Criteria:**
- User can upload a CSV file via HTTP form
- System validates required columns (fecha de operación, concepto, fecha valor, importe, saldo)
- Parsed expenses display in HTML table with date, description, amount
- Clear error message if columns are missing
- Estimated effort: 3-4 hours

### Tasks

- [x] 1.1 Create CSV upload HTTP endpoint and form
  - Create GET /upload route to serve upload.html form
  - Create POST /upload route to accept multipart/form-data file uploads
  - Validate file extension (.csv)
  - _Requirements: 1.0, 14.0_

- [x] 1.2 Implement CSV parser with header validation
  - Parse CSV headers from first row
  - Validate required columns: fecha de operación, concepto, fecha valor, importe, saldo
  - Return descriptive error if columns missing
  - Create in-memory Expense objects from valid rows
  - _Requirements: 1.0, 13.0_

- [x] 1.3 Create upload response template and raw data display
  - Create template to render parsed expenses in HTML table
  - Display columns: date, description, amount, balance
  - Show import summary (success count, error count)
  - _Requirements: 1.0, 14.0_

- [x] 1.4 Add basic styling and form controls
  - Create upload form HTML with file input and submit button
  - Add minimal CSS for table readability
  - Show error messages in red
  - _Requirements: 14.0_

- [x] 1.5 Write unit tests for CSV parsing
  - Test valid CSV with all required columns
  - Test missing required columns
  - Test various date formats (DD/MM/YYYY, DD-MM-YYYY)
  - Test decimal separator handling (comma and period)
  - _Requirements: 1.0, 13.0_

- [x] 1.6 Write integration test for upload endpoint
  - Upload valid CSV file via POST
  - Verify HTTP 200 response with parsed data
  - Verify error response for invalid CSV
  - _Requirements: 1.0_

---

## Slice 2: Database Storage and Persistence

**Goal:** Store parsed CSV expenses in SQLite and verify data persists across app restarts.

**Success Criteria:**
- Expenses stored in SQLite database after upload
- Data survives application restart
- Dashboard loads expenses from database
- Estimated effort: 3-4 hours

### Tasks

- [x] 2.1 Create database schema and migrations
  - Create Expenses table: id, date, description, amount, saldo, category_id, confidence_level, imported_at
  - Create Categories table: id, name, display_name
  - Create Classification_Rules table: id, pattern, category_id, created_at, updated_at, priority
  - Add database indexes on date, category_id, description
  - _Requirements: 12.0_

- [x] 2.2 Implement ExpenseRepository with CRUD methods
  - Create ExpenseRepository.Create(expense) method
  - Create ExpenseRepository.GetAll() method
  - Implement transaction handling for batch inserts
  - _Requirements: 1.0, 12.0_

- [x] 2.3 Integrate database storage into CSV upload flow
  - After successful CSV parsing, store expenses to database
  - Update upload response to show "X expenses saved to database"
  - Handle database errors gracefully with user-friendly messages
  - _Requirements: 1.0, 12.0_

- [x] 2.4 Create dashboard page that loads expenses from database
  - Create GET / route to serve dashboard
  - Query all expenses from database
  - Render expenses table from stored data
  - _Requirements: 1.0, 12.0_

- [x] 2.5 Add database initialization to app startup
  - Auto-create schema if not exists
  - Verify database integrity on startup
  - Log migration status
  - _Requirements: 12.0_

- [x] 2.6 Write unit tests for ExpenseRepository
  - Test Create and GetAll methods
  - Test transaction rollback on error
  - Test index performance with 1000+ records
  - _Requirements: 12.0_

- [x] 2.7 Write integration test for persistence
  - Upload CSV and insert expenses
  - Verify data in database
  - Restart app and verify data still present
  - _Requirements: 1.0, 12.0_

---

## Slice 3: Basic Classification

**Goal:** Automatically assign expenses to categories using fuzzy keyword matching.

**Success Criteria:**
- Each uploaded expense assigned to a category
- Confidence level displayed (high, medium, low)
- Classification decisions logged
- Estimated effort: 4-5 hours

### Tasks

- [x] 3.1 Define classification keyword patterns
  - Create keyword mapping for all 7 categories
  - supermercado: "mercadona", "consum", "carrefour", "aldi", etc.
  - medico: "farmacia", "clinica", "dental", etc.
  - niños: "colegio", "educarte", "crececonmusica", etc.
  - ocio: "cinema", "cine", "netflix", "amazon prime", etc.
  - deporte: "club natacion", "delfin", "altitud", etc.
  - suministros: "iberdrola", "telefonica", "agua", "gas", etc.
  - casa: "alquiler", "renta", "volkswagen renting", etc.
  - _Requirements: 2.0_

- [x] 3.2 Implement FuzzyClassifierService with keyword matching
  - Create case-insensitive substring matching algorithm
  - Assign confidence based on match strength (exact, partial, keyword)
  - Implement priority resolution when multiple patterns match
  - _Requirements: 2.0_

- [x] 3.3 Integrate classifier into CSV import flow
  - Call FuzzyClassifierService for each parsed expense
  - Store category_id and confidence_level in database
  - Log classification decisions with description, pattern matched, confidence
  - _Requirements: 2.0_

- [x] 3.4 Update dashboard to display classification results
  - Add category column to expense table
  - Display confidence level (high, medium, low) as visual indicator or text
  - Sort by confidence to highlight uncertain classifications
  - _Requirements: 2.0, 14.0_

- [x] 3.5 Create classification logging system
  - Log each classification decision with timestamp, description, category, confidence
  - Store logs in database or file for debugging
  - Provide log viewer endpoint for admin
  - _Requirements: 2.0_

- [x] 3.6 Write unit tests for FuzzyClassifierService
  - Test exact keyword matches
  - Test partial matches with confidence scoring
  - Test priority resolution for conflicting patterns
  - Test case-insensitive matching
  - Test default category assignment for no matches
  - _Requirements: 2.0_

- [x] 3.7 Write integration test for classification on import
  - Upload CSV with mixed expense descriptions
  - Verify each expense assigned to correct category
  - Verify confidence levels assigned
  - _Requirements: 2.0_

---

## Slice 4: Sorting

**Goal:** Enable users to sort expenses by date, amount, category, or description.

**Success Criteria:**
- Sort controls visible on dashboard
- Sort preference persists across page refreshes (cookie)
- All four sort options work correctly
- Estimated effort: 2-3 hours

### Tasks

- [x] 4.1 Add sort UI controls to expense table
  - Create sort dropdown/buttons (Date, Amount, Category, Description)
  - Create sort direction toggle (ascending/descending)
  - Add visual indicator showing current sort
  - _Requirements: 5.0, 14.0_

- [x] 4.2 Implement ExpenseRepository.GetByFilters with sorting
  - Add sortBy parameter (date, amount, category, description)
  - Add sortDirection parameter (asc, desc)
  - Implement sorting logic at repository level (SQL ORDER BY)
  - _Requirements: 5.0_

- [x] 4.3 Update dashboard route to handle sort parameters
  - Accept sort parameters from query string or form
  - Pass to repository.GetByFilters()
  - Render table with sorted results
  - _Requirements: 5.0, 14.0_

- [x] 4.4 Implement sort preference persistence via cookie
  - Store selected sort field and direction in HTTP cookie
  - Load stored preference on page load
  - Update cookie when user changes sort
  - _Requirements: 5.0_

- [x] 4.5 Add sort indicators and visual feedback
  - Show arrow or icon indicating sort direction
  - Highlight current sort column in table header
  - Provide clear visual feedback on sort change
  - _Requirements: 5.0, 14.0_

- [x] 4.6 Write unit tests for sorting logic
  - Test sort by date ascending/descending
  - Test sort by amount ascending/descending
  - Test sort by category with grouping
  - _Requirements: 5.0_

- [x] 4.7 Write integration test for sort persistence
  - Load dashboard with sort preference cookie
  - Verify correct sort applied on load
  - Change sort, verify page reloads with new sort
  - _Requirements: 5.0_

---

## Slice 5: Category Filtering

**Goal:** Allow users to filter expenses by one or more categories.

**Success Criteria:**
- Category filter checkboxes visible on dashboard
- Selecting categories filters table correctly
- Multiple categories use OR logic
- Sort and display update with filtered data
- Estimated effort: 2-3 hours

### Tasks

- [x] 5.1 Add category filter UI controls
  - Create checkbox list for all 7 categories
  - Display category name and icon/color
  - Add "Select All" / "Clear All" convenience buttons
  - _Requirements: 6.0, 14.0_

- [x] 5.2 Extend ExpenseRepository.GetByFilters with category filtering
  - Add categoryIds parameter (array/list)
  - Implement SQL WHERE clause for category matching (IN operator)
  - Support multiple categories with OR logic
  - _Requirements: 6.0_

- [x] 5.3 Update dashboard to apply category filters
  - Accept category filter from form submission or query string
  - Pass to repository.GetByFilters()
  - Combine with existing sort
  - Update table with filtered results
  - _Requirements: 6.0_

- [x] 5.4 Show active filter indication
  - Display "Filters Active: X categories selected"
  - Show selected categories clearly
  - Provide easy "Clear Filters" button
  - _Requirements: 6.0, 14.0_

- [x] 5.5 Ensure visual continuity with sort
  - Sorting works correctly with category filter applied
  - Filter and sort preferences both persist (cookie)
  - Display both sort and filter state clearly
  - _Requirements: 5.0, 6.0_

- [x] 5.6 Write unit tests for category filtering
  - Test single category filter
  - Test multiple category filters (OR logic)
  - Test filter combined with sort
  - Test empty result set handling
  - _Requirements: 6.0_

- [x] 5.7 Write integration test for filtering workflow
  - Apply category filter, verify table updates
  - Combine filter with sort, verify both applied
  - Clear filter, verify all expenses show again
  - _Requirements: 6.0_

---

## Slice 6: Date Range Filtering

**Goal:** Allow users to filter expenses by date ranges (all, current month, custom dates).

**Success Criteria:**
- Date range filter controls visible on dashboard
- Supports all/current month/previous month/custom date ranges
- Validates date input (start ≤ end)
- Works correctly combined with category filter and sort
- Estimated effort: 3-4 hours

### Tasks

- [x] 6.1 Add date range filter UI controls
  - Create radio buttons: "All Dates", "Current Month", "Previous Month"
  - Add custom date range inputs (start date, end date picker)
  - Add visual indicator showing active date filter
  - _Requirements: 7.0, 14.0_

- [x] 6.2 Implement date range validation
  - Validate that start_date ≤ end_date
  - Show error message for invalid ranges
  - Disable "Apply" button for invalid input
  - _Requirements: 7.0_

- [x] 6.3 Extend ExpenseRepository.GetByFilters with date filtering
  - Add startDate and endDate parameters
  - Implement SQL WHERE clause for date range (BETWEEN)
  - Support "current month" calculated from current date
  - Support "previous month" calculated from current date
  - _Requirements: 7.0_

- [x] 6.4 Update dashboard to apply date range filters
  - Accept date filter from form submission or query string
  - Pass to repository.GetByFilters()
  - Combine with existing category filter and sort
  - Update table with filtered results
  - _Requirements: 7.0_

- [x] 6.5 Display active date filter state
  - Show "Date Filter: Current Month" or date range
  - Display start and end dates clearly
  - Provide "Clear Date Filter" button
  - _Requirements: 7.0, 14.0_

- [x] 6.6 Ensure filter combination works smoothly
  - Date filter + category filter works correctly
  - Date filter + sort works correctly
  - Date filter + category filter + sort all work together
  - All preferences persist via cookie
  - _Requirements: 5.0, 6.0, 7.0_

- [x] 6.7 Write unit tests for date range filtering
  - Test current month filter
  - Test previous month filter
  - Test custom date range validation
  - Test date range with invalid input
  - Test combined filters
  - _Requirements: 7.0_

- [x] 6.8 Write integration test for date filtering workflow
  - Apply date range, verify table updates
  - Combine date filter with category and sort, verify all applied
  - Test edge cases (first/last day of month, invalid dates)
  - _Requirements: 7.0_

---

## Slice 7: Expense Correction UI

**Goal:** Enable users to view expense details and correct assigned categories.

**Success Criteria:**
- Clicking expense in list opens detail page
- Detail page shows category dropdown
- User can select new category and save
- Category updated in database and list
- Estimated effort: 3-4 hours

### Tasks

- [x] 7.1 Create expense detail page route and template
  - Create GET /expenses/:id route
  - Query expense from database by ID
  - Render detail page with all expense information
  - _Requirements: 3.0, 14.0_

- [x] 7.2 Add category correction UI to detail page
  - Display current category with label
  - Create dropdown with all 7 categories
  - Show current confidence level
  - Add "Save Changes" button
  - _Requirements: 3.0, 14.0_

- [x] 7.3 Create POST handler to update expense category
  - Accept category ID from form submission
  - Validate category exists and is valid
  - Update expense in database
  - _Requirements: 3.0, 12.0_

- [x] 7.4 Add confirmation and redirect after update
  - Show "Category updated successfully" message
  - Redirect to expense list after save
  - Preserve active filters and sort in redirect
  - _Requirements: 3.0, 14.0_

- [x] 7.5 Display expense in list as clickable link
  - Make expense row clickable to open detail page
  - Add visual indicator that row is clickable (cursor change)
  - _Requirements: 3.0, 14.0_

- [x] 7.6 Add navigation back to list from detail page
  - Show "Back to List" link on detail page
  - Preserve scroll position if possible
  - _Requirements: 3.0, 14.0_

- [x] 7.7 Write unit tests for expense detail and update
  - Test loading expense by ID
  - Test updating expense category
  - Test invalid category rejection
  - _Requirements: 3.0_

- [x] 7.8 Write integration test for correction workflow
  - Navigate to expense detail
  - Change category
  - Verify update in database
  - Verify change visible in list
  - _Requirements: 3.0_

---

## Slice 8: Summary Statistics Widget

**Goal:** Display total spending, count, average, and top category statistics.

**Success Criteria:**
- Dashboard shows summary stats widget
- Statistics update when filters applied
- Currency formatted correctly (euros, 2 decimals)
- Estimated effort: 2-3 hours

### Tasks

- [x] 8.1 Implement SummaryStatistics calculation service
  - Create method to calculate: total, count, average, top_category
  - Accept expense list as input
  - Format currency as euros (€X,XXX.XX)
  - _Requirements: 11.0_

- [x] 8.2 Add statistics retrieval to repository
  - Create ExpenseRepository.GetStatistics(filters) method
  - Calculate totals grouped by category
  - Sort by amount descending to find top category
  - _Requirements: 11.0, 12.0_

- [x] 8.3 Create summary statistics UI widget
  - Design widget layout showing 4 main stats
  - Create HTML template for stats display
  - Style with clear typography and spacing
  - _Requirements: 11.0, 14.0_

- [x] 8.4 Integrate statistics into dashboard
  - Call repository.GetStatistics() on page load
  - Pass current filters to statistics calculation
  - Render statistics widget above expense table
  - _Requirements: 11.0_

- [x] 8.5 Update statistics when filters change
  - Re-calculate statistics when filters applied
  - Update widget without full page reload if possible
  - Show "no data" message if filtered results empty
  - _Requirements: 11.0, 6.0, 7.0_

- [x] 8.6 Write unit tests for statistics calculation
  - Test total calculation with various amounts
  - Test average calculation
  - Test top category identification
  - Test currency formatting
  - _Requirements: 11.0_

- [x] 8.7 Write integration test for statistics widget
  - Load dashboard with no filters, verify stats correct
  - Apply category filter, verify stats update
  - Apply date range filter, verify stats update
  - _Requirements: 11.0, 6.0, 7.0_

---

## Slice 9: Pie Chart Widget

**Goal:** Display pie chart showing expense breakdown by category.

**Success Criteria:**
- Dashboard shows pie chart visualization
- Chart segments labeled with category, amount, percentage
- Tooltip shows on hover with details
- Responsive on mobile/tablet/desktop
- Estimated effort: 4-5 hours

### Tasks

- [x] 9.1 Create pie chart data preparation service
  - Create method to calculate category totals and percentages
  - Sort categories by amount descending
  - Handle edge cases (0% categories, rounding)
  - _Requirements: 8.0_

- [x] 9.2 Implement SVG pie chart rendering
  - Create SVG pie chart generator using Go/server-side rendering
  - Calculate SVG path data for pie slices
  - Assign colors to categories consistently
  - _Requirements: 8.0, 14.0_

- [x] 9.3 Add labels and legends to pie chart
  - Display category names and percentages on chart
  - Create legend showing category colors
  - Position labels to avoid overlap
  - _Requirements: 8.0, 14.0_

- [x] 9.4 Implement tooltip functionality
  - Add hover tooltips showing category, amount, percentage
  - Use JavaScript for tooltip positioning
  - Update tooltip position on mouse move
  - _Requirements: 8.0, 14.0_

- [x] 9.5 Integrate pie chart into dashboard
  - Add pie chart section to dashboard template
  - Call data preparation service on page load
  - Render pie chart SVG into page
  - Position chart responsively
  - _Requirements: 8.0, 14.0_

- [x] 9.6 Make pie chart responsive for mobile
  - Reduce chart size on screens < 768px width
  - Stack labels vertically if needed
  - Ensure all elements readable on mobile
  - _Requirements: 8.0, 15.0_

- [x] 9.7 Update pie chart when filters change
  - Re-calculate chart data when category/date filter applied
  - Update chart display with new data
  - Handle case where no expenses match filter
  - _Requirements: 8.0, 6.0, 7.0_

- [x] 9.8 Write unit tests for pie chart generation
  - Test data preparation with various category totals
  - Test SVG path generation for various slice sizes
  - Test color assignment consistency
  - Test percentage calculations
  - _Requirements: 8.0_

- [x] 9.9 Write integration test for pie chart rendering
  - Load dashboard, verify pie chart displays
  - Apply filter, verify chart updates
  - Test responsiveness on different screen sizes
  - _Requirements: 8.0, 15.0_

---

## Slice 10: Weekly and Monthly Histograms

**Goal:** Display bar charts showing spending trends by week and month.

**Success Criteria:**
- Dashboard shows weekly histogram (7-day periods)
- Dashboard shows monthly histogram (calendar months)
- Charts stack or group by category
- Charts responsive on all screen sizes
- Estimated effort: 5-6 hours

### Tasks

- [~] 10.1 Create histogram data preparation service
  - Create method to aggregate expenses by week (Mon-Sun)
  - Create method to aggregate expenses by month
  - Calculate totals per category per period
  - Sort periods chronologically
  - _Requirements: 9.0, 10.0_

- [~] 10.2 Implement weekly histogram rendering
  - Create SVG bar chart generator for weekly data
  - Calculate bar dimensions and positions
  - Stack or group bars by category
  - Assign consistent colors to categories
  - _Requirements: 9.0, 14.0_

- [~] 10.3 Implement monthly histogram rendering
  - Create SVG bar chart generator for monthly data
  - Calculate bar dimensions and positions
  - Stack or group bars by category
  - Use same color scheme as weekly chart
  - _Requirements: 10.0, 14.0_

- [~] 10.4 Add axes and labels to histograms
  - Display x-axis with week/month labels
  - Display y-axis with amount scale
  - Add axis labels and gridlines if needed
  - Format y-axis scale appropriately
  - _Requirements: 9.0, 10.0, 14.0_

- [~] 10.5 Implement tooltips for histogram bars
  - Show week/month, category, and amount on hover
  - Position tooltip near cursor
  - Update position on mouse move
  - _Requirements: 9.0, 10.0, 14.0_

- [~] 10.6 Integrate histograms into dashboard
  - Add weekly histogram section to dashboard
  - Add monthly histogram section to dashboard
  - Call data preparation services on page load
  - Render charts into page with proper spacing
  - _Requirements: 9.0, 10.0, 14.0_

- [~] 10.7 Make histograms responsive for mobile
  - Reduce chart size on screens < 768px
  - Stack charts vertically on mobile
  - Reduce bar width to prevent overflow
  - Ensure all labels readable
  - _Requirements: 9.0, 10.0, 15.0_

- [~] 10.8 Update histograms when filters change
  - Re-calculate chart data when filters applied
  - Update chart display with new data
  - Handle case where no expenses in time range
  - _Requirements: 9.0, 10.0, 6.0, 7.0_

- [ ]* 10.9 Write unit tests for histogram data aggregation
  - Test weekly aggregation with various dates
  - Test monthly aggregation with various dates
  - Test category subtotals per period
  - Test edge cases (week boundaries, year boundaries)
  - _Requirements: 9.0, 10.0_

- [ ]* 10.10 Write unit tests for histogram rendering
  - Test SVG generation for various data shapes
  - Test bar positioning and sizing
  - Test axis label formatting
  - _Requirements: 9.0, 10.0_

- [ ]* 10.11 Write integration test for histogram display
  - Load dashboard, verify weekly and monthly charts visible
  - Apply date range filter, verify charts update
  - Apply category filter, verify bars update
  - Test responsiveness on mobile/tablet/desktop
  - _Requirements: 9.0, 10.0, 15.0_

---

## Slice 11: Re-Classification and Polish

**Goal:** Auto-update expenses when corrections made; add error handling, logging, and styling.

**Success Criteria:**
- Correcting one expense auto-updates similar expenses
- Dashboard fully styled and responsive
- Error handling and logging throughout app
- Comprehensive test coverage
- Production-ready README and deployment guide
- Estimated effort: 6-7 hours

### Tasks

- [~] 11.1 Implement ReClassifyService for automatic updates
  - Create service to re-classify all expenses after correction
  - Use description pattern matching to find similar expenses
  - Update category for all matching expenses
  - _Requirements: 4.0, 3.0_

- [~] 11.2 Integrate re-classification into correction flow
  - After expense category update, trigger re-classification
  - Show "X similar expenses re-classified" message
  - Update all affected expenses in database
  - Maintain data integrity
  - _Requirements: 4.0, 3.0_

- [~] 11.3 Add re-classification summary and logging
  - Create re-classification log entry with: count, categories affected, timestamp
  - Display re-classification summary to user
  - Store history for audit purposes
  - _Requirements: 4.0, 3.0_

- [~] 11.4 Refresh dashboard after re-classification
  - Reload table data after re-classification completes
  - Update all statistics and charts
  - Show "Dashboard updated" confirmation
  - _Requirements: 4.0_

- [~] 11.5 Create comprehensive CSS stylesheet
  - Design responsive layout for all screen sizes
  - Create mobile-first CSS with media queries
  - Style forms, tables, buttons, and charts
  - Ensure accessibility (contrast, font sizes)
  - _Requirements: 14.0, 15.0_

- [~] 11.6 Add global error handling and middleware
  - Create error handler for all routes
  - Add request logging middleware
  - Implement CSRF protection for forms
  - Add security headers (HSTS, XSS protection, etc.)
  - _Requirements: 14.0_

- [~] 11.7 Add server-side validation and user feedback
  - Validate all form inputs server-side
  - Display validation errors near form fields
  - Add success/error flash messages
  - _Requirements: 14.0_

- [~] 11.8 Create comprehensive unit test suite
  - Test all service classes (Classifier, Statistics, ReClassify)
  - Test all repository methods
  - Test error conditions and edge cases
  - Aim for 80%+ code coverage
  - _Requirements: 1.0-15.0_

- [~] 11.9 Create comprehensive integration test suite
  - Test full CSV import flow end-to-end
  - Test complete filtering and sorting workflows
  - Test correction and re-classification workflow
  - Test dashboard with all widgets
  - _Requirements: 1.0-15.0_

- [~] 11.10 Write README with setup and usage instructions
  - Document prerequisites (Go version, SQLite, dependencies)
  - Document build and run instructions
  - Provide example CSV file format
  - Document configuration options
  - _Requirements: 14.0_

- [~] 11.11 Write deployment and production guide
  - Document database migration strategy
  - Provide deployment checklist
  - Document scaling considerations
  - Document backup and recovery procedures
  - _Requirements: 12.0, 14.0_

- [~] 11.12 Final testing and polish
  - Test all features on desktop/tablet/mobile
  - Verify all error cases handled gracefully
  - Check accessibility with screen reader
  - Performance testing (load times, database queries)
  - _Requirements: 14.0, 15.0_

- [ ]* 11.13 Write property-based tests for core logic
  - Property test: For any expense, round-trip (classify → view → update) preserves data
  - Property test: For any filter combination, statistics are consistent with displayed data
  - Property test: Re-classification maintains data integrity
  - _Requirements: 1.0, 2.0, 4.0_

- [~] 11.14 Checkpoint - Final verification
  - Ensure all tests pass
  - Verify all slices working together
  - Test complete workflow from CSV to dashboard
  - Ask user for feedback and approval
  - _Requirements: 1.0-15.0_

---

## Implementation Notes

### Database Persistence
- SQLite database file stored in project directory
- Auto-migrated on app startup
- Indexes created on frequently queried columns (date, category_id)

### Cookie-Based Preferences
- Sort preferences stored in HTTP cookie (max 10 years)
- Date/category filters NOT persisted (user preference)
- Cookie-based state is optional and gracefully degraded if cookies disabled

### Error Handling Strategy
- All errors caught and logged with context
- User-friendly error messages displayed (no stack traces)
- Failed imports show summary: "50 expenses imported, 3 skipped due to errors"

### Testing Requirements
- Unit tests focus on business logic (services, repository)
- Integration tests focus on workflows and UI responses
- All tests marked with `*` are optional for MVP but recommended

### Performance Considerations
- Database queries optimized with indexes
- Re-classification may be slow for 10,000+ expenses (background task in future)
- CSV parsing limited to reasonable file size (< 10MB)

### Security
- All form inputs validated server-side
- CSRF tokens on all POST forms
- SQL injection prevented through prepared statements
- No sensitive data logged or displayed

---

## Task Dependency Graph

```json
{
  "waves": [
    {
      "id": 0,
      "tasks": [
        "1.1",
        "1.2",
        "2.1"
      ]
    },
    {
      "id": 1,
      "tasks": [
        "1.3",
        "1.4",
        "2.2",
        "3.1"
      ]
    },
    {
      "id": 2,
      "tasks": [
        "1.5",
        "2.3",
        "2.4",
        "2.5",
        "3.2"
      ]
    },
    {
      "id": 3,
      "tasks": [
        "1.6",
        "2.6",
        "2.7",
        "3.3",
        "4.1"
      ]
    },
    {
      "id": 4,
      "tasks": [
        "3.4",
        "3.5",
        "3.6",
        "3.7",
        "4.2",
        "4.3"
      ]
    },
    {
      "id": 5,
      "tasks": [
        "4.4",
        "4.5",
        "4.6",
        "4.7",
        "5.1",
        "5.2"
      ]
    },
    {
      "id": 6,
      "tasks": [
        "5.3",
        "5.4",
        "5.5",
        "5.6",
        "5.7",
        "6.1"
      ]
    },
    {
      "id": 7,
      "tasks": [
        "6.2",
        "6.3",
        "6.4",
        "6.5",
        "6.6",
        "6.7",
        "6.8",
        "7.1"
      ]
    },
    {
      "id": 8,
      "tasks": [
        "7.2",
        "7.3",
        "7.4",
        "7.5",
        "7.6",
        "7.7",
        "7.8",
        "8.1"
      ]
    },
    {
      "id": 9,
      "tasks": [
        "8.2",
        "8.3",
        "8.4",
        "8.5",
        "8.6",
        "8.7",
        "9.1"
      ]
    },
    {
      "id": 10,
      "tasks": [
        "9.2",
        "9.3",
        "9.4",
        "9.5",
        "9.6",
        "9.7",
        "9.8",
        "9.9",
        "10.1"
      ]
    },
    {
      "id": 11,
      "tasks": [
        "10.2",
        "10.3",
        "10.4",
        "10.5",
        "10.6",
        "10.7",
        "10.8",
        "10.9",
        "10.10",
        "10.11"
      ]
    },
    {
      "id": 12,
      "tasks": [
        "11.1",
        "11.2",
        "11.3",
        "11.4",
        "11.5",
        "11.6",
        "11.7"
      ]
    },
    {
      "id": 13,
      "tasks": [
        "11.8",
        "11.9",
        "11.10",
        "11.11",
        "11.12",
        "11.13"
      ]
    },
    {
      "id": 14,
      "tasks": [
        "11.14"
      ]
    }
  ]
}
```

---

## Success Metrics

By the end of all slices, the application will:

1. ✅ Accept CSV uploads with validation (Slice 1-2)
2. ✅ Automatically classify expenses with confidence levels (Slice 3)
3. ✅ Sort by 4 different attributes with persistence (Slice 4)
4. ✅ Filter by category with multi-select (Slice 5)
5. ✅ Filter by date range (all, month, custom) (Slice 6)
6. ✅ Correct classifications and update database (Slice 7)
7. ✅ Display summary statistics (Slice 8)
8. ✅ Visualize data with pie chart (Slice 9)
9. ✅ Show spending trends with histograms (Slice 10)
10. ✅ Auto-update similar expenses on correction (Slice 11)
11. ✅ Production-ready with tests, docs, and styling (Slice 11)

Each slice is independently testable in the browser and builds incrementally toward a complete expense tracking system.
