export namespace main {
	
	export class CreateDatabaseRequest {
	    name: string;
	    password: string;
	    keyFile: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateDatabaseRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.password = source["password"];
	        this.keyFile = source["keyFile"];
	    }
	}
	export class RecentDatabase {
	    name: string;
	    path: string;
	    modified: string;
	
	    static createFrom(source: any = {}) {
	        return new RecentDatabase(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.modified = source["modified"];
	    }
	}
	export class SaveEntryRequest {
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
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new SaveEntryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.url = source["url"];
	        this.note = source["note"];
	        this.groupId = source["groupId"];
	        this.folder = source["folder"];
	        this.category = source["category"];
	        this.favorite = source["favorite"];
	        this.tags = source["tags"];
	    }
	}
	export class VaultEntry {
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
	    createdAt: string;
	    modifiedAt: string;
	    lastUsedAt: string;
	    color: string;
	    initials: string;
	    tags: string[];
	    trashed: boolean;
	    deletedAt?: string;
	    historyCount: number;
	
	    static createFrom(source: any = {}) {
	        return new VaultEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.url = source["url"];
	        this.note = source["note"];
	        this.groupId = source["groupId"];
	        this.folder = source["folder"];
	        this.category = source["category"];
	        this.favorite = source["favorite"];
	        this.modified = source["modified"];
	        this.createdAt = source["createdAt"];
	        this.modifiedAt = source["modifiedAt"];
	        this.lastUsedAt = source["lastUsedAt"];
	        this.color = source["color"];
	        this.initials = source["initials"];
	        this.tags = source["tags"];
	        this.trashed = source["trashed"];
	        this.deletedAt = source["deletedAt"];
	        this.historyCount = source["historyCount"];
	    }
	}
	export class SaveEntryResult {
	    entry: VaultEntry;
	
	    static createFrom(source: any = {}) {
	        return new SaveEntryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entry = this.convertValues(source["entry"], VaultEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UpdateInfo {
	    currentVersion: string;
	    latestVersion: string;
	    releaseName: string;
	    releaseNotes: string;
	    publishedAt: string;
	    releaseUrl: string;
	    available: boolean;
	    canAutoUpdate: boolean;
	    statusMessage: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.releaseName = source["releaseName"];
	        this.releaseNotes = source["releaseNotes"];
	        this.publishedAt = source["publishedAt"];
	        this.releaseUrl = source["releaseUrl"];
	        this.available = source["available"];
	        this.canAutoUpdate = source["canAutoUpdate"];
	        this.statusMessage = source["statusMessage"];
	    }
	}
	
	export class VaultGroup {
	    id: string;
	    name: string;
	    parentId?: string;
	    depth: number;
	    iconId: number;
	
	    static createFrom(source: any = {}) {
	        return new VaultGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.parentId = source["parentId"];
	        this.depth = source["depth"];
	        this.iconId = source["iconId"];
	    }
	}
	export class VaultSnapshot {
	    name: string;
	    path: string;
	    entries: VaultEntry[];
	    groups: VaultGroup[];
	
	    static createFrom(source: any = {}) {
	        return new VaultSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.entries = this.convertValues(source["entries"], VaultEntry);
	        this.groups = this.convertValues(source["groups"], VaultGroup);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

