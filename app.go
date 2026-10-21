package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yearnfar/keynest/internal/keynest"
	"github.com/yearnfar/keynest/internal/updater"
)

type App struct {
	ctx                   context.Context
	vault                 *keynest.Service
	updates               *updater.Service
	stopSystemLockMonitor func()
}

const systemLockEvent = "keynest:system-lock"

type RecentDatabase struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Modified string `json:"modified"`
}

type VaultEntry struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	URL          string   `json:"url"`
	Note         string   `json:"note"`
	GroupID      string   `json:"groupId"`
	Folder       string   `json:"folder"`
	Category     string   `json:"category"`
	Favorite     bool     `json:"favorite"`
	Modified     string   `json:"modified"`
	CreatedAt    string   `json:"createdAt"`
	ModifiedAt   string   `json:"modifiedAt"`
	LastUsedAt   string   `json:"lastUsedAt"`
	Color        string   `json:"color"`
	Initials     string   `json:"initials"`
	Tags         []string `json:"tags"`
	Trashed      bool     `json:"trashed"`
	DeletedAt    string   `json:"deletedAt,omitempty"`
	HistoryCount int      `json:"historyCount"`
}

type VaultGroup struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
	Depth    int    `json:"depth"`
	IconID   int64  `json:"iconId"`
}

type VaultSnapshot struct {
	Name    string       `json:"name"`
	Path    string       `json:"path"`
	Entries []VaultEntry `json:"entries"`
	Groups  []VaultGroup `json:"groups"`
}

type CreateDatabaseRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	KeyFile  string `json:"keyFile"`
}

type SaveEntryRequest struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	URL      string   `json:"url"`
	Note     string   `json:"note"`
	GroupID  string   `json:"groupId"`
	Folder   string   `json:"folder"`
	Category string   `json:"category"`
	Favorite bool     `json:"favorite"`
	Tags     []string `json:"tags"`
}

type SaveEntryResult struct {
	Entry VaultEntry `json:"entry"`
}

type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseName    string `json:"releaseName"`
	ReleaseNotes   string `json:"releaseNotes"`
	PublishedAt    string `json:"publishedAt"`
	ReleaseURL     string `json:"releaseUrl"`
	Available      bool   `json:"available"`
	CanAutoUpdate  bool   `json:"canAutoUpdate"`
	StatusMessage  string `json:"statusMessage"`
}

func NewApp() *App {
	return &App{vault: keynest.NewService(), updates: updater.NewService()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.stopSystemLockMonitor = startSystemLockMonitor(a.handleSystemLock)
}

func (a *App) shutdown(context.Context) {
	if a.stopSystemLockMonitor != nil {
		a.stopSystemLockMonitor()
		a.stopSystemLockMonitor = nil
	}
	a.vault.Lock()
}

func (a *App) handleSystemLock() {
	a.vault.Lock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, systemLockEvent)
	}
}

func (a *App) ChooseDatabase() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "打开 KeePass 数据库",
		Filters: []runtime.FileFilter{{
			DisplayName: "KeePass 数据库 (*.kdbx)",
			Pattern:     "*.kdbx",
		}},
	})
}

func (a *App) ChooseKeyFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择密钥文件",
		Filters: []runtime.FileFilter{{
			DisplayName: "KeePass 密钥文件",
			Pattern:     "*.key;*.keyx;*.xml;*",
		}},
	})
}

func (a *App) ListRecentDatabases() []RecentDatabase {
	recent := a.vault.ListRecentDatabases()
	result := make([]RecentDatabase, len(recent))
	for i, item := range recent {
		result[i] = RecentDatabase(item)
	}
	return result
}

func (a *App) UnlockDatabase(path, password, keyFile string) (VaultSnapshot, error) {
	snapshot, err := a.vault.Unlock(path, password, keyFile)
	if err != nil {
		return VaultSnapshot{}, err
	}
	return toVaultSnapshot(snapshot), nil
}

func (a *App) CreateDatabase(request CreateDatabaseRequest) (VaultSnapshot, error) {
	createRequest := keynest.CreateRequest{
		Name: request.Name, Password: request.Password, KeyFile: request.KeyFile,
	}
	if err := keynest.ValidateCreateRequest(createRequest); err != nil {
		return VaultSnapshot{}, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "新建 KeePass 数据库",
		DefaultFilename: keynest.SafeFilename(request.Name) + ".kdbx",
		Filters: []runtime.FileFilter{{
			DisplayName: "KeePass 数据库 (*.kdbx)",
			Pattern:     "*.kdbx",
		}},
	})
	if err != nil || path == "" {
		return VaultSnapshot{}, err
	}
	if !strings.EqualFold(filepath.Ext(path), ".kdbx") {
		path += ".kdbx"
	}
	snapshot, err := a.vault.Create(path, createRequest)
	if err != nil {
		return VaultSnapshot{}, err
	}
	return toVaultSnapshot(snapshot), nil
}

func (a *App) SaveEntry(request SaveEntryRequest) (SaveEntryResult, error) {
	result, err := a.vault.SaveEntry(keynest.SaveEntryRequest{
		ID: request.ID, Name: request.Name, Username: request.Username,
		Password: request.Password, URL: request.URL, Note: request.Note,
		GroupID: request.GroupID, Folder: request.Folder, Category: request.Category,
		Favorite: request.Favorite, Tags: request.Tags,
	})
	if err != nil {
		return SaveEntryResult{}, err
	}
	return SaveEntryResult{Entry: toVaultEntry(result.Entry)}, nil
}

