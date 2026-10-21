package keynest

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

const recycleRetention = 30 * 24 * time.Hour

type Service struct {
	mu     sync.Mutex
	db     *gokeepasslib.Database
	dbPath string
}

func NewService() *Service {
	return &Service{}
}

func ValidateCreateRequest(request CreateRequest) error {
	if strings.TrimSpace(request.Name) == "" {
		return errors.New("请输入保险库名称")
	}
	if request.Password == "" && request.KeyFile == "" {
		return errors.New("请设置主密码或密钥文件")
	}
	return nil
}

func (s *Service) ListRecentDatabases() []RecentDatabase {
	return listRecentDatabases()
}

func (s *Service) Unlock(path, password, keyFile string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	db, err := decodeDatabase(path, password, keyFile)
	if err != nil {
		return Snapshot{}, err
	}
	if purgeExpiredRecycleBin(db, time.Now()) > 0 {
		if err := writeDatabase(db, path); err != nil {
			return Snapshot{}, fmt.Errorf("无法清理过期回收站记录: %w", err)
		}
	}
	s.db = db
	s.dbPath = path
	addRecent(path)
	return snapshotFromDatabase(db, path), nil
}

func (s *Service) Create(path string, request CreateRequest) (Snapshot, error) {
	if err := ValidateCreateRequest(request); err != nil {
		return Snapshot{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := createDatabaseAt(path, request.Name, request.Password, request.KeyFile)
	if err != nil {
		return Snapshot{}, err
	}
	s.db = db
	s.dbPath = path
	addRecent(path)
	return snapshotFromDatabase(db, path), nil
}

func (s *Service) SaveEntry(request SaveEntryRequest) (SaveEntryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return SaveEntryResult{}, errors.New("保险库尚未解锁")
	}
	templates := templatesFromDatabase(s.db)
	templateID := strings.TrimSpace(request.TemplateID)
	if templateID == "" {
		templateID = inferTemplateID(request.Category)
	}
	template, found := findTemplate(templates, templateID)
	if !found {
		return SaveEntryResult{}, errors.New("条目模板不存在")
	}
	fields, err := normalizeEntryFields(request.Fields, template, request.ID == "")
	if err != nil {
		return SaveEntryResult{}, err
	}

	entry, group := findEntry(s.db.Content.Root.Groups, request.ID)
	if entry == nil {
		if request.GroupID != "" {
			groupID, err := decodeEntryID(request.GroupID)
			if err != nil {
				return SaveEntryResult{}, errors.New("分组不存在")
			}
			group = findGroupByUUID(s.db.Content.Root.Groups, groupID)
			recycleBin := recycleBinGroup(s.db)
			if group == nil || (recycleBin != nil && groupContains(recycleBin, group.UUID)) {
				return SaveEntryResult{}, errors.New("分组不存在")
			}
		}
		if group == nil {
			group = findGroup(s.db.Content.Root.Groups, request.Folder)
		}
		if group == nil && request.GroupID == "" && strings.TrimSpace(request.Folder) == "" {
			group = ensureUngroupedGroup(s.db)
		}
		if group == nil {
			if len(s.db.Content.Root.Groups) == 0 {
				newGroup := gokeepasslib.NewGroup()
				newGroup.Name = defaultFolder(request.Folder)
				s.db.Content.Root.Groups = append(s.db.Content.Root.Groups, newGroup)
			}
			group = &s.db.Content.Root.Groups[0]
		}
		newEntry := gokeepasslib.NewEntry()
		group.Entries = append(group.Entries, newEntry)
		entry = &group.Entries[len(group.Entries)-1]
	} else {
		var destination *gokeepasslib.Group
		if request.GroupID != "" {
			groupID, err := decodeEntryID(request.GroupID)
			if err != nil {
				return SaveEntryResult{}, errors.New("分组不存在")
			}
			destination = findGroupByUUID(s.db.Content.Root.Groups, groupID)
			recycleBin := recycleBinGroup(s.db)
			if destination == nil || isUngroupedGroup(destination) || (recycleBin != nil && groupContains(recycleBin, destination.UUID)) {
				return SaveEntryResult{}, errors.New("分组不存在")
			}
		} else if strings.TrimSpace(request.Folder) != "" {
			destination = findGroup(s.db.Content.Root.Groups, request.Folder)
			recycleBin := recycleBinGroup(s.db)
			if destination == nil || isUngroupedGroup(destination) || (recycleBin != nil && groupContains(recycleBin, destination.UUID)) {
				return SaveEntryResult{}, errors.New("分组不存在")
			}
		} else {
			destination = ensureUngroupedGroup(s.db)
			entry, group = findEntry(s.db.Content.Root.Groups, request.ID)
		}
		if entry == nil || group == nil {
			return SaveEntryResult{}, errors.New("记录不存在")
		}

		appendEntryHistory(entry, s.db.Content.Meta.HistoryMaxItems)
		if !group.UUID.Compare(destination.UUID) {
			entryID := entry.UUID
			previousParent := group.UUID
			moved, found := popEntry(group, entryID)
			if !found {
				return SaveEntryResult{}, errors.New("记录不存在")
			}
			now := wrappers.Now()
			moved.PreviousParentGroup = &previousParent
			moved.Times.LocationChanged = &now
			destination.Entries = append(destination.Entries, moved)
			entry = &destination.Entries[len(destination.Entries)-1]
			group = destination
		}
	}

	setEntryValue(entry, "Title", request.Name, false)
	if request.Fields != nil {
		replaceEntryFields(entry, fields)
	} else {
		setEntryValue(entry, "UserName", request.Username, false)
		setEntryValue(entry, "Password", request.Password, true)
		setEntryValue(entry, "URL", request.URL, false)
		setEntryValue(entry, "Notes", request.Note, false)
	}
	setEntryValue(entry, "Category", defaultCategory(request.Category), false)
	setEntryValue(entry, favoriteField, fmt.Sprintf("%t", request.Favorite), false)
	setTemplateMetadata(entry, template.ID, template.Version)
	entry.Tags = strings.Join(request.Tags, ";")
	now := wrappers.Now()
	entry.Times.LastModificationTime = &now
	savedFolder := group.Name
	savedGroupID := uuidString(group.UUID)
	if isUngroupedGroup(group) {
		savedFolder = ""
		savedGroupID = ""
	}
	saved := mapEntry(entry, savedFolder, savedGroupID, isRecycleBinGroup(s.db, group))

	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return SaveEntryResult{}, err
	}
	return SaveEntryResult{Entry: saved}, nil
}

