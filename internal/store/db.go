package store

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"jenkinsAgent/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}

	// Pre-migration: handle SQLite schema changes for test_envs
	if db.Migrator().HasTable("test_envs") {
		// Check if old host column exists (pre-migration state)
		hasHost := db.Migrator().HasColumn(&model.TestEnv{}, "Host")
		// Check if table was incorrectly migrated (missing AUTOINCREMENT)
		var tableSQL string
		db.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name='test_envs'").Scan(&tableSQL)
		needsRebuild := hasHost || !strings.Contains(tableSQL, "AUTOINCREMENT")

		if needsRebuild {
			log.Printf("[store] migrating test_envs: rebuilding table with proper schema")
			db.Exec(`CREATE TABLE IF NOT EXISTS test_envs_fixed (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL DEFAULT '',
				url TEXT NOT NULL DEFAULT '',
				script_content TEXT,
				last_run_status TEXT DEFAULT 'idle',
				last_run_output TEXT,
				last_run_at DATETIME,
				created_at DATETIME,
				updated_at DATETIME
			)`)
			if hasHost {
				db.Exec(`INSERT INTO test_envs_fixed (id, name, url, script_content, last_run_status, last_run_output, last_run_at, created_at, updated_at) SELECT id, name, '', script_content, last_run_status, last_run_output, last_run_at, created_at, updated_at FROM test_envs`)
			} else {
				db.Exec(`INSERT INTO test_envs_fixed (id, name, url, script_content, last_run_status, last_run_output, last_run_at, created_at, updated_at) SELECT id, name, url, script_content, last_run_status, last_run_output, last_run_at, created_at, updated_at FROM test_envs`)
			}
			db.Exec(`DROP TABLE test_envs`)
			db.Exec(`ALTER TABLE test_envs_fixed RENAME TO test_envs`)
		}
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.Product{},
		&model.Component{},
		&model.Release{},
		&model.ReleaseComponent{},
		&model.Build{},
		&model.ConfigItem{},
		&model.SysConfig{},
		&model.TestEnv{},
		&model.TestEnvScript{},
		&model.ProductTestEnv{},
		&model.Artifact{},
		&model.FAQ{},
	); err != nil {
		return err
	}

	DB = db
	log.Printf("[store] database initialized at %s", dbPath)
	return nil
}