func (a *App) GetEntryHistory(id string) ([]VaultEntry, error) {
	history, err := a.vault.GetEntryHistory(id)
	if err != nil {
		return nil, err
	}
	result := make([]VaultEntry, len(history))
	for i, entry := range history {
		result[i] = toVaultEntry(entry)
	}
	return result, nil
}

func (a *App) TouchEntry(id string) (VaultEntry, error) {
	entry, err := a.vault.TouchEntry(id)
	if err != nil {
		return VaultEntry{}, err
	}
	return toVaultEntry(entry), nil
}

func (a *App) ChangePassword(currentPassword, newPassword string) error {
	return a.vault.ChangePassword(currentPassword, newPassword)
}

func (a *App) CreateGroup(name, parentID string, iconID int64) (VaultGroup, error) {
	group, err := a.vault.CreateGroup(name, parentID, iconID)
	if err != nil {
		return VaultGroup{}, err
	}
	return toVaultGroup(group), nil
}

func (a *App) UpdateGroup(id, name, parentID string, iconID int64) (VaultSnapshot, error) {
	snapshot, err := a.vault.UpdateGroup(id, name, parentID, iconID)
	if err != nil {
		return VaultSnapshot{}, err
	}
	return toVaultSnapshot(snapshot), nil
}

func (a *App) MoveGroup(id, parentID string) (VaultSnapshot, error) {
	snapshot, err := a.vault.MoveGroup(id, parentID)
	if err != nil {
		return VaultSnapshot{}, err
	}
	return toVaultSnapshot(snapshot), nil
}

func (a *App) DeleteGroup(id string) (VaultSnapshot, error) {
	snapshot, err := a.vault.DeleteGroup(id)
	if err != nil {
		return VaultSnapshot{}, err
	}
	return toVaultSnapshot(snapshot), nil
}

func (a *App) EmptyRecycleBin() (int, error) {
	return a.vault.EmptyRecycleBin()
}

func (a *App) MoveEntryToRecycleBin(id string) (VaultEntry, error) {
	entry, err := a.vault.MoveEntryToRecycleBin(id)
	if err != nil {
		return VaultEntry{}, err
	}
	return toVaultEntry(entry), nil
}

func (a *App) RestoreRecycleBinEntry(id string) (VaultEntry, error) {
	entry, err := a.vault.RestoreRecycleBinEntry(id)
	if err != nil {
		return VaultEntry{}, err
	}
	return toVaultEntry(entry), nil
}

func (a *App) DeleteRecycleBinEntry(id string) error {
	return a.vault.DeleteRecycleBinEntry(id)
}

func (a *App) LockDatabase() {
	a.vault.Lock()
}

func (a *App) CurrentUpdateInfo() UpdateInfo {
	return toUpdateInfo(a.updates.CurrentInfo())
}

func (a *App) CheckForUpdates() (UpdateInfo, error) {
	info, err := a.updates.Check(a.ctx)
	return toUpdateInfo(info), err
}

func (a *App) InstallUpdate() (UpdateInfo, error) {
	info, err := a.updates.Install(a.ctx)
	return toUpdateInfo(info), err
}

func (a *App) RestartApplication() error {
	if err := a.updates.Restart(); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

func toVaultSnapshot(snapshot keynest.Snapshot) VaultSnapshot {
	entries := make([]VaultEntry, len(snapshot.Entries))
	for i, entry := range snapshot.Entries {
		entries[i] = toVaultEntry(entry)
	}
	groups := make([]VaultGroup, len(snapshot.Groups))
	for i, group := range snapshot.Groups {
		groups[i] = toVaultGroup(group)
	}
	return VaultSnapshot{Name: snapshot.Name, Path: snapshot.Path, Entries: entries, Groups: groups}
}

func toVaultEntry(entry keynest.Entry) VaultEntry {
	return VaultEntry{
		ID: entry.ID, Name: entry.Name, Username: entry.Username,
		Password: entry.Password, URL: entry.URL, Note: entry.Note, GroupID: entry.GroupID,
		Folder: entry.Folder, Category: entry.Category, Favorite: entry.Favorite,
		Modified: entry.Modified, CreatedAt: entry.CreatedAt, ModifiedAt: entry.ModifiedAt,
		LastUsedAt: entry.LastUsedAt, Color: entry.Color, Initials: entry.Initials,
		Tags: entry.Tags, Trashed: entry.Trashed, DeletedAt: entry.DeletedAt,
		HistoryCount: entry.HistoryCount,
	}
}

func toVaultGroup(group keynest.Group) VaultGroup {
	return VaultGroup{
		ID: group.ID, Name: group.Name, ParentID: group.ParentID,
		Depth: group.Depth, IconID: group.IconID,
	}
}

func toUpdateInfo(info updater.Info) UpdateInfo {
	return UpdateInfo{
		CurrentVersion: info.CurrentVersion,
		LatestVersion:  info.LatestVersion,
		ReleaseName:    info.ReleaseName,
		ReleaseNotes:   info.ReleaseNotes,
		PublishedAt:    info.PublishedAt,
		ReleaseURL:     info.ReleaseURL,
		Available:      info.Available,
		CanAutoUpdate:  info.CanAutoUpdate,
		StatusMessage:  info.StatusMessage,
	}
}
