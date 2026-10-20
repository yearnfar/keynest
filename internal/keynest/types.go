package keynest

type RecentDatabase struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Modified string `json:"modified"`
}

type Entry struct {
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

type Group struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
	Depth    int    `json:"depth"`
	IconID   int64  `json:"iconId"`
}

type Snapshot struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	Entries []Entry `json:"entries"`
	Groups  []Group `json:"groups"`
}

type CreateRequest struct {
	Name     string
	Password string
	KeyFile  string
}

type SaveEntryRequest struct {
	ID       string
	Name     string
	Username string
	Password string
	URL      string
	Note     string
	GroupID  string
	Folder   string
	Category string
	Favorite bool
	Tags     []string
}

type SaveEntryResult struct {
	Entry Entry
}
