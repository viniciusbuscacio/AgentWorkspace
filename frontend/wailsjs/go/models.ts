export namespace domain {
	
	export class TokenUsage {
	    input: number;
	    output: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new TokenUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input = source["input"];
	        this.output = source["output"];
	        this.total = source["total"];
	    }
	}
	export class AgentReply {
	    text: string;
	    provider?: string;
	    model?: string;
	    tokenUsage?: TokenUsage;
	
	    static createFrom(source: any = {}) {
	        return new AgentReply(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.tokenUsage = this.convertValues(source["tokenUsage"], TokenUsage);
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
	export class Attachment {
	    name: string;
	    type: string;
	    dataUri: string;
	
	    static createFrom(source: any = {}) {
	        return new Attachment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.dataUri = source["dataUri"];
	    }
	}
	export class BrowserStatus {
	    id: string;
	    running: boolean;
	    headless?: boolean;
	    port: number;
	    profileDir: string;
	    binary: string;
	    notice?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.running = source["running"];
	        this.headless = source["headless"];
	        this.port = source["port"];
	        this.profileDir = source["profileDir"];
	        this.binary = source["binary"];
	        this.notice = source["notice"];
	    }
	}
	export class BrowserTab {
	    id: string;
	    title: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserTab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	    }
	}
	export class Chat {
	    id: string;
	    title: string;
	    archived: boolean;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Chat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.archived = source["archived"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class EffectiveInstructionsMetadata {
	    generatedAt?: string;
	    charCount: number;
	    devOverrideActive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EffectiveInstructionsMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = source["generatedAt"];
	        this.charCount = source["charCount"];
	        this.devOverrideActive = source["devOverrideActive"];
	    }
	}
	export class InstructionSource {
	    id: string;
	    kind: string;
	    origin?: string;
	    enabled: boolean;
	    active: boolean;
	    chars?: number;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new InstructionSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.origin = source["origin"];
	        this.enabled = source["enabled"];
	        this.active = source["active"];
	        this.chars = source["chars"];
	        this.reason = source["reason"];
	    }
	}
	export class EffectiveInstructions {
	    content: string;
	    sources: InstructionSource[];
	    metadata: EffectiveInstructionsMetadata;
	    redacted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EffectiveInstructions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.sources = this.convertValues(source["sources"], InstructionSource);
	        this.metadata = this.convertValues(source["metadata"], EffectiveInstructionsMetadata);
	        this.redacted = source["redacted"];
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
	
	export class FirewallRule {
	    action: string;
	    service: string;
	    interface: string;
	    origin: string;
	
	    static createFrom(source: any = {}) {
	        return new FirewallRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.service = source["service"];
	        this.interface = source["interface"];
	        this.origin = source["origin"];
	    }
	}
	export class InstructionDocument {
	    id: string;
	    title: string;
	    description: string;
	    origin: string;
	    status: string[];
	    editable: boolean;
	    resettable: boolean;
	    enabled: boolean;
	    contentHash?: string;
	    seedHash?: string;
	    seedVersion?: string;
	    updatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new InstructionDocument(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.origin = source["origin"];
	        this.status = source["status"];
	        this.editable = source["editable"];
	        this.resettable = source["resettable"];
	        this.enabled = source["enabled"];
	        this.contentHash = source["contentHash"];
	        this.seedHash = source["seedHash"];
	        this.seedVersion = source["seedVersion"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class InstructionDocumentContent {
	    id: string;
	    content: string;
	    origin: string;
	    editable: boolean;
	    resettable: boolean;
	    status: string[];
	    contentHash?: string;
	    seedHash?: string;
	    seedVersion?: string;
	    updatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new InstructionDocumentContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.content = source["content"];
	        this.origin = source["origin"];
	        this.editable = source["editable"];
	        this.resettable = source["resettable"];
	        this.status = source["status"];
	        this.contentHash = source["contentHash"];
	        this.seedHash = source["seedHash"];
	        this.seedVersion = source["seedVersion"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	
	export class LLMTurn {
	    id: string;
	    sessionId: string;
	    turnIndex: number;
	    requestJson: string;
	    responseText?: string;
	    toolCallsJson?: string;
	    model?: string;
	    promptTokens?: number;
	    completionTokens?: number;
	    finishReason?: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new LLMTurn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.turnIndex = source["turnIndex"];
	        this.requestJson = source["requestJson"];
	        this.responseText = source["responseText"];
	        this.toolCallsJson = source["toolCallsJson"];
	        this.model = source["model"];
	        this.promptTokens = source["promptTokens"];
	        this.completionTokens = source["completionTokens"];
	        this.finishReason = source["finishReason"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class LogDateCount {
	    date: string;
	    entries: number;
	
	    static createFrom(source: any = {}) {
	        return new LogDateCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.entries = source["entries"];
	    }
	}
	export class LogEntry {
	    id: string;
	    timestamp: string;
	    level: number;
	    levelName: string;
	    source: string;
	    moduleId?: string;
	    moduleType?: string;
	    sessionId?: string;
	    message: string;
	    contextJson?: string;
	    errorName?: string;
	    errorMessage?: string;
	    errorStack?: string;
	    createdAt: string;
	    event?: string;
	    severity?: string;
	    traceId?: string;
	    spanId?: string;
	    parentSpanId?: string;
	    durationMs?: number;
	    status?: string;
	    attributesJson?: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.level = source["level"];
	        this.levelName = source["levelName"];
	        this.source = source["source"];
	        this.moduleId = source["moduleId"];
	        this.moduleType = source["moduleType"];
	        this.sessionId = source["sessionId"];
	        this.message = source["message"];
	        this.contextJson = source["contextJson"];
	        this.errorName = source["errorName"];
	        this.errorMessage = source["errorMessage"];
	        this.errorStack = source["errorStack"];
	        this.createdAt = source["createdAt"];
	        this.event = source["event"];
	        this.severity = source["severity"];
	        this.traceId = source["traceId"];
	        this.spanId = source["spanId"];
	        this.parentSpanId = source["parentSpanId"];
	        this.durationMs = source["durationMs"];
	        this.status = source["status"];
	        this.attributesJson = source["attributesJson"];
	    }
	}
	export class McpConnection {
	    id: string;
	    name: string;
	    enabled: boolean;
	    transport: string;
	    url: string;
	    authType: string;
	    hasSecret: boolean;
	    lastStatus: string;
	    lastCheckedAt: string;
	    lastError: string;
	    toolCount: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new McpConnection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.transport = source["transport"];
	        this.url = source["url"];
	        this.authType = source["authType"];
	        this.hasSecret = source["hasSecret"];
	        this.lastStatus = source["lastStatus"];
	        this.lastCheckedAt = source["lastCheckedAt"];
	        this.lastError = source["lastError"];
	        this.toolCount = source["toolCount"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class McpConnectionTestResult {
	    connectionId: string;
	    status: string;
	    toolCount: number;
	    durationMs: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new McpConnectionTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionId = source["connectionId"];
	        this.status = source["status"];
	        this.toolCount = source["toolCount"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	    }
	}
	export class Message {
	    id: string;
	    sessionId: string;
	    role: string;
	    content: string;
	    createdAt: string;
	    attachments?: Attachment[];
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.createdAt = source["createdAt"];
	        this.attachments = this.convertValues(source["attachments"], Attachment);
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
	export class NetworkInterface {
	    name: string;
	    ip: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkInterface(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ip = source["ip"];
	        this.kind = source["kind"];
	    }
	}
	export class Note {
	    id: string;
	    title: string;
	    content: string;
	    updatedAt: string;
	    pinned: boolean;
	    archived: boolean;
	    inPrompt: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Note(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.updatedAt = source["updatedAt"];
	        this.pinned = source["pinned"];
	        this.archived = source["archived"];
	        this.inPrompt = source["inPrompt"];
	    }
	}
	export class PasswordEntry {
	    id: string;
	    name: string;
	    username: string;
	    password: string;
	    url: string;
	    notes: string;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new PasswordEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ProfileInfo {
	    id: string;
	    name: string;
	    avatar: string;
	    vaultDir: string;
	    lastUsed: string;
	    createdAt: string;
	    hasVault: boolean;
	    hasRecovery: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.avatar = source["avatar"];
	        this.vaultDir = source["vaultDir"];
	        this.lastUsed = source["lastUsed"];
	        this.createdAt = source["createdAt"];
	        this.hasVault = source["hasVault"];
	        this.hasRecovery = source["hasRecovery"];
	    }
	}
	export class ProviderBalanceResult {
	    available: boolean;
	    used?: string;
	    limit?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderBalanceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.used = source["used"];
	        this.limit = source["limit"];
	        this.error = source["error"];
	    }
	}
	export class ProviderInfo {
	    id: string;
	    name: string;
	    authType: string;
	    status: string;
	    model?: string;
	    connected: boolean;
	    baseUrl?: string;
	    apiKeyPlaceholder?: string;
	    baseUrlPlaceholder?: string;
	    defaultModel: string;
	    models: string[];
	    authDescription?: string;
	    allowCustomModel?: boolean;
	    requiresBaseUrl?: boolean;
	    enabled: boolean;
	    custom?: boolean;
	    deletable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProviderInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.authType = source["authType"];
	        this.status = source["status"];
	        this.model = source["model"];
	        this.connected = source["connected"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKeyPlaceholder = source["apiKeyPlaceholder"];
	        this.baseUrlPlaceholder = source["baseUrlPlaceholder"];
	        this.defaultModel = source["defaultModel"];
	        this.models = source["models"];
	        this.authDescription = source["authDescription"];
	        this.allowCustomModel = source["allowCustomModel"];
	        this.requiresBaseUrl = source["requiresBaseUrl"];
	        this.enabled = source["enabled"];
	        this.custom = source["custom"];
	        this.deletable = source["deletable"];
	    }
	}
	export class ProviderModelsResult {
	    models: string[];
	    source: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderModelsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.models = source["models"];
	        this.source = source["source"];
	        this.error = source["error"];
	    }
	}
	export class ProviderOperationResult {
	    success: boolean;
	    error?: string;
	    warning?: string;
	    saved?: boolean;
	    activated?: boolean;
	    canceled?: boolean;
	    providerId?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderOperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.warning = source["warning"];
	        this.saved = source["saved"];
	        this.activated = source["activated"];
	        this.canceled = source["canceled"];
	        this.providerId = source["providerId"];
	    }
	}
	export class ProviderSaveConfigInput {
	    provider: string;
	    model?: string;
	    apiKey?: string;
	    credential?: string;
	    baseUrl?: string;
	    setActive: boolean;
	    skipValidation?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProviderSaveConfigInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.apiKey = source["apiKey"];
	        this.credential = source["credential"];
	        this.baseUrl = source["baseUrl"];
	        this.setActive = source["setActive"];
	        this.skipValidation = source["skipValidation"];
	    }
	}
	export class ProviderStatus {
	    active: string;
	    providers: ProviderInfo[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.providers = this.convertValues(source["providers"], ProviderInfo);
	        this.error = source["error"];
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
	export class ProviderTestResult {
	    success: boolean;
	    latency?: string;
	    model?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.latency = source["latency"];
	        this.model = source["model"];
	        this.error = source["error"];
	    }
	}
	export class SkillFile {
	    path: string;
	    content: string;
	    contentHash: string;
	    seedHash: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	        this.contentHash = source["contentHash"];
	        this.seedHash = source["seedHash"];
	    }
	}
	export class Skill {
	    id: string;
	    name: string;
	    description: string;
	    enabled: boolean;
	    origin: string;
	    seedVersion: string;
	    deleted: boolean;
	    files: SkillFile[];
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.enabled = source["enabled"];
	        this.origin = source["origin"];
	        this.seedVersion = source["seedVersion"];
	        this.deleted = source["deleted"];
	        this.files = this.convertValues(source["files"], SkillFile);
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
	
	export class TasksAttachment {
	    id: string;
	    itemId: string;
	    name: string;
	    mimeType: string;
	    size: number;
	    createdAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new TasksAttachment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.itemId = source["itemId"];
	        this.name = source["name"];
	        this.mimeType = source["mimeType"];
	        this.size = source["size"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class TasksItem {
	    id: string;
	    title: string;
	    body: string;
	    status: string;
	    position: number;
	    createdAt?: string;
	    attachments: TasksAttachment[];
	
	    static createFrom(source: any = {}) {
	        return new TasksItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.body = source["body"];
	        this.status = source["status"];
	        this.position = source["position"];
	        this.createdAt = source["createdAt"];
	        this.attachments = this.convertValues(source["attachments"], TasksAttachment);
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
	
	export class UserMemoryDoc {
	    content: string;
	    backup?: string;
	    lastCondensedAt?: string;
	    updatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new UserMemoryDoc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.backup = source["backup"];
	        this.lastCondensedAt = source["lastCondensedAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

export namespace dto {
	
	export class APIServerStatus {
	    success: boolean;
	    error?: string;
	    autostart: boolean;
	    running: boolean;
	    port: number;
	    url?: string;
	    tlsEnabled: boolean;
	    coverageWarning?: string;
	
	    static createFrom(source: any = {}) {
	        return new APIServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.autostart = source["autostart"];
	        this.running = source["running"];
	        this.port = source["port"];
	        this.url = source["url"];
	        this.tlsEnabled = source["tlsEnabled"];
	        this.coverageWarning = source["coverageWarning"];
	    }
	}
	export class AgentFirewallState {
	    success: boolean;
	    error?: string;
	    rules: domain.FirewallRule[];
	    interfaces: domain.NetworkInterface[];
	    services: string[];
	
	    static createFrom(source: any = {}) {
	        return new AgentFirewallState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.rules = this.convertValues(source["rules"], domain.FirewallRule);
	        this.interfaces = this.convertValues(source["interfaces"], domain.NetworkInterface);
	        this.services = source["services"];
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
	export class AudioTranscriptionResult {
	    success: boolean;
	    error?: string;
	    text?: string;
	    provider?: string;
	    model?: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioTranscriptionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.text = source["text"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	    }
	}
	export class BrowserExecutableResult {
	    success: boolean;
	    error?: string;
	    canceled?: boolean;
	    path?: string;
	    defaultPath?: string;
	    custom: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BrowserExecutableResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.canceled = source["canceled"];
	        this.path = source["path"];
	        this.defaultPath = source["defaultPath"];
	        this.custom = source["custom"];
	    }
	}
	export class BrowserScreenshotResult {
	    success: boolean;
	    error?: string;
	    dataUri?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserScreenshotResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.dataUri = source["dataUri"];
	    }
	}
	export class BrowserStatusResult {
	    success: boolean;
	    error?: string;
	    status?: domain.BrowserStatus;
	    autostart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BrowserStatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.status = this.convertValues(source["status"], domain.BrowserStatus);
	        this.autostart = source["autostart"];
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
	export class BrowserTabsResult {
	    success: boolean;
	    error?: string;
	    tabs: domain.BrowserTab[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserTabsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.tabs = this.convertValues(source["tabs"], domain.BrowserTab);
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
	export class ChatModelResult {
	    success: boolean;
	    error?: string;
	    provider?: string;
	    providerName?: string;
	    model?: string;
	    cleared?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChatModelResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.provider = source["provider"];
	        this.providerName = source["providerName"];
	        this.model = source["model"];
	        this.cleared = source["cleared"];
	    }
	}
	export class VaultStatusResponse {
	    exists: boolean;
	    unlocked: boolean;
	    vaultDir: string;
	    hasRecovery: boolean;
	    currentProfile?: domain.ProfileInfo;
	
	    static createFrom(source: any = {}) {
	        return new VaultStatusResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	        this.unlocked = source["unlocked"];
	        this.vaultDir = source["vaultDir"];
	        this.hasRecovery = source["hasRecovery"];
	        this.currentProfile = this.convertValues(source["currentProfile"], domain.ProfileInfo);
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
	export class ChatOperationResult {
	    success: boolean;
	    error?: string;
	    status: VaultStatusResponse;
	    chats?: domain.Chat[];
	    chat?: domain.Chat;
	
	    static createFrom(source: any = {}) {
	        return new ChatOperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.status = this.convertValues(source["status"], VaultStatusResponse);
	        this.chats = this.convertValues(source["chats"], domain.Chat);
	        this.chat = this.convertValues(source["chat"], domain.Chat);
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
	export class ChatSendResult {
	    success: boolean;
	    error?: string;
	    sessionId?: string;
	    runId?: string;
	    userMessage?: domain.Message;
	    assistantMessage?: domain.Message;
	    reply: domain.AgentReply;
	
	    static createFrom(source: any = {}) {
	        return new ChatSendResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.sessionId = source["sessionId"];
	        this.runId = source["runId"];
	        this.userMessage = this.convertValues(source["userMessage"], domain.Message);
	        this.assistantMessage = this.convertValues(source["assistantMessage"], domain.Message);
	        this.reply = this.convertValues(source["reply"], domain.AgentReply);
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
	export class ChatSessionInfo {
	    provider: string;
	    model: string;
	    ready: boolean;
	    planMode: boolean;
	    streaming: boolean;
	    partialReply?: string;
	    compactReady: boolean;
	    goalReady: boolean;
	    messageCount: number;
	    tokenUsage?: domain.TokenUsage;
	    lastUsage?: domain.TokenUsage;
	    contextWindow?: number;
	    contextUsedPercent?: number;
	    compactAtPercent?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatSessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.ready = source["ready"];
	        this.planMode = source["planMode"];
	        this.streaming = source["streaming"];
	        this.partialReply = source["partialReply"];
	        this.compactReady = source["compactReady"];
	        this.goalReady = source["goalReady"];
	        this.messageCount = source["messageCount"];
	        this.tokenUsage = this.convertValues(source["tokenUsage"], domain.TokenUsage);
	        this.lastUsage = this.convertValues(source["lastUsage"], domain.TokenUsage);
	        this.contextWindow = source["contextWindow"];
	        this.contextUsedPercent = source["contextUsedPercent"];
	        this.compactAtPercent = source["compactAtPercent"];
	        this.error = source["error"];
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
	export class CodesignTrustResult {
	    supported: boolean;
	    hasCertificate: boolean;
	    trusted: boolean;
	    certificateName?: string;
	
	    static createFrom(source: any = {}) {
	        return new CodesignTrustResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.hasCertificate = source["hasCertificate"];
	        this.trusted = source["trusted"];
	        this.certificateName = source["certificateName"];
	    }
	}
	export class EffectiveInstructionsResult {
	    success: boolean;
	    error?: string;
	    effective?: domain.EffectiveInstructions;
	    generatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new EffectiveInstructionsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.effective = this.convertValues(source["effective"], domain.EffectiveInstructions);
	        this.generatedAt = source["generatedAt"];
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
	export class FolderDialogResult {
	    canceled: boolean;
	    path?: string;
	
	    static createFrom(source: any = {}) {
	        return new FolderDialogResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.canceled = source["canceled"];
	        this.path = source["path"];
	    }
	}
	export class SkillSkip {
	    id: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillSkip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.reason = source["reason"];
	    }
	}
	export class ImportSummary {
	    imported: string[];
	    skipped: SkillSkip[];
	
	    static createFrom(source: any = {}) {
	        return new ImportSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.skipped = this.convertValues(source["skipped"], SkillSkip);
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
	export class ImportSkillsResult {
	    success: boolean;
	    error?: string;
	    summary: ImportSummary;
	
	    static createFrom(source: any = {}) {
	        return new ImportSkillsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.summary = this.convertValues(source["summary"], ImportSummary);
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
	
	export class InstructionReadResult {
	    success: boolean;
	    error?: string;
	    document?: domain.InstructionDocumentContent;
	
	    static createFrom(source: any = {}) {
	        return new InstructionReadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.document = this.convertValues(source["document"], domain.InstructionDocumentContent);
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
	export class InstructionSaveResult {
	    success: boolean;
	    error?: string;
	    contextRefreshed: boolean;
	    effectiveChanged: boolean;
	    restartRequired: boolean;
	    devOverrideActive: boolean;
	    warning?: string;
	    document?: domain.InstructionDocument;
	
	    static createFrom(source: any = {}) {
	        return new InstructionSaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.contextRefreshed = source["contextRefreshed"];
	        this.effectiveChanged = source["effectiveChanged"];
	        this.restartRequired = source["restartRequired"];
	        this.devOverrideActive = source["devOverrideActive"];
	        this.warning = source["warning"];
	        this.document = this.convertValues(source["document"], domain.InstructionDocument);
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
	export class InstructionSourcesResult {
	    success: boolean;
	    error?: string;
	    sources: domain.InstructionSource[];
	
	    static createFrom(source: any = {}) {
	        return new InstructionSourcesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.sources = this.convertValues(source["sources"], domain.InstructionSource);
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
	export class InstructionsListResult {
	    success: boolean;
	    error?: string;
	    documents: domain.InstructionDocument[];
	
	    static createFrom(source: any = {}) {
	        return new InstructionsListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.documents = this.convertValues(source["documents"], domain.InstructionDocument);
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
	export class LogDatesResult {
	    success: boolean;
	    error?: string;
	    dates: domain.LogDateCount[];
	
	    static createFrom(source: any = {}) {
	        return new LogDatesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.dates = this.convertValues(source["dates"], domain.LogDateCount);
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
	export class LogRetentionResult {
	    success: boolean;
	    error?: string;
	    deleted: number;
	
	    static createFrom(source: any = {}) {
	        return new LogRetentionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.deleted = source["deleted"];
	    }
	}
	export class LogsResult {
	    success: boolean;
	    error?: string;
	    logs: domain.LogEntry[];
	
	    static createFrom(source: any = {}) {
	        return new LogsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.logs = this.convertValues(source["logs"], domain.LogEntry);
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
	export class MacosPermission {
	    id: string;
	    name: string;
	    why: string;
	    settingsPane: string;
	    deepLink: string;
	    hasProbe: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MacosPermission(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.why = source["why"];
	        this.settingsPane = source["settingsPane"];
	        this.deepLink = source["deepLink"];
	        this.hasProbe = source["hasProbe"];
	    }
	}
	export class MacosPermissionsResult {
	    items: MacosPermission[];
	
	    static createFrom(source: any = {}) {
	        return new MacosPermissionsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], MacosPermission);
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
	export class MacosProbeResult {
	    id: string;
	    status: string;
	    detail?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new MacosProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	        this.error = source["error"];
	    }
	}
	export class McpConnectionResult {
	    success: boolean;
	    error?: string;
	    connection?: domain.McpConnection;
	
	    static createFrom(source: any = {}) {
	        return new McpConnectionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.connection = this.convertValues(source["connection"], domain.McpConnection);
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
	export class McpConnectionTestResult {
	    success: boolean;
	    error?: string;
	    result?: domain.McpConnectionTestResult;
	
	    static createFrom(source: any = {}) {
	        return new McpConnectionTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.result = this.convertValues(source["result"], domain.McpConnectionTestResult);
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
	export class McpConnectionsResult {
	    success: boolean;
	    error?: string;
	    connections: domain.McpConnection[];
	
	    static createFrom(source: any = {}) {
	        return new McpConnectionsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.connections = this.convertValues(source["connections"], domain.McpConnection);
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
	export class McpToolView {
	    name: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new McpToolView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	    }
	}
	export class McpToolsResult {
	    success: boolean;
	    error?: string;
	    connectionId: string;
	    tools: McpToolView[];
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new McpToolsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.connectionId = source["connectionId"];
	        this.tools = this.convertValues(source["tools"], McpToolView);
	        this.truncated = source["truncated"];
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
	export class ModuleInfo {
	    id: string;
	    name: string;
	    icon: string;
	    description: string;
	    core: boolean;
	    fixed: boolean;
	    comingSoon: boolean;
	    added: boolean;
	    hidden: boolean;
	    sidebarPosition: number;
	
	    static createFrom(source: any = {}) {
	        return new ModuleInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.icon = source["icon"];
	        this.description = source["description"];
	        this.core = source["core"];
	        this.fixed = source["fixed"];
	        this.comingSoon = source["comingSoon"];
	        this.added = source["added"];
	        this.hidden = source["hidden"];
	        this.sidebarPosition = source["sidebarPosition"];
	    }
	}
	export class ModulesResult {
	    success: boolean;
	    error?: string;
	    modules: ModuleInfo[];
	
	    static createFrom(source: any = {}) {
	        return new ModulesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.modules = this.convertValues(source["modules"], ModuleInfo);
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
	export class NoteResult {
	    success: boolean;
	    error?: string;
	    note?: domain.Note;
	
	    static createFrom(source: any = {}) {
	        return new NoteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.note = this.convertValues(source["note"], domain.Note);
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
	export class NotesResult {
	    success: boolean;
	    error?: string;
	    notes: domain.Note[];
	
	    static createFrom(source: any = {}) {
	        return new NotesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.notes = this.convertValues(source["notes"], domain.Note);
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
	export class ObsidianFileDialogResult {
	    path?: string;
	    canceled?: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ObsidianFileDialogResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.canceled = source["canceled"];
	        this.error = source["error"];
	    }
	}
	export class ObsidianSettings {
	    enabled: boolean;
	    vaultDir: string;
	    writeEnabled: boolean;
	    deleteEnabled: boolean;
	    alwaysRead: string[];
	    alwaysReadChars: number;
	
	    static createFrom(source: any = {}) {
	        return new ObsidianSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.vaultDir = source["vaultDir"];
	        this.writeEnabled = source["writeEnabled"];
	        this.deleteEnabled = source["deleteEnabled"];
	        this.alwaysRead = source["alwaysRead"];
	        this.alwaysReadChars = source["alwaysReadChars"];
	    }
	}
	export class ObsidianTestResult {
	    success: boolean;
	    message?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ObsidianTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.error = source["error"];
	    }
	}
	export class OperationResult {
	    success: boolean;
	    error?: string;
	    recoveryKey?: string;
	    newRecoveryKey?: string;
	    status: VaultStatusResponse;
	    chats?: domain.Chat[];
	    profile?: domain.ProfileInfo;
	    valid?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.recoveryKey = source["recoveryKey"];
	        this.newRecoveryKey = source["newRecoveryKey"];
	        this.status = this.convertValues(source["status"], VaultStatusResponse);
	        this.chats = this.convertValues(source["chats"], domain.Chat);
	        this.profile = this.convertValues(source["profile"], domain.ProfileInfo);
	        this.valid = source["valid"];
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
	export class PasswordResult {
	    success: boolean;
	    error?: string;
	    password?: domain.PasswordEntry;
	
	    static createFrom(source: any = {}) {
	        return new PasswordResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.password = this.convertValues(source["password"], domain.PasswordEntry);
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
	export class PasswordsResult {
	    success: boolean;
	    error?: string;
	    passwords: domain.PasswordEntry[];
	
	    static createFrom(source: any = {}) {
	        return new PasswordsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.passwords = this.convertValues(source["passwords"], domain.PasswordEntry);
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
	export class PlanModeResult {
	    success: boolean;
	    enabled: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PlanModeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.enabled = source["enabled"];
	        this.error = source["error"];
	    }
	}
	export class PromptDebugModeResponse {
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PromptDebugModeResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	    }
	}
	export class SandboxSettings {
	    mode: string;
	    allowedFolders: string[];
	    builtinDenied: string[];
	    builtinAllowed: string[];
	    workspaceRoot: string;
	    selfDev: boolean;
	    roots: string[];
	
	    static createFrom(source: any = {}) {
	        return new SandboxSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.allowedFolders = source["allowedFolders"];
	        this.builtinDenied = source["builtinDenied"];
	        this.builtinAllowed = source["builtinAllowed"];
	        this.workspaceRoot = source["workspaceRoot"];
	        this.selfDev = source["selfDev"];
	        this.roots = source["roots"];
	    }
	}
	export class SandboxSettingsResult {
	    success: boolean;
	    error?: string;
	    canceled?: boolean;
	    settings?: SandboxSettings;
	
	    static createFrom(source: any = {}) {
	        return new SandboxSettingsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.canceled = source["canceled"];
	        this.settings = this.convertValues(source["settings"], SandboxSettings);
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
	export class SandboxTestResult {
	    allowed: boolean;
	    reason: string;
	    paths?: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new SandboxTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allowed = source["allowed"];
	        this.reason = source["reason"];
	        this.paths = source["paths"];
	        this.error = source["error"];
	    }
	}
	export class SecretResult {
	    success: boolean;
	    error?: string;
	    value?: string;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SecretResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.value = source["value"];
	        this.exists = source["exists"];
	    }
	}
	export class ServerIndicatorEntry {
	    running: boolean;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerIndicatorEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.port = source["port"];
	    }
	}
	export class ServerIndicator {
	    rest: ServerIndicatorEntry;
	    mcp: ServerIndicatorEntry;
	    web: ServerIndicatorEntry;
	
	    static createFrom(source: any = {}) {
	        return new ServerIndicator(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rest = this.convertValues(source["rest"], ServerIndicatorEntry);
	        this.mcp = this.convertValues(source["mcp"], ServerIndicatorEntry);
	        this.web = this.convertValues(source["web"], ServerIndicatorEntry);
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
	
	export class ServerTLSStatus {
	    success: boolean;
	    error?: string;
	    mode: string;
	    hasCertificate: boolean;
	    ready: boolean;
	    selfSigned: boolean;
	    subject?: string;
	    issuer?: string;
	    notBefore?: string;
	    notAfter?: string;
	    fingerprintSha256?: string;
	    dnsNames?: string[];
	    ipAddresses?: string[];
	    expiringSoon: boolean;
	    coverageWarning?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerTLSStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.mode = source["mode"];
	        this.hasCertificate = source["hasCertificate"];
	        this.ready = source["ready"];
	        this.selfSigned = source["selfSigned"];
	        this.subject = source["subject"];
	        this.issuer = source["issuer"];
	        this.notBefore = source["notBefore"];
	        this.notAfter = source["notAfter"];
	        this.fingerprintSha256 = source["fingerprintSha256"];
	        this.dnsNames = source["dnsNames"];
	        this.ipAddresses = source["ipAddresses"];
	        this.expiringSoon = source["expiringSoon"];
	        this.coverageWarning = source["coverageWarning"];
	    }
	}
	export class SkillDetailResult {
	    success: boolean;
	    error?: string;
	    skill?: domain.Skill;
	
	    static createFrom(source: any = {}) {
	        return new SkillDetailResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.skill = this.convertValues(source["skill"], domain.Skill);
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
	export class SkillResult {
	    success: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	    }
	}
	
	export class SkillView {
	    id: string;
	    name: string;
	    description: string;
	    origin: string;
	    enabled: boolean;
	    customized: boolean;
	    updateAvailable: boolean;
	    deleted: boolean;
	    fileCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SkillView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.origin = source["origin"];
	        this.enabled = source["enabled"];
	        this.customized = source["customized"];
	        this.updateAvailable = source["updateAvailable"];
	        this.deleted = source["deleted"];
	        this.fileCount = source["fileCount"];
	    }
	}
	export class SkillsResult {
	    success: boolean;
	    error?: string;
	    skills: SkillView[];
	
	    static createFrom(source: any = {}) {
	        return new SkillsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.skills = this.convertValues(source["skills"], SkillView);
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
	export class SubagentModeResponse {
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new SubagentModeResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	    }
	}
	export class TasksAttachmentDataResult {
	    success: boolean;
	    error?: string;
	    attachment?: domain.TasksAttachment;
	    dataUri?: string;
	
	    static createFrom(source: any = {}) {
	        return new TasksAttachmentDataResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.attachment = this.convertValues(source["attachment"], domain.TasksAttachment);
	        this.dataUri = source["dataUri"];
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
	export class TasksAttachmentResult {
	    success: boolean;
	    error?: string;
	    attachment?: domain.TasksAttachment;
	
	    static createFrom(source: any = {}) {
	        return new TasksAttachmentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.attachment = this.convertValues(source["attachment"], domain.TasksAttachment);
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
	export class TasksItemResult {
	    success: boolean;
	    error?: string;
	    item?: domain.TasksItem;
	
	    static createFrom(source: any = {}) {
	        return new TasksItemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.item = this.convertValues(source["item"], domain.TasksItem);
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
	export class TasksResult {
	    success: boolean;
	    error?: string;
	    items: domain.TasksItem[];
	
	    static createFrom(source: any = {}) {
	        return new TasksResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.items = this.convertValues(source["items"], domain.TasksItem);
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
	export class ThemeSuggestionResult {
	    success: boolean;
	    error?: string;
	    suggestion?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ThemeSuggestionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.suggestion = source["suggestion"];
	    }
	}
	export class UserMemoryDocResult {
	    success: boolean;
	    error?: string;
	    doc?: domain.UserMemoryDoc;
	
	    static createFrom(source: any = {}) {
	        return new UserMemoryDocResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.doc = this.convertValues(source["doc"], domain.UserMemoryDoc);
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
	
	export class VoiceCaptureResult {
	    success: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new VoiceCaptureResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	    }
	}
	export class WallpaperImageResult {
	    success: boolean;
	    error?: string;
	    dataUri?: string;
	
	    static createFrom(source: any = {}) {
	        return new WallpaperImageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.dataUri = source["dataUri"];
	    }
	}
	export class WallpaperUploadResult {
	    success: boolean;
	    error?: string;
	    canceled?: boolean;
	    id?: string;
	    custom: string[];
	
	    static createFrom(source: any = {}) {
	        return new WallpaperUploadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.canceled = source["canceled"];
	        this.id = source["id"];
	        this.custom = source["custom"];
	    }
	}
	export class WebBindCandidate {
	    iface: string;
	    ip: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new WebBindCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.iface = source["iface"];
	        this.ip = source["ip"];
	        this.kind = source["kind"];
	    }
	}
	export class WebBindInterfacesResult {
	    success: boolean;
	    error?: string;
	    candidates: WebBindCandidate[];
	
	    static createFrom(source: any = {}) {
	        return new WebBindInterfacesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.candidates = this.convertValues(source["candidates"], WebBindCandidate);
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
	export class WebServerStatus {
	    success: boolean;
	    error?: string;
	    enabled: boolean;
	    running: boolean;
	    port: number;
	    url?: string;
	    bindMode: string;
	    bindAddr?: string;
	    allowedCIDRs?: string[];
	    sessionTTLMinutes: number;
	    tailscaleIp?: string;
	    tailscaleDetected: boolean;
	    tlsEnabled: boolean;
	    coverageWarning?: string;
	
	    static createFrom(source: any = {}) {
	        return new WebServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.enabled = source["enabled"];
	        this.running = source["running"];
	        this.port = source["port"];
	        this.url = source["url"];
	        this.bindMode = source["bindMode"];
	        this.bindAddr = source["bindAddr"];
	        this.allowedCIDRs = source["allowedCIDRs"];
	        this.sessionTTLMinutes = source["sessionTTLMinutes"];
	        this.tailscaleIp = source["tailscaleIp"];
	        this.tailscaleDetected = source["tailscaleDetected"];
	        this.tlsEnabled = source["tlsEnabled"];
	        this.coverageWarning = source["coverageWarning"];
	    }
	}

}

