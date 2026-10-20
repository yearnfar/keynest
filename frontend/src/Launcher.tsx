import { FormEvent, useEffect, useMemo, useState } from 'react';
import {
    ArrowRight,
    Database,
    Eye,
    EyeOff,
    FileKey2,
    FilePlus2,
    FolderOpen,
    HardDrive,
    KeyRound,
    MoreHorizontal,
    ShieldCheck,
    Sparkles,
    X,
} from 'lucide-react';
import {
    ChooseDatabase,
    ChooseKeyFile,
    CreateDatabase,
    ListRecentDatabases,
    UnlockDatabase,
} from '../wailsjs/go/main/App';
import {useI18n} from './i18n';

type VaultSnapshotEntry = {
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
    historyCount?: number;
};

export type VaultSnapshot = {
    name: string;
    path: string;
    entries: VaultSnapshotEntry[];
    groups: Array<{
        id: string;
        name: string;
        parentId?: string;
        depth: number;
        iconId: number;
    }>;
};

type RecentDatabase = { name: string; path: string; modified: string };

type LauncherProps = {
    onUnlock: (snapshot: VaultSnapshot) => void;
    onDemo: () => void;
};

const hasWailsBridge = () => Boolean((window as Window & { go?: unknown }).go);

function Launcher({ onUnlock, onDemo }: LauncherProps) {
    const {t} = useI18n();
    const browserRecent = useMemo<RecentDatabase[]>(() => [
        { name: t('launcher.demoPersonalVault'), path: '/Users/demo/Documents/personal.kdbx', modified: t('common.justNow') },
        { name: t('launcher.demoWorkVault'), path: '/Users/demo/Documents/work.kdbx', modified: t('launcher.yesterday') },
        { name: t('launcher.demoFamilyVault'), path: '/Users/demo/Documents/family.kdbx', modified: t('launcher.daysAgo', {count: 6}) },
    ], [t]);
    const [recent, setRecent] = useState<RecentDatabase[]>([]);
    const [selectedPath, setSelectedPath] = useState('');
    const [password, setPassword] = useState('');
    const [keyFile, setKeyFile] = useState('');
    const [showPassword, setShowPassword] = useState(false);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const [createOpen, setCreateOpen] = useState(false);
    const [moreOpen, setMoreOpen] = useState(false);

    useEffect(() => {
        if (!hasWailsBridge()) {
            setRecent(browserRecent);
            setSelectedPath(browserRecent[0].path);
            return;
        }
        ListRecentDatabases().then((items) => {
            const next = (items ?? []) as RecentDatabase[];
            setRecent(next);
            if (next.length > 0) setSelectedPath(next[0].path);
        }).catch(() => setRecent([]));
    }, [browserRecent]);

    const selected = useMemo(() => recent.find((item) => item.path === selectedPath), [recent, selectedPath]);
    const selectedName = selected?.name || selectedPath.split(/[\\/]/).pop()?.replace(/\.kdbx$/i, '') || t('launcher.selectVault');

    const chooseDatabase = async () => {
        setError('');
        if (!hasWailsBridge()) {
            setSelectedPath(browserRecent[0].path);
            return;
        }
        const path = await ChooseDatabase();
        if (path) {
            setSelectedPath(path);
            const name = path.split(/[\\/]/).pop()?.replace(/\.kdbx$/i, '') || t('launcher.localVault');
            setRecent((items) => [{ name, path, modified: t('common.justNow') }, ...items.filter((item) => item.path !== path)]);
        }
    };

    const chooseKeyFile = async () => {
        if (!hasWailsBridge()) {
            setKeyFile('/Users/demo/Documents/keynest.keyx');
            return;
        }
        const path = await ChooseKeyFile();
        if (path) setKeyFile(path);
    };

    const unlock = async (event: FormEvent) => {
        event.preventDefault();
        if (!selectedPath) {
            setError(t('launcher.selectFirst'));
            return;
        }
        setLoading(true);
        setError('');
        try {
            if (!hasWailsBridge()) {
                onDemo();
                return;
            }
            const snapshot = await UnlockDatabase(selectedPath, password, keyFile);
            onUnlock(snapshot as VaultSnapshot);
        } catch (reason) {
            setError(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setLoading(false);
        }
    };

    const createDatabase = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const vaultPassword = String(data.get('password'));
        if (vaultPassword !== String(data.get('confirmPassword'))) {
            setError(t('launcher.passwordMismatch'));
            return;
        }
        setLoading(true);
        setError('');
        try {
            if (!hasWailsBridge()) {
                setCreateOpen(false);
                onDemo();
                return;
            }
            const snapshot = await CreateDatabase({
                name: String(data.get('name')),
                password: vaultPassword,
                keyFile,
            });
            if (!snapshot.path) return;
            setCreateOpen(false);
            onUnlock(snapshot as VaultSnapshot);
        } catch (reason) {
            setError(String(reason).replace(/^Error:\s*/, ''));
        } finally {
            setLoading(false);
        }
    };

    return (
        <main className="launcher-screen">
            <div className="launcher-brand"><span><ShieldCheck size={18} /></span>KeyNest</div>
            <div className="local-badge"><HardDrive size={14} /> {t('launcher.localOnly')}</div>

            <section className="launcher-workspace" aria-label={t('launcher.openVault')}>
                <nav className="launcher-actions" aria-label={t('launcher.vaultActions')}>
                    <button onClick={chooseDatabase}><FolderOpen /><span>{t('launcher.open')}</span></button>
                    <button onClick={() => { setError(''); setCreateOpen(true); }}><FilePlus2 /><span>{t('launcher.create')}</span></button>
                    <button onClick={onDemo}><Sparkles /><span>{t('launcher.demo')}</span></button>
                    <div className="more-action">
                        <button onClick={() => setMoreOpen((value) => !value)}><MoreHorizontal /><span>{t('launcher.more')}</span></button>
                        {moreOpen && <div className="launcher-popover"><strong>KeyNest</strong><span>{t('launcher.compatible')}</span><span>{t('launcher.dataLocal')}</span></div>}
                    </div>
                </nav>

                <form className="unlock-form" onSubmit={unlock}>
                    <div className="current-database">
                        <Database size={18} />
                        <div><span>{t('launcher.currentVault')}</span><strong>{selectedName}</strong></div>
                        <button type="button" onClick={chooseDatabase}>{t('launcher.change')}</button>
                    </div>

                    <div className={`master-password ${error ? 'has-error' : ''}`}>
                        <LockGlyph />
                        <input
                            type={showPassword ? 'text' : 'password'}
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                            placeholder={t('launcher.passwordPlaceholder', {name: selectedName})}
                            aria-label={t('launcher.masterPassword')}
                            autoFocus
                        />
                        <button type="button" onClick={() => setShowPassword((value) => !value)} title={t(showPassword ? 'launcher.hidePassword' : 'launcher.showPassword')}>
                            {showPassword ? <EyeOff size={19} /> : <Eye size={19} />}
                        </button>
                        <button className="unlock-submit" type="submit" disabled={loading} title={t('launcher.unlock')}><ArrowRight size={24} /></button>
                    </div>

                    <div className="keyfile-row">
                        <button type="button" onClick={chooseKeyFile}><KeyRound size={16} /><span>{keyFile ? keyFile.split(/[\\/]/).pop() : t('launcher.addKeyFile')}</span></button>
                        {keyFile && <button type="button" className="clear-keyfile" onClick={() => setKeyFile('')} title={t('launcher.removeKeyFile')}><X size={14} /></button>}
                    </div>
                    {error && <p className="launcher-error">{error}</p>}
                </form>

                <div className="recent-databases">
                    <div className="recent-label">{t('launcher.recent')}</div>
                    {recent.length > 0 ? recent.map((item) => (
                        <button key={item.path} className={item.path === selectedPath ? 'selected' : ''} onClick={() => { setSelectedPath(item.path); setPassword(''); setError(''); }} title={item.path}>
                            <Database size={16} /><span><strong>{item.name}</strong><small>{item.modified}</small></span>
                        </button>
                    )) : <button className="recent-empty" onClick={chooseDatabase}><FolderOpen size={16} /><span><strong>{t('launcher.openLocal')}</strong><small>{t('launcher.noRecent')}</small></span></button>}
                </div>
            </section>

            <footer className="launcher-footer"><FileKey2 size={13} /> {t('launcher.encryption')}</footer>

            {createOpen && (
                <div className="modal-backdrop dark-backdrop" onMouseDown={() => setCreateOpen(false)}>
                    <form className="create-dialog vault-dialog" onSubmit={createDatabase} onMouseDown={(event) => event.stopPropagation()}>
                        <header><div><h2>{t('launcher.createTitle')}</h2><p>{t('launcher.createDescription')}</p></div><button type="button" className="icon-button" onClick={() => setCreateOpen(false)} title={t('common.close')}><X size={18} /></button></header>
                        <label>{t('launcher.vaultName')}<input name="name" required autoFocus placeholder={t('launcher.vaultNamePlaceholder')} /></label>
                        <label>{t('launcher.masterPassword')}<input name="password" type="password" required={!keyFile} placeholder={t('launcher.passwordPlaceholderNew')} /></label>
                        <label>{t('launcher.confirmPassword')}<input name="confirmPassword" type="password" required={!keyFile} placeholder={t('launcher.confirmPasswordPlaceholder')} /></label>
                        <button type="button" className="keyfile-picker" onClick={chooseKeyFile}><KeyRound size={16} />{keyFile ? keyFile.split(/[\\/]/).pop() : t('launcher.useKeyFile')}</button>
                        {error && <p className="dialog-error">{error}</p>}
                        <footer><button type="button" className="secondary-button" onClick={() => setCreateOpen(false)}>{t('common.cancel')}</button><button type="submit" className="primary-button" disabled={loading}><FilePlus2 size={16} /> {t('launcher.createKdbx')}</button></footer>
                    </form>
                </div>
            )}
        </main>
    );
}

function LockGlyph() {
    return <div className="password-lock"><ShieldCheck size={19} /></div>;
}

export default Launcher;
