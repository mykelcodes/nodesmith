<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { api, toErrorMessage, type Job, type JobDoneEvent, type PlanStep } from '$lib/api';
	import { VirtualLogList } from '$lib/components/console';
	import { WizardProgress } from '$lib/components/form';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Badge, Button, EmptyState } from '$lib/components/ui';
	import { jobRuntime, wizard } from '$lib/stores';
	import { countCompletedPlanSteps, derivePlanStepState } from '$lib/stores/job';

	let currentJob = $state<Job | null>(wizard.snapshot.job);
	let autoscroll = $state(true);
	let cancelling = $state(false);
	let error = $state('');
	let initialising = $state(true);
	let redirectTimer: number | undefined;
	let recoveringLogs: Promise<void> | null = null;
	let finalising: Promise<void> | null = null;
	let statusPoll: ReturnType<typeof setInterval> | undefined;

	const STATUS_POLL_INTERVAL_MS = 2500;

	const terminal = $derived(
		$jobRuntime.done !== null ||
			currentJob?.state === 'success' ||
			currentJob?.state === 'failed' ||
			currentJob?.state === 'cancelled'
	);
	const completedSteps = $derived(
		countCompletedPlanSteps(wizard.snapshot.plan?.steps ?? [], currentJob, $jobRuntime.steps)
	);

	function stateForStep(step: PlanStep, index: number) {
		return derivePlanStepState(currentJob, $jobRuntime.steps[step.id], index);
	}

	function doneFromJob(job: Job): JobDoneEvent | null {
		if (!['success', 'failed', 'cancelled'].includes(job.state)) return null;
		const started = Date.parse(job.startedAt);
		const ended = Date.parse(job.endedAt);
		return {
			jobId: job.id,
			state: job.state as 'success' | 'failed' | 'cancelled',
			exitCode: job.exitCode,
			durationMs:
				Number.isFinite(started) && Number.isFinite(ended) ? Math.max(0, ended - started) : 0,
			projectDir: job.projectDir,
			error: job.error
		};
	}

	function recoverRetainedLogs(
		jobId: string,
		fromSeq = jobRuntime.nextExpectedSequence
	): Promise<void> {
		if (recoveringLogs) return recoveringLogs;
		const operation = (async () => {
			const lines = await api.scaffold.logs(jobId, fromSeq);
			jobRuntime.replay(jobId, lines);
		})();
		recoveringLogs = operation;
		void operation.then(
			() => {
				if (recoveringLogs === operation) recoveringLogs = null;
			},
			() => {
				if (recoveringLogs === operation) recoveringLogs = null;
			}
		);
		return operation;
	}

	function complete(event: JobDoneEvent): Promise<void> {
		if (currentJob?.id === event.jobId) {
			currentJob = {
				...currentJob,
				state: event.state,
				exitCode: event.exitCode,
				projectDir: event.projectDir,
				error: event.error
			};
			wizard.updateJob(currentJob);
		}
		jobRuntime.setDone(event);
		wizard.setDone(event);
		if (finalising) return finalising;
		finalising = (async () => {
			try {
				await recoverRetainedLogs(event.jobId);
			} catch (caught) {
				error = toErrorMessage(caught);
				jobRuntime.setError(error);
			}
			if (redirectTimer) window.clearTimeout(redirectTimer);
			redirectTimer = window.setTimeout(() => {
				void goto(resolve('/result'));
			}, 850);
		})();
		return finalising;
	}

	function stopStatusPoll() {
		if (statusPoll === undefined) return;
		clearInterval(statusPoll);
		statusPoll = undefined;
	}

	// Terminal state normally arrives on nodesmith:job:done. That delivery is
	// best-effort — the backend drops events under queue pressure and the
	// bridge is not transactional — so the console polls as well. Without this
	// a single dropped event strands the page on "Creating your project" with
	// no way forward.
	function startStatusPoll(jobId: string) {
		if (statusPoll !== undefined) return;
		statusPoll = setInterval(() => {
			void (async () => {
				try {
					const job = await refreshStatus(jobId);
					const done = doneFromJob(job);
					if (done && !$jobRuntime.done) await complete(done);
				} catch {
					// A transient status failure is not worth surfacing while
					// the run is live. The next tick retries.
				}
			})();
		}, STATUS_POLL_INTERVAL_MS);
	}

	$effect(() => {
		if (!terminal) return;
		stopStatusPoll();
		cancelling = false;
	});

	async function refreshStatus(jobId: string): Promise<Job> {
		currentJob = await api.scaffold.status(jobId);
		wizard.updateJob(currentJob);
		return currentJob;
	}

	async function hydrate(jobId: string) {
		initialising = true;
		error = '';
		try {
			const [lines, job] = await Promise.all([api.scaffold.logs(jobId, 0), refreshStatus(jobId)]);
			jobRuntime.replay(jobId, lines);
			const done = doneFromJob(job);
			if (done && !$jobRuntime.done) await complete(done);
		} catch (caught) {
			error = toErrorMessage(caught);
			jobRuntime.setError(error);
		} finally {
			initialising = false;
		}
	}

	async function cancelJob() {
		const jobId = currentJob?.id;
		if (!jobId || terminal || cancelling) return;
		cancelling = true;
		error = '';
		try {
			await api.scaffold.cancel(jobId);
		} catch (caught) {
			error = toErrorMessage(caught);
			cancelling = false;
		}
	}

	onMount(() => {
		const jobId = wizard.snapshot.job?.id || jobRuntime.snapshot.jobId;
		if (!jobId) {
			error = 'Start a reviewed project plan before opening the run console.';
			initialising = false;
			return;
		}

		jobRuntime.begin(jobId);
		const unsubscribers: (() => void)[] = [];
		try {
			unsubscribers.push(
				api.events.onJobStarted((event) => {
					if (event.jobId === jobId) jobRuntime.setStarted(event);
				}),
				api.events.onJobStep((event) => {
					if (event.jobId === jobId) jobRuntime.setStep(event);
				}),
				api.events.onJobLog((event) => {
					if (event.jobId !== jobId) return;
					if (jobRuntime.addLog(event)) {
						void recoverRetainedLogs(jobId).catch((caught) => {
							error = toErrorMessage(caught);
							jobRuntime.setError(error);
						});
					}
				}),
				api.events.onJobDone((event) => {
					if (event.jobId !== jobId) return;
					void complete(event);
					void refreshStatus(jobId).catch((caught) => {
						error = toErrorMessage(caught);
					});
				})
			);
		} catch (caught) {
			error = toErrorMessage(caught);
		}
		void hydrate(jobId);
		startStatusPoll(jobId);

		return () => {
			for (const unsubscribe of unsubscribers) unsubscribe();
			stopStatusPoll();
			if (redirectTimer) window.clearTimeout(redirectTimer);
		};
	});
