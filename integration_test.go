package main

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/ghab/red-numbers/database"
)

// TestFullWorkflowEndToEnd drives the whole app the way a real user would,
// through the exact handler chain main() serves (mux + all middleware):
// upload a CSV, see it classified on the dashboard, sort and filter it,
// correct a miscategorized expense, watch a similar expense (and the
// summary/pie chart widgets) update automatically, and finally add a new
// category and use it. Slice 10 (task 10.9) calls for a comprehensive
// integration suite; this is the cross-domain spine that no single
// domain's own tests can exercise.
func TestFullWorkflowEndToEnd(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbPath := filepath.Join(t.TempDir(), "integration.db")
	db, err := database.Initialize(context.Background(), database.Config{DBPath: dbPath}, logger)
	if err != nil {
		t.Fatalf("database.Initialize failed: %v", err)
	}
	defer database.Close(db, logger)

	handler, err := newRouter(db, logger)
	if err != nil {
		t.Fatalf("newRouter failed: %v", err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := server.Client()
	jar := newCookieJar()
	client.Jar = jar

	// --- 1. Health check works unauthenticated -----------------------------
	healthResp, err := client.Get(server.URL + "/health")
	if err != nil || healthResp.StatusCode != http.StatusOK {
		t.Fatalf("health check failed: resp=%v err=%v", healthResp, err)
	}
	healthResp.Body.Close()

	// --- 2. Upload a CSV with a mixed bag of expenses -----------------------
	csvContent := "fecha de operación;concepto;fecha valor;importe;saldo\n" +
		"01/01/2026;MERCADONA MADRID;01/01/2026;-45,50;954,50\n" +
		"02/01/2026;Mercadóna  Madrid;02/01/2026;-12,00;942,50\n" +
		"03/01/2026;Netflix mensual;03/01/2026;-9,99;932,51\n" +
		"04/01/2026;NOMINA EMPRESA;04/01/2026;1500,00;2432,51\n"

	uploadCSRF := csrfTokenFromCookie(t, jar, server.URL)
	uploadBody, contentType := multipartCSV(t, csvContent, uploadCSRF)
	uploadResp, err := client.Post(server.URL+"/upload", contentType, uploadBody)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	uploadHTML := readBody(t, uploadResp)
	if uploadResp.StatusCode != http.StatusOK || !strings.Contains(uploadHTML, "Expenses saved to database:") {
		t.Fatalf("expected successful upload, status=%d body=%s", uploadResp.StatusCode, uploadHTML)
	}
	if !strings.Contains(uploadHTML, "income") {
		t.Fatalf("expected the salary row classified as income in the preview, body=%s", uploadHTML)
	}

	// --- 3. Dashboard shows the imported expenses, sorted and filtered ------
	dashboardHTML := getBody(t, client, server.URL+"/")
	if !strings.Contains(dashboardHTML, "4 expenses stored in the database.") {
		t.Fatalf("expected 4 stored expenses, dashboard=%s", dashboardHTML)
	}

	sortedAsc := getBody(t, client, server.URL+"/?sort=amount&direction=asc")
	if !strings.Contains(sortedAsc, "NOMINA EMPRESA") {
		t.Fatalf("expected salary row present when sorted by amount, body=%s", sortedAsc)
	}

	categoryIDs := extractCategoryOptionIDs(t, dashboardHTML)
	supermercadoID, ok := categoryIDs["Supermercado"]
	if !ok {
		t.Fatalf("expected a Supermercado category option, got %v", categoryIDs)
	}
	filtered := getBody(t, client, server.URL+"/?category="+supermercadoID)
	if !strings.Contains(filtered, "1 category selected") {
		t.Fatalf("expected the category filter to report as active, body=%s", filtered)
	}

	// --- 4. Correct one MERCADONA expense; the similarly-described one and
	//        the summary/pie widgets must update automatically -------------
	expenseID := firstExpenseIDWithDescription(t, db, "MERCADONA MADRID")
	otherCategoryID, ok := categoryIDs["Ocio"]
	if !ok {
		t.Fatalf("expected an Ocio category option, got %v", categoryIDs)
	}

	detailHTML := getBody(t, client, server.URL+"/expenses/"+strconv.FormatInt(expenseID, 10))
	detailCSRF := extractHiddenValue(t, detailHTML, "csrf_token")

	correctForm := url.Values{"category_id": {otherCategoryID}, "csrf_token": {detailCSRF}, "return": {"/"}}
	correctResp, err := client.PostForm(server.URL+"/expenses/"+strconv.FormatInt(expenseID, 10), correctForm)
	if err != nil {
		t.Fatalf("category correction request failed: %v", err)
	}
	correctedHTML := readBody(t, correctResp)
	if !strings.Contains(correctedHTML, "similar expense(s) were also re-classified") {
		t.Fatalf("expected a re-classification confirmation, body=%s", correctedHTML)
	}

	otherID := firstExpenseIDWithDescription(t, db, "Mercadóna  Madrid")
	var newCategoryName string
	if err := db.QueryRow(`SELECT c.name FROM expenses e JOIN categories c ON c.id = e.category_id WHERE e.id = ?`, otherID).Scan(&newCategoryName); err != nil {
		t.Fatalf("query re-classified expense: %v", err)
	}
	if newCategoryName != "ocio" {
		t.Fatalf("expected the similarly-described expense to be re-classified to ocio, got %q", newCategoryName)
	}

	// --- 5. Add a brand-new category from the expense detail page ----------
	addCategoryForm := url.Values{"display_name": {"Mascotas"}, "csrf_token": {detailCSRF}, "return": {"/expenses/" + strconv.FormatInt(expenseID, 10)}}
	addResp, err := client.PostForm(server.URL+"/categories", addCategoryForm)
	if err != nil {
		t.Fatalf("add-category request failed: %v", err)
	}
	addedHTML := readBody(t, addResp)
	if !strings.Contains(addedHTML, "Mascotas") || !strings.Contains(addedHTML, "added") {
		t.Fatalf("expected confirmation that Mascotas was added, body=%s", addedHTML)
	}

	// --- 6. CSRF is actually enforced, not just decorative ------------------
	badForm := url.Values{"category_id": {otherCategoryID}, "csrf_token": {"0000000000000000000000000000000000000000000000000000000000000"}, "return": {"/"}}
	rejectedResp, err := client.PostForm(server.URL+"/expenses/"+strconv.FormatInt(expenseID, 10), badForm)
	if err != nil {
		t.Fatalf("request with bad CSRF token failed transport-wise: %v", err)
	}
	if rejectedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected a bad CSRF token to be rejected with 403, got %d", rejectedResp.StatusCode)
	}
	rejectedResp.Body.Close()
}

// --- test helpers ------------------------------------------------------------

// cookieJar is a minimal http.CookieJar that merges cookies by name per host,
// since a response that doesn't touch a given cookie shouldn't erase it (the
// standard library only calls SetCookies with the cookies a response actually
// sent, not the full accumulated set).
type cookieJar struct {
	byHost map[string]map[string]*http.Cookie
}

func newCookieJar() *cookieJar {
	return &cookieJar{byHost: make(map[string]map[string]*http.Cookie)}
}

func (j *cookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	existing, ok := j.byHost[u.Host]
	if !ok {
		existing = make(map[string]*http.Cookie)
		j.byHost[u.Host] = existing
	}
	for _, cookie := range cookies {
		existing[cookie.Name] = cookie
	}
}

func (j *cookieJar) Cookies(u *url.URL) []*http.Cookie {
	existing := j.byHost[u.Host]
	cookies := make([]*http.Cookie, 0, len(existing))
	for _, cookie := range existing {
		cookies = append(cookies, cookie)
	}
	return cookies
}

func csrfTokenFromCookie(t *testing.T, jar *cookieJar, serverURL string) string {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	for _, cookie := range jar.Cookies(parsed) {
		if cookie.Name == "csrf_token" {
			return cookie.Value
		}
	}
	t.Fatal("no csrf_token cookie found; did a GET request run first?")
	return ""
}

func multipartCSV(t *testing.T, csv, csrfToken string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("csrf_token", csrfToken); err != nil {
		t.Fatalf("write csrf field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "expenses.csv")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte(csv)); err != nil {
		t.Fatalf("write csv content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func getBody(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d: %s", url, resp.StatusCode, body)
	}
	return body
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return string(data)
}

// categoryCheckboxPattern matches the dashboard's category filter checkboxes,
// e.g. `<input type="checkbox" name="category" value="7"> Casa</label>`.
var categoryCheckboxPattern = regexp.MustCompile(`name="category" value="(\d+)"[^>]*> ([^<]+)</label>`)

func extractCategoryOptionIDs(t *testing.T, html string) map[string]string {
	t.Helper()
	ids := make(map[string]string)
	for _, match := range categoryCheckboxPattern.FindAllStringSubmatch(html, -1) {
		ids[match[2]] = match[1]
	}
	if len(ids) == 0 {
		t.Fatalf("no category filter checkboxes found in: %s", html)
	}
	return ids
}

func extractHiddenValue(t *testing.T, html, fieldName string) string {
	t.Helper()
	pattern := regexp.MustCompile(`name="` + fieldName + `" value="([a-f0-9]+)"`)
	match := pattern.FindStringSubmatch(html)
	if match == nil {
		t.Fatalf("hidden field %q not found in: %s", fieldName, html)
	}
	return match[1]
}

func firstExpenseIDWithDescription(t *testing.T, db *sql.DB, description string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`SELECT id FROM expenses WHERE description = ? ORDER BY id LIMIT 1`, description).Scan(&id); err != nil {
		t.Fatalf("query expense by description %q: %v", description, err)
	}
	return id
}
