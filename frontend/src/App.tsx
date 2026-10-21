import { DragEvent, FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import {
    Check,
    ChevronDown,
    ChevronRight,
    Copy,
    Edit3,
    Eye,
    EyeOff,
    Folder,
    FolderOpen,
    FolderPlus,
    Globe2,
    History as HistoryIcon,
    Info,
    LayoutGrid,
    Languages,
    KeyRound,
    LockKeyhole,
    Menu,
    MoreHorizontal,
    Plus,
    RotateCcw,
    Search,
    Settings,
    ShieldCheck,
    Star,
    Trash2,
    X,
    Clock3,
    ContactRound,
    CreditCard,
    FileKey2,
    StickyNote,
    Wifi,
    Zap,
} from 'lucide-react';
import './App.css';
import Launcher, { VaultSnapshot } from './Launcher';
import { ChangePassword, CreateGroup, DeleteEntryTemplate, DeleteGroup, DeleteRecycleBinEntry, EmptyRecycleBin, GetEntryHistory, LockDatabase, MoveEntryToRecycleBin, MoveGroup, ReorderEntryTemplates, RestoreRecycleBinEntry, SaveEntry, SaveEntryTemplate, SetEntryTemplateEnabled, TouchEntry, UpdateGroup } from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import { BrowserOpenURL } from '../wailsjs/runtime/runtime';
import PasswordGenerator, { calculatePasswordStrength, generateDefaultPassword, passwordStrengthMessageKey } from './PasswordGenerator';
import UpdateSettings from './UpdateSettings';
import { DEFAULT_GROUP_ICON_ID, KeePassGroupIcon, keepassGroupIcons } from './KeePassGroupIcon';
import {localizedGroupIconLabel, useI18n, type MessageKey} from './i18n';
import TemplateManager from './TemplateManager';
import {builtInTemplates, categoryForTemplate, fieldValue, fieldsFromLegacyEntry, templateFor, templateIDForCategory, type EntryField, type EntryTemplate, type TemplateField} from './entryTemplates';

type VaultItem = {
    id: string;
    name: string;
    username: string;
    password: string;
    url: string;
    note: string;
    groupId: string;
    folder: string;
    category: string;
    favorite: boolean;
    modified: string;
    createdAt?: string;
    modifiedAt?: string;
    lastUsedAt?: string;
    color: string;
    initials: string;
    tags: string[];
    trashed?: boolean;
    deletedAt?: string;
    previousFolder?: string;
    historyCount?: number;
    templateId: string;
    templateVersion: number;
    fields: EntryField[];
};

type VaultGroup = {
    id: string;
    name: string;
    parentId?: string;
    depth: number;
    iconId: number;
};

type SaveResult = {
    entry: VaultItem;
};

const seedGroups: VaultGroup[] = [
    { id: 'demo-personal', name: '个人', depth: 0, iconId: 60 },
    { id: 'demo-work', name: '工作', depth: 0, iconId: 37 },
    { id: 'demo-shared', name: '共享', depth: 0, iconId: 5 },
];

const demoLastUsedAt = (minutesAgo: number) => new Date(Date.now() - minutesAgo * 60 * 1000).toISOString();
const demoTime = (daysAgo: number) => new Date(Date.now() - daysAgo * 24 * 60 * 60 * 1000).toISOString();

const seedItems: VaultItem[] = [
    { id: 'demo-1', name: 'GitHub', username: 'yearnfar@example.com', password: 'V7!q9z#L2mR8', url: 'https://github.com', note: '个人开发账号，已开启双重验证。', groupId: 'demo-work', folder: '工作', category: '登录信息', favorite: true, modified: '刚刚', createdAt: demoTime(420), modifiedAt: demoTime(0), lastUsedAt: demoLastUsedAt(2), color: '#171717', initials: 'GH', tags: ['开发', '2FA'], templateId: 'login', templateVersion: 1, fields: [] },
    { id: 'demo-2', name: 'Notion', username: 'studio@example.com', password: 'B5$p1w!K9xQ3', url: 'https://notion.so', note: '团队知识库与项目文档。', groupId: 'demo-work', folder: '工作', category: '登录信息', favorite: true, modified: '昨天', createdAt: demoTime(310), modifiedAt: demoTime(1), lastUsedAt: demoLastUsedAt(18), color: '#f1eee7', initials: 'N', tags: ['协作'], templateId: 'login', templateVersion: 1, fields: [] },
    { id: 'demo-3', name: 'Apple ID', username: 'yearnfar@icloud.com', password: 'Q2^m8p@D4sY1', url: 'https://appleid.apple.com', note: '主设备账号。恢复密钥保存在安全笔记中。', groupId: 'demo-personal', folder: '个人', category: '登录信息', favorite: true, modified: '3 天前', createdAt: demoTime(760), modifiedAt: demoTime(3), lastUsedAt: demoLastUsedAt(95), color: '#454545', initials: 'A', tags: ['重要'], templateId: 'login', templateVersion: 1, fields: [] },
    { id: 'demo-4', name: '招商银行', username: '张三', password: '482615', url: '', note: '工资卡。', groupId: 'demo-personal', folder: '个人', category: '银行卡', favorite: false, modified: '8 月 21 日', createdAt: demoTime(520), modifiedAt: demoTime(59), lastUsedAt: demoLastUsedAt(380), color: '#ba1e2e', initials: '招', tags: ['财务'], templateId: 'bank-card', templateVersion: 1, fields: [{key: 'UserName', value: '张三', protected: false}, {key: 'Card Number', value: '6217000012345678', protected: true}, {key: 'Expiry Date', value: '2029-08', protected: false}, {key: 'CVV', value: '318', protected: true}, {key: 'Password', value: '482615', protected: true}, {key: 'Notes', value: '工资卡。', protected: false}] },
    { id: 'demo-5', name: '家庭 Wi-Fi', username: 'KeyNest-Home', password: 'Home#2026!Net', url: 'http://192.168.1.1', note: '', groupId: 'demo-personal', folder: '个人', category: 'Wi-Fi', favorite: false, modified: '7 月 19 日', createdAt: demoTime(900), modifiedAt: demoTime(92), lastUsedAt: demoLastUsedAt(1440), color: '#39766e', initials: 'Wi', tags: ['家庭'], templateId: 'wifi', templateVersion: 1, fields: [{key: 'UserName', value: 'KeyNest-Home', protected: false}, {key: 'Password', value: 'Home#2026!Net', protected: true}, {key: 'Security Type', value: 'WPA3', protected: false}, {key: 'URL', value: 'http://192.168.1.1', protected: false}] },
    { id: 'demo-6', name: 'Figma', username: 'design@example.com', password: 'F1gma@Work#88', url: 'https://figma.com', note: '产品设计团队席位。', groupId: 'demo-work', folder: '工作', category: '登录信息', favorite: false, modified: '6 月 08 日', createdAt: demoTime(680), modifiedAt: demoTime(133), lastUsedAt: demoLastUsedAt(4320), color: '#a54cf5', initials: 'F', tags: ['设计'], templateId: 'login', templateVersion: 1, fields: [] },
].map((item) => ({...item, fields: fieldsFromLegacyEntry(item)}));

const groupFilter = (id: string) => `group:${id}`;
const ROOT_GROUP_DROP_TARGET = '__keynest-root-group__';
const UNGROUPED_DROP_TARGET = '__keynest-ungrouped__';
const RECYCLE_RETENTION_DAYS = 30;
const DAY_IN_MS = 24 * 60 * 60 * 1000;
const AUTO_LOCK_STORAGE_KEY = 'keynest.autoLockMinutes';
const AUTO_LOCK_OPTIONS = [1, 5, 15, 30] as const;
type AutoLockMinutes = typeof AUTO_LOCK_OPTIONS[number];

const initialAutoLockMinutes = (): AutoLockMinutes => {
    const stored = Number(localStorage.getItem(AUTO_LOCK_STORAGE_KEY));
    return AUTO_LOCK_OPTIONS.includes(stored as AutoLockMinutes) ? stored as AutoLockMinutes : 5;
};

const recycleDaysRemaining = (deletedAt: string | undefined) => {
    if (!deletedAt) return null;
    const deletedTime = Date.parse(deletedAt);
    if (Number.isNaN(deletedTime)) return null;
    return Math.max(0, Math.ceil((deletedTime + RECYCLE_RETENTION_DAYS * DAY_IN_MS - Date.now()) / DAY_IN_MS));
};

const formatLastUsed = (value: string | undefined, fallback: string, locale: string, justNow: string) => {
    if (!value) return fallback;
    const timestamp = new Date(value);
    if (Number.isNaN(timestamp.getTime())) return fallback;
    const now = new Date();
    const age = now.getTime() - timestamp.getTime();
    if (age < 60 * 1000) return justNow;
    const pad = (part: number) => String(part).padStart(2, '0');
    if (age < 24 * 60 * 60 * 1000) return `${pad(timestamp.getHours())}:${pad(timestamp.getMinutes())}`;
    if (timestamp.getFullYear() === now.getFullYear()) return new Intl.DateTimeFormat(locale, {month: 'short', day: '2-digit'}).format(timestamp);
    return new Intl.DateTimeFormat(locale, {year: 'numeric', month: 'short', day: '2-digit'}).format(timestamp);
};

const formatDetailTime = (value: string | undefined, locale: string, unknown: string) => {
    if (!value) return unknown;
    const timestamp = new Date(value);
    if (Number.isNaN(timestamp.getTime())) return unknown;
    return new Intl.DateTimeFormat(locale, {dateStyle: 'long', timeStyle: 'medium'}).format(timestamp);
};

const descendantGroupIds = (groups: VaultGroup[], rootID: string) => {
    const ids = new Set([rootID]);
    let changed = true;
    while (changed) {
        changed = false;
        for (const group of groups) {
            if (group.parentId && ids.has(group.parentId) && !ids.has(group.id)) {
                ids.add(group.id);
                changed = true;
            }
        }
    }
    return ids;
};

const insertGroup = (groups: VaultGroup[], created: VaultGroup) => {
    if (!created.parentId) return [...groups, created];
    const parentIndex = groups.findIndex((group) => group.id === created.parentId);
    if (parentIndex < 0) return [...groups, created];
    let insertAt = parentIndex + 1;
    while (insertAt < groups.length && groups[insertAt].depth > groups[parentIndex].depth) insertAt++;
    return [...groups.slice(0, insertAt), created, ...groups.slice(insertAt)];
};

const arrangeGroups = (groups: VaultGroup[]) => {
    const groupsByID = new Map(groups.map((group) => [group.id, group]));
    const children = new Map<string, VaultGroup[]>();
    for (const group of groups) {
        const parentID = group.parentId && groupsByID.has(group.parentId) ? group.parentId : '';
        children.set(parentID, [...(children.get(parentID) ?? []), group]);
    }
    const arranged: VaultGroup[] = [];
    const visited = new Set<string>();
    const append = (group: VaultGroup, depth: number) => {
        if (visited.has(group.id)) return;
        visited.add(group.id);
        arranged.push({ ...group, depth });
        for (const child of children.get(group.id) ?? []) append(child, depth + 1);
    };
    for (const group of children.get('') ?? []) append(group, 0);
    for (const group of groups) append(group, 0);
    return arranged;
};

const normalizeVaultItem = (entry: Omit<VaultItem, 'templateId' | 'templateVersion' | 'fields'> & Partial<Pick<VaultItem, 'templateId' | 'templateVersion' | 'fields'>>): VaultItem => ({
    ...entry,
    templateId: entry.templateId || templateIDForCategory(entry.category),
    templateVersion: entry.templateVersion || 1,
    fields: fieldsFromLegacyEntry(entry),
});

const navItems: Array<{labelKey: MessageKey; icon: typeof Clock3; filter: string}> = [
    { labelKey: 'nav.recent', icon: Clock3, filter: 'recent' },
    { labelKey: 'nav.all', icon: LayoutGrid, filter: 'all' },
    { labelKey: 'nav.favorites', icon: Star, filter: 'favorite' },
    { labelKey: 'nav.ungrouped', icon: FolderOpen, filter: 'ungrouped' },
];

const templateNameKeys: Record<string, MessageKey> = {
    login: 'template.login',
    'bank-card': 'template.bankCard',
    wifi: 'template.wifi',
    'secure-note': 'template.secureNote',
    identity: 'template.identity',
};

const templateIcons: Record<string, typeof FileKey2> = {
    login: KeyRound,
    'bank-card': CreditCard,
    wifi: Wifi,
    'secure-note': StickyNote,
    identity: ContactRound,
};

const fieldLabelKeys: Record<string, MessageKey> = {
    username: 'entry.username', password: 'entry.password', url: 'entry.url', notes: 'entry.notes',
    cardholder: 'template.cardholder', cardNumber: 'template.cardNumber', expiryDate: 'template.expiryDate',
    cvv: 'template.cvv', pin: 'template.pin', customerService: 'template.customerService',
    ssid: 'template.ssid', securityType: 'template.securityType', routerAddress: 'template.routerAddress',
    fullName: 'template.fullName', documentNumber: 'template.documentNumber', birthDate: 'template.birthDate',
    issuedDate: 'template.issuedDate',
};

const sectionLabelKeys: Record<string, MessageKey> = {
    login: 'entry.login', website: 'entry.website', notes: 'entry.notes', card: 'template.cardDetails',
    security: 'settings.security', contact: 'template.contact', network: 'template.network',
    identity: 'template.identityDetails', validity: 'template.validity', details: 'template.details',
};

function App() {
    const {locale, setLocale, t} = useI18n();
    const templateName = (template: EntryTemplate) => template.builtIn && templateNameKeys[template.id] ? t(templateNameKeys[template.id]) : template.name;
    const templateFieldLabel = (template: EntryTemplate, field: TemplateField) => template.builtIn && fieldLabelKeys[field.id] ? t(fieldLabelKeys[field.id]) : field.label;
    const templateSectionLabel = (section: string) => sectionLabelKeys[section] ? t(sectionLabelKeys[section]) : section;
    const [items, setItems] = useState(seedItems);
    const [groups, setGroups] = useState(seedGroups);
    const [templates, setTemplates] = useState<EntryTemplate[]>(builtInTemplates);
    const [selectedId, setSelectedId] = useState('demo-1');
    const [vaultOpen, setVaultOpen] = useState(false);
    const [vaultName, setVaultName] = useState('个人保险库');
    const [vaultPath, setVaultPath] = useState('');
    const [demoMode, setDemoMode] = useState(true);
    const [filter, setFilter] = useState('all');
    const [groupIconID, setGroupIconID] = useState(DEFAULT_GROUP_ICON_ID);
    const [collapsedGroupIDs, setCollapsedGroupIDs] = useState<Set<string>>(() => new Set());
    const [query, setQuery] = useState('');
    const [showPassword, setShowPassword] = useState(false);
    const [editing, setEditing] = useState(false);
    const [editingBackup, setEditingBackup] = useState<VaultItem | null>(null);
    const [draftEntryID, setDraftEntryID] = useState<string | null>(null);
    const [draftPassword, setDraftPassword] = useState('');
    const [generator, setGenerator] = useState<'detail' | null>(null);
    const [groupDialogOpen, setGroupDialogOpen] = useState(false);
    const [creatingGroup, setCreatingGroup] = useState(false);
    const [editingGroup, setEditingGroup] = useState<VaultGroup | null>(null);
    const [deleteGroupTarget, setDeleteGroupTarget] = useState<VaultGroup | null>(null);
    const [groupMenuID, setGroupMenuID] = useState<string | null>(null);
    const [groupActionPending, setGroupActionPending] = useState(false);
    const [clearTrashOpen, setClearTrashOpen] = useState(false);
    const [clearingTrash, setClearingTrash] = useState(false);
    const [deleteTarget, setDeleteTarget] = useState<VaultItem | null>(null);
    const [demoHistories, setDemoHistories] = useState<Record<string, VaultItem[]>>({});
    const [historyOpen, setHistoryOpen] = useState(false);
    const [historyVersions, setHistoryVersions] = useState<VaultItem[]>([]);
    const [historyLoading, setHistoryLoading] = useState(false);
    const [historyError, setHistoryError] = useState('');
    const [historyIndex, setHistoryIndex] = useState(0);
    const [showHistoryPassword, setShowHistoryPassword] = useState(false);
    const [entryActionPending, setEntryActionPending] = useState(false);
    const [draggedEntryID, setDraggedEntryID] = useState<string | null>(null);
    const [dropTargetGroupID, setDropTargetGroupID] = useState<string | null>(null);
    const [draggedGroupID, setDraggedGroupID] = useState<string | null>(null);
    const [groupDropTargetID, setGroupDropTargetID] = useState<string | null>(null);
    const [moreMenuOpen, setMoreMenuOpen] = useState(false);
    const [settingsOpen, setSettingsOpen] = useState(false);
    const [settingsQuery, setSettingsQuery] = useState('');
    const [settingsPage, setSettingsPage] = useState<'general' | 'security' | 'templates' | 'updates'>('general');
    const [autoLockMinutes, setAutoLockMinutes] = useState<AutoLockMinutes>(initialAutoLockMinutes);
    const [changePasswordOpen, setChangePasswordOpen] = useState(false);
    const [changingPassword, setChangingPassword] = useState(false);
    const [visibleMasterPasswordField, setVisibleMasterPasswordField] = useState<'current' | 'new' | 'confirm' | null>(null);
    const [toast, setToast] = useState('');
    const [mobileNavOpen, setMobileNavOpen] = useState(false);
    const [templatePickerOpen, setTemplatePickerOpen] = useState(false);
    const [deleteTemplateTarget, setDeleteTemplateTarget] = useState<EntryTemplate | null>(null);
    const [deletingTemplate, setDeletingTemplate] = useState(false);
    const [revealedFields, setRevealedFields] = useState<Set<string>>(() => new Set());
    const moreMenuRef = useRef<HTMLDivElement>(null);
    const groupMenuRef = useRef<HTMLDivElement>(null);
    const templatePickerRef = useRef<HTMLDivElement>(null);
    const historyRequestRef = useRef(0);
    const autoLockTimerRef = useRef<number | null>(null);
    const lastActivityAtRef = useRef(Date.now());
    const autoLockRunningRef = useRef(false);
    const lockVaultRef = useRef<(() => Promise<void>) | null>(null);

    const activeGroupIDs = useMemo(() => {
        if (!filter.startsWith('group:')) return null;
        return new Set([filter.slice('group:'.length)]);
    }, [filter]);

    const visibleItems = useMemo(() => {
        const normalized = query.trim().toLowerCase();
        let filtered = items.filter((item) => {
            const matchesLocation = filter === 'trash' ? item.trashed : !item.trashed;
            const matchesFilter = filter === 'trash'
                || filter === 'all'
                || filter === 'recent'
                || (filter === 'favorite' && item.favorite)
                || (filter === 'ungrouped' && !item.groupId)
                || Boolean(activeGroupIDs?.has(item.groupId))
                || item.category === filter;
            return matchesLocation && matchesFilter;
        });
        if (filter === 'trash') {
            filtered.sort((left, right) => Date.parse(right.deletedAt ?? '') - Date.parse(left.deletedAt ?? ''));
        } else if (filter === 'recent') {
            filtered.sort((left, right) => (Date.parse(right.lastUsedAt ?? '') || 0) - (Date.parse(left.lastUsedAt ?? '') || 0));
        }
        if (!normalized) return filtered;
        return filtered.filter((item) => {
            const template = templateFor(templates, item.templateId);
            const searchableKeys = new Set(template.fields.filter((field) => field.searchable && !field.protected).map((field) => field.storageKey));
            const searchableValues = item.fields.filter((field) => searchableKeys.has(field.key) && !field.protected).map((field) => field.value);
            return [item.name, ...searchableValues, ...item.tags].join(' ').toLowerCase().includes(normalized);
        });
    }, [activeGroupIDs, filter, items, query, templates]);
    const availableTemplates = useMemo(() => templates.filter((template) => !template.deleted), [templates]);
    const managedTemplates = useMemo(() => availableTemplates.filter((template) => template.id !== 'login'), [availableTemplates]);
    const enabledManagedTemplates = useMemo(() => managedTemplates.filter((template) => template.enabled), [managedTemplates]);
    const defaultEntryTemplate = templateFor(templates, 'login');
    const selected = visibleItems.find((item) => item.id === selectedId) ?? visibleItems[0];
    const selectedTemplate = templateFor(templates, selected?.templateId);
    const selectedTemplateSections = useMemo(() => {
        const sections = new Map<string, TemplateField[]>();
        for (const field of selectedTemplate.fields) sections.set(field.section, [...(sections.get(field.section) ?? []), field]);
        const knownKeys = new Set(selectedTemplate.fields.map((field) => field.storageKey));
        const extraFields = selected?.fields.filter((field) => !knownKeys.has(field.key)) ?? [];
        return {sections: [...sections.entries()], extraFields};
    }, [selected?.fields, selectedTemplate]);
    const historyVersion = historyVersions[historyIndex] ?? historyVersions[0];
    const historyTemplate = templateFor(templates, historyVersion?.templateId);
    const historyDisplayFields = useMemo(() => {
        if (!historyVersion) return [];
        const values = new Map(historyVersion.fields.map((field) => [field.key, field]));
        const ordered = historyTemplate.fields.map((field) => {
            const existing = values.get(field.storageKey);
            return {key: field.storageKey, value: existing?.value ?? '', protected: existing?.protected ?? field.protected, label: templateFieldLabel(historyTemplate, field)};
        });
        const known = new Set(historyTemplate.fields.map((field) => field.storageKey));
        return [...ordered, ...historyVersion.fields.filter((field) => !known.has(field.key)).map((field) => ({...field, label: field.key}))];
    }, [historyTemplate, historyVersion, locale]);
    const isEditingSelected = editing && !selected?.trashed;
    const isDraftSelected = Boolean(selected && selected.id === draftEntryID);
    const visibleItemCount = visibleItems.filter((item) => item.id !== draftEntryID).length;
    const detailPasswordStrength = calculatePasswordStrength(isEditingSelected ? draftPassword : selected?.password ?? '');
    const favoriteCount = items.filter((item) => item.favorite && !item.trashed).length;
    const ungroupedCount = items.filter((item) => item.id !== draftEntryID && !item.trashed && !item.groupId).length;
    const trashCount = items.filter((item) => item.trashed).length;
    const draggedEntry = draggedEntryID ? items.find((item) => item.id === draggedEntryID) : undefined;
    const draggedGroup = draggedGroupID ? groups.find((group) => group.id === draggedGroupID) : undefined;
    const draggedGroupDescendantIDs = useMemo(
        () => draggedGroup ? descendantGroupIds(groups, draggedGroup.id) : new Set<string>(),
        [draggedGroup, groups],
    );

    const groupCounts = useMemo(() => {
        const counts = new Map<string, number>();
        for (const group of groups) {
            counts.set(group.id, items.filter((item) => item.id !== draftEntryID && !item.trashed && item.groupId === group.id).length);
        }
        return counts;
    }, [draftEntryID, groups, items]);

    const groupTree = useMemo(() => {
        const groupsByID = new Map(groups.map((group) => [group.id, group]));
        const parentIDs = new Set(groups.flatMap((group) => group.parentId ? [group.parentId] : []));
        const visible = groups.filter((group) => {
            const visited = new Set<string>();
            let parentID = group.parentId;
            while (parentID && !visited.has(parentID)) {
                if (collapsedGroupIDs.has(parentID)) return false;
                visited.add(parentID);
                parentID = groupsByID.get(parentID)?.parentId;
            }
            return true;
        });
        return { groupsByID, parentIDs, visible };
    }, [collapsedGroupIDs, groups]);

    const deleteGroupSummary = useMemo(() => {
        if (!deleteGroupTarget) return { childGroups: 0, entries: 0 };
        const groupIDs = descendantGroupIds(groups, deleteGroupTarget.id);
        return {
            childGroups: groupIDs.size - 1,
            entries: items.filter((item) => !item.trashed && groupIDs.has(item.groupId)).length,
        };
    }, [deleteGroupTarget, groups, items]);

    const selectedGroupPath = useMemo(() => {
        if (!selected || selected.trashed) return [];
        const path: VaultGroup[] = [];
        const visited = new Set<string>();
        let group = groupTree.groupsByID.get(selected.groupId);
        while (group && !visited.has(group.id)) {
            path.unshift(group);
            visited.add(group.id);
            group = group.parentId ? groupTree.groupsByID.get(group.parentId) : undefined;
        }
        return path;
    }, [groupTree.groupsByID, selected]);

    const cancelItemEditing = () => {
        if (draftEntryID) {
            setItems((current) => current.filter((item) => item.id !== draftEntryID));
            setSelectedId((current) => current === draftEntryID ? '' : current);
            setDraftEntryID(null);
        } else if (editingBackup) {
            setItems((current) => current.map((item) => item.id === editingBackup.id ? editingBackup : item));
            setDraftPassword(editingBackup.password);
        }
        setEditingBackup(null);
        setEditing(false);
        setGenerator(null);
        setShowPassword(false);
    };

    const beginItemEditing = () => {
        if (!selected || selected.trashed) return;
        if (!editing) {
            setEditingBackup({...selected, fields: selected.fields.map((field) => ({...field}))});
            setDraftPassword(selected.password);
        }
        setEditing(true);
        setGenerator(null);
        setMoreMenuOpen(false);
    };

    const selectGroup = (groupID: string) => {
        cancelItemEditing();
        setCollapsedGroupIDs((current) => {
            const next = new Set(current);
            const visited = new Set<string>();
            let parentID = groupTree.groupsByID.get(groupID)?.parentId;
            while (parentID && !visited.has(parentID)) {
                next.delete(parentID);
                visited.add(parentID);
                parentID = groupTree.groupsByID.get(parentID)?.parentId;
            }
            return next;
        });
        setSettingsOpen(false);
        setFilter(groupFilter(groupID));
        setMobileNavOpen(false);
    };

    const sectionTitle = filter === 'all'
        ? t('nav.all')
        : filter === 'recent'
            ? t('nav.recent')
            : filter === 'ungrouped'
                ? t('nav.ungrouped')
            : filter === 'trash'
                ? t('nav.trash')
                : filter.startsWith('group:')
                    ? groups.find((group) => group.id === filter.slice('group:'.length))?.name ?? t('nav.group')
                    : filter;
    const normalizedSettingsQuery = settingsQuery.trim().toLowerCase();
    const generalSettingVisible = !normalizedSettingsQuery || '通用 语言 language general 中文 english'.includes(normalizedSettingsQuery);
    const securitySettingVisible = !normalizedSettingsQuery || '安全 主密码 修改密码 自动锁定 空闲 分钟 security master password auto lock idle minute'.includes(normalizedSettingsQuery);
    const templateSettingVisible = !normalizedSettingsQuery || '模板 字段 登录 银行卡 wifi 笔记 身份 template field login card note identity'.includes(normalizedSettingsQuery);
    const updateSettingVisible = !normalizedSettingsQuery || '应用 关于 版本 更新 自动更新 application about version update github releases'.includes(normalizedSettingsQuery);

    useEffect(() => {
        if (!moreMenuOpen) return;
        const closeOnOutsideClick = (event: MouseEvent) => {
            if (!moreMenuRef.current?.contains(event.target as Node)) setMoreMenuOpen(false);
        };
        const closeOnEscape = (event: KeyboardEvent) => {
            if (event.key === 'Escape') setMoreMenuOpen(false);
        };
        document.addEventListener('mousedown', closeOnOutsideClick);
        document.addEventListener('keydown', closeOnEscape);
        return () => {
            document.removeEventListener('mousedown', closeOnOutsideClick);
            document.removeEventListener('keydown', closeOnEscape);
        };
    }, [moreMenuOpen]);

    useEffect(() => setMoreMenuOpen(false), [filter, selected?.id]);

    useEffect(() => {
        if (!templatePickerOpen) return;
        const closeOnOutsideClick = (event: MouseEvent) => {
            if (!templatePickerRef.current?.contains(event.target as Node)) setTemplatePickerOpen(false);
        };
        const closeOnEscape = (event: KeyboardEvent) => {
            if (event.key === 'Escape') setTemplatePickerOpen(false);
        };
        document.addEventListener('mousedown', closeOnOutsideClick);
        document.addEventListener('keydown', closeOnEscape);
        return () => {
            document.removeEventListener('mousedown', closeOnOutsideClick);
            document.removeEventListener('keydown', closeOnEscape);
        };
    }, [templatePickerOpen]);

    useEffect(() => {
        historyRequestRef.current += 1;
        setHistoryOpen(false);
        setHistoryVersions([]);
        setHistoryLoading(false);
        setHistoryError('');
        setHistoryIndex(0);
        setShowHistoryPassword(false);
        setRevealedFields(new Set());
    }, [selected?.id]);

    useEffect(() => {
        if (!groupMenuID) return;
        const closeOnOutsideClick = (event: MouseEvent) => {
            if (!groupMenuRef.current?.contains(event.target as Node)) setGroupMenuID(null);
        };
        const closeOnEscape = (event: KeyboardEvent) => {
            if (event.key === 'Escape') setGroupMenuID(null);
        };
        document.addEventListener('mousedown', closeOnOutsideClick);
        document.addEventListener('keydown', closeOnEscape);
        return () => {
            document.removeEventListener('mousedown', closeOnOutsideClick);
            document.removeEventListener('keydown', closeOnEscape);
        };
    }, [groupMenuID]);

    useEffect(() => setGroupMenuID(null), [filter]);

    const notify = (message: string) => {
        setToast(message);
        window.setTimeout(() => setToast(''), 1800);
    };

    const markEntryUsed = async (item: VaultItem) => {
        if (item.trashed || item.id === draftEntryID) return;
        try {
            const touched = demoMode || !(window as Window & { go?: unknown }).go
                ? { ...item, lastUsedAt: new Date().toISOString() }
                : await TouchEntry(item.id) as VaultItem;
            setItems((current) => current.map((candidate) => candidate.id === item.id ? touched : candidate));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        }
    };

    const copyValue = async (label: string, value: string, item?: VaultItem) => {
        try {
            await navigator.clipboard.writeText(value);
            if (item) await markEntryUsed(item);
            notify(t('entry.copySuccess', {label}));
        } catch {
            notify(t('entry.copyFailed'));
        }
    };

    const selectItem = (item: VaultItem) => {
        if (draftEntryID && item.id !== draftEntryID) {
            setItems((current) => current.filter((candidate) => candidate.id !== draftEntryID));
            setDraftEntryID(null);
        }
        setSelectedId(item.id);
        setDraftPassword(item.password);
        setShowPassword(item.id === draftEntryID);
        setEditing(item.id === draftEntryID);
        setEditingBackup(null);
        setGenerator(null);
    };

    const openEntryWebsite = (item: VaultItem) => {
        if (!item.url || item.trashed) return;
        if ((window as Window & { runtime?: unknown }).runtime) BrowserOpenURL(item.url);
        else window.open(item.url, '_blank', 'noopener,noreferrer');
        void markEntryUsed(item);
    };

    const openSnapshot = (snapshot: VaultSnapshot, demo = false) => {
        const next = snapshot.entries.map((entry) => normalizeVaultItem(entry as VaultItem));
        const nextGroups = (snapshot.groups ?? []) as VaultGroup[];
        setItems(next);
        setGroups(nextGroups);
        setTemplates(snapshot.templates?.length ? snapshot.templates : builtInTemplates);
        setSelectedId(next[0]?.id ?? '');
        setDraftPassword(next[0]?.password ?? '');
        setVaultName(snapshot.name);
        setVaultPath(snapshot.path);
        setDemoMode(demo);
        setDemoHistories({});
        setDraftEntryID(null);
        setVaultOpen(true);
        setSettingsOpen(false);
        setSettingsQuery('');
        setDeleteTemplateTarget(null);
        setFilter('all');
        setQuery('');
        setCollapsedGroupIDs(new Set());
    };

    const openDemo = () => openSnapshot({ name: `KeyNest ${t('launcher.demo')}`, path: 'demo.kdbx', entries: seedItems, groups: seedGroups, templates: builtInTemplates }, true);

    const persistEntry = async (item: VaultItem): Promise<SaveResult> => {
        if (demoMode || !(window as Window & { go?: unknown }).go) {
            if (item.id.startsWith('new-')) {
                return { entry: { ...item, id: item.id.replace(/^new-/, 'demo-'), historyCount: 0 } };
            }
            const current = items.find((candidate) => candidate.id === item.id && !candidate.trashed);
            if (!current) return { entry: { ...item, historyCount: 0 } };
            const version: VaultItem = { ...current, historyCount: 0 };
            const history = [version, ...(demoHistories[current.id] ?? [])].slice(0, 10);
            setDemoHistories((histories) => ({ ...histories, [current.id]: history }));
            return { entry: { ...item, historyCount: history.length } };
        }
        const saved = await SaveEntry(main.SaveEntryRequest.createFrom({
            id: item.id.startsWith('new-') ? '' : item.id,
            name: item.name,
            username: item.username,
            password: item.password,
            url: item.url,
            note: item.note,
            groupId: item.groupId,
            folder: item.folder,
            category: item.category,
            favorite: item.favorite,
            tags: item.tags,
            templateId: item.templateId,
            templateVersion: item.templateVersion,
            fields: item.fields,
        }));
        return saved as SaveResult;
    };

    const closeHistory = () => {
        historyRequestRef.current += 1;
        setHistoryOpen(false);
        setHistoryLoading(false);
        setShowHistoryPassword(false);
    };

    const openHistory = async () => {
        if (!selected || selected.trashed || (selected.historyCount ?? 0) === 0) return;
        const entryID = selected.id;
        const requestID = historyRequestRef.current + 1;
        historyRequestRef.current = requestID;
        setHistoryOpen(true);
        setHistoryVersions([]);
        setHistoryIndex(0);
        setShowHistoryPassword(false);
        setHistoryError('');
        setHistoryLoading(true);
        try {
            const history = demoMode || !(window as Window & { go?: unknown }).go
                ? demoHistories[entryID] ?? []
                : await GetEntryHistory(entryID) as VaultItem[];
            if (historyRequestRef.current !== requestID) return;
            setHistoryVersions(history.map((entry) => normalizeVaultItem(entry)));
        } catch {
            if (historyRequestRef.current !== requestID) return;
            setHistoryError(t('entry.historyLoadFailed'));
        } finally {
            if (historyRequestRef.current === requestID) setHistoryLoading(false);
        }
    };

    const moveEntryToGroup = async (entryID: string, groupID: string) => {
        const item = items.find((candidate) => candidate.id === entryID);
        const targetGroup = groupID ? groups.find((group) => group.id === groupID) : undefined;
        if (!item || item.trashed || (groupID && !targetGroup) || item.groupId === groupID || entryActionPending) return;
        setEntryActionPending(true);
        try {
            const { entry } = await persistEntry({
                ...item,
                groupId: targetGroup?.id ?? '',
                folder: targetGroup?.name ?? '',
                modified: '刚刚',
                modifiedAt: new Date().toISOString(),
            });
            setItems((current) => current.map((candidate) => candidate.id === item.id ? entry : candidate));
            setSelectedId(entry.id);
            setDraftPassword(entry.password);
            setEditing(false);
            setGenerator(null);
            notify(targetGroup ? t('entry.movedTo', {name: targetGroup.name}) : t('entry.movedUngrouped'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setEntryActionPending(false);
        }
    };

    const startEntryDrag = (event: DragEvent<HTMLButtonElement>, item: VaultItem) => {
        if (item.trashed || editing || entryActionPending) {
            event.preventDefault();
            return;
        }
        event.dataTransfer.effectAllowed = 'move';
        event.dataTransfer.setData('application/x-keynest-entry-id', item.id);
        event.dataTransfer.setData('text/plain', item.id);
        setDraggedEntryID(item.id);
        setDropTargetGroupID(null);
        setDraggedGroupID(null);
        setGroupDropTargetID(null);
    };

    const finishEntryDrag = () => {
        setDraggedEntryID(null);
        setDropTargetGroupID(null);
    };

    const moveGroupToParent = async (groupID: string, parentID: string) => {
        const group = groups.find((candidate) => candidate.id === groupID);
        const parent = parentID ? groups.find((candidate) => candidate.id === parentID) : undefined;
        if (!group || (parentID && (!parent || descendantGroupIds(groups, group.id).has(parentID))) || groupActionPending) return;
        setGroupActionPending(true);
        try {
            if (demoMode || !(window as Window & { go?: unknown }).go) {
                setGroups((current) => {
                    const moved = current.find((candidate) => candidate.id === group.id);
                    if (!moved) return current;
                    return arrangeGroups([
                        { ...moved, parentId: parentID || undefined },
                        ...current.filter((candidate) => candidate.id !== group.id),
                    ]);
                });
            } else {
                const snapshot = await MoveGroup(group.id, parentID) as VaultSnapshot;
                setGroups((snapshot.groups ?? []) as VaultGroup[]);
                setItems(snapshot.entries.map((entry) => normalizeVaultItem(entry as VaultItem)));
            }
            if (parentID) {
                setCollapsedGroupIDs((current) => {
                    const next = new Set(current);
                    next.delete(parentID);
                    return next;
                });
            }
            notify(parent ? t('entry.movedChildFirst', {name: parent.name}) : t('entry.movedTop'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setGroupActionPending(false);
        }
    };

    const startGroupDrag = (event: DragEvent<HTMLButtonElement>, group: VaultGroup) => {
        if (groupActionPending) {
            event.preventDefault();
            return;
        }
        event.dataTransfer.effectAllowed = 'move';
        event.dataTransfer.setData('application/x-keynest-group-id', group.id);
        setDraggedGroupID(group.id);
        setGroupDropTargetID(null);
        setDraggedEntryID(null);
        setDropTargetGroupID(null);
        setGroupMenuID(null);
    };

    const finishGroupDrag = () => {
        setDraggedGroupID(null);
        setGroupDropTargetID(null);
    };

    const toggleFavorite = async () => {
        if (!selected) return;
        const updated = { ...selected, favorite: !selected.favorite, modified: '刚刚', modifiedAt: new Date().toISOString() };
        try {
            const { entry } = await persistEntry(updated);
            setItems((current) => current.map((item) => item.id === selected.id ? entry : item));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        }
    };

    const saveItem = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (!selected || selected.trashed) return;
        const data = new FormData(event.currentTarget);
        const name = String(data.get('name')).trim() || 'no title';
        const templateKeys = new Set(selectedTemplate.fields.map((field) => field.storageKey));
        const fields: EntryField[] = [
            ...selectedTemplate.fields.map((field) => ({
                key: field.storageKey,
                value: String(data.get(`field:${field.storageKey}`) ?? ''),
                protected: field.protected,
            })),
            ...selected.fields.filter((field) => !templateKeys.has(field.key)).map((field) => ({
                ...field,
                value: String(data.get(`extra:${field.key}`) ?? field.value),
            })),
        ];
        const updated: VaultItem = {
            ...selected,
            name,
            initials: name.slice(0, 2).toUpperCase(),
            username: fieldValue(fields, 'UserName'),
            password: fieldValue(fields, 'Password'),
            url: fieldValue(fields, 'URL'),
            note: fieldValue(fields, 'Notes'),
            fields,
            templateId: selectedTemplate.id,
            templateVersion: selectedTemplate.version,
            category: selectedTemplate.builtIn ? categoryForTemplate(selectedTemplate.id) : selectedTemplate.name,
            modified: '刚刚',
            modifiedAt: new Date().toISOString(),
        };
        const creating = selected.id === draftEntryID;
        const passwordChanged = selected.password !== updated.password;
        try {
            const result = await persistEntry(updated);
            setItems((current) => current.map((item) => item.id === selected.id ? result.entry : item));
            setSelectedId(result.entry.id);
            setDraftEntryID(null);
            setEditing(false);
            setEditingBackup(null);
            setDraftPassword(result.entry.password);
            notify(creating ? t('entry.created') : passwordChanged ? t('entry.passwordUpdated') : t('entry.saved'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        }
    };

    const createItem = (templateID = 'login') => {
        if (entryActionPending) return;
        const template = templateFor(templates, templateID);
        const nextId = `new-${Date.now()}`;
        const name = 'no title';
        const selectedGroupID = filter.startsWith('group:') ? filter.slice('group:'.length) : '';
        const selectedGroup = groups.find((group) => group.id === selectedGroupID);
        const now = new Date().toISOString();
        const password = template.id === 'login' || template.id === 'wifi' ? generateDefaultPassword() : '';
        const fields = template.fields.map((field) => ({
            key: field.storageKey,
            value: field.storageKey === 'Password' ? password : '',
            protected: field.protected,
        }));
        const next: VaultItem = {
            id: nextId,
            name,
            username: '',
            password,
            url: '',
            note: '',
            groupId: selectedGroup?.id ?? '',
            folder: selectedGroup?.name ?? '',
            category: template.builtIn ? categoryForTemplate(template.id) : template.name,
            favorite: false,
            modified: '刚刚',
            createdAt: now,
            modifiedAt: now,
            lastUsedAt: now,
            color: '#356d65',
            initials: name.slice(0, 2).toUpperCase(),
            tags: [],
            templateId: template.id,
            templateVersion: template.version,
            fields,
        };
        setItems((current) => [next, ...current.filter((item) => item.id !== draftEntryID)]);
        setDraftEntryID(next.id);
        setSelectedId(next.id);
        setFilter(selectedGroup && filter.startsWith('group:') ? groupFilter(selectedGroup.id) : filter === 'ungrouped' ? 'ungrouped' : 'all');
        setQuery('');
        setDraftPassword(next.password);
        setShowPassword(true);
        setEditing(true);
        setEditingBackup(null);
        setGenerator(null);
        setTemplatePickerOpen(false);
    };

    const changeSelectedTemplate = (templateID: string) => {
        if (!selected || selected.trashed) return;
        if (!isDraftSelected && !window.confirm(t('template.changeWarning'))) return;
        const template = templateFor(templates, templateID);
        const existingByKey = new Map(selected.fields.map((field) => [field.key, field]));
        const templateKeys = new Set(template.fields.map((field) => field.storageKey));
        const fields = [
            ...template.fields.map((field) => existingByKey.get(field.storageKey) ?? {key: field.storageKey, value: '', protected: field.protected}),
            ...selected.fields.filter((field) => !templateKeys.has(field.key)),
        ];
        const updated = normalizeVaultItem({
            ...selected,
            templateId: template.id,
            templateVersion: template.version,
            category: template.builtIn ? categoryForTemplate(template.id) : template.name,
            fields,
            username: fieldValue(fields, 'UserName'),
            password: fieldValue(fields, 'Password'),
            url: fieldValue(fields, 'URL'),
            note: fieldValue(fields, 'Notes'),
        });
        setItems((current) => current.map((item) => item.id === selected.id ? updated : item));
        setDraftPassword(updated.password);
        setRevealedFields(new Set());
    };

    const saveTemplate = async (template: EntryTemplate) => {
        try {
            if (demoMode || !(window as Window & { go?: unknown }).go) {
                const saved = {...template, id: template.id || `custom-demo-${Date.now()}`, version: template.id ? template.version + 1 : 1, builtIn: false, deleted: false};
                setTemplates((current) => current.some((candidate) => candidate.id === saved.id)
                    ? current.map((candidate) => candidate.id === saved.id ? saved : candidate)
                    : [...current, saved]);
            } else {
                const saved = await SaveEntryTemplate(main.VaultEntryTemplate.createFrom(template)) as EntryTemplate;
                setTemplates((current) => current.some((candidate) => candidate.id === saved.id)
                    ? current.map((candidate) => candidate.id === saved.id ? saved : candidate)
                    : [...current, saved]);
            }
            notify(t('template.saved'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
            throw reason;
        }
    };

    const deleteTemplate = async () => {
        if (!deleteTemplateTarget || deletingTemplate) return;
        setDeletingTemplate(true);
        try {
            if (demoMode || !(window as Window & { go?: unknown }).go) {
                setTemplates((current) => current.map((candidate) => candidate.id === deleteTemplateTarget.id ? {...candidate, deleted: true} : candidate));
            } else {
                setTemplates(await DeleteEntryTemplate(deleteTemplateTarget.id) as EntryTemplate[]);
            }
            setDeleteTemplateTarget(null);
            notify(t('template.deleted'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setDeletingTemplate(false);
        }
    };

    const reorderTemplates = async (ordered: EntryTemplate[]) => {
        const ids = ordered.map((template) => template.id);
        try {
            if (demoMode || !(window as Window & { go?: unknown }).go) {
                setTemplates((current) => {
                    const byID = new Map(current.map((template) => [template.id, template]));
                    const login = byID.get('login');
                    const reordered = ids.flatMap((id) => {
                        const template = byID.get(id);
                        return template && !template.deleted && template.id !== 'login' ? [template] : [];
                    });
                    const deleted = current.filter((template) => template.deleted);
                    return [...(login ? [login] : []), ...reordered, ...deleted];
                });
            } else {
                setTemplates(await ReorderEntryTemplates(ids) as EntryTemplate[]);
            }
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
            throw reason;
        }
    };

    const setTemplateEnabled = async (template: EntryTemplate, enabled: boolean) => {
        try {
            if (demoMode || !(window as Window & { go?: unknown }).go) {
                setTemplates((current) => current.map((candidate) => candidate.id === template.id ? {...candidate, enabled} : candidate));
            } else {
                setTemplates(await SetEntryTemplateEnabled(template.id, enabled) as EntryTemplate[]);
            }
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
            throw reason;
        }
    };

    const saveGroup = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const name = String(data.get('name')).trim();
        const parentId = editingGroup?.parentId ?? '';
        if (!name) return;
        setCreatingGroup(true);
        try {
            const parent = groups.find((group) => group.id === parentId);
            let selectedGroupID: string;
            if (editingGroup) {
                selectedGroupID = editingGroup.id;
                if (demoMode || !(window as Window & { go?: unknown }).go) {
                    setGroups((current) => current.map((group) => group.id === editingGroup.id
                        ? { ...group, name, iconId: groupIconID }
                        : group));
                    setItems((current) => current.map((item) => item.groupId === editingGroup.id ? { ...item, folder: name } : item));
                } else {
                    const snapshot = await UpdateGroup(editingGroup.id, name, parentId, groupIconID) as VaultSnapshot;
                    setGroups((snapshot.groups ?? []) as VaultGroup[]);
                    setItems(snapshot.entries.map((entry) => normalizeVaultItem(entry as VaultItem)));
                }
            } else {
                const created = demoMode || !(window as Window & { go?: unknown }).go
                    ? { id: `demo-group-${Date.now()}`, name, parentId: parentId || undefined, depth: parent ? parent.depth + 1 : 0, iconId: groupIconID }
                    : await CreateGroup(name, parentId, groupIconID) as VaultGroup;
                setGroups((current) => insertGroup(current, created));
                selectedGroupID = created.id;
            }
            if (parentId) {
                setCollapsedGroupIDs((current) => {
                    const next = new Set(current);
                    const visited = new Set<string>();
                    let ancestorID: string | undefined = parentId;
                    while (ancestorID && !visited.has(ancestorID)) {
                        next.delete(ancestorID);
                        visited.add(ancestorID);
                        ancestorID = groupTree.groupsByID.get(ancestorID)?.parentId;
                    }
                    return next;
                });
            }
            setFilter(groupFilter(selectedGroupID));
            setGroupDialogOpen(false);
            setEditingGroup(null);
            setGroupIconID(DEFAULT_GROUP_ICON_ID);
            notify(t(editingGroup ? 'entry.groupUpdated' : 'entry.groupCreated'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setCreatingGroup(false);
        }
    };

    const openCreateGroup = () => {
        cancelItemEditing();
        setEditingGroup(null);
        setGroupIconID(DEFAULT_GROUP_ICON_ID);
        setGroupMenuID(null);
        setGroupDialogOpen(true);
    };

    const openEditGroup = (group: VaultGroup) => {
        setEditingGroup(group);
        setGroupIconID(group.iconId);
        setGroupMenuID(null);
        setGroupDialogOpen(true);
    };

    const closeGroupDialog = () => {
        setGroupDialogOpen(false);
        setEditingGroup(null);
        setGroupIconID(DEFAULT_GROUP_ICON_ID);
    };

    const deleteSelectedGroup = async () => {
        if (!deleteGroupTarget) return;
        setGroupActionPending(true);
        const deletedIDs = descendantGroupIds(groups, deleteGroupTarget.id);
        try {
            let nextItems: VaultItem[];
            if (demoMode || !(window as Window & { go?: unknown }).go) {
                nextItems = items.map((item) => deletedIDs.has(item.groupId) && !item.trashed
                    ? { ...item, groupId: '', folder: '', modified: '刚刚', modifiedAt: new Date().toISOString() }
                    : item);
                setGroups((current) => current.filter((group) => !deletedIDs.has(group.id)));
            } else {
                const snapshot = await DeleteGroup(deleteGroupTarget.id) as VaultSnapshot;
                nextItems = snapshot.entries.map((entry) => normalizeVaultItem(entry as VaultItem));
                setGroups((snapshot.groups ?? []) as VaultGroup[]);
            }
            setItems(nextItems);
            setCollapsedGroupIDs((current) => new Set([...current].filter((id) => !deletedIDs.has(id))));
            if (filter.startsWith('group:') && deletedIDs.has(filter.slice('group:'.length))) setFilter('all');
            setDeleteGroupTarget(null);
            notify(t('entry.groupDeleted'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setGroupActionPending(false);
        }
    };

    const lockVault = async () => {
        cancelItemEditing();
        if (!demoMode && (window as Window & { go?: unknown }).go) await LockDatabase();
        setVaultOpen(false);
        setShowPassword(false);
        setEditing(false);
        setGenerator(null);
        setGroupDialogOpen(false);
        setEditingGroup(null);
        setDeleteGroupTarget(null);
        setGroupMenuID(null);
        setClearTrashOpen(false);
        setDeleteTarget(null);
        setDeleteTemplateTarget(null);
        setMoreMenuOpen(false);
        setSettingsOpen(false);
        setSettingsQuery('');
        setMobileNavOpen(false);
        setChangePasswordOpen(false);
        setVisibleMasterPasswordField(null);
        setCollapsedGroupIDs(new Set());
    };
    lockVaultRef.current = lockVault;

    useEffect(() => {
        localStorage.setItem(AUTO_LOCK_STORAGE_KEY, String(autoLockMinutes));
    }, [autoLockMinutes]);

    useEffect(() => {
        const clearTimer = () => {
            if (autoLockTimerRef.current === null) return;
            window.clearTimeout(autoLockTimerRef.current);
            autoLockTimerRef.current = null;
        };
        if (!vaultOpen) {
            clearTimer();
            autoLockRunningRef.current = false;
            return;
        }

        const timeoutMs = autoLockMinutes * 60 * 1000;
        lastActivityAtRef.current = Date.now();
        autoLockRunningRef.current = false;

        const lockIfIdle = () => {
            clearTimer();
            const remaining = timeoutMs - (Date.now() - lastActivityAtRef.current);
            if (remaining <= 0) {
                if (!autoLockRunningRef.current) {
                    autoLockRunningRef.current = true;
                    void lockVaultRef.current?.().catch(() => {
                        autoLockRunningRef.current = false;
                    });
                }
                return;
            }
            autoLockTimerRef.current = window.setTimeout(lockIfIdle, remaining);
        };
        const recordActivity = () => {
            if (autoLockRunningRef.current) return;
            lastActivityAtRef.current = Date.now();
            clearTimer();
            autoLockTimerRef.current = window.setTimeout(lockIfIdle, timeoutMs);
        };
        const checkAfterPause = () => {
            if (document.visibilityState === 'visible') lockIfIdle();
        };
        const activityEvents = ['pointerdown', 'keydown', 'wheel', 'touchstart'] as const;
        for (const eventName of activityEvents) window.addEventListener(eventName, recordActivity, {capture: true, passive: true});
        window.addEventListener('focus', checkAfterPause);
        document.addEventListener('visibilitychange', checkAfterPause);
        autoLockTimerRef.current = window.setTimeout(lockIfIdle, timeoutMs);

        return () => {
            clearTimer();
            for (const eventName of activityEvents) window.removeEventListener(eventName, recordActivity, {capture: true});
            window.removeEventListener('focus', checkAfterPause);
            document.removeEventListener('visibilitychange', checkAfterPause);
        };
    }, [autoLockMinutes, vaultOpen]);

    const closeChangePassword = () => {
        if (changingPassword) return;
        setChangePasswordOpen(false);
        setVisibleMasterPasswordField(null);
    };

    const changeMasterPassword = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const currentPassword = String(data.get('currentPassword'));
        const newMasterPassword = String(data.get('newPassword'));
        const confirmPassword = String(data.get('confirmPassword'));
        if (!newMasterPassword) {
            notify(t('dialog.enterNewPassword'));
            return;
        }
        if (newMasterPassword !== confirmPassword) {
            notify(t('dialog.newPasswordMismatch'));
            return;
        }
        setChangingPassword(true);
        try {
            if (!demoMode && (window as Window & { go?: unknown }).go) {
                await ChangePassword(currentPassword, newMasterPassword);
            }
            setChangePasswordOpen(false);
            setVisibleMasterPasswordField(null);
            notify(t(demoMode ? 'dialog.demoPasswordUnchanged' : 'dialog.passwordChanged'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setChangingPassword(false);
        }
    };

    const clearRecycleBin = async () => {
        setClearingTrash(true);
        try {
            if (!demoMode && (window as Window & { go?: unknown }).go) {
                await EmptyRecycleBin();
            }
            setItems((current) => current.filter((item) => !item.trashed));
            setSelectedId('');
            setShowPassword(false);
            setEditing(false);
            setClearTrashOpen(false);
            notify(t('entry.trashCleared'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setClearingTrash(false);
        }
    };

    const restoreSelected = async () => {
        if (!selected?.trashed) return;
        setEntryActionPending(true);
        try {
            const restoreGroup = groups.find((group) => group.id === selected.groupId);
            const restored = !demoMode && (window as Window & { go?: unknown }).go
                ? await RestoreRecycleBinEntry(selected.id) as VaultItem
                : { ...selected, groupId: restoreGroup?.id ?? '', folder: restoreGroup?.name ?? '', lastUsedAt: new Date().toISOString(), trashed: false, deletedAt: undefined };
            setItems((current) => current.map((item) => item.id === selected.id ? restored : item));
            setSelectedId('');
            setShowPassword(false);
            notify(t('entry.restored'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setEntryActionPending(false);
        }
    };

    const deleteRecycleBinItem = async () => {
        if (!deleteTarget) return;
        setEntryActionPending(true);
        try {
            if (!demoMode && (window as Window & { go?: unknown }).go) {
                await DeleteRecycleBinEntry(deleteTarget.id);
            }
            setItems((current) => current.filter((item) => item.id !== deleteTarget.id));
            setSelectedId('');
            setShowPassword(false);
            setDeleteTarget(null);
            notify(t('entry.deleted'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setEntryActionPending(false);
        }
    };

    const duplicateSelected = async () => {
        if (!selected || selected.trashed) return;
        setEntryActionPending(true);
        setMoreMenuOpen(false);
        try {
            const duplicate: VaultItem = {
                ...selected,
                id: `new-${Date.now()}`,
                modified: '刚刚',
                createdAt: new Date().toISOString(),
                modifiedAt: new Date().toISOString(),
                lastUsedAt: new Date().toISOString(),
                trashed: false,
                deletedAt: undefined,
                previousFolder: undefined,
                historyCount: 0,
            };
            const { entry } = await persistEntry(duplicate);
            setItems((current) => [entry, ...current]);
            setSelectedId(entry.id);
            setDraftPassword(entry.password);
            setShowPassword(false);
            setEditing(false);
            setGenerator(null);
            notify(t('entry.duplicated'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setEntryActionPending(false);
        }
    };

    const moveSelectedToRecycleBin = async () => {
        if (!selected || selected.trashed) return;
        setEntryActionPending(true);
        setMoreMenuOpen(false);
        try {
            const recycled = !demoMode && (window as Window & { go?: unknown }).go
                ? await MoveEntryToRecycleBin(selected.id) as VaultItem
                : {
                    ...selected,
                    folder: '回收站',
                    modified: '刚刚',
                    trashed: true,
                    deletedAt: new Date().toISOString(),
                    previousFolder: selected.folder,
                };
            setItems((current) => current.map((item) => item.id === selected.id ? recycled : item));
            setSelectedId('');
            setShowPassword(false);
            setEditing(false);
            setGenerator(null);
            notify(t('entry.movedTrash'));
        } catch (reason) {
            notify(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setEntryActionPending(false);
        }
    };

    if (!vaultOpen) {
        return <Launcher onUnlock={(snapshot) => openSnapshot(snapshot)} onDemo={openDemo} />;
    }

    return (
        <main className="app-shell">
            <aside className={`sidebar ${mobileNavOpen ? 'sidebar-open' : ''}`}>
                <div className="brand-row">
                    <div className="brand-mark"><ShieldCheck size={19} /></div>
                    <span>KeyNest</span>
                    <button className="icon-button sidebar-close" onClick={() => setMobileNavOpen(false)} title={t('nav.close')}><X size={17} /></button>
                </div>

                <button className="vault-switcher" title={vaultPath}>
                    <span className="vault-avatar">{vaultName.slice(0, 1).toUpperCase()}</span>
                    <span className="vault-copy"><strong>{vaultName}</strong><small>{t(demoMode ? 'nav.demoMode' : 'nav.localKdbx')}</small></span>
                    <ChevronDown size={15} />
                </button>

                <nav className="nav-sections" aria-label={t('nav.vault')}>
                    <div className="nav-group primary-nav">
                        {navItems.map(({ labelKey, icon: Icon, filter: value }) => {
                            const acceptsUngroupedDrop = Boolean(
                                value === 'ungrouped'
                                && draggedEntry
                                && !draggedEntry.trashed
                                && draggedEntry.groupId
                                && !entryActionPending
                            );
                            return <button
                                key={value}
                                className={`nav-row ${!settingsOpen && filter === value ? 'active' : ''} ${dropTargetGroupID === UNGROUPED_DROP_TARGET && acceptsUngroupedDrop ? 'entry-drop-target' : ''}`}
                                onClick={() => { cancelItemEditing(); setSettingsOpen(false); setFilter(value); }}
                                onDragEnter={(event) => {
                                    if (!acceptsUngroupedDrop) {
                                        if (value === 'ungrouped' && draggedEntry) setDropTargetGroupID(null);
                                        return;
                                    }
                                    event.preventDefault();
                                    setDropTargetGroupID(UNGROUPED_DROP_TARGET);
                                }}
                                onDragOver={(event) => {
                                    if (!acceptsUngroupedDrop) return;
                                    event.preventDefault();
                                    event.dataTransfer.dropEffect = 'move';
                                }}
                                onDrop={(event) => {
                                    if (!acceptsUngroupedDrop) return;
                                    event.preventDefault();
                                    const entryID = event.dataTransfer.getData('application/x-keynest-entry-id') || draggedEntryID;
                                    finishEntryDrag();
                                    if (entryID) void moveEntryToGroup(entryID, '');
                                }}
                                title={acceptsUngroupedDrop ? t('nav.moveUngrouped') : undefined}
                            >
                                <Icon size={17} /><span>{t(labelKey)}</span>
                                {value === 'all' && <em>{items.filter((item) => item.id !== draftEntryID && !item.trashed).length}</em>}
                                {value === 'favorite' && <em>{favoriteCount}</em>}
                                {value === 'ungrouped' && <em>{ungroupedCount}</em>}
                            </button>;
                        })}
                    </div>

                    <div className="nav-group group-nav-section">
                        <div
                            className={`nav-label group-root-drop-zone ${groupDropTargetID === ROOT_GROUP_DROP_TARGET ? 'drop-target' : ''}`}
                            onDragEnter={(event) => {
                                if (!draggedGroup || groupActionPending) {
                                    if (draggedGroup) setGroupDropTargetID(null);
                                    return;
                                }
                                event.preventDefault();
                                setGroupDropTargetID(ROOT_GROUP_DROP_TARGET);
                            }}
                            onDragOver={(event) => {
                                if (!draggedGroup || groupActionPending) return;
                                event.preventDefault();
                                event.dataTransfer.dropEffect = 'move';
                            }}
                            onDrop={(event) => {
                                event.preventDefault();
                                const groupID = event.dataTransfer.getData('application/x-keynest-group-id') || draggedGroupID;
                                const acceptsRootDrop = Boolean(draggedGroup && !groupActionPending);
                                finishGroupDrag();
                                if (acceptsRootDrop && groupID) void moveGroupToParent(groupID, '');
                            }}
                        ><span>{t('nav.groups')}</span><button type="button" onClick={openCreateGroup} title={t('nav.newGroup')}><FolderPlus size={14} /></button></div>
                        <div className="group-tree-scroll">
                            {groupTree.visible.map((group) => {
                                const hasChildren = groupTree.parentIDs.has(group.id);
                                const collapsed = collapsedGroupIDs.has(group.id);
                                const acceptsEntryDrop = Boolean(draggedEntry && !draggedEntry.trashed && draggedEntry.groupId !== group.id && !entryActionPending);
                                const acceptsGroupDrop = Boolean(
                                    draggedGroup
                                    && !draggedGroupDescendantIDs.has(group.id)
                                    && !groupActionPending
                                );
                                return <div
                                    key={group.id}
                                    className={`group-tree-row ${filter === groupFilter(group.id) ? 'active' : ''} ${groupMenuID === group.id ? 'menu-open' : ''} ${dropTargetGroupID === group.id && acceptsEntryDrop ? 'entry-drop-target' : ''} ${groupDropTargetID === group.id && acceptsGroupDrop ? 'group-drop-target' : ''} ${draggedGroupID === group.id ? 'dragging' : ''}`}
                                    style={{ paddingLeft: `${5 + group.depth * 14}px` }}
                                    onDragEnter={(event) => {
                                        if (acceptsGroupDrop) {
                                            event.preventDefault();
                                            setGroupDropTargetID(group.id);
                                            setDropTargetGroupID(null);
                                        } else if (acceptsEntryDrop) {
                                            event.preventDefault();
                                            setDropTargetGroupID(group.id);
                                            setGroupDropTargetID(null);
                                        } else {
                                            if (draggedGroup) setGroupDropTargetID(null);
                                            if (draggedEntry) setDropTargetGroupID(null);
                                        }
                                    }}
                                    onDragOver={(event) => {
                                        if (!acceptsEntryDrop && !acceptsGroupDrop) return;
                                        event.preventDefault();
                                        event.dataTransfer.dropEffect = 'move';
                                    }}
                                    onDrop={(event) => {
                                        event.preventDefault();
                                        const entryID = event.dataTransfer.getData('application/x-keynest-entry-id') || draggedEntryID;
                                        const movedGroupID = event.dataTransfer.getData('application/x-keynest-group-id') || draggedGroupID;
                                        if (acceptsGroupDrop && movedGroupID) {
                                            finishGroupDrag();
                                            void moveGroupToParent(movedGroupID, group.id);
                                        } else {
                                            finishEntryDrag();
                                            if (acceptsEntryDrop && entryID) void moveEntryToGroup(entryID, group.id);
                                        }
                                    }}
                                >
                                    {hasChildren ? <button
                                        type="button"
                                        className="group-toggle"
                                        onClick={() => setCollapsedGroupIDs((current) => {
                                            const next = new Set(current);
                                            if (collapsed) next.delete(group.id);
                                            else next.add(group.id);
                                            return next;
                                        })}
                                        aria-label={t(collapsed ? 'nav.expandNamed' : 'nav.collapseNamed', {name: group.name})}
                                        aria-expanded={!collapsed}
                                        title={t(collapsed ? 'nav.expandGroup' : 'nav.collapseGroup')}
                                    >
                                        {collapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
                                    </button> : <span className="group-toggle-spacer" />}
                                    <button
                                        type="button"
                                        className="group-row-main"
                                        draggable={!groupActionPending}
                                        onDragStart={(event) => startGroupDrag(event, group)}
                                        onDragEnd={finishGroupDrag}
                                        onClick={() => selectGroup(group.id)}
                                        title={acceptsEntryDrop ? t('nav.moveTo', {name: group.name}) : acceptsGroupDrop ? t('nav.moveToChildFirst', {name: group.name}) : group.name}
                                    >
                                        <KeePassGroupIcon iconId={group.iconId} size={17} /><span>{group.name}</span><em>{groupCounts.get(group.id) ?? 0}</em>
                                    </button>
                                    <div className="group-actions" ref={groupMenuID === group.id ? groupMenuRef : undefined}>
                                        <button
                                            type="button"
                                            className="group-more-button"
                                            onClick={() => {
                                                setMoreMenuOpen(false);
                                                setGroupMenuID((current) => current === group.id ? null : group.id);
                                            }}
                                            title={t('nav.groupActions')}
                                            aria-label={t('nav.namedGroupActions', {name: group.name})}
                                            aria-haspopup="menu"
                                            aria-expanded={groupMenuID === group.id}
                                        ><MoreHorizontal size={15} /></button>
                                        {groupMenuID === group.id && <div className="group-action-menu" role="menu">
                                            <button type="button" role="menuitem" onClick={() => openEditGroup(group)}><Edit3 size={15} /><span>{t('common.edit')}</span></button>
                                            <button type="button" role="menuitem" className="destructive" onClick={() => { setDeleteGroupTarget(group); setGroupMenuID(null); }}><Trash2 size={15} /><span>{t('common.delete')}</span></button>
                                        </div>}
                                    </div>
                                </div>;
                            })}
                        </div>
                    </div>

                </nav>

                <div className="sidebar-footer">
                    <button className={`nav-row ${!settingsOpen && filter === 'trash' ? 'active' : ''}`} onClick={() => { cancelItemEditing(); setSettingsOpen(false); setFilter('trash'); }}><Trash2 size={17} /><span>{t('nav.trash')}</span><em>{trashCount}</em></button>
                    <button className={`nav-row ${settingsOpen ? 'active' : ''}`} type="button" onClick={() => { cancelItemEditing(); setSettingsOpen(true); setSettingsQuery(''); setMobileNavOpen(false); }}>
                        <Settings size={17} /><span>{t('nav.settings')}</span><ChevronRight size={13} />
                    </button>
                    <button className="nav-row lock-vault-button" type="button" onClick={() => void lockVault()}>
                        <LockKeyhole size={17} /><span>{t('nav.lock')}</span>
                    </button>
                </div>
            </aside>

            {settingsOpen ? <>
                <section className="settings-nav-pane">
                    <header className="settings-nav-header">
                        <button className="icon-button mobile-menu" onClick={() => setMobileNavOpen(true)} title={t('nav.open')}><Menu size={19} /></button>
                        <h1>{t('settings.title')}</h1>
                    </header>
                    <div className="settings-search">
                        <Search size={16} />
                        <input value={settingsQuery} onChange={(event) => setSettingsQuery(event.target.value)} placeholder={t('settings.search')} aria-label={t('settings.search')} />
                        {settingsQuery && <button type="button" onClick={() => setSettingsQuery('')} title={t('settings.clearSearch')}><X size={14} /></button>}
                    </div>
                    <div className="settings-nav-list">
                        <span>{t('settings.application')}</span>
                        <button type="button" className={settingsPage === 'general' ? 'active' : ''} onClick={() => setSettingsPage('general')}><Languages size={17} /><span>{t('settings.general')}</span><ChevronRight size={13} /></button>
                        <button type="button" className={settingsPage === 'updates' ? 'active' : ''} onClick={() => setSettingsPage('updates')}><Info size={17} /><span>{t('settings.about')}</span><ChevronRight size={13} /></button>
                        <span className="settings-nav-group">{t('settings.vault')}</span>
                        <button type="button" className={settingsPage === 'security' ? 'active' : ''} onClick={() => setSettingsPage('security')}><ShieldCheck size={17} /><span>{t('settings.security')}</span><ChevronRight size={13} /></button>
                        <button type="button" className={settingsPage === 'templates' ? 'active' : ''} onClick={() => setSettingsPage('templates')}><FileKey2 size={17} /><span>{t('template.manage')}</span><ChevronRight size={13} /></button>
                    </div>
                </section>

                <section className="settings-detail-pane">
                    <header className="detail-toolbar settings-detail-toolbar">
                        <div className="sync-state"><span /><span>{t(demoMode ? 'settings.demoData' : 'settings.savedLocal')}</span></div>
                    </header>
                    <div className="settings-detail-content">
                        <h1>{t(settingsPage === 'general' ? 'settings.general' : settingsPage === 'security' ? 'settings.security' : settingsPage === 'templates' ? 'template.manage' : 'settings.about')}</h1>
                        {settingsPage === 'general'
                            ? generalSettingVisible
                                ? <section className="settings-section">
                                    <h2>{t('settings.language')}</h2>
                                    <div className="settings-panel">
                                        <div className="settings-row">
                                            <div><strong>{t('settings.language')}</strong><p>{t('settings.languageDescription')}</p></div>
                                            <select className="settings-select" value={locale} onChange={(event) => setLocale(event.target.value as typeof locale)} aria-label={t('settings.language')}>
                                                <option value="zh-CN">{t('settings.zhCN')}</option>
                                                <option value="en">{t('settings.english')}</option>
                                            </select>
                                        </div>
                                    </div>
                                </section>
                                : <div className="settings-empty"><Search size={22}/><span>{t('common.noMatchingSettings')}</span></div>
                            : settingsPage === 'security'
                            ? <section className="settings-section">
                                <h2>{t('settings.security')}</h2>
                                {securitySettingVisible
                                    ? <div className="settings-panel">
                                        <div className="settings-row">
                                            <div><strong>{t('settings.autoLock')}</strong><p>{t('settings.autoLockDescription')}</p></div>
                                            <select
                                                className="settings-select"
                                                value={autoLockMinutes}
                                                onChange={(event) => setAutoLockMinutes(Number(event.target.value) as AutoLockMinutes)}
                                                aria-label={t('settings.autoLock')}
                                            >
                                                {AUTO_LOCK_OPTIONS.map((minutes) => <option key={minutes} value={minutes}>{minutes} {t(minutes === 1 ? 'settings.minute' : 'settings.minutes')}</option>)}
                                            </select>
                                        </div>
                                        <div className="settings-row">
                                            <div><strong>{t('settings.masterPassword')}</strong><p>{t('settings.masterPasswordDescription', {name: vaultName})}</p></div>
                                            <button type="button" className="secondary-button" onClick={() => { setVisibleMasterPasswordField(null); setChangePasswordOpen(true); }}><KeyRound size={15} /> {t('settings.modify')}</button>
                                        </div>
                                    </div>
                                    : <div className="settings-empty"><Search size={22} /><span>{t('common.noMatchingSettings')}</span></div>}
                            </section>
                            : settingsPage === 'templates'
                                ? templateSettingVisible
                                    ? <TemplateManager templates={managedTemplates.map((template) => ({...template, name: templateName(template), fields: template.fields.map((field) => ({...field, label: templateFieldLabel(template, field)}))}))} t={t} onSave={saveTemplate} onDelete={setDeleteTemplateTarget} onReorder={reorderTemplates} onToggleEnabled={setTemplateEnabled}/>
                                    : <div className="settings-empty"><Search size={22}/><span>{t('common.noMatchingSettings')}</span></div>
                            : updateSettingVisible
                                ? <UpdateSettings/>
                                : <div className="settings-empty"><Search size={22}/><span>{t('common.noMatchingSettings')}</span></div>}
                    </div>
                </section>
            </> : <>
            <section className="item-pane">
                <header className="pane-toolbar">
                    <button className="icon-button mobile-menu" onClick={() => setMobileNavOpen(true)} title={t('nav.open')}><Menu size={19} /></button>
                    <div className="search-field">
                        <Search size={17} />
                        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('entry.search')} aria-label={t('entry.search')} />
                        <kbd>⌘ K</kbd>
                    </div>
                </header>

                <div className="list-heading">
                    <div><h1>{sectionTitle}</h1><span>{t('common.items', {count: visibleItemCount})}</span></div>
                    {filter === 'trash'
                        ? trashCount > 0 && <button className="clear-trash-button" onClick={() => setClearTrashOpen(true)} title={t('entry.clearTrash')}><Trash2 size={14} /> {t('entry.clear')}</button>
                        : <div className="template-create" ref={templatePickerRef}>
                            <button className={`add-button ${templatePickerOpen ? 'active' : ''}`} onClick={() => setTemplatePickerOpen((open) => !open)} disabled={entryActionPending} title={t('entry.new')} aria-haspopup="menu" aria-expanded={templatePickerOpen}><Plus size={18} /></button>
                            {templatePickerOpen && <div className="template-picker" role="menu">
                                <button type="button" role="menuitem" onClick={() => createItem('login')}><Plus size={17}/><span><strong>{t('entry.new')}</strong><small>{t('template.fieldCount', {count: defaultEntryTemplate.fields.length})}</small></span></button>
                                <span>{t('template.choose')}</span>
                                {enabledManagedTemplates.map((template) => {
                                    const Icon = templateIcons[template.id] ?? FileKey2;
                                    return <button key={template.id} type="button" role="menuitem" onClick={() => createItem(template.id)}><Icon size={17}/><span><strong>{templateName(template)}</strong><small>{t('template.fieldCount', {count: template.fields.length})}</small></span></button>;
                                })}
                            </div>}
                        </div>}
                </div>

                <div className="item-list" role="listbox" aria-label={sectionTitle}>
                    {visibleItems.map((item) => {
                        const remainingDays = filter === 'trash' ? recycleDaysRemaining(item.deletedAt) : null;
                        return <button
                            key={item.id}
                            className={`item-row ${item.id === selected?.id ? 'selected' : ''} ${draggedEntryID === item.id ? 'dragging' : ''}`}
                            onClick={() => selectItem(item)}
                            draggable={!item.trashed && !editing && !entryActionPending}
                            onDragStart={(event) => startEntryDrag(event, item)}
                            onDragEnd={finishEntryDrag}
                            role="option"
                            aria-selected={item.id === selected?.id}
                        >
                            <span className="site-icon" style={{ background: item.color, color: item.name === 'Notion' ? '#171717' : '#fff' }}>{item.initials}</span>
                            <span className="item-summary">
                                <span className="item-name">{item.name}{item.favorite && <Star size={12} fill="currentColor" />}</span>
                                <span>{item.username}</span>
                            </span>
                            <span className={`item-time ${remainingDays !== null ? 'retention-time' : ''}`}>
                                {remainingDays === null
                                    ? formatLastUsed(filter === 'recent' ? item.lastUsedAt : item.modifiedAt, item.modified, locale, t('common.justNow'))
                                    : t(remainingDays === 0 ? 'entry.autoDeleteToday' : remainingDays === 1 ? 'entry.autoDeleteDay' : 'entry.autoDeleteDays', {count: remainingDays})}
                            </span>
                        </button>;
                    })}
                    {visibleItems.length === 0 && <div className="empty-state">{filter === 'trash' ? <Trash2 size={24} /> : <Search size={24} />}<strong>{t(filter === 'trash' ? 'entry.trashEmpty' : 'entry.emptySearch')}</strong><span>{t(filter === 'trash' ? 'entry.noOldRecords' : 'entry.tryAnother')}</span></div>}
                </div>
            </section>

            <section className="detail-pane">
                <header className="detail-toolbar">
                    <div className={`sync-state ${isDraftSelected ? 'pending' : ''}`}><span /><span>{t(isDraftSelected ? 'entry.unsavedDraft' : demoMode ? 'settings.demoData' : 'settings.savedLocal')}</span></div>
                    {selected && (selected.trashed
                        ? <div className="toolbar-actions">
                            <button className="icon-button" type="button" onClick={restoreSelected} disabled={entryActionPending} title={t('entry.restore')} aria-label={t('entry.restore')}><RotateCcw size={18} /></button>
                            <button className="icon-button destructive-icon" type="button" onClick={() => setDeleteTarget(selected)} disabled={entryActionPending} title={t('entry.permanentDelete')} aria-label={t('entry.permanentDelete')}><Trash2 size={18} /></button>
                        </div>
                        : isDraftSelected ? null : <div className="toolbar-actions">
                            <button className={`icon-button ${selected.favorite ? 'is-favorite' : ''}`} onClick={toggleFavorite} title={t(selected.favorite ? 'entry.favoriteRemove' : 'entry.favoriteAdd')}><Star size={18} fill={selected.favorite ? 'currentColor' : 'none'} /></button>
                            <button className="icon-button" onClick={() => editing ? cancelItemEditing() : beginItemEditing()} title={t('entry.edit')}><Edit3 size={18} /></button>
                            <div className="entry-more" ref={moreMenuRef}>
                                <button className={`icon-button ${moreMenuOpen ? 'active' : ''}`} type="button" onClick={() => setMoreMenuOpen((value) => !value)} title={t('entry.moreActions')} aria-haspopup="menu" aria-expanded={moreMenuOpen}><MoreHorizontal size={19} /></button>
                                {moreMenuOpen && <div className="entry-actions-menu" role="menu">
                                    <button type="button" role="menuitem" onClick={duplicateSelected} disabled={entryActionPending}><Copy size={16} /><span>{t('common.copy')}</span></button>
                                    <button type="button" role="menuitem" className="destructive" onClick={moveSelectedToRecycleBin} disabled={entryActionPending}><Trash2 size={16} /><span>{t('common.delete')}</span></button>
                                </div>}
                            </div>
                        </div>)}
                </header>

                {selected ? <form key={selected.id} className="detail-content" onSubmit={saveItem}>
                    <div className="detail-title">
                        <span className="site-icon detail-icon" style={{ background: selected.color, color: selected.name === 'Notion' ? '#171717' : '#fff' }}>{selected.initials}</span>
                        <div>
                            {isEditingSelected ? <input className="title-input" name="name" defaultValue={selected.name} autoFocus /> : <h2>{selected.name}</h2>}
                            {(selectedGroupPath.length > 0 || selected.folder) && <span className="detail-meta">
                                <Folder size={13} />
                                {selectedGroupPath.length > 0
                                    ? <span className="detail-group-path">
                                        {selectedGroupPath.map((group) => <button key={group.id} type="button" onClick={() => selectGroup(group.id)} title={t('entry.selectGroup', {name: group.name})}>{group.name}</button>)}
                                    </span>
                                    : <span>{selected.folder}</span>}
                            </span>}
                            <span className="entry-template-meta"><FileKey2 size={12}/>{isEditingSelected
                                ? <select value={selectedTemplate.id} onChange={(event) => changeSelectedTemplate(event.target.value)} aria-label={t('template.change')}>
                                    {selectedTemplate.deleted && <option value={selectedTemplate.id}>{templateName(selectedTemplate)}</option>}
                                    {availableTemplates.map((template) => <option key={template.id} value={template.id}>{templateName(template)}</option>)}
                                </select>
                                : <span>{templateName(selectedTemplate)}</span>}
                            </span>
                        </div>
                    </div>

                    {selectedTemplateSections.sections.map(([section, sectionFields]) => <div className={`detail-section ${section === 'notes' ? 'notes-section' : ''}`} key={section}>
                        <h3>{templateSectionLabel(section)}</h3>
                        {sectionFields.map((field) => {
                            const storedValue = fieldValue(selected.fields, field.storageKey);
                            const currentValue = field.storageKey === 'Password' && isEditingSelected ? draftPassword : storedValue;
                            const revealed = field.storageKey === 'Password' ? showPassword : revealedFields.has(field.storageKey);
                            const displayValue = field.protected && !revealed && currentValue ? '••••••••••••' : currentValue || t('common.notSet');
                            const label = templateFieldLabel(selectedTemplate, field);
                            const fieldActions = <div className={field.storageKey === 'Password' ? 'password-actions-wrap' : 'field-actions'}>
                                <div className="field-actions">
                                    {field.storageKey === 'Password' && !selected.trashed && <button type="button" className={`field-action ${generator === 'detail' ? 'active' : ''}`} onClick={() => setGenerator((value) => value === 'detail' ? null : 'detail')} title={t('entry.generatePassword')}><Zap size={16}/></button>}
                                    {field.protected && <button type="button" className="field-action" onClick={() => field.storageKey === 'Password' ? setShowPassword((value) => !value) : setRevealedFields((current) => {
                                        const next = new Set(current);
                                        if (next.has(field.storageKey)) next.delete(field.storageKey); else next.add(field.storageKey);
                                        return next;
                                    })} title={t(revealed ? 'launcher.hidePassword' : 'launcher.showPassword')}>{revealed ? <EyeOff size={16}/> : <Eye size={16}/>}</button>}
                                    {field.type === 'url' && <button type="button" className="field-action" onClick={() => openEntryWebsite({...selected, url: currentValue})} disabled={!currentValue} title={t('entry.openWebsite')}><Globe2 size={16}/></button>}
                                    <button type="button" className="field-action" onClick={() => copyValue(label, currentValue, isDraftSelected ? undefined : selected)} disabled={!currentValue} title={t('template.copyField', {name: label})}><Copy size={16}/></button>
                                </div>
                                {field.storageKey === 'Password' && generator === 'detail' && (
                                    <PasswordGenerator onClose={() => setGenerator(null)} onUse={(password) => { beginItemEditing(); setDraftPassword(password); setShowPassword(true); setGenerator(null); }}/>
                                )}
                            </div>;
                            return <Field
                                key={field.id}
                                label={label}
                                name={`field:${field.storageKey}`}
                                value={displayValue}
                                originalValue={storedValue}
                                inputValue={field.storageKey === 'Password' ? draftPassword : undefined}
                                onInputChange={field.storageKey === 'Password' ? setDraftPassword : undefined}
                                inputType={field.type === 'password' && !revealed ? 'password' : field.type === 'textarea' ? 'text' : field.type}
                                multiline={field.type === 'textarea'}
                                required={field.required}
                                editing={isEditingSelected}
                                strong={field.protected}
                                link={field.type === 'url' && Boolean(currentValue)}
                                action={fieldActions}
                            />;
                        })}
                        {sectionFields.some((field) => field.storageKey === 'Password') && <div className="strength-row" data-level={detailPasswordStrength.level}>
                            <span>{t('entry.passwordStrength')}</span>
                            <div className="strength-bars" role="meter" aria-label={t('entry.passwordStrength')} aria-valuemin={0} aria-valuemax={4} aria-valuenow={detailPasswordStrength.level}>{[0, 1, 2, 3].map((level) => <i key={level} className={level < detailPasswordStrength.level ? 'active' : ''}/>)}</div>
                            <strong>{t(passwordStrengthMessageKey(detailPasswordStrength.level))}</strong>
                        </div>}
                    </div>)}

                    {selectedTemplateSections.extraFields.length > 0 && <div className="detail-section">
                        <h3>{t('template.otherFields')}</h3>
                        {selectedTemplateSections.extraFields.map((field) => <Field key={field.key} label={field.key} name={`extra:${field.key}`} value={field.protected && !revealedFields.has(field.key) ? '••••••••••••' : field.value || t('common.notSet')} originalValue={field.value} inputType={field.protected && !revealedFields.has(field.key) ? 'password' : 'text'} editing={isEditingSelected} strong={field.protected} action={<div className="field-actions">{field.protected && <button type="button" className="field-action" onClick={() => setRevealedFields((current) => { const next = new Set(current); if (next.has(field.key)) next.delete(field.key); else next.add(field.key); return next; })} title={t(revealedFields.has(field.key) ? 'launcher.hidePassword' : 'launcher.showPassword')}>{revealedFields.has(field.key) ? <EyeOff size={16}/> : <Eye size={16}/>}</button>}<button type="button" className="field-action" onClick={() => copyValue(field.key, field.value, isDraftSelected ? undefined : selected)}><Copy size={16}/></button></div>}/>) }
                    </div>}

                    <div className="detail-section tags-section">
                        <h3>{t('entry.tags')}</h3>
                        <div className="tag-list">{selected.tags.map((tag) => <span key={tag}>{tag}</span>)}<button type="button" title={t('entry.addTag')}><Plus size={13} /></button></div>
                    </div>

                    <div className="detail-section time-section">
                        <h3>{t('entry.time')}</h3>
                        <div className="entry-time-row"><span>{t('entry.createdAt')}</span><time dateTime={selected.createdAt}>{formatDetailTime(selected.createdAt, locale, t('common.unknown'))}</time></div>
                        <div className="entry-time-row"><span>{t('entry.modifiedAt')}</span><time dateTime={selected.modifiedAt}>{formatDetailTime(selected.modifiedAt, locale, t('common.unknown'))}</time></div>
                        {!selected.trashed && <button type="button" className="history-link" onClick={() => void openHistory()} disabled={(selected.historyCount ?? 0) === 0}>
                            <HistoryIcon size={17} />
                            <span><strong>{t('entry.history')}</strong><small>{(selected.historyCount ?? 0) > 0 ? t('entry.historyCount', {count: selected.historyCount ?? 0}) : t('entry.historyEmpty')}</small></span>
                            <ChevronRight size={16} />
                        </button>}
                    </div>

                    {isEditingSelected && <div className="edit-actions"><button type="button" className="secondary-button" onClick={cancelItemEditing}>{t('common.cancel')}</button><button type="submit" className="primary-button"><Check size={16} /> {t('common.saveChanges')}</button></div>}
                </form> : filter === 'trash'
                    ? <div className="empty-detail"><Trash2 size={30} /><strong>{t('entry.trashEmpty')}</strong><span>{t('entry.noOldRecords')}</span></div>
                    : <div className="empty-detail"><ShieldCheck size={30} /><strong>{t('entry.vaultEmpty')}</strong><span>{t('entry.createFirst')}</span><button className="primary-button" onClick={() => setTemplatePickerOpen(true)} disabled={entryActionPending}><Plus size={16} /> {t('entry.new')}</button></div>}
            </section>
            </>}

            {historyOpen && selected && (
                <div className="modal-backdrop" onMouseDown={closeHistory}>
                    <div className="create-dialog history-dialog" role="dialog" aria-modal="true" aria-labelledby="history-title" onMouseDown={(event) => event.stopPropagation()}>
                        <header>
                            <div><h2 id="history-title">{t('entry.history')}</h2><p>{t('entry.historyDescription', {name: selected.name})}</p></div>
                            <button type="button" className="icon-button" onClick={closeHistory} title={t('common.close')}><X size={18} /></button>
                        </header>
                        {historyLoading ? <div className="history-status" role="status"><Clock3 size={22} /><span>{t('entry.historyLoading')}</span></div>
                            : historyError ? <div className="history-status history-error" role="alert"><Info size={22} /><span>{historyError}</span><button type="button" className="secondary-button" onClick={() => void openHistory()}>{t('entry.historyRetry')}</button></div>
                            : !historyVersion ? <div className="history-status"><HistoryIcon size={22} /><span>{t('entry.historyEmpty')}</span></div>
                            : <div className="history-layout">
                            <nav className="history-list" aria-label={t('entry.history')}>
                                {historyVersions.map((version, index) => (
                                    <button key={`${version.modifiedAt}-${index}`} type="button" className={historyIndex === index ? 'selected' : ''} onClick={() => { setHistoryIndex(index); setShowHistoryPassword(false); }}>
                                        <HistoryIcon size={15} />
                                        <span><strong>{formatDetailTime(version.modifiedAt, locale, t('common.unknown'))}</strong><small>{version.name}</small></span>
                                        <ChevronRight size={14} />
                                    </button>
                                ))}
                            </nav>
                            <section className="history-preview">
                                <div className="history-preview-title"><span className="site-icon" style={{background: historyVersion.color, color: '#fff'}}>{historyVersion.initials}</span><div><h3>{historyVersion.name}</h3><time dateTime={historyVersion.modifiedAt}>{formatDetailTime(historyVersion.modifiedAt, locale, t('common.unknown'))}</time></div></div>
                                {historyDisplayFields.map((field) => <Field key={field.key} label={field.label} name={`history:${field.key}`} value={field.protected && !showHistoryPassword && field.value ? '••••••••••••' : field.value || t('common.notSet')} editing={false} strong={field.protected} action={<div className="field-actions">{field.protected && <button type="button" className="field-action" onClick={() => setShowHistoryPassword((value) => !value)} title={t(showHistoryPassword ? 'launcher.hidePassword' : 'launcher.showPassword')}>{showHistoryPassword ? <EyeOff size={16}/> : <Eye size={16}/>}</button>}<button type="button" className="field-action" onClick={() => copyValue(field.label, field.value)} disabled={!field.value} title={t('template.copyField', {name: field.label})}><Copy size={16}/></button></div>}/>) }
                                {historyVersion.tags.length > 0 && <div className="tag-list history-tags">{historyVersion.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>}
                            </section>
                        </div>}
                    </div>
                </div>
            )}

            {changePasswordOpen && (
                <div className="modal-backdrop" onMouseDown={closeChangePassword}>
                    <form className="create-dialog change-password-dialog" onSubmit={changeMasterPassword} onMouseDown={(event) => event.stopPropagation()}>
                        <header>
                            <div><h2>{t('dialog.changePassword')}</h2><p>{vaultName}</p></div>
                            <button type="button" className="icon-button" onClick={closeChangePassword} disabled={changingPassword} title={t('common.close')}><X size={18} /></button>
                        </header>
                        <label>{t('dialog.currentPassword')}
                            <div className="dialog-password">
                                <input name="currentPassword" type={visibleMasterPasswordField === 'current' ? 'text' : 'password'} autoComplete="current-password" autoFocus placeholder={t('dialog.currentPasswordPlaceholder')} />
                                <button type="button" onClick={() => setVisibleMasterPasswordField((value) => value === 'current' ? null : 'current')} title={t(visibleMasterPasswordField === 'current' ? 'dialog.hideCurrentPassword' : 'dialog.showCurrentPassword')}>{visibleMasterPasswordField === 'current' ? <EyeOff size={16} /> : <Eye size={16} />}</button>
                            </div>
                        </label>
                        <label>{t('dialog.newPassword')}
                            <div className="dialog-password">
                                <input name="newPassword" type={visibleMasterPasswordField === 'new' ? 'text' : 'password'} autoComplete="new-password" required />
                                <button type="button" onClick={() => setVisibleMasterPasswordField((value) => value === 'new' ? null : 'new')} title={t(visibleMasterPasswordField === 'new' ? 'dialog.hideNewPassword' : 'dialog.showNewPassword')}>{visibleMasterPasswordField === 'new' ? <EyeOff size={16} /> : <Eye size={16} />}</button>
                            </div>
                        </label>
                        <label>{t('dialog.confirmNewPassword')}
                            <div className="dialog-password">
                                <input name="confirmPassword" type={visibleMasterPasswordField === 'confirm' ? 'text' : 'password'} autoComplete="new-password" required />
                                <button type="button" onClick={() => setVisibleMasterPasswordField((value) => value === 'confirm' ? null : 'confirm')} title={t(visibleMasterPasswordField === 'confirm' ? 'dialog.hideConfirmation' : 'dialog.showConfirmation')}>{visibleMasterPasswordField === 'confirm' ? <EyeOff size={16} /> : <Eye size={16} />}</button>
                            </div>
                        </label>
                        <footer>
                            <button type="button" className="secondary-button" onClick={closeChangePassword} disabled={changingPassword}>{t('common.cancel')}</button>
                            <button type="submit" className="primary-button" disabled={changingPassword}><KeyRound size={15} /> {t(changingPassword ? 'dialog.changing' : 'dialog.modifyPassword')}</button>
                        </footer>
                    </form>
                </div>
            )}

            {groupDialogOpen && (
                <div className="modal-backdrop" onMouseDown={() => !creatingGroup && closeGroupDialog()}>
                    <form key={editingGroup?.id ?? 'new-group'} className="create-dialog group-dialog" onSubmit={saveGroup} onMouseDown={(event) => event.stopPropagation()}>
                        <header>
                            <div><h2>{t(editingGroup ? 'dialog.editGroup' : 'dialog.newGroup')}</h2><p>{t(editingGroup ? 'dialog.editGroupDescription' : 'dialog.newGroupDescription')}</p></div>
                            <button type="button" className="icon-button" onClick={closeGroupDialog} disabled={creatingGroup} title={t('common.close')}><X size={18} /></button>
                        </header>
                        <label>{t('dialog.name')}<input name="name" required autoFocus defaultValue={editingGroup?.name ?? ''} placeholder={t('dialog.namePlaceholder')} /></label>
                        <fieldset className="group-icon-fieldset">
                            <legend>{t('dialog.icon')}</legend>
                            <div className="group-icon-picker" role="radiogroup" aria-label={t('dialog.groupIcon')}>
                                {keepassGroupIcons.map(({ id, label }) => {
                                    const localizedLabel = localizedGroupIconLabel(id, label, locale);
                                    return (
                                        <button
                                            key={id}
                                            type="button"
                                            className={groupIconID === id ? 'selected' : ''}
                                            onClick={() => setGroupIconID(id)}
                                            role="radio"
                                            aria-checked={groupIconID === id}
                                            aria-label={localizedLabel}
                                            title={localizedLabel}
                                        >
                                            <KeePassGroupIcon iconId={id} size={18} />
                                        </button>
                                    );
                                })}
                            </div>
                        </fieldset>
                        <footer>
                            <button type="button" className="secondary-button" onClick={closeGroupDialog} disabled={creatingGroup}>{t('common.cancel')}</button>
                            <button type="submit" className="primary-button" disabled={creatingGroup}>{editingGroup ? <Edit3 size={15} /> : <FolderPlus size={15} />} {t(creatingGroup ? 'common.saving' : editingGroup ? 'common.saveChanges' : 'dialog.createGroup')}</button>
                        </footer>
                    </form>
                </div>
            )}

            {deleteGroupTarget && (
                <div className="modal-backdrop" onMouseDown={() => !groupActionPending && setDeleteGroupTarget(null)}>
                    <div className="create-dialog confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="delete-group-title" onMouseDown={(event) => event.stopPropagation()}>
                        <header>
                            <div>
                                <h2 id="delete-group-title">{t('dialog.deleteGroup')}</h2>
                                <p>{t('dialog.deleteGroupDescription', {
                                    name: deleteGroupTarget.name,
                                    children: deleteGroupSummary.childGroups > 0 ? t('dialog.childGroups', {count: deleteGroupSummary.childGroups}) : '',
                                    entries: deleteGroupSummary.entries,
                                })}</p>
                            </div>
                            <button type="button" className="icon-button" onClick={() => setDeleteGroupTarget(null)} disabled={groupActionPending} title={t('common.close')}><X size={18} /></button>
                        </header>
                        <footer>
                            <button type="button" className="secondary-button" onClick={() => setDeleteGroupTarget(null)} disabled={groupActionPending}>{t('common.cancel')}</button>
                            <button type="button" className="danger-button" onClick={deleteSelectedGroup} disabled={groupActionPending}><Trash2 size={15} /> {t(groupActionPending ? 'common.deleting' : 'dialog.deleteGroup')}</button>
                        </footer>
                    </div>
                </div>
            )}

            {deleteTemplateTarget && (
                <div className="modal-backdrop" onMouseDown={() => !deletingTemplate && setDeleteTemplateTarget(null)}>
                    <div className="create-dialog confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="delete-template-title" onMouseDown={(event) => event.stopPropagation()}>
                        <header>
                            <div><h2 id="delete-template-title">{t('template.delete')}</h2><p>{t('template.deleteConfirm', {name: templateName(deleteTemplateTarget)})}</p></div>
                            <button type="button" className="icon-button" onClick={() => setDeleteTemplateTarget(null)} disabled={deletingTemplate} title={t('common.close')}><X size={18}/></button>
                        </header>
                        <footer>
                            <button type="button" className="secondary-button" onClick={() => setDeleteTemplateTarget(null)} disabled={deletingTemplate}>{t('common.cancel')}</button>
                            <button type="button" className="danger-button" onClick={() => void deleteTemplate()} disabled={deletingTemplate}><Trash2 size={15}/> {t(deletingTemplate ? 'common.deleting' : 'template.delete')}</button>
                        </footer>
                    </div>
                </div>
            )}

            {clearTrashOpen && (
                <div className="modal-backdrop" onMouseDown={() => !clearingTrash && setClearTrashOpen(false)}>
                    <div className="create-dialog confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="clear-trash-title" onMouseDown={(event) => event.stopPropagation()}>
                        <header>
                            <div><h2 id="clear-trash-title">{t('dialog.clearTrash')}</h2><p>{t('dialog.clearTrashDescription', {count: trashCount})}</p></div>
                            <button type="button" className="icon-button" onClick={() => setClearTrashOpen(false)} disabled={clearingTrash} title={t('common.close')}><X size={18} /></button>
                        </header>
                        <footer>
                            <button type="button" className="secondary-button" onClick={() => setClearTrashOpen(false)} disabled={clearingTrash}>{t('common.cancel')}</button>
                            <button type="button" className="danger-button" onClick={clearRecycleBin} disabled={clearingTrash}><Trash2 size={15} /> {t(clearingTrash ? 'common.clearing' : 'dialog.clearAll')}</button>
                        </footer>
                    </div>
                </div>
            )}

            {deleteTarget && (
                <div className="modal-backdrop" onMouseDown={() => !entryActionPending && setDeleteTarget(null)}>
                    <div className="create-dialog confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="delete-entry-title" onMouseDown={(event) => event.stopPropagation()}>
                        <header>
                            <div><h2 id="delete-entry-title">{t('dialog.deleteEntry')}</h2><p>{t('dialog.deleteEntryDescription', {name: deleteTarget.name})}</p></div>
                            <button type="button" className="icon-button" onClick={() => setDeleteTarget(null)} disabled={entryActionPending} title={t('common.close')}><X size={18} /></button>
                        </header>
                        <footer>
                            <button type="button" className="secondary-button" onClick={() => setDeleteTarget(null)} disabled={entryActionPending}>{t('common.cancel')}</button>
                            <button type="button" className="danger-button" onClick={deleteRecycleBinItem} disabled={entryActionPending}><Trash2 size={15} /> {t(entryActionPending ? 'common.deleting' : 'entry.permanentDelete')}</button>
                        </footer>
                    </div>
                </div>
            )}

            {toast && <div className="toast"><Check size={15} />{toast}</div>}
        </main>
    );
}

type FieldProps = {
    label: string;
    name: string;
    value: string;
    originalValue?: string;
    inputValue?: string;
    onInputChange?: (value: string) => void;
    inputType?: 'text' | 'password' | 'url' | 'date' | 'month';
    editing: boolean;
    action: React.ReactNode;
    strong?: boolean;
    link?: boolean;
    multiline?: boolean;
    required?: boolean;
};

function Field({ label, name, value, originalValue, inputValue, onInputChange, inputType = 'text', editing, action, strong, link, multiline, required }: FieldProps) {
    return (
        <div className="detail-field">
            <label>{label}</label>
            <div className="field-value">
                {editing
                    ? multiline
                        ? <textarea name={name} defaultValue={originalValue ?? value} required={required}/>
                        : inputValue !== undefined
                            ? <input name={name} type={inputType} value={inputValue} onChange={(event) => onInputChange?.(event.target.value)} required={required}/>
                            : <input name={name} type={inputType} defaultValue={originalValue ?? value} required={required}/>
                    : <span className={`${strong ? 'password-value' : ''} ${link ? 'link-value' : ''}`}>{value}</span>}
                {action}
            </div>
        </div>
    );
}

export default App;
