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

// A factory, not a shared constant. Spreading one module-level object into
// store.set leaves every reset sharing the same lines array and steps record,
// so the immutability of this store would rest on every future writer
// remembering never to mutate in place.
function createInitialState(): JobRuntimeSnapshot {
	return {
		jobId: '',
		lines: [],
		steps: {},
		started: null,
		done: null,
		error: ''
	};
}

// Matches the backend replay ring, so anything dropped here is still
// recoverable through ScaffoldService.Logs. Without a cap the buffer grows for
// the lifetime of the job and every flush copies all of it.
export const MAX_RETAINED_LOG_LINES = 10_000;

const store = writable<JobRuntimeSnapshot>(createInitialState());

// Per-job bookkeeping that must not leak across jobs.
//
// These fields used to be file-scoped `let`s reset one by one, by convention,
// from beginJob and clear. Any new entry point that forgot one would corrupt
// dedupe state for the next job. Holding them on a single object means a reset
// is one assignment that cannot partially miss a field.
//
// Delivery is ordered in practice but not guaranteed, so acceptance is tracked
// as a contiguous watermark plus the few sequences seen beyond it. In the
// ordered case outOfOrder stays empty, which keeps this O(1) rather than
// remembering every sequence for the lifetime of the job.
interface SequenceTracking {
	pendingLines: Map<number, LogLine>;
	highestSequence: number;
	contiguousUpTo: number;
	outOfOrder: Set<number>;
}

function createTracking(): SequenceTracking {
	return {
		pendingLines: new Map(),
		highestSequence: -1,
		contiguousUpTo: -1,
		outOfOrder: new Set()
	};
}

let tracking = createTracking();
let flushHandle: number | ReturnType<typeof setTimeout> | null = null;
let flushUsesAnimationFrame = false;

function hasSeen(seq: number): boolean {
	return seq <= tracking.contiguousUpTo || tracking.outOfOrder.has(seq);
}

function markSeen(seq: number) {
	if (seq <= tracking.contiguousUpTo) return;
	tracking.outOfOrder.add(seq);
	while (tracking.outOfOrder.delete(tracking.contiguousUpTo + 1)) {
		tracking.contiguousUpTo += 1;
	}
}

function capLines(lines: LogLine[]): LogLine[] {
	if (lines.length <= MAX_RETAINED_LOG_LINES) return lines;
	return lines.slice(lines.length - MAX_RETAINED_LOG_LINES);
}

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

	// index addresses the step array that stepCount describes, so it is always in
	// range: a successful job has every step succeeded.
	if (job.state === 'success') return 'success';
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

function resetSequenceTracking() {
	tracking = createTracking();
}

function beginJob(jobId: string) {
	if (get(store).jobId === jobId) return;
	cancelScheduledFlush();
	resetSequenceTracking();
	store.set({ ...createInitialState(), jobId });
}

function flushPendingLines() {
	if (tracking.pendingLines.size === 0) return;
	const additions = [...tracking.pendingLines.values()].sort((left, right) => left.seq - right.seq);
	tracking.pendingLines = new Map();
	store.update((snapshot) => ({
		...snapshot,
		lines: capLines(mergeLogLines(snapshot.lines, additions))
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
	if (hasSeen(line.seq) || tracking.pendingLines.has(line.seq)) return;
	markSeen(line.seq);
	tracking.pendingLines.set(line.seq, line);
	tracking.highestSequence = Math.max(tracking.highestSequence, line.seq);
}

export const jobRuntime = {
	subscribe: store.subscribe,
	get snapshot() {
		return get(store);
	},
	// The first sequence not yet accepted. Recovering from this point refetches
	// only the gap, instead of pulling the whole retained buffer back across
	// the bridge every time an event is dropped.
	get nextExpectedSequence() {
		return tracking.contiguousUpTo + 1;
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
		const hasGap = event.seq > tracking.highestSequence + 1;
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
		resetSequenceTracking();
		store.set(createInitialState());
	}
};
