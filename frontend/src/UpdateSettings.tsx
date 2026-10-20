import {useEffect, useState} from 'react';
import {CheckCircle2, Code2, Download, ExternalLink, RefreshCw, RotateCcw, ShieldCheck, TriangleAlert} from 'lucide-react';
import {CheckForUpdates, CurrentUpdateInfo, InstallUpdate, RestartApplication} from '../wailsjs/go/main/App';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';
import {useI18n} from './i18n';

type UpdateInfo = {
    currentVersion: string;
    latestVersion: string;
    releaseName: string;
    releaseNotes: string;
    publishedAt: string;
    releaseUrl: string;
    available: boolean;
    canAutoUpdate: boolean;
    statusMessage: string;
};

type UpdatePhase = 'idle' | 'checking' | 'available' | 'current' | 'installing' | 'ready' | 'error';

const hasWailsBridge = () => Boolean((window as Window & {go?: unknown}).go);
const buildVersion = import.meta.env.VITE_APP_VERSION || '0.1.0';

const versionLabel = (version: string, unknown: string) => {
    if (!version) return unknown;
    return /^v?\d+\.\d+\.\d+(?:[-+].*)?$/i.test(version) ? `v${version.replace(/^v/i, '')}` : version;
};

const repositoryUrl = 'https://github.com/yearnfar/keynest';
const releasesUrl = `${repositoryUrl}/releases`;

function UpdateSettings() {
    const {t} = useI18n();
    const [info, setInfo] = useState<UpdateInfo>({
        currentVersion: buildVersion, latestVersion: '', releaseName: '', releaseNotes: '', publishedAt: '',
        releaseUrl: '', available: false, canAutoUpdate: false, statusMessage: '',
    });
    const [phase, setPhase] = useState<UpdatePhase>('idle');
    const [message, setMessage] = useState('');

    useEffect(() => {
        if (!hasWailsBridge()) {
            setMessage(t('update.preview'));
            return;
        }
        CurrentUpdateInfo()
            .then((result) => {
                const next = result as UpdateInfo;
                setInfo(next);
                setMessage(next.statusMessage);
            })
            .catch((reason) => setMessage(String(reason).replace(/^Error:\s*/, '')));
    }, [t]);

    const check = async () => {
        if (!hasWailsBridge()) return;
        setPhase('checking');
        setMessage(t('update.connecting'));
        try {
            const next = await CheckForUpdates() as UpdateInfo;
            setInfo(next);
            setPhase(next.available ? 'available' : 'current');
            setMessage(next.statusMessage);
        } catch (reason) {
            setPhase('error');
            setMessage(String(reason).replace(/^Error:\s*/, ''));
        }
    };

    const install = async () => {
        setPhase('installing');
        setMessage(t('update.downloading'));
        try {
            const next = await InstallUpdate() as UpdateInfo;
            setInfo(next);
            setPhase('ready');
            setMessage(next.statusMessage);
        } catch (reason) {
            setPhase('error');
            setMessage(String(reason).replace(/^Error:\s*/, ''));
        }
    };

    const openUrl = (url: string) => {
        if (hasWailsBridge()) void BrowserOpenURL(url);
        else window.open(url, '_blank', 'noopener,noreferrer');
    };

    const busy = phase === 'checking' || phase === 'installing';
    const primaryLabel = phase === 'ready'
        ? t('update.restart')
        : info.available
            ? info.canAutoUpdate
                ? phase === 'installing' ? t('update.installing') : t('update.updateTo', {version: versionLabel(info.latestVersion, t('common.unknown'))})
                : t('update.viewVersion', {version: versionLabel(info.latestVersion, t('common.unknown'))})
            : phase === 'checking' ? t('update.checking') : t('update.check');

    const primaryIcon = phase === 'ready'
        ? <RotateCcw size={16}/>
        : info.available && info.canAutoUpdate
            ? <Download size={16}/>
            : info.available
                ? <ExternalLink size={16}/>
                : <RefreshCw size={16} className={phase === 'checking' ? 'spinning' : ''}/>;

    const primaryAction = () => {
        if (phase === 'ready') {
            void RestartApplication();
        } else if (info.available && info.canAutoUpdate) {
            void install();
        } else if (info.available && info.releaseUrl) {
            openUrl(info.releaseUrl);
        } else {
            void check();
        }
    };

    const statusText = message || (info.available
        ? t('update.newVersion', {version: versionLabel(info.latestVersion, t('common.unknown'))})
        : t('update.currentVersion', {version: versionLabel(info.currentVersion, t('common.unknown'))}));

    const statusIcon = phase === 'error'
        ? <TriangleAlert size={15}/>
        : phase === 'current' || phase === 'ready'
            ? <CheckCircle2 size={15}/>
            : <RefreshCw size={15} className={phase === 'checking' || phase === 'installing' ? 'spinning' : ''}/>;

    return <>
        <section className="settings-section about-section">
            <div className="about-panel">
                <div className="about-header">
                    <div className="about-brand">
                        <span className="about-logo" aria-hidden="true"><ShieldCheck size={27}/></span>
                        <div>
                            <strong>KeyNest</strong>
                            <span>{t('update.version', {version: versionLabel(info.currentVersion, t('common.unknown'))})}</span>
                        </div>
                    </div>
                    <button type="button" className="primary-button about-primary-action" onClick={primaryAction} disabled={!hasWailsBridge() || busy}>
                        {primaryIcon}<span>{primaryLabel}</span>
                    </button>
                </div>

                <div className={`about-status ${phase === 'error' ? 'error' : ''}`} role="status">
                    {statusIcon}<span>{statusText}</span>
                </div>

                <div className="about-links">
                    <button type="button" className="secondary-button" onClick={() => openUrl(repositoryUrl)}><Code2 size={16}/> GitHub</button>
                    <button type="button" className="secondary-button" onClick={() => openUrl(info.releaseUrl || releasesUrl)}><ExternalLink size={16}/> {t('update.changelog')}</button>
                </div>
            </div>
        </section>
    </>;
}

export default UpdateSettings;
