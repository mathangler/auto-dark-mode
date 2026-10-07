export namespace main {
	
	export class City {
	    name: string;
	    nameZh: string;
	    lat: number;
	    lon: number;
	
	    static createFrom(source: any = {}) {
	        return new City(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.nameZh = source["nameZh"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	    }
	}
	export class Config {
	    enabled: boolean;
	    mode: string;
	    darkStart?: string;
	    lightStart?: string;
	    locationSource: string;
	    lat?: number;
	    lon?: number;
	    city?: string;
	    autoStart: boolean;
	    manualTheme: string;
	    lang: string;
	    closeToTray: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.mode = source["mode"];
	        this.darkStart = source["darkStart"];
	        this.lightStart = source["lightStart"];
	        this.locationSource = source["locationSource"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.city = source["city"];
	        this.autoStart = source["autoStart"];
	        this.manualTheme = source["manualTheme"];
	        this.lang = source["lang"];
	        this.closeToTray = source["closeToTray"];
	    }
	}
	export class LocationResult {
	    lat: number;
	    lon: number;
	    source: string;
	    name: string;
	    nameZh?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new LocationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.source = source["source"];
	        this.name = source["name"];
	        this.nameZh = source["nameZh"];
	        this.error = source["error"];
	    }
	}
	export class State {
	    enabled: boolean;
	    mode: string;
	    currentTheme: string;
	    desiredTheme: string;
	    manualTheme: string;
	    nextTransition: string;
	    location: LocationResult;
	    sunrise?: string;
	    sunset?: string;
	    darkStart?: string;
	    lightStart?: string;
	    autoStart: boolean;
	    closeToTray: boolean;
	    lang: string;
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.mode = source["mode"];
	        this.currentTheme = source["currentTheme"];
	        this.desiredTheme = source["desiredTheme"];
	        this.manualTheme = source["manualTheme"];
	        this.nextTransition = source["nextTransition"];
	        this.location = this.convertValues(source["location"], LocationResult);
	        this.sunrise = source["sunrise"];
	        this.sunset = source["sunset"];
	        this.darkStart = source["darkStart"];
	        this.lightStart = source["lightStart"];
	        this.autoStart = source["autoStart"];
	        this.closeToTray = source["closeToTray"];
	        this.lang = source["lang"];
	        this.lastError = source["lastError"];
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

