package server

import (
	"fmt"
)

// tomlStorage is the [[storage]] TOML block: a named, typed connection with
// the list of containers reachable through it. Backend-specific credentials
// live in a sub-table ([storage.swift], [storage.s3], …).
type tomlStorage struct {
	Name       string
	Type       string
	Containers []string
	Swift      *tomlSwiftConfig
}

// StorageConfig is the validated configuration of a named storage connection
type StorageConfig struct {
	Name       string
	Type       string
	Containers []string
	Swift      *SwiftConfig
}

// NewStoragesConfigFromToml validates [[storage]] blocks and builds the list
// of StorageConfig. It performs pure config validation (no network access).
func NewStoragesConfigFromToml(tStorages []*tomlStorage) ([]*StorageConfig, error) {
	if len(tStorages) == 0 {
		return nil, fmt.Errorf("you must provide at least one [[storage]] config")
	}

	storages := make([]*StorageConfig, 0, len(tStorages))
	seenNames := make(map[string]bool)
	seenContainers := make(map[string]string) // container name -> storage name

	for _, tStorage := range tStorages {
		if tStorage.Name == "" {
			return nil, fmt.Errorf("storage must have a 'name' setting")
		}
		if seenNames[tStorage.Name] {
			return nil, fmt.Errorf("duplicate storage name '%s'", tStorage.Name)
		}
		seenNames[tStorage.Name] = true

		if len(tStorage.Containers) == 0 {
			return nil, fmt.Errorf("storage '%s' must list at least one container", tStorage.Name)
		}

		storage := &StorageConfig{
			Name:       tStorage.Name,
			Type:       tStorage.Type,
			Containers: tStorage.Containers,
		}

		switch tStorage.Type {
		case StorageTypeSwift:
			if tStorage.Swift == nil {
				return nil, fmt.Errorf("storage '%s' is of type 'swift' but has no [storage.swift] sub-table", tStorage.Name)
			}
			swiftConfig, err := NewSwiftConfigFromToml(tStorage.Swift)
			if err != nil {
				return nil, fmt.Errorf("storage '%s': %s", tStorage.Name, err)
			}
			storage.Swift = swiftConfig
		case "":
			return nil, fmt.Errorf("storage '%s' must have a 'type' setting (ex: 'swift')", tStorage.Name)
		default:
			return nil, fmt.Errorf("storage '%s': unknown type '%s'", tStorage.Name, tStorage.Type)
		}

		for _, container := range tStorage.Containers {
			if container == "" {
				return nil, fmt.Errorf("storage '%s' has an empty container name", tStorage.Name)
			}
			if other, exists := seenContainers[container]; exists {
				return nil, fmt.Errorf("container '%s' is declared in both storage '%s' and '%s'", container, other, tStorage.Name)
			}
			seenContainers[container] = tStorage.Name
		}

		storages = append(storages, storage)
	}

	return storages, nil
}
