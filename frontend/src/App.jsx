import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import {
    GetInstalledGames,
    GetRunningProtonProcesses,
    GetCompanions,
    AddCompanion,
    RemoveCompanion,
    SelectCompanionExe,
    GetMonitors,
    GetCoverImage,
    SetFavorite,
    GetSettings,
    SetSettings,
    LaunchCompanions,
    CheckForUpdate,
    ApplyUpdate,
    RestartApp,
    OpenReleaseURL,
} from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { I18nContext, LANGUAGES, makeT, resolveLanguage, useT } from './i18n';
import './App.css';

const POLL_INTERVAL = 2500;

function applyScaleCorrection(monitorScale) {
    if (monitorScale > 1) {
        document.documentElement.style.zoom = String(monitorScale);
    } else {
        document.documentElement.style.zoom = '';
    }
}

export default function App() {
    const [games, setGames] = useState([]);
    const [runningIds, setRunningIds] = useState(new Set());
    const [runningProcs, setRunningProcs] = useState({});
    const [companions, setCompanions] = useState({}); // appId → [{exePath, name}]
    const [filter, setFilter] = useState('');
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);
    const [companionModal, setCompanionModal] = useState(null); // { game }
    const [currentMonitor, setCurrentMonitor] = useState(null);
    const [settings, setSettings] = useState({ autoLaunchCompanions: true, autoDetectGames: true, startOnBoot: false, language: '' });
    const [showSettings, setShowSettings] = useState(false);
    const [update, setUpdate] = useState(null); // UpdateStatus dal backend
    const intervalRef = useRef(null);
    const lang = resolveLanguage(settings.language);
    const t = useMemo(() => makeT(lang), [lang]);

    useEffect(() => { document.documentElement.lang = lang; }, [lang]);

    const refreshCompanions = useCallback(() => {
        GetCompanions().then(c => setCompanions(c || {})).catch(() => {});
    }, []);

    const refreshGames = useCallback(() => {
        GetInstalledGames()
            .then(g => { setGames(g || []); setLoading(false); })
            .catch(e => { setError(String(e)); setLoading(false); });
    }, []);

    const toggleFavorite = useCallback((appId, favorite) => {
        SetFavorite(appId, favorite).then(refreshGames).catch(() => {});
    }, [refreshGames]);

    const launchCompanions = useCallback((appId) => {
        return LaunchCompanions(appId).catch(() => 0);
    }, []);

    const saveSettings = useCallback((next) => {
        setSettings(next);
        SetSettings(next).catch(() => {});
    }, []);

    const checkUpdate = useCallback(() => {
        return CheckForUpdate().then(u => { setUpdate(u); return u; }).catch(() => null);
    }, []);

    useEffect(() => {
        refreshGames();
        refreshCompanions();
        GetSettings().then(s => { if (s) setSettings(s); }).catch(() => {});
        checkUpdate();
        GetMonitors().then(monitors => {
            const primary = monitors?.find(m => m.primary) || monitors?.[0];
            if (primary) setCurrentMonitor(primary);
        }).catch(() => {});
    }, [refreshGames, refreshCompanions, checkUpdate]);

    // Ascolta i cambi di monitor e applica correzione zoom se necessario
    useEffect(() => {
        const unsub = EventsOn('display:scale', (monitor) => {
            setCurrentMonitor(monitor);
            applyScaleCorrection(monitor.scale);
        });
        return unsub;
    }, []);

    useEffect(() => {
        if (!settings.autoDetectGames) {
            setRunningIds(new Set());
            setRunningProcs({});
            return;
        }
        const poll = () => {
            GetRunningProtonProcesses()
                .then(procs => {
                    const ids = new Set();
                    const byApp = {};
                    for (const p of (procs || [])) {
                        ids.add(p.appId);
                        if (!byApp[p.appId] || (p.isGameExe && !byApp[p.appId].isGameExe))
                            byApp[p.appId] = p;
                    }
                    setRunningIds(ids);
                    setRunningProcs(byApp);
                })
                .catch(() => {});
        };
        poll();
        intervalRef.current = setInterval(poll, POLL_INTERVAL);
        return () => clearInterval(intervalRef.current);
    }, [settings.autoDetectGames]);

    const filtered = games.filter(g =>
        g.name.toLowerCase().includes(filter.toLowerCase()) ||
        g.appId.includes(filter)
    );

    const runningCount = games.filter(g => runningIds.has(g.appId)).length;
    const favs = filtered.filter(g => g.favorite);
    const rest = filtered.filter(g => !g.favorite);

    return (
        <I18nContext.Provider value={t}>
        <div className="app">
            <header className="header">
                <span className="header-title">Protonaut</span>
                <div className="header-stats">
                    <span className="stat">{t('header.games', { count: games.length })}</span>
                    {currentMonitor && (
                        <span className="stat stat-monitor" title={`Monitor: ${currentMonitor.name}`}>
                            {currentMonitor.name}{currentMonitor.scale > 1 ? `: Zoom ${currentMonitor.scale}x` : ''}
                        </span>
                    )}
                    {runningCount > 0 && (
                        <span className="stat stat-running">
                            <span className="pulse-dot" />
                            {t('header.running', { count: runningCount })}
                        </span>
                    )}
                    {update?.updateAvailable && !showSettings && (
                        <button
                            className="update-badge"
                            onClick={() => setShowSettings(true)}
                            title={t('header.updateTitle', { version: update.latestVersion })}
                        >
                            <span className="pulse-dot" />
                            {t('header.update', { version: update.latestVersion })}
                        </button>
                    )}
                    <button
                        className={`settings-gear${showSettings ? ' settings-gear--active' : ''}${update?.updateAvailable ? ' settings-gear--dot' : ''}`}
                        onClick={() => setShowSettings(s => !s)}
                        title={t('header.settings')}
                        aria-label={t('header.settings')}
                    >⚙</button>
                </div>
            </header>

            {showSettings ? (
                <SettingsPage settings={settings} onChange={saveSettings}
                    update={update} onCheckUpdate={checkUpdate} />
            ) : (
            <>
            <div className="search-bar">
                <input
                    className="search-input"
                    type="text"
                    placeholder={t('library.search')}
                    value={filter}
                    onChange={e => setFilter(e.target.value)}
                    autoComplete="off"
                />
            </div>

            <div className="game-grid-wrapper">
                {loading && <div className="state-msg">{t('library.loading')}</div>}
                {error && <div className="state-msg error">{error}</div>}
                {!loading && !error && filtered.length === 0 && (
                    <div className="state-msg">{t('library.empty')}</div>
                )}

                {favs.length > 0 && (
                    <>
                        <div className="section-label">{t('library.favorites')}</div>
                        <div className="game-grid">
                            {favs.map(g => (
                                <GameCard key={g.appId} game={g}
                                    runningIds={runningIds} runningProcs={runningProcs}
                                    companions={companions[g.appId] || []}
                                    onOpenModal={() => setCompanionModal({ game: g })}
                                    onRemove={(exe) => RemoveCompanion(g.appId, exe).then(refreshCompanions)}
                                    onToggleFavorite={toggleFavorite}
                                    onLaunch={launchCompanions}
                                />
                            ))}
                        </div>
                    </>
                )}

                {rest.length > 0 && (
                    <>
                        {favs.length > 0 && <div className="section-label">{t('library.all')}</div>}
                        <div className="game-grid">
                            {rest.map(g => (
                                <GameCard key={g.appId} game={g}
                                    runningIds={runningIds} runningProcs={runningProcs}
                                    companions={companions[g.appId] || []}
                                    onOpenModal={() => setCompanionModal({ game: g })}
                                    onRemove={(exe) => RemoveCompanion(g.appId, exe).then(refreshCompanions)}
                                    onToggleFavorite={toggleFavorite}
                                    onLaunch={launchCompanions}
                                />
                            ))}
                        </div>
                    </>
                )}
            </div>
            </>
            )}

            {companionModal && (
                <CompanionModal
                    game={companionModal.game}
                    companions={companions[companionModal.game.appId] || []}
                    onClose={() => setCompanionModal(null)}
                    onAdded={refreshCompanions}
                    onRemoved={refreshCompanions}
                />
            )}
        </div>
        </I18nContext.Provider>
    );
}

