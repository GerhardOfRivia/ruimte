package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/GerhardOfRivia/aarde/internal/catalog"
	"github.com/GerhardOfRivia/aarde/internal/footprintexport"
)

var exportSlots = make(chan struct{}, 2)

func (h Handler) export(w http.ResponseWriter, r *http.Request, q catalog.Query) {
	if err := catalog.NormalizeQuery(&q); err != nil {
		Error(w, 400, "invalid_request", err.Error())
		return
	}
	select {
	case exportSlots <- struct{}{}:
		defer func() { <-exportSlots }()
	default:
		Error(w, 503, "unavailable", "footprint exports are busy; retry later")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	path, cleanup, err := footprintexport.Generate(ctx, func(w io.Writer) error { return h.catalog.WriteFootprints(ctx, q, w) })
	if err != nil {
		switch {
		case errors.Is(err, catalog.ErrExportLimit):
			Error(w, 413, "export_too_large", err.Error())
		case errors.Is(err, catalog.ErrNotFound):
			Error(w, 404, "not_found", "No footprints to export.")
		case errors.Is(err, footprintexport.ErrUnavailable):
			Error(w, 503, "unavailable", "GeoPackage export requires ogr2ogr.")
		default:
			failure(w, err)
		}
		return
	}
	defer cleanup()
	w.Header().Set("Content-Type", "application/geopackage+sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="aarde-footprints-`+time.Now().UTC().Format("20060102-150405")+`.gpkg"`)
	http.ServeFile(w, r, path)
}
