package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

type exportStore struct {
	Store
	count    int
	filename string
}

func (s exportStore) ExportFootprints(ctx context.Context, q Query, visit func(Footprint) error) error {
	for i := 0; i < s.count; i++ {
		if err := visit(Footprint{Filename: s.filename, Geometry: json.RawMessage(`{"type":"MultiPolygon","coordinates":[]}`)}); err != nil {
			return err
		}
	}
	return nil
}
func TestExportBounds(t *testing.T) {
	for _, tc := range []struct {
		count int
		name  string
		want  error
	}{{0, "", ErrNotFound}, {501, "", nil}, {MaxExportRecords + 1, "", ErrExportLimit}, {1, strings.Repeat("x", MaxExportBytes), ErrExportLimit}} {
		err := New(exportStore{count: tc.count, filename: tc.name}).WriteFootprints(context.Background(), Query{Limit: 1, Offset: 99}, io.Discard)
		if !errors.Is(err, tc.want) {
			t.Fatalf("count %d: %v", tc.count, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := New(exportStore{}).WriteFootprints(ctx, Query{}, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