function GameCard({ game, runningIds, runningProcs, companions, onOpenModal, onRemove, onToggleFavorite, onLaunch }) {
    const t = useT();
    const isRunning = runningIds.has(game.appId);
    const proc = runningProcs[game.appId];
    const [imgSrc, setImgSrc] = useState(null);
    const [launchFlash, setLaunchFlash] = useState(null); // null | 'ok' | 'empty'
    const flashRef = useRef(null);

    useEffect(() => {
        let cancelled = false;
        setImgSrc(null);
        GetCoverImage(game.appId)
            .then(dataUri => { if (!cancelled) setImgSrc(dataUri); })
            .catch(() => {});
        return () => { cancelled = true; };
    }, [game.appId]);

    useEffect(() => () => clearTimeout(flashRef.current), []);

    const hasCompanions = companions.length > 0;

    const handleLaunch = () => {
        if (!hasCompanions) {
            onOpenModal();
            return;
        }
        Promise.resolve(onLaunch(game.appId)).then(started => {
            setLaunchFlash(started > 0 ? 'ok' : 'busy');
            clearTimeout(flashRef.current);
            flashRef.current = setTimeout(() => setLaunchFlash(null), 1600);
        });
    };

    const flashText = launchFlash === 'ok' ? t('card.launched')
        : launchFlash === 'busy' ? t('card.alreadyRunning') : '';

    return (
        <div
            className={`game-card${isRunning ? ' game-card--running' : ''}${hasCompanions ? ' game-card--launchable' : ''}`}
            onClick={handleLaunch}
            title={hasCompanions ? t('card.launchTitle') : t('card.noCompanionTitle')}
        >
            {launchFlash && <div className="card-launch-flash">{flashText}</div>}
            <div className="game-card__cover">
                {imgSrc ? (
                    <img
                        className="game-cover-img"
                        src={imgSrc}
                        alt=""
                        loading="lazy"
                    />
                ) : (
                    <div className="game-cover-placeholder">{game.name}</div>
                )}
                <button
                    className={`card-fav-star${game.favorite ? ' card-fav-star--active' : ''}`}
                    onClick={(e) => { e.stopPropagation(); onToggleFavorite(game.appId, !game.favorite); }}
                    title={game.favorite ? t('card.removeFavorite') : t('card.addFavorite')}
                    aria-label={game.favorite ? t('card.removeFavorite') : t('card.addFavorite')}
                />
                {isRunning && <span className="card-running-dot"><span className="pulse-dot" /></span>}
            </div>

            <div className="game-card__body">
                <div className="card-name">{game.name}</div>
                <div className="card-meta">
                    <span className="card-appid">{game.appId}</span>
                    {isRunning && proc && <span className="running-badge">PID {proc.pid}</span>}
                    {isRunning && <span className="game-status">{t('card.running')}</span>}
                </div>
            </div>

            <div className="game-card__footer">
                {companions.map(c => (
                    <span key={c.exePath} className={`companion-chip${isRunning ? ' companion-chip--active' : ''}`}>
                        {c.name}
                        <button
                            className="companion-chip-remove"
                            title={t('card.remove')}
                            onClick={(e) => { e.stopPropagation(); onRemove(c.exePath); }}
                        >×</button>
                    </span>
                ))}
                <button className="add-companion-btn" onClick={(e) => { e.stopPropagation(); onOpenModal(); }}>+ Companion</button>
            </div>
        </div>
    );
}