func normalizeEntryFields(fields []EntryField, template EntryTemplate, validateRequired bool) ([]EntryField, error) {
	if fields == nil {
		return nil, nil
	}
	if len(fields) > 64 {
		return nil, errors.New("条目字段不能超过 64 个")
	}
	rules := make(map[string]TemplateField, len(template.Fields))
	for _, field := range template.Fields {
		rules[field.StorageKey] = field
	}
	seen := make(map[string]struct{}, len(fields))
	normalized := make([]EntryField, 0, len(fields))
	for _, field := range fields {
		field.Key = strings.TrimSpace(field.Key)
		if field.Key == "" || len(field.Key) > 128 || field.Key == "Title" || field.Key == "Category" || field.Key == favoriteField {
			return nil, fmt.Errorf("字段键 %q 不可用", field.Key)
		}
		if _, exists := seen[field.Key]; exists {
			return nil, fmt.Errorf("字段键 %q 重复", field.Key)
		}
		if rule, exists := rules[field.Key]; exists {
			if rule.Protected || rule.Type == "password" {
				field.Protected = true
			}
		}
		if field.Key == "Password" {
			field.Protected = true
		}
		seen[field.Key] = struct{}{}
		normalized = append(normalized, field)
	}
	if validateRequired {
		for _, rule := range template.Fields {
			if rule.Required {
				present := false
				for _, field := range normalized {
					if field.Key == rule.StorageKey && strings.TrimSpace(field.Value) != "" {
						present = true
						break
					}
				}
				if !present {
					return nil, fmt.Errorf("%s 为必填字段", rule.Label)
				}
			}
		}
	}
	return normalized, nil
}

