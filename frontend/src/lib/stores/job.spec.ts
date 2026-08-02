import { describe, expect, it } from 'vitest';
import type { Job, JobStepEvent, LogLine, PlanStep } from '$lib/api';
import { countCompletedPlanSteps, derivePlanStepState, jobRuntime, mergeLogLines } from './job';

function line(seq: number, text = String(seq)): LogLine {
	return { seq, text, stepId: 'scaffold', stream: 'stdout' };
}

function job(state: Job['state'], stepIndex: number, stepCount = 3): Job {
	return {
		id: 'job-1',
		state,
		stepIndex,
		stepCount,
		exitCode: state === 'success' ? 0 : -1,
		projectDir: '/projects/demo',
		startedAt: '2026-07-28T12:00:00Z',
		endedAt: '2026-07-28T12:00:01Z',
		error: ''
	};
}

const planSteps: PlanStep[] = ['create', 'install', 'git'].map((id) => ({
	id,
	kind: 'command',
	label: id,
	bin: id,
	args: [],
	dir: '/projects',
	env: {},
	display: id,
	config: null,
	setup: null
}));

describe('mergeLogLines', () => {
	it('keeps sequence order when a lower sequence arrives in a later frame', () => {
		const firstFrame = mergeLogLines([], [line(2), line(4)]);
		const secondFrame = mergeLogLines(firstFrame, [line(1), line(3)]);

		expect(secondFrame.map((item) => item.seq)).toEqual([1, 2, 3, 4]);
	});

	it('deduplicates replay and live lines by sequence while retaining the first payload', () => {
		const merged = mergeLogLines([line(1, 'replayed')], [line(1, 'live'), line(2)]);

		expect(merged).toEqual([line(1, 'replayed'), line(2)]);
	});

	it('orders 50,000 lines and removes duplicates from out-of-order frames', () => {
		const outOfOrder = Array.from({ length: 50_000 }, (_, index) => line(50_000 - index));
		const duplicates = [line(1, 'duplicate'), line(25_000, 'duplicate'), line(50_000, 'duplicate')];

		const merged = mergeLogLines([], [...outOfOrder, ...duplicates]);

		expect(merged).toHaveLength(50_000);
		expect(merged[0].seq).toBe(1);
		expect(merged[24_999].seq).toBe(25_000);
		expect(merged[49_999].seq).toBe(50_000);
		expect(merged.every((item, index) => item.seq === index + 1)).toBe(true);
	});
});

describe('hydrated plan progress', () => {
	it('reconstructs all successful steps when no step events survived remount', () => {
		const hydrated = job('success', 2);

		expect(
			planSteps.map((_step, index) => derivePlanStepState(hydrated, undefined, index))
		).toEqual(['success', 'success', 'success']);
		expect(countCompletedPlanSteps(planSteps, hydrated, {})).toBe(3);
	});

	it('reconstructs completed and failed steps from a failed Job snapshot', () => {
		const hydrated = job('failed', 1);

		expect(
			planSteps.map((_step, index) => derivePlanStepState(hydrated, undefined, index))
		).toEqual(['success', 'failed', 'pending']);
		expect(countCompletedPlanSteps(planSteps, hydrated, {})).toBe(1);
	});

	it('does not leave a stale running event active after cancellation', () => {
		const staleRunning: JobStepEvent = {
			jobId: 'job-1',
			stepId: 'install',
			index: 1,
			total: 3,
			state: 'running'
		};
		const hydrated = job('cancelled', 1);

		expect(derivePlanStepState(hydrated, staleRunning, 1)).toBe('pending');
		expect(countCompletedPlanSteps(planSteps, hydrated, { install: staleRunning })).toBe(1);
	});
});

describe('jobRuntime terminal event state', () => {
	it('retains failed step and failed job event details', () => {
		jobRuntime.clear();
		jobRuntime.begin('failed-job');
		jobRuntime.setStep({
			jobId: 'failed-job',
			stepId: 'install',
			index: 2,
			total: 3,
			state: 'failed'
		});
		jobRuntime.setDone({
			jobId: 'failed-job',
			state: 'failed',
			exitCode: 1,
			durationMs: 1200,
			projectDir: '/projects/demo',
			error: 'install failed'
		});

		expect(jobRuntime.snapshot.steps.install.state).toBe('failed');
		expect(jobRuntime.snapshot.done).toMatchObject({
			state: 'failed',
			exitCode: 1,
			error: 'install failed'
		});
	});

	it('retains cancelled job event state without rewriting it as failure', () => {
		jobRuntime.clear();
		jobRuntime.begin('cancelled-job');
		jobRuntime.setDone({
			jobId: 'cancelled-job',
			state: 'cancelled',
			exitCode: -1,
			durationMs: 300,
			projectDir: '/projects/demo',
			error: ''
		});

		expect(jobRuntime.snapshot.done).toMatchObject({
			state: 'cancelled',
			exitCode: -1,
			error: ''
		});
	});

	it('reports a live sequence gap so the route can replay retained backend logs', () => {
		jobRuntime.clear();
		jobRuntime.begin('gapped-job');

		expect(
			jobRuntime.addLog({
				jobId: 'gapped-job',
				seq: 0,
				stepId: 'create',
				stream: 'stdout',
				text: 'first'
			})
		).toBe(false);
		expect(
			jobRuntime.addLog({
				jobId: 'gapped-job',
				seq: 2,
				stepId: 'create',
				stream: 'stdout',
				text: 'third'
			})
		).toBe(true);
		jobRuntime.clear();
	});

	it('fills a dropped live sequence from authoritative retained replay', () => {
		jobRuntime.clear();
		jobRuntime.begin('recovered-job');
		jobRuntime.addLog({
			jobId: 'recovered-job',
			seq: 0,
			stepId: 'create',
			stream: 'stdout',
			text: 'first'
		});
		jobRuntime.addLog({
			jobId: 'recovered-job',
			seq: 2,
			stepId: 'create',
			stream: 'stdout',
			text: 'third'
		});

		jobRuntime.replay('recovered-job', [line(0, 'first'), line(1, 'second'), line(2, 'third')]);

		expect(jobRuntime.snapshot.lines.map((item) => item.seq)).toEqual([0, 1, 2]);
		jobRuntime.clear();
	});
});
