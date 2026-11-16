// Package portable implements the stable garage-crm-export/1 data contract.
// Portable archives contain all persisted tables, including attachment bytes.
package portable

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jonathanselby/garage-crm/internal/app"
	"github.com/jonathanselby/garage-crm/internal/sqlite"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const Format = "garage-crm-export/1"
const maxArchiveBytes = int64(4 << 30)
const maxEntryBytes = int64(1 << 30)

type Item struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	Count  int    `json:"count"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	Format     string `json:"format_version"`
	App        string `json:"app_version"`
	Schema     int    `json:"schema_version"`
	ExportedAt string `json:"exported_at_utc"`
	Files      []Item `json:"files"`
}

func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Export(db *sqlite.DB, destination string) error {
	if _, err := os.Lstat(destination); err == nil {
		return errors.New("destination exists; refusing overwrite")
	} else if !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".garage-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = os.Chmod(f.Name(), 0600); err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	m := Manifest{Format: Format, App: "0.1.0-dev", Schema: 1, ExportedAt: time.Now().UTC().Format(time.RFC3339)}
	// Consistent snapshot: export reads a temporary online backup, not the live WAL.
	snap, err := os.CreateTemp(dir, ".garage-snapshot-*.db")
	if err != nil {
		return err
	}
	snapPath := snap.Name()
	snap.Close()
	os.Remove(snapPath)
	defer os.Remove(snapPath)
	if err := db.Backup(snapPath); err != nil {
		return err
	}
	snapshot, err := sqlite.Open(snapPath)
	if err != nil {
		return err
	}
	defer snapshot.Close()
	for _, table := range app.ExportTables {
		rows, err := snapshot.Query("SELECT * FROM " + table + " ORDER BY rowid")
		if err != nil {
			return fmt.Errorf("read %s: %w", table, err)
		}
		buf, err := json.Marshal(rows)
		if err != nil {
			return err
		}
		entry := "tables/" + table + ".json"
		w, err := zw.Create(entry)
		if err != nil {
			return err
		}
		if _, err = w.Write(buf); err != nil {
			return err
		}
		m.Files = append(m.Files, Item{Name: entry, Bytes: int64(len(buf)), Count: len(rows), SHA256: hash(buf)})
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	w, err := zw.Create("manifest.json")
	if err != nil {
		return err
	}
	if _, err = w.Write(b); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), destination); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}
func Inspect(path string) (Manifest, error) { m, _, err := readArchive(path); return m, err }
func readArchive(path string) (Manifest, map[string][]byte, error) {
	m := Manifest{}
	fi, err := os.Lstat(path)
	if err != nil {
		return m, nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.Mode().IsRegular() {
		return m, nil, errors.New("archive must be a regular file")
	}
	if fi.Size() > maxArchiveBytes {
		return m, nil, errors.New("archive too large")
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return m, nil, err
	}
	defer z.Close()
	payload := map[string][]byte{}
	seen := map[string]bool{}
	var total int64
	if len(z.File) != len(app.ExportTables)+1 {
		return m, nil, fmt.Errorf("archive has %d entries, expected %d", len(z.File), len(app.ExportTables)+1)
	}
	for _, file := range z.File {
		if strings.Contains(file.Name, "\\") || strings.Contains(file.Name, "..") || strings.HasPrefix(file.Name, "/") || strings.HasSuffix(file.Name, "/") || seen[file.Name] || file.Mode()&os.ModeSymlink != 0 {
			return m, nil, errors.New("unsafe or duplicate archive entry")
		}
		seen[file.Name] = true
		if file.Name != "manifest.json" {
			table := strings.TrimSuffix(strings.TrimPrefix(file.Name, "tables/"), ".json")
			if !strings.HasPrefix(file.Name, "tables/") || !strings.HasSuffix(file.Name, ".json") || !app.HasTable(table) {
				return m, nil, errors.New("unknown archive entry")
			}
		}
		if file.UncompressedSize64 > uint64(maxEntryBytes) {
			return m, nil, errors.New("entry too large")
		}
		total += int64(file.UncompressedSize64)
		if total > maxArchiveBytes {
			return m, nil, errors.New("unpacked data too large")
		}
		reader, err := file.Open()
		if err != nil {
			return m, nil, err
		}
		b, err := io.ReadAll(io.LimitReader(reader, maxEntryBytes+1))
		reader.Close()
		if err != nil {
			return m, nil, err
		}
		if int64(len(b)) > maxEntryBytes {
			return m, nil, errors.New("entry expansion exceeded budget")
		}
		payload[file.Name] = b
	}
	if err := json.Unmarshal(payload["manifest.json"], &m); err != nil {
		return m, nil, err
	}
	if m.Format != Format || m.Schema != 1 {
		return m, nil, errors.New("unsupported archive format or schema")
	}
	if len(m.Files) != len(app.ExportTables) {
		return m, nil, errors.New("incomplete table manifest")
	}
	manifestNames := map[string]bool{}
	for _, item := range m.Files {
		if manifestNames[item.Name] {
			return m, nil, errors.New("duplicate manifest file")
		}
		manifestNames[item.Name] = true
		b, ok := payload[item.Name]
		if !ok || int64(len(b)) != item.Bytes || hash(b) != item.SHA256 {
			return m, nil, fmt.Errorf("checksum/size mismatch: %s", item.Name)
		}
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(b, &rows); err != nil {
			return m, nil, fmt.Errorf("invalid rows in %s: %w", item.Name, err)
		}
		if len(rows) != item.Count {
			return m, nil, errors.New("manifest row count mismatch")
		}
	}
	return m, payload, nil
}
func ImportIntoEmpty(archive, databasePath string) error {
	if _, err := os.Lstat(databasePath); err == nil {
		return errors.New("target database exists; import requires a fresh location")
	} else if !os.IsNotExist(err) {
		return err
	}
	_, entries, err := readArchive(archive)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(databasePath), 0700); err != nil {
		return err
	}
	stage := databasePath + ".staging-" + app.NewID()
	defer os.Remove(stage)
	defer os.Remove(stage + "-wal")
	defer os.Remove(stage + "-shm")
	db, err := sqlite.Open(stage)
	if err != nil {
		return err
	}
	if err = app.Migrate(db); err != nil {
		db.Close()
		return err
	}
	columns := map[string]map[string]bool{}
	for _, table := range app.ExportTables {
		rows, e := db.Query("PRAGMA table_info(" + table + ")")
		if e != nil {
			db.Close()
			return e
		}
		columns[table] = map[string]bool{}
		for _, row := range rows {
			columns[table][fmt.Sprint(row["name"])] = true
		}
	}
	err = db.Tx(func(tx *sqlite.Session) error {
		if err := tx.Exec("PRAGMA defer_foreign_keys=ON"); err != nil {
			return err
		}
		for _, table := range app.ExportTables {
			dec := json.NewDecoder(strings.NewReader(string(entries["tables/"+table+".json"])))
			dec.UseNumber()
			var rows []map[string]any
			if err := dec.Decode(&rows); err != nil {
				return err
			}
			for _, row := range rows {
				cols := make([]string, 0, len(row))
				for col := range row {
					if !columns[table][col] {
						return fmt.Errorf("unknown column %s in %s", col, table)
					}
					cols = append(cols, col)
				}
				sort.Strings(cols)
				args := []any{}
				marks := []string{}
				for _, col := range cols {
					v := row[col]
					if table == "attachments" && col == "data" {
						encoded, ok := v.(string)
						if !ok {
							return errors.New("invalid attachment payload")
						}
						raw, e := base64.StdEncoding.DecodeString(encoded)
						if e != nil {
							return e
						}
						v = raw
					} else if n, ok := v.(json.Number); ok {
						parsed, e := n.Int64()
						if e != nil {
							return e
						}
						v = parsed
					}
					args = append(args, v)
					marks = append(marks, "?")
				}
				if err := tx.Exec("INSERT INTO "+table+"("+strings.Join(cols, ",")+") VALUES("+strings.Join(marks, ",")+")", args...); err != nil {
					return fmt.Errorf("import %s: %w", table, err)
				}
			}
		}
		checks, err := tx.Query("PRAGMA foreign_key_check")
		if err != nil {
			return err
		}
		if len(checks) != 0 {
			return fmt.Errorf("import violates %d foreign keys", len(checks))
		}
		return nil
	})
	if err != nil {
		db.Close()
		return err
	}
	if err = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		db.Close()
		return err
	}
	if err = db.Close(); err != nil {
		return err
	}
	if err = os.Rename(stage, databasePath); err != nil {
		return err
	}
	if d, e := os.Open(filepath.Dir(databasePath)); e == nil {
		d.Sync()
		d.Close()
	}
	return nil
}
