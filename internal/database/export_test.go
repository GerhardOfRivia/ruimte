package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/GerhardOfRivia/aarde/internal/api"
	"github.com/GerhardOfRivia/aarde/internal/catalog"
	"github.com/GerhardOfRivia/aarde/internal/geo"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportSnapshotAndFilters(t *testing.T) {
	ctx, _, service, cat := testDB(t)
	const shape = `{"type":"MultiPolygon","coordinates":[[[[0,0],[4,0],[4,4],[0,4],[0,0]],[[1,1],[1,2],[2,2],[2,1],[1,1]]],[[[5,5],[6,5],[6,6],[5,5]]]]}`
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	zero := 0.0
	for i := 0; i < 501; i++ {
		item := model(t, cat, fmt.Sprintf("image-%03d", i), shape)
		item.AcquiredAt = &when
		if i%2 == 0 {
			zero := 0.0
			item.CloudCover = &zero
		}
		if _, _, err := service.Import(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	other := model(t, cat+"-other", "image-001", shape)
	if _, _, err := service.Import(ctx, other); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		q     catalog.Query
		count int
	}{{catalog.Query{CatalogID: cat}, 501},
		{catalog.Query{ImageIDs: []string{"image-001"}}, 2},
		{catalog.Query{CatalogID: cat, CloudCoverLT: &zero}, 0},
		{catalog.Query{CatalogID: cat, CloudCoverLTE: &zero}, 251},
		{catalog.Query{CatalogID: cat, CloudCoverLTE: &zero, CloudCoverUnknown: "include"}, 501},
		{catalog.Query{CatalogID: cat, AcquiredBefore: &when}, 0},
		{catalog.Query{CatalogID: cat, AcquiredFrom: &when}, 501}, {catalog.Query{CatalogID: cat, CloudCoverUnknown: "only"}, 250}, {catalog.Query{CatalogID: cat, ImageIDs: []string{"image-001"}}, 1}, {catalog.Query{CatalogID: cat, Geometry: ptrGeometry(t, `{"type":"Polygon","coordinates":[[[1.2,1.2],[1.8,1.2],[1.8,1.8],[1.2,1.2]]]}`)}, 0}} {
		var out bytes.Buffer
		err := service.WriteFootprints(ctx, tc.q, &out)
		if tc.count == 0 {
			if err != catalog.ErrNotFound {
				t.Fatal(err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		var data struct {
			Features []struct {
				Geometry   json.RawMessage
				Properties catalog.Footprint
			}
		}
		if err := json.Unmarshal(out.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if len(data.Features) != tc.count {
			t.Fatalf("got %d want %d", len(data.Features), tc.count)
		}
		for _, f := range data.Features {
			var a, b any
			json.Unmarshal(f.Geometry, &a)
			json.Unmarshal([]byte(shape), &b)
			aa, _ := json.Marshal(a)
			bb, _ := json.Marshal(b)
			if !bytes.Equal(aa, bb) {
				t.Fatal("geometry changed")
			}
			if f.Properties.FileLocation != "/data/"+f.Properties.Filename {
				t.Fatal(f.Properties)
			}
		}
	}
	if _, err := exec.LookPath("ogrinfo"); err != nil {
		t.Skip("ogrinfo required for HTTP export inspection")
	}
	handler := api.Routes(service, "test", api.Access{Token: "test-token", PublicRead: true}, "none")
	for _, name := range []string{cat, cat + "-missing"} {
		req := httptest.NewRequest("POST", "/imagery/export/gpkg", strings.NewReader(fmt.Sprintf(`{"catalog_id":%q}`, name)))
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if name != cat {
			if w.Code != 404 {
				t.Fatalf("empty: %d %s", w.Code, w.Body.String())
			}
			continue
		}
		if w.Code != 200 {
			t.Fatalf("export: %d %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "application/geopackage+sqlite3" || !strings.Contains(w.Header().Get("Content-Disposition"), ".gpkg") {
			t.Fatal(w.Header())
		}
		path := filepath.Join(t.TempDir(), "export.gpkg")
		if err := os.WriteFile(path, w.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("ogrinfo", "-so", "-al", path).CombinedOutput()
		if err != nil || !strings.Contains(string(out), "Feature Count: 501") {
			t.Fatalf("%v: %s", err, out)
		}
	}

}

func ptrGeometry(t *testing.T, raw string) *geo.Geometry { g := geometry(t, raw); return &g }
