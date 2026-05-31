export namespace app {
	
	export class Config {
	    base_url: string;
	    api_key: string;
	    provider: string;
	    model: string;
	    port: number;
	    show_reasoning?: boolean;
	    max_tokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.port = source["port"];
	        this.show_reasoning = source["show_reasoning"];
	        this.max_tokens = source["max_tokens"];
	    }
	}
	export class ModelsResult {
	    models: string[];
	    count: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.models = source["models"];
	        this.count = source["count"];
	        this.error = source["error"];
	    }
	}
	export class StatusDTO {
	    ca_installed: boolean;
	    hosts_mapped: boolean;
	    proxy_running: boolean;
	    config_valid: boolean;
	    config_error: string;
	    ca_cert_path: string;
	    config_path: string;
	    upstream: string;
	    provider: string;
	    model: string;
	    listen_address: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ca_installed = source["ca_installed"];
	        this.hosts_mapped = source["hosts_mapped"];
	        this.proxy_running = source["proxy_running"];
	        this.config_valid = source["config_valid"];
	        this.config_error = source["config_error"];
	        this.ca_cert_path = source["ca_cert_path"];
	        this.config_path = source["config_path"];
	        this.upstream = source["upstream"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.listen_address = source["listen_address"];
	    }
	}
	export class TestResult {
	    ok: boolean;
	    duration_ms: number;
	    model_count: number;
	    detail: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.duration_ms = source["duration_ms"];
	        this.model_count = source["model_count"];
	        this.detail = source["detail"];
	        this.error = source["error"];
	    }
	}
	export class UsageDTO {
	    total_requests: number;
	    total_tokens: number;
	    error_count: number;
	    recent: proxy.UsageRecord[];
	
	    static createFrom(source: any = {}) {
	        return new UsageDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_requests = source["total_requests"];
	        this.total_tokens = source["total_tokens"];
	        this.error_count = source["error_count"];
	        this.recent = this.convertValues(source["recent"], proxy.UsageRecord);
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

export namespace proxy {
	
	export class UsageRecord {
	    at: string;
	    model: string;
	    provider: string;
	    prompt_tokens: number;
	    output_tokens: number;
	    total_tokens: number;
	    duration_ms: number;
	    status: string;
	    error_detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new UsageRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = source["at"];
	        this.model = source["model"];
	        this.provider = source["provider"];
	        this.prompt_tokens = source["prompt_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.total_tokens = source["total_tokens"];
	        this.duration_ms = source["duration_ms"];
	        this.status = source["status"];
	        this.error_detail = source["error_detail"];
	    }
	}

}

