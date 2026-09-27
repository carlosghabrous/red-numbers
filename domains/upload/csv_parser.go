package upload

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ghab/red-numbers/domains/expenses"
)

// CSVParser handles parsing and validation of CSV files.
type CSVParser struct{}

// NewCSVParser creates a new CSV parser.
func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

// RequiredColumns defines the columns that must be present in the CSV.
var RequiredColumns = []string{"fecha de operación", "concepto", "fecha valor", "importe", "saldo"}

// ParseCSVFile parses a CSV file and returns Expense objects.
// It validates headers and skips invalid rows, collecting errors for reporting.
func (p *CSVParser) ParseCSVFile(filePath string) ([]expenses.Expense, error) {
	parsed, _, err := p.ParseCSVFileWithStats(filePath)
	return parsed, err
}

// ParseCSVFileWithStats parses a CSV file and reports rows skipped during parsing.
func (p *CSVParser) ParseCSVFileWithStats(filePath string) ([]expenses.Expense, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	reader.Comment = '#'
	reader.FieldsPerRecord = -1

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read CSV file: %w", err)
	}

	if len(records) == 0 {
		return nil, 0, fmt.Errorf("CSV file is empty")
	}

	// Find and validate the header row. Bank exports may contain metadata rows
	// before the transaction table.
	headerIdx, headerMap, err := p.findHeaderRow(records)
	if err != nil {
		return nil, 0, err
	}

	// Parse data rows
	parsed := make([]expenses.Expense, 0)
	skippedRows := 0
	for rowIdx := headerIdx + 1; rowIdx < len(records); rowIdx++ {
		row := records[rowIdx]

		// Skip empty rows
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue
		}

		expense, err := p.parseRow(row, headerMap)
		if err != nil {
			// Log error but continue parsing other rows
			fmt.Printf("Warning: Skipping row %d: %v\n", rowIdx+1, err)
			skippedRows++
			continue
		}

		parsed = append(parsed, *expense)
	}

	return parsed, skippedRows, nil
}

func (p *CSVParser) findHeaderRow(records [][]string) (int, map[string]int, error) {
	var lastErr error
	for rowIdx, row := range records {
		headerMap, err := p.validateHeaders(row)
		if err == nil {
			return rowIdx, headerMap, nil
		}
		lastErr = err
	}

	return 0, nil, lastErr
}

// validateHeaders checks that all required columns are present and returns a column index map.
func (p *CSVParser) validateHeaders(headers []string) (map[string]int, error) {
	// Convert headers to lowercase for case-insensitive matching
	lowerHeaders := make([]string, len(headers))
	for i, h := range headers {
		lowerHeaders[i] = strings.ToLower(strings.TrimSpace(h))
	}

	// Create map of column name to index
	columnMap := make(map[string]int)
	for i, h := range lowerHeaders {
		columnMap[h] = i
	}

	// Check for required columns
	var missingColumns []string
	for _, required := range RequiredColumns {
		if _, exists := columnMap[required]; !exists {
			missingColumns = append(missingColumns, required)
		}
	}

	if len(missingColumns) > 0 {
		return nil, fmt.Errorf("CSV is missing required columns: %v", missingColumns)
	}

	return columnMap, nil
}

// parseRow extracts a CSV row into an Expense object.
func (p *CSVParser) parseRow(row []string, columnMap map[string]int) (*expenses.Expense, error) {
	// Extract required columns
	fechaIdx := columnMap["fecha de operación"]
	conceptoIdx := columnMap["concepto"]
	fechaValorIdx := columnMap["fecha valor"]
	importeIdx := columnMap["importe"]
	saldoIdx := columnMap["saldo"]

	// Validate column indices are within bounds
	maxIdx := len(row)
	if fechaIdx >= maxIdx || conceptoIdx >= maxIdx || fechaValorIdx >= maxIdx || importeIdx >= maxIdx || saldoIdx >= maxIdx {
		return nil, fmt.Errorf("row has fewer columns than expected")
	}

	fechaStr := strings.TrimSpace(row[fechaIdx])
	concepto := strings.TrimSpace(row[conceptoIdx])
	importeStr := strings.TrimSpace(row[importeIdx])
	saldoStr := strings.TrimSpace(row[saldoIdx])

	// Parse date
	if fechaStr == "" {
		return nil, fmt.Errorf("fecha (date) is empty")
	}
	date, err := p.parseDate(fechaStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format in fecha: %w", err)
	}

	// Parse amount (importe)
	if importeStr == "" {
		return nil, fmt.Errorf("importe (amount) is empty")
	}
	amount, err := p.parseAmount(importeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid amount format in importe: %w", err)
	}

	// Parse balance (saldo)
	balance := 0.0
	if saldoStr != "" {
		var err error
		balance, err = p.parseAmount(saldoStr)
		if err != nil {
			// Balance is optional, so we don't fail if it's invalid
			balance = 0.0
		}
	}

	expense := &expenses.Expense{
		Date:        date,
		Description: concepto,
		Amount:      amount,
		Balance:     balance,
		ImportedAt:  time.Now(),
	}

	return expense, nil
}

// parseDate parses a date string in either DD/MM/YYYY or DD-MM-YYYY format.
func (p *CSVParser) parseDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)

	// Try DD/MM/YYYY format first
	if t, err := time.Parse("02/01/2006", dateStr); err == nil {
		return t, nil
	}

	// Try DD-MM-YYYY format
	if t, err := time.Parse("02-01-2006", dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("date format not recognized: %s (expected DD/MM/YYYY or DD-MM-YYYY)", dateStr)
}

// parseAmount parses an amount string handling both comma and period decimal separators.
// Supports European format (1.234,56) and US format (1234.56).
func (p *CSVParser) parseAmount(amountStr string) (float64, error) {
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return 0, fmt.Errorf("amount is empty")
	}

	// Remove currency symbols and whitespace
	amountStr = strings.Trim(amountStr, "€$ \t")

	// Count commas and periods to determine separator
	commaCount := strings.Count(amountStr, ",")
	periodCount := strings.Count(amountStr, ".")

	// Handle cases where both separators are present
	if commaCount > 0 && periodCount > 0 {
		// If comma appears after period, use comma as decimal separator (European)
		lastCommaIdx := strings.LastIndex(amountStr, ",")
		lastPeriodIdx := strings.LastIndex(amountStr, ".")
		if lastCommaIdx > lastPeriodIdx {
			// European format: remove periods and replace comma with period
			amountStr = strings.ReplaceAll(amountStr, ".", "")
			amountStr = strings.Replace(amountStr, ",", ".", 1)
		} else {
			// US format: remove commas
			amountStr = strings.ReplaceAll(amountStr, ",", "")
		}
	} else if commaCount > 0 && periodCount == 0 {
		// Only commas present - could be thousands or decimal separator
		// If comma is in last 3 positions, it's likely a decimal separator (European)
		if lastCommaIdx := strings.LastIndex(amountStr, ","); lastCommaIdx >= len(amountStr)-4 {
			amountStr = strings.Replace(amountStr, ",", ".", 1)
		} else {
			// It's a thousands separator, remove it
			amountStr = strings.ReplaceAll(amountStr, ",", "")
		}
	}
	// If only periods, leave as is (US format)

	value, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse amount as float: %w", err)
	}

	return value, nil
}
