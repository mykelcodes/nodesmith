import { browser } from '$app/environment';
import { get, writable } from 'svelte/store';
import type { Job, JobDoneEvent, Plan, Recipe, ScaffoldRequest } from '$lib/api';
import {
	parseJob,
	parseJobDoneEvent,
	parsePlan,
	parseRecipe,
	parseScaffoldRequest
} from '$lib/api/parse';

const STORAGE_KEY = 'nodesmith:wizard:v1';

export interface WizardSnapshot {
	recipe: Recipe | null;
	request: ScaffoldRequest | null;
	plan: Plan | null;
	job: Job | null;
	done: JobDoneEvent | null;
}

const emptySnapshot: WizardSnapshot = {
	recipe: null,
	request: null,
	plan: null,
	job: null,
	done: null
};

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function loadSnapshot(): WizardSnapshot {
	if (!browser) return emptySnapshot;

	try {
		const raw = window.localStorage.getItem(STORAGE_KEY);
		if (!raw) return emptySnapshot;
		const parsed: unknown = JSON.parse(raw);
		if (!isRecord(parsed)) return emptySnapshot;

		return {
			recipe: parsed.recipe === null ? null : parseRecipe(parsed.recipe, 'saved wizard recipe'),
			request:
				parsed.request === null
					? null
					: parseScaffoldRequest(parsed.request, 'saved wizard request'),
			plan: parsed.plan === null ? null : parsePlan(parsed.plan, 'saved wizard plan'),
			job: parsed.job === null ? null : parseJob(parsed.job, 'saved wizard job'),
			done: parsed.done === null ? null : parseJobDoneEvent(parsed.done, 'saved wizard result')
		};
	} catch {
		window.localStorage.removeItem(STORAGE_KEY);
		return emptySnapshot;
	}
}

const store = writable<WizardSnapshot>(loadSnapshot());

if (browser) {
	store.subscribe((snapshot) => {
		try {
			window.localStorage.setItem(STORAGE_KEY, JSON.stringify(snapshot));
		} catch {
			// A full or unavailable localStorage must not make the wizard unusable.
		}
	});
}

function replace(changes: Partial<WizardSnapshot>) {
	store.update((snapshot) => ({ ...snapshot, ...changes }));
}

export const wizard = {
	subscribe: store.subscribe,
	get snapshot() {
		return get(store);
	},
	selectRecipe(recipe: Recipe) {
		const current = get(store);
		if (current.recipe?.id === recipe.id) {
			replace({ recipe });
			return;
		}
		replace({ recipe, request: null, plan: null, job: null, done: null });
	},
	setRequest(request: ScaffoldRequest) {
		const current = get(store);
		const requestChanged = JSON.stringify(current.request) !== JSON.stringify(request);
		replace({
			request,
			...(requestChanged ? { plan: null, job: null, done: null } : {})
		});
	},
	syncDefaultParentDir(parentDir: string) {
		const current = get(store);
		if (!current.request || current.request.parentDir === parentDir) return;
		replace({
			request: { ...current.request, parentDir },
			plan: null,
			job: null,
			done: null
		});
	},
	setPlan(plan: Plan) {
		replace({ plan, job: null, done: null });
	},
	setJob(job: Job) {
		replace({ job, done: null });
	},
	updateJob(job: Job) {
		replace({ job });
	},
	setDone(done: JobDoneEvent) {
		replace({ done });
	},
	loadRequest(request: ScaffoldRequest) {
		replace({ request, plan: null, job: null, done: null });
	},
	resetRun() {
		replace({ plan: null, job: null, done: null });
	},
	reset() {
		store.set({ ...emptySnapshot });
	}
};
