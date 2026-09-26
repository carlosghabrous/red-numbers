package handlers

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewUploadHandler(logger)

	tests := []struct {
		name           string
		csv            string
		expectedStatus int
		expectedText   string
	}{
		{
			name: "valid CSV displays parsed data",
			csv: "fecha de operación;concepto;fecha valor;importe;saldo\n" +
				"05/01/2026;Mercadona;05/01/2026;-18,00;2.121,64\n",
			expectedStatus: http.StatusOK,
			expectedText:   "Mercadona",
		},
		{
			name: "missing required column returns error",
			csv: "fecha de operación;concepto;fecha valor;saldo\n" +
				"05/01/2026;Mercadona;05/01/2026;2.121,64\n",
			expectedStatus: http.StatusBadRequest,
			expectedText:   "importe",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "expenses.csv")
			if err != nil {
				t.Fatalf("CreateFormFile failed: %v", err)
			}
			if _, err := part.Write([]byte(test.csv)); err != nil {
				t.Fatalf("Writing CSV failed: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("Closing multipart writer failed: %v", err)
			}

			request := httptest.NewRequest(http.MethodPost, "/upload", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()

			handler.HandlePostUpload(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf("Expected status %d, got %d", test.expectedStatus, response.Code)
			}
			if !bytes.Contains(response.Body.Bytes(), []byte(test.expectedText)) {
				t.Errorf("Expected response to contain %q", test.expectedText)
			}
		})
	}
}
