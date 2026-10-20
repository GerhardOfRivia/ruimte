// Package footprintexport converts bounded, stored catalog geometry to GeoPackage.
package footprintexport

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var ErrUnavailable = errors.New("ogr2ogr is unavailable")

// Generate owns its temporary directory until cleanup is called by the recipient.
func Generate(ctx context.Context, write func(io.Writer) error) (path string, cleanup func(), err error) {
	tool, err := exec.LookPath("ogr2ogr")
	if err != nil {
		return "", nil, ErrUnavailable
	}
	dir, err := os.MkdirTemp("", "aarde-footprints-")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.RemoveAll(dir) }
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	input := filepath.Join(dir, "footprints.geojson")
	f, err := os.Create(input)
	if err != nil {
		return "", cleanup, err
	}
	err = write(f)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", cleanup, err
	}
	// Explicit VRT fields prevent all-null cloud values from becoming text.
	vrt := `<OGRVRTDataSource><OGRVRTLayer name="image_footprints"><SrcDataSource relativeToVRT="1">footprints.geojson</SrcDataSource><SrcLayer>footprints</SrcLayer><LayerSRS>EPSG:4326</LayerSRS><GeometryType>wkbMultiPolygon</GeometryType><Field name="filename" type="String"/><Field name="cloud_cover" type="Real"/><Field name="file_location" type="String"/></OGRVRTLayer></OGRVRTDataSource>`
	source := filepath.Join(dir, "source.vrt")
	if err = os.WriteFile(source, []byte(vrt), 0600); err != nil {
		return "", cleanup, err
	}
	path = filepath.Join(dir, "footprints.gpkg")
	cmd := exec.CommandContext(ctx, tool, "--config", "GDAL_PAM_ENABLED", "NO", "--config", "OGR_GEOJSON_DATE_AS_STRING", "YES", "--config", "OGR_SQLITE_PRAGMA", "page_size=4096,max_page_count=32768", "-f", "GPKG", path, source, "-nln", "image_footprints", "-nlt", "MULTIPOLYGON", "-a_srs", "EPSG:4326")
	// Discard utility output: it can contain sensitive paths and is otherwise unbounded.
	if err = cmd.Run(); err != nil {
		return "", cleanup, err
	}
	return path, cleanup, nil
}