func (s *Service) SaveTemplate(template EntryTemplate) (EntryTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return EntryTemplate{}, errors.New("保险库尚未解锁")
	}
	if template.BuiltIn {
		return EntryTemplate{}, errors.New("内置模板不能修改")
	}
	custom := customTemplatesFromDatabase(s.db)
	creating := template.ID == ""
	if creating {
		id := gokeepasslib.NewUUID()
		template.ID = "custom-" + strings.TrimRight(base64.RawURLEncoding.EncodeToString(id[:]), "=")
		template.Version = 1
	} else {
		for _, builtIn := range builtInTemplates() {
			if builtIn.ID == template.ID {
				return EntryTemplate{}, errors.New("内置模板不能修改")
			}
		}
	}
	template.BuiltIn = false
	template.Deleted = false
	normalizeTemplate(&template)
	if err := validateTemplate(template); err != nil {
		return EntryTemplate{}, err
	}
	updated := false
	for i := range custom {
		if custom[i].ID == template.ID {
			template.Version = custom[i].Version + 1
			custom[i] = template
			updated = true
			break
		}
	}
	if !updated {
		if !creating {
			return EntryTemplate{}, errors.New("模板不存在")
		}
		custom = append(custom, template)
	}
	if err := saveCustomTemplates(s.db, custom); err != nil {
		return EntryTemplate{}, err
	}
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return EntryTemplate{}, err
	}
	saved, found := findTemplate(templatesFromDatabase(s.db), template.ID)
	if !found {
		return EntryTemplate{}, errors.New("模板保存失败")
	}
	return saved, nil
}

func (s *Service) DeleteTemplate(id string) ([]EntryTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return nil, errors.New("保险库尚未解锁")
	}
	id = strings.TrimSpace(id)
	if id == "login" {
		return nil, errors.New("登录信息模板不能删除")
	}
	templates := templatesFromDatabase(s.db)
	template, found := findTemplate(templates, id)
	if !found || template.Deleted {
		return nil, errors.New("模板不存在")
	}
	preferences := templatePreferencesFromDatabase(s.db)
	inUse := databaseUsesTemplate(s.db, id)
	if !inUse {
		if template.BuiltIn {
			preferences.Removed = append(preferences.Removed, id)
		} else {
			custom := customTemplatesFromDatabase(s.db)
			retained := custom[:0]
			for _, candidate := range custom {
				if candidate.ID != id {
					retained = append(retained, candidate)
				}
			}
			if err := saveCustomTemplates(s.db, retained); err != nil {
				return nil, err
			}
		}
		hidden := preferences.Hidden[:0]
		for _, hiddenID := range preferences.Hidden {
			if hiddenID != id {
				hidden = append(hidden, hiddenID)
			}
		}
		preferences.Hidden = hidden
	} else {
		if !slices.Contains(preferences.Hidden, id) {
			preferences.Hidden = append(preferences.Hidden, id)
		}
	}
	order := preferences.Order[:0]
	for _, orderedID := range preferences.Order {
		if orderedID != id {
			order = append(order, orderedID)
		}
	}
	preferences.Order = order
	disabled := preferences.Disabled[:0]
	for _, disabledID := range preferences.Disabled {
		if disabledID != id {
			disabled = append(disabled, disabledID)
		}
	}
	preferences.Disabled = disabled
	if err := saveTemplatePreferences(s.db, preferences); err != nil {
		return nil, err
	}
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return nil, err
	}
	return templatesFromDatabase(s.db), nil
}

