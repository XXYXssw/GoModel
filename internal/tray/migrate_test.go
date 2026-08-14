package tray

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func createDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatalf("open source db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO t (v) VALUES ('keep-me')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close source db: %v", err)
	}
}

func assertRow(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		t.Fatalf("open migrated db: %v", err)
	}
	defer db.Close()
	var v string
	if err := db.QueryRow("SELECT v FROM t LIMIT 1").Scan(&v); err != nil {
		t.Fatalf("query migrated db: %v", err)
	}
	if v != "keep-me" {
		t.Fatalf("migrated value = %q, want %q", v, "keep-me")
	}
}

func TestMigrateDataDir_VacuumInto(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	createDB(t, filepath.Join(src, "gomodel.db"))

	if err := migrateDataDir(src, dest); err != nil {
		t.Fatalf("migrateDataDir() error: %v", err)
	}
	assertRow(t, filepath.Join(dest, "gomodel.db"))
}

func TestMigrateDataDir_CopiesAuxiliaryFiles(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	createDB(t, filepath.Join(src, "gomodel.db"))
	if err := os.WriteFile(filepath.Join(src, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "gomodel.pid"), []byte("12345\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := migrateDataDir(src, dest); err != nil {
		t.Fatalf("migrateDataDir() error: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "notes.txt")); err != nil || string(b) != "hello" {
		t.Fatalf("aux file not copied: %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "gomodel.pid")); !os.IsNotExist(err) {
		t.Fatalf("stale pid file must not be migrated, stat error = %v", err)
	}
}

func TestMigrateDataDir_KeepsExistingTarget(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	createDB(t, filepath.Join(src, "gomodel.db"))
	createDB(t, filepath.Join(dest, "gomodel.db"))
	// Put a marker in the target so an overwrite would be detectable.
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(dest, "gomodel.db")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO t (v) VALUES ('target-owned')"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := migrateDataDir(src, dest); err != nil {
		t.Fatalf("migrateDataDir() error: %v", err)
	}
	db, err = sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(dest, "gomodel.db"))+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM t WHERE v = 'target-owned'").Scan(&n); err != nil {
		t.Fatalf("query target: %v", err)
	}
	if n != 1 {
		t.Fatalf("existing target db was overwritten; target-owned rows = %d", n)
	}
}
