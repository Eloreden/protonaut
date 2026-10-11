import { createContext, useContext } from 'react';

export const LANGUAGES = [
    { code: 'en', label: 'English' },
    { code: 'it', label: 'Italiano' },
];

const translations = {
    en: {
        'header.games': '{count} games',
        'header.running': '{count} running',
        'header.updateTitle': 'New version {version} available',
        'header.update': 'Update {version}',
        'header.settings': 'Settings',

        'library.search': 'Search name or AppID...',
        'library.loading': 'Loading libraries...',
        'library.empty': 'No games found.',
        'library.favorites': '★ Favorites',
        'library.all': 'All games',

        'card.launched': '▶ Companions launched',
        'card.alreadyRunning': '● Already running',
        'card.launchTitle': 'Click to launch companions',
        'card.noCompanionTitle': 'No companion configured — click to add one',
        'card.removeFavorite': 'Remove from favorites',
        'card.addFavorite': 'Add to favorites',
        'card.running': 'RUNNING',
        'card.remove': 'Remove',

        'settings.title': 'Settings',
        'settings.language': 'Language',
        'settings.languageHint': 'Interface language. "Automatic" follows the system language.',
        'settings.languageAuto': 'Automatic',
        'settings.autoDetect': 'Automatic game detection',
        'settings.autoDetectHint': 'Watches running Proton games in the background and highlights their status in the library.',
        'settings.autoLaunch': 'Auto-launch companions',
        'settings.autoLaunchHint': 'Automatically launches the paired companions as soon as the matching game is detected running.',
        'settings.startOnBoot': 'Start on login',
        'settings.startOnBootHint': 'Adds Protonaut to the system autostart, so it starts by itself when you log in.',
        'settings.note': 'Manual launch: click a game tile in the library to launch its companions at any time, even with auto-launch turned off.',

        'update.applying': 'Downloading and applying the update...',
        'update.applied': 'Update applied. Restart to use it.',
        'update.available': 'Update available: {version}',
        'update.upToDate': "You're on the latest version",
        'update.current': 'Current version:',
        'update.whatsNew': "what's new",
        'update.checking': 'Checking...',
        'update.check': 'Check',
        'update.updating': 'Updating...',
        'update.updateNow': 'Update now',
        'update.download': 'Download',
        'update.restartNow': 'Restart now',
        'update.appImageNote': 'Install {tool} (or AppImageLauncher) to update from the app.',
        'update.packageNote': 'Package install: update through your system package manager (e.g. Octopi / paru).',

        'modal.title': 'Companion — {name}',
        'modal.remove': 'Remove',
        'modal.hint': 'They will be launched automatically when the game starts:',
        'modal.close': 'Close',
        'modal.selecting': 'Selecting...',
        'modal.addExe': '+ Add exe',
    },
    it: {
        'header.games': '{count} giochi',
        'header.running': '{count} in esecuzione',
        'header.updateTitle': 'Nuova versione {version} disponibile',
        'header.update': 'Aggiornamento {version}',
        'header.settings': 'Impostazioni',

        'library.search': 'Cerca nome o AppID...',
        'library.loading': 'Caricamento librerie...',
        'library.empty': 'Nessun gioco trovato.',
        'library.favorites': '★ Preferiti',
        'library.all': 'Tutti i giochi',

        'card.launched': '▶ Companion avviati',
        'card.alreadyRunning': '● Già in esecuzione',
        'card.launchTitle': 'Clic per lanciare i companion',
        'card.noCompanionTitle': 'Nessun companion configurato — clic per aggiungerne',
        'card.removeFavorite': 'Rimuovi dai preferiti',
        'card.addFavorite': 'Aggiungi ai preferiti',
        'card.running': 'IN ESECUZIONE',
        'card.remove': 'Rimuovi',

        'settings.title': 'Impostazioni',
        'settings.language': 'Lingua',
        'settings.languageHint': 'Lingua dell\'interfaccia. "Automatica" segue la lingua di sistema.',
        'settings.languageAuto': 'Automatica',
        'settings.autoDetect': 'Rilevamento automatico dei giochi',
        'settings.autoDetectHint': 'Controlla in background i giochi Proton in esecuzione ed evidenzia il loro stato nella libreria.',
        'settings.autoLaunch': 'Avvio automatico dei companion',
        'settings.autoLaunchHint': 'Lancia automaticamente i companion abbinati appena il gioco corrispondente viene rilevato in esecuzione.',
        'settings.startOnBoot': "Avvia all'avvio del PC",
        'settings.startOnBootHint': 'Aggiunge Protonaut agli avvii automatici del sistema, così parte da solo al login.',
        'settings.note': "Avvio manuale: clicca sulla cella di un gioco nella libreria per lanciare i suoi companion in qualsiasi momento, anche con l'avvio automatico disattivato.",

        'update.applying': "Download e applicazione dell'aggiornamento in corso...",
        'update.applied': 'Aggiornamento applicato. Riavvia per usarlo.',
        'update.available': 'Aggiornamento disponibile: {version}',
        'update.upToDate': "Sei all'ultima versione",
        'update.current': 'Versione attuale:',
        'update.whatsNew': 'novità',
        'update.checking': 'Controllo...',
        'update.check': 'Controlla',
        'update.updating': 'Aggiornamento...',
        'update.updateNow': 'Aggiorna ora',
        'update.download': 'Scarica',
        'update.restartNow': 'Riavvia ora',
        'update.appImageNote': "Installa {tool} (o AppImageLauncher) per aggiornare dall'app.",
        'update.packageNote': 'Installazione da pacchetto: aggiorna tramite il gestore pacchetti del sistema (es. Octopi / paru).',

        'modal.title': 'Companion — {name}',
        'modal.remove': 'Rimuovi',
        'modal.hint': "Verranno lanciati automaticamente all'avvio del gioco:",
        'modal.close': 'Chiudi',
        'modal.selecting': 'Seleziona...',
        'modal.addExe': '+ Aggiungi exe',
    },
};

export function resolveLanguage(pref) {
    if (translations[pref]) return pref;
    for (const l of navigator.languages || [navigator.language]) {
        const code = (l || '').toLowerCase().split('-')[0];
        if (translations[code]) return code;
    }
    return 'en';
}

export function makeT(lang) {
    const dict = translations[lang] || translations.en;
    return (key, params) => {
        let s = dict[key] ?? translations.en[key] ?? key;
        if (params) {
            for (const [k, v] of Object.entries(params)) s = s.replaceAll(`{${k}}`, v);
        }
        return s;
    };
}

export const I18nContext = createContext(makeT('en'));

export const useT = () => useContext(I18nContext);
