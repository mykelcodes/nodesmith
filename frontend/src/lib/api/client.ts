import * as RecipeBindings from '../wailsjs/go/services/RecipeService.js';
import * as ScaffoldBindings from '../wailsjs/go/services/ScaffoldService.js';
import * as StoreBindings from '../wailsjs/go/services/StoreService.js';
import * as ToolchainBindings from '../wailsjs/go/services/ToolchainService.js';
import { services as WailsModels } from '../wailsjs/go/models';
import {
	parseArray,
	parseHistoryEntry,
	parseJob,
	parseLogLine,
	parsePlan,
	parsePreset,
	parsePresetInput,
	parseRecipe,
	parseRecipeSummary,
	parseReloadResult,
	parseScaffoldRequest,
	parseSettings,
	parseString,
	parseToolchain,
	parseValidationResult,
	parseVoid
} from './parse';
import type { Preset, ScaffoldRequest, Settings } from './types';

type ServiceName = 'RecipeService' | 'ScaffoldService' | 'StoreService' | 'ToolchainService';
type WailsWindow = Window & {
	go?: {
		services?: Partial<Record<ServiceName, Record<string, unknown>>>;
	};
};

export class NodesmithBridgeError extends Error {
	readonly operation: string;
	readonly cause?: unknown;

	constructor(operation: string, message: string, cause?: unknown) {
		super(`${operation}: ${message}`);
		this.name = 'NodesmithBridgeError';
		this.operation = operation;
		this.cause = cause;
	}
}

export function toErrorMessage(error: unknown): string {
	if (error instanceof Error && error.message.trim() !== '') return error.message;
	if (typeof error === 'string' && error.trim() !== '') return error;
	return 'An unexpected error occurred. Please try again.';
}

function ensureBinding(service: ServiceName, method: string, operation: string): void {
	if (typeof window === 'undefined') {
		throw new NodesmithBridgeError(
			operation,
			'the Wails v2 desktop bridge is unavailable outside the desktop application'
		);
	}
	const bridge = (window as WailsWindow).go?.services?.[service]?.[method];
	if (typeof bridge !== 'function') {
		throw new NodesmithBridgeError(
			operation,
			`the Wails v2 binding ${service}.${method} is unavailable — rebuild or run the app with Wails v2`
		);
	}
}

async function call<T>(
	service: ServiceName,
	method: string,
	operation: string,
	invoke: () => Promise<unknown>,
	parse: (value: unknown) => T
): Promise<T> {
	ensureBinding(service, method, operation);
	try {
		return parse(await invoke());
	} catch (error) {
		if (error instanceof NodesmithBridgeError) throw error;
		throw new NodesmithBridgeError(operation, toErrorMessage(error), error);
	}
}

// Go's time.Time rejects an empty string, so unset timestamps travel as its zero value.
const goZeroTime = '0001-01-01T00:00:00Z';

function wireTime(value: string): string {
	return value === '' ? goZeroTime : value;
}

function wireScaffoldRequest(request: ScaffoldRequest): WailsModels.ScaffoldRequest {
	return new WailsModels.ScaffoldRequest({
		...request,
		minimumReleaseAge: request.minimumReleaseAge ?? undefined
	});
}

const recipes = {
	list: () =>
		call('RecipeService', 'List', 'Load recipes', RecipeBindings.List, (value) =>
			parseArray(value, 'RecipeService.List', parseRecipeSummary)
		),
	get: (id: string) =>
		call(
			'RecipeService',
			'Get',
			'Load recipe',
			() => RecipeBindings.Get(id),
			(value) => parseRecipe(value, 'RecipeService.Get')
		),
	reload: () =>
		call('RecipeService', 'Reload', 'Reload recipes', RecipeBindings.Reload, (value) =>
			parseReloadResult(value, 'RecipeService.Reload')
		),
	validate: (raw: string) =>
		call(
			'RecipeService',
			'Validate',
			'Validate recipe',
			() => RecipeBindings.Validate(raw),
			(value) => parseValidationResult(value, 'RecipeService.Validate')
		),
	openRecipeDir: () =>
		call(
			'RecipeService',
			'OpenRecipeDir',
			'Open recipe directory',
			RecipeBindings.OpenRecipeDir,
			(value) => parseVoid(value, 'RecipeService.OpenRecipeDir')
		)
};

const toolchain = {
	detect: (force = false) =>
		call(
			'ToolchainService',
			'Detect',
			'Detect local tools',
			() => ToolchainBindings.Detect(force),
			(value) => parseToolchain(value, 'ToolchainService.Detect')
		),
	resolvedPath: () =>
		call(
			'ToolchainService',
			'ResolvedPath',
			'Load executable PATH',
			ToolchainBindings.ResolvedPath,
			(value) => parseString(value, 'ToolchainService.ResolvedPath')
		),
	setPathOverride: (path: string) =>
		call(
			'ToolchainService',
			'SetPathOverride',
			'Update executable PATH',
			() => ToolchainBindings.SetPathOverride(path),
			(value) => parseVoid(value, 'ToolchainService.SetPathOverride')
		)
};

