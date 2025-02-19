package gohive

import (
	"database/sql/driver"

	hiveserver2 "github.com/mlycore/bgohive/hiveserver"
)

type hiveResult struct {
	insertId int64
	affected int64
}

func (r *hiveResult) LastInsertId() (int64, error) {
	return r.insertId, nil
}

func (r *hiveResult) RowsAffected() (int64, error) {
	return r.affected, nil
}

func newHiveResult(op *hiveserver2.TOperationHandle) driver.Result {
	var na int64 = -1
	if op.ModifiedRowCount != nil {
		na = int64(*op.ModifiedRowCount)
	}
	return &hiveResult{insertId: -1, affected: na}
}
