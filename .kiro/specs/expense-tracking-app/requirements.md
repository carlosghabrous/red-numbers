# Requirements Document

## Introduction

This document specifies the requirements for an expense tracking web application that enables users to upload bank export CSV files, automatically categorize expenses through intelligent fuzzy logic, review and correct categorizations, and visualize spending patterns through interactive charts and filtering options.

The system processes Spanish-language bank export files, stores expenses in SQLite, and provides a server-side rendered interface with minimal client-side JavaScript for efficient browsing and analysis of personal finances.

## Glossary

- **System**: Expense_Tracking_Application (the entire application)
- **User**: An individual who uploads and analyzes their personal expense data
- **Admin**: A user with permission to upload CSV files and correct categorizations
- **CSV_File**: A comma-separated values file exported from a bank, containing transaction data
- **Bank_Export**: A CSV file containing banking transaction records with standard columns
- **Expense**: A single transaction record extracted from a Bank_Export file
- **Category**: One of the predefined expense types: supermercado, medico, niños, ocio, deporte, suministros, casa
- **Fuzzy_Classifier**: An algorithm that uses pattern matching to assign expenses to categories based on transaction descriptions
- **Categorization**: The assignment of an Expense to a specific Category
- **Miscategorized_Expense**: An Expense assigned to an incorrect Category
- **Classification_Rule**: A learned pattern created when a User corrects a Miscategorized_Expense
- **Re-Classification**: The process of updating all stored expenses based on new or updated Classification_Rules
- **Visualization_Widget**: A UI component that displays expense data in chart or summary form
- **Date_Range**: A pair of dates defining a span of time (start_date, end_date)
- **Week_Period**: A 7-day period from Monday through Sunday
- **Month_Period**: All Expenses within a calendar month
- **Expense_Query**: A set of filters applied to retrieve and display a subset of Expenses
- **SQLite_Database**: The persistent data store holding all Expenses, Categories, and Classification_Rules

## Requirements

### Requirement 1: CSV File Upload and Ingestion

**User Story:** As an Admin, I want to upload bank export CSV files, so that I can import my transaction history into the system.

#### Acceptance Criteria

1. WHEN an Admin accesses the CSV upload interface, THE System SHALL display a file upload form with instructions for selecting a valid Bank_Export file
2. WHEN an Admin selects a CSV_File and submits the upload, THE System SHALL validate that the file contains the required columns: fecha, concepto, importe, saldo
3. IF the CSV_File is missing required columns, THEN THE System SHALL return a descriptive error message specifying which columns are missing
4. WHEN a valid CSV_File is uploaded, THE System SHALL parse all Expense records and store them in the SQLite_Database
5. WHEN CSV_File rows are parsed, THE System SHALL extract: transaction date (fecha), description (concepto), amount (importe), and running balance (saldo)
6. IF an Expense record has invalid or missing data in required fields (fecha, importe), THEN THE System SHALL skip that row, log the error, and continue importing remaining valid rows
7. WHEN all valid Expenses from a CSV_File are imported, THE System SHALL display a summary showing the number of successfully imported Expenses and any errors encountered

### Requirement 2: Intelligent Expense Categorization

**User Story:** As a User, I want expenses to be automatically categorized based on transaction descriptions, so that I don't have to manually assign categories to every transaction.

#### Acceptance Criteria

1. WHEN an Expense is imported, THE Fuzzy_Classifier SHALL analyze the concepto (description) and assign the Expense to one of these Categories: supermercado, medico, niños, ocio, deporte, suministros, casa
2. WHEN the Fuzzy_Classifier processes a description, THE System SHALL use pattern matching (keyword-based matching or similar fuzzy logic) to identify the most likely Category
3. WHEN an Expense matches multiple patterns with equal confidence, THE System SHALL assign the Expense to the highest-priority Category based on predefined heuristics
4. WHEN an Expense does not match any classification pattern with confidence above a threshold, THE System SHALL assign it to a default Category and mark it as low-confidence
5. FOR Expenses with descriptions containing: "mercadona", "consum", "carrefour", "aldi", "hiperchina", "dia retail", THE Fuzzy_Classifier SHALL assign Category: supermercado
6. FOR Expenses with descriptions containing: "farmacia", "clinica", "dental", "medico", THE Fuzzy_Classifier SHALL assign Category: medico
7. FOR Expenses with descriptions containing: "colegio", "educarte", "crececonmusica", THE Fuzzy_Classifier SHALL assign Category: niños
8. FOR Expenses with descriptions containing: "cinema", "cine", "ocine", "enjoy", "amazon prime", "netflix", "downdog", "pilates", THE Fuzzy_Classifier SHALL assign Category: ocio
9. FOR Expenses with descriptions containing: "club natacion", "delfin", "mirador", "altitud", "bergueda", "extreme", THE Fuzzy_Classifier SHALL assign Category: deporte
10. FOR Expenses with descriptions containing: "iberdrola", "pepe mobile", "telefonica", "telefónica", "agua", "gas", THE Fuzzy_Classifier SHALL assign Category: suministros
11. FOR Expenses with descriptions containing: "alquiler", "renta", "casa", "volkswagen renting", THE Fuzzy_Classifier SHALL assign Category: casa
12. WHEN a Classification_Rule is updated (via correction), THE System SHALL immediately re-classify ALL stored Expenses unconditionally, including previously corrected or confidently classified ones

