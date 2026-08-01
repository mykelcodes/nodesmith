import type {
	Answers,
	HistoryEntry,
	InstallPolicy,
	Job,
	JobDoneEvent,
	JobLogEvent,
	JobStartedEvent,
	JobState,
	JobStepEvent,
	JobStepState,
	LogLine,
	LogStream,
	Plan,
	PlanStep,
	PlanStepKind,
	Preset,
	ProjectConfig,
	Recipe,
	RecipeArg,
	RecipeField,
	RecipeOption,
	RecipeRequirements,
	RecipeStep,
	RecipeSummary,
	RecipesReloadedEvent,
	ReloadResult,
	ScaffoldRequest,
	Settings,
	Theme,
	Tool,
	Toolchain,
	ValidationResult
} from './types';

type UnknownRecord = Record<string, unknown>;

function invalid(path: string, expected: string): never {
	throw new TypeError(`Invalid Nodesmith bridge response at ${path}: expected ${expected}.`);
}

function object(value: unknown, path: string): UnknownRecord {
	if (typeof value !== 'object' || value === null || Array.isArray(value)) {
		return invalid(path, 'an object');
	}
	return value as UnknownRecord;
}

function string(value: unknown, path: string): string {
	if (typeof value !== 'string') return invalid(path, 'a string');
	return value;
}

function number(value: unknown, path: string): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) {
		return invalid(path, 'a finite number');
	}
	return value;
}

function nullableNumber(value: unknown, path: string): number | null {
	if (value === null || value === undefined) return null;
	return number(value, path);
}

function integer(value: unknown, path: string): number {
	const result = number(value, path);
	if (!Number.isInteger(result)) return invalid(path, 'an integer');
	return result;
}

const MAX_RELEASE_AGE_MINUTES = 525600;

// Go encodes an unset *int as null, and the generated Wails model types it as
// optional, so both spellings mean "inherit".
function releaseAgeMinutes(value: unknown, path: string): number | null {
	if (value === null || value === undefined) return null;
	const result = integer(value, path);
	if (result < 0 || result > MAX_RELEASE_AGE_MINUTES) {
		return invalid(path, `an integer between 0 and ${MAX_RELEASE_AGE_MINUTES} minutes`);
	}
	return result;
}

function boolean(value: unknown, path: string): boolean {
	if (typeof value !== 'boolean') return invalid(path, 'a boolean');
	return value;
}

function array<T>(value: unknown, path: string, parse: (item: unknown, path: string) => T): T[] {
	// A nil Go slice encodes as null, which means "empty" on the bridge.
	if (value === null) return [];
	if (!Array.isArray(value)) return invalid(path, 'an array');
	return value.map((item, index) => parse(item, `${path}[${index}]`));
}

function stringArray(value: unknown, path: string): string[] {
	return array(value, path, string);
}

function stringRecord(value: unknown, path: string): Record<string, string> {
	if (value === null) return {};
	const source = object(value, path);
	return Object.fromEntries(
		Object.entries(source).map(([key, item]) => [key, string(item, `${path}.${key}`)])
	);
}

function oneOf<T extends string>(value: unknown, path: string, choices: readonly T[]): T {
	const result = string(value, path);
	if (!choices.some((choice) => choice === result)) {
		return invalid(path, choices.map((choice) => JSON.stringify(choice)).join(', '));
	}
	return result as T;
}

function isoTime(value: unknown, path: string): string {
	const result = string(value, path);
	if (Number.isNaN(Date.parse(result))) return invalid(path, 'an ISO-8601 timestamp');
	return result;
}

function newOrIsoTime(value: unknown, path: string): string {
	const result = string(value, path);
	return result === '' ? result : isoTime(result, path);
}

function recipeOption(value: unknown, path: string): RecipeOption {
	const source = object(value, path);
	return {
		value: string(source.value, `${path}.value`),
		label: string(source.label, `${path}.label`)
	};
}

