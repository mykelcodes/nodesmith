export type RecipeFieldType = 'select' | 'multiselect' | 'boolean' | 'text' | 'number';
export type InstallPolicy = 'optional' | 'required';
export type AnswerValue = string | number | boolean | string[];
export type Answers = Record<string, AnswerValue>;

export interface RecipeSummary {
	id: string;
	name: string;
	category: string;
	description: string;
	docsUrl: string;
	tags: string[];
	icon: string;
	verifiedAt: string;
	installPolicy: InstallPolicy;
	available: boolean;
	unavailableReasons: string[];
	defaultPackageManager: string;
}

export interface RecipeRequirements {
	node: string;
	packageManagers: string[];
	tools: string[];
}

export interface RecipeOption {
	value: string;
	label: string;
}

export type RecipeField =
	| {
			id: string;
			label: string;
			type: 'select' | 'text';
			default: string;
			help: string;
			options: RecipeOption[];
			visibleIf: string;
	  }
	| {
			id: string;
			label: string;
			type: 'multiselect';
			default: string[];
			help: string;
			options: RecipeOption[];
			visibleIf: string;
	  }
	| {
			id: string;
			label: string;
			type: 'boolean';
			default: boolean;
			help: string;
			options: RecipeOption[];
			visibleIf: string;
	  }
	| {
			id: string;
			label: string;
			type: 'number';
			default: number;
			help: string;
			options: RecipeOption[];
			visibleIf: string;
	  };

export interface ConditionalArg {
	if: string;
	then: RecipeArg[];
	else?: RecipeArg[];
}

export interface ForEachArg {
	forEach: string;
	args: RecipeArg[];
}

export type RecipeArg = string | ConditionalArg | ForEachArg;

export interface RecipeStep {
	id: string;
	label: string;
	bin: string;
	cwd: string;
	env: Record<string, string>;
	args: RecipeArg[];
	when: string;
}

export interface Recipe {
	schemaVersion: number;
	id: string;
	name: string;
	category: string;
	description: string;
	docsUrl: string;
	tags: string[];
	icon: string;
	verifiedAt: string;
	installPolicy: InstallPolicy;
	requires: RecipeRequirements;
	fields: RecipeField[];
	steps: RecipeStep[];
	available: boolean;
	unavailableReasons: string[];
}

export interface ReloadResult {
	count: number;
	warnings: string[];
	overrides: string[];
}

export interface ValidationResult {
	valid: boolean;
	error: string;
}

export interface Tool {
	name: string;
	path: string;
	version: string;
	present: boolean;
	error: string;
}

export interface Toolchain {
	path: string;
	detectedAt: string;
	tools: Tool[];
}

export interface ScaffoldRequest {
	recipeId: string;
	projectName: string;
	parentDir: string;
	packageManager: string;
	installDeps: boolean;
	gitInit: boolean;
	answers: Answers;
}

export interface PlanStep {
	id: string;
	label: string;
	bin: string;
	args: string[];
	dir: string;
	env: Record<string, string>;
	display: string;
}

export interface Plan {
	recipeId: string;
	projectDir: string;
	steps: PlanStep[];
	warnings: string[];
	hash: string;
}

export type JobState = 'pending' | 'running' | 'success' | 'failed' | 'cancelled';

export interface Job {
	id: string;
	state: JobState;
	stepIndex: number;
	stepCount: number;
	exitCode: number;
	projectDir: string;
	startedAt: string;
	endedAt: string;
	error: string;
}

export type LogStream = 'stdout' | 'stderr';

export interface LogLine {
	seq: number;
	stream: LogStream;
	text: string;
	stepId: string;
}

export type Theme = 'dark' | 'light' | 'system';
export type Editor = string;

export interface Settings {
	defaultParentDir: string;
	pathOverride: string;
	editor: Editor;
	theme: Theme;
	openAfterCreate: boolean;
}

export interface Preset {
	id: string;
	name: string;
	request: ScaffoldRequest;
	createdAt: string;
	updatedAt: string;
}

export interface HistoryEntry {
	id: string;
	recipeId: string;
	recipeName: string;
	projectName: string;
	projectDir: string;
	packageManager: string;
	state: string;
	planHash: string;
	durationMs: number;
	createdAt: string;
	error: string;
}

export interface JobStartedEvent {
	jobId: string;
	projectDir: string;
	stepCount: number;
}

export type JobStepState = 'running' | 'success' | 'failed' | 'skipped';

export interface JobStepEvent {
	jobId: string;
	stepId: string;
	index: number;
	total: number;
	state: JobStepState;
}

export interface JobLogEvent {
	jobId: string;
	seq: number;
	stepId: string;
	stream: LogStream;
	text: string;
}

export interface JobDoneEvent {
	jobId: string;
	state: JobState;
	exitCode: number;
	durationMs: number;
	projectDir: string;
	error: string;
}

export interface RecipesReloadedEvent {
	count: number;
	warnings: string[];
}

export interface NodesmithEventMap {
	'nodesmith:job:started': JobStartedEvent;
	'nodesmith:job:step': JobStepEvent;
	'nodesmith:job:log': JobLogEvent;
	'nodesmith:job:done': JobDoneEvent;
	'nodesmith:toolchain:changed': Toolchain;
	'nodesmith:recipes:reloaded': RecipesReloadedEvent;
}

export type NodesmithEventName = keyof NodesmithEventMap;