function SettingsPage({ settings, onChange, update, onCheckUpdate }) {
    const t = useT();
    const toggle = (key) => onChange({ ...settings, [key]: !settings[key] });

    return (
        <div className="settings-page">
            <h2 className="settings-title">{t('settings.title')}</h2>

            <UpdateSection update={update} onCheckUpdate={onCheckUpdate} />

            <div className="settings-list">
                <div className="setting-row setting-row--static">
                    <div className="setting-text">
                        <div className="setting-label">{t('settings.language')}</div>
                        <div className="setting-hint">{t('settings.languageHint')}</div>
                    </div>
                    <select
                        className="setting-select"
                        value={settings.language || ''}
                        onChange={e => onChange({ ...settings, language: e.target.value })}
                        aria-label={t('settings.language')}
                    >
                        <option value="">{t('settings.languageAuto')}</option>
                        {LANGUAGES.map(l => <option key={l.code} value={l.code}>{l.label}</option>)}
                    </select>
                </div>
                <SettingRow
                    label={t('settings.autoDetect')}
                    hint={t('settings.autoDetectHint')}
                    checked={settings.autoDetectGames}
                    onToggle={() => toggle('autoDetectGames')}
                />
                <SettingRow
                    label={t('settings.autoLaunch')}
                    hint={t('settings.autoLaunchHint')}
                    checked={settings.autoLaunchCompanions}
                    onToggle={() => toggle('autoLaunchCompanions')}
                />
                <SettingRow
                    label={t('settings.startOnBoot')}
                    hint={t('settings.startOnBootHint')}
                    checked={settings.startOnBoot}
                    onToggle={() => toggle('startOnBoot')}
                />
            </div>

            <p className="settings-note">{t('settings.note')}</p>
        </div>
    );
}

