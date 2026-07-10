import { useState, useEffect, useRef, useCallback } from 'react';
import {
    GetInstalledGames,
    GetRunningProtonProcesses,
    GetCompanions,
    AddCompanion,
    RemoveCompanion,
    SelectCompanionExe,
    GetMonitors,
} from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
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
    const intervalRef = useRef(null);

    const refreshCompanions = useCallback(() => {
        GetCompanions().then(c => setCompanions(c || {})).catch(() => {});
    }, []);

    useEffect(() => {
        GetInstalledGames()
            .then(g => { setGames(g || []); setLoading(false); })
            .catch(e => { setError(String(e)); setLoading(false); });
        refreshCompanions();
        GetMonitors().then(monitors => {
            const primary = monitors?.find(m => m.primary) || monitors?.[0];
            if (primary) setCurrentMonitor(primary);
        }).catch(() => {});
    }, [refreshCompanions]);

    // Ascolta i cambi di monitor e applica correzione zoom se necessario
    useEffect(() => {
        const unsub = EventsOn('display:scale', (monitor) => {
            setCurrentMonitor(monitor);
            applyScaleCorrection(monitor.scale);
        });
        return unsub;
    }, []);

    useEffect(() => {
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
    }, []);

    const filtered = games.filter(g =>
        g.name.toLowerCase().includes(filter.toLowerCase()) ||
        g.appId.includes(filter)
    );

    const runningCount = games.filter(g => runningIds.has(g.appId)).length;
    const favs = filtered.filter(g => g.favorite);
    const rest = filtered.filter(g => !g.favorite);

    return (
        <div className="app">
            <header className="header">
                <span className="header-title">Protonaut</span>
                <div className="header-stats">
                    <span className="stat">{games.length} giochi</span>
                    {currentMonitor && (
                        <span className="stat stat-monitor" title={`Monitor: ${currentMonitor.name}`}>
                            {currentMonitor.name}{currentMonitor.scale > 1 ? `: Zoom ${currentMonitor.scale}x` : ''}
                        </span>
                    )}
                    {runningCount > 0 && (
                        <span className="stat stat-running">
                            <span className="pulse-dot" />
                            {runningCount} in esecuzione
                        </span>
                    )}
                </div>
            </header>

            <div className="search-bar">
                <input
                    className="search-input"
                    type="text"
                    placeholder="Cerca nome o AppID..."
                    value={filter}
                    onChange={e => setFilter(e.target.value)}
                    autoComplete="off"
                />
            </div>

            <div className="game-grid-wrapper">
                {loading && <div className="state-msg">Caricamento librerie...</div>}
                {error && <div className="state-msg error">{error}</div>}
                {!loading && !error && filtered.length === 0 && (
                    <div className="state-msg">Nessun gioco trovato.</div>
                )}

                {favs.length > 0 && (
                    <>
                        <div className="section-label">★ Preferiti</div>
                        <div className="game-grid">
                            {favs.map(g => (
                                <GameCard key={g.appId} game={g}
                                    runningIds={runningIds} runningProcs={runningProcs}
                                    companions={companions[g.appId] || []}
                                    onOpenModal={() => setCompanionModal({ game: g })}
                                    onRemove={(exe) => RemoveCompanion(g.appId, exe).then(refreshCompanions)}
                                />
                            ))}
                        </div>
                    </>
                )}

                {rest.length > 0 && (
                    <>
                        {favs.length > 0 && <div className="section-label">Tutti i giochi</div>}
                        <div className="game-grid">
                            {rest.map(g => (
                                <GameCard key={g.appId} game={g}
                                    runningIds={runningIds} runningProcs={runningProcs}
                                    companions={companions[g.appId] || []}
                                    onOpenModal={() => setCompanionModal({ game: g })}
                                    onRemove={(exe) => RemoveCompanion(g.appId, exe).then(refreshCompanions)}
                                />
                            ))}
                        </div>
                    </>
                )}
            </div>

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
    );
}

const IMG_FALLBACKS = (appId) => [
    `https://cdn.akamai.steamstatic.com/steam/apps/${appId}/header.jpg`,
    `https://cdn.akamai.steamstatic.com/steam/apps/${appId}/capsule_231x87.jpg`,
    `https://cdn.akamai.steamstatic.com/steam/apps/${appId}/capsule_sm_120.jpg`,
];

function GameCard({ game, runningIds, runningProcs, companions, onOpenModal, onRemove }) {
    const isRunning = runningIds.has(game.appId);
    const proc = runningProcs[game.appId];
    const [imgIdx, setImgIdx] = useState(0);
    const fallbacks = IMG_FALLBACKS(game.appId);
    const imgUrl = fallbacks[imgIdx];

    return (
        <div className={`game-card${isRunning ? ' game-card--running' : ''}`}>
            <div className="game-card__cover">
                {imgUrl ? (
                    <img
                        className="game-cover-img"
                        src={imgUrl}
                        alt=""
                        loading="lazy"
                        onError={() => setImgIdx(i => i + 1 < fallbacks.length ? i + 1 : fallbacks.length)}
                    />
                ) : (
                    <div className="game-cover-placeholder">{game.name}</div>
                )}
                {game.favorite && <span className="card-fav-star">★</span>}
                {isRunning && <span className="card-running-dot"><span className="pulse-dot" /></span>}
            </div>

            <div className="game-card__body">
                <div className="card-name">{game.name}</div>
                <div className="card-meta">
                    <span className="card-appid">{game.appId}</span>
                    {isRunning && proc && <span className="running-badge">PID {proc.pid}</span>}
                    {isRunning && <span className="game-status">IN ESECUZIONE</span>}
                </div>
            </div>

            <div className="game-card__footer">
                {companions.map(c => (
                    <span key={c.exePath} className={`companion-chip${isRunning ? ' companion-chip--active' : ''}`}>
                        {c.name}
                        <button
                            className="companion-chip-remove"
                            title="Rimuovi"
                            onClick={() => onRemove(c.exePath)}
                        >×</button>
                    </span>
                ))}
                <button className="add-companion-btn" onClick={onOpenModal}>+ Companion</button>
            </div>
        </div>
    );
}

function CompanionModal({ game, companions, onClose, onAdded, onRemoved }) {
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
                    <span className="modal-title">Companion — {game.name}</span>
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
                                    >Rimuovi</button>
                                </div>
                            ))}
                        </div>
                    )}

                    <p className="modal-hint">
                        Verranno lanciati automaticamente all'avvio del gioco:<br />
                        <code>protontricks-launch --no-bwrap --appid {game.appId} &lt;exe&gt;</code>
                    </p>
                </div>

                <div className="modal-footer">
                    <button className="modal-btn modal-btn--cancel" onClick={onClose}>Chiudi</button>
                    <button className="modal-btn modal-btn--browse" onClick={browse} disabled={browsing}>
                        {browsing ? 'Seleziona...' : '+ Aggiungi exe'}
                    </button>
                </div>
            </div>
        </div>
    );
}
