// Package sqlite is a deliberately small, direct binding to the installed
// libsqlite3. No SQL strings are constructed from untrusted field names.
// On Linux the executable uses the system SQLite library; distribution must
// bundle or declare this runtime dependency.
package sqlite

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>
*/
import "C"
import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

type Row map[string]any
type DB struct {
	mu   sync.Mutex
	ptr  *C.sqlite3
	path string
}
type Session struct{ db *DB }

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	var p *C.sqlite3
	rc := C.sqlite3_open_v2(c, &p, C.SQLITE_OPEN_READWRITE|C.SQLITE_OPEN_CREATE|C.SQLITE_OPEN_FULLMUTEX, nil)
	if rc != C.SQLITE_OK {
		s := "SQLite open error"
		if p != nil {
			s = C.GoString(C.sqlite3_errmsg(p))
			C.sqlite3_close(p)
		}
		return nil, errors.New(s)
	}
	d := &DB{ptr: p, path: path}
	if err := d.Exec(`PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA busy_timeout=5000; PRAGMA secure_delete=ON;`); err != nil {
		d.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ptr == nil {
		return nil
	}
	rc := C.sqlite3_close(d.ptr)
	if rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite close %d", rc)
	}
	d.ptr = nil
	return nil
}
func (d *DB) Path() string { return d.path }
func (d *DB) Err() error   { return errors.New(C.GoString(C.sqlite3_errmsg(d.ptr))) }
func (s *Session) exec(sql string, args ...any) (int64, error) {
	return s.db.execute(sql, args, false, nil)
}
func (s *Session) query(sql string, args ...any) ([]Row, error) {
	var rows []Row
	_, err := s.db.execute(sql, args, true, &rows)
	return rows, err
}
func (d *DB) Exec(sql string, args ...any) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.execute(sql, args, false, nil)
	return err
}
func (d *DB) Run(sql string, args ...any) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.execute(sql, args, false, nil)
}
func (d *DB) Query(sql string, args ...any) ([]Row, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var rows []Row
	_, err := d.execute(sql, args, true, &rows)
	return rows, err
}
func (d *DB) Tx(fn func(*Session) error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.execute("BEGIN IMMEDIATE", nil, false, nil); err != nil {
		return err
	}
	defer d.execute("ROLLBACK", nil, false, nil)
	if err := fn(&Session{d}); err != nil {
		return err
	}
	_, err := d.execute("COMMIT", nil, false, nil)
	return err
}
func (s *Session) Exec(sql string, args ...any) error           { _, err := s.exec(sql, args...); return err }
func (s *Session) Run(sql string, args ...any) (int64, error)   { return s.exec(sql, args...) }
func (s *Session) Query(sql string, args ...any) ([]Row, error) { return s.query(sql, args...) }
func (d *DB) execute(sql string, args []any, collect bool, out *[]Row) (int64, error) {
	// sqlite3_prepare_v2 prepares one statement at a time. For schema creation
	// allow trailing statements only in the zero-argument Exec path.
	if len(args) == 0 && !collect && (len(sql) > 0 && (sql[0] == 'P' || sql[0] == 'C' || sql[0] == 'A')) {
		c := C.CString(sql)
		defer C.free(unsafe.Pointer(c))
		var errmsg *C.char
		if C.sqlite3_exec(d.ptr, c, nil, nil, &errmsg) != C.SQLITE_OK {
			err := C.GoString(errmsg)
			C.sqlite3_free(unsafe.Pointer(errmsg))
			return 0, errors.New(err)
		}
		return int64(C.sqlite3_changes(d.ptr)), nil
	}
	c := C.CString(sql)
	defer C.free(unsafe.Pointer(c))
	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(d.ptr, c, -1, &stmt, nil); rc != C.SQLITE_OK {
		return 0, d.Err()
	}
	defer C.sqlite3_finalize(stmt)
	if int(C.sqlite3_bind_parameter_count(stmt)) != len(args) {
		return 0, fmt.Errorf("SQL argument mismatch: expected %d got %d", int(C.sqlite3_bind_parameter_count(stmt)), len(args))
	}
	for i, v := range args {
		idx := C.int(i + 1)
		var rc C.int
		switch v := v.(type) {
		case nil:
			rc = C.sqlite3_bind_null(stmt, idx)
		case string:
			cs := C.CString(v)
			rc = C.sqlite3_bind_text(stmt, idx, cs, C.int(len(v)), C.SQLITE_TRANSIENT)
			C.free(unsafe.Pointer(cs))
		case int:
			rc = C.sqlite3_bind_int64(stmt, idx, C.sqlite3_int64(v))
		case int64:
			rc = C.sqlite3_bind_int64(stmt, idx, C.sqlite3_int64(v))
		case bool:
			if v {
				rc = C.sqlite3_bind_int64(stmt, idx, 1)
			} else {
				rc = C.sqlite3_bind_int64(stmt, idx, 0)
			}
		case []byte:
			if len(v) == 0 {
				rc = C.sqlite3_bind_zeroblob(stmt, idx, 0)
			} else {
				rc = C.sqlite3_bind_blob(stmt, idx, unsafe.Pointer(&v[0]), C.int(len(v)), C.SQLITE_TRANSIENT)
			}
		default:
			return 0, fmt.Errorf("unsupported SQLite argument type %T", v)
		}
		if rc != C.SQLITE_OK {
			return 0, d.Err()
		}
	}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			break
		}
		if rc != C.SQLITE_ROW {
			return 0, d.Err()
		}
		if collect {
			row := Row{}
			n := int(C.sqlite3_column_count(stmt))
			for i := 0; i < n; i++ {
				idx := C.int(i)
				name := C.GoString(C.sqlite3_column_name(stmt, idx))
				switch C.sqlite3_column_type(stmt, idx) {
				case C.SQLITE_INTEGER:
					row[name] = int64(C.sqlite3_column_int64(stmt, idx))
				case C.SQLITE_FLOAT:
					row[name] = float64(C.sqlite3_column_double(stmt, idx))
				case C.SQLITE_BLOB:
					p := C.sqlite3_column_blob(stmt, idx)
					row[name] = C.GoBytes(p, C.sqlite3_column_bytes(stmt, idx))
				case C.SQLITE_TEXT:
					row[name] = C.GoString((*C.char)(unsafe.Pointer(C.sqlite3_column_text(stmt, idx))))
				default:
					row[name] = nil
				}
			}
			*out = append(*out, row)
		}
	}
	return int64(C.sqlite3_changes(d.ptr)), nil
}
func (d *DB) Backup(path string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Exclusive 0600 creation *before* SQLite opens the destination prevents
	// an exposure window in shared directories. The destination must not exist.
	destFile, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err = destFile.Close(); err != nil {
		os.Remove(path)
		return err
	}
	valid := false
	defer func() {
		if !valid {
			os.Remove(path)
		}
	}()
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	var dest *C.sqlite3
	if rc := C.sqlite3_open_v2(c, &dest, C.SQLITE_OPEN_READWRITE|C.SQLITE_OPEN_FULLMUTEX, nil); rc != C.SQLITE_OK {
		if dest != nil {
			C.sqlite3_close(dest)
		}
		return fmt.Errorf("backup open %d", rc)
	}
	main := C.CString("main")
	defer C.free(unsafe.Pointer(main))
	b := C.sqlite3_backup_init(dest, main, d.ptr, main)
	if b == nil {
		C.sqlite3_close(dest)
		return errors.New("backup init failed")
	}
	rc := C.sqlite3_backup_step(b, -1)
	finish := C.sqlite3_backup_finish(b)
	closeCode := C.sqlite3_close(dest)
	if rc != C.SQLITE_DONE || finish != C.SQLITE_OK || closeCode != C.SQLITE_OK {
		return fmt.Errorf("backup failed %d/%d/%d", rc, finish, closeCode)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	err = f.Sync()
	f.Close()
	if err != nil {
		return err
	}
	valid = true
	return nil
}
func Checksum(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
