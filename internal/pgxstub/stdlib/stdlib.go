package stdlib

import (
	"database/sql"
	"database/sql/driver"
	"errors"
)

const driverName = "pgx"

func init() {
	sql.Register(driverName, stubDriver{})
}

type stubDriver struct{}

type stubConn struct{}

type stubStmt struct{}

type stubTx struct{}

func (stubDriver) Open(name string) (driver.Conn, error) {
	return nil, errors.New("pgx stdlib stub: driver not available in offline build")
}

func (stubConn) Prepare(query string) (driver.Stmt, error) { return stubStmt{}, nil }
func (stubConn) Close() error                              { return nil }
func (stubConn) Begin() (driver.Tx, error)                 { return stubTx{}, nil }

func (stubStmt) Close() error { return nil }
func (stubStmt) NumInput() int { return -1 }
func (stubStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, errors.New("pgx stdlib stub: exec not supported")
}
func (stubStmt) Query(args []driver.Value) (driver.Rows, error) {
	return nil, errors.New("pgx stdlib stub: query not supported")
}

func (stubTx) Commit() error   { return nil }
func (stubTx) Rollback() error { return nil }