function recipeField(value: unknown, path: string): RecipeField {
	const source = object(value, path);
	const common = {
		id: string(source.id, `${path}.id`),
		label: string(source.label, `${path}.label`),
		help: string(source.help, `${path}.help`),
		options: array(source.options, `${path}.options`, recipeOption),
		visibleIf: string(source.visibleIf, `${path}.visibleIf`),
		required: boolean(source.required, `${path}.required`),
		pattern: string(source.pattern, `${path}.pattern`),
		// Optional bounds arrive as JSON null when the recipe does not declare
		// them, because the Go side models them as pointers.
		minLength: nullableNumber(source.minLength, `${path}.minLength`),
		maxLength: nullableNumber(source.maxLength, `${path}.maxLength`),
		min: nullableNumber(source.min, `${path}.min`),
		max: nullableNumber(source.max, `${path}.max`)
	};
	const type = oneOf(source.type, `${path}.type`, [
		'select',
		'multiselect',
		'boolean',
		'text',
		'number'
	] as const);
	switch (type) {
		case 'select':
		case 'text':
			return { ...common, type, default: string(source.default, `${path}.default`) };
		case 'multiselect':
			return { ...common, type, default: stringArray(source.default, `${path}.default`) };
		case 'boolean':
			return { ...common, type, default: boolean(source.default, `${path}.default`) };
		case 'number':
			return { ...common, type, default: number(source.default, `${path}.default`) };
	}
}

function recipeArg(value: unknown, path: string): RecipeArg {
	if (typeof value === 'string') return value;
	const source = object(value, path);
	if (typeof source.if === 'string' && Array.isArray(source.then)) {
		const result: RecipeArg = {
			if: source.if,
			then: array(source.then, `${path}.then`, recipeArg)
		};
		if (source.else !== undefined) {
			result.else = array(source.else, `${path}.else`, recipeArg);
		}
		return result;
	}
	if (typeof source.forEach === 'string' && Array.isArray(source.args)) {
		return {
			forEach: source.forEach,
			args: array(source.args, `${path}.args`, recipeArg)
		};
	}
	return invalid(path, 'a literal, conditional, or forEach argument');
}

function recipeStep(value: unknown, path: string): RecipeStep {
	const source = object(value, path);
	return {
		id: string(source.id, `${path}.id`),
		label: string(source.label, `${path}.label`),
		bin: string(source.bin, `${path}.bin`),
		cwd: string(source.cwd, `${path}.cwd`),
		env: stringRecord(source.env, `${path}.env`),
		args: array(source.args, `${path}.args`, recipeArg),
		when: string(source.when, `${path}.when`)
	};
}

function recipeRequirements(value: unknown, path: string): RecipeRequirements {
	const source = object(value, path);
	return {
		node: string(source.node, `${path}.node`),
		packageManagers: stringArray(source.packageManagers, `${path}.packageManagers`),
		tools: stringArray(source.tools, `${path}.tools`)
	};
}

export function parseRecipeSummary(value: unknown, path = 'RecipeSummary'): RecipeSummary {
	const source = object(value, path);
	const installPolicies = ['optional', 'required'] as const;
	return {
		id: string(source.id, `${path}.id`),
		name: string(source.name, `${path}.name`),
		category: string(source.category, `${path}.category`),
		description: string(source.description, `${path}.description`),
		docsUrl: string(source.docsUrl, `${path}.docsUrl`),
		tags: stringArray(source.tags, `${path}.tags`),
		icon: string(source.icon, `${path}.icon`),
		verifiedAt: string(source.verifiedAt, `${path}.verifiedAt`),
		installPolicy: oneOf<InstallPolicy>(
			source.installPolicy,
			`${path}.installPolicy`,
			installPolicies
		),
		minimumReleaseAge: releaseAgeMinutes(source.minimumReleaseAge, `${path}.minimumReleaseAge`),
		available: boolean(source.available, `${path}.available`),
		unavailableReasons: stringArray(source.unavailableReasons, `${path}.unavailableReasons`),
		defaultPackageManager: string(source.defaultPackageManager, `${path}.defaultPackageManager`)
	};
}

