package tray

import (
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // Pure Go SQLite driver, used for a consistent VACUUM INTO snapshot.

	"github.com/enterpilot/gomodel/internal/platformdir"
)

// migrateLegacyData moves a project-local ./data directory (the layout used
// when the CLI ran from a source checkout) into the OS-conventional per-user
// data directory that the menu-bar app uses. It runs once, before the gateway
// starts, and is a no-op when the target database already exists — the common
// case, because the CLI and the app resolve the same path (platformdir).
//
// The database is snapshotted with SQLite's VACUUM INTO so a WAL database is
// copied consistently even if the source directory is mid-write; auxiliary
// files (except stale pid files) are copied verbatim.
func migrateLegacyData() {
	targetDB := platformdir.DataFile("gomodel.db")
	if _, err := os.Stat(targetDB); err == nil {
		return // target already in use; nothing to migrate
	}

	for _, src := range legacyDataCandidates() {
		srcDB := filepath.Join(src, "gomodel.db")
		if _, err := os.Stat(srcDB); err != nil {
			continue
		}
		if err := migrateDataDir(src, filepath.Dir(targetDB)); err != nil {
			slog.Warn("legacy data migration failed", "source", src, "target", filepath.Dir(targetDB), "error", err)
			continue
		}
		slog.Info("migrated legacy data into the per-user data directory",
			"source", src, "target", filepath.Dir(targetDB))
		return
	}
}

// legacyDataCandidates are the places a previous build could have kept a
// project-local data directory: the current working directory (running the
// CLI from a checkout) and the directory holding the executable (running the
// tray binary straight from a build).
func legacyDataCandidates() []string {
	candidates := []string{filepath.Join(".", "data")}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "data"))
	}
	return candidates
}

// migrateDataDir copies the contents of src into dest, snapshotting
// gomodel.db with VACUUM INTO when possible.
func migrateDataDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	srcDB := filepath.Join(src, "gomodel.db")
	destDB := filepath.Join(dest, "gomodel.db")
	if _, err := os.Stat(destDB); err != nil {
		if err := vacuumInto(srcDB, destDB); err != nil {
			slog.Warn("VACUUM INTO failed; copying the database files verbatim", "source", srcDB, "error", err)
			if err := copyDBFiles(src, dest); err != nil {
				return err
			}
		}
	}

	return copyAuxiliaryFiles(src, dest)
}

// vacuumInto opens srcDB read-only and writes a consistent snapshot to destDB.
func vacuumInto(srcDB, destDB string) error {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(srcDB)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec("VACUUM INTO " + sqliteQuote(destDB)); err != nil {
		return err
	}
	return db.Close()
}

func sqliteQuote(path string) string {
	return "'" + strings.ReplaceAll(filepath.ToSlash(path), "'", "''") + "'"
}

// copyDBFiles falls back to copying gomodel.db with its WAL and shared-memory
// sidecars, in the order SQLite expects them.
func copyDBFiles(src, dest string) error {
	for _, name := range []string{"gomodel.db", "gomodel.db-wal", "gomodel.db-shm"} {
		if err := copyFile(filepath.Join(src, name), filepath.Join(dest, name)); err != nil {
			return err
		}
	}
	return nil
}

// copyAuxiliaryFiles copies every remaining regular file in src into dest,
// skipping files that already exist and stale pid files.
func copyAuxiliaryFiles(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".pid") || strings.HasPrefix(name, "gomodel.db") {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		return copyFile(path, filepath.Join(dest, rel))
	})
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
