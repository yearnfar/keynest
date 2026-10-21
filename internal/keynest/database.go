package keynest

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

const (
	favoriteField        = "KeyNestFavorite"
	internalGroupField   = "KeyNestInternalGroup"
	ungroupedGroupMarker = "ungrouped"
)

func credentials(password, keyFile string) (*gokeepasslib.DBCredentials, error) {
	switch {
	case password != "" && keyFile != "":
		return gokeepasslib.NewPasswordAndKeyCredentials(password, keyFile)
	case keyFile != "":
		return gokeepasslib.NewKeyCredentials(keyFile)
	default:
		return gokeepasslib.NewPasswordCredentials(password), nil
	}
}

func decodeDatabase(path, password, keyFile string) (*gokeepasslib.Database, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开数据库: %w", err)
	}
	defer file.Close()

	creds, err := credentials(password, keyFile)
	if err != nil {
		return nil, fmt.Errorf("密钥文件无效: %w", err)
	}
	db := gokeepasslib.NewDatabase()
	db.Credentials = creds
	if err := gokeepasslib.NewDecoder(file).Decode(db); err != nil {
		return nil, databaseDecodeError(err)
	}
	if err := db.UnlockProtectedEntries(); err != nil {
		return nil, fmt.Errorf("无法解锁数据库: %w", err)
	}
	return db, nil
}

func databaseDecodeError(err error) error {
	message := err.Error()
	switch {
	case strings.Contains(message, "Wrong password?"):
		return errors.New("主密码或密钥文件与该数据库不匹配")
	case strings.Contains(message, "Sha256 of header mismatching"),
		strings.Contains(message, "failed to verify HMAC"):
		return fmt.Errorf("KDBX 文件完整性校验失败，文件可能已损坏: %w", err)
	case errors.Is(err, gokeepasslib.ErrUnsupportedEncrypterType):
		return fmt.Errorf("KDBX 使用了 Keynest 暂不支持的加密算法: %w", err)
	case errors.Is(err, gokeepasslib.ErrUnsupportedStreamType):
		return fmt.Errorf("KDBX 使用了 Keynest 暂不支持的保护流算法: %w", err)
	case errors.Is(err, io.EOF) && strings.Contains(message, `group child "Times"`):
		return fmt.Errorf("KDBX 分组的时间字段格式不兼容（Times 中存在空值或长度不足）: %w", err)
	default:
		return fmt.Errorf("无法解析 KDBX 数据库: %w", err)
	}
}

func createDatabaseAt(path, name, password, keyFile string) (*gokeepasslib.Database, error) {
	creds, err := credentials(password, keyFile)
	if err != nil {
		return nil, fmt.Errorf("密钥文件无效: %w", err)
	}
	db := gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion41())
	db.Credentials = creds
	db.Content.Meta.DatabaseName = strings.TrimSpace(name)
	root := gokeepasslib.NewGroup()
	root.Name = "个人"
	db.Content.Root.Groups = []gokeepasslib.Group{root}
	if err := db.LockProtectedEntries(); err != nil {
		return nil, err
	}
	if err := encodeDatabase(db, path); err != nil {
		return nil, err
	}
	if err := db.UnlockProtectedEntries(); err != nil {
		return nil, err
	}
	return db, nil
}

func writeDatabase(db *gokeepasslib.Database, path string) error {
	if err := db.LockProtectedEntries(); err != nil {
		return err
	}
	defer db.UnlockProtectedEntries()
	return encodeDatabase(db, path)
}

