package httpapi

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"testing"
)

type fakeRows struct {
	pgx.Rows
	values  []Variant
	index   int
	scanErr error
	rowsErr error
	closed  bool
}

func (r *fakeRows) Next() bool { r.index++; return r.index <= len(r.values) }
func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	v := r.values[r.index-1]
	*(dest[0].(*string)) = v.SportCode
	*(dest[1].(*string)) = v.Sport
	*(dest[2].(*string)) = v.Variant
	*(dest[3].(*int)) = v.Players
	return nil
}
func (r *fakeRows) Close()     { r.closed = true }
func (r *fakeRows) Err() error { return r.rowsErr }

type fakeDatabase struct {
	pingErr  error
	queryErr error
	rows     *fakeRows
	queries  int
}

func (d *fakeDatabase) Ping(context.Context) error { return d.pingErr }
func (d *fakeDatabase) Query(context.Context, string, ...any) (pgx.Rows, error) {
	d.queries++
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	return d.rows, nil
}

func TestPostgresStoreSportsHandlesAllFailureStages(t *testing.T) {
	failure := errors.New("database failure")
	for _, tc := range []struct {
		name                       string
		queryErr, scanErr, rowsErr error
	}{
		{name: "success"}, {name: "query", queryErr: failure}, {name: "scan", scanErr: failure}, {name: "iteration", rowsErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := &fakeRows{values: []Variant{{SportCode: "futsal", Players: 5}}, scanErr: tc.scanErr, rowsErr: tc.rowsErr}
			db := &fakeDatabase{queryErr: tc.queryErr, rows: rows}
			got, err := NewPostgresStore(db).Sports(context.Background())
			wantErr := tc.queryErr != nil || tc.scanErr != nil || tc.rowsErr != nil
			if (err != nil) != wantErr {
				t.Fatalf("error=%v wantErr=%v", err, wantErr)
			}
			if !wantErr && (len(got) != 1 || got[0].Players != 5) {
				t.Fatal("catalog row was not decoded")
			}
			if tc.queryErr == nil && !rows.closed {
				t.Fatal("rows were not closed")
			}
			if wantErr && got != nil {
				t.Fatal("a partial catalog must not be returned")
			}
		})
	}
}

func TestPostgresReadinessRequiresSchemaAndSeed(t *testing.T) {
	failure := errors.New("unavailable")
	for _, tc := range []struct {
		name                                string
		pingErr, queryErr, scanErr, rowsErr error
		empty                               bool
	}{
		{name: "healthy"}, {name: "database unavailable", pingErr: failure}, {name: "schema missing", queryErr: failure},
		{name: "seed missing", empty: true}, {name: "decode failure", scanErr: failure}, {name: "read failure", rowsErr: failure, empty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := &fakeRows{scanErr: tc.scanErr, rowsErr: tc.rowsErr}
			if !tc.empty {
				rows.values = []Variant{{SportCode: "futsal", Players: 5}}
			}
			db := &fakeDatabase{pingErr: tc.pingErr, queryErr: tc.queryErr, rows: rows}
			err := NewPostgresStore(db).Ready(context.Background())
			if (err == nil) != (tc.name == "healthy") {
				t.Fatalf("unexpected readiness error: %v", err)
			}
			if tc.pingErr != nil && db.queries != 0 {
				t.Fatal("schema queried after connection failure")
			}
			if tc.pingErr == nil && tc.queryErr == nil && !rows.closed {
				t.Fatal("readiness rows were not closed")
			}
		})
	}
}
