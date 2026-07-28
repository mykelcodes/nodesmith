import { get, writable } from 'svelte/store';
import type {
	Job,
	JobDoneEvent,
	JobLogEvent,
	JobStartedEvent,
	JobStepEvent,
	JobStepState,
	LogLine,
	PlanStep
} from '$lib/api';

export interface JobRuntimeSnapshot {
	jobId: string;
	lines: LogLine[];
	steps: Record<string, JobStepEvent>;
	started: JobStartedEvent | null;
	done: JobDoneEvent | null;
	error: string;
}

const initialState: JobRuntimeSnapshot = {
	jobId: '',
	lines: [],
	steps: {},
	started: null,
	done: null,
	error: ''
};

const store = writable<JobRuntimeSnapshot>(initialState);
let seenSequences = new Set<number>();
let pendingLines = new Map<number, LogLine>();
let flushHandle: number | ReturnType<typeof setTimeout> | null = null;
let flushUsesAnimationFrame = false;
let highestSequence = -1;

export function mergeLogLines(
	existing: readonly LogLine[],
	additions: readonly LogLine[]
): LogLine[] {
	if (additions.length === 0) return [...existing];
	const sortedAdditions = [...additions].sort((left, right) => left.seq - right.seq);
	const uniqueAdditions = sortedAdditions.filter(
		(line, index) => index === 0 || sortedAdditions[index - 1].seq !== line.seq
	);
	const lastExisting = existing.at(-1);
	if (!lastExisting || uniqueAdditions[0].seq > lastExisting.seq) {
		return [...existing, ...uniqueAdditions];
	}

	const merged: LogLine[] = [];
	let existingIndex = 0;
	let additionIndex = 0;
	while (existingIndex < existing.length || additionIndex < uniqueAdditions.length) {
		const current = existing[existingIndex];
		const addition = uniqueAdditions[additionIndex];
		if (!addition || (current && current.seq < addition.seq)) {
			merged.push(current);
			existingIndex += 1;
		} else if (!current || addition.seq < current.seq) {
			merged.push(addition);
			additionIndex += 1;
		} else {
			merged.push(current);
			existingIndex += 1;
			additionIndex += 1;
		}
	}
	return merged;
}

export type DisplayStepState = JobStepState | 'pending';

export function derivePlanStepState(
	job: Job | null,
	event: JobStepEvent | undefined,
	index: number
): DisplayStepState {
	if (event && event.state !== 'running') return event.state;
	if (!job || job.state === 'pending' || job.stepIndex < 0) return event?.state ?? 'pending';

	if (job.state === 'success') return index < job.stepCount ? 'success' : 'pending';
	if (job.state === 'failed') {
		if (index < job.stepIndex) return 'success';
		if (index === job.stepIndex) return 'failed';
		return 'pending';
	}
	if (job.state === 'cancelled') {
		if (index < job.stepIndex) return 'success';
		return 'pending';
	}
	if (index < job.stepIndex) return 'success';
	if (index === job.stepIndex) return 'running';
	return 'pending';
}

export function countCompletedPlanSteps(
	steps: readonly PlanStep[],
	job: Job | null,
	events: Readonly<Record<string, JobStepEvent>>
): number {
	return steps.filter((step, index) => {
		const state = derivePlanStepState(job, events[step.id], index);
		return state === 'success' || state === 'skipped';
	}).length;
}

function cancelScheduledFlush() {
	if (flushHandle === null) return;
	if (flushUsesAnimationFrame && typeof cancelAnimationFrame === 'function') {
		cancelAnimationFrame(flushHandle as number);
	} else {
		clearTimeout(flushHandle);
	}
	flushHandle = null;
	flushUsesAnimationFrame = false;
}

function beginJob(jobId: string) {
	if (get(store).jobId === jobId) return;
	cancelScheduledFlush();
	seenSequences = new Set();
	pendingLines = new Map();
	highestSequence = -1;
	store.set({ ...initialState, jobId });
}

function flushPendingLines() {
	if (pendingLines.size === 0) return;
	const additions = [...pendingLines.values()].sort((left, right) => left.seq - right.seq);
	pendingLines = new Map();
	store.update((snapshot) => ({
		...snapshot,
		lines: mergeLogLines(snapshot.lines, additions)
	}));
}

function scheduleFlush() {
	if (flushHandle !== null) return;
	const flush = () => {
		flushHandle = null;
		flushUsesAnimationFrame = false;
		flushPendingLines();
	};

	if (typeof requestAnimationFrame === 'function') {
		flushUsesAnimationFrame = true;
		flushHandle = requestAnimationFrame(flush);
	} else {
		flushUsesAnimationFrame = false;
		flushHandle = setTimeout(flush, 16);
	}
}

function acceptLine(line: LogLine) {
	if (seenSequences.has(line.seq) || pendingLines.has(line.seq)) return;
	seenSequences.add(line.seq);
	pendingLines.set(line.seq, line);
	highestSequence = Math.max(highestSequence, line.seq);
}

export const jobRuntime = {
	subscribe: store.subscribe,
	get snapshot() {
		return get(store);
	},
	begin(jobId: string) {
		beginJob(jobId);
	},
	replay(jobId: string, lines: readonly LogLine[]) {
		if (get(store).jobId !== jobId) beginJob(jobId);
		for (const line of lines) acceptLine(line);
		flushPendingLines();
	},
	addLog(event: JobLogEvent) {
		if (get(store).jobId !== event.jobId) return false;
		const hasGap = event.seq > highestSequence + 1;
		acceptLine({
			seq: event.seq,
			stream: event.stream,
			text: event.text,
			stepId: event.stepId
		});
		scheduleFlush();
		return hasGap;
	},
	setStarted(event: JobStartedEvent) {
		if (get(store).jobId !== event.jobId) beginJob(event.jobId);
		store.update((snapshot) => ({ ...snapshot, started: event }));
	},
	setStep(event: JobStepEvent) {
		if (get(store).jobId !== event.jobId) return;
		store.update((snapshot) => ({
			...snapshot,
			steps: { ...snapshot.steps, [event.stepId]: event }
		}));
	},
	setDone(event: JobDoneEvent) {
		if (get(store).jobId !== event.jobId) return;
		store.update((snapshot) => ({ ...snapshot, done: event }));
	},
	setError(error: string) {
		store.update((snapshot) => ({ ...snapshot, error }));
	},
	clear() {
		cancelScheduledFlush();
		seenSequences = new Set();
		pendingLines = new Map();
		highestSequence = -1;
		store.set(initialState);
	}
};
