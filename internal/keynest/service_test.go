package keynest

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

func newTestService(t *testing.T, path, password string) *Service {
	t.Helper()
	db, err := createDatabaseAt(path, "测试保险库", password, "")
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	if err := saveTemplatePreferences(db, templatePreferences{}); err != nil {
		t.Fatalf("reset template preferences: %v", err)
	}
	if err := writeDatabase(db, path); err != nil {
		t.Fatalf("persist test template preferences: %v", err)
	}
	return &Service{db: db, dbPath: path}
}

func TestNewDatabaseStartsWithoutManagedTemplates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := NewService()
	snapshot, err := service.Create(path, CreateRequest{Name: "Empty templates", Password: "master-password"})
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	if len(snapshot.Templates) != 1 || snapshot.Templates[0].ID != "login" {
		t.Fatalf("new database templates = %#v, want login only", snapshot.Templates)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	templates := templatesFromDatabase(reopened)
	if len(templates) != 1 || templates[0].ID != "login" {
		t.Fatalf("new database templates after reopen = %#v, want login only", templates)
	}
}

func TestDatabaseDecodeError(t *testing.T) {
	tests := []struct {
		name       string
		input      error
		want       string
		wrappedErr error
	}{
		{
			name:  "credential mismatch",
			input: errors.New("Wrong password? HMAC-SHA256 of header mismatching"),
			want:  "主密码或密钥文件与该数据库不匹配",
		},
		{
			name:       "invalid group time",
			input:      fmt.Errorf(`failed to unmarshal group child "Times": %w`, io.EOF),
			want:       `KDBX 分组的时间字段格式不兼容（Times 中存在空值或长度不足）: failed to unmarshal group child "Times": EOF`,
			wrappedErr: io.EOF,
		},
		{
			name:       "unsupported cipher",
			input:      gokeepasslib.ErrUnsupportedEncrypterType,
			want:       "KDBX 使用了 Keynest 暂不支持的加密算法: Type of encrypter unsupported",
			wrappedErr: gokeepasslib.ErrUnsupportedEncrypterType,
		},
		{
			name:  "content integrity failure",
			input: errors.New("failed to verify HMAC for block 1"),
			want:  "KDBX 文件完整性校验失败，文件可能已损坏: failed to verify HMAC for block 1",
		},
		{
			name:  "other parse error",
			input: errors.New("XML syntax error"),
			want:  "无法解析 KDBX 数据库: XML syntax error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := databaseDecodeError(test.input)
			if actual.Error() != test.want {
				t.Fatalf("databaseDecodeError() = %q, want %q", actual, test.want)
			}
			if test.wrappedErr != nil && !errors.Is(actual, test.wrappedErr) {
				t.Fatalf("databaseDecodeError() does not wrap %v", test.wrappedErr)
			}
			if strings.Contains(actual.Error(), "主密码") && test.name != "credential mismatch" {
				t.Fatalf("non-credential error was reported as a password problem: %q", actual)
			}
		})
	}
}

func TestFormatModifiedTimeIncludesYearForEarlierYears(t *testing.T) {
	now := time.Date(2026, time.October, 18, 12, 0, 0, 0, time.Local)
	tests := []struct {
		name     string
		value    time.Time
		expected string
	}{
		{name: "earlier year", value: time.Date(2025, time.December, 31, 23, 59, 0, 0, time.Local), expected: "2025 年 12 月 31 日"},
		{name: "current year", value: time.Date(2026, time.August, 21, 9, 30, 0, 0, time.Local), expected: "08 月 21 日"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := formatModifiedTime(test.value, now); actual != test.expected {
				t.Fatalf("formatModifiedTime() = %q, want %q", actual, test.expected)
			}
		})
	}
}

func TestMapEntryIncludesCreationAndModificationTimes(t *testing.T) {
	entry := gokeepasslib.NewEntry()
	createdAt := time.Date(2025, time.March, 4, 5, 6, 7, 8, time.UTC)
	modifiedAt := time.Date(2026, time.October, 19, 10, 11, 12, 13, time.UTC)
	entry.Times.CreationTime = &wrappers.TimeWrapper{Time: createdAt}
	entry.Times.LastModificationTime = &wrappers.TimeWrapper{Time: modifiedAt}

	mapped := mapEntry(&entry, "个人", "group-id", false)
	if mapped.CreatedAt != createdAt.Format(time.RFC3339Nano) {
		t.Fatalf("createdAt = %q, want %q", mapped.CreatedAt, createdAt.Format(time.RFC3339Nano))
	}
	if mapped.ModifiedAt != modifiedAt.Format(time.RFC3339Nano) {
		t.Fatalf("modifiedAt = %q, want %q", mapped.ModifiedAt, modifiedAt.Format(time.RFC3339Nano))
	}
}

func TestKDBXCreateSaveAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "correct horse battery staple")
	result, err := service.SaveEntry(SaveEntryRequest{
		Name:     "Example",
		Username: "person@example.com",
		Password: "a-strong-password",
		URL:      "https://example.com",
		Note:     "stored locally",
		Folder:   "个人",
		Category: "登录信息",
		Favorite: true,
		Tags:     []string{"test", "local"},
	})
	if err != nil {
		t.Fatalf("save entry: %v", err)
	}
	if result.Entry.ID == "" {
		t.Fatal("saved entry has no id")
	}

	reopened, err := decodeDatabase(path, "correct horse battery staple", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	snapshot := snapshotFromDatabase(reopened, path)
	if len(snapshot.Entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(snapshot.Entries))
	}
	entry := snapshot.Entries[0]
	if entry.Password != "a-strong-password" || entry.Username != "person@example.com" {
		t.Fatalf("entry did not survive round trip: %#v", entry)
	}
	if !entry.Favorite || len(entry.Tags) != 2 {
		t.Fatalf("metadata did not survive round trip: %#v", entry)
	}
}

