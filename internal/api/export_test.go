package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExportAccessAndValidation(t *testing.T) {
	h := Routes(nil, "test", Access{Token: "test-token", PublicRead: true}, "none")
	for _, tc := range []struct {
		token, body string
		status      int
	}{{"", "{}", 401}, {"wrong", "{}", 401}, {"test-token", `{"cloud_cover_lte":101}`, 400}, {"test-token", `null`, 400}, {"test-token", `{"geometry":null}`, 400}, {"test-token", `{"acquired_from":"bad"}`, 400}, {"test-token", `{"geometry":{"type":"Point","coordinates":[0,0]}}`, 400}, {"test-token", `{"limit":50}`, 400}, {"test-token", `{"surprise":true}`, 400}} {
		req := httptest.NewRequest("POST", "/imagery/export/gpkg", strings.NewReader(tc.body))
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.body, w.Code, w.Body.String())
		}
	}
}

func TestExportUnavailableAndBusy(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	h := Routes(nil, "test", Access{Token: "test-token"}, "none")
	check := func(message string) {
		req := httptest.NewRequest("POST", "/imagery/export/gpkg", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 503 || !strings.Contains(w.Body.String(), message) {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	check("requires ogr2ogr")
	for i := 0; i < cap(exportSlots); i++ {
		exportSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(exportSlots); i++ {
			<-exportSlots
		}
	}()
	check("busy")
}