const scaffold = {
	plan: (request: ScaffoldRequest) => {
		const safeRequest = parseScaffoldRequest(request);
		return call(
			'ScaffoldService',
			'Plan',
			'Build project plan',
			() => ScaffoldBindings.Plan(wireScaffoldRequest(safeRequest)),
			(value) => parsePlan(value, 'ScaffoldService.Plan')
		);
	},
	start: (request: ScaffoldRequest) => {
		const safeRequest = parseScaffoldRequest(request);
		return call(
			'ScaffoldService',
			'Start',
			'Start project creation',
			() => ScaffoldBindings.Start(wireScaffoldRequest(safeRequest)),
			(value) => parseJob(value, 'ScaffoldService.Start')
		);
	},
	cancel: (jobId: string) =>
		call(
			'ScaffoldService',
			'Cancel',
			'Cancel project creation',
			() => ScaffoldBindings.Cancel(jobId),
			(value) => parseVoid(value, 'ScaffoldService.Cancel')
		),
	status: (jobId: string) =>
		call(
			'ScaffoldService',
			'Status',
			'Load project status',
			() => ScaffoldBindings.Status(jobId),
			(value) => parseJob(value, 'ScaffoldService.Status')
		),
	logs: (jobId: string, fromSeq = 0) =>
		call(
			'ScaffoldService',
			'Logs',
			'Load project output',
			() => ScaffoldBindings.Logs(jobId, fromSeq),
			(value) => parseArray(value, 'ScaffoldService.Logs', parseLogLine)
		),
	pickDirectory: (startAt = '') =>
		call(
			'ScaffoldService',
			'PickDirectory',
			'Choose parent directory',
			() => ScaffoldBindings.PickDirectory(startAt),
			(value) => parseString(value, 'ScaffoldService.PickDirectory')
		),
	openInEditor: (dir: string, editor: string) =>
		call(
			'ScaffoldService',
			'OpenInEditor',
			'Open project in editor',
			() => ScaffoldBindings.OpenInEditor(dir, editor),
			(value) => parseVoid(value, 'ScaffoldService.OpenInEditor')
		),
	revealInFileManager: (dir: string) =>
		call(
			'ScaffoldService',
			'RevealInFileManager',
			'Reveal project in file manager',
			() => ScaffoldBindings.RevealInFileManager(dir),
			(value) => parseVoid(value, 'ScaffoldService.RevealInFileManager')
		)
};

const store = {
	getSettings: () =>
		call('StoreService', 'GetSettings', 'Load settings', StoreBindings.GetSettings, (value) =>
			parseSettings(value, 'StoreService.GetSettings')
		),
	saveSettings: (settings: Settings) => {
		const safeSettings = parseSettings(settings);
		// The generated binding models an unset *int as optional, so "no global
		// cooldown" has to travel as undefined rather than null.
		const bindingSettings = new WailsModels.Settings({
			...safeSettings,
			minimumReleaseAge: safeSettings.minimumReleaseAge ?? undefined
		});
		return call(
			'StoreService',
			'SaveSettings',
			'Save settings',
			() => StoreBindings.SaveSettings(bindingSettings),
			(value) => parseVoid(value, 'StoreService.SaveSettings')
		);
	},
	listPresets: () =>
		call('StoreService', 'ListPresets', 'Load presets', StoreBindings.ListPresets, (value) =>
			parseArray(value, 'StoreService.ListPresets', parsePreset)
		),
	savePreset: (preset: Preset) => {
		const safePreset = parsePresetInput(preset);
		const bindingPreset = new WailsModels.Preset({
			...safePreset,
			request: wireScaffoldRequest(safePreset.request),
			createdAt: wireTime(safePreset.createdAt),
			updatedAt: wireTime(safePreset.updatedAt)
		});
		return call(
			'StoreService',
			'SavePreset',
			'Save preset',
			() => StoreBindings.SavePreset(bindingPreset),
			(value) => parseVoid(value, 'StoreService.SavePreset')
		);
	},
	deletePreset: (id: string) =>
		call(
			'StoreService',
			'DeletePreset',
			'Delete preset',
			() => StoreBindings.DeletePreset(id),
			(value) => parseVoid(value, 'StoreService.DeletePreset')
		),
	listHistory: (limit = 0) =>
		call(
			'StoreService',
			'ListHistory',
			'Load project history',
			() => StoreBindings.ListHistory(limit),
			(value) => parseArray(value, 'StoreService.ListHistory', parseHistoryEntry)
		),
	clearHistory: () =>
		call(
			'StoreService',
			'ClearHistory',
			'Clear project history',
			StoreBindings.ClearHistory,
			(value) => parseVoid(value, 'StoreService.ClearHistory')
		)
};

export const serviceApi = { recipes, toolchain, scaffold, store } as const;