func (s *Service) SetTemplateEnabled(id string, enabled bool) ([]EntryTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return nil, errors.New("保险库尚未解锁")
	}
	id = strings.TrimSpace(id)
	if id == "login" {
		return nil, errors.New("新增条目模板必须保持启用")
	}
	template, found := findTemplate(templatesFromDatabase(s.db), id)
	if !found || template.Deleted {
		return nil, errors.New("模板不存在")
	}
	preferences := templatePreferencesFromDatabase(s.db)
	disabled := make([]string, 0, len(preferences.Disabled)+1)
	seen := false
	for _, disabledID := range preferences.Disabled {
		if disabledID == id {
			if !enabled && !seen {
				disabled = append(disabled, id)
				seen = true
			}
			continue
		}
		disabled = append(disabled, disabledID)
	}
	if !enabled && !seen {
		disabled = append(disabled, id)
	}
	preferences.Disabled = disabled
	if err := saveTemplatePreferences(s.db, preferences); err != nil {
		return nil, err
	}
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return nil, err
	}
	return templatesFromDatabase(s.db), nil
}

func (s *Service) ReorderTemplates(ids []string) ([]EntryTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return nil, errors.New("保险库尚未解锁")
	}
	templates := templatesFromDatabase(s.db)
	expected := make(map[string]struct{})
	for _, template := range templates {
		if template.ID != "login" && !template.Deleted {
			expected[template.ID] = struct{}{}
		}
	}
	if len(ids) != len(expected) {
		return nil, errors.New("模板顺序无效")
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := expected[id]; !exists {
			return nil, errors.New("模板顺序无效")
		}
		if _, exists := seen[id]; exists {
			return nil, errors.New("模板顺序无效")
		}
		seen[id] = struct{}{}
	}
	preferences := templatePreferencesFromDatabase(s.db)
	preferences.Order = append([]string(nil), ids...)
	if err := saveTemplatePreferences(s.db, preferences); err != nil {
		return nil, err
	}
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return nil, err
	}
	return templatesFromDatabase(s.db), nil
}

func appendEntryHistory(entry *gokeepasslib.Entry, maxItems int64) {
	if maxItems == 0 {
		return
	}

	version := *entry
	version.Values = append([]gokeepasslib.ValueData(nil), entry.Values...)
	version.Binaries = append([]gokeepasslib.BinaryReference(nil), entry.Binaries...)
	version.CustomData = append([]gokeepasslib.CustomData(nil), entry.CustomData...)
	version.Histories = nil

	entries := make([]gokeepasslib.Entry, 0)
	for _, history := range entry.Histories {
		entries = append(entries, history.Entries...)
	}
	entries = append(entries, version)
	if maxItems > 0 && int64(len(entries)) > maxItems {
		entries = entries[len(entries)-int(maxItems):]
	}
	entry.Histories = []gokeepasslib.History{{Entries: entries}}
}

func (s *Service) GetEntryHistory(id string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return nil, errors.New("保险库尚未解锁")
	}

	entry, group := findEntry(s.db.Content.Root.Groups, id)
	if entry == nil || group == nil {
		return nil, errors.New("记录不存在")
	}
	folder := group.Name
	groupID := uuidString(group.UUID)
	if isUngroupedGroup(group) {
		folder = ""
		groupID = ""
	}
	return mapEntryHistory(entry, folder, groupID), nil
}

func (s *Service) TouchEntry(id string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return Entry{}, errors.New("保险库尚未解锁")
	}

	target, err := decodeEntryID(id)
	if err != nil {
		return Entry{}, errors.New("记录不存在")
	}
	entry, group := findEntryByUUID(s.db.Content.Root.Groups, target)
	if entry == nil || group == nil {
		return Entry{}, errors.New("记录不存在")
	}
	recycleBin := recycleBinGroup(s.db)
	if recycleBin != nil && groupContains(recycleBin, group.UUID) {
		return Entry{}, errors.New("回收站记录不能加入最近使用")
	}

	now := wrappers.Now()
	entry.Times.LastAccessTime = &now
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return Entry{}, err
	}
	folder := group.Name
	groupID := uuidString(group.UUID)
	if isUngroupedGroup(group) {
		folder = ""
		groupID = ""
	}
	return mapEntry(entry, folder, groupID, false), nil
}

