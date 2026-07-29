import { describe, expect, it, vi } from 'vitest';
import type { Job, JobDoneEvent, LogLine, Plan, Recipe, ScaffoldRequest } from '$lib/api';
import { jobRuntime } from './job';
import { wizard } from './wizard';

const recipe: Recipe = {
	schemaVersion: 1,
	id: 'vite',
	name: 'Vite',
	category: 'frontend',
	description: 'Vite project',
	docsUrl: 'https://vite.dev',
	tags: ['frontend'],
	icon: 'vite',
	verifiedAt: '2026-07-28',
	installPolicy: 'optional',
	minimumReleaseAge: null,
	requires: { node: '>=20', packageManagers: ['pnpm'], tools: [] },
	fields: [],
	steps: [],
	available: true,
	unavailableReasons: []
};

const request: ScaffoldRequest = {
	recipeId: 'vite',
	projectName: 'demo',
	parentDir: '/projects',
	packageManager: 'pnpm',
	installDeps: true,
	gitInit: true,
	minimumReleaseAge: null,
	answers: {}
};

const plan: Plan = {
	recipeId: 'vite',
	projectDir: '/projects/demo',
	steps: [
		{
			id: 'create',
			kind: 'command',
			label: 'Create project',
			bin: 'pnpm',
			args: ['create', 'vite', 'demo'],
			dir: '/projects',
			env: { CI: '1' },
			display: 'pnpm create vite demo',
			config: null
		}
	],
	warnings: [],
	hash: 'reviewed-plan'
};

const runningJob: Job = {
	id: 'job-1',
	state: 'running',
	stepIndex: 0,
	stepCount: 1,
	exitCode: -1,
	projectDir: '/projects/demo',
	startedAt: '2026-07-28T12:00:00Z',
	endedAt: '0001-01-01T00:00:00Z',
	error: ''
};

const done: JobDoneEvent = {
	jobId: 'job-1',
	state: 'success',
	exitCode: 0,
	durationMs: 500,
	projectDir: '/projects/demo',
	error: ''
};

describe('mocked Wails wizard route path', () => {
	it('carries typed binding results from catalogue through configure, review, run, and result', async () => {
		wizard.reset();
		jobRuntime.clear();
		const retainedLines: LogLine[] = [
			{ seq: 0, stepId: 'create', stream: 'stdout', text: 'created demo' }
		];
		const bindings = {
			getRecipe: vi.fn(async (recipeId: string) => {
				void recipeId;
				return recipe;
			}),
			plan: vi.fn(async (scaffoldRequest: ScaffoldRequest) => {
				void scaffoldRequest;
				return plan;
			}),
			start: vi.fn(async (scaffoldRequest: ScaffoldRequest) => {
				void scaffoldRequest;
				return runningJob;
			}),
			logs: vi.fn(async (jobId: string, fromSequence: number) => {
				void jobId;
				void fromSequence;
				return retainedLines;
			})
		};

		wizard.selectRecipe(await bindings.getRecipe('vite'));
		expect(wizard.snapshot.recipe?.id).toBe('vite');
		expect(wizard.snapshot.request).toBeNull();

		wizard.setRequest(request);
		expect(wizard.snapshot.request).toEqual(request);
		expect(wizard.snapshot.plan).toBeNull();

		wizard.setPlan(await bindings.plan(wizard.snapshot.request!));
		expect(wizard.snapshot.plan?.hash).toBe('reviewed-plan');

		const job = await bindings.start(wizard.snapshot.request!);
		wizard.setJob(job);
		jobRuntime.begin(job.id);
		jobRuntime.replay(job.id, await bindings.logs(job.id, 0));
		expect(wizard.snapshot.job?.state).toBe('running');
		expect(jobRuntime.snapshot.lines).toEqual(retainedLines);

		jobRuntime.setDone(done);
		wizard.setDone(done);
		expect(wizard.snapshot.done).toEqual(done);
		expect(jobRuntime.snapshot.done).toEqual(done);
		expect(bindings.getRecipe).toHaveBeenCalledWith('vite');
		expect(bindings.plan).toHaveBeenCalledWith(request);
		expect(bindings.start).toHaveBeenCalledWith(request);
		expect(bindings.logs).toHaveBeenCalledWith('job-1', 0);
	});

	it('invalidates reviewed and terminal state when configuration changes', () => {
		wizard.reset();
		wizard.selectRecipe(recipe);
		wizard.setRequest(request);
		wizard.setPlan(plan);
		wizard.setJob(runningJob);
		wizard.setDone(done);

		wizard.setRequest({ ...request, projectName: 'changed' });

		expect(wizard.snapshot.request?.projectName).toBe('changed');
		expect(wizard.snapshot.plan).toBeNull();
		expect(wizard.snapshot.job).toBeNull();
		expect(wizard.snapshot.done).toBeNull();
	});
});
