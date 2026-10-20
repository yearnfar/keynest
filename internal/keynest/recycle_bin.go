package keynest

import (
	"time"

	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
)

func purgeExpiredRecycleBin(db *gokeepasslib.Database, now time.Time) int {
	recycleBin := recycleBinGroup(db)
	if recycleBin == nil {
		return 0
	}
	return purgeExpiredEntries(db.Content.Root, recycleBin, now.Add(-recycleRetention), wrappers.Now())
}

func purgeExpiredEntries(root *gokeepasslib.RootData, group *gokeepasslib.Group, cutoff time.Time, deletedAt wrappers.TimeWrapper) int {
	retained := make([]gokeepasslib.Entry, 0, len(group.Entries))
	removed := 0
	for i := range group.Entries {
		entry := group.Entries[i]
		if entry.Times.LocationChanged != nil && entry.Times.LocationChanged.Time.Before(cutoff) {
			appendDeletedObject(root, entry.UUID, deletedAt)
			removed++
			continue
		}
		retained = append(retained, entry)
	}
	group.Entries = retained
	for i := range group.Groups {
		removed += purgeExpiredEntries(root, &group.Groups[i], cutoff, deletedAt)
	}
	return removed
}

func emptyRecycleBinContents(root *gokeepasslib.RootData, recycleBin *gokeepasslib.Group, deletedAt wrappers.TimeWrapper) int {
	removed := 0
	for i := range recycleBin.Entries {
		appendDeletedObject(root, recycleBin.Entries[i].UUID, deletedAt)
		removed++
	}
	for i := range recycleBin.Groups {
		removed += appendDeletedGroup(root, &recycleBin.Groups[i], deletedAt)
	}
	recycleBin.Entries = nil
	recycleBin.Groups = nil
	now := deletedAt
	recycleBin.Times.LastModificationTime = &now
	return removed
}

func appendDeletedGroup(root *gokeepasslib.RootData, group *gokeepasslib.Group, deletedAt wrappers.TimeWrapper) int {
	removed := 0
	for i := range group.Entries {
		appendDeletedObject(root, group.Entries[i].UUID, deletedAt)
		removed++
	}
	for i := range group.Groups {
		removed += appendDeletedGroup(root, &group.Groups[i], deletedAt)
	}
	appendDeletedObject(root, group.UUID, deletedAt)
	return removed
}

func appendDeletedObject(root *gokeepasslib.RootData, id gokeepasslib.UUID, deletedAt wrappers.TimeWrapper) {
	timestamp := deletedAt
	root.DeletedObjects = append(root.DeletedObjects, gokeepasslib.DeletedObjectData{
		UUID:         id,
		DeletionTime: &timestamp,
	})
}

func recycleBinGroup(db *gokeepasslib.Database) *gokeepasslib.Group {
	if db.Content.Meta.RecycleBinUUID.IsZero() {
		return nil
	}
	return findGroupByUUID(db.Content.Root.Groups, db.Content.Meta.RecycleBinUUID)
}

func restoreDestination(db *gokeepasslib.Database, recycleBin *gokeepasslib.Group, previousParent *gokeepasslib.UUID) *gokeepasslib.Group {
	if previousParent != nil && !groupContains(recycleBin, *previousParent) {
		if group := findGroupByUUID(db.Content.Root.Groups, *previousParent); group != nil {
			return group
		}
	}
	return ensureUngroupedGroup(db)
}

func groupContains(group *gokeepasslib.Group, id gokeepasslib.UUID) bool {
	if group.UUID.Compare(id) {
		return true
	}
	return findGroupByUUID(group.Groups, id) != nil
}

func popEntry(group *gokeepasslib.Group, target gokeepasslib.UUID) (gokeepasslib.Entry, bool) {
	for i := range group.Entries {
		if group.Entries[i].UUID.Compare(target) {
			entry := group.Entries[i]
			group.Entries = append(group.Entries[:i], group.Entries[i+1:]...)
			return entry, true
		}
	}
	for i := range group.Groups {
		if entry, found := popEntry(&group.Groups[i], target); found {
			return entry, true
		}
	}
	return gokeepasslib.Entry{}, false
}

func popGroup(groups *[]gokeepasslib.Group, target gokeepasslib.UUID) (gokeepasslib.Group, *gokeepasslib.UUID, bool) {
	for i := range *groups {
		if (*groups)[i].UUID.Compare(target) {
			group := (*groups)[i]
			*groups = append((*groups)[:i], (*groups)[i+1:]...)
			return group, nil, true
		}
		parentUUID := (*groups)[i].UUID
		if group, _, found := popGroup(&(*groups)[i].Groups, target); found {
			return group, &parentUUID, true
		}
	}
	return gokeepasslib.Group{}, nil, false
}

func collectGroupEntries(group *gokeepasslib.Group) []gokeepasslib.Entry {
	entries := append([]gokeepasslib.Entry(nil), group.Entries...)
	for i := range group.Groups {
		entries = append(entries, collectGroupEntries(&group.Groups[i])...)
	}
	return entries
}

func appendDeletedGroupStructure(root *gokeepasslib.RootData, group *gokeepasslib.Group, deletedAt wrappers.TimeWrapper) {
	for i := range group.Groups {
		appendDeletedGroupStructure(root, &group.Groups[i], deletedAt)
	}
	appendDeletedObject(root, group.UUID, deletedAt)
}

func isUngroupedGroup(group *gokeepasslib.Group) bool {
	if group == nil {
		return false
	}
	for _, item := range group.CustomData {
		if item.Key == internalGroupField && item.Value == ungroupedGroupMarker {
			return true
		}
	}
	return false
}

func ensureUngroupedGroup(db *gokeepasslib.Database) *gokeepasslib.Group {
	for i := range db.Content.Root.Groups {
		if isUngroupedGroup(&db.Content.Root.Groups[i]) {
			return &db.Content.Root.Groups[i]
		}
	}
	group := gokeepasslib.NewGroup()
	group.Name = "KeyNest Ungrouped"
	group.IconID = 48
	group.CustomData = []gokeepasslib.CustomData{{Key: internalGroupField, Value: ungroupedGroupMarker}}
	db.Content.Root.Groups = append(db.Content.Root.Groups, group)
	return &db.Content.Root.Groups[len(db.Content.Root.Groups)-1]
}

func ensureRecycleBin(db *gokeepasslib.Database) *gokeepasslib.Group {
	meta := db.Content.Meta
	if group := recycleBinGroup(db); group != nil {
		meta.RecycleBinEnabled = wrappers.NewBoolWrapper(true)
		return group
	}

	group := gokeepasslib.NewGroup()
	group.Name = "回收站"
	group.IconID = 43
	db.Content.Root.Groups = append(db.Content.Root.Groups, group)
	recycleBin := &db.Content.Root.Groups[len(db.Content.Root.Groups)-1]
	now := wrappers.Now()
	meta.RecycleBinEnabled = wrappers.NewBoolWrapper(true)
	meta.RecycleBinUUID = recycleBin.UUID
	meta.RecycleBinChanged = &now
	return recycleBin
}

func isRecycleBinGroup(db *gokeepasslib.Database, group *gokeepasslib.Group) bool {
	return group != nil && !db.Content.Meta.RecycleBinUUID.IsZero() && group.UUID.Compare(db.Content.Meta.RecycleBinUUID)
}
