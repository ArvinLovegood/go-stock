export namespace agent {
	
	export class BacktestItem {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    recommendId: number;
	    stockCode: string;
	    stockName: string;
	    rating: string;
	    periodDays: number;
	    // Go type: time
	    recommendTime: any;
	    recommendPrice: number;
	    endPrice: number;
	    returnPct: number;
	    benchmarkPct: number;
	    excessPct: number;
	    outcome: string;
	    modelName: string;
	    systemPrompt: string;
	    userPrompt: string;
	    recommendTimeStr: string;
	
	    static createFrom(source: any = {}) {
	        return new BacktestItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.recommendId = source["recommendId"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.rating = source["rating"];
	        this.periodDays = source["periodDays"];
	        this.recommendTime = this.convertValues(source["recommendTime"], null);
	        this.recommendPrice = source["recommendPrice"];
	        this.endPrice = source["endPrice"];
	        this.returnPct = source["returnPct"];
	        this.benchmarkPct = source["benchmarkPct"];
	        this.excessPct = source["excessPct"];
	        this.outcome = source["outcome"];
	        this.modelName = source["modelName"];
	        this.systemPrompt = source["systemPrompt"];
	        this.userPrompt = source["userPrompt"];
	        this.recommendTimeStr = source["recommendTimeStr"];
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
	export class BacktestPageData {
	    list: BacktestItem[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new BacktestPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], BacktestItem);
	        this.total = source["total"];
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
	export class GroupStat {
	    name: string;
	    content: string;
	    total: number;
	    win: number;
	    winRate: number;
	    avgReturn: number;
	    avgExcess: number;
	
	    static createFrom(source: any = {}) {
	        return new GroupStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.content = source["content"];
	        this.total = source["total"];
	        this.win = source["win"];
	        this.winRate = source["winRate"];
	        this.avgReturn = source["avgReturn"];
	        this.avgExcess = source["avgExcess"];
	    }
	}
	export class RatingStat {
	    total: number;
	    win: number;
	    winRate: number;
	
	    static createFrom(source: any = {}) {
	        return new RatingStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.win = source["win"];
	        this.winRate = source["winRate"];
	    }
	}
	export class BacktestStats {
	    total: number;
	    win: number;
	    lose: number;
	    winRate: number;
	    byRating: Record<string, RatingStat>;
	    byModel: GroupStat[];
	    bySystemPrompt: GroupStat[];
	    byUserPrompt: GroupStat[];
	    bestModel?: GroupStat;
	    bestSystemPrompt?: GroupStat;
	    bestUserPrompt?: GroupStat;
	
	    static createFrom(source: any = {}) {
	        return new BacktestStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.win = source["win"];
	        this.lose = source["lose"];
	        this.winRate = source["winRate"];
	        this.byRating = this.convertValues(source["byRating"], RatingStat, true);
	        this.byModel = this.convertValues(source["byModel"], GroupStat);
	        this.bySystemPrompt = this.convertValues(source["bySystemPrompt"], GroupStat);
	        this.byUserPrompt = this.convertValues(source["byUserPrompt"], GroupStat);
	        this.bestModel = this.convertValues(source["bestModel"], GroupStat);
	        this.bestSystemPrompt = this.convertValues(source["bestSystemPrompt"], GroupStat);
	        this.bestUserPrompt = this.convertValues(source["bestUserPrompt"], GroupStat);
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
	export class FeedbackItem {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    userKey: string;
	    sessionId: string;
	    question: string;
	    response: string;
	    rating: number;
	    reason: string;
	    mode: string;
	    // Go type: time
	    feedbackAt: any;
	    processed: boolean;
	    feedbackAtStr: string;
	
	    static createFrom(source: any = {}) {
	        return new FeedbackItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.userKey = source["userKey"];
	        this.sessionId = source["sessionId"];
	        this.question = source["question"];
	        this.response = source["response"];
	        this.rating = source["rating"];
	        this.reason = source["reason"];
	        this.mode = source["mode"];
	        this.feedbackAt = this.convertValues(source["feedbackAt"], null);
	        this.processed = source["processed"];
	        this.feedbackAtStr = source["feedbackAtStr"];
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
	export class FeedbackPageData {
	    list: FeedbackItem[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new FeedbackPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], FeedbackItem);
	        this.total = source["total"];
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
	export class FeedbackStats {
	    total: number;
	    upCount: number;
	    downCount: number;
	    upRate: number;
	
	    static createFrom(source: any = {}) {
	        return new FeedbackStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.upRate = source["upRate"];
	    }
	}
	
	export class KBAIServiceOption {
	    id: number;
	    name: string;
	    baseUrl: string;
	    modelName: string;
	    modelType: string;
	    embeddingModel: string;
	
	    static createFrom(source: any = {}) {
	        return new KBAIServiceOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.modelName = source["modelName"];
	        this.modelType = source["modelType"];
	        this.embeddingModel = source["embeddingModel"];
	    }
	}
	export class KBDocumentIndex {
	    docId: string;
	    source: string;
	    chunkIndex: number;
	    totalChunks: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new KBDocumentIndex(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.docId = source["docId"];
	        this.source = source["source"];
	        this.chunkIndex = source["chunkIndex"];
	        this.totalChunks = source["totalChunks"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class KnowledgeBaseDocument {
	    id: string;
	    source: string;
	    chunkIndex: number;
	    totalChunks: number;
	    createdAt: string;
	    contentPreview: string;
	    metadata: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeBaseDocument(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.chunkIndex = source["chunkIndex"];
	        this.totalChunks = source["totalChunks"];
	        this.createdAt = source["createdAt"];
	        this.contentPreview = source["contentPreview"];
	        this.metadata = source["metadata"];
	    }
	}
	export class KBDocumentsPage {
	    items: KnowledgeBaseDocument[];
	    total: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new KBDocumentsPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], KnowledgeBaseDocument);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
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
	export class KBFileImportResult {
	    filePath: string;
	    fileName: string;
	    success: boolean;
	    docIds: string[];
	    chunkCount: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new KBFileImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.fileName = source["fileName"];
	        this.success = source["success"];
	        this.docIds = source["docIds"];
	        this.chunkCount = source["chunkCount"];
	        this.error = source["error"];
	    }
	}
	export class KBGraphEdge {
	    source: string;
	    target: string;
	    relation: string;
	    weight: number;
	
	    static createFrom(source: any = {}) {
	        return new KBGraphEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	        this.relation = source["relation"];
	        this.weight = source["weight"];
	    }
	}
	export class KBGraphNode {
	    id: string;
	    name: string;
	    type: string;
	    weight: number;
	
	    static createFrom(source: any = {}) {
	        return new KBGraphNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.weight = source["weight"];
	    }
	}
	export class KBGraph {
	    kbName: string;
	    nodes: KBGraphNode[];
	    edges: KBGraphEdge[];
	    // Go type: time
	    builtAt: any;
	    docCount: number;
	
	    static createFrom(source: any = {}) {
	        return new KBGraph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kbName = source["kbName"];
	        this.nodes = this.convertValues(source["nodes"], KBGraphNode);
	        this.edges = this.convertValues(source["edges"], KBGraphEdge);
	        this.builtAt = this.convertValues(source["builtAt"], null);
	        this.docCount = source["docCount"];
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
	export class KBGraphBuildStatus {
	    isBuilding: boolean;
	    totalDocs: number;
	    processedDocs: number;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt?: any;
	    error?: string;
	    nodeCount: number;
	    edgeCount: number;
	
	    static createFrom(source: any = {}) {
	        return new KBGraphBuildStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isBuilding = source["isBuilding"];
	        this.totalDocs = source["totalDocs"];
	        this.processedDocs = source["processedDocs"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.error = source["error"];
	        this.nodeCount = source["nodeCount"];
	        this.edgeCount = source["edgeCount"];
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
	
	
	export class KBVectorizingStatus {
	    isVectorizing: boolean;
	    totalFiles: number;
	    processedFiles: number;
	    successCount: number;
	    failedCount: number;
	    totalChunks: number;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt?: any;
	    results?: KBFileImportResult[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new KBVectorizingStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isVectorizing = source["isVectorizing"];
	        this.totalFiles = source["totalFiles"];
	        this.processedFiles = source["processedFiles"];
	        this.successCount = source["successCount"];
	        this.failedCount = source["failedCount"];
	        this.totalChunks = source["totalChunks"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.results = this.convertValues(source["results"], KBFileImportResult);
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
	
	export class KnowledgeBaseInfo {
	    name: string;
	    description: string;
	    documentCount: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    documents?: KBDocumentIndex[];
	    aiConfigId: number;
	    aiConfigName?: string;
	    embeddingModel?: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeBaseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.documentCount = source["documentCount"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.documents = this.convertValues(source["documents"], KBDocumentIndex);
	        this.aiConfigId = source["aiConfigId"];
	        this.aiConfigName = source["aiConfigName"];
	        this.embeddingModel = source["embeddingModel"];
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
	export class KnowledgeBaseSearchResult {
	    kbName: string;
	    documentId: string;
	    source: string;
	    content: string;
	    similarity: number;
	    metadata: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeBaseSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kbName = source["kbName"];
	        this.documentId = source["documentId"];
	        this.source = source["source"];
	        this.content = source["content"];
	        this.similarity = source["similarity"];
	        this.metadata = source["metadata"];
	    }
	}
	export class LTMInfo {
	    ready: boolean;
	    docCount: number;
	    error?: string;
	    aiConfigId: number;
	    aiConfigName?: string;
	
	    static createFrom(source: any = {}) {
	        return new LTMInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ready = source["ready"];
	        this.docCount = source["docCount"];
	        this.error = source["error"];
	        this.aiConfigId = source["aiConfigId"];
	        this.aiConfigName = source["aiConfigName"];
	    }
	}
	export class MemoryRecall {
	    question: string;
	    reply: string;
	    mode: string;
	    date: string;
	    reportPath: string;
	    similarity: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryRecall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.question = source["question"];
	        this.reply = source["reply"];
	        this.mode = source["mode"];
	        this.date = source["date"];
	        this.reportPath = source["reportPath"];
	        this.similarity = source["similarity"];
	    }
	}
	
	export class UnifiedKnowledgeHit {
	    sourceType: string;
	    kbName: string;
	    source: string;
	    content: string;
	    similarity: number;
	    metadata: Record<string, string>;
	    question?: string;
	    mode?: string;
	    date?: string;
	    reportPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new UnifiedKnowledgeHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceType = source["sourceType"];
	        this.kbName = source["kbName"];
	        this.source = source["source"];
	        this.content = source["content"];
	        this.similarity = source["similarity"];
	        this.metadata = source["metadata"];
	        this.question = source["question"];
	        this.mode = source["mode"];
	        this.date = source["date"];
	        this.reportPath = source["reportPath"];
	    }
	}
	export class UserProfileSnapshot {
	    content: string;
	    enabled: boolean;
	    updatedAt: string;
	    knownFields: number;
	    totalFields: number;
	
	    static createFrom(source: any = {}) {
	        return new UserProfileSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.enabled = source["enabled"];
	        this.updatedAt = source["updatedAt"];
	        this.knownFields = source["knownFields"];
	        this.totalFields = source["totalFields"];
	    }
	}

}

export namespace data {
	
	export class AIConfig {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    name: string;
	    baseUrl: string;
	    apiKey: string;
	    modelName: string;
	    modelType: string;
	    maxTokens: number;
	    contextWindow: number;
	    temperature: number;
	    timeOut: number;
	    httpProxy: string;
	    httpProxyEnabled: boolean;
	    sessionId: string;
	    thinking: boolean;
	    extraHeaders: string;
	    embeddingModel: string;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.modelName = source["modelName"];
	        this.modelType = source["modelType"];
	        this.maxTokens = source["maxTokens"];
	        this.contextWindow = source["contextWindow"];
	        this.temperature = source["temperature"];
	        this.timeOut = source["timeOut"];
	        this.httpProxy = source["httpProxy"];
	        this.httpProxyEnabled = source["httpProxyEnabled"];
	        this.sessionId = source["sessionId"];
	        this.thinking = source["thinking"];
	        this.extraHeaders = source["extraHeaders"];
	        this.embeddingModel = source["embeddingModel"];
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
	export class AllStockInfoPageData {
	    list: models.AllStockInfo[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new AllStockInfoPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], models.AllStockInfo);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class AllStockInfoQuery {
	    page: number;
	    pageSize: number;
	    securityCode: string;
	    securityName: string;
	    market: string;
	    industry: string;
	    concept: string;
	    minPrice: string;
	    maxPrice: string;
	    minChange: string;
	    maxChange: string;
	    searchKeyWord: string;
	
	    static createFrom(source: any = {}) {
	        return new AllStockInfoQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.securityCode = source["securityCode"];
	        this.securityName = source["securityName"];
	        this.market = source["market"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	        this.minPrice = source["minPrice"];
	        this.maxPrice = source["maxPrice"];
	        this.minChange = source["minChange"];
	        this.maxChange = source["maxChange"];
	        this.searchKeyWord = source["searchKeyWord"];
	    }
	}
	export class ChangeRankItem {
	    name: string;
	    code?: string;
	    count: number;
	    upCount: number;
	    downCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ChangeRankItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.code = source["code"];
	        this.count = source["count"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	    }
	}
	export class ChangeRankResult {
	    topStocks: ChangeRankItem[];
	    topIndustries: ChangeRankItem[];
	    topConcepts: ChangeRankItem[];
	
	    static createFrom(source: any = {}) {
	        return new ChangeRankResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.topStocks = this.convertValues(source["topStocks"], ChangeRankItem);
	        this.topIndustries = this.convertValues(source["topIndustries"], ChangeRankItem);
	        this.topConcepts = this.convertValues(source["topConcepts"], ChangeRankItem);
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
	export class ChangeTypeDailyStats {
	    changeDate: string;
	    typeName: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ChangeTypeDailyStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changeDate = source["changeDate"];
	        this.typeName = source["typeName"];
	        this.count = source["count"];
	    }
	}
	export class ChipBin {
	    price: number;
	    vol: number;
	    ratio: number;
	
	    static createFrom(source: any = {}) {
	        return new ChipBin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.price = source["price"];
	        this.vol = source["vol"];
	        this.ratio = source["ratio"];
	    }
	}
	export class ChipDistributionResult {
	    stockCode: string;
	    days: number;
	    bins: number;
	    current: number;
	    avgCost: number;
	    profitRatio: number;
	    minPrice: number;
	    maxPrice: number;
	    sumVol: number;
	    items: ChipBin[];
	
	    static createFrom(source: any = {}) {
	        return new ChipDistributionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.days = source["days"];
	        this.bins = source["bins"];
	        this.current = source["current"];
	        this.avgCost = source["avgCost"];
	        this.profitRatio = source["profitRatio"];
	        this.minPrice = source["minPrice"];
	        this.maxPrice = source["maxPrice"];
	        this.sumVol = source["sumVol"];
	        this.items = this.convertValues(source["items"], ChipBin);
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
	export class Concept {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    name: string;
	    sort: number;
	
	    static createFrom(source: any = {}) {
	        return new Concept(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.name = source["name"];
	        this.sort = source["sort"];
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
	export class ConceptPlate {
	    code: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ConceptPlate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	    }
	}
	export class ConceptStock {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    stockCode: string;
	    conceptId: number;
	    conceptInfo: Concept;
	
	    static createFrom(source: any = {}) {
	        return new ConceptStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.stockCode = source["stockCode"];
	        this.conceptId = source["conceptId"];
	        this.conceptInfo = this.convertValues(source["conceptInfo"], Concept);
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
	export class DailyChangeStats {
	    changeDate: string;
	    totalCount: number;
	    upCount: number;
	    downCount: number;
	    limitUp: number;
	    limitDown: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyChangeStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changeDate = source["changeDate"];
	        this.totalCount = source["totalCount"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.limitUp = source["limitUp"];
	        this.limitDown = source["limitDown"];
	    }
	}
	export class DailyDimensionStats {
	    changeDate: string;
	    upCount: number;
	    downCount: number;
	    totalCount: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyDimensionStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changeDate = source["changeDate"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.totalCount = source["totalCount"];
	    }
	}
	export class FundBasic {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    code: string;
	    name: string;
	    fullName: string;
	    type: string;
	    establishment: string;
	    scale: string;
	    company: string;
	    manager: string;
	    rating: string;
	    trackingTarget: string;
	    netUnitValue?: number;
	    netUnitValueDate: string;
	    netEstimatedUnit?: number;
	    netEstimatedUnitTime: string;
	    netAccumulated?: number;
	    netGrowth1?: number;
	    netGrowth3?: number;
	    netGrowth6?: number;
	    netGrowth12?: number;
	    netGrowth36?: number;
	    netGrowth60?: number;
	    netGrowthYTD?: number;
	    netGrowthAll?: number;
	
	    static createFrom(source: any = {}) {
	        return new FundBasic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.type = source["type"];
	        this.establishment = source["establishment"];
	        this.scale = source["scale"];
	        this.company = source["company"];
	        this.manager = source["manager"];
	        this.rating = source["rating"];
	        this.trackingTarget = source["trackingTarget"];
	        this.netUnitValue = source["netUnitValue"];
	        this.netUnitValueDate = source["netUnitValueDate"];
	        this.netEstimatedUnit = source["netEstimatedUnit"];
	        this.netEstimatedUnitTime = source["netEstimatedUnitTime"];
	        this.netAccumulated = source["netAccumulated"];
	        this.netGrowth1 = source["netGrowth1"];
	        this.netGrowth3 = source["netGrowth3"];
	        this.netGrowth6 = source["netGrowth6"];
	        this.netGrowth12 = source["netGrowth12"];
	        this.netGrowth36 = source["netGrowth36"];
	        this.netGrowth60 = source["netGrowth60"];
	        this.netGrowthYTD = source["netGrowthYTD"];
	        this.netGrowthAll = source["netGrowthAll"];
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
	export class FollowedFund {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    code: string;
	    name: string;
	    netUnitValue?: number;
	    netUnitValueDate: string;
	    netEstimatedUnit?: number;
	    netEstimatedUnitTime: string;
	    netAccumulated?: number;
	    netEstimatedRate?: number;
	    netUnitValuePrev?: number;
	    netActualRate?: number;
	    fundBasic: FundBasic;
	
	    static createFrom(source: any = {}) {
	        return new FollowedFund(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.netUnitValue = source["netUnitValue"];
	        this.netUnitValueDate = source["netUnitValueDate"];
	        this.netEstimatedUnit = source["netEstimatedUnit"];
	        this.netEstimatedUnitTime = source["netEstimatedUnitTime"];
	        this.netAccumulated = source["netAccumulated"];
	        this.netEstimatedRate = source["netEstimatedRate"];
	        this.netUnitValuePrev = source["netUnitValuePrev"];
	        this.netActualRate = source["netActualRate"];
	        this.fundBasic = this.convertValues(source["fundBasic"], FundBasic);
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
	export class FollowedFundPagedResult {
	    items: FollowedFund[];
	    totalCount: number;
	    pageIndex: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new FollowedFundPagedResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], FollowedFund);
	        this.totalCount = source["totalCount"];
	        this.pageIndex = source["pageIndex"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class Group {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    name: string;
	    sort: number;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.name = source["name"];
	        this.sort = source["sort"];
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
	export class GroupStock {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    stockCode: string;
	    groupId: number;
	    groupInfo: Group;
	
	    static createFrom(source: any = {}) {
	        return new GroupStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.stockCode = source["stockCode"];
	        this.groupId = source["groupId"];
	        this.groupInfo = this.convertValues(source["groupInfo"], Group);
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
	export class FollowedStock {
	    StockCode: string;
	    Name: string;
	    Volume: number;
	    CostPrice: number;
	    Price: number;
	    PriceChange: number;
	    ChangePercent: number;
	    AlarmChangePercent: number;
	    AlarmPrice: number;
	    // Go type: time
	    Time: any;
	    Sort: number;
	    Cron?: string;
	    IsDel: number;
	    Groups: GroupStock[];
	    AiConfigId: number;
	    EntryPrice: number;
	    TakeProfitPrice: number;
	    StopLossPrice: number;
	
	    static createFrom(source: any = {}) {
	        return new FollowedStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.StockCode = source["StockCode"];
	        this.Name = source["Name"];
	        this.Volume = source["Volume"];
	        this.CostPrice = source["CostPrice"];
	        this.Price = source["Price"];
	        this.PriceChange = source["PriceChange"];
	        this.ChangePercent = source["ChangePercent"];
	        this.AlarmChangePercent = source["AlarmChangePercent"];
	        this.AlarmPrice = source["AlarmPrice"];
	        this.Time = this.convertValues(source["Time"], null);
	        this.Sort = source["Sort"];
	        this.Cron = source["Cron"];
	        this.IsDel = source["IsDel"];
	        this.Groups = this.convertValues(source["Groups"], GroupStock);
	        this.AiConfigId = source["AiConfigId"];
	        this.EntryPrice = source["EntryPrice"];
	        this.TakeProfitPrice = source["TakeProfitPrice"];
	        this.StopLossPrice = source["StopLossPrice"];
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
	
	export class FundHistoryNetValue {
	    date: string;
	    netValue: number;
	    accumValue: number;
	    dailyGrowth: number;
	    buyStatus: string;
	    sellStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new FundHistoryNetValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.netValue = source["netValue"];
	        this.accumValue = source["accumValue"];
	        this.dailyGrowth = source["dailyGrowth"];
	        this.buyStatus = source["buyStatus"];
	        this.sellStatus = source["sellStatus"];
	    }
	}
	export class FundHoldingStock {
	    rank: number;
	    stockCode: string;
	    stockName: string;
	    ratio: number;
	    shares: string;
	    marketCap: string;
	    quarter: string;
	    price?: number;
	    changeRate?: number;
	    market: string;
	
	    static createFrom(source: any = {}) {
	        return new FundHoldingStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.ratio = source["ratio"];
	        this.shares = source["shares"];
	        this.marketCap = source["marketCap"];
	        this.quarter = source["quarter"];
	        this.price = source["price"];
	        this.changeRate = source["changeRate"];
	        this.market = source["market"];
	    }
	}
	export class FundRankingItem {
	    code: string;
	    name: string;
	    pinyin: string;
	    netValueDate: string;
	    netUnitValue?: number;
	    netAccumulated?: number;
	    dailyGrowth?: number;
	    weekGrowth?: number;
	    monthGrowth?: number;
	    threeMonthGrowth?: number;
	    sixMonthGrowth?: number;
	    yearGrowth?: number;
	    twoYearGrowth?: number;
	    threeYearGrowth?: number;
	    ytdGrowth?: number;
	    sinceInception?: number;
	    establishDate: string;
	    purchasable: boolean;
	    scale?: number;
	    purchaseRate?: number;
	    discountRate?: number;
	    fundTypeDetail: string;
	
	    static createFrom(source: any = {}) {
	        return new FundRankingItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.pinyin = source["pinyin"];
	        this.netValueDate = source["netValueDate"];
	        this.netUnitValue = source["netUnitValue"];
	        this.netAccumulated = source["netAccumulated"];
	        this.dailyGrowth = source["dailyGrowth"];
	        this.weekGrowth = source["weekGrowth"];
	        this.monthGrowth = source["monthGrowth"];
	        this.threeMonthGrowth = source["threeMonthGrowth"];
	        this.sixMonthGrowth = source["sixMonthGrowth"];
	        this.yearGrowth = source["yearGrowth"];
	        this.twoYearGrowth = source["twoYearGrowth"];
	        this.threeYearGrowth = source["threeYearGrowth"];
	        this.ytdGrowth = source["ytdGrowth"];
	        this.sinceInception = source["sinceInception"];
	        this.establishDate = source["establishDate"];
	        this.purchasable = source["purchasable"];
	        this.scale = source["scale"];
	        this.purchaseRate = source["purchaseRate"];
	        this.discountRate = source["discountRate"];
	        this.fundTypeDetail = source["fundTypeDetail"];
	    }
	}
	export class FundRankingResult {
	    items: FundRankingItem[];
	    totalCount: number;
	    pageIndex: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new FundRankingResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], FundRankingItem);
	        this.totalCount = source["totalCount"];
	        this.pageIndex = source["pageIndex"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class FundSearchItem {
	    code: string;
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new FundSearchItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class FuturesMemberRank {
	    contract: string;
	    rank: number;
	    volName: string;
	    volume: number;
	    volChange: number;
	    longName: string;
	    longPosition: number;
	    longChange: number;
	    shortName: string;
	    shortPosition: number;
	    shortChange: number;
	
	    static createFrom(source: any = {}) {
	        return new FuturesMemberRank(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contract = source["contract"];
	        this.rank = source["rank"];
	        this.volName = source["volName"];
	        this.volume = source["volume"];
	        this.volChange = source["volChange"];
	        this.longName = source["longName"];
	        this.longPosition = source["longPosition"];
	        this.longChange = source["longChange"];
	        this.shortName = source["shortName"];
	        this.shortPosition = source["shortPosition"];
	        this.shortChange = source["shortChange"];
	    }
	}
	export class FuturesPositionRow {
	    tradeDate: string;
	    settlePrice: number;
	    longPosition: number;
	    longChange: number;
	    shortPosition: number;
	    shortChange: number;
	    netPosition: number;
	    indexClose: number;
	    indexChange: number;
	    basis: number;
	
	    static createFrom(source: any = {}) {
	        return new FuturesPositionRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tradeDate = source["tradeDate"];
	        this.settlePrice = source["settlePrice"];
	        this.longPosition = source["longPosition"];
	        this.longChange = source["longChange"];
	        this.shortPosition = source["shortPosition"];
	        this.shortChange = source["shortChange"];
	        this.netPosition = source["netPosition"];
	        this.indexClose = source["indexClose"];
	        this.indexChange = source["indexChange"];
	        this.basis = source["basis"];
	    }
	}
	export class FuturesPositionResp {
	    variety: string;
	    varietyName: string;
	    contractCode: string;
	    indexCode: string;
	    source: string;
	    rows: FuturesPositionRow[];
	
	    static createFrom(source: any = {}) {
	        return new FuturesPositionResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variety = source["variety"];
	        this.varietyName = source["varietyName"];
	        this.contractCode = source["contractCode"];
	        this.indexCode = source["indexCode"];
	        this.source = source["source"];
	        this.rows = this.convertValues(source["rows"], FuturesPositionRow);
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
	
	export class GlobalIndexTrendItem {
	    time: string;
	    price: number;
	    avgPrice: number;
	    volume: number;
	    amount: number;
	
	    static createFrom(source: any = {}) {
	        return new GlobalIndexTrendItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.price = source["price"];
	        this.avgPrice = source["avgPrice"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
	    }
	}
	export class GlobalIndexTrendResult {
	    code: string;
	    name: string;
	    preClose: number;
	    date: string;
	    items: GlobalIndexTrendItem[];
	
	    static createFrom(source: any = {}) {
	        return new GlobalIndexTrendResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.preClose = source["preClose"];
	        this.date = source["date"];
	        this.items = this.convertValues(source["items"], GlobalIndexTrendItem);
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
	
	
	export class IndexQuoteItem {
	    secu_code: string;
	    secu_name: string;
	    last_px: number;
	    change: number;
	    change_px: number;
	
	    static createFrom(source: any = {}) {
	        return new IndexQuoteItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.secu_code = source["secu_code"];
	        this.secu_name = source["secu_name"];
	        this.last_px = source["last_px"];
	        this.change = source["change"];
	        this.change_px = source["change_px"];
	    }
	}
	export class IndexTlineItem {
	    date: number;
	    minute: number;
	    last_px: number;
	    change: number;
	    change_color: number;
	    amp: number;
	    preclose_px: number;
	    open_px: number;
	    change_px: number;
	    business_amount: number;
	    business_balance: number;
	
	    static createFrom(source: any = {}) {
	        return new IndexTlineItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.minute = source["minute"];
	        this.last_px = source["last_px"];
	        this.change = source["change"];
	        this.change_color = source["change_color"];
	        this.amp = source["amp"];
	        this.preclose_px = source["preclose_px"];
	        this.open_px = source["open_px"];
	        this.change_px = source["change_px"];
	        this.business_amount = source["business_amount"];
	        this.business_balance = source["business_balance"];
	    }
	}
	export class IndexTlineResult {
	    date: string;
	    totalBalance: number;
	    prevBalance: number;
	    balanceChange: number;
	    balanceChangePct: number;
	    items: IndexTlineItem[];
	
	    static createFrom(source: any = {}) {
	        return new IndexTlineResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.totalBalance = source["totalBalance"];
	        this.prevBalance = source["prevBalance"];
	        this.balanceChange = source["balanceChange"];
	        this.balanceChangePct = source["balanceChangePct"];
	        this.items = this.convertValues(source["items"], IndexTlineItem);
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
	export class IndustryPlate {
	    code: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new IndustryPlate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	    }
	}
	export class KLineData {
	    day: string;
	    open: string;
	    close: string;
	    high: string;
	    low: string;
	    volume: string;
	    amount: string;
	    changePercent: string;
	    changeValue: string;
	    amplitude: string;
	    turnoverRate: string;
	    volumeRatio: string;
	    ma?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new KLineData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.day = source["day"];
	        this.open = source["open"];
	        this.close = source["close"];
	        this.high = source["high"];
	        this.low = source["low"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
	        this.changePercent = source["changePercent"];
	        this.changeValue = source["changeValue"];
	        this.amplitude = source["amplitude"];
	        this.turnoverRate = source["turnoverRate"];
	        this.volumeRatio = source["volumeRatio"];
	        this.ma = source["ma"];
	    }
	}
	export class KLineSourceResult {
	    data?: KLineData[];
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new KLineSourceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data = this.convertValues(source["data"], KLineData);
	        this.source = source["source"];
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
	export class MarketEmotion {
	    market_degree: string;
	    shsz_balance: string;
	    shsz_balance_change_px: string;
	    up_ratio: string;
	    up_ratio_num: string;
	    performance: string;
	    up_open_ratio: string;
	    profit_ratio: string;
	    // Go type: struct { SuspendNum int "json:\"suspend_num\""; UpNum int "json:\"up_num\""; DownNum int "json:\"down_num\""; RiseNum int "json:\"rise_num\""; FallNum int "json:\"fall_num\""; FlatNum int "json:\"flat_num\"" }
	    up_down_dis: any;
	    // Go type: struct { Row1 []string "json:\"row1\""; Row2 []string "json:\"row2\""; Row3 []string "json:\"row3\"" }
	    limit_up_board: any;
	
	    static createFrom(source: any = {}) {
	        return new MarketEmotion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.market_degree = source["market_degree"];
	        this.shsz_balance = source["shsz_balance"];
	        this.shsz_balance_change_px = source["shsz_balance_change_px"];
	        this.up_ratio = source["up_ratio"];
	        this.up_ratio_num = source["up_ratio_num"];
	        this.performance = source["performance"];
	        this.up_open_ratio = source["up_open_ratio"];
	        this.profit_ratio = source["profit_ratio"];
	        this.up_down_dis = this.convertValues(source["up_down_dis"], Object);
	        this.limit_up_board = this.convertValues(source["limit_up_board"], Object);
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
	export class SectorAnchor {
	    symbol_code: string;
	    symbol_name: string;
	    article_id: number;
	    c_time: string;
	    float: string;
	
	    static createFrom(source: any = {}) {
	        return new SectorAnchor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol_code = source["symbol_code"];
	        this.symbol_name = source["symbol_name"];
	        this.article_id = source["article_id"];
	        this.c_time = source["c_time"];
	        this.float = source["float"];
	    }
	}
	export class SettingConfig {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    tushareToken: string;
	    localPushEnable: boolean;
	    dingPushEnable: boolean;
	    dingRobot: string;
	    feishuPushEnable: boolean;
	    feishuRobot: string;
	    feishuSecret: string;
	    feishuBotEnable: boolean;
	    feishuAppId: string;
	    feishuAppSecret: string;
	    feishuBotAiConfigId: number;
	    feishuBotSysPromptId: number;
	    feishuBotEnableTools: boolean;
	    feishuBotThinking: boolean;
	    feishuBotAgentMode: string;
	    updateBasicInfoOnStart: boolean;
	    refreshInterval: number;
	    openAiEnable: boolean;
	    prompt: string;
	    checkUpdate: boolean;
	    updateChannel: string;
	    questionTemplate: string;
	    crawlTimeOut: number;
	    kDays: number;
	    enableDanmu: boolean;
	    browserPath: string;
	    enableNews: boolean;
	    darkTheme: boolean;
	    browserPoolSize: number;
	    enableFund: boolean;
	    enablePushNews: boolean;
	    enableOnlyPushRedNews: boolean;
	    sponsorCode: string;
	    httpProxy: string;
	    httpProxyEnabled: boolean;
	    enableAgent: boolean;
	    qgqpBId: string;
	    iwencaiApiKey: string;
	    emApiKey: string;
	    windowWidth: number;
	    windowHeight: number;
	    promptPlazaApiBase: string;
	    longTermMemoryAiConfigId: number;
	    aiConfigs: AIConfig[];
	
	    static createFrom(source: any = {}) {
	        return new SettingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.tushareToken = source["tushareToken"];
	        this.localPushEnable = source["localPushEnable"];
	        this.dingPushEnable = source["dingPushEnable"];
	        this.dingRobot = source["dingRobot"];
	        this.feishuPushEnable = source["feishuPushEnable"];
	        this.feishuRobot = source["feishuRobot"];
	        this.feishuSecret = source["feishuSecret"];
	        this.feishuBotEnable = source["feishuBotEnable"];
	        this.feishuAppId = source["feishuAppId"];
	        this.feishuAppSecret = source["feishuAppSecret"];
	        this.feishuBotAiConfigId = source["feishuBotAiConfigId"];
	        this.feishuBotSysPromptId = source["feishuBotSysPromptId"];
	        this.feishuBotEnableTools = source["feishuBotEnableTools"];
	        this.feishuBotThinking = source["feishuBotThinking"];
	        this.feishuBotAgentMode = source["feishuBotAgentMode"];
	        this.updateBasicInfoOnStart = source["updateBasicInfoOnStart"];
	        this.refreshInterval = source["refreshInterval"];
	        this.openAiEnable = source["openAiEnable"];
	        this.prompt = source["prompt"];
	        this.checkUpdate = source["checkUpdate"];
	        this.updateChannel = source["updateChannel"];
	        this.questionTemplate = source["questionTemplate"];
	        this.crawlTimeOut = source["crawlTimeOut"];
	        this.kDays = source["kDays"];
	        this.enableDanmu = source["enableDanmu"];
	        this.browserPath = source["browserPath"];
	        this.enableNews = source["enableNews"];
	        this.darkTheme = source["darkTheme"];
	        this.browserPoolSize = source["browserPoolSize"];
	        this.enableFund = source["enableFund"];
	        this.enablePushNews = source["enablePushNews"];
	        this.enableOnlyPushRedNews = source["enableOnlyPushRedNews"];
	        this.sponsorCode = source["sponsorCode"];
	        this.httpProxy = source["httpProxy"];
	        this.httpProxyEnabled = source["httpProxyEnabled"];
	        this.enableAgent = source["enableAgent"];
	        this.qgqpBId = source["qgqpBId"];
	        this.iwencaiApiKey = source["iwencaiApiKey"];
	        this.emApiKey = source["emApiKey"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.promptPlazaApiBase = source["promptPlazaApiBase"];
	        this.longTermMemoryAiConfigId = source["longTermMemoryAiConfigId"];
	        this.aiConfigs = this.convertValues(source["aiConfigs"], AIConfig);
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
	export class StockBasic {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    ts_code: string;
	    symbol: string;
	    name: string;
	    area: string;
	    industry: string;
	    fullname: string;
	    enname: string;
	    cnspell: string;
	    market: string;
	    exchange: string;
	    curr_type: string;
	    list_status: string;
	    list_date: string;
	    delist_date: string;
	    is_hs: string;
	    act_name: string;
	    act_ent_type: string;
	    bk_name: string;
	    bk_code: string;
	
	    static createFrom(source: any = {}) {
	        return new StockBasic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.ts_code = source["ts_code"];
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.area = source["area"];
	        this.industry = source["industry"];
	        this.fullname = source["fullname"];
	        this.enname = source["enname"];
	        this.cnspell = source["cnspell"];
	        this.market = source["market"];
	        this.exchange = source["exchange"];
	        this.curr_type = source["curr_type"];
	        this.list_status = source["list_status"];
	        this.list_date = source["list_date"];
	        this.delist_date = source["delist_date"];
	        this.is_hs = source["is_hs"];
	        this.act_name = source["act_name"];
	        this.act_ent_type = source["act_ent_type"];
	        this.bk_name = source["bk_name"];
	        this.bk_code = source["bk_code"];
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
	export class StockChangeItem {
	    time: string;
	    code: string;
	    name: string;
	    market: number;
	    changeType: number;
	    typeName: string;
	    volume: number;
	    price: number;
	    changeRate: number;
	    amount: number;
	    industry: string;
	    concept: string;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.market = source["market"];
	        this.changeType = source["changeType"];
	        this.typeName = source["typeName"];
	        this.volume = source["volume"];
	        this.price = source["price"];
	        this.changeRate = source["changeRate"];
	        this.amount = source["amount"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	    }
	}
	export class StockChangesResponse {
	    totalCount: number;
	    data: StockChangeItem[];
	
	    static createFrom(source: any = {}) {
	        return new StockChangesResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalCount = source["totalCount"];
	        this.data = this.convertValues(source["data"], StockChangeItem);
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
	export class StockInfo {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    "日期": string;
	    "时间": string;
	    "股票代码": string;
	    "股票名称": string;
	    "上次当前价格": number;
	    "当前价格": string;
	    "成交的股票数": string;
	    "成交金额": string;
	    "今日开盘价": string;
	    "昨日收盘价": string;
	    "今日最高价": string;
	    "今日最低价": string;
	    "竞买价": string;
	    "竞卖价": string;
	    "买一报价": string;
	    "买一申报": string;
	    "买二报价": string;
	    "买二申报": string;
	    "买三报价": string;
	    "买三申报": string;
	    "买四报价": string;
	    "买四申报": string;
	    "买五报价": string;
	    "买五申报": string;
	    "卖一报价": string;
	    "卖一申报": string;
	    "卖二报价": string;
	    "卖二申报": string;
	    "卖三报价": string;
	    "卖三申报": string;
	    "卖四报价": string;
	    "卖四申报": string;
	    "卖五报价": string;
	    "卖五申报": string;
	    "市场": string;
	    "盘前盘后": string;
	    "盘前盘后涨跌幅": string;
	    changePercent: number;
	    changePrice: number;
	    highRate: number;
	    lowRate: number;
	    costPrice: number;
	    costVolume: number;
	    profit: number;
	    profitAmount: number;
	    profitAmountToday: number;
	    sort: number;
	    alarmChangePercent: number;
	    alarmPrice: number;
	    Groups: GroupStock[];
	
	    static createFrom(source: any = {}) {
	        return new StockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this["日期"] = source["日期"];
	        this["时间"] = source["时间"];
	        this["股票代码"] = source["股票代码"];
	        this["股票名称"] = source["股票名称"];
	        this["上次当前价格"] = source["上次当前价格"];
	        this["当前价格"] = source["当前价格"];
	        this["成交的股票数"] = source["成交的股票数"];
	        this["成交金额"] = source["成交金额"];
	        this["今日开盘价"] = source["今日开盘价"];
	        this["昨日收盘价"] = source["昨日收盘价"];
	        this["今日最高价"] = source["今日最高价"];
	        this["今日最低价"] = source["今日最低价"];
	        this["竞买价"] = source["竞买价"];
	        this["竞卖价"] = source["竞卖价"];
	        this["买一报价"] = source["买一报价"];
	        this["买一申报"] = source["买一申报"];
	        this["买二报价"] = source["买二报价"];
	        this["买二申报"] = source["买二申报"];
	        this["买三报价"] = source["买三报价"];
	        this["买三申报"] = source["买三申报"];
	        this["买四报价"] = source["买四报价"];
	        this["买四申报"] = source["买四申报"];
	        this["买五报价"] = source["买五报价"];
	        this["买五申报"] = source["买五申报"];
	        this["卖一报价"] = source["卖一报价"];
	        this["卖一申报"] = source["卖一申报"];
	        this["卖二报价"] = source["卖二报价"];
	        this["卖二申报"] = source["卖二申报"];
	        this["卖三报价"] = source["卖三报价"];
	        this["卖三申报"] = source["卖三申报"];
	        this["卖四报价"] = source["卖四报价"];
	        this["卖四申报"] = source["卖四申报"];
	        this["卖五报价"] = source["卖五报价"];
	        this["卖五申报"] = source["卖五申报"];
	        this["市场"] = source["市场"];
	        this["盘前盘后"] = source["盘前盘后"];
	        this["盘前盘后涨跌幅"] = source["盘前盘后涨跌幅"];
	        this.changePercent = source["changePercent"];
	        this.changePrice = source["changePrice"];
	        this.highRate = source["highRate"];
	        this.lowRate = source["lowRate"];
	        this.costPrice = source["costPrice"];
	        this.costVolume = source["costVolume"];
	        this.profit = source["profit"];
	        this.profitAmount = source["profitAmount"];
	        this.profitAmountToday = source["profitAmountToday"];
	        this.sort = source["sort"];
	        this.alarmChangePercent = source["alarmChangePercent"];
	        this.alarmPrice = source["alarmPrice"];
	        this.Groups = this.convertValues(source["Groups"], GroupStock);
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
	export class TdxFinanceInfo {
	    market: number;
	    code: string;
	    floatShares: number;
	    totalShares: number;
	    eps: number;
	    totalAssets: number;
	    currentAssets: number;
	    fixedAssets: number;
	    intangibleAssets: number;
	    shareholderCount: number;
	    currentLiabilities: number;
	    longTermLiabilities: number;
	    capitalReserve: number;
	    totalEquity: number;
	    operatingRevenue: number;
	    operatingCost: number;
	    accountsReceivable: number;
	    operatingProfit: number;
	    investmentIncome: number;
	    netCashFlow: number;
	    inventory: number;
	    totalProfit: number;
	    afterTaxProfit: number;
	    netProfit: number;
	    undistributedProfit: number;
	    netAssetsPerShare: number;
	    ipoDate: string;
	    updatedDate: string;
	
	    static createFrom(source: any = {}) {
	        return new TdxFinanceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.market = source["market"];
	        this.code = source["code"];
	        this.floatShares = source["floatShares"];
	        this.totalShares = source["totalShares"];
	        this.eps = source["eps"];
	        this.totalAssets = source["totalAssets"];
	        this.currentAssets = source["currentAssets"];
	        this.fixedAssets = source["fixedAssets"];
	        this.intangibleAssets = source["intangibleAssets"];
	        this.shareholderCount = source["shareholderCount"];
	        this.currentLiabilities = source["currentLiabilities"];
	        this.longTermLiabilities = source["longTermLiabilities"];
	        this.capitalReserve = source["capitalReserve"];
	        this.totalEquity = source["totalEquity"];
	        this.operatingRevenue = source["operatingRevenue"];
	        this.operatingCost = source["operatingCost"];
	        this.accountsReceivable = source["accountsReceivable"];
	        this.operatingProfit = source["operatingProfit"];
	        this.investmentIncome = source["investmentIncome"];
	        this.netCashFlow = source["netCashFlow"];
	        this.inventory = source["inventory"];
	        this.totalProfit = source["totalProfit"];
	        this.afterTaxProfit = source["afterTaxProfit"];
	        this.netProfit = source["netProfit"];
	        this.undistributedProfit = source["undistributedProfit"];
	        this.netAssetsPerShare = source["netAssetsPerShare"];
	        this.ipoDate = source["ipoDate"];
	        this.updatedDate = source["updatedDate"];
	    }
	}
	export class TdxXDXRItem {
	    date: string;
	    category: number;
	    name: string;
	    fenhong?: number;
	    peigujia?: number;
	    songzhuangu?: number;
	    peigu?: number;
	    suogu?: number;
	    preFloatShares?: number;
	    preTotalShares?: number;
	    postFloatShares?: number;
	    postTotalShares?: number;
	
	    static createFrom(source: any = {}) {
	        return new TdxXDXRItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.category = source["category"];
	        this.name = source["name"];
	        this.fenhong = source["fenhong"];
	        this.peigujia = source["peigujia"];
	        this.songzhuangu = source["songzhuangu"];
	        this.peigu = source["peigu"];
	        this.suogu = source["suogu"];
	        this.preFloatShares = source["preFloatShares"];
	        this.preTotalShares = source["preTotalShares"];
	        this.postFloatShares = source["postFloatShares"];
	        this.postTotalShares = source["postTotalShares"];
	    }
	}
	export class TdxCompanyInfoSection {
	    name: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new TdxCompanyInfoSection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.content = source["content"];
	    }
	}
	export class TdxCompanyInfoBundle {
	    sections: TdxCompanyInfoSection[];
	    xdxr: TdxXDXRItem[];
	    finance?: TdxFinanceInfo;
	
	    static createFrom(source: any = {}) {
	        return new TdxCompanyInfoBundle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sections = this.convertValues(source["sections"], TdxCompanyInfoSection);
	        this.xdxr = this.convertValues(source["xdxr"], TdxXDXRItem);
	        this.finance = this.convertValues(source["finance"], TdxFinanceInfo);
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
	
	
	export class TdxMinuteTimeData {
	    time: string;
	    price: number;
	    avg: number;
	    vol: number;
	
	    static createFrom(source: any = {}) {
	        return new TdxMinuteTimeData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.price = source["price"];
	        this.avg = source["avg"];
	        this.vol = source["vol"];
	    }
	}
	export class TdxMinuteTimeDataBundle {
	    stockCode: string;
	    date: string;
	    preClose: number;
	    open: number;
	    high: number;
	    low: number;
	    close: number;
	    vol: number;
	    amount: number;
	    items: TdxMinuteTimeData[];
	
	    static createFrom(source: any = {}) {
	        return new TdxMinuteTimeDataBundle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.date = source["date"];
	        this.preClose = source["preClose"];
	        this.open = source["open"];
	        this.high = source["high"];
	        this.low = source["low"];
	        this.close = source["close"];
	        this.vol = source["vol"];
	        this.amount = source["amount"];
	        this.items = this.convertValues(source["items"], TdxMinuteTimeData);
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
	
	export class TradingRecord {
	    ID: number;
	    StockCode: string;
	    StockName: string;
	    Direction: string;
	    Price: number;
	    Volume: number;
	    Amount: number;
	    // Go type: time
	    TradingTime: any;
	    Reason: string;
	    StopLossPrice: number;
	    TakeProfitPrice: number;
	    Fee: number;
	    MarketValue: number;
	    Mindset: string;
	    recordedClosePrice: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.StockCode = source["StockCode"];
	        this.StockName = source["StockName"];
	        this.Direction = source["Direction"];
	        this.Price = source["Price"];
	        this.Volume = source["Volume"];
	        this.Amount = source["Amount"];
	        this.TradingTime = this.convertValues(source["TradingTime"], null);
	        this.Reason = source["Reason"];
	        this.StopLossPrice = source["StopLossPrice"];
	        this.TakeProfitPrice = source["TakeProfitPrice"];
	        this.Fee = source["Fee"];
	        this.MarketValue = source["MarketValue"];
	        this.Mindset = source["Mindset"];
	        this.recordedClosePrice = source["recordedClosePrice"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
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
	export class TradingRecordImportResult {
	    total: number;
	    imported: number;
	    skipped: number;
	    failed: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.imported = source["imported"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.message = source["message"];
	    }
	}
	export class TradingRecordItem {
	    ID: number;
	    StockCode: string;
	    StockName: string;
	    Direction: string;
	    Price: number;
	    Volume: number;
	    Amount: number;
	    // Go type: time
	    TradingTime: any;
	    Reason: string;
	    StopLossPrice: number;
	    TakeProfitPrice: number;
	    Fee: number;
	    MarketValue: number;
	    Mindset: string;
	    recordedClosePrice: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    closePrice: number;
	    profitAmount: number;
	    profitPercent: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.StockCode = source["StockCode"];
	        this.StockName = source["StockName"];
	        this.Direction = source["Direction"];
	        this.Price = source["Price"];
	        this.Volume = source["Volume"];
	        this.Amount = source["Amount"];
	        this.TradingTime = this.convertValues(source["TradingTime"], null);
	        this.Reason = source["Reason"];
	        this.StopLossPrice = source["StopLossPrice"];
	        this.TakeProfitPrice = source["TakeProfitPrice"];
	        this.Fee = source["Fee"];
	        this.MarketValue = source["MarketValue"];
	        this.Mindset = source["Mindset"];
	        this.recordedClosePrice = source["recordedClosePrice"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.closePrice = source["closePrice"];
	        this.profitAmount = source["profitAmount"];
	        this.profitPercent = source["profitPercent"];
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
	export class TradingRecordListQuery {
	    page: number;
	    pageSize: number;
	    keyword: string;
	    direction: string;
	    startDate: string;
	    endDate: string;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordListQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.keyword = source["keyword"];
	        this.direction = source["direction"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	    }
	}
	export class TradingRecordPageData {
	    list: TradingRecordItem[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], TradingRecordItem);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class TradingRecordStatistics {
	    totalBuyAmount: number;
	    totalSellAmount: number;
	    totalProfit: number;
	    profitRate: number;
	    holdingsAmount: number;
	    currentValue: number;
	    stockCount: number;
	    todayBuyAmount: number;
	    todaySellAmount: number;
	    todayRealizedProfit: number;
	    todayFloatingProfit: number;
	    todayProfit: number;
	    todayProfitRate: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordStatistics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalBuyAmount = source["totalBuyAmount"];
	        this.totalSellAmount = source["totalSellAmount"];
	        this.totalProfit = source["totalProfit"];
	        this.profitRate = source["profitRate"];
	        this.holdingsAmount = source["holdingsAmount"];
	        this.currentValue = source["currentValue"];
	        this.stockCount = source["stockCount"];
	        this.todayBuyAmount = source["todayBuyAmount"];
	        this.todaySellAmount = source["todaySellAmount"];
	        this.todayRealizedProfit = source["todayRealizedProfit"];
	        this.todayFloatingProfit = source["todayFloatingProfit"];
	        this.todayProfit = source["todayProfit"];
	        this.todayProfitRate = source["todayProfitRate"];
	    }
	}
	export class TypeCountStats {
	    typeName: string;
	    upCount: number;
	    downCount: number;
	    totalCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TypeCountStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.typeName = source["typeName"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.totalCount = source["totalCount"];
	    }
	}

}

export namespace lo {
	
	export class Tuple2_string_string_ {
	    A: string;
	    B: string;
	
	    static createFrom(source: any = {}) {
	        return new Tuple2_string_string_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.A = source["A"];
	        this.B = source["B"];
	    }
	}

}

export namespace main {
	
	export class AiModelInfo {
	    modelName: string;
	    maxTokens: number;
	    contextWindow: number;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new AiModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modelName = source["modelName"];
	        this.maxTokens = source["maxTokens"];
	        this.contextWindow = source["contextWindow"];
	        this.source = source["source"];
	    }
	}
	export class FilesystemSkillInfo {
	    name: string;
	    description: string;
	    dirName: string;
	
	    static createFrom(source: any = {}) {
	        return new FilesystemSkillInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.dirName = source["dirName"];
	    }
	}
	export class SkillFileInfo {
	    name: string;
	    path: string;
	    isDir: boolean;
	    size: number;
	    modTime: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	    }
	}

}

export namespace models {
	
	export class AIResponseResult {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    chatId: string;
	    modelName: string;
	    stockCode: string;
	    stockName: string;
	    question: string;
	    content: string;
	    IsDel: number;
	
	    static createFrom(source: any = {}) {
	        return new AIResponseResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.chatId = source["chatId"];
	        this.modelName = source["modelName"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.question = source["question"];
	        this.content = source["content"];
	        this.IsDel = source["IsDel"];
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
	export class AIResponseResultPageData {
	    list: AIResponseResult[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new AIResponseResultPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], AIResponseResult);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class AIResponseResultQuery {
	    page: number;
	    pageSize: number;
	    chatId: string;
	    modelName: string;
	    stockCode: string;
	    stockName: string;
	    question: string;
	    startDate: string;
	    endDate: string;
	
	    static createFrom(source: any = {}) {
	        return new AIResponseResultQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.chatId = source["chatId"];
	        this.modelName = source["modelName"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.question = source["question"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	    }
	}
	export class AgentFeedback {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    userKey: string;
	    sessionId: string;
	    question: string;
	    response: string;
	    rating: number;
	    reason: string;
	    mode: string;
	    // Go type: time
	    feedbackAt: any;
	    processed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AgentFeedback(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.userKey = source["userKey"];
	        this.sessionId = source["sessionId"];
	        this.question = source["question"];
	        this.response = source["response"];
	        this.rating = source["rating"];
	        this.reason = source["reason"];
	        this.mode = source["mode"];
	        this.feedbackAt = this.convertValues(source["feedbackAt"], null);
	        this.processed = source["processed"];
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
	export class AiAssistantMessage {
	    role: string;
	    content: string;
	    reasoning: string;
	    time: string;
	    modelName?: string;
	    toolCalls?: number[];
	    toolResults?: number[];
	    timeline?: number[];
	    steps?: string[];
	    jsonMarkdown?: string;
	
	    static createFrom(source: any = {}) {
	        return new AiAssistantMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.reasoning = source["reasoning"];
	        this.time = source["time"];
	        this.modelName = source["modelName"];
	        this.toolCalls = source["toolCalls"];
	        this.toolResults = source["toolResults"];
	        this.timeline = source["timeline"];
	        this.steps = source["steps"];
	        this.jsonMarkdown = source["jsonMarkdown"];
	    }
	}
	export class AiAssistantSessionResp {
	    messages: AiAssistantMessage[];
	    sessionId: string;
	
	    static createFrom(source: any = {}) {
	        return new AiAssistantSessionResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], AiAssistantMessage);
	        this.sessionId = source["sessionId"];
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
	export class AiRecommendStocks {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    // Go type: time
	    dataTime?: any;
	    modelName: string;
	    rating: string;
	    stockCode: string;
	    stockName: string;
	    bkCode: string;
	    bkName: string;
	    stockPrice: string;
	    stockCurrentPrice: string;
	    stockCurrentPriceTime: string;
	    stockClosePrice: string;
	    stockPrePrice: string;
	    recommendReason: string;
	    recommendBuyPrice: string;
	    recommendBuyPriceMin: number;
	    recommendBuyPriceMax: number;
	    recommendStopProfitPrice: string;
	    recommendStopProfitPriceMin: number;
	    recommendStopProfitPriceMax: number;
	    recommendStopLossPrice: string;
	    riskRemarks: string;
	    remarks: string;
	    systemPrompt: string;
	    userPrompt: string;
	    enableAlert: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AiRecommendStocks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.dataTime = this.convertValues(source["dataTime"], null);
	        this.modelName = source["modelName"];
	        this.rating = source["rating"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.bkCode = source["bkCode"];
	        this.bkName = source["bkName"];
	        this.stockPrice = source["stockPrice"];
	        this.stockCurrentPrice = source["stockCurrentPrice"];
	        this.stockCurrentPriceTime = source["stockCurrentPriceTime"];
	        this.stockClosePrice = source["stockClosePrice"];
	        this.stockPrePrice = source["stockPrePrice"];
	        this.recommendReason = source["recommendReason"];
	        this.recommendBuyPrice = source["recommendBuyPrice"];
	        this.recommendBuyPriceMin = source["recommendBuyPriceMin"];
	        this.recommendBuyPriceMax = source["recommendBuyPriceMax"];
	        this.recommendStopProfitPrice = source["recommendStopProfitPrice"];
	        this.recommendStopProfitPriceMin = source["recommendStopProfitPriceMin"];
	        this.recommendStopProfitPriceMax = source["recommendStopProfitPriceMax"];
	        this.recommendStopLossPrice = source["recommendStopLossPrice"];
	        this.riskRemarks = source["riskRemarks"];
	        this.remarks = source["remarks"];
	        this.systemPrompt = source["systemPrompt"];
	        this.userPrompt = source["userPrompt"];
	        this.enableAlert = source["enableAlert"];
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
	export class AiRecommendStocksPageData {
	    list: AiRecommendStocks[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new AiRecommendStocksPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], AiRecommendStocks);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class AiRecommendStocksQuery {
	    page: number;
	    pageSize: number;
	    modelName: string;
	    stockCode: string;
	    stockName: string;
	    bkCode: string;
	    bkName: string;
	    startDate: string;
	    endDate: string;
	    enableAlert?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AiRecommendStocksQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.modelName = source["modelName"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.bkCode = source["bkCode"];
	        this.bkName = source["bkName"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.enableAlert = source["enableAlert"];
	    }
	}
	export class AllStockInfo {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    SECUCODE: string;
	    SECURITY_CODE: string;
	    SECURITY_NAME_ABBR: string;
	    NEW_PRICE: string;
	    CHANGE_RATE: string;
	    VOLUME_RATIO: string;
	    HIGH_PRICE: string;
	    LOW_PRICE: string;
	    PRE_CLOSE_PRICE: string;
	    VOLUME: string;
	    DEAL_AMOUNT: string;
	    TURNOVERRATE: string;
	    MARKET: string;
	    CONCEPT: string;
	    INDUSTRY: string;
	    MAX_TRADE_DATE: string;
	
	    static createFrom(source: any = {}) {
	        return new AllStockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.SECUCODE = source["SECUCODE"];
	        this.SECURITY_CODE = source["SECURITY_CODE"];
	        this.SECURITY_NAME_ABBR = source["SECURITY_NAME_ABBR"];
	        this.NEW_PRICE = source["NEW_PRICE"];
	        this.CHANGE_RATE = source["CHANGE_RATE"];
	        this.VOLUME_RATIO = source["VOLUME_RATIO"];
	        this.HIGH_PRICE = source["HIGH_PRICE"];
	        this.LOW_PRICE = source["LOW_PRICE"];
	        this.PRE_CLOSE_PRICE = source["PRE_CLOSE_PRICE"];
	        this.VOLUME = source["VOLUME"];
	        this.DEAL_AMOUNT = source["DEAL_AMOUNT"];
	        this.TURNOVERRATE = source["TURNOVERRATE"];
	        this.MARKET = source["MARKET"];
	        this.CONCEPT = source["CONCEPT"];
	        this.INDUSTRY = source["INDUSTRY"];
	        this.MAX_TRADE_DATE = source["MAX_TRADE_DATE"];
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
	export class AllStocksResp {
	    version: any;
	    // Go type: struct { Nextpage bool "json:\"nextpage\""; Currentpage int "json:\"currentpage\""; Data []models
	    result: any;
	    success: boolean;
	    message: string;
	    code: number;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new AllStocksResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.result = this.convertValues(source["result"], Object);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.code = source["code"];
	        this.url = source["url"];
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
	export class BKFundFlow {
	    id: number;
	    code: string;
	    name: string;
	    netInflow: number;
	    snapTime: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new BKFundFlow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.netInflow = source["netInflow"];
	        this.snapTime = source["snapTime"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class BKFundFlowPoint {
	    snapTime: string;
	    netInflow: number;
	
	    static createFrom(source: any = {}) {
	        return new BKFundFlowPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapTime = source["snapTime"];
	        this.netInflow = source["netInflow"];
	    }
	}
	export class ConceptStock {
	    code: string;
	    name: string;
	    price: string;
	    changePercent: string;
	    change: string;
	    speed: string;
	    turnover: string;
	    volumeRatio: string;
	    amplitude: string;
	    dealAmount: string;
	    flowShares: string;
	    flowMarketCap: string;
	    peRatio: string;
	
	    static createFrom(source: any = {}) {
	        return new ConceptStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.changePercent = source["changePercent"];
	        this.change = source["change"];
	        this.speed = source["speed"];
	        this.turnover = source["turnover"];
	        this.volumeRatio = source["volumeRatio"];
	        this.amplitude = source["amplitude"];
	        this.dealAmount = source["dealAmount"];
	        this.flowShares = source["flowShares"];
	        this.flowMarketCap = source["flowMarketCap"];
	        this.peRatio = source["peRatio"];
	    }
	}
	export class ConceptMarket {
	    open: string;
	    preClose: string;
	    low: string;
	    high: string;
	    volume: string;
	    changePercent: string;
	    changeRank: string;
	    upDownCount: string;
	    netInflow: string;
	    dealAmount: string;
	
	    static createFrom(source: any = {}) {
	        return new ConceptMarket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.open = source["open"];
	        this.preClose = source["preClose"];
	        this.low = source["low"];
	        this.high = source["high"];
	        this.volume = source["volume"];
	        this.changePercent = source["changePercent"];
	        this.changeRank = source["changeRank"];
	        this.upDownCount = source["upDownCount"];
	        this.netInflow = source["netInflow"];
	        this.dealAmount = source["dealAmount"];
	    }
	}
	export class ConceptDetailInfo {
	    conceptCode: string;
	    plateCode: string;
	    name: string;
	    definition: string;
	    market: ConceptMarket;
	    stocks: ConceptStock[];
	
	    static createFrom(source: any = {}) {
	        return new ConceptDetailInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.conceptCode = source["conceptCode"];
	        this.plateCode = source["plateCode"];
	        this.name = source["name"];
	        this.definition = source["definition"];
	        this.market = this.convertValues(source["market"], ConceptMarket);
	        this.stocks = this.convertValues(source["stocks"], ConceptStock);
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
	export class ConceptFundFlow {
	    id: number;
	    code: string;
	    name: string;
	    netInflow: number;
	    snapTime: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ConceptFundFlow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.netInflow = source["netInflow"];
	        this.snapTime = source["snapTime"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class ConceptFundFlowPoint {
	    snapTime: string;
	    netInflow: number;
	
	    static createFrom(source: any = {}) {
	        return new ConceptFundFlowPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapTime = source["snapTime"];
	        this.netInflow = source["netInflow"];
	    }
	}
	export class ConceptKLineItem {
	    date: string;
	    open: number;
	    close: number;
	    low: number;
	    high: number;
	    volume: number;
	
	    static createFrom(source: any = {}) {
	        return new ConceptKLineItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.open = source["open"];
	        this.close = source["close"];
	        this.low = source["low"];
	        this.high = source["high"];
	        this.volume = source["volume"];
	    }
	}
	export class ConceptKLineData {
	    name: string;
	    total: number;
	    start: string;
	    factor: number;
	    issuePrice: number;
	    kLines: ConceptKLineItem[];
	
	    static createFrom(source: any = {}) {
	        return new ConceptKLineData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.total = source["total"];
	        this.start = source["start"];
	        this.factor = source["factor"];
	        this.issuePrice = source["issuePrice"];
	        this.kLines = this.convertValues(source["kLines"], ConceptKLineItem);
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
	
	
	
	export class CronTask {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    cronExpr: string;
	    taskType: string;
	    target: string;
	    params: string;
	    enable: boolean;
	    // Go type: time
	    lastRunAt?: any;
	    // Go type: time
	    nextRunAt?: any;
	    runCount: number;
	    status: string;
	    description: string;
	    lastRunResult: string;
	
	    static createFrom(source: any = {}) {
	        return new CronTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.cronExpr = source["cronExpr"];
	        this.taskType = source["taskType"];
	        this.target = source["target"];
	        this.params = source["params"];
	        this.enable = source["enable"];
	        this.lastRunAt = this.convertValues(source["lastRunAt"], null);
	        this.nextRunAt = this.convertValues(source["nextRunAt"], null);
	        this.runCount = source["runCount"];
	        this.status = source["status"];
	        this.description = source["description"];
	        this.lastRunResult = source["lastRunResult"];
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
	export class CronTaskPageResp {
	    total: number;
	    data: CronTask[];
	
	    static createFrom(source: any = {}) {
	        return new CronTaskPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], CronTask);
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
	export class CronTaskQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    taskType: string;
	    status: string;
	    enable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CronTaskQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.taskType = source["taskType"];
	        this.status = source["status"];
	        this.enable = source["enable"];
	    }
	}
	export class CustomStrategy {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    query: string;
	    description: string;
	    sortOrder: number;
	
	    static createFrom(source: any = {}) {
	        return new CustomStrategy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.query = source["query"];
	        this.description = source["description"];
	        this.sortOrder = source["sortOrder"];
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
	export class CustomStrategyPageData {
	    list: CustomStrategy[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new CustomStrategyPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], CustomStrategy);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class CustomStrategyQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new CustomStrategyQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	    }
	}
	export class DailyOperationPlan {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    planDate: string;
	    planEndDate: string;
	    stockCode: string;
	    stockName: string;
	    overallJudgment: string;
	    scenarios: string;
	    discipline: string;
	    summary: string;
	    riskWarning: string;
	    status: string;
	    remarks: string;
	    enableAlert: boolean;
	    notifyChannels: string;
	
	    static createFrom(source: any = {}) {
	        return new DailyOperationPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.planDate = source["planDate"];
	        this.planEndDate = source["planEndDate"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.overallJudgment = source["overallJudgment"];
	        this.scenarios = source["scenarios"];
	        this.discipline = source["discipline"];
	        this.summary = source["summary"];
	        this.riskWarning = source["riskWarning"];
	        this.status = source["status"];
	        this.remarks = source["remarks"];
	        this.enableAlert = source["enableAlert"];
	        this.notifyChannels = source["notifyChannels"];
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
	export class DailyOperationPlanPageData {
	    list: DailyOperationPlan[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyOperationPlanPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], DailyOperationPlan);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class DailyOperationPlanQuery {
	    page: number;
	    pageSize: number;
	    stockCode: string;
	    stockName: string;
	    planDate: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new DailyOperationPlanQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.planDate = source["planDate"];
	        this.status = source["status"];
	    }
	}
	export class MCPServer {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    description: string;
	    url: string;
	    type: string;
	    headers: string;
	    command: string;
	    args: string;
	    enable: boolean;
	    status: string;
	    testResult: string;
	    authType: string;
	    authConfig: string;
	    // Go type: time
	    tokenExpireAt: any;
	
	    static createFrom(source: any = {}) {
	        return new MCPServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.url = source["url"];
	        this.type = source["type"];
	        this.headers = source["headers"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.enable = source["enable"];
	        this.status = source["status"];
	        this.testResult = source["testResult"];
	        this.authType = source["authType"];
	        this.authConfig = source["authConfig"];
	        this.tokenExpireAt = this.convertValues(source["tokenExpireAt"], null);
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
	export class MCPServerPageResp {
	    total: number;
	    data: MCPServer[];
	
	    static createFrom(source: any = {}) {
	        return new MCPServerPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], MCPServer);
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
	export class MCPServerQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    status: string;
	    enable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.enable = source["enable"];
	    }
	}
	export class MCPServerTool {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    mcpServerId: number;
	    toolName: string;
	    description: string;
	    paramsSchema: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPServerTool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.mcpServerId = source["mcpServerId"];
	        this.toolName = source["toolName"];
	        this.description = source["description"];
	        this.paramsSchema = source["paramsSchema"];
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
	export class MarketStatistic {
	    id: number;
	    dataDate: string;
	    dataTime: string;
	    upCount: number;
	    downCount: number;
	    upRatio: number;
	    upDownRatio: number;
	    sentimentDesc: string;
	    limitUp: number;
	    limitDown: number;
	    limitRatio: number;
	    shUpCount: number;
	    shDownCount: number;
	    szUpCount: number;
	    szDownCount: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new MarketStatistic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.dataDate = source["dataDate"];
	        this.dataTime = source["dataTime"];
	        this.upCount = source["upCount"];
	        this.downCount = source["downCount"];
	        this.upRatio = source["upRatio"];
	        this.upDownRatio = source["upDownRatio"];
	        this.sentimentDesc = source["sentimentDesc"];
	        this.limitUp = source["limitUp"];
	        this.limitDown = source["limitDown"];
	        this.limitRatio = source["limitRatio"];
	        this.shUpCount = source["shUpCount"];
	        this.shDownCount = source["shDownCount"];
	        this.szUpCount = source["szUpCount"];
	        this.szDownCount = source["szDownCount"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class Prompt {
	    ID: number;
	    name: string;
	    content: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new Prompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.type = source["type"];
	    }
	}
	export class PromptTemplate {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    name: string;
	    content: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new PromptTemplate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.name = source["name"];
	        this.content = source["content"];
	        this.type = source["type"];
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
	export class PromptTemplatePageData {
	    list: PromptTemplate[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new PromptTemplatePageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], PromptTemplate);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class PromptTemplateQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    type: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new PromptTemplateQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.content = source["content"];
	    }
	}
	export class RzrqRankItem {
	    stockCode: string;
	    stockName: string;
	    date: number;
	    lrye: string;
	    lryeRate: string;
	    rzye: string;
	    rzyeRate: string;
	    rqye: string;
	    rqyeRate: string;
	    jmr: string;
	    jmrRate: string;
	    rzmre: string;
	    rzche: string;
	    rzjmce: string;
	    yezf: string;
	    close_price: string;
	    close_profit: string;
	    marketId: string;
	
	    static createFrom(source: any = {}) {
	        return new RzrqRankItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.date = source["date"];
	        this.lrye = source["lrye"];
	        this.lryeRate = source["lryeRate"];
	        this.rzye = source["rzye"];
	        this.rzyeRate = source["rzyeRate"];
	        this.rqye = source["rqye"];
	        this.rqyeRate = source["rqyeRate"];
	        this.jmr = source["jmr"];
	        this.jmrRate = source["jmrRate"];
	        this.rzmre = source["rzmre"];
	        this.rzche = source["rzche"];
	        this.rzjmce = source["rzjmce"];
	        this.yezf = source["yezf"];
	        this.close_price = source["close_price"];
	        this.close_profit = source["close_profit"];
	        this.marketId = source["marketId"];
	    }
	}
	export class RzrqRankData {
	    type: string;
	    list: RzrqRankItem[];
	
	    static createFrom(source: any = {}) {
	        return new RzrqRankData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.list = this.convertValues(source["list"], RzrqRankItem);
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
	
	export class RzrqTrendItem {
	    date: string;
	    rzye: string;
	    rzjlr: string;
	    spj: string;
	    spzf: string;
	
	    static createFrom(source: any = {}) {
	        return new RzrqTrendItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.rzye = source["rzye"];
	        this.rzjlr = source["rzjlr"];
	        this.spj = source["spj"];
	        this.spzf = source["spzf"];
	    }
	}
	export class RzrqTrendData {
	    type: string;
	    code: string;
	    items: RzrqTrendItem[];
	    rzyeUnit: string;
	    rzjlrUnit: string;
	    spjUnit: string;
	    spzfUnit: string;
	    updateTime: string;
	
	    static createFrom(source: any = {}) {
	        return new RzrqTrendData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.code = source["code"];
	        this.items = this.convertValues(source["items"], RzrqTrendItem);
	        this.rzyeUnit = source["rzyeUnit"];
	        this.rzjlrUnit = source["rzjlrUnit"];
	        this.spjUnit = source["spjUnit"];
	        this.spzfUnit = source["spzfUnit"];
	        this.updateTime = source["updateTime"];
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
	
	export class SentimentResult {
	    Score: number;
	    Category: number;
	    PositiveCount: number;
	    NegativeCount: number;
	    Description: string;
	
	    static createFrom(source: any = {}) {
	        return new SentimentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Score = source["Score"];
	        this.Category = source["Category"];
	        this.PositiveCount = source["PositiveCount"];
	        this.NegativeCount = source["NegativeCount"];
	        this.Description = source["Description"];
	    }
	}
	export class Skill {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    description: string;
	    category: string;
	    systemPrompt: string;
	    examples: string;
	    triggerKeywords: string;
	    mcpServerIds: string;
	    enable: boolean;
	    sortOrder: number;
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.systemPrompt = source["systemPrompt"];
	        this.examples = source["examples"];
	        this.triggerKeywords = source["triggerKeywords"];
	        this.mcpServerIds = source["mcpServerIds"];
	        this.enable = source["enable"];
	        this.sortOrder = source["sortOrder"];
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
	export class SkillPageResp {
	    total: number;
	    data: Skill[];
	
	    static createFrom(source: any = {}) {
	        return new SkillPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], Skill);
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
	export class SkillQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    category: string;
	    enable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SkillQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.enable = source["enable"];
	    }
	}
	export class StockChangeHistory {
	    id: number;
	    changeTime: string;
	    changeDate: string;
	    stockCode: string;
	    stockName: string;
	    market: number;
	    changeType: number;
	    typeName: string;
	    volume: number;
	    price: number;
	    changeRate: number;
	    amount: number;
	    industry: string;
	    concept: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.changeTime = source["changeTime"];
	        this.changeDate = source["changeDate"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.market = source["market"];
	        this.changeType = source["changeType"];
	        this.typeName = source["typeName"];
	        this.volume = source["volume"];
	        this.price = source["price"];
	        this.changeRate = source["changeRate"];
	        this.amount = source["amount"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class StockChangeHistoryPageData {
	    list: StockChangeHistory[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeHistoryPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], StockChangeHistory);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
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
	export class StockChangeHistoryQuery {
	    stockCode: string;
	    stockName: string;
	    changeType: number;
	    changeTypes: number[];
	    typeName: string;
	    startDate: string;
	    endDate: string;
	    startTime: string;
	    endTime: string;
	    minVolume: number;
	    minAmount: number;
	    minChangeRate: number;
	    maxChangeRate: number;
	    industry: string;
	    concept: string;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeHistoryQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.changeType = source["changeType"];
	        this.changeTypes = source["changeTypes"];
	        this.typeName = source["typeName"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.minVolume = source["minVolume"];
	        this.minAmount = source["minAmount"];
	        this.minChangeRate = source["minChangeRate"];
	        this.maxChangeRate = source["maxChangeRate"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}
	export class StockInfo {
	    SECUCODE: string;
	    SECURITY_CODE: string;
	    SECURITY_NAME_ABBR: string;
	    NEW_PRICE: any;
	    CHANGE_RATE: any;
	    VOLUME_RATIO: any;
	    HIGH_PRICE: any;
	    LOW_PRICE: any;
	    PRE_CLOSE_PRICE: any;
	    VOLUME: any;
	    DEAL_AMOUNT: any;
	    TURNOVERRATE: any;
	    MARKET: string;
	    CONCEPT: any;
	    INDUSTRY: string;
	    MAX_TRADE_DATE: string;
	
	    static createFrom(source: any = {}) {
	        return new StockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SECUCODE = source["SECUCODE"];
	        this.SECURITY_CODE = source["SECURITY_CODE"];
	        this.SECURITY_NAME_ABBR = source["SECURITY_NAME_ABBR"];
	        this.NEW_PRICE = source["NEW_PRICE"];
	        this.CHANGE_RATE = source["CHANGE_RATE"];
	        this.VOLUME_RATIO = source["VOLUME_RATIO"];
	        this.HIGH_PRICE = source["HIGH_PRICE"];
	        this.LOW_PRICE = source["LOW_PRICE"];
	        this.PRE_CLOSE_PRICE = source["PRE_CLOSE_PRICE"];
	        this.VOLUME = source["VOLUME"];
	        this.DEAL_AMOUNT = source["DEAL_AMOUNT"];
	        this.TURNOVERRATE = source["TURNOVERRATE"];
	        this.MARKET = source["MARKET"];
	        this.CONCEPT = source["CONCEPT"];
	        this.INDUSTRY = source["INDUSTRY"];
	        this.MAX_TRADE_DATE = source["MAX_TRADE_DATE"];
	    }
	}
	export class TechnicalIndicators {
	    MACD_GOLDEN_FORK: boolean;
	    KDJ_GOLDEN_FORK: boolean;
	    BREAK_THROUGH: boolean;
	    LOW_FUNDS_INFLOW: boolean;
	    HIGH_FUNDS_OUTFLOW: boolean;
	    BREAKUP_MA_5DAYS: boolean;
	    LONG_AVG_ARRAY: boolean;
	    SHORT_AVG_ARRAY: boolean;
	    UPPER_LARGE_VOLUME: boolean;
	    DOWN_NARROW_VOLUME: boolean;
	    ONE_DAYANG_LINE: boolean;
	    TWO_DAYANG_LINES: boolean;
	    RISE_SUN: boolean;
	    POWER_FULGUN: boolean;
	    RESTORE_JUSTICE: boolean;
	    DOWN_7DAYS: boolean;
	    UPPER_8DAYS: boolean;
	    UPPER_9DAYS: boolean;
	    UPPER_4DAYS: boolean;
	    HEAVEN_RULE: boolean;
	    UPSIDE_VOLUME: boolean;
	    BEARISH_ENGULFING: boolean;
	    REVERSING_HAMMER: boolean;
	    SHOOTING_STAR: boolean;
	    EVENING_STAR: boolean;
	    FIRST_DAWN: boolean;
	    PREGNANT: boolean;
	    BLACK_CLOUD_TOPS: boolean;
	    MORNING_STAR: boolean;
	    NARROW_FINISH: boolean;
	    UPP_DAYS: number;
	    CONCERN_RANK_7DAYS: number;
	    UPNDAY: number;
	    DOWNNDAY: number;
	
	    static createFrom(source: any = {}) {
	        return new TechnicalIndicators(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MACD_GOLDEN_FORK = source["MACD_GOLDEN_FORK"];
	        this.KDJ_GOLDEN_FORK = source["KDJ_GOLDEN_FORK"];
	        this.BREAK_THROUGH = source["BREAK_THROUGH"];
	        this.LOW_FUNDS_INFLOW = source["LOW_FUNDS_INFLOW"];
	        this.HIGH_FUNDS_OUTFLOW = source["HIGH_FUNDS_OUTFLOW"];
	        this.BREAKUP_MA_5DAYS = source["BREAKUP_MA_5DAYS"];
	        this.LONG_AVG_ARRAY = source["LONG_AVG_ARRAY"];
	        this.SHORT_AVG_ARRAY = source["SHORT_AVG_ARRAY"];
	        this.UPPER_LARGE_VOLUME = source["UPPER_LARGE_VOLUME"];
	        this.DOWN_NARROW_VOLUME = source["DOWN_NARROW_VOLUME"];
	        this.ONE_DAYANG_LINE = source["ONE_DAYANG_LINE"];
	        this.TWO_DAYANG_LINES = source["TWO_DAYANG_LINES"];
	        this.RISE_SUN = source["RISE_SUN"];
	        this.POWER_FULGUN = source["POWER_FULGUN"];
	        this.RESTORE_JUSTICE = source["RESTORE_JUSTICE"];
	        this.DOWN_7DAYS = source["DOWN_7DAYS"];
	        this.UPPER_8DAYS = source["UPPER_8DAYS"];
	        this.UPPER_9DAYS = source["UPPER_9DAYS"];
	        this.UPPER_4DAYS = source["UPPER_4DAYS"];
	        this.HEAVEN_RULE = source["HEAVEN_RULE"];
	        this.UPSIDE_VOLUME = source["UPSIDE_VOLUME"];
	        this.BEARISH_ENGULFING = source["BEARISH_ENGULFING"];
	        this.REVERSING_HAMMER = source["REVERSING_HAMMER"];
	        this.SHOOTING_STAR = source["SHOOTING_STAR"];
	        this.EVENING_STAR = source["EVENING_STAR"];
	        this.FIRST_DAWN = source["FIRST_DAWN"];
	        this.PREGNANT = source["PREGNANT"];
	        this.BLACK_CLOUD_TOPS = source["BLACK_CLOUD_TOPS"];
	        this.MORNING_STAR = source["MORNING_STAR"];
	        this.NARROW_FINISH = source["NARROW_FINISH"];
	        this.UPP_DAYS = source["UPP_DAYS"];
	        this.CONCERN_RANK_7DAYS = source["CONCERN_RANK_7DAYS"];
	        this.UPNDAY = source["UPNDAY"];
	        this.DOWNNDAY = source["DOWNNDAY"];
	    }
	}
	export class VersionInfo {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    version: string;
	    content: string;
	    icon: string;
	    alipay: string;
	    wxpay: string;
	    wxgzh: string;
	    buildTimeStamp: number;
	    officialStatement: string;
	    IsDel: number;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.version = source["version"];
	        this.content = source["content"];
	        this.icon = source["icon"];
	        this.alipay = source["alipay"];
	        this.wxpay = source["wxpay"];
	        this.wxgzh = source["wxgzh"];
	        this.buildTimeStamp = source["buildTimeStamp"];
	        this.officialStatement = source["officialStatement"];
	        this.IsDel = source["IsDel"];
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

