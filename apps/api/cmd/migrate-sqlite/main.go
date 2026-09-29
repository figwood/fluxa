package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"fluxa-api/internal/config"
	"fluxa-api/internal/infrastructure/persistence/gormdb"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type tableMapping struct{ source, target string }

var tables = []tableMapping{
	{"mvp_projects", "projects"},
	{"mvp_gitlab_repositories", "gitlab_repositories"},
	{"mvp_project_gitlab_repositories", "project_gitlab_repositories"},
	{"mvp_project_services", "project_services"},
	{"mvp_users", "users"},
	{"mvp_sys_roles", "sys_roles"},
	{"mvp_sys_role_members", "sys_role_members"},
	{"mvp_privileges", "privileges"},
	{"mvp_role_privileges", "role_privileges"},
	{"mvp_project_role_defs", "project_role_defs"},
	{"mvp_project_members", "project_members"},
	{"mvp_tasks", "tasks"},
	{"mvp_task_events", "task_events"},
	{"mvp_artifacts", "artifacts"},
	{"mvp_environment_policies", "environment_policies"},
	{"mvp_project_ci_tokens", "project_ci_tokens"},
	{"mvp_releases", "releases"},
	{"mvp_release_tasks", "release_tasks"},
	{"mvp_release_services", "release_services"},
	{"mvp_release_jobs", "release_jobs"},
	{"mvp_release_events", "release_events"},
	{"mvp_deployments", "deployments"},
	{"mvp_worker_settings", "worker_settings"},
	{"mvp_worker_heartbeats", "worker_heartbeats"},
	{"mvp_project_workflows", "project_workflows"},
	{"mvp_project_workflow_stages", "project_workflow_stages"},
	{"mvp_project_workflow_statuses", "project_workflow_statuses"},
	{"mvp_project_workflow_transitions", "project_workflow_transitions"},
}

func main() {
	defaultPath := findSQLite()
	sourcePath := flag.String("sqlite", defaultPath, "path to the source SQLite database")
	replace := flag.Bool("replace", false, "replace existing PostgreSQL application data with SQLite data")
	flag.Parse()
	if *sourcePath == "" {
		log.Fatal("SQLite source not found; pass -sqlite")
	}
	if _, err := os.Stat(*sourcePath); err != nil {
		log.Fatal(err)
	}

	source, err := gorm.Open(sqlite.Open(*sourcePath), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	cfg := config.Load()
	cfg.DBDriver = "postgres"
	cfg.AutoMigrate = true
	target, err := gormdb.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}

	total := 0
	err = target.Transaction(func(tx *gorm.DB) error {
		if *replace {
			if err := clearTarget(tx); err != nil {
				return err
			}
		}
		for _, mapping := range tables {
			count, err := copyTable(source, tx, mapping)
			if err != nil {
				return fmt.Errorf("%s -> %s: %w", mapping.source, mapping.target, err)
			}
			if count > 0 {
				log.Printf("%-42s -> %-36s %d rows", mapping.source, mapping.target, count)
			}
			total += count
		}
		return resetSequences(tx)
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("SQLite migration completed: %d source rows processed", total)
}

func clearTarget(db *gorm.DB) error {
	// The source database is authoritative in replace mode. TRUNCATE is part of
	// the surrounding transaction, so any later copy error restores everything.
	tables := []string{
		"workflow_events", "approval_tasks", "workflow_step_executions", "workflow_executions",
		"release_events", "release_jobs", "release_services", "release_tasks", "releases",
		"deployments", "artifacts", "task_events", "tasks",
		"project_workflow_transitions", "project_workflow_statuses", "project_workflow_stages", "project_workflows",
		"environment_policies", "project_ci_tokens", "project_services",
		"project_members", "project_role_defs", "role_privileges", "sys_role_members", "privileges", "sys_roles", "users",
		"project_gitlab_repositories", "gitlab_repositories", "worker_heartbeats", "worker_settings", "projects",
	}
	return db.Exec("TRUNCATE TABLE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE").Error
}

func copyTable(source, target *gorm.DB, mapping tableMapping) (int, error) {
	if !source.Migrator().HasTable(mapping.source) {
		return 0, nil
	}
	if !target.Migrator().HasTable(mapping.target) {
		return 0, fmt.Errorf("target table does not exist")
	}
	sourceColumns, err := columnNames(source, mapping.source)
	if err != nil {
		return 0, err
	}
	targetTypes, err := targetColumnTypes(target, mapping.target)
	if err != nil {
		return 0, err
	}
	columns := make([]string, 0, len(sourceColumns))
	for _, column := range sourceColumns {
		if _, ok := targetTypes[column]; ok {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return 0, nil
	}
	quoted := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = `"` + strings.ReplaceAll(column, `"`, `""`) + `"`
	}
	sqlDB, err := source.DB()
	if err != nil {
		return 0, err
	}
	rows, err := sqlDB.Query(`SELECT ` + strings.Join(quoted, ",") + ` FROM "` + strings.ReplaceAll(mapping.source, `"`, `""`) + `"`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	primaryKey := "id"
	if mapping.target == "project_ci_tokens" {
		primaryKey = "project_id"
	}
	if mapping.target == "worker_settings" {
		primaryKey = "key"
	}
	updates := make([]string, 0, len(columns)-1)
	for _, column := range columns {
		if column != primaryKey {
			updates = append(updates, column)
		}
	}
	for rows.Next() {
		raw := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return count, err
		}
		record := map[string]any{}
		for i, column := range columns {
			record[column] = normalize(raw[i], targetTypes[column])
		}
		conflict := clause.OnConflict{Columns: []clause.Column{{Name: primaryKey}}, DoUpdates: clause.AssignmentColumns(updates)}
		if err := target.Table(mapping.target).Clauses(conflict).Create(record).Error; err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func columnNames(db *gorm.DB, table string) ([]string, error) {
	rows, err := db.Raw(`PRAGMA table_info("` + strings.ReplaceAll(table, `"`, `""`) + `")`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var cid int
		var name, kind string
		var notnull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notnull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
func targetColumnTypes(db *gorm.DB, table string) (map[string]string, error) {
	types, err := db.Migrator().ColumnTypes(table)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, column := range types {
		out[column.Name()] = strings.ToLower(column.DatabaseTypeName())
	}
	return out, nil
}
func normalize(value any, kind string) any {
	if value == nil {
		return nil
	}
	if bytes, ok := value.([]byte); ok {
		value = string(bytes)
	}
	if kind == "bool" || kind == "boolean" {
		switch v := value.(type) {
		case int64:
			return v != 0
		case string:
			return v == "1" || strings.EqualFold(v, "true")
		}
	}
	return value
}

func resetSequences(db *gorm.DB) error {
	for _, table := range []string{"users", "sys_roles", "sys_role_members", "privileges", "role_privileges", "project_role_defs", "project_members", "project_gitlab_repositories", "environment_policies"} {
		stmt := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s','id'), GREATEST(COALESCE((SELECT MAX(id) FROM %s), 1), 1), true)`, table, table)
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func findSQLite() string {
	dir, _ := os.Getwd()
	for {
		candidate := filepath.Join(dir, "data", "fluxa.db")
		if info, err := os.Stat(candidate); err == nil && info.Size() > 0 {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