func TestSaveEntryStoresHistoryWithoutListingOldVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Example", Username: "old-user", Password: "old-password", Note: "old note", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	updated, err := service.SaveEntry(SaveEntryRequest{
		ID: created.Entry.ID, Name: "Example", Username: "new-user", Password: "new-password", Note: "new note", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("update password: %v", err)
	}
	if updated.Entry.HistoryCount != 1 {
		t.Fatalf("expected one history version, got %#v", updated.Entry)
	}
	history, err := service.GetEntryHistory(updated.Entry.ID)
	if err != nil || len(history) != 1 {
		t.Fatalf("get entry history: history=%#v err=%v", history, err)
	}
	old := history[0]
	if old.Password != "old-password" || old.Username != "old-user" || old.Note != "old note" || old.Trashed {
		t.Fatalf("unexpected history version: %#v", old)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	snapshot := snapshotFromDatabase(reopened, path)
	if len(snapshot.Entries) != 1 {
		t.Fatalf("history version appeared in the entry list: %#v", snapshot.Entries)
	}
	current := snapshot.Entries[0]
	if current.Password != "new-password" || current.HistoryCount != 1 {
		t.Fatalf("history did not survive reopen: %#v", current)
	}
	reopenedService := &Service{db: reopened, dbPath: path}
	persistedHistory, err := reopenedService.GetEntryHistory(current.ID)
	if err != nil || len(persistedHistory) != 1 || persistedHistory[0].Password != "old-password" {
		t.Fatalf("persisted history could not be loaded: history=%#v err=%v", persistedHistory, err)
	}

	renamed, err := service.SaveEntry(SaveEntryRequest{
		ID: updated.Entry.ID, Name: "Renamed", Password: "new-password", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("update without password change: %v", err)
	}
	renamedHistory, err := service.GetEntryHistory(renamed.Entry.ID)
	if err != nil || renamed.Entry.HistoryCount != 2 || len(renamedHistory) != 2 || renamedHistory[0].Password != "new-password" || renamedHistory[1].Password != "old-password" {
		t.Fatalf("non-password edit did not create ordered history: entry=%#v history=%#v err=%v", renamed.Entry, renamedHistory, err)
	}
}

func TestMapEntrySortsHistoryByModificationTimeDescending(t *testing.T) {
	current := gokeepasslib.NewEntry()
	base := time.Date(2026, time.October, 20, 8, 0, 0, 0, time.UTC)
	versions := make([]gokeepasslib.Entry, 3)
	for i, item := range []struct {
		name string
		age  time.Duration
	}{
		{name: "Newest", age: 0},
		{name: "Oldest", age: 2 * time.Hour},
		{name: "Middle", age: time.Hour},
	} {
		versions[i] = gokeepasslib.NewEntry()
		setEntryValue(&versions[i], "Title", item.name, false)
		modified := wrappers.TimeWrapper{Time: base.Add(-item.age)}
		versions[i].Times.LastModificationTime = &modified
	}
	current.Histories = []gokeepasslib.History{{Entries: versions}}

	mapped := mapEntry(&current, "个人", "group-id", false)
	history := mapEntryHistory(&current, "个人", "group-id")
	if mapped.HistoryCount != 3 || len(history) != 3 || history[0].Name != "Newest" || history[1].Name != "Middle" || history[2].Name != "Oldest" {
		t.Fatalf("history was not sorted newest first: entry=%#v history=%#v", mapped, history)
	}
}

func TestInitialPasswordStoresBlankHistoryVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "no title", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create blank entry: %v", err)
	}
	updated, err := service.SaveEntry(SaveEntryRequest{
		ID: created.Entry.ID, Name: "Example", Password: "first-password", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("set initial password: %v", err)
	}
	history, historyErr := service.GetEntryHistory(updated.Entry.ID)
	if updated.Entry.Password != "first-password" || updated.Entry.Name != "Example" || updated.Entry.HistoryCount != 1 || historyErr != nil || len(history) != 1 || history[0].Password != "" {
		t.Fatalf("entry was not updated: %#v", updated.Entry)
	}
}

func TestEntryHistoryRespectsDatabaseItemLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	service.db.Content.Meta.HistoryMaxItems = 2
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Example 0", Password: "password", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}

	current := created
	for i := 1; i <= 3; i++ {
		current, err = service.SaveEntry(SaveEntryRequest{
			ID: current.Entry.ID, Name: fmt.Sprintf("Example %d", i), Password: "password", Folder: "个人", Category: "登录信息",
		})
		if err != nil {
			t.Fatalf("update entry %d: %v", i, err)
		}
	}

	history, historyErr := service.GetEntryHistory(current.Entry.ID)
	if current.Entry.HistoryCount != 2 || historyErr != nil || len(history) != 2 || history[0].Name != "Example 2" || history[1].Name != "Example 1" {
		t.Fatalf("unexpected capped history: entry=%#v history=%#v err=%v", current.Entry, history, historyErr)
	}
}

func TestRecycleBinRetentionAndEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	for i := 1; i <= 2; i++ {
		created, err := service.SaveEntry(SaveEntryRequest{
			Name: fmt.Sprintf("Example %d", i), Password: fmt.Sprintf("password-%d", i), Folder: "个人", Category: "登录信息",
		})
		if err != nil {
			t.Fatalf("create entry %d: %v", i, err)
		}
		if _, err := service.MoveEntryToRecycleBin(created.Entry.ID); err != nil {
			t.Fatalf("move entry %d to recycle bin: %v", i, err)
		}
	}

	recycleBin := recycleBinGroup(service.db)
	if recycleBin == nil || len(recycleBin.Entries) != 2 {
		t.Fatalf("expected two recycled entries, got %#v", recycleBin)
	}
	now := time.Now()
	recycleBin.Entries[0].Times.LocationChanged.Time = now.Add(-31 * 24 * time.Hour)
	recycleBin.Entries[1].Times.LocationChanged.Time = now.Add(-29 * 24 * time.Hour)
	if removed := purgeExpiredRecycleBin(service.db, now); removed != 1 {
		t.Fatalf("expected one expired entry removed, got %d", removed)
	}
	if len(recycleBin.Entries) != 1 || recycleBin.Entries[0].GetPassword() != "password-2" {
		t.Fatalf("recent recycled entry should be retained: %#v", recycleBin.Entries)
	}

	removed, err := service.EmptyRecycleBin()
	if err != nil {
		t.Fatalf("empty recycle bin: %v", err)
	}
	if removed != 1 || len(recycleBin.Entries) != 0 {
		t.Fatalf("expected one retained entry to be cleared, removed=%d", removed)
	}
	if len(service.db.Content.Root.DeletedObjects) != 2 {
		t.Fatalf("expected permanent deletions to be tracked, got %d", len(service.db.Content.Root.DeletedObjects))
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	for _, entry := range snapshotFromDatabase(reopened, path).Entries {
		if entry.Trashed {
			t.Fatalf("recycle bin entry survived clear: %#v", entry)
		}
	}
}

func TestRestoreAndDeleteRecycleBinEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Restore me", Password: "password-1", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	storedEntry, _ := findEntry(service.db.Content.Root.Groups, created.Entry.ID)
	if storedEntry == nil || storedEntry.Times.LastAccessTime == nil {
		t.Fatal("created entry has no last access time")
	}
	oldAccess := time.Now().Add(-48 * time.Hour)
	storedEntry.Times.LastAccessTime.Time = oldAccess
	recycled, err := service.MoveEntryToRecycleBin(created.Entry.ID)
	if err != nil {
		t.Fatalf("move entry to recycle bin: %v", err)
	}
	restored, err := service.RestoreRecycleBinEntry(recycled.ID)
	if err != nil {
		t.Fatalf("restore entry: %v", err)
	}
	if restored.Trashed || restored.Password != "password-1" || restored.Folder != "个人" || restored.DeletedAt != "" {
		t.Fatalf("unexpected restored entry: %#v", restored)
	}
	restoredAt, err := time.Parse(time.RFC3339Nano, restored.LastUsedAt)
	if err != nil || !restoredAt.After(oldAccess) {
		t.Fatalf("restore did not update last access time: value=%q err=%v", restored.LastUsedAt, err)
	}
	if recycleBin := recycleBinGroup(service.db); recycleBin == nil || len(recycleBin.Entries) != 0 {
		t.Fatalf("restored entry remained in recycle bin: %#v", recycleBin)
	}

	deleteCandidate, err := service.SaveEntry(SaveEntryRequest{
		Name: "Delete me", Password: "password-2", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create entry for permanent deletion: %v", err)
	}
	deleted, err := service.MoveEntryToRecycleBin(deleteCandidate.Entry.ID)
	if err != nil {
		t.Fatalf("move entry for permanent deletion: %v", err)
	}
	deletedObjectsBefore := len(service.db.Content.Root.DeletedObjects)
	if err := service.DeleteRecycleBinEntry(deleted.ID); err != nil {
		t.Fatalf("delete recycle bin entry: %v", err)
	}
	if len(service.db.Content.Root.DeletedObjects) != deletedObjectsBefore+1 {
		t.Fatal("permanent deletion was not recorded in KDBX DeletedObjects")
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	trashed := 0
	restoredFound := false
	for _, entry := range snapshotFromDatabase(reopened, path).Entries {
		if entry.Trashed {
			trashed++
		}
		if entry.ID == restored.ID && entry.Password == "password-1" {
			persistedAt, parseErr := time.Parse(time.RFC3339Nano, entry.LastUsedAt)
			if parseErr != nil || restoredAt.Sub(persistedAt).Abs() >= time.Second {
				t.Fatalf("restored last access time did not survive reopen: value=%q err=%v", entry.LastUsedAt, parseErr)
			}
			restoredFound = true
		}
	}
	if trashed != 0 || !restoredFound {
		t.Fatalf("restore/delete did not survive reopen: trashed=%d restored=%t", trashed, restoredFound)
	}
}

func TestMoveEntryToRecycleBinAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Delete me", Password: "password", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	deletedObjectsBefore := len(service.db.Content.Root.DeletedObjects)
	recycled, err := service.MoveEntryToRecycleBin(created.Entry.ID)
	if err != nil {
		t.Fatalf("move entry to recycle bin: %v", err)
	}
	if !recycled.Trashed || recycled.DeletedAt == "" || recycled.Folder != "回收站" {
		t.Fatalf("unexpected recycled entry: %#v", recycled)
	}
	if len(service.db.Content.Root.DeletedObjects) != deletedObjectsBefore {
		t.Fatal("moving to recycle bin must not create a KDBX DeletedObject")
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	service = &Service{db: reopened, dbPath: path}
	restored, err := service.RestoreRecycleBinEntry(recycled.ID)
	if err != nil {
		t.Fatalf("restore recycled entry: %v", err)
	}
	if restored.Trashed || restored.Folder != "个人" || restored.Name != "Delete me" {
		t.Fatalf("entry was not restored to its original folder: %#v", restored)
	}

	reopened, err = decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen restored database: %v", err)
	}
	entries := snapshotFromDatabase(reopened, path).Entries
	if len(entries) != 1 || entries[0].Trashed || entries[0].Folder != "个人" {
		t.Fatalf("restored state did not survive reopen: %#v", entries)
	}
}

