<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import {
		api,
		copyText,
		toErrorMessage,
		type Job,
		type JobDoneEvent,
		type Settings
	} from '$lib/api';
	import { WizardProgress } from '$lib/components/form';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { ResultSummary } from '$lib/components/result';
	import { Button, EmptyState } from '$lib/components/ui';
	import { jobRuntime, wizard } from '$lib/stores';

	let result = $state<JobDoneEvent | null>(wizard.snapshot.done || jobRuntime.snapshot.done);
	let settings = $state<Settings | null>(null);
	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let busyAction = $state('');

	const errorLines = $derived(
		$jobRuntime.lines.filter((line) => line.stream === 'stderr').slice(-10)
	);
	const editorLabel = $derived(
		settings?.editor === 'code'
			? 'VS Code'
			: settings?.editor === 'cursor'
				? 'Cursor'
				: settings?.editor === 'zed'
					? 'Zed'
					: 'editor'
	);

	function fromJob(job: Job): JobDoneEvent | null {
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

	async function runAction(name: string, action: () => Promise<void>, successMessage: string) {
		busyAction = name;
		error = '';
		notice = '';
		try {
			await action();
			notice = successMessage;
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			busyAction = '';
		}
	}

	async function openEditor(automatic = false) {
		if (!result || !settings) return;
		await runAction(
			'editor',
			() => api.scaffold.openInEditor(result!.projectDir, settings!.editor),
			automatic ? `Opened automatically in ${editorLabel}.` : `Opened in ${editorLabel}.`
		);
	}

	async function rehydrateLogs(jobId: string) {
		jobRuntime.begin(jobId);
		const lines = await api.scaffold.logs(jobId, 0);
		jobRuntime.replay(jobId, lines);
	}

	function formatFullLog(): string {
		return jobRuntime.snapshot.lines
			.map((line) => `[${line.stream}] [${line.stepId}] ${line.text}`)
			.join('\n');
	}

	async function copyFullLog() {
		if (!result) return;
		await runAction(
			'copy-log',
			async () => {
				await rehydrateLogs(result!.jobId);
				await copyText(formatFullLog());
			},
			'Full log copied.'
		);
	}

	async function initialise() {
		loading = true;
		error = '';
		try {
			settings = await api.store.getSettings();
			if (!result && wizard.snapshot.job?.id) {
				const job = await api.scaffold.status(wizard.snapshot.job.id);
				result = fromJob(job);
				if (result) {
					wizard.updateJob(job);
					wizard.setDone(result);
				}
			}
			if (!result) throw new Error('No completed project run is available.');
			await rehydrateLogs(result.jobId);

			if (result.state === 'success' && settings.openAfterCreate) {
				const key = `nodesmith:opened:${result.jobId}`;
				if (!window.sessionStorage.getItem(key)) {
					window.sessionStorage.setItem(key, 'true');
					await openEditor(true);
				}
			}
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			loading = false;
		}
	}

	function createAnother() {
		jobRuntime.clear();
		wizard.reset();
		void goto(resolve('/'));
	}

	function retry() {
		jobRuntime.clear();
		wizard.resetRun();
		void goto(resolve('/review'));
	}

	onMount(() => {
		void initialise();
	});
</script>

<svelte:head>
	<title>Project result · Nodesmith</title>
</svelte:head>

<WizardProgress current="result" />

{#if loading}
	<div
		class="mt-8 h-80 animate-pulse rounded-panel border border-line bg-panel/70"
		aria-busy="true"
	></div>
{:else if !result}
	<div class="mt-8">
		<EmptyState
			title="No completed run"
			description={error || 'Create a project to see its result.'}
		>
			{#snippet icon()}<Icon name="history" class="size-5" />{/snippet}
			{#snippet action()}
				<Button variant="secondary" onclick={() => goto(resolve('/'))}>Choose a recipe</Button>
			{/snippet}
		</EmptyState>
	</div>
{:else}
	<div class="mt-8">
		<ResultSummary
			{result}
			{errorLines}
			{editorLabel}
			{busyAction}
			onOpenEditor={() => openEditor(false)}
			onReveal={() =>
				runAction(
					'reveal',
					() => api.scaffold.revealInFileManager(result!.projectDir),
					'Revealed the project in your file manager.'
				)}
			onCopyPath={() =>
				runAction('copy-path', () => copyText(result!.projectDir), 'Project path copied.')}
			onCopyLog={copyFullLog}
			onCreateAnother={createAnother}
			onRetry={retry}
		/>

		{#if notice}
			<p
				class="mt-4 rounded-control border border-success/25 bg-success-soft px-4 py-3 text-sm text-success"
				role="status"
			>
				{notice}
			</p>
		{/if}
		{#if error}
			<p
				class="mt-4 rounded-control border border-danger/30 bg-danger-soft px-4 py-3 text-sm text-danger"
				role="alert"
			>
				{error}
			</p>
		{/if}
	</div>
{/if}
