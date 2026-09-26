package services

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseValidCSVFile(t *testing.T) {
	parser := NewCSVParser()

	// Create a temporary CSV file with proper formatting
	content := "fecha de operación;concepto;fecha valor;importe;saldo\n" +
		"01/01/2024;Mercadona Compra;01/01/2024;45.50;1234.56\n" +
		"02/01/2024;Farmacia Medica;02/01/2024;65.00;1169.56\n"

	tmpFile, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	expenses, err := parser.ParseCSVFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseCSVFile failed: %v", err)
	}

	if len(expenses) != 2 {
		t.Errorf("Expected 2 expenses, got %d", len(expenses))
	}

	// Check first expense
	if expenses[0].Description != "Mercadona Compra" {
		t.Errorf("Expected description 'Mercadona Compra', got '%s'", expenses[0].Description)
	}
	if expenses[0].Amount != 45.50 {
		t.Errorf("Expected amount 45.50, got %.2f", expenses[0].Amount)
	}
	if expenses[0].Balance != 1234.56 {
		t.Errorf("Expected balance 1234.56, got %.2f", expenses[0].Balance)
	}
}

func TestParseCSVMissingRequiredColumns(t *testing.T) {
	parser := NewCSVParser()

	// Create CSV with missing 'importe' column
	content := "fecha de operación;concepto;fecha valor;saldo\n" +
		"01/01/2024;Mercadona Compra;01/01/2024;1234.56\n"

	tmpFile, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	_, err = parser.ParseCSVFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for missing required columns, got nil")
	}
	if !strings.Contains(err.Error(), "missing required columns") {
		t.Errorf("Expected error about missing columns, got: %v", err)
	}
}

func TestParseRowInvalidDate(t *testing.T) {
	parser := NewCSVParser()

	// Create CSV with invalid date format
	content := "fecha de operación;concepto;fecha valor;importe;saldo\n" +
		"2024-01-01;Mercadona Compra;01/01/2024;45.50;1234.56\n"

	tmpFile, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	expenses, err := parser.ParseCSVFile(tmpFile.Name())
	// Should skip the invalid row and return empty list
	if len(expenses) != 0 {
		t.Errorf("Expected 0 expenses for invalid row, got %d", len(expenses))
	}
}

func TestParseDateFormats(t *testing.T) {
	parser := NewCSVParser()

	tests := []struct {
		name        string
		dateStr     string
		expectedDay int
	}{
		{"DD/MM/YYYY format", "15/03/2024", 15},
		{"DD-MM-YYYY format", "15-03-2024", 15},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			date, err := parser.parseDate(test.dateStr)
			if err != nil {
				t.Errorf("parseDate failed: %v", err)
			}
			if date.Day() != test.expectedDay {
				t.Errorf("Expected day %d, got %d", test.expectedDay, date.Day())
			}
		})
	}
}

func TestParseDateInvalidFormat(t *testing.T) {
	parser := NewCSVParser()

	_, err := parser.parseDate("2024-15-03")
	if err == nil {
		t.Error("Expected error for invalid date format")
	}
}

func TestParseAmountFormats(t *testing.T) {
	parser := NewCSVParser()

	tests := []struct {
		name      string
		amountStr string
		expected  float64
	}{
		{"European format with comma", "1.234,56", 1234.56},
		{"European format simple", "45,50", 45.50},
		{"US format", "1234.56", 1234.56},
		{"US format simple", "45.50", 45.50},
		{"No decimal separator", "100", 100.0},
		{"European thousands with comma", "1.000,00", 1000.0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			amount, err := parser.parseAmount(test.amountStr)
			if err != nil {
				t.Errorf("parseAmount failed: %v", err)
			}
			if amount != test.expected {
				t.Errorf("Expected %.2f, got %.2f", test.expected, amount)
			}
		})
	}
}

func TestParseAmountWithCurrency(t *testing.T) {
	parser := NewCSVParser()

	amount, err := parser.parseAmount("€45,50")
	if err != nil {
		t.Errorf("parseAmount failed: %v", err)
	}
	if amount != 45.50 {
		t.Errorf("Expected 45.50, got %.2f", amount)
	}
}