func TestRestoreRecycleBinEntryClearsDeletedGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	group, err := service.CreateGroup("临时分组", "", 48)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Restore ungrouped", Password: "password", GroupID: group.ID, Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	recycled, err := service.MoveEntryToRecycleBin(created.Entry.ID)
	if err != nil {
		t.Fatalf("move entry to recycle bin: %v", err)
	}
	if _, err := service.DeleteGroup(group.ID); err != nil {
		t.Fatalf("delete original group: %v", err)
	}

	restored, err := service.RestoreRecycleBinEntry(recycled.ID)
	if err != nil {
		t.Fatalf("restore entry: %v", err)
	}
	if restored.Trashed || restored.GroupID != "" || restored.Folder != "" {
		t.Fatalf("entry restored with a deleted group: %#v", restored)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	entries := snapshotFromDatabase(reopened, path).Entries
	if len(entries) != 1 || entries[0].Trashed || entries[0].GroupID != "" || entries[0].Folder != "" {
		t.Fatalf("ungrouped restore did not survive reopen: %#v", entries)
	}
}

func TestKDBXRejectsWrongPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	if _, err := createDatabaseAt(path, "测试保险库", "right-password", ""); err != nil {
		t.Fatalf("create database: %v", err)
	}
	if _, err := decodeDatabase(path, "wrong-password", ""); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

func TestChangePasswordReencryptsDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "old-master-password")
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Example", Password: "entry-password", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if err := service.ChangePassword("wrong-password", "new-master-password"); err == nil {
		t.Fatal("expected incorrect current password to be rejected")
	}
	if _, err := decodeDatabase(path, "old-master-password", ""); err != nil {
		t.Fatalf("incorrect current password changed database credentials: %v", err)
	}

	if err := service.ChangePassword("old-master-password", "new-master-password"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, err := decodeDatabase(path, "old-master-password", ""); err == nil {
		t.Fatal("old master password still unlocks database")
	}
	reopened, err := decodeDatabase(path, "new-master-password", "")
	if err != nil {
		t.Fatalf("new master password does not unlock database: %v", err)
	}
	snapshot := snapshotFromDatabase(reopened, path)
	if len(snapshot.Entries) != 1 || snapshot.Entries[0].ID != created.Entry.ID || snapshot.Entries[0].Password != "entry-password" {
		t.Fatalf("entry did not survive password change: %#v", snapshot.Entries)
	}
}

func TestGroupsMapToKDBXGroupsAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	initial := snapshotFromDatabase(service.db, path)
	if len(initial.Groups) != 1 || initial.Groups[0].Name != "个人" || initial.Groups[0].ID == "" {
		t.Fatalf("expected the KDBX root group in snapshot: %#v", initial.Groups)
	}

	work, err := service.CreateGroup("工作", "", 37)
	if err != nil {
		t.Fatalf("create root group: %v", err)
	}
	accounts, err := service.CreateGroup("账号", work.ID, 48)
	if err != nil {
		t.Fatalf("create nested group: %v", err)
	}
	if work.IconID != 37 || accounts.ParentID != work.ID || accounts.Depth != 1 || accounts.IconID != 48 {
		t.Fatalf("unexpected nested group: %#v", accounts)
	}
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "GitHub", Password: "password", GroupID: accounts.ID, Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("save entry in group: %v", err)
	}
	if created.Entry.GroupID != accounts.ID || created.Entry.Folder != "账号" {
		t.Fatalf("entry was not assigned to its KDBX group: %#v", created.Entry)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	snapshot := snapshotFromDatabase(reopened, path)
	if len(snapshot.Groups) != 3 {
		t.Fatalf("expected three persisted groups, got %#v", snapshot.Groups)
	}
	if snapshot.Groups[1].IconID != 37 || snapshot.Groups[2].IconID != 48 {
		t.Fatalf("group icons did not survive reopen: %#v", snapshot.Groups)
	}
	found := false
	for _, entry := range snapshot.Entries {
		if entry.ID == created.Entry.ID && entry.GroupID == accounts.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("group assignment did not survive reopen")
	}
}

func TestSaveEntryMovesEntryToSelectedGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	personal := snapshotFromDatabase(service.db, path).Groups[0]
	work, err := service.CreateGroup("工作", "", 37)
	if err != nil {
		t.Fatalf("create work group: %v", err)
	}
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "GitHub", Password: "password", GroupID: personal.ID, Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}

	updated, err := service.SaveEntry(SaveEntryRequest{
		ID: created.Entry.ID, Name: "GitHub", Password: "password", GroupID: work.ID, Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("move entry: %v", err)
	}
	if updated.Entry.GroupID != work.ID || updated.Entry.Folder != "工作" || updated.Entry.HistoryCount != 1 {
		t.Fatalf("entry was not moved to the selected group: %#v", updated)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	persisted := snapshotFromDatabase(reopened, path)
	if len(persisted.Entries) != 1 || persisted.Entries[0].ID != created.Entry.ID || persisted.Entries[0].GroupID != work.ID {
		t.Fatalf("moved entry did not survive reopen: %#v", persisted.Entries)
	}
}

func TestTouchEntryUpdatesLastAccessTimeAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Recent", Password: "password", Folder: "个人", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	entry, _ := findEntry(service.db.Content.Root.Groups, created.Entry.ID)
	if entry == nil || entry.Times.LastAccessTime == nil {
		t.Fatal("created entry has no last access time")
	}
	oldAccess := time.Now().Add(-48 * time.Hour)
	entry.Times.LastAccessTime.Time = oldAccess
	if err := writeDatabase(service.db, path); err != nil {
		t.Fatalf("persist old access time: %v", err)
	}

	touched, err := service.TouchEntry(created.Entry.ID)
	if err != nil {
		t.Fatalf("touch entry: %v", err)
	}
	touchedAt, err := time.Parse(time.RFC3339Nano, touched.LastUsedAt)
	if err != nil || !touchedAt.After(oldAccess) {
		t.Fatalf("last access time was not updated: value=%q err=%v", touched.LastUsedAt, err)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	persisted := snapshotFromDatabase(reopened, path)
	if len(persisted.Entries) != 1 {
		t.Fatalf("last access time did not survive reopen: %#v", persisted.Entries)
	}
	persistedAt, err := time.Parse(time.RFC3339Nano, persisted.Entries[0].LastUsedAt)
	if err != nil || touchedAt.Sub(persistedAt).Abs() >= time.Second {
		t.Fatalf("last access time did not survive reopen: value=%q err=%v", persisted.Entries[0].LastUsedAt, err)
	}
}

func TestCreateGroupRejectsInvalidIcon(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	if _, err := service.CreateGroup("无效图标", "", 69); err == nil {
		t.Fatal("expected invalid group icon to be rejected")
	}
}

func TestUpdateGroupPreservesOrderAndPersistsGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	personal := snapshotFromDatabase(service.db, path).Groups[0]
	work, err := service.CreateGroup("工作", "", 37)
	if err != nil {
		t.Fatalf("create work group: %v", err)
	}
	first, err := service.CreateGroup("第一组", work.ID, 47)
	if err != nil {
		t.Fatalf("create first child group: %v", err)
	}
	accounts, err := service.CreateGroup("账号", work.ID, 48)
	if err != nil {
		t.Fatalf("create accounts group: %v", err)
	}
	last, err := service.CreateGroup("最后一组", work.ID, 49)
	if err != nil {
		t.Fatalf("create last child group: %v", err)
	}
	other, err := service.CreateGroup("其他", "", 5)
	if err != nil {
		t.Fatalf("create other group: %v", err)
	}
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "GitHub", Password: "password", GroupID: accounts.ID, Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("save entry: %v", err)
	}

	snapshot, err := service.UpdateGroup(accounts.ID, "凭据", work.ID, 12)
	if err != nil {
		t.Fatalf("update group: %v", err)
	}
	assertGroupOrder(t, snapshot.Groups, work.ID, first.ID, accounts.ID, last.ID, other.ID)
	var updated *Group
	for i := range snapshot.Groups {
		if snapshot.Groups[i].ID == accounts.ID {
			updated = &snapshot.Groups[i]
			break
		}
	}
	if updated == nil || updated.Name != "凭据" || updated.ParentID != work.ID || updated.Depth != 1 || updated.IconID != 12 {
		t.Fatalf("unexpected updated group: %#v", updated)
	}
	for _, entry := range snapshot.Entries {
		if entry.ID == created.Entry.ID && (entry.GroupID != accounts.ID || entry.Folder != "凭据") {
			t.Fatalf("entry did not follow renamed group: %#v", entry)
		}
	}
	if _, err := service.UpdateGroup(accounts.ID, "凭据", personal.ID, 12); err == nil {
		t.Fatal("expected editing a group to reject a parent change")
	}
	if snapshot, err = service.UpdateGroup(work.ID, "工作资料", "", 8); err != nil {
		t.Fatalf("update top-level group: %v", err)
	}
	assertGroupOrder(t, snapshot.Groups, personal.ID, work.ID, first.ID, accounts.ID, last.ID, other.ID)

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	persisted := snapshotFromDatabase(reopened, path)
	persistedFound := false
	for _, group := range persisted.Groups {
		if group.ID == accounts.ID {
			persistedFound = true
			if group.Name != "凭据" || group.ParentID != work.ID || group.IconID != 12 {
				t.Fatalf("updated group did not survive reopen: %#v", group)
			}
		}
	}
	if !persistedFound {
		t.Fatal("updated group missing after reopen")
	}
	assertGroupOrder(t, persisted.Groups, personal.ID, work.ID, first.ID, accounts.ID, last.ID, other.ID)
}

func TestMoveGroupPlacesGroupFirstAndPersistsOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	work, err := service.CreateGroup("工作", "", 37)
	if err != nil {
		t.Fatalf("create work group: %v", err)
	}
	first, err := service.CreateGroup("第一组", work.ID, 48)
	if err != nil {
		t.Fatalf("create first child: %v", err)
	}
	second, err := service.CreateGroup("第二组", work.ID, 49)
	if err != nil {
		t.Fatalf("create second child: %v", err)
	}
	other, err := service.CreateGroup("其他", "", 5)
	if err != nil {
		t.Fatalf("create other group: %v", err)
	}

	snapshot, err := service.MoveGroup(second.ID, other.ID)
	if err != nil {
		t.Fatalf("move child to another parent: %v", err)
	}
	assertGroupOrder(t, snapshot.Groups, other.ID, second.ID)

	if _, err := service.MoveGroup(second.ID, work.ID); err != nil {
		t.Fatalf("move child back to original parent: %v", err)
	}
	snapshot, err = service.MoveGroup(first.ID, work.ID)
	if err != nil {
		t.Fatalf("reorder child within current parent: %v", err)
	}
	assertGroupOrder(t, snapshot.Groups, work.ID, first.ID, second.ID)

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	assertGroupOrder(t, snapshotFromDatabase(reopened, path).Groups, work.ID, first.ID, second.ID)
}

func assertGroupOrder(t *testing.T, groups []Group, ids ...string) {
	t.Helper()
	positions := make(map[string]int, len(ids))
	for index, group := range groups {
		positions[group.ID] = index
	}
	for index := 1; index < len(ids); index++ {
		if positions[ids[index]] != positions[ids[index-1]]+1 {
			t.Fatalf("expected adjacent group order %v, got %#v", ids, groups)
		}
	}
}