</script>

<svelte:head>
	<title>Run project scaffold · Nodesmith</title>
</svelte:head>

<WizardProgress current="run" />

<header
	class="mt-8 flex flex-col gap-4 border-b border-line pb-6 lg:flex-row lg:items-end lg:justify-between"
>
	<div>
		<p class="text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">Live execution</p>
		<h1 class="mt-1 text-3xl font-bold tracking-[-0.04em] text-ink">
			{terminal ? 'Execution complete' : 'Creating your project'}
		</h1>
		<p class="mt-2 max-w-2xl text-sm leading-6 text-ink-muted">
			Output is streamed from the Wails v2 runner and retained by the backend so this view can
			reattach safely.
		</p>
	</div>
	<div class="flex items-center gap-2">
		<Badge
			tone={$jobRuntime.done?.state === 'success'
				? 'success'
				: $jobRuntime.done?.state === 'failed'
					? 'danger'
					: $jobRuntime.done?.state === 'cancelled'
						? 'warning'
						: 'accent'}
			dot
		>
			{$jobRuntime.done?.state || currentJob?.state || 'connecting'}
		</Badge>
		{#if currentJob}
			<Badge>{completedSteps}/{currentJob.stepCount} complete</Badge>
		{/if}
	</div>
</header>

{#if error && !currentJob}
	<div class="mt-6">
		<EmptyState title="No active job is available" description={error}>
			{#snippet action()}
				<Button variant="secondary" onclick={() => goto(resolve('/'))}>Choose a recipe</Button>
			{/snippet}
		</EmptyState>
	</div>
{:else}
	{#if wizard.snapshot.plan}
		<section
			class="mt-6 rounded-panel border border-line bg-panel/70 p-4"
			aria-label="Step progress"
		>
			<ol class="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
				{#each wizard.snapshot.plan.steps as step, index (step.id)}
					{@const state = stateForStep(step, index)}
					<li
						class={[
							'flex min-w-0 items-center gap-3 rounded-control border px-3 py-2.5',
							state === 'running' && 'border-brand/40 bg-brand-soft',
							state === 'success' && 'border-success/25 bg-success-soft',
							state === 'failed' && 'border-danger/30 bg-danger-soft',
							state === 'skipped' && 'border-line bg-panel-raised opacity-70',
							state === 'pending' && 'border-line bg-panel-raised/65'
						]}
					>
						<span
							class={[
								'flex size-6 shrink-0 items-center justify-center rounded-full border',
								state === 'running' && 'border-brand/40 text-brand-strong',
								state === 'success' && 'border-success/40 text-success',
								state === 'failed' && 'border-danger/40 text-danger',
								(state === 'pending' || state === 'skipped') && 'border-line-strong text-ink-faint'
							]}
						>
							{#if state === 'running'}
								<Icon name="refresh" class="size-3 animate-spin" />
							{:else if state === 'success'}
								<Icon name="check" class="size-3" />
							{:else if state === 'failed'}
								<Icon name="x" class="size-3" />
							{:else}
								<span class="font-mono text-[0.625rem]">{index + 1}</span>
							{/if}
						</span>
						<div class="min-w-0">
							<p class="truncate text-xs font-semibold text-ink">{step.label}</p>
							<p
								class="mt-0.5 text-[0.625rem] font-bold tracking-[0.08em] text-ink-faint uppercase"
							>
								{state}
							</p>
						</div>
					</li>
				{/each}
			</ol>
		</section>
	{/if}

	<section
		class="mt-4 overflow-hidden rounded-panel border border-line-strong bg-[#07090d] shadow-panel"
	>
		<header
			class="flex items-center justify-between gap-4 border-b border-line bg-panel px-3 py-2.5"
		>
			<div class="flex min-w-0 items-center gap-2">
				<span class="flex gap-1.5" aria-hidden="true">
					<span class="size-2.5 rounded-full bg-danger/70"></span>
					<span class="size-2.5 rounded-full bg-warning/70"></span>
					<span class="size-2.5 rounded-full bg-success/70"></span>
				</span>
				<span class="ml-2 truncate font-mono text-[0.6875rem] text-ink-faint">
					{$jobRuntime.lines.length.toLocaleString()} lines
				</span>
			</div>
			<Button
				variant="ghost"
				size="sm"
				onclick={() => (autoscroll = !autoscroll)}
				aria-pressed={autoscroll}
			>
				<Icon name={autoscroll ? 'check' : 'activity'} class="size-3.5" />
				{autoscroll ? 'Following output' : 'Follow output'}
			</Button>
		</header>
		<div class="h-[min(48vh,34rem)] min-h-80">
			<!--
				The console renders untrusted generator output. A boundary keeps a
				rendering failure here from taking down the run page, which is the one
				page that must keep reporting job state and offering Cancel.
			-->
			<svelte:boundary>
				<VirtualLogList
					lines={$jobRuntime.lines}
					{autoscroll}
					onAutoscrollChange={(enabled) => (autoscroll = enabled)}
				/>
				{#snippet failed(error, reset)}
					<div
						class="flex h-full flex-col items-center justify-center gap-3 rounded-control border border-line bg-canvas/65 px-6 text-center"
						role="alert"
					>
						<p class="text-sm text-ink-muted">
							The output console stopped rendering. The run itself is unaffected and the full log is
							still retained.
						</p>
						<p class="font-mono text-xs break-words text-ink-faint">{toErrorMessage(error)}</p>
						<Button variant="secondary" size="sm" onclick={reset}>Retry console</Button>
					</div>
				{/snippet}
			</svelte:boundary>
		</div>
	</section>

	<footer
		class="mt-4 flex flex-col gap-3 rounded-panel border border-line bg-panel/70 p-4 sm:flex-row sm:items-center sm:justify-between"
	>
		<div class="min-w-0">
			<p class="text-xs font-semibold text-ink">
				{initialising
					? 'Reattaching to output…'
					: terminal
						? 'The runner has stopped.'
						: 'You can leave this page and return without losing logs.'}
			</p>
			{#if error}<p class="mt-1 text-xs text-danger" role="alert">{error}</p>{/if}
		</div>
		<div class="flex items-center justify-end gap-2">
			{#if terminal}
				<Button onclick={() => goto(resolve('/result'))}>
					View result <Icon name="arrowRight" class="size-4" />
				</Button>
			{:else}
				<Button variant="danger" onclick={cancelJob} loading={cancelling}>
					{#snippet icon()}<Icon name="stop" class="size-4" />{/snippet}
					{cancelling ? 'Cancelling' : 'Cancel run'}
				</Button>
			{/if}
		</div>
	</footer>
{/if}