func TestParseAmountInvalid(t *testing.T) {
	parser := NewCSVParser()

	_, err := parser.parseAmount("not-a-number")
	if err == nil {
		t.Error("Expected error for invalid amount")
	}
}

func TestValidateHeadersCaseInsensitive(t *testing.T) {
	parser := NewCSVParser()

	headers := []string{"FECHA DE OPERACIÓN", "CONCEPTO", "FECHA VALOR", "IMPORTE", "SALDO"}
	headerMap, err := parser.validateHeaders(headers)
	if err != nil {
		t.Fatalf("validateHeaders failed: %v", err)
	}

	// Check that all required columns are found
	for _, required := range RequiredColumns {
		if _, exists := headerMap[required]; !exists {
			t.Errorf("Expected column '%s' not found in header map", required)
		}
	}
}

func TestParseCSVWithEmptyRows(t *testing.T) {
	parser := NewCSVParser()

	// Create CSV with empty rows
	content := "fecha de operación;concepto;fecha valor;importe;saldo\n" +
		"01/01/2024;Mercadona Compra;01/01/2024;45.50;1234.56\n" +
		"\n" +
		"02/01/2024;Farmacia Medica;02/01/2024;65.00;1169.56\n"

	tmpFile, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	expenses, err := parser.ParseCSVFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseCSVFile failed: %v", err)
	}

	// Should skip empty row and parse 2 valid expenses
	if len(expenses) != 2 {
		t.Errorf("Expected 2 expenses, got %d", len(expenses))
	}
}

func TestParseCSVMissingRequiredField(t *testing.T) {
	parser := NewCSVParser()

	// Create CSV with missing fecha
	content := "fecha de operación;concepto;fecha valor;importe;saldo\n" +
		";Mercadona Compra;01/01/2024;45.50;1234.56\n" +
		"02/01/2024;Farmacia Medica;02/01/2024;65.00;1169.56\n"

	tmpFile, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	expenses, err := parser.ParseCSVFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseCSVFile failed: %v", err)
	}

	// Should skip row with missing fecha and parse 1 valid expense
	if len(expenses) != 1 {
		t.Errorf("Expected 1 expense, got %d", len(expenses))
	}
	if expenses[0].Description != "Farmacia Medica" {
		t.Errorf("Expected 'Farmacia Medica', got '%s'", expenses[0].Description)
	}
}

func TestParseRowImportedAtTimestamp(t *testing.T) {
	parser := NewCSVParser()

	// Record time before parsing
	beforeParse := time.Now()

	content := "fecha de operación;concepto;fecha valor;importe;saldo\n" +
		"01/01/2024;Mercadona Compra;01/01/2024;45.50;1234.56\n"

	tmpFile, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	expenses, err := parser.ParseCSVFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseCSVFile failed: %v", err)
	}

	afterParse := time.Now()

	if len(expenses) != 1 {
		t.Fatalf("Expected 1 expense, got %d", len(expenses))
	}

	// Check that ImportedAt is set and within expected timeframe
	if expenses[0].ImportedAt.Before(beforeParse) || expenses[0].ImportedAt.After(afterParse) {
		t.Errorf("ImportedAt timestamp outside expected range")
	}
}

func TestParseCSVEmptyFile(t *testing.T) {
	parser := NewCSVParser()

	tmpFile, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	_, err = parser.ParseCSVFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for empty CSV file")
	}
}

func TestParseAmountEdgeCases(t *testing.T) {
	parser := NewCSVParser()

	tests := []struct {
		name      string
		amountStr string
		expected  float64
	}{
		{"Negative amount European", "-45,50", -45.50},
		{"Negative amount US", "-45.50", -45.50},
		{"Zero", "0", 0.0},
		{"Leading space", " 45,50", 45.50},
		{"Trailing space", "45,50 ", 45.50},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			amount, err := parser.parseAmount(test.amountStr)
			if err != nil {
				t.Errorf("parseAmount failed: %v", err)
			}
			if amount != test.expected {
				t.Errorf("Expected %.2f, got %.2f", test.expected, amount)
			}
		})
	}
}