func TestDeleteGroupKeepsEntriesWithoutGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	work, err := service.CreateGroup("工作", "", 37)
	if err != nil {
		t.Fatalf("create work group: %v", err)
	}
	accounts, err := service.CreateGroup("账号", work.ID, 48)
	if err != nil {
		t.Fatalf("create accounts group: %v", err)
	}
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "GitHub", Password: "password", GroupID: accounts.ID, Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("save entry: %v", err)
	}

	snapshot, err := service.DeleteGroup(work.ID)
	if err != nil {
		t.Fatalf("delete group: %v", err)
	}
	for _, group := range snapshot.Groups {
		if group.ID == work.ID || group.ID == accounts.ID {
			t.Fatalf("deleted group remains visible: %#v", snapshot.Groups)
		}
	}
	var retained *Entry
	for i := range snapshot.Entries {
		if snapshot.Entries[i].ID == created.Entry.ID {
			retained = &snapshot.Entries[i]
			break
		}
	}
	if retained == nil || retained.Trashed || retained.DeletedAt != "" || retained.GroupID != "" || retained.Folder != "" || retained.Password != "password" {
		t.Fatalf("group entry was not retained without a group: %#v", retained)
	}
	if recycleBin := recycleBinGroup(service.db); recycleBin != nil && len(recycleBin.Entries) > 0 {
		t.Fatalf("group entries unexpectedly moved to recycle bin: %#v", recycleBin.Entries)
	}
	if ungrouped := ensureUngroupedGroup(service.db); len(ungrouped.Entries) != 1 || ungrouped.Entries[0].GetTitle() != "GitHub" {
		t.Fatalf("entry missing from internal ungrouped container: %#v", ungrouped.Entries)
	}
	updated, err := service.SaveEntry(SaveEntryRequest{
		ID: created.Entry.ID, Name: "GitHub updated", Password: "password", GroupID: "", Folder: "", Category: "登录信息",
	})
	if err != nil {
		t.Fatalf("save ungrouped entry: %v", err)
	}
	if updated.Entry.GroupID != "" || updated.Entry.Folder != "" || updated.Entry.Name != "GitHub updated" {
		t.Fatalf("saving restored a visible group association: %#v", updated.Entry)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	persisted := snapshotFromDatabase(reopened, path)
	if len(persisted.Entries) != 1 || persisted.Entries[0].Trashed || persisted.Entries[0].GroupID != "" || persisted.Entries[0].Folder != "" {
		t.Fatalf("deleted group state did not survive reopen: %#v", persisted)
	}
	for _, group := range persisted.Groups {
		if group.Name == "KeyNest Ungrouped" {
			t.Fatalf("internal ungrouped container is visible: %#v", persisted.Groups)
		}
	}
}

func TestTemplateFieldsRoundTripAndHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Bank card", Folder: "个人", Category: "银行卡", TemplateID: "bank-card", TemplateVersion: 1,
		Fields: []EntryField{
			{Key: "UserName", Value: "Test Person"},
			{Key: "Card Number", Value: "6217000012345678"},
			{Key: "Password", Value: "482615"},
			{Key: "Notes", Value: "Primary card"},
		},
	})
	if err != nil {
		t.Fatalf("save template entry: %v", err)
	}
	if created.Entry.TemplateID != "bank-card" || created.Entry.TemplateVersion != 1 {
		t.Fatalf("unexpected template metadata: %#v", created.Entry)
	}
	var cardNumber EntryField
	for _, field := range created.Entry.Fields {
		if field.Key == "Card Number" {
			cardNumber = field
		}
	}
	if cardNumber.Value != "6217000012345678" || !cardNumber.Protected {
		t.Fatalf("sensitive template field was not protected: %#v", cardNumber)
	}

	updatedFields := append([]EntryField(nil), created.Entry.Fields...)
	for i := range updatedFields {
		if updatedFields[i].Key == "Card Number" {
			updatedFields[i].Value = "6217000099999999"
		}
	}
	updated, err := service.SaveEntry(SaveEntryRequest{
		ID: created.Entry.ID, Name: "Bank card", Folder: "个人", Category: "银行卡", TemplateID: "bank-card", Fields: updatedFields,
	})
	if err != nil {
		t.Fatalf("update template entry: %v", err)
	}
	if updated.Entry.HistoryCount != 1 {
		t.Fatalf("template update did not create history: %#v", updated.Entry)
	}
	history, err := service.GetEntryHistory(updated.Entry.ID)
	if err != nil || len(history) != 1 {
		t.Fatalf("get template history: history=%#v err=%v", history, err)
	}
	if got := fieldValueForTest(history[0].Fields, "Card Number"); got != "6217000012345678" {
		t.Fatalf("history card number = %q", got)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	persisted := snapshotFromDatabase(reopened, path).Entries[0]
	if persisted.TemplateID != "bank-card" || fieldValueForTest(persisted.Fields, "Card Number") != "6217000099999999" {
		t.Fatalf("template entry did not survive reopen: %#v", persisted)
	}
}

func TestCustomTemplateCanBeRemovedWhileInUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	template, err := service.SaveTemplate(EntryTemplate{
		Name: "Server", Fields: []TemplateField{
			{ID: "host", StorageKey: "Host", Label: "Host", Type: "text", Section: "details", Required: true, Searchable: true},
			{ID: "token", StorageKey: "API Token", Label: "API Token", Type: "password", Section: "details", Protected: true},
		},
	})
	if err != nil {
		t.Fatalf("save custom template: %v", err)
	}
	if template.ID == "" || template.BuiltIn || !template.Enabled || template.Version != 1 {
		t.Fatalf("unexpected custom template: %#v", template)
	}
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Production", Folder: "个人", Category: "Server", TemplateID: template.ID,
		Fields: []EntryField{{Key: "Host", Value: "server.example.com"}, {Key: "API Token", Value: "secret"}},
	})
	if err != nil {
		t.Fatalf("save custom template entry: %v", err)
	}
	templates, err := service.DeleteTemplate(template.ID)
	if err != nil {
		t.Fatalf("delete template: %v", err)
	}
	deleted, found := findTemplate(templates, template.ID)
	if !found || !deleted.Deleted {
		t.Fatalf("deleted template definition was not retained: %#v", templates)
	}
	updated, err := service.SaveEntry(SaveEntryRequest{
		ID: created.Entry.ID, Name: "Production", Folder: "个人", Category: "Server", TemplateID: template.ID,
		Fields: created.Entry.Fields,
	})
	if err != nil || updated.Entry.TemplateID != template.ID {
		t.Fatalf("save entry with deleted template: entry=%#v err=%v", updated.Entry, err)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	templates = templatesFromDatabase(reopened)
	persisted, found := findTemplate(templates, template.ID)
	if !found || persisted.Name != "Server" || len(persisted.Fields) != 2 || !persisted.Deleted {
		t.Fatalf("custom template did not survive reopen: %#v", templates)
	}
}

