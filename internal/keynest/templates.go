package keynest

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

const (
	templatesMetadataKey    = "KeyNestTemplates"
	templatePreferencesKey  = "KeyNestTemplatePreferences"
	entryTemplateIDKey      = "KeyNestTemplateID"
	entryTemplateVersionKey = "KeyNestTemplateVersion"
)

var templateFieldIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

type templatePreferences struct {
	Order    []string `json:"order"`
	Hidden   []string `json:"hidden"`
	Disabled []string `json:"disabled"`
	Removed  []string `json:"removed"`
}

func builtInTemplates() []EntryTemplate {
	return []EntryTemplate{
		{ID: "login", Version: 1, Name: "Login", BuiltIn: true, Enabled: true, Fields: []TemplateField{
			{ID: "username", StorageKey: "UserName", Label: "Username", Type: "text", Section: "login", Searchable: true},
			{ID: "password", StorageKey: "Password", Label: "Password", Type: "password", Section: "login", Protected: true},
			{ID: "url", StorageKey: "URL", Label: "Website", Type: "url", Section: "website", Searchable: true},
			{ID: "notes", StorageKey: "Notes", Label: "Notes", Type: "textarea", Section: "notes", Searchable: true},
		}},
		{ID: "bank-card", Version: 1, Name: "Bank Card", BuiltIn: true, Enabled: true, Fields: []TemplateField{
			{ID: "cardholder", StorageKey: "UserName", Label: "Cardholder", Type: "text", Section: "card", Searchable: true},
			{ID: "cardNumber", StorageKey: "Card Number", Label: "Card Number", Type: "password", Section: "card", Required: true, Protected: true},
			{ID: "expiryDate", StorageKey: "Expiry Date", Label: "Expiry Date", Type: "month", Section: "card"},
			{ID: "cvv", StorageKey: "CVV", Label: "CVV", Type: "password", Section: "security", Protected: true},
			{ID: "pin", StorageKey: "Password", Label: "PIN", Type: "password", Section: "security", Protected: true},
			{ID: "customerService", StorageKey: "Customer Service", Label: "Customer Service", Type: "text", Section: "contact", Searchable: true},
			{ID: "notes", StorageKey: "Notes", Label: "Notes", Type: "textarea", Section: "notes", Searchable: true},
		}},
		{ID: "wifi", Version: 1, Name: "Wi-Fi", BuiltIn: true, Enabled: true, Fields: []TemplateField{
			{ID: "ssid", StorageKey: "UserName", Label: "Network Name", Type: "text", Section: "network", Required: true, Searchable: true},
			{ID: "password", StorageKey: "Password", Label: "Password", Type: "password", Section: "network", Protected: true},
			{ID: "securityType", StorageKey: "Security Type", Label: "Security Type", Type: "text", Section: "network", Searchable: true},
			{ID: "routerAddress", StorageKey: "URL", Label: "Router Address", Type: "url", Section: "network", Searchable: true},
			{ID: "notes", StorageKey: "Notes", Label: "Notes", Type: "textarea", Section: "notes", Searchable: true},
		}},
		{ID: "secure-note", Version: 1, Name: "Secure Note", BuiltIn: true, Enabled: true, Fields: []TemplateField{
			{ID: "notes", StorageKey: "Notes", Label: "Notes", Type: "textarea", Section: "notes", Required: true, Protected: true},
		}},
		{ID: "identity", Version: 1, Name: "Identity", BuiltIn: true, Enabled: true, Fields: []TemplateField{
			{ID: "fullName", StorageKey: "UserName", Label: "Full Name", Type: "text", Section: "identity", Required: true, Searchable: true},
			{ID: "documentNumber", StorageKey: "Document Number", Label: "Document Number", Type: "password", Section: "identity", Protected: true},
			{ID: "birthDate", StorageKey: "Birth Date", Label: "Birth Date", Type: "date", Section: "identity"},
			{ID: "issuedDate", StorageKey: "Issued Date", Label: "Issued Date", Type: "date", Section: "validity"},
			{ID: "expiryDate", StorageKey: "Expiry Date", Label: "Expiry Date", Type: "date", Section: "validity"},
			{ID: "notes", StorageKey: "Notes", Label: "Notes", Type: "textarea", Section: "notes", Searchable: true},
		}},
	}
}

func newDatabaseTemplatePreferences() templatePreferences {
	preferences := templatePreferences{}
	for _, template := range builtInTemplates() {
		if template.ID != "login" {
			preferences.Removed = append(preferences.Removed, template.ID)
		}
	}
	return preferences
}