### Requirement 3: Classification Review and User Correction

**User Story:** As an Admin, I want to review automatically assigned categories and correct any miscategorizations, so that my expense data is accurate and my categorization rules improve over time.

#### Acceptance Criteria

1. WHEN viewing an Expense record, THE System SHALL display the assigned Category with a clear indication of confidence level (high, medium, low)
2. WHEN an Admin identifies a Miscategorized_Expense, THE System SHALL provide a UI control to change the Category to the correct value
3. WHEN an Admin corrects a Miscategorized_Expense, THE System SHALL save the correction and update the Classification_Rule for that pattern
4. WHEN a Classification_Rule is created or updated from a correction, THE System SHALL store the original description pattern and the corrected Category
5. WHEN a correction is submitted, THE System SHALL display a confirmation message showing the updated Category and offer the option to apply the corrected rule to all similar Expenses
6. WHERE an Admin chooses to apply a correction rule retroactively, THE System SHALL re-classify all stored Expenses that match the updated pattern
7. WHEN an Admin submits a correction, THE System SHALL log the change with timestamp and user information for audit purposes

### Requirement 4: Re-Classification Engine

**User Story:** As an Admin, I want automatic re-classification of all expenses when I correct a miscategorization, so that my entire history stays consistent with my updated understanding of expense categories.

#### Acceptance Criteria

1. WHEN a Classification_Rule is updated (either created or modified), THE System SHALL trigger a Re-Classification process
2. WHEN a Re-Classification process begins, THE System SHALL iterate through all stored Expenses in the SQLite_Database
3. DURING Re-Classification, THE System SHALL apply the updated Classification_Rules to each Expense and update its Category if the new classification differs from the current one
4. WHEN Re-Classification completes, THE System SHALL display a summary showing: number of Expenses re-classified (non-negative count), number of changes made (non-negative count), and the affected Categories
5. DURING Re-Classification, THE System SHALL maintain data integrity and not lose or corrupt Expense records
6. IF Re-Classification encounters an error processing a specific Expense, THEN THE System SHALL log the error and continue processing remaining Expenses
7. WHEN Re-Classification completes, THE System SHALL update all Visualization_Widgets to reflect the new categorization

### Requirement 5: Expense Viewing and Sorting

**User Story:** As a User, I want to view all imported expenses sorted by date, with options to sort by other attributes, so that I can find and analyze transactions efficiently.

#### Acceptance Criteria

1. WHEN a User navigates to the Expense List view, THE System SHALL display all imported Expenses sorted by transaction date in descending order (newest first) by default
2. WHEN a User selects a sort option, THE System SHALL re-order the Expense list according to the selected attribute: date, amount, category, or description
3. WHEN a User selects ascending or descending sort direction, THE System SHALL apply the sort direction to the currently selected sort attribute
4. WHEN sorting by amount, THE System SHALL order Expenses from smallest to largest (ascending) or largest to smallest (descending)
5. WHEN sorting by category, THE System SHALL group Expenses by Category and order category groups alphabetically by name, then sort within each group by date (descending)
6. WHEN viewing an Expense List, THE System SHALL display each Expense with: date, description, amount, category, and confidence level

### Requirement 6: Expense Filtering by Type

**User Story:** As a User, I want to filter expenses by category type, so that I can focus on specific spending areas.

#### Acceptance Criteria

1. WHEN viewing the Expense List, THE System SHALL provide UI controls to filter by one or more Categories
2. WHEN a User selects a Category filter, THE System SHALL display only Expenses assigned to the selected Category
3. WHEN a User selects multiple Category filters, THE System SHALL display Expenses matching any of the selected Categories (OR logic)
4. WHEN a User clears all Category filters, THE System SHALL display all Expenses again
5. WHEN a Category filter is applied, THE System SHALL update all Visualization_Widgets to show data only for the filtered Categories
6. WHEN viewing filtered data, THE System SHALL display a clear indication that filters are active and show both filter indication and the selected Categories checked separately