func TestUnusedCustomTemplateIsPermanentlyRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	template, err := service.SaveTemplate(EntryTemplate{
		Name: "Temporary", Fields: []TemplateField{
			{ID: "value", StorageKey: "Temporary Value", Label: "Value", Type: "text", Section: "details"},
		},
	})
	if err != nil {
		t.Fatalf("save custom template: %v", err)
	}
	if _, err := service.SetTemplateEnabled(template.ID, false); err != nil {
		t.Fatalf("disable custom template: %v", err)
	}
	if _, err := service.ReorderTemplates([]string{"identity", "secure-note", "wifi", "bank-card", template.ID}); err != nil {
		t.Fatalf("reorder templates: %v", err)
	}

	templates, err := service.DeleteTemplate(template.ID)
	if err != nil {
		t.Fatalf("delete unused custom template: %v", err)
	}
	if _, found := findTemplate(templates, template.ID); found {
		t.Fatalf("unused template remained in runtime definitions: %#v", templates)
	}
	if _, found := findTemplate(customTemplatesFromDatabase(service.db), template.ID); found {
		t.Fatal("unused template remained in stored custom definitions")
	}
	preferences := templatePreferencesFromDatabase(service.db)
	for name, ids := range map[string][]string{"order": preferences.Order, "hidden": preferences.Hidden, "disabled": preferences.Disabled} {
		for _, id := range ids {
			if id == template.ID {
				t.Fatalf("deleted template remained in %s preferences: %#v", name, preferences)
			}
		}
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	if _, found := findTemplate(templatesFromDatabase(reopened), template.ID); found {
		t.Fatal("unused template survived database reopen")
	}
}

func TestCustomTemplateInRecycleBinIsSoftDeleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	template, err := service.SaveTemplate(EntryTemplate{
		Name: "Archived", Fields: []TemplateField{
			{ID: "value", StorageKey: "Archived Value", Label: "Value", Type: "text", Section: "details"},
		},
	})
	if err != nil {
		t.Fatalf("save custom template: %v", err)
	}
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Archived item", Category: template.Name, TemplateID: template.ID,
		Fields: []EntryField{{Key: "Archived Value", Value: "kept"}},
	})
	if err != nil {
		t.Fatalf("save custom template entry: %v", err)
	}
	if _, err := service.MoveEntryToRecycleBin(created.Entry.ID); err != nil {
		t.Fatalf("move entry to recycle bin: %v", err)
	}

	templates, err := service.DeleteTemplate(template.ID)
	if err != nil {
		t.Fatalf("delete template used in recycle bin: %v", err)
	}
	deleted, found := findTemplate(templates, template.ID)
	if !found || !deleted.Deleted {
		t.Fatalf("template used in recycle bin was not soft deleted: %#v", templates)
	}
}

func TestCustomTemplateInEntryHistoryIsSoftDeleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	template, err := service.SaveTemplate(EntryTemplate{
		Name: "Historical", Fields: []TemplateField{
			{ID: "value", StorageKey: "Historical Value", Label: "Value", Type: "text", Section: "details"},
		},
	})
	if err != nil {
		t.Fatalf("save custom template: %v", err)
	}
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Historical item", Category: template.Name, TemplateID: template.ID,
		Fields: []EntryField{{Key: "Historical Value", Value: "kept"}},
	})
	if err != nil {
		t.Fatalf("save custom template entry: %v", err)
	}
	if _, err := service.SaveEntry(SaveEntryRequest{
		ID: created.Entry.ID, Name: "Login item", Category: "登录信息", TemplateID: "login",
		Fields: []EntryField{{Key: "UserName", Value: "user"}, {Key: "Password", Value: "password"}},
	}); err != nil {
		t.Fatalf("change entry template: %v", err)
	}

	templates, err := service.DeleteTemplate(template.ID)
	if err != nil {
		t.Fatalf("delete template used in history: %v", err)
	}
	deleted, found := findTemplate(templates, template.ID)
	if !found || !deleted.Deleted {
		t.Fatalf("template used in history was not soft deleted: %#v", templates)
	}
}

func TestUnusedBuiltInTemplateIsRemovedWithoutSoftDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")

	templates, err := service.DeleteTemplate("bank-card")
	if err != nil {
		t.Fatalf("delete unused built-in template: %v", err)
	}
	if _, found := findTemplate(templates, "bank-card"); found {
		t.Fatalf("unused built-in template remained in runtime definitions: %#v", templates)
	}
	preferences := templatePreferencesFromDatabase(service.db)
	if len(preferences.Removed) != 1 || preferences.Removed[0] != "bank-card" || len(preferences.Hidden) != 0 {
		t.Fatalf("unused built-in template was not permanently removed: %#v", preferences)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	if _, found := findTemplate(templatesFromDatabase(reopened), "bank-card"); found {
		t.Fatal("unused built-in template survived database reopen")
	}
}