func (s *Service) ChangePassword(currentPassword, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" || s.db.Credentials == nil {
		return errors.New("保险库尚未解锁")
	}
	if newPassword == "" {
		return errors.New("请输入新主密码")
	}
	if s.db.Credentials.Passphrase != nil {
		currentCredentials := gokeepasslib.NewPasswordCredentials(currentPassword)
		if !bytes.Equal(currentCredentials.Passphrase, s.db.Credentials.Passphrase) {
			return errors.New("当前主密码不正确")
		}
	}

	previousCredentials := s.db.Credentials
	updatedCredentials := gokeepasslib.NewPasswordCredentials(newPassword)
	updatedCredentials.Key = append([]byte(nil), previousCredentials.Key...)
	updatedCredentials.Windows = append([]byte(nil), previousCredentials.Windows...)
	s.db.Credentials = updatedCredentials
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		s.db.Credentials = previousCredentials
		return err
	}
	return nil
}

func (s *Service) EmptyRecycleBin() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return 0, errors.New("保险库尚未解锁")
	}

	recycleBin := recycleBinGroup(s.db)
	if recycleBin == nil || (len(recycleBin.Entries) == 0 && len(recycleBin.Groups) == 0) {
		return 0, nil
	}
	removed := emptyRecycleBinContents(s.db.Content.Root, recycleBin, wrappers.Now())
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return 0, err
	}
	return removed, nil
}

func (s *Service) CreateGroup(name, parentID string, iconID int64) (Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return Group{}, errors.New("保险库尚未解锁")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, errors.New("请输入分组名称")
	}
	if iconID < 0 || iconID > 68 {
		return Group{}, errors.New("分组图标无效")
	}

	newGroup := gokeepasslib.NewGroup()
	newGroup.Name = name
	newGroup.IconID = iconID
	depth := 0
	if parentID == "" {
		s.db.Content.Root.Groups = append(s.db.Content.Root.Groups, newGroup)
	} else {
		parentUUID, err := decodeEntryID(parentID)
		if err != nil {
			return Group{}, errors.New("父分组不存在")
		}
		parent := findGroupByUUID(s.db.Content.Root.Groups, parentUUID)
		recycleBin := recycleBinGroup(s.db)
		if parent == nil || isUngroupedGroup(parent) || (recycleBin != nil && groupContains(recycleBin, parent.UUID)) {
			return Group{}, errors.New("父分组不存在")
		}
		if parentDepth, found := findGroupDepth(s.db.Content.Root.Groups, parent.UUID, 0); found {
			depth = parentDepth + 1
		}
		parent.Groups = append(parent.Groups, newGroup)
	}
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return Group{}, err
	}
	return mapGroup(&newGroup, parentID, depth), nil
}

func (s *Service) UpdateGroup(id, name, parentID string, iconID int64) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return Snapshot{}, errors.New("保险库尚未解锁")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Snapshot{}, errors.New("请输入分组名称")
	}
	if iconID < 0 || iconID > 68 {
		return Snapshot{}, errors.New("分组图标无效")
	}
	targetUUID, err := decodeEntryID(id)
	if err != nil {
		return Snapshot{}, errors.New("分组不存在")
	}
	target := findGroupByUUID(s.db.Content.Root.Groups, targetUUID)
	recycleBin := recycleBinGroup(s.db)
	if target == nil || isUngroupedGroup(target) || (recycleBin != nil && groupContains(recycleBin, target.UUID)) {
		return Snapshot{}, errors.New("分组不存在")
	}

	currentParent, found := findGroupParentUUID(s.db.Content.Root.Groups, targetUUID, nil)
	if !found {
		return Snapshot{}, errors.New("分组不存在")
	}
	parentChanged := (currentParent == nil) != (parentID == "")
	if currentParent != nil && parentID != "" {
		parentUUID, decodeErr := decodeEntryID(parentID)
		parentChanged = decodeErr != nil || !currentParent.Compare(parentUUID)
	}
	if parentChanged {
		return Snapshot{}, errors.New("编辑分组不能修改父分组")
	}

	target.Name = name
	target.IconID = iconID
	now := wrappers.Now()
	target.Times.LastModificationTime = &now
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return Snapshot{}, err
	}
	return snapshotFromDatabase(s.db, s.dbPath), nil
}