### Requirement 7: Expense Filtering by Date Range

**User Story:** As a User, I want to filter expenses by date ranges (specific dates, months, weeks), so that I can analyze spending patterns over specific time periods.

#### Acceptance Criteria

1. WHEN viewing the Expense List, THE System SHALL provide date range filtering controls with options for: custom date range, current month, previous month, current year, or custom weeks
2. WHEN a User enters a custom Date_Range (start_date and end_date), THE System SHALL validate that start_date is not after end_date
3. IF start_date is after end_date, THEN THE System SHALL display an error message and reject the filter
4. WHEN a User selects "current month", THE System SHALL display only Expenses from the first to the last day of the current calendar month
5. WHEN a User selects "by month", THE System SHALL display a month selector and filter Expenses to the selected Month_Period
6. WHEN a User selects "by week", THE System SHALL filter Expenses to a Week_Period (Monday through Sunday) and provide controls to navigate between weeks
7. WHEN a Date_Range filter is applied, THE System SHALL update all Visualization_Widgets immediately to display data only from the filtered time period
8. WHEN a User clears the Date_Range filter, THE System SHALL display Expenses from all dates again

### Requirement 8: Monthly Expense Breakdown Pie Chart

**User Story:** As a User, I want to see a pie chart showing my total expenses broken down by category for a selected month, so that I can understand where my money is going.

#### Acceptance Criteria

1. WHEN viewing the dashboard or reports page, THE Pie_Chart_Widget SHALL display a visual pie chart showing the proportion of expenses for each Category
2. WHEN a Month_Period filter is active, THE Pie_Chart_Widget SHALL display the breakdown of expenses only for the selected month
3. WHEN calculating pie chart segments, THE System SHALL sum all Expense amounts for each Category and calculate the percentage of total spending
4. WHEN an Expense is re-classified, THE Pie_Chart_Widget SHALL automatically update to reflect the new categorization
4. WHEN hovering over a pie chart segment, THE System SHALL display a tooltip showing the Category name, total amount, and percentage of total spending
5. WHEN clicking on a pie chart segment, THE System SHALL defer this optional filtering feature to a future release

### Requirement 9: Weekly Expense Histogram by Category

**User Story:** As a User, I want to see a histogram showing expense distribution across weeks, with separate bars for each category, so that I can identify weekly spending trends.

#### Acceptance Criteria

1. WHEN viewing the histogram widget, THE System SHALL display a bar chart with weeks on the x-axis and expense amounts on the y-axis
2. FOR each Week_Period in the displayed range, THE System SHALL calculate the total spending per Category and display as stacked or grouped bars
3. WHEN a Category filter is active, THE Histogram_Widget SHALL display bars only for the selected Categories
4. WHEN a Date_Range filter is active, THE Histogram_Widget SHALL display data only for weeks within the selected range
5. WHEN an Expense is re-classified or imported, THE Histogram_Widget SHALL automatically update to reflect the changes
6. WHEN hovering over a bar, THE System SHALL display a tooltip showing the week, Category, and total amount for that week

### Requirement 10: Monthly Expense Histogram by Category

**User Story:** As a User, I want to see a histogram showing expense distribution across months, with separate bars for each category, so that I can identify monthly spending trends and seasonal patterns.

#### Acceptance Criteria

1. WHEN viewing the monthly histogram widget, THE System SHALL display a bar chart with months on the x-axis and expense amounts on the y-axis
2. FOR each Month_Period in the imported data range, THE System SHALL calculate the total spending per Category and display as stacked or grouped bars
3. WHEN a Category filter is active, THE Monthly_Histogram_Widget SHALL display bars only for the selected Categories
4. WHEN a Date_Range filter is active, THE Monthly_Histogram_Widget SHALL display data only for months within the selected range
5. WHEN an Expense is re-classified or imported, THE Monthly_Histogram_Widget SHALL automatically update to reflect the changes
6. WHEN hovering over a bar, THE System SHALL display a tooltip showing the month, Category, and total amount for that month

### Requirement 11: Summary Statistics Widget

**User Story:** As a User, I want to see summary statistics showing total spending, average transaction amount, and other key metrics, so that I can quickly understand my spending patterns.

#### Acceptance Criteria

1. WHEN viewing the summary widget, THE System SHALL display: total spending amount, number of transactions, average transaction amount, and highest category by spending
2. WHEN a Date_Range or Category filter is active, THE Summary_Widget SHALL calculate statistics only for the filtered Expenses
3. WHEN an Expense is imported or re-classified, THE Summary_Widget SHALL automatically update to reflect the current data
4. WHEN viewing the summary, THE System SHALL display currency formatted amounts (euros, with 2 decimal places)

