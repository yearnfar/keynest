package keynest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func listRecentDatabases() []RecentDatabase {
	path, err := recentStorePath()
	if err != nil {
		return []RecentDatabase{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return []RecentDatabase{}
	}
	var recent []RecentDatabase
	if json.Unmarshal(data, &recent) != nil {
		return []RecentDatabase{}
	}
	available := make([]RecentDatabase, 0, len(recent))
	for _, item := range recent {
		if _, err := os.Stat(item.Path); err == nil {
			available = append(available, item)
		}
	}
	return available
}

func recentStorePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "KeyNest")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "recent.json"), nil
}

func addRecent(path string) {
	storePath, err := recentStorePath()
	if err != nil {
		return
	}
	recent := listRecentDatabases()
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	next := []RecentDatabase{{Name: name, Path: path, Modified: "刚刚"}}
	for _, candidate := range recent {
		if candidate.Path != path && len(next) < 8 {
			next = append(next, candidate)
		}
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err == nil {
		_ = os.WriteFile(storePath, data, 0600)
	}
}