func (s *Service) MoveGroup(id, parentID string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return Snapshot{}, errors.New("保险库尚未解锁")
	}
	targetUUID, err := decodeEntryID(id)
	if err != nil {
		return Snapshot{}, errors.New("分组不存在")
	}
	target := findGroupByUUID(s.db.Content.Root.Groups, targetUUID)
	if target == nil {
		return Snapshot{}, errors.New("分组不存在")
	}
	return s.updateGroup(id, target.Name, parentID, target.IconID, true)
}

func (s *Service) updateGroup(id, name, parentID string, iconID int64, insertFirst bool) (Snapshot, error) {
	if s.db == nil || s.dbPath == "" {
		return Snapshot{}, errors.New("保险库尚未解锁")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Snapshot{}, errors.New("请输入分组名称")
	}
	if iconID < 0 || iconID > 68 {
		return Snapshot{}, errors.New("分组图标无效")
	}
	targetUUID, err := decodeEntryID(id)
	if err != nil {
		return Snapshot{}, errors.New("分组不存在")
	}
	target := findGroupByUUID(s.db.Content.Root.Groups, targetUUID)
	recycleBin := recycleBinGroup(s.db)
	if target == nil || isUngroupedGroup(target) || (recycleBin != nil && groupContains(recycleBin, target.UUID)) {
		return Snapshot{}, errors.New("分组不存在")
	}

	var parentUUID gokeepasslib.UUID
	if parentID != "" {
		parentUUID, err = decodeEntryID(parentID)
		if err != nil {
			return Snapshot{}, errors.New("父分组不存在")
		}
		parent := findGroupByUUID(s.db.Content.Root.Groups, parentUUID)
		if parent == nil || isUngroupedGroup(parent) || (recycleBin != nil && groupContains(recycleBin, parent.UUID)) {
			return Snapshot{}, errors.New("父分组不存在")
		}
		if groupContains(target, parentUUID) {
			return Snapshot{}, errors.New("不能将分组移动到自身或其子分组中")
		}
	}

	moved, previousParent, found := popGroup(&s.db.Content.Root.Groups, targetUUID)
	if !found {
		return Snapshot{}, errors.New("分组不存在")
	}
	moved.Name = name
	moved.IconID = iconID
	now := wrappers.Now()
	moved.Times.LastModificationTime = &now
	parentChanged := (previousParent == nil) != (parentID == "")
	if previousParent != nil && parentID != "" && !previousParent.Compare(parentUUID) {
		parentChanged = true
	}
	if parentChanged {
		moved.Times.LocationChanged = &now
	}
	if parentID == "" {
		if insertFirst {
			s.db.Content.Root.Groups = append([]gokeepasslib.Group{moved}, s.db.Content.Root.Groups...)
		} else {
			s.db.Content.Root.Groups = append(s.db.Content.Root.Groups, moved)
		}
	} else {
		parent := findGroupByUUID(s.db.Content.Root.Groups, parentUUID)
		if parent == nil {
			return Snapshot{}, errors.New("父分组不存在")
		}
		if insertFirst {
			parent.Groups = append([]gokeepasslib.Group{moved}, parent.Groups...)
		} else {
			parent.Groups = append(parent.Groups, moved)
		}
	}
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return Snapshot{}, err
	}
	return snapshotFromDatabase(s.db, s.dbPath), nil
}