func TestTemplateOrderAndBuiltInDeletionPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")

	reordered, err := service.ReorderTemplates([]string{"identity", "secure-note", "wifi", "bank-card"})
	if err != nil {
		t.Fatalf("reorder templates: %v", err)
	}
	want := []string{"login", "identity", "secure-note", "wifi", "bank-card"}
	for i, id := range want {
		if reordered[i].ID != id {
			t.Fatalf("template %d = %q, want %q", i, reordered[i].ID, id)
		}
	}

	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Bank card", Folder: "个人", Category: "银行卡", TemplateID: "bank-card",
		Fields: []EntryField{{Key: "Card Number", Value: "6217000012345678"}},
	})
	if err != nil {
		t.Fatalf("save bank card: %v", err)
	}
	templates, err := service.DeleteTemplate("bank-card")
	if err != nil {
		t.Fatalf("delete built-in template: %v", err)
	}
	deleted, found := findTemplate(templates, "bank-card")
	if !found || !deleted.Deleted {
		t.Fatalf("built-in template was not marked deleted: %#v", templates)
	}
	if _, err := service.DeleteTemplate("login"); err == nil {
		t.Fatal("DeleteTemplate(login) succeeded")
	}
	if _, err := service.SaveEntry(SaveEntryRequest{
		ID: created.Entry.ID, Name: "Bank card", Folder: "个人", Category: "银行卡", TemplateID: "bank-card",
		Fields: created.Entry.Fields,
	}); err != nil {
		t.Fatalf("save entry with deleted built-in template: %v", err)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	persisted := templatesFromDatabase(reopened)
	if persisted[0].ID != "login" || persisted[1].ID != "identity" || persisted[2].ID != "secure-note" || persisted[3].ID != "wifi" {
		t.Fatalf("template order did not survive reopen: %#v", persisted)
	}
	deleted, found = findTemplate(persisted, "bank-card")
	if !found || !deleted.Deleted {
		t.Fatalf("built-in deletion did not survive reopen: %#v", persisted)
	}
}

func TestTemplateEnabledStatePersistsWithoutAffectingExistingEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Office Wi-Fi", Folder: "个人", Category: "Wi-Fi", TemplateID: "wifi",
		Fields: []EntryField{{Key: "UserName", Value: "Office"}, {Key: "Password", Value: "secret"}},
	})
	if err != nil {
		t.Fatalf("save Wi-Fi entry: %v", err)
	}

	templates, err := service.SetTemplateEnabled("wifi", false)
	if err != nil {
		t.Fatalf("disable Wi-Fi template: %v", err)
	}
	wifi, found := findTemplate(templates, "wifi")
	if !found || wifi.Enabled || wifi.Deleted {
		t.Fatalf("Wi-Fi template was not disabled: %#v", wifi)
	}
	if _, err := service.SetTemplateEnabled("login", false); err == nil {
		t.Fatal("SetTemplateEnabled(login, false) succeeded")
	}

	reordered, err := service.ReorderTemplates([]string{"wifi", "identity", "secure-note", "bank-card"})
	if err != nil {
		t.Fatalf("reorder templates with a disabled template: %v", err)
	}
	if reordered[1].ID != "wifi" || reordered[1].Enabled {
		t.Fatalf("disabled template was not kept in the ordered list: %#v", reordered)
	}
	updated, err := service.SaveEntry(SaveEntryRequest{
		ID: created.Entry.ID, Name: "Office Wi-Fi updated", Folder: "个人", Category: "Wi-Fi", TemplateID: "wifi",
		Fields: created.Entry.Fields,
	})
	if err != nil || updated.Entry.TemplateID != "wifi" {
		t.Fatalf("save existing entry with disabled template: entry=%#v err=%v", updated.Entry, err)
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	wifi, found = findTemplate(templatesFromDatabase(reopened), "wifi")
	if !found || wifi.Enabled {
		t.Fatalf("disabled state did not survive reopen: %#v", wifi)
	}
	reopenedService := &Service{db: reopened, dbPath: path}
	if _, err := reopenedService.SetTemplateEnabled("wifi", true); err != nil {
		t.Fatalf("re-enable Wi-Fi template: %v", err)
	}

	reopenedAgain, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database after enabling: %v", err)
	}
	wifi, found = findTemplate(templatesFromDatabase(reopenedAgain), "wifi")
	if !found || !wifi.Enabled {
		t.Fatalf("enabled state did not survive reopen: %#v", wifi)
	}
}

func TestCustomTemplatePasswordFieldsAreAlwaysProtected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	service := newTestService(t, path, "master-password")
	template, err := service.SaveTemplate(EntryTemplate{
		Name: "Credentials", Fields: []TemplateField{
			{ID: "token", StorageKey: " API Token ", Label: " API Token ", Type: " password ", Section: " details "},
			{ID: "password", StorageKey: "Password", Label: "Password", Type: "text", Section: "details"},
		},
	})
	if err != nil {
		t.Fatalf("save custom template: %v", err)
	}
	for _, field := range template.Fields {
		if !field.Protected {
			t.Fatalf("sensitive template field was not normalized: %#v", field)
		}
	}
	if template.Fields[0].StorageKey != "API Token" || template.Fields[0].Type != "password" {
		t.Fatalf("template field text was not normalized: %#v", template.Fields[0])
	}

	created, err := service.SaveEntry(SaveEntryRequest{
		Name: "Production", Folder: "个人", Category: template.Name, TemplateID: template.ID,
		Fields: []EntryField{
			{Key: "API Token", Value: "token", Protected: false},
			{Key: "Password", Value: "password", Protected: false},
		},
	})
	if err != nil {
		t.Fatalf("save template entry: %v", err)
	}
	for _, field := range created.Entry.Fields {
		if !field.Protected {
			t.Fatalf("sensitive entry field was not protected: %#v", field)
		}
	}

	reopened, err := decodeDatabase(path, "master-password", "")
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	for _, field := range snapshotFromDatabase(reopened, path).Entries[0].Fields {
		if !field.Protected {
			t.Fatalf("sensitive entry field lost protection after reopen: %#v", field)
		}
	}
}

func fieldValueForTest(fields []EntryField, key string) string {
	for _, field := range fields {
		if field.Key == key {
			return field.Value
		}
	}
	return ""
}
