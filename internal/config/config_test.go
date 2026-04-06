package config

import (
	"path/filepath"
	"testing"
)

func TestLoadConfigUsesIsolatedViperInstance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.yaml")
	pathB := filepath.Join(dir, "b.yaml")

	cfgA := &Config{
		StorageType: StorageTypeLocal,
		Local: LocalStorageConfig{
			Path:          filepath.Join(dir, "a-backups"),
			RetentionDays: 7,
		},
		Databases: []DatabaseConfig{{
			ID:          "db-a",
			Type:        "postgres",
			ContainerID: "container-a",
			Name:        "db_a",
			Schedule:    "0 2 * * *",
			Enabled:     true,
		}},
		Paths: &Paths{ConfigFile: pathA},
	}

	cfgB := &Config{
		StorageType: StorageTypeLocal,
		Local: LocalStorageConfig{
			Path:          filepath.Join(dir, "b-backups"),
			RetentionDays: 30,
		},
		Databases: []DatabaseConfig{{
			ID:          "db-b",
			Type:        "mysql",
			ContainerID: "container-b",
			Name:        "db_b",
			Schedule:    "0 3 * * *",
			Enabled:     true,
		}},
		Paths: &Paths{ConfigFile: pathB},
	}

	if err := SaveConfig(cfgA, pathA); err != nil {
		t.Fatalf("SaveConfig(cfgA) error = %v", err)
	}
	if err := SaveConfig(cfgB, pathB); err != nil {
		t.Fatalf("SaveConfig(cfgB) error = %v", err)
	}

	loadedA, err := LoadConfig(pathA)
	if err != nil {
		t.Fatalf("LoadConfig(pathA) error = %v", err)
	}
	loadedB, err := LoadConfig(pathB)
	if err != nil {
		t.Fatalf("LoadConfig(pathB) error = %v", err)
	}

	if loadedA.Local.Path == loadedB.Local.Path {
		t.Fatalf("expected isolated config values, got same local.path = %q", loadedA.Local.Path)
	}

	if len(loadedA.Databases) != 1 || loadedA.Databases[0].ID != "db-a" {
		t.Fatalf("loadedA databases mismatch: %+v", loadedA.Databases)
	}

	if len(loadedB.Databases) != 1 || loadedB.Databases[0].ID != "db-b" {
		t.Fatalf("loadedB databases mismatch: %+v", loadedB.Databases)
	}
}