function UpdateSection({ update, onCheckUpdate }) {
    const t = useT();
    const [checking, setChecking] = useState(false);
    const [phase, setPhase] = useState('idle'); // idle | updating | done | error
    const [message, setMessage] = useState('');

    const recheck = () => {
        setChecking(true);
        Promise.resolve(onCheckUpdate()).finally(() => setChecking(false));
    };

    const doUpdate = () => {
        setPhase('updating');
        setMessage(t('update.applying'));
        ApplyUpdate()
            .then(() => { setPhase('done'); setMessage(t('update.applied')); })
            .catch((e) => { setPhase('error'); setMessage(String(e)); });
    };

    const cur = update?.currentVersion || '—';
    const available = update?.updateAvailable;

    return (
        <div className={`update-card${available ? ' update-card--available' : ''}`}>
            <div className="update-card-main">
                <div className="update-card-text">
                    <div className="update-card-title">
                        {available
                            ? t('update.available', { version: update.latestVersion })
                            : t('update.upToDate')}
                    </div>
                    <div className="update-card-sub">
                        {t('update.current')} <code>{cur}</code>
                        {available && update?.releaseUrl && (
                            <>  ·  <a className="update-link"
                                onClick={() => OpenReleaseURL(update.releaseUrl)}>{t('update.whatsNew')}</a></>
                        )}
                    </div>
                </div>

                <div className="update-card-actions">
                    <button className="update-btn update-btn--ghost"
                        onClick={recheck} disabled={checking || phase === 'updating'}>
                        {checking ? t('update.checking') : t('update.check')}
                    </button>

                    {available && phase !== 'done' && (
                        update?.canSelfUpdate ? (
                            <button className="update-btn update-btn--primary"
                                onClick={doUpdate} disabled={phase === 'updating'}>
                                {phase === 'updating' ? t('update.updating') : t('update.updateNow')}
                            </button>
                        ) : (
                            <button className="update-btn update-btn--primary"
                                onClick={() => update?.releaseUrl && OpenReleaseURL(update.releaseUrl)}>
                                {t('update.download')}
                            </button>
                        )
                    )}

                    {phase === 'done' && (
                        <button className="update-btn update-btn--primary" onClick={() => RestartApp()}>
                            {t('update.restartNow')}
                        </button>
                    )}
                </div>
            </div>

            {message && <div className={`update-msg update-msg--${phase}`}>{message}</div>}

            {available && !update?.canSelfUpdate && update?.isAppImage && (
                <div className="update-note">
                    {t('update.appImageNote').split('{tool}').flatMap((part, i) =>
                        i === 0 ? [part] : [<code key={i}>AppImageUpdate</code>, part])}
                </div>
            )}
            {available && !update?.isAppImage && (
                <div className="update-note">
                    {t('update.packageNote')}
                </div>
            )}
        </div>
    );
}

function SettingRow({ label, hint, checked, onToggle }) {
    return (
        <div className="setting-row" onClick={onToggle}>
            <div className="setting-text">
                <div className="setting-label">{label}</div>
                <div className="setting-hint">{hint}</div>
            </div>
            <button
                className={`setting-switch${checked ? ' setting-switch--on' : ''}`}
                role="switch"
                aria-checked={checked}
                aria-label={label}
                onClick={(e) => { e.stopPropagation(); onToggle(); }}
            >
                <span className="setting-switch-knob" />
            </button>
        </div>
    );
}

function CompanionModal({ game, companions, onClose, onAdded, onRemoved }) {
    const t = useT();
    const [browsing, setBrowsing] = useState(false);

    const browse = () => {
        setBrowsing(true);
        SelectCompanionExe()
            .then(path => {
                if (!path) return;
                return AddCompanion(game.appId, path).then(onAdded);
            })
            .catch(() => {})
            .finally(() => setBrowsing(false));
    };

    const remove = (exePath) => {
        RemoveCompanion(game.appId, exePath).then(onRemoved);
    };

    return (
        <div className="modal-backdrop" onClick={e => e.target === e.currentTarget && onClose()}>
            <div className="modal">
                <div className="modal-header">
                    <span className="modal-title">{t('modal.title', { name: game.name })}</span>
                    <button className="modal-close" onClick={onClose}>✕</button>
                </div>

                <div className="modal-body">
                    <div className="modal-game-label">
                        <span className="game-name">{game.name}</span>
                        <span className="game-sep">:</span>
                        <span className="game-appid">{game.appId}</span>
                    </div>

                    {/* Lista companion già configurati */}
                    {companions.length > 0 && (
                        <div className="modal-companion-list">
                            {companions.map(c => (
                                <div key={c.exePath} className="modal-companion-row">
                                    <span className="modal-companion-name">{c.name}</span>
                                    <span className="modal-companion-path">{c.exePath}</span>
                                    <button
                                        className="modal-remove-btn"
                                        onClick={() => remove(c.exePath)}
                                    >{t('modal.remove')}</button>
                                </div>
                            ))}
                        </div>
                    )}

                    <p className="modal-hint">
                        {t('modal.hint')}<br />
                        <code>protontricks-launch --no-bwrap --appid {game.appId} &lt;exe&gt;</code>
                    </p>
                </div>

                <div className="modal-footer">
                    <button className="modal-btn modal-btn--cancel" onClick={onClose}>{t('modal.close')}</button>
                    <button className="modal-btn modal-btn--browse" onClick={browse} disabled={browsing}>
                        {browsing ? t('modal.selecting') : t('modal.addExe')}
                    </button>
                </div>
            </div>
        </div>
    );
}
