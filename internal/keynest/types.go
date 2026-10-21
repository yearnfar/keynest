package keynest

type RecentDatabase struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Modified string `json:"modified"`
}

type Entry struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Username        string       `json:"username"`
	Password        string       `json:"password"`
	URL             string       `json:"url"`
	Note            string       `json:"note"`
	GroupID         string       `json:"groupId"`
	Folder          string       `json:"folder"`
	Category        string       `json:"category"`
	Favorite        bool         `json:"favorite"`
	Modified        string       `json:"modified"`
	CreatedAt       string       `json:"createdAt"`
	ModifiedAt      string       `json:"modifiedAt"`
	LastUsedAt      string       `json:"lastUsedAt"`
	Color           string       `json:"color"`
	Initials        string       `json:"initials"`
	Tags            []string     `json:"tags"`
	Trashed         bool         `json:"trashed"`
	DeletedAt       string       `json:"deletedAt,omitempty"`
	HistoryCount    int          `json:"historyCount"`
	TemplateID      string       `json:"templateId"`
	TemplateVersion int          `json:"templateVersion"`
	Fields          []EntryField `json:"fields"`
}

type EntryField struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Protected bool   `json:"protected"`
}

type TemplateField struct {
	ID         string `json:"id"`
	StorageKey string `json:"storageKey"`
	Label      string `json:"label"`
	Type       string `json:"type"`
	Section    string `json:"section"`
	Required   bool   `json:"required"`
	Protected  bool   `json:"protected"`
	Searchable bool   `json:"searchable"`
}

type EntryTemplate struct {
	ID      string          `json:"id"`
	Version int             `json:"version"`
	Name    string          `json:"name"`
	BuiltIn bool            `json:"builtIn"`
	Enabled bool            `json:"enabled"`
	Deleted bool            `json:"deleted,omitempty"`
	Fields  []TemplateField `json:"fields"`
}

type Group struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
	Depth    int    `json:"depth"`
	IconID   int64  `json:"iconId"`
}

type Snapshot struct {
	Name      string          `json:"name"`
	Path      string          `json:"path"`
	Entries   []Entry         `json:"entries"`
	Groups    []Group         `json:"groups"`
	Templates []EntryTemplate `json:"templates"`
}

type CreateRequest struct {
	Name     string
	Password string
	KeyFile  string
}

type SaveEntryRequest struct {
	ID              string
	Name            string
	Username        string
	Password        string
	URL             string
	Note            string
	GroupID         string
	Folder          string
	Category        string
	Favorite        bool
	Tags            []string
	TemplateID      string
	TemplateVersion int
	Fields          []EntryField
}

type SaveEntryResult struct {
	Entry Entry
}