func encodeDatabase(db *gokeepasslib.Database, path string) error {
	var buffer bytes.Buffer
	if err := gokeepasslib.NewEncoder(&buffer).Encode(db); err != nil {
		return fmt.Errorf("无法加密数据库: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".keynest-*.kdbx")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(buffer.Bytes()); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("无法保存数据库: %w", err)
	}
	return nil
}

func snapshotFromDatabase(db *gokeepasslib.Database, path string) Snapshot {
	entries := make([]Entry, 0)
	groups := make([]Group, 0)
	collectEntries(db.Content.Root.Groups, db.Content.Meta.RecycleBinUUID, false, &entries)
	collectGroups(db.Content.Root.Groups, db.Content.Meta.RecycleBinUUID, "", 0, &groups)
	name := strings.TrimSpace(db.Content.Meta.DatabaseName)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return Snapshot{Name: name, Path: path, Entries: entries, Groups: groups, Templates: templatesFromDatabase(db)}
}

func collectGroups(groups []gokeepasslib.Group, recycleBinUUID gokeepasslib.UUID, parentID string, depth int, result *[]Group) {
	for i := range groups {
		group := &groups[i]
		if (!recycleBinUUID.IsZero() && group.UUID.Compare(recycleBinUUID)) || isUngroupedGroup(group) {
			continue
		}
		mapped := mapGroup(group, parentID, depth)
		*result = append(*result, mapped)
		collectGroups(group.Groups, recycleBinUUID, mapped.ID, depth+1, result)
	}
}

func collectEntries(groups []gokeepasslib.Group, recycleBinUUID gokeepasslib.UUID, insideRecycleBin bool, result *[]Entry) {
	for i := range groups {
		group := &groups[i]
		trashed := insideRecycleBin || (!recycleBinUUID.IsZero() && group.UUID.Compare(recycleBinUUID))
		folder := group.Name
		groupID := uuidString(group.UUID)
		if isUngroupedGroup(group) {
			folder = ""
			groupID = ""
		}
		for j := range group.Entries {
			*result = append(*result, mapEntry(&group.Entries[j], folder, groupID, trashed))
		}
		collectEntries(group.Groups, recycleBinUUID, trashed, result)
	}
}

func mapEntry(entry *gokeepasslib.Entry, folder, groupID string, trashed bool) Entry {
	mapped := mapEntryVersion(entry, folder, groupID, trashed)
	for i := range entry.Histories {
		mapped.HistoryCount += len(entry.Histories[i].Entries)
	}
	return mapped
}

func mapEntryVersion(entry *gokeepasslib.Entry, folder, groupID string, trashed bool) Entry {
	id, _ := entry.UUID.MarshalText()
	name := entry.GetTitle()
	if name == "" {
		name = "未命名条目"
	}
	entryTime := entry.Times.LastModificationTime
	deletedAt := ""
	if trashed && entry.Times.LocationChanged != nil {
		entryTime = entry.Times.LocationChanged
		deletedAt = entry.Times.LocationChanged.Time.UTC().Format(time.RFC3339Nano)
	}
	modified := "KDBX"
	if entryTime != nil {
		modified = formatModifiedTime(entryTime.Time, time.Now())
	}
	createdAt := ""
	if entry.Times.CreationTime != nil {
		createdAt = entry.Times.CreationTime.Time.UTC().Format(time.RFC3339Nano)
	}
	modifiedAt := ""
	if entry.Times.LastModificationTime != nil {
		modifiedAt = entry.Times.LastModificationTime.Time.UTC().Format(time.RFC3339Nano)
	}
	lastUsedAt := ""
	lastUsedTime := entry.Times.LastAccessTime
	if lastUsedTime == nil {
		lastUsedTime = entry.Times.LastModificationTime
	}
	if lastUsedTime != nil {
		lastUsedAt = lastUsedTime.Time.UTC().Format(time.RFC3339Nano)
	}
	category := defaultCategory(entry.GetContent("Category"))
	templateID, templateVersion := templateMetadata(entry, category)
	mappedFolder := defaultFolder(folder)
	if groupID == "" {
		mappedFolder = ""
	}
	mapped := Entry{
		ID:              string(id),
		Name:            name,
		Username:        entry.GetContent("UserName"),
		Password:        entry.GetPassword(),
		URL:             entry.GetContent("URL"),
		Note:            entry.GetContent("Notes"),
		GroupID:         groupID,
		Folder:          mappedFolder,
		Category:        category,
		Favorite:        strings.EqualFold(entry.GetContent(favoriteField), "true"),
		Modified:        modified,
		CreatedAt:       createdAt,
		ModifiedAt:      modifiedAt,
		LastUsedAt:      lastUsedAt,
		Color:           entryColor(entry.UUID[0]),
		Initials:        initials(name),
		Tags:            splitTags(entry.Tags),
		Trashed:         trashed,
		DeletedAt:       deletedAt,
		TemplateID:      templateID,
		TemplateVersion: templateVersion,
		Fields:          entryFields(entry),
	}
	return mapped
}

func mapEntryHistory(entry *gokeepasslib.Entry, folder, groupID string) []Entry {
	history := make([]Entry, 0)
	for i := range entry.Histories {
		for j := range entry.Histories[i].Entries {
			version := &entry.Histories[i].Entries[j]
			history = append(history, mapEntryVersion(version, folder, groupID, false))
		}
	}
	sort.SliceStable(history, func(i, j int) bool {
		left, leftErr := time.Parse(time.RFC3339Nano, history[i].ModifiedAt)
		right, rightErr := time.Parse(time.RFC3339Nano, history[j].ModifiedAt)
		if leftErr != nil {
			return false
		}
		if rightErr != nil {
			return true
		}
		return left.After(right)
	})
	return history
}

func formatModifiedTime(value, now time.Time) string {
	localValue := value.Local()
	localNow := now.Local()
	if localValue.Year() != localNow.Year() {
		return localValue.Format("2006 年 01 月 02 日")
	}

	age := now.Sub(value)
	switch {
	case age < 5*time.Minute:
		return "刚刚"
	case age < 24*time.Hour:
		return localValue.Format("15:04")
	default:
		return localValue.Format("01 月 02 日")
	}
}

func mapGroup(group *gokeepasslib.Group, parentID string, depth int) Group {
	return Group{
		ID: uuidString(group.UUID), Name: defaultFolder(group.Name),
		ParentID: parentID, Depth: depth, IconID: group.IconID,
	}
}

func uuidString(id gokeepasslib.UUID) string {
	encoded, _ := id.MarshalText()
	return string(encoded)
}

func findEntry(groups []gokeepasslib.Group, id string) (*gokeepasslib.Entry, *gokeepasslib.Group) {
	target, err := decodeEntryID(id)
	if err != nil {
		return nil, nil
	}
	return findEntryByUUID(groups, target)
}

func decodeEntryID(id string) (gokeepasslib.UUID, error) {
	decoded, err := base64.StdEncoding.DecodeString(id)
	if err != nil || len(decoded) != 16 {
		return gokeepasslib.UUID{}, errors.New("invalid entry id")
	}
	var target gokeepasslib.UUID
	copy(target[:], decoded)
	return target, nil
}

func findEntryByUUID(groups []gokeepasslib.Group, target gokeepasslib.UUID) (*gokeepasslib.Entry, *gokeepasslib.Group) {
	for i := range groups {
		group := &groups[i]
		for j := range group.Entries {
			if group.Entries[j].UUID.Compare(target) {
				return &group.Entries[j], group
			}
		}
		if entry, parent := findEntryByUUID(group.Groups, target); entry != nil {
			return entry, parent
		}
	}
	return nil, nil
}

func findGroup(groups []gokeepasslib.Group, name string) *gokeepasslib.Group {
	for i := range groups {
		if groups[i].Name == name {
			return &groups[i]
		}
		if group := findGroup(groups[i].Groups, name); group != nil {
			return group
		}
	}
	return nil
}

func findGroupByUUID(groups []gokeepasslib.Group, id gokeepasslib.UUID) *gokeepasslib.Group {
	for i := range groups {
		if groups[i].UUID.Compare(id) {
			return &groups[i]
		}
		if group := findGroupByUUID(groups[i].Groups, id); group != nil {
			return group
		}
	}
	return nil
}

func findGroupParentUUID(groups []gokeepasslib.Group, id gokeepasslib.UUID, parent *gokeepasslib.UUID) (*gokeepasslib.UUID, bool) {
	for i := range groups {
		if groups[i].UUID.Compare(id) {
			return parent, true
		}
		current := groups[i].UUID
		if foundParent, found := findGroupParentUUID(groups[i].Groups, id, &current); found {
			return foundParent, true
		}
	}
	return nil, false
}

func findGroupDepth(groups []gokeepasslib.Group, id gokeepasslib.UUID, depth int) (int, bool) {
	for i := range groups {
		if groups[i].UUID.Compare(id) {
			return depth, true
		}
		if nestedDepth, found := findGroupDepth(groups[i].Groups, id, depth+1); found {
			return nestedDepth, true
		}
	}
	return 0, false
}

func setEntryValue(entry *gokeepasslib.Entry, key, content string, protected bool) {
	value := entry.Get(key)
	if value == nil {
		entry.Values = append(entry.Values, gokeepasslib.ValueData{Key: key})
		value = &entry.Values[len(entry.Values)-1]
	}
	value.Value.Content = content
	value.Value.Protected = wrappers.NewBoolWrapper(protected)
}

func entryFields(entry *gokeepasslib.Entry) []EntryField {
	fields := make([]EntryField, 0, len(entry.Values))
	for i := range entry.Values {
		value := entry.Values[i]
		if value.Key == "Title" || value.Key == "Category" || value.Key == favoriteField {
			continue
		}
		fields = append(fields, EntryField{Key: value.Key, Value: value.Value.Content, Protected: value.Value.Protected.Bool})
	}
	return fields
}

func replaceEntryFields(entry *gokeepasslib.Entry, fields []EntryField) {
	values := make([]gokeepasslib.ValueData, 0, len(fields)+3)
	for i := range entry.Values {
		if entry.Values[i].Key == "Title" || entry.Values[i].Key == "Category" || entry.Values[i].Key == favoriteField {
			values = append(values, entry.Values[i])
		}
	}
	entry.Values = values
	for _, field := range fields {
		setEntryValue(entry, field.Key, field.Value, field.Protected)
	}
}

func splitTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	return strings.FieldsFunc(value, func(r rune) bool { return r == ';' || r == ',' })
}

func initials(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return "?"
	}
	if len(runes) == 1 {
		return strings.ToUpper(string(runes))
	}
	return strings.ToUpper(string(runes[:2]))
}

func entryColor(seed byte) string {
	palette := []string{"#285f57", "#2c5871", "#6c4d82", "#8a4f46", "#375b42", "#5c586f"}
	return palette[int(seed)%len(palette)]
}

func defaultFolder(value string) string {
	if strings.TrimSpace(value) == "" || value == "NewDatabase" {
		return "个人"
	}
	return value
}

func defaultCategory(value string) string {
	if strings.TrimSpace(value) == "" {
		return "登录信息"
	}
	return value
}

func SafeFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(value)
	if value == "" {
		return "KeyNest"
	}
	return value
}
