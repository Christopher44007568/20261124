package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stereonearclip/internal/stereo"
)

func TestSceneListAndReport(t *testing.T) {
	handler, err := New()
	if err != nil {
		t.Fatal(err)
	}

	scenesResponse := httptest.NewRecorder()
	handler.ServeHTTP(scenesResponse, httptest.NewRequest(http.MethodGet, "/api/scenes", nil))
	if scenesResponse.Code != http.StatusOK {
		t.Fatalf("scene list status=%d", scenesResponse.Code)
	}
	var scenes []stereo.SceneInfo
	if err := json.Unmarshal(scenesResponse.Body.Bytes(), &scenes); err != nil {
		t.Fatal(err)
	}
	if len(scenes) != len(stereo.SceneIDs) {
		t.Fatalf("got %d scenes, want %d", len(scenes), len(stereo.SceneIDs))
	}

	reportResponse := httptest.NewRecorder()
	handler.ServeHTTP(reportResponse, httptest.NewRequest(http.MethodGet, "/api/report?scene=large-plane", nil))
	if reportResponse.Code != http.StatusOK {
		t.Fatalf("report status=%d body=%s", reportResponse.Code, reportResponse.Body.String())
	}
	var report stereo.SceneReport
	if err := json.Unmarshal(reportResponse.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.ID != "large-plane" {
		t.Fatalf("report id=%q", report.ID)
	}
	if len(report.Eyes) != 2 {
		t.Fatalf("got %d eyes", len(report.Eyes))
	}
	if report.Eyes[0].Signature == report.Eyes[1].Signature {
		t.Fatal("HTTP report copied one eye into the other")
	}
	if !report.Comparison.DistinctNearCenters || !report.Comparison.DistinctClipResults {
		t.Fatal("HTTP report lost per-eye independence")
	}
}

func TestInvalidSceneAndMethods(t *testing.T) {
	handler, err := New()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		method string
		target string
		status int
	}{
		{http.MethodGet, "/api/report?scene=missing", http.StatusBadRequest},
		{http.MethodPost, "/api/report?scene=large-plane", http.StatusMethodNotAllowed},
	}
	for _, tc := range tests {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.target, nil))
		if rec.Code != tc.status {
			t.Errorf("%s %s status=%d, want %d", tc.method, tc.target, rec.Code, tc.status)
		}
	}
}

func TestEmbeddedPageAndHeaders(t *testing.T) {
	handler, err := New()
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "逐眼近裁剪对照台") {
		t.Fatal("index page did not contain the expected heading")
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
		t.Fatalf("missing CSP header: %q", got)
	}
}