export function parseRecipe(value: unknown, path = 'Recipe'): Recipe {
	const source = object(value, path);
	const installPolicies = ['optional', 'required'] as const;
	return {
		schemaVersion: integer(source.schemaVersion, `${path}.schemaVersion`),
		id: string(source.id, `${path}.id`),
		name: string(source.name, `${path}.name`),
		category: string(source.category, `${path}.category`),
		description: string(source.description, `${path}.description`),
		docsUrl: string(source.docsUrl, `${path}.docsUrl`),
		tags: stringArray(source.tags, `${path}.tags`),
		icon: string(source.icon, `${path}.icon`),
		verifiedAt: string(source.verifiedAt, `${path}.verifiedAt`),
		installPolicy: oneOf<InstallPolicy>(
			source.installPolicy,
			`${path}.installPolicy`,
			installPolicies
		),
		minimumReleaseAge: releaseAgeMinutes(source.minimumReleaseAge, `${path}.minimumReleaseAge`),
		requires: recipeRequirements(source.requires, `${path}.requires`),
		fields: array(source.fields, `${path}.fields`, recipeField),
		steps: array(source.steps, `${path}.steps`, recipeStep),
		available: boolean(source.available, `${path}.available`),
		unavailableReasons: stringArray(source.unavailableReasons, `${path}.unavailableReasons`)
	};
}

export function parseReloadResult(value: unknown, path = 'ReloadResult'): ReloadResult {
	const source = object(value, path);
	return {
		count: integer(source.count, `${path}.count`),
		warnings: stringArray(source.warnings, `${path}.warnings`),
		overrides: stringArray(source.overrides, `${path}.overrides`)
	};
}

export function parseValidationResult(value: unknown, path = 'ValidationResult'): ValidationResult {
	const source = object(value, path);
	return {
		valid: boolean(source.valid, `${path}.valid`),
		error: string(source.error, `${path}.error`)
	};
}

function parseTool(value: unknown, path: string): Tool {
	const source = object(value, path);
	return {
		name: string(source.name, `${path}.name`),
		path: string(source.path, `${path}.path`),
		version: string(source.version, `${path}.version`),
		present: boolean(source.present, `${path}.present`),
		error: string(source.error, `${path}.error`)
	};
}

export function parseToolchain(value: unknown, path = 'Toolchain'): Toolchain {
	const source = object(value, path);
	return {
		path: string(source.path, `${path}.path`),
		detectedAt: isoTime(source.detectedAt, `${path}.detectedAt`),
		tools: array(source.tools, `${path}.tools`, parseTool),
		pathWarning: string(source.pathWarning, `${path}.pathWarning`)
	};
}

function answer(value: unknown, path: string): string | number | boolean | string[] {
	if (typeof value === 'string' || typeof value === 'boolean') return value;
	if (typeof value === 'number' && Number.isFinite(value)) return value;
	if (Array.isArray(value)) return stringArray(value, path);
	return invalid(path, 'a string, finite number, boolean, or string array');
}

export function parseAnswers(value: unknown, path = 'Answers'): Answers {
	const source = object(value, path);
	return Object.fromEntries(
		Object.entries(source).map(([key, item]) => [key, answer(item, `${path}.${key}`)])
	);
}

export function parseScaffoldRequest(value: unknown, path = 'ScaffoldRequest'): ScaffoldRequest {
	const source = object(value, path);
	return {
		recipeId: string(source.recipeId, `${path}.recipeId`),
		projectName: string(source.projectName, `${path}.projectName`),
		parentDir: string(source.parentDir, `${path}.parentDir`),
		packageManager: string(source.packageManager, `${path}.packageManager`),
		installDeps: boolean(source.installDeps, `${path}.installDeps`),
		gitInit: boolean(source.gitInit, `${path}.gitInit`),
		minimumReleaseAge: releaseAgeMinutes(source.minimumReleaseAge, `${path}.minimumReleaseAge`),
		answers: parseAnswers(source.answers, `${path}.answers`)
	};
}

