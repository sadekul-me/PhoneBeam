package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"phonebeam.dev/coordinator/internal/qr"
	"phonebeam.dev/coordinator/internal/session"
)

func testServer(t *testing.T) (*session.Coordinator, http.Handler) {
	t.Helper()
	coord := session.NewCoordinator("http://127.0.0.1:8080", 120*time.Second, 60*time.Minute)
	handler := New(coord, []string{"http://localhost:5173"}, log.New(io.Discard, "", 0)).Handler()
	return coord, handler
}

func TestHealth(t *testing.T) {
	_, h := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestCreateAndScanHTTP(t *testing.T) {
	_, h := testServer(t)
	body := `{"operator_display_name":"Op","requested_capabilities":["screen.read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatal("missing cors")
	}
	var created struct {
		OperatorToken string `json:"operator_token"`
		Session       struct {
			ID    string             `json:"id"`
			State string             `json:"state"`
			QR    *session.QRPayload `json:"qr"`
		} `json:"session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Session.State != "QR_AVAILABLE" || created.Session.QR == nil {
		t.Fatalf("%+v", created.Session)
	}
	encoded, err := json.Marshal(created.Session.QR)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := qr.Parse(string(encoded), []string{"http://127.0.0.1:8080"}, time.Now()); err != nil {
		t.Fatalf("qr parse: %v", err)
	}
	scanBody, _ := json.Marshal(map[string]string{"pid": created.Session.QR.PID})
	scanReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+created.Session.ID+"/scan", bytes.NewReader(scanBody))
	scanReq.Header.Set("Content-Type", "application/json")
	scanRec := httptest.NewRecorder()
	h.ServeHTTP(scanRec, scanReq)
	if scanRec.Code != 200 {
		t.Fatalf("scan=%d %s", scanRec.Code, scanRec.Body.String())
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	_, h := testServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/sessions", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestOversizedJSONRejected(t *testing.T) {
	_, h := testServer(t)
	huge := `{"operator_display_name":"` + strings.Repeat("a", 20_000) + `","requested_capabilities":["screen.read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(huge))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == 201 {
		t.Fatal("oversized body accepted")
	}
}