func templatesFromDatabase(db *gokeepasslib.Database) []EntryTemplate {
	all := append(builtInTemplates(), customTemplatesFromDatabase(db)...)
	preferences := templatePreferencesFromDatabase(db)
	hidden := make(map[string]struct{}, len(preferences.Hidden))
	for _, id := range preferences.Hidden {
		if id != "login" {
			hidden[id] = struct{}{}
		}
	}
	disabled := make(map[string]struct{}, len(preferences.Disabled))
	for _, id := range preferences.Disabled {
		if id != "login" {
			disabled[id] = struct{}{}
		}
	}
	removed := make(map[string]struct{}, len(preferences.Removed))
	for _, id := range preferences.Removed {
		if id != "login" {
			removed[id] = struct{}{}
		}
	}
	byID := make(map[string]EntryTemplate, len(all))
	for _, template := range all {
		if _, isRemoved := removed[template.ID]; isRemoved {
			continue
		}
		_, template.Deleted = hidden[template.ID]
		_, isDisabled := disabled[template.ID]
		template.Enabled = !isDisabled
		if template.ID == "login" {
			template.Deleted = false
			template.Enabled = true
		}
		byID[template.ID] = template
	}
	ordered := make([]EntryTemplate, 0, len(all))
	seen := make(map[string]struct{}, len(all))
	appendTemplate := func(id string, includeDeleted bool) {
		template, exists := byID[id]
		if !exists || (!includeDeleted && template.Deleted) {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		ordered = append(ordered, template)
		seen[id] = struct{}{}
	}
	appendTemplate("login", false)
	for _, id := range preferences.Order {
		if id != "login" {
			appendTemplate(id, false)
		}
	}
	for _, template := range all {
		appendTemplate(template.ID, false)
	}
	for _, template := range all {
		appendTemplate(template.ID, true)
	}
	return ordered
}

func normalizeTemplate(template *EntryTemplate) {
	template.Name = strings.TrimSpace(template.Name)
	for i := range template.Fields {
		template.Fields[i].ID = strings.TrimSpace(template.Fields[i].ID)
		template.Fields[i].StorageKey = strings.TrimSpace(template.Fields[i].StorageKey)
		template.Fields[i].Label = strings.TrimSpace(template.Fields[i].Label)
		template.Fields[i].Type = strings.ToLower(strings.TrimSpace(template.Fields[i].Type))
		template.Fields[i].Section = strings.TrimSpace(template.Fields[i].Section)
		if template.Fields[i].Section == "" {
			template.Fields[i].Section = "details"
		}
		if template.Fields[i].Type == "password" || template.Fields[i].StorageKey == "Password" {
			template.Fields[i].Protected = true
		}
	}
}

func customTemplatesFromDatabase(db *gokeepasslib.Database) []EntryTemplate {
	value := metaCustomDataValue(db.Content.Meta.CustomData, templatesMetadataKey)
	if value == "" {
		return nil
	}
	var stored []EntryTemplate
	if err := json.Unmarshal([]byte(value), &stored); err != nil {
		return nil
	}
	builtInIDs := make(map[string]struct{})
	for _, template := range builtInTemplates() {
		builtInIDs[template.ID] = struct{}{}
	}
	custom := make([]EntryTemplate, 0, len(stored))
	seen := make(map[string]struct{}, len(stored))
	for _, template := range stored {
		template.BuiltIn = false
		template.Enabled = true
		template.Deleted = false
		normalizeTemplate(&template)
		if _, exists := builtInIDs[template.ID]; exists {
			continue
		}
		if _, exists := seen[template.ID]; exists {
			continue
		}
		if validateTemplate(template) == nil {
			custom = append(custom, template)
			seen[template.ID] = struct{}{}
		}
	}
	return custom
}

func saveCustomTemplates(db *gokeepasslib.Database, templates []EntryTemplate) error {
	stored := make([]EntryTemplate, len(templates))
	for i, template := range templates {
		template.Enabled = true
		template.Deleted = false
		stored[i] = template
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	setMetaCustomDataValue(&db.Content.Meta.CustomData, templatesMetadataKey, string(encoded))
	return nil
}

func templatePreferencesFromDatabase(db *gokeepasslib.Database) templatePreferences {
	value := metaCustomDataValue(db.Content.Meta.CustomData, templatePreferencesKey)
	if value == "" {
		return templatePreferences{}
	}
	var preferences templatePreferences
	if err := json.Unmarshal([]byte(value), &preferences); err != nil {
		return templatePreferences{}
	}
	return preferences
}

func saveTemplatePreferences(db *gokeepasslib.Database, preferences templatePreferences) error {
	encoded, err := json.Marshal(preferences)
	if err != nil {
		return err
	}
	setMetaCustomDataValue(&db.Content.Meta.CustomData, templatePreferencesKey, string(encoded))
	return nil
}

func validateTemplate(template EntryTemplate) error {
	if strings.TrimSpace(template.Name) == "" {
		return errors.New("请输入模板名称")
	}
	if !templateFieldIDPattern.MatchString(template.ID) {
		return errors.New("模板 ID 无效")
	}
	if len(template.Fields) == 0 {
		return errors.New("模板至少需要一个字段")
	}
	if len(template.Fields) > 32 {
		return errors.New("模板字段不能超过 32 个")
	}
	ids := make(map[string]struct{}, len(template.Fields))
	keys := make(map[string]struct{}, len(template.Fields))
	for _, field := range template.Fields {
		if !templateFieldIDPattern.MatchString(field.ID) {
			return fmt.Errorf("字段 ID %q 无效", field.ID)
		}
		if strings.TrimSpace(field.Label) == "" || strings.TrimSpace(field.StorageKey) == "" {
			return errors.New("字段名称不能为空")
		}
		if len(field.StorageKey) > 128 || strings.HasPrefix(field.StorageKey, "KeyNest") || field.StorageKey == "Title" || field.StorageKey == "Category" {
			return fmt.Errorf("字段键 %q 不可用", field.StorageKey)
		}
		if _, exists := ids[field.ID]; exists {
			return fmt.Errorf("字段 ID %q 重复", field.ID)
		}
		if _, exists := keys[field.StorageKey]; exists {
			return fmt.Errorf("字段键 %q 重复", field.StorageKey)
		}
		ids[field.ID] = struct{}{}
		keys[field.StorageKey] = struct{}{}
		switch field.Type {
		case "text", "password", "url", "date", "month", "textarea":
		default:
			return fmt.Errorf("字段类型 %q 不受支持", field.Type)
		}
	}
	return nil
}

func findTemplate(templates []EntryTemplate, id string) (EntryTemplate, bool) {
	for _, template := range templates {
		if template.ID == id {
			return template, true
		}
	}
	return EntryTemplate{}, false
}

func databaseUsesTemplate(db *gokeepasslib.Database, id string) bool {
	return groupsUseTemplate(db.Content.Root.Groups, id)
}

func groupsUseTemplate(groups []gokeepasslib.Group, id string) bool {
	for i := range groups {
		for j := range groups[i].Entries {
			if entryUsesTemplate(&groups[i].Entries[j], id) {
				return true
			}
		}
		if groupsUseTemplate(groups[i].Groups, id) {
			return true
		}
	}
	return false
}

func entryUsesTemplate(entry *gokeepasslib.Entry, id string) bool {
	templateID, _ := templateMetadata(entry, entry.GetContent("Category"))
	if templateID == id {
		return true
	}
	for i := range entry.Histories {
		for j := range entry.Histories[i].Entries {
			if entryUsesTemplate(&entry.Histories[i].Entries[j], id) {
				return true
			}
		}
	}
	return false
}

func inferTemplateID(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "银行卡", "bank card", "card":
		return "bank-card"
	case "wi-fi", "wifi", "无线网络":
		return "wifi"
	case "安全笔记", "secure note", "note":
		return "secure-note"
	case "身份", "identity":
		return "identity"
	default:
		return "login"
	}
}

func templateMetadata(entry *gokeepasslib.Entry, category string) (string, int) {
	id := entryCustomDataValue(entry.CustomData, entryTemplateIDKey)
	if id == "" {
		id = inferTemplateID(category)
	}
	version, _ := strconv.Atoi(entryCustomDataValue(entry.CustomData, entryTemplateVersionKey))
	if version < 1 {
		version = 1
	}
	return id, version
}

func setTemplateMetadata(entry *gokeepasslib.Entry, id string, version int) {
	if version < 1 {
		version = 1
	}
	setEntryCustomDataValue(&entry.CustomData, entryTemplateIDKey, id)
	setEntryCustomDataValue(&entry.CustomData, entryTemplateVersionKey, strconv.Itoa(version))
}

func metaCustomDataValue(items []gokeepasslib.CustomData, key string) string {
	for i := range items {
		if items[i].Key == key {
			return items[i].Value
		}
	}
	return ""
}

func entryCustomDataValue(items []gokeepasslib.CustomData, key string) string {
	return metaCustomDataValue(items, key)
}

func setMetaCustomDataValue(items *[]gokeepasslib.CustomData, key, value string) {
	for i := range *items {
		if (*items)[i].Key == key {
			(*items)[i].Value = value
			now := wrappers.Now()
			(*items)[i].LastModificationTime = &now
			return
		}
	}
	now := wrappers.Now()
	*items = append(*items, gokeepasslib.CustomData{Key: key, Value: value, LastModificationTime: &now})
}

func setEntryCustomDataValue(items *[]gokeepasslib.CustomData, key, value string) {
	setMetaCustomDataValue(items, key, value)
}
