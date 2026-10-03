package upload

import (
	"os"
	"testing"
)

func writeTempCSV(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	return tmpFile.Name()
}

// TestParseCommaDelimitedCSV reproduces a real bank export that uses comma
// (not semicolon) as the field delimiter, with a leading blank line, a
// leading empty column, and quoted European-format amounts (so the decimal
// comma inside "-37,53" isn't mistaken for a field separator).
func TestParseCommaDelimitedCSV(t *testing.T) {
	parser := NewCSVParser()
	content := "\n" +
		",FECHA DE OPERACIÓN,CONCEPTO,FECHA VALOR,IMPORTE,SALDO\n" +
		",30/09/2026,TARJETA *0085 ALDI AVD BALEARES,30/09/2026,\"-37,53\",\"3.374,31\"\n" +
		",29/09/2026,TRASPASO A CTA DE: CARLOS GHABROUS LARRE,29/09/2026,\"3.000,00\",\"3.853,84\"\n"

	path := writeTempCSV(t, content)
	expenses, err := parser.ParseCSVFile(path)
	if err != nil {
		t.Fatalf("ParseCSVFile failed: %v", err)
	}
	if len(expenses) != 2 {
		t.Fatalf("expected 2 expenses, got %d: %+v", len(expenses), expenses)
	}
	if expenses[0].Description != "TARJETA *0085 ALDI AVD BALEARES" || expenses[0].Amount != -37.53 || expenses[0].Balance != 3374.31 {
		t.Errorf("unexpected first expense: %+v", expenses[0])
	}
	if expenses[1].Amount != 3000.00 || expenses[1].Balance != 3853.84 {
		t.Errorf("expected thousands separator handled correctly, got %+v", expenses[1])
	}
}

func TestParseCommaDelimitedCSVMissingColumnsStillReportsError(t *testing.T) {
	parser := NewCSVParser()
	content := ",FECHA DE OPERACIÓN,CONCEPTO,FECHA VALOR,SALDO\n" +
		",30/09/2026,ALDI,30/09/2026,\"100,00\"\n"

	path := writeTempCSV(t, content)
	if _, err := parser.ParseCSVFile(path); err == nil {
		t.Fatal("expected an error for a comma-delimited file missing the importe column")
	}
}

// Semicolon-delimited files (the original format) must keep working exactly
// as before now that comma is also tried.
func TestParseSemicolonDelimitedCSVStillWorks(t *testing.T) {
	parser := NewCSVParser()
	content := "fecha de operación;concepto;fecha valor;importe;saldo\n" +
		"01/01/2024;Mercadona Compra;01/01/2024;45,50;1.234,56\n"

	path := writeTempCSV(t, content)
	expenses, err := parser.ParseCSVFile(path)
	if err != nil {
		t.Fatalf("ParseCSVFile failed: %v", err)
	}
	if len(expenses) != 1 || expenses[0].Amount != 45.50 || expenses[0].Balance != 1234.56 {
		t.Fatalf("unexpected result: %+v err=%v", expenses, err)
	}
}
