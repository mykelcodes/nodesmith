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
	/** Recipe-authored package cooldown in minutes, or null to inherit the global preference. */
	minimumReleaseAge: number | null;
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

/**
 * Optional value constraints declared by a recipe field.
 *
 * These mirror the backend's `recipe.Field` so the form can reject a bad answer
 * before the bridge call. The planner enforces the same rules again and stays
 * authoritative — this is a better error message, not a security boundary.
 *
 * `null` means the constraint was not declared.
 */
export interface RecipeFieldConstraints {
	/** Requires an explicit answer instead of falling back to `default`. */
	required: boolean;
	/** RE2 source for a text answer. Empty when undeclared. Unanchored. */
	pattern: string;
	/** Text length bounds, counted in characters rather than bytes. */
	minLength: number | null;
	maxLength: number | null;
	/** Inclusive bounds for a number answer. */
	min: number | null;
	max: number | null;
}

interface RecipeFieldBase extends RecipeFieldConstraints {
	id: string;
	label: string;
	help: string;
	options: RecipeOption[];
	visibleIf: string;
}

export type RecipeField =
	| (RecipeFieldBase & { type: 'select' | 'text'; default: string })
	| (RecipeFieldBase & { type: 'multiselect'; default: string[] })
	| (RecipeFieldBase & { type: 'boolean'; default: boolean })
	| (RecipeFieldBase & { type: 'number'; default: number });

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
	/** Recipe-authored package cooldown in minutes, or null to inherit the global preference. */
	minimumReleaseAge: number | null;
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
	/**
	 * Set when login-shell PATH discovery failed and Nodesmith fell back to the
	 * PATH it was started with. This is the usual root cause of every tool
	 * appearing missing on a Finder-launched macOS app.
	 */
	pathWarning: string;
}

export interface ScaffoldRequest {
	recipeId: string;
	projectName: string;
	parentDir: string;
	packageManager: string;
	installDeps: boolean;
	gitInit: boolean;
	/** Request-specific package cooldown in minutes, or null to inherit the recipe/global default. */
	minimumReleaseAge: number | null;
	answers: Answers;
}

export type PlanStepKind = 'command' | 'project-config';

export interface ProjectConfig {
	path: string;
	format: 'properties' | 'toml' | 'yaml';
	section: string;
	key: string;
	value: string;
}

export interface PlanStep {
	id: string;
	kind: PlanStepKind;
	label: string;
	bin: string;
	args: string[];
	dir: string;
	env: Record<string, string>;
	display: string;
	config: ProjectConfig | null;
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
	/** Global package cooldown in minutes, or null to leave package managers on their own configuration. */
	minimumReleaseAge: number | null;
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
	overrides: string[];
}

export interface NodesmithEventMap {
	'nodesmith:job:started': JobStartedEvent;
	'nodesmith:job:step': JobStepEvent;
	'nodesmith:job:log': JobLogEvent;
	'nodesmith:job:done': JobDoneEvent;
	'nodesmith:toolchain:changed': Toolchain;
	'nodesmith:recipes:reloaded': RecipesReloadedEvent;
	'nodesmith:settings:changed': Settings;
}

export type NodesmithEventName = keyof NodesmithEventMap;