function parsePlanStep(value: unknown, path: string): PlanStep {
	const source = object(value, path);
	const kinds = ['command', 'project-config'] as const;
	return {
		id: string(source.id, `${path}.id`),
		kind:
			source.kind === undefined
				? 'command'
				: oneOf<PlanStepKind>(source.kind, `${path}.kind`, kinds),
		label: string(source.label, `${path}.label`),
		bin: string(source.bin, `${path}.bin`),
		args: stringArray(source.args, `${path}.args`),
		dir: string(source.dir, `${path}.dir`),
		env: stringRecord(source.env, `${path}.env`),
		display: string(source.display, `${path}.display`),
		config:
			source.config === null || source.config === undefined
				? null
				: parseProjectConfig(source.config, `${path}.config`)
	};
}

function parseProjectConfig(value: unknown, path: string): ProjectConfig {
	const source = object(value, path);
	return {
		path: string(source.path, `${path}.path`),
		format: oneOf(source.format, `${path}.format`, ['properties', 'toml', 'yaml'] as const),
		section: string(source.section, `${path}.section`),
		key: string(source.key, `${path}.key`),
		value: string(source.value, `${path}.value`)
	};
}

export function parsePlan(value: unknown, path = 'Plan'): Plan {
	const source = object(value, path);
	return {
		recipeId: string(source.recipeId, `${path}.recipeId`),
		projectDir: string(source.projectDir, `${path}.projectDir`),
		steps: array(source.steps, `${path}.steps`, parsePlanStep),
		warnings: stringArray(source.warnings, `${path}.warnings`),
		hash: string(source.hash, `${path}.hash`)
	};
}

const jobStates = ['pending', 'running', 'success', 'failed', 'cancelled'] as const;
const logStreams = ['stdout', 'stderr'] as const;

export function parseJob(value: unknown, path = 'Job'): Job {
	const source = object(value, path);
	return {
		id: string(source.id, `${path}.id`),
		state: oneOf<JobState>(source.state, `${path}.state`, jobStates),
		stepIndex: integer(source.stepIndex, `${path}.stepIndex`),
		stepCount: integer(source.stepCount, `${path}.stepCount`),
		exitCode: integer(source.exitCode, `${path}.exitCode`),
		projectDir: string(source.projectDir, `${path}.projectDir`),
		startedAt: isoTime(source.startedAt, `${path}.startedAt`),
		endedAt: isoTime(source.endedAt, `${path}.endedAt`),
		error: string(source.error, `${path}.error`)
	};
}

export function parseLogLine(value: unknown, path = 'LogLine'): LogLine {
	const source = object(value, path);
	return {
		seq: integer(source.seq, `${path}.seq`),
		stream: oneOf<LogStream>(source.stream, `${path}.stream`, logStreams),
		text: string(source.text, `${path}.text`),
		stepId: string(source.stepId, `${path}.stepId`)
	};
}

const themes = ['dark', 'light', 'system'] as const;

export function parseSettings(value: unknown, path = 'Settings'): Settings {
	const source = object(value, path);
	return {
		defaultParentDir: string(source.defaultParentDir, `${path}.defaultParentDir`),
		pathOverride: string(source.pathOverride, `${path}.pathOverride`),
		editor: string(source.editor, `${path}.editor`),
		theme: oneOf<Theme>(source.theme, `${path}.theme`, themes),
		openAfterCreate: boolean(source.openAfterCreate, `${path}.openAfterCreate`),
		minimumReleaseAge: releaseAgeMinutes(source.minimumReleaseAge, `${path}.minimumReleaseAge`)
	};
}

export function parsePreset(value: unknown, path = 'Preset'): Preset {
	const source = object(value, path);
	return {
		id: string(source.id, `${path}.id`),
		name: string(source.name, `${path}.name`),
		request: parseScaffoldRequest(source.request, `${path}.request`),
		createdAt: isoTime(source.createdAt, `${path}.createdAt`),
		updatedAt: isoTime(source.updatedAt, `${path}.updatedAt`)
	};
}

