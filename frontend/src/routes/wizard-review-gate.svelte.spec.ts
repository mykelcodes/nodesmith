import { page } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Job, Plan, Recipe, ScaffoldRequest } from '$lib/api';

/**
 * An end-to-end walk of the wizard against a stubbed bridge.
 *
 * The plan-hash review boundary is the application's central safety property:
 * the commands the user approved are the commands that run. Until now it was
 * verified only by unit tests on either side of the bridge — the Go service knew
 * it rejected a mismatch, and the Svelte page knew it called `start`, but
 * nothing checked that the two halves compose.
 *
 * `stubBackend` below reproduces the real `ScaffoldService` contract rather than
 * returning canned values: `plan` resolves the current recipe and records the
 * hash it showed the user, and `start` re-resolves and refuses when the hash
 * moved. That is what makes the mutation case meaningful.
 */

interface StubBackend {
	/** Replaces the recipe's resolved argv, as an edited user recipe would. */
	mutateSteps(args: string[]): void;
	planCalls: number;
	startCalls: number;
}

const backend = vi.hoisted(() => {
	const state = {
		args: ['create', 'demo'] as string[],
		reviewedHash: null as string | null,
		planCalls: 0,
		startCalls: 0
	};
	return state;
});

const apiMocks = vi.hoisted(() => ({
	plan: vi.fn(),
	start: vi.fn(),
	status: vi.fn(),
	logs: vi.fn(),
	cancel: vi.fn(),
	copyText: vi.fn()
}));

vi.mock('$lib/api', () => ({
	api: {
		scaffold: {
			plan: apiMocks.plan,
			start: apiMocks.start,
			status: apiMocks.status,
			logs: apiMocks.logs,
			cancel: apiMocks.cancel
		},
		events: {
			onJobStarted: () => () => {},
			onJobStep: () => () => {},
			onJobLog: () => () => {},
			onJobDone: () => () => {}
		}
	},
	copyText: apiMocks.copyText,
	toErrorMessage: (error: unknown) => (error instanceof Error ? error.message : String(error))
}));

vi.mock('$app/navigation', () => ({ goto: vi.fn(async () => {}) }));
vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

import ReviewPage from './review/+page.svelte';
import { wizard } from '$lib/stores';

const recipe: Recipe = {
	schemaVersion: 1,
	id: 'stub',
	name: 'Stub recipe',
	category: 'tooling',
	description: 'A recipe used to walk the wizard end to end.',
	docsUrl: 'https://example.invalid/docs',
	tags: ['tooling'],
	icon: 'vite',
	verifiedAt: '2026-07-28',
	installPolicy: 'optional',
	minimumReleaseAge: null,
	requires: { node: '>=20', packageManagers: ['npm'], tools: [] },
	fields: [],
	steps: [],
	available: true,
	unavailableReasons: []
};

const request: ScaffoldRequest = {
	recipeId: 'stub',
	projectName: 'demo',
	parentDir: '/projects',
	packageManager: 'npm',
	installDeps: false,
	gitInit: false,
	minimumReleaseAge: null,
	answers: {}
};

/** Stands in for the Go side's SHA-256 over the canonicalised steps. */
function hashOf(args: readonly string[]): string {
	return `hash:${args.join(' ')}`;
}

function resolvePlan(): Plan {
	return {
		recipeId: 'stub',
		projectDir: '/projects/demo',
		hash: hashOf(backend.args),
		warnings: [],
		steps: [
			{
				id: 'create',
				label: 'Create test project',
				bin: 'npx',
				dir: '/projects',
				env: { CI: '1' },
				args: [...backend.args],
				display: `npx ${backend.args.join(' ')}`,
				kind: 'command',
				config: null
			}
		]
	};
}

function stubBackend(): StubBackend {
	apiMocks.plan.mockImplementation(async () => {
		backend.planCalls += 1;
		const plan = resolvePlan();
		// The review entry is single-use and always records the hash the user
		// was actually shown.
		backend.reviewedHash = plan.hash;
		return plan;
	});

	apiMocks.start.mockImplementation(async (): Promise<Job> => {
		backend.startCalls += 1;
		if (backend.reviewedHash === null) {
			throw new Error('review the resolved commands before starting this project');
		}
		// Re-resolution is what catches a recipe that changed after review.
		if (resolvePlan().hash !== backend.reviewedHash) {
			throw new Error(
				'the resolved commands changed after review; review them again before running'
			);
		}
		backend.reviewedHash = null;
		return {
			id: 'job-1',
			state: 'running',
			stepIndex: 0,
			stepCount: 1,
			exitCode: -1,
			projectDir: '/projects/demo',
			startedAt: '2026-08-01T12:00:00Z',
			endedAt: '0001-01-01T00:00:00Z',
			error: ''
		};
	});

	return {
		mutateSteps(args: string[]) {
			backend.args = args;
		},
		get planCalls() {
			return backend.planCalls;
		},
		get startCalls() {
			return backend.startCalls;
		}
	};
}

describe('wizard review gate, end to end over a stubbed bridge', () => {
	let harness: StubBackend;

	beforeEach(() => {
		window.localStorage.clear();
		backend.args = ['create', 'demo'];
		backend.reviewedHash = null;
		backend.planCalls = 0;
		backend.startCalls = 0;
		apiMocks.plan.mockReset();
		apiMocks.start.mockReset();
		apiMocks.copyText.mockReset().mockResolvedValue(undefined);
		harness = stubBackend();

		// Enter the wizard as the configure step leaves it.
		wizard.reset();
		wizard.selectRecipe(recipe);
		wizard.setRequest(request);
	});

	it('shows the resolved commands and starts the job the user approved', async () => {
		render(ReviewPage);

		await expect.element(page.getByText('npx create demo')).toBeInTheDocument();

		await page.getByRole('button', { name: /run 1 step/i }).click();

		await vi.waitFor(() => expect(harness.startCalls).toBe(1));
		await vi.waitFor(() => expect(wizard.snapshot.job?.id).toBe('job-1'));
	});

	// The property the boundary exists for. The user reviewed one set of
	// commands; the recipe then changed underneath them; starting must be
	// refused rather than silently running the new commands.
	it('refuses to start when the plan changed after review', async () => {
		render(ReviewPage);

		await expect.element(page.getByText('npx create demo')).toBeInTheDocument();
		expect(harness.planCalls).toBe(1);

		harness.mutateSteps(['create', 'demo', '--unexpected-flag']);

		await page.getByRole('button', { name: /run 1 step/i }).click();

		await expect.element(page.getByText(/changed after review/i)).toBeInTheDocument();
		expect(wizard.snapshot.job).toBeNull();
	});

	// A single review authorises a single run, so a second start must fail even
	// though nothing about the plan changed.
	it('treats a review as single use', async () => {
		render(ReviewPage);
		await expect.element(page.getByText('npx create demo')).toBeInTheDocument();

		await page.getByRole('button', { name: /run 1 step/i }).click();
		await vi.waitFor(() => expect(harness.startCalls).toBe(1));

		await expect(apiMocks.start()).rejects.toThrow(/review the resolved commands/i);
	});
});
