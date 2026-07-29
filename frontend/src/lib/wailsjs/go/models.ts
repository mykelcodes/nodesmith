export namespace services {
	
	export class HistoryEntry {
	    id: string;
	    recipeId: string;
	    recipeName: string;
	    projectName: string;
	    projectDir: string;
	    packageManager: string;
	    state: string;
	    planHash: string;
	    durationMs: number;
	    // Go type: time
	    createdAt: any;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.recipeId = source["recipeId"];
	        this.recipeName = source["recipeName"];
	        this.projectName = source["projectName"];
	        this.projectDir = source["projectDir"];
	        this.packageManager = source["packageManager"];
	        this.state = source["state"];
	        this.planHash = source["planHash"];
	        this.durationMs = source["durationMs"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class Job {
	    id: string;
	    state: string;
	    stepIndex: number;
	    stepCount: number;
	    exitCode: number;
	    projectDir: string;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    endedAt: any;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.state = source["state"];
	        this.stepIndex = source["stepIndex"];
	        this.stepCount = source["stepCount"];
	        this.exitCode = source["exitCode"];
	        this.projectDir = source["projectDir"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.endedAt = this.convertValues(source["endedAt"], null);
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
	export class LogLine {
	    seq: number;
	    stream: string;
	    text: string;
	    stepId: string;
	
	    static createFrom(source: any = {}) {
	        return new LogLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seq = source["seq"];
	        this.stream = source["stream"];
	        this.text = source["text"];
	        this.stepId = source["stepId"];
	    }
	}
	export class ProjectConfig {
	    path: string;
	    format: string;
	    section: string;
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.format = source["format"];
	        this.section = source["section"];
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class PlanStep {
	    id: string;
	    kind: string;
	    label: string;
	    bin: string;
	    args: string[];
	    dir: string;
	    env: Record<string, string>;
	    display: string;
	    config?: ProjectConfig;
	
	    static createFrom(source: any = {}) {
	        return new PlanStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.label = source["label"];
	        this.bin = source["bin"];
	        this.args = source["args"];
	        this.dir = source["dir"];
	        this.env = source["env"];
	        this.display = source["display"];
	        this.config = this.convertValues(source["config"], ProjectConfig);
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
	export class Plan {
	    recipeId: string;
	    projectDir: string;
	    steps: PlanStep[];
	    warnings: string[];
	    hash: string;
	
	    static createFrom(source: any = {}) {
	        return new Plan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recipeId = source["recipeId"];
	        this.projectDir = source["projectDir"];
	        this.steps = this.convertValues(source["steps"], PlanStep);
	        this.warnings = source["warnings"];
	        this.hash = source["hash"];
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
	
	export class ScaffoldRequest {
	    recipeId: string;
	    projectName: string;
	    parentDir: string;
	    packageManager: string;
	    installDeps: boolean;
	    gitInit: boolean;
	    minimumReleaseAge?: number;
	    answers: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ScaffoldRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recipeId = source["recipeId"];
	        this.projectName = source["projectName"];
	        this.parentDir = source["parentDir"];
	        this.packageManager = source["packageManager"];
	        this.installDeps = source["installDeps"];
	        this.gitInit = source["gitInit"];
	        this.minimumReleaseAge = source["minimumReleaseAge"];
	        this.answers = source["answers"];
	    }
	}
	export class Preset {
	    id: string;
	    name: string;
	    request: ScaffoldRequest;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Preset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.request = this.convertValues(source["request"], ScaffoldRequest);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	
	export class RecipeStep {
	    id: string;
	    label: string;
	    bin: string;
	    cwd: string;
	    env: Record<string, string>;
	    args: any[];
	    when: string;
	
	    static createFrom(source: any = {}) {
	        return new RecipeStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.bin = source["bin"];
	        this.cwd = source["cwd"];
	        this.env = source["env"];
	        this.args = source["args"];
	        this.when = source["when"];
	    }
	}
	export class RecipeOption {
	    value: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new RecipeOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	    }
	}
	export class RecipeField {
	    id: string;
	    label: string;
	    type: string;
	    default: any;
	    help: string;
	    options: RecipeOption[];
	    visibleIf: string;
	
	    static createFrom(source: any = {}) {
	        return new RecipeField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.type = source["type"];
	        this.default = source["default"];
	        this.help = source["help"];
	        this.options = this.convertValues(source["options"], RecipeOption);
	        this.visibleIf = source["visibleIf"];
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
	export class RecipeRequirements {
	    node: string;
	    packageManagers: string[];
	    tools: string[];
	
	    static createFrom(source: any = {}) {
	        return new RecipeRequirements(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node = source["node"];
	        this.packageManagers = source["packageManagers"];
	        this.tools = source["tools"];
	    }
	}
	export class Recipe {
	    schemaVersion: number;
	    id: string;
	    name: string;
	    category: string;
	    description: string;
	    docsUrl: string;
	    tags: string[];
	    icon: string;
	    verifiedAt: string;
	    installPolicy: string;
	    minimumReleaseAge?: number;
	    requires: RecipeRequirements;
	    fields: RecipeField[];
	    steps: RecipeStep[];
	    available: boolean;
	    unavailableReasons: string[];
	
	    static createFrom(source: any = {}) {
	        return new Recipe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.description = source["description"];
	        this.docsUrl = source["docsUrl"];
	        this.tags = source["tags"];
	        this.icon = source["icon"];
	        this.verifiedAt = source["verifiedAt"];
	        this.installPolicy = source["installPolicy"];
	        this.minimumReleaseAge = source["minimumReleaseAge"];
	        this.requires = this.convertValues(source["requires"], RecipeRequirements);
	        this.fields = this.convertValues(source["fields"], RecipeField);
	        this.steps = this.convertValues(source["steps"], RecipeStep);
	        this.available = source["available"];
	        this.unavailableReasons = source["unavailableReasons"];
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
	
	
	
	
	export class RecipeSummary {
	    id: string;
	    name: string;
	    category: string;
	    description: string;
	    docsUrl: string;
	    tags: string[];
	    icon: string;
	    verifiedAt: string;
	    installPolicy: string;
	    minimumReleaseAge?: number;
	    available: boolean;
	    unavailableReasons: string[];
	    defaultPackageManager: string;
	
	    static createFrom(source: any = {}) {
	        return new RecipeSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.description = source["description"];
	        this.docsUrl = source["docsUrl"];
	        this.tags = source["tags"];
	        this.icon = source["icon"];
	        this.verifiedAt = source["verifiedAt"];
	        this.installPolicy = source["installPolicy"];
	        this.minimumReleaseAge = source["minimumReleaseAge"];
	        this.available = source["available"];
	        this.unavailableReasons = source["unavailableReasons"];
	        this.defaultPackageManager = source["defaultPackageManager"];
	    }
	}
	export class ReloadResult {
	    count: number;
	    warnings: string[];
	    overrides: string[];
	
	    static createFrom(source: any = {}) {
	        return new ReloadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.warnings = source["warnings"];
	        this.overrides = source["overrides"];
	    }
	}
	
	export class Settings {
	    defaultParentDir: string;
	    pathOverride: string;
	    editor: string;
	    theme: string;
	    openAfterCreate: boolean;
	    minimumReleaseAge?: number;
	    recipeMinimumReleaseAge?: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultParentDir = source["defaultParentDir"];
	        this.pathOverride = source["pathOverride"];
	        this.editor = source["editor"];
	        this.theme = source["theme"];
	        this.openAfterCreate = source["openAfterCreate"];
	        this.minimumReleaseAge = source["minimumReleaseAge"];
	        this.recipeMinimumReleaseAge = source["recipeMinimumReleaseAge"];
	    }
	}
	export class Tool {
	    name: string;
	    path: string;
	    version: string;
	    present: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new Tool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.present = source["present"];
	        this.error = source["error"];
	    }
	}
	export class Toolchain {
	    path: string;
	    // Go type: time
	    detectedAt: any;
	    tools: Tool[];
	
	    static createFrom(source: any = {}) {
	        return new Toolchain(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.detectedAt = this.convertValues(source["detectedAt"], null);
	        this.tools = this.convertValues(source["tools"], Tool);
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
	export class ValidationResult {
	    valid: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.error = source["error"];
	    }
	}

}

