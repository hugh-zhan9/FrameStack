package volumes_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"idea/internal/volumes"
)

func TestPostgresStoreListVolumesReturnsItems(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticVolumeRows{
			items: []volumes.Volume{
				{ID: 7, DisplayName: "Media", MountPath: "/Volumes/media", IsOnline: true},
			},
		},
	}
	store := volumes.PostgresStore{Rows: queryer}

	items, err := store.ListVolumes(context.Background())
	if err != nil {
		t.Fatalf("expected list to succeed: %v", err)
	}
	if len(items) != 1 || items[0].DisplayName != "Media" {
		t.Fatalf("unexpected volumes: %#v", items)
	}
	if !strings.Contains(normalizeSQL(queryer.query), normalizeSQL("from volumes")) {
		t.Fatalf("unexpected query: %s", queryer.query)
	}
}

func TestPostgresStoreCreateVolumeReturnsCreatedVolume(t *testing.T) {
	queryer := &recordingRowQueryer{
		row: staticVolumeRow{
			volume: volumes.Volume{ID: 8, DisplayName: "Disk 2", MountPath: "/Volumes/disk2", IsOnline: true},
		},
	}
	store := volumes.PostgresStore{Queryer: queryer}

	item, err := store.CreateVolume(context.Background(), volumes.CreateVolumeInput{
		DisplayName: "Disk 2",
		MountPath:   "/Volumes/disk2",
	})
	if err != nil {
		t.Fatalf("expected create to succeed: %v", err)
	}
	if item.ID != 8 || item.MountPath != "/Volumes/disk2" {
		t.Fatalf("unexpected created volume: %#v", item)
	}
}

func TestPostgresStoreCreateVolumeRejectsMissingFields(t *testing.T) {
	store := volumes.PostgresStore{}

	_, err := store.CreateVolume(context.Background(), volumes.CreateVolumeInput{})
	if err == nil {
		t.Fatal("expected create to fail")
	}
}

func TestPostgresStoreListVolumesReturnsQueryError(t *testing.T) {
	store := volumes.PostgresStore{
		Rows: &recordingRowsQueryer{err: errors.New("db down")},
	}

	_, err := store.ListVolumes(context.Background())
	if err == nil {
		t.Fatal("expected list to fail")
	}
}

func TestPostgresStoreDeleteVolumeRemovesVolumeAndFiles(t *testing.T) {
	execer := &recordingExecer{}
	store := volumes.PostgresStore{Execer: execer}

	err := store.DeleteVolume(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected delete to succeed: %v", err)
	}
	if len(execer.args) != 1 || execer.args[0] != int64(7) {
		t.Fatalf("unexpected delete args: %#v", execer.args)
	}
	if !strings.Contains(normalizeSQL(execer.query), normalizeSQL("delete from files")) {
		t.Fatalf("expected file cleanup query, got %s", execer.query)
	}
	if !strings.Contains(normalizeSQL(execer.query), normalizeSQL("delete from volumes")) {
		t.Fatalf("expected volume delete query, got %s", execer.query)
	}
}

func TestPostgresStoreDeleteVolumeRejectsMissingID(t *testing.T) {
	store := volumes.PostgresStore{}

	err := store.DeleteVolume(context.Background(), 0)
	if err == nil {
		t.Fatal("expected delete to fail")
	}
}

type recordingRowsQueryer struct {
	query string
	args  []any
	rows  volumes.RowsScanner
	err   error
}

func (r *recordingRowsQueryer) QueryContext(_ context.Context, query string, args ...any) (volumes.RowsScanner, error) {
	r.query = query
	r.args = args
	if r.err != nil {
		return nil, r.err
	}
	return r.rows, nil
}

type recordingRowQueryer struct {
	query string
	args  []any
	row   staticVolumeRow
}

func (r *recordingRowQueryer) QueryRowContext(_ context.Context, query string, args ...any) volumes.RowScanner {
	r.query = query
	r.args = args
	return r.row
}

type recordingExecer struct {
	query string
	args  []any
	err   error
}

func (r *recordingExecer) ExecContext(_ context.Context, query string, args ...any) error {
	r.query = query
	r.args = args
	return r.err
}

type staticVolumeRows struct {
	items []volumes.Volume
	index int
}

func (r *staticVolumeRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticVolumeRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.ID
	*dest[1].(*string) = item.DisplayName
	*dest[2].(*string) = item.MountPath
	*dest[3].(*bool) = item.IsOnline
	return nil
}

func (r *staticVolumeRows) Err() error   { return nil }
func (r *staticVolumeRows) Close() error { return nil }

type staticVolumeRow struct {
	volume volumes.Volume
	err    error
}

func (r staticVolumeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.volume.ID
	*dest[1].(*string) = r.volume.DisplayName
	*dest[2].(*string) = r.volume.MountPath
	*dest[3].(*bool) = r.volume.IsOnline
	return nil
}

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