export function parsePresetInput(value: unknown, path = 'Preset'): Preset {
	const source = object(value, path);
	return {
		id: string(source.id, `${path}.id`),
		name: string(source.name, `${path}.name`),
		request: parseScaffoldRequest(source.request, `${path}.request`),
		createdAt: newOrIsoTime(source.createdAt, `${path}.createdAt`),
		updatedAt: newOrIsoTime(source.updatedAt, `${path}.updatedAt`)
	};
}

export function parseHistoryEntry(value: unknown, path = 'HistoryEntry'): HistoryEntry {
	const source = object(value, path);
	return {
		id: string(source.id, `${path}.id`),
		recipeId: string(source.recipeId, `${path}.recipeId`),
		recipeName: string(source.recipeName, `${path}.recipeName`),
		projectName: string(source.projectName, `${path}.projectName`),
		projectDir: string(source.projectDir, `${path}.projectDir`),
		packageManager: string(source.packageManager, `${path}.packageManager`),
		state: string(source.state, `${path}.state`),
		planHash: string(source.planHash, `${path}.planHash`),
		durationMs: integer(source.durationMs, `${path}.durationMs`),
		createdAt: isoTime(source.createdAt, `${path}.createdAt`),
		error: string(source.error, `${path}.error`)
	};
}

export function parseJobStartedEvent(value: unknown, path = 'JobStartedEvent'): JobStartedEvent {
	const source = object(value, path);
	return {
		jobId: string(source.jobId, `${path}.jobId`),
		projectDir: string(source.projectDir, `${path}.projectDir`),
		stepCount: integer(source.stepCount, `${path}.stepCount`)
	};
}

const jobStepStates = ['running', 'success', 'failed', 'skipped'] as const;

export function parseJobStepEvent(value: unknown, path = 'JobStepEvent'): JobStepEvent {
	const source = object(value, path);
	return {
		jobId: string(source.jobId, `${path}.jobId`),
		stepId: string(source.stepId, `${path}.stepId`),
		index: integer(source.index, `${path}.index`),
		total: integer(source.total, `${path}.total`),
		state: oneOf<JobStepState>(source.state, `${path}.state`, jobStepStates)
	};
}

export function parseJobLogEvent(value: unknown, path = 'JobLogEvent'): JobLogEvent {
	const source = object(value, path);
	return {
		jobId: string(source.jobId, `${path}.jobId`),
		seq: integer(source.seq, `${path}.seq`),
		stepId: string(source.stepId, `${path}.stepId`),
		stream: oneOf<LogStream>(source.stream, `${path}.stream`, logStreams),
		text: string(source.text, `${path}.text`)
	};
}

export function parseJobDoneEvent(value: unknown, path = 'JobDoneEvent'): JobDoneEvent {
	const source = object(value, path);
	return {
		jobId: string(source.jobId, `${path}.jobId`),
		state: oneOf<JobState>(source.state, `${path}.state`, jobStates),
		exitCode: integer(source.exitCode, `${path}.exitCode`),
		durationMs: integer(source.durationMs, `${path}.durationMs`),
		projectDir: string(source.projectDir, `${path}.projectDir`),
		error: string(source.error, `${path}.error`)
	};
}

export function parseRecipesReloadedEvent(
	value: unknown,
	path = 'RecipesReloadedEvent'
): RecipesReloadedEvent {
	const source = object(value, path);
	return {
		count: integer(source.count, `${path}.count`),
		warnings: stringArray(source.warnings, `${path}.warnings`),
		overrides: stringArray(source.overrides, `${path}.overrides`)
	};
}

export function parseString(value: unknown, path = 'value'): string {
	return string(value, path);
}

export function parseVoid(value: unknown, path = 'result'): void {
	if (value !== undefined && value !== null) return invalid(path, 'no value');
}

export function parseArray<T>(
	value: unknown,
	path: string,
	parse: (item: unknown, path: string) => T
): T[] {
	return array(value, path, parse);
}
