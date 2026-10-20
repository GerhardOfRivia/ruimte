package footprintexport

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	if _, err := exec.LookPath("ogr2ogr"); err != nil {
		t.Skip("ogr2ogr required")
	}
	for _, cloud := range []string{"null", "0", "12.5"} {
		t.Run(cloud, func(t *testing.T) {
			path, cleanup, err := Generate(context.Background(), func(w io.Writer) error {
				_, err := io.WriteString(w, `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"MultiPolygon","coordinates":[[[[0,0],[4,0],[4,4],[0,4],[0,0]],[[1,1],[1,2],[2,2],[2,1],[1,1]]],[[[5,5],[6,5],[6,6],[5,5]]]]},"properties":{"filename":"2026-01-01","file_location":"/data/a.tif","cloud_cover":`+cloud+`}}]}`)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			out, err := exec.Command("ogrinfo", "-al", path).CombinedOutput()
			if err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			for _, expected := range []string{"image_footprints", "Multi Polygon", "Feature Count: 1", `ID["EPSG",4326]`, "cloud_cover: Real", "filename (String) = 2026-01-01", "file_location (String) = /data/a.tif", "(1 1,1 2,2 2,2 1,1 1)", "(5 5,6 5,6 6,5 5)"} {
				if !strings.Contains(string(out), expected) {
					t.Errorf("missing %q in %s", expected, out)
				}
			}
			want := cloud
			if cloud == "null" {
				want = "(null)"
			}
			if !strings.Contains(string(out), "cloud_cover (Real) = "+want) {
				t.Fatal(string(out))
			}
			cleanup()
			if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
				t.Fatal("temporary directory remains")
			}
		})
	}
}

func TestFailureCleanup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if _, _, err := Generate(context.Background(), func(io.Writer) error { return nil }); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	for _, script := range []string{"#!/bin/sh\nexit 1\n", "#!/bin/sh\nexec /bin/sleep 30\n"} {
		if err := os.WriteFile(filepath.Join(bin, "ogr2ogr"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		_, _, err := Generate(ctx, func(w io.Writer) error { cancel(); return nil })
		cancel()
		if err == nil {
			t.Fatal("expected cancellation")
		}
		_, _, err = Generate(context.Background(), func(io.Writer) error { return errors.New("write failed") })
		if err == nil {
			t.Fatal("expected write failure")
		}
	}
	os.WriteFile(filepath.Join(bin, "ogr2ogr"), []byte("#!/bin/sh\nexit 1\n"), 0700)
	if _, _, err := Generate(context.Background(), func(io.Writer) error { return nil }); err == nil {
		t.Fatal("expected utility failure")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files remain: %v %v", entries, err)
	}
}