### Requirement 12: Database Schema and Data Persistence

**User Story:** As a Developer, I want a robust SQLite database schema that stores all expense data, categories, and classification rules, so that the system can reliably persist and query expense data.

#### Acceptance Criteria

1. THE System SHALL create and maintain a SQLite_Database with tables for: Expenses, Categories, Classification_Rules, and optionally Audit_Log
2. THE Expenses table SHALL include columns: id (primary key), date, description, amount, saldo, category_id (foreign key), confidence_level, imported_at
3. THE Categories table SHALL include columns: id (primary key), name (unique), display_name
4. THE Classification_Rules table SHALL include columns: id (primary key), pattern, category_id (foreign key), created_at, updated_at, priority
5. THE Audit_Log table (optional) SHALL track: timestamp, user_id, action, expense_id, old_value, new_value
6. WHEN querying Expenses, THE System SHALL efficiently retrieve records filtered by date range, category, and description using database indexes
7. WHEN the application starts, THE System SHALL verify database schema integrity and create tables if they do not exist

### Requirement 13: CSV File Format Validation

**User Story:** As a System, I want to validate that uploaded CSV files match the expected format, so that users receive clear feedback about upload issues.

#### Acceptance Criteria

1. WHEN a CSV_File is selected for upload, THE System SHALL verify that the file has a .csv extension
2. WHEN a CSV_File is parsed, THE System SHALL verify that the first row contains the expected column headers: fecha, concepto, importe, saldo
3. IF required columns are missing, THEN THE System SHALL reject the file and display which columns are missing
4. WHEN parsing CSV rows, THE System SHALL handle various date formats (DD/MM/YYYY, DD-MM-YYYY) and convert them to a standard internal format
5. WHEN parsing the importe column, THE System SHALL handle both comma (European decimal separator) and period as decimal separators
6. IF a row cannot be parsed due to data type errors (non-numeric importe), THEN THE System SHALL skip the row and log the error, continuing to import remaining rows

### Requirement 14: User Interface and Server-Side Rendering

**User Story:** As a User, I want a responsive, easy-to-use interface with minimal client-side complexity, so that the application works reliably across devices and doesn't require complex JavaScript frameworks.

#### Acceptance Criteria

1. THE System SHALL render HTML pages on the server and serve fully-formed HTML to the browser
2. WHEN a User performs actions (sorting, filtering, uploading), THE System SHALL process the request server-side and return updated HTML or redirect to a new page
3. WHEN rendering the Expense List page, THE System SHALL include pagination controls if the number of Expenses exceeds 50 per page
4. WHEN a page includes interactive elements (filters, sorts), THE System SHALL use standard HTML form controls (select, input, button) for accessibility
5. THE System SHALL use CSS media queries to ensure the layout is responsive across all desktop, tablet, and mobile viewports as an unconditional mandate
6. WHEN a User performs a server request that results in an error, THE System SHALL display a user-friendly error message with recovery options

### Requirement 15: Visualization Widget Responsiveness

**User Story:** As a User, I want visualization widgets to be responsive and adapt to my screen size, so that I can view charts on desktop, tablet, and mobile devices.

#### Acceptance Criteria

1. WHEN viewing pie charts and histograms on a mobile device, THE System SHALL render charts that fit within the viewport without horizontal scrolling
2. WHEN screen width is less than 768px, THE System SHALL stack chart elements vertically and reduce font sizes to maintain readability
3. WHEN a chart is displayed, THE System SHALL ensure all axis labels, legend entries, and tooltips are readable on the target device
4. WHEN rendering histograms with many categories or months, THE System SHALL provide horizontal scroll or reduce bar width to fit within viewport

---

## Notes for Design and Implementation

### Fuzzy Classification Strategy

The fuzzy classification approach should:
- Use keyword matching against the concepto (transaction description)
- Maintain a priority order for overlapping matches
- Allow for case-insensitive and partial matching
- Support multiple keywords per category for flexibility
- Include confidence scoring (high/medium/low) based on match strength

### Re-Classification Complexity

Re-classification should be:
- Triggered after any classification rule change
- Performed in a background task to avoid blocking the UI
- Tracked for performance monitoring
- Logged for audit purposes
- Able to handle edge cases (e.g., when a correction matches conflicting rules)

### Future Enhancements (Out of Scope for MVP)

These features are mentioned here for reference but are not required for initial implementation:
- Recurring expense identification and prediction
- Budget alerts and thresholds
- Export reports to PDF or Excel
- Multi-user support with role-based access control
- API for programmatic access
- Mobile app native versions
- Machine learning-based classification refinement
