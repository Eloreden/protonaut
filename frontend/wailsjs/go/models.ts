export namespace backend {
	
	export class MonitorInfo {
	    name: string;
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	    scale: number;
	    primary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MonitorInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.scale = source["scale"];
	        this.primary = source["primary"];
	    }
	}

}

export namespace models {
	
	export class Game {
	    appId: string;
	    name: string;
	    installDir: string;
	    libraryId: string;
	    fullPath: string;
	    favorite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.name = source["name"];
	        this.installDir = source["installDir"];
	        this.libraryId = source["libraryId"];
	        this.fullPath = source["fullPath"];
	        this.favorite = source["favorite"];
	    }
	}
	export class Library {
	    id: string;
	    name: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Library(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	    }
	}
	export class ProtonProcess {
	    pid: number;
	    appId: string;
	    name: string;
	    exe: string;
	    cmdline: string;
	    isGameExe: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProtonProcess(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.appId = source["appId"];
	        this.name = source["name"];
	        this.exe = source["exe"];
	        this.cmdline = source["cmdline"];
	        this.isGameExe = source["isGameExe"];
	    }
	}
	export class SteamStatus {
	    found: boolean;
	    steamPath: string;
	    vdfPath: string;
	    reasons: string[];
	
	    static createFrom(source: any = {}) {
	        return new SteamStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.steamPath = source["steamPath"];
	        this.vdfPath = source["vdfPath"];
	        this.reasons = source["reasons"];
	    }
	}

}

