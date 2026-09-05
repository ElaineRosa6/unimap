package history

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// Deliver one valid row before a driver failure: Query and Scan both succeed,
// so only checking Rows.Err after iteration can detect the incomplete read.
type iterationConnector struct{ terminal error }

func (c iterationConnector) Connect(context.Context) (driver.Conn, error) {
	return iterationConn{c.terminal}, nil
}
func (c iterationConnector) Driver() driver.Driver { return iterationDriver{c.terminal} }

type iterationDriver struct{ terminal error }

func (d iterationDriver) Open(string) (driver.Conn, error) { return iterationConn{d.terminal}, nil }

type iterationConn struct{ terminal error }

func (iterationConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected Prepare")
}
func (iterationConn) Close() error              { return nil }
func (iterationConn) Begin() (driver.Tx, error) { return nil, errors.New("unexpected Begin") }
func (c iterationConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.HasPrefix(query, "SELECT COUNT(*)") {
		return &iterationRows{columns: []string{"count"}, values: []driver.Value{int64(1)}, terminal: io.EOF}, nil
	}
	if strings.Contains(query, "FROM operation_results") {
		return &iterationRows{columns: []string{"id", "history_id", "data"}, values: []driver.Value{int64(1), int64(1), "{}"}, terminal: c.terminal}, nil
	}
	if strings.Contains(query, "FROM operation_history") {
		return &iterationRows{columns: []string{"id", "operation_type", "input", "status", "total_count", "summary", "duration_ms", "created_at"}, values: []driver.Value{int64(1), "query", "fixture", "success", int64(1), "", int64(0), time.Unix(0, 0)}, terminal: c.terminal}, nil
	}
	return nil, errors.New("unexpected query: " + query)
}

type iterationRows struct {
	columns   []string
	values    []driver.Value
	terminal  error
	delivered bool
}

func (r *iterationRows) Columns() []string { return r.columns }
func (*iterationRows) Close() error        { return nil }
func (r *iterationRows) Next(dest []driver.Value) error {
	if r.delivered {
		return r.terminal
	}
	copy(dest, r.values)
	r.delivered = true
	return nil
}

func TestHistoryIterationErrors(t *testing.T) {
	failure := errors.New("fixture late row failure")
	for _, terminal := range []error{failure, io.EOF} {
		t.Run(terminal.Error(), func(t *testing.T) {
			db := sql.OpenDB(iterationConnector{terminal})
			t.Cleanup(func() { _ = db.Close() })
			repo := NewRepository(db)
			for _, filter := range []string{"", "query"} {
				items, total, err := repo.ListHistory(filter, 20, 0)
				if terminal == failure {
					if !errors.Is(err, failure) || items != nil || total != 0 {
						t.Errorf("ListHistory(%q) = %v, %d, %v; want nil, 0, wrapped failure", filter, items, total, err)
					}
				} else if err != nil || len(items) != 1 || total != 1 {
					t.Errorf("ListHistory normal EOF = %v, %d, %v", items, total, err)
				}
			}
			results, err := repo.GetResults(1)
			if terminal == failure {
				if !errors.Is(err, failure) || results != nil {
					t.Errorf("GetResults = %v, %v; want nil, wrapped failure", results, err)
				}
			} else if err != nil || len(results) != 1 {
				t.Errorf("GetResults normal EOF = %v, %v", results, err)
			}
		})
	}
}
