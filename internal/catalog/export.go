package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

const MaxExportRecords = 10000
const MaxExportBytes = 32 << 20

var ErrExportLimit = errors.New("export exceeds 10,000 records or 32 MiB; narrow the search")

type Footprint struct {
	Filename     string          `json:"filename"`
	CloudCover   *float64        `json:"cloud_cover"`
	FileLocation string          `json:"file_location"`
	Geometry     json.RawMessage `json:"-"`
}

// WriteFootprints uses the same normalized filters and topology validation as search.
func (s *Service) WriteFootprints(ctx context.Context, q Query, w io.Writer) error {
	q.Limit, q.Offset = 0, 0
	if err := NormalizeQuery(&q); err != nil {
		return err
	}
	if q.Geometry != nil {
		if err := s.store.ValidateGeometry(ctx, *q.Geometry); err != nil {
			return err
		}
	}
	count, size := 0, 0
	write := func(b []byte) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		size += len(b)
		if size > MaxExportBytes {
			return ErrExportLimit
		}
		_, err := w.Write(b)
		return err
	}
	if err := write([]byte(`{"type":"FeatureCollection","features":[`)); err != nil {
		return err
	}
	err := s.store.ExportFootprints(ctx, q, func(f Footprint) error {
		count++
		if count > MaxExportRecords {
			return ErrExportLimit
		}
		b, err := json.Marshal(struct {
			Type       string          `json:"type"`
			Geometry   json.RawMessage `json:"geometry"`
			Properties Footprint       `json:"properties"`
		}{"Feature", f.Geometry, f})
		if err != nil {
			return err
		}
		if count > 1 {
			b = append([]byte(","), b...)
		}
		return write(b)
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return write([]byte("]}"))
}