func (s *Service) DeleteGroup(id string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return Snapshot{}, errors.New("保险库尚未解锁")
	}
	targetUUID, err := decodeEntryID(id)
	if err != nil {
		return Snapshot{}, errors.New("分组不存在")
	}
	target := findGroupByUUID(s.db.Content.Root.Groups, targetUUID)
	recycleBin := recycleBinGroup(s.db)
	if target == nil || isUngroupedGroup(target) || (recycleBin != nil && groupContains(recycleBin, target.UUID)) {
		return Snapshot{}, errors.New("分组不存在")
	}

	moved, _, found := popGroup(&s.db.Content.Root.Groups, targetUUID)
	if !found {
		return Snapshot{}, errors.New("分组不存在")
	}
	now := wrappers.Now()
	entries := collectGroupEntries(&moved)
	if len(entries) > 0 {
		destination := ensureUngroupedGroup(s.db)
		for i := range entries {
			entries[i].PreviousParentGroup = nil
			locationChanged := now
			lastModified := now
			entries[i].Times.LocationChanged = &locationChanged
			entries[i].Times.LastModificationTime = &lastModified
			destination.Entries = append(destination.Entries, entries[i])
		}
	}
	appendDeletedGroupStructure(s.db.Content.Root, &moved, now)
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return Snapshot{}, err
	}
	return snapshotFromDatabase(s.db, s.dbPath), nil
}

func (s *Service) MoveEntryToRecycleBin(id string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return Entry{}, errors.New("保险库尚未解锁")
	}

	target, err := decodeEntryID(id)
	if err != nil {
		return Entry{}, errors.New("记录不存在")
	}
	entry, parent := findEntryByUUID(s.db.Content.Root.Groups, target)
	if entry == nil || parent == nil {
		return Entry{}, errors.New("记录不存在")
	}
	recycleBin := recycleBinGroup(s.db)
	if recycleBin != nil && groupContains(recycleBin, parent.UUID) {
		return Entry{}, errors.New("记录已在回收站中")
	}

	previousParent := parent.UUID
	moved, found := popEntry(parent, target)
	if !found {
		return Entry{}, errors.New("记录不存在")
	}
	moved.PreviousParentGroup = &previousParent
	now := wrappers.Now()
	moved.Times.LocationChanged = &now
	recycleBin = ensureRecycleBin(s.db)
	recycleBin.Entries = append(recycleBin.Entries, moved)
	recycled := mapEntry(&recycleBin.Entries[len(recycleBin.Entries)-1], recycleBin.Name, uuidString(recycleBin.UUID), true)
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return Entry{}, err
	}
	return recycled, nil
}

func (s *Service) RestoreRecycleBinEntry(id string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return Entry{}, errors.New("保险库尚未解锁")
	}

	recycleBin := recycleBinGroup(s.db)
	if recycleBin == nil {
		return Entry{}, errors.New("回收站记录不存在")
	}
	target, err := decodeEntryID(id)
	if err != nil {
		return Entry{}, errors.New("回收站记录不存在")
	}
	entry, found := popEntry(recycleBin, target)
	if !found {
		return Entry{}, errors.New("回收站记录不存在")
	}

	destination := restoreDestination(s.db, recycleBin, entry.PreviousParentGroup)
	entry.PreviousParentGroup = nil
	now := wrappers.Now()
	entry.Times.LocationChanged = &now
	entry.Times.LastAccessTime = &now
	destination.Entries = append(destination.Entries, entry)
	folder := destination.Name
	groupID := uuidString(destination.UUID)
	if isUngroupedGroup(destination) {
		folder = ""
		groupID = ""
	}
	restored := mapEntry(&destination.Entries[len(destination.Entries)-1], folder, groupID, false)
	if err := writeDatabase(s.db, s.dbPath); err != nil {
		return Entry{}, err
	}
	return restored, nil
}

func (s *Service) DeleteRecycleBinEntry(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || s.dbPath == "" {
		return errors.New("保险库尚未解锁")
	}

	recycleBin := recycleBinGroup(s.db)
	if recycleBin == nil {
		return errors.New("回收站记录不存在")
	}
	target, err := decodeEntryID(id)
	if err != nil {
		return errors.New("回收站记录不存在")
	}
	entry, found := popEntry(recycleBin, target)
	if !found {
		return errors.New("回收站记录不存在")
	}
	appendDeletedObject(s.db.Content.Root, entry.UUID, wrappers.Now())
	return writeDatabase(s.db, s.dbPath)
}

func (s *Service) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = nil
	s.dbPath = ""
}
