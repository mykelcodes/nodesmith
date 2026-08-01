<script lang="ts">
	import type { JobDoneEvent, LogLine } from '$lib/api';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Badge, Button } from '$lib/components/ui';

	interface Props {
		result: JobDoneEvent;
		errorLines: readonly LogLine[];
		editorLabel: string;
		busyAction?: string;
		onOpenEditor: () => void;
		onReveal: () => void;
		onCopyPath: () => void;
		onCopyLog: () => void;
		onCreateAnother: () => void;
		onRetry: () => void;
	}

	let {
		result,
		errorLines,
		editorLabel,
		busyAction = '',
		onOpenEditor,
		onReveal,
		onCopyPath,
		onCopyLog,
		onCreateAnother,
		onRetry
	}: Props = $props();

	const succeeded = $derived(result.state === 'success');
	const cancelled = $derived(result.state === 'cancelled');
	const duration = $derived(
		result.durationMs < 1000
			? `${result.durationMs} ms`
			: result.durationMs < 60_000
				? `${(result.durationMs / 1000).toFixed(1)} s`
				: `${Math.floor(result.durationMs / 60_000)}m ${Math.round((result.durationMs % 60_000) / 1000)}s`
	);
</script>

<section
	class={[
		'overflow-hidden rounded-panel border shadow-panel',
		succeeded && 'border-success/30 bg-success-soft/40',
		cancelled && 'border-warning/30 bg-warning-soft/35',
		!succeeded && !cancelled && 'border-danger/30 bg-danger-soft/35'
	]}
>
	<div class="flex flex-col items-start gap-5 p-6 sm:flex-row sm:p-8">
		<div
			class={[
				'flex size-14 shrink-0 items-center justify-center rounded-2xl border',
				succeeded && 'border-success/35 bg-success-soft text-success',
				cancelled && 'border-warning/35 bg-warning-soft text-warning',
				!succeeded && !cancelled && 'border-danger/35 bg-danger-soft text-danger'
			]}
		>
			<Icon name={succeeded ? 'circleCheck' : cancelled ? 'stop' : 'circleX'} class="size-7" />
		</div>
		<div class="min-w-0 flex-1">
			<Badge tone={succeeded ? 'success' : cancelled ? 'warning' : 'danger'} dot>
				{result.state}
			</Badge>
			<h1 class="mt-3 text-3xl font-bold tracking-[-0.04em] text-ink">
				{succeeded
					? 'Project created successfully'
					: cancelled
						? 'Project creation cancelled'
						: 'Project creation failed'}
			</h1>
			<p class="mt-2 max-w-2xl text-sm leading-6 text-ink-muted">
				{succeeded
					? `Nodesmith finished every planned step in ${duration}.`
					: result.error || 'The runner stopped before completing the execution plan.'}
			</p>

			<div
				class="mt-5 flex min-w-0 items-center gap-2 rounded-control border border-line bg-canvas/55 px-3 py-2.5"
			>
				<Icon name="folder" class="size-4 shrink-0 text-brand-strong" />
				<code class="min-w-0 flex-1 truncate font-mono text-xs text-ink" title={result.projectDir}>
					{result.projectDir}
				</code>
				<Button
					variant="ghost"
					size="icon"
					class="size-7"
					onclick={onCopyPath}
					aria-label="Copy project path"
					loading={busyAction === 'copy-path'}
				>
					{#snippet icon()}<Icon name="copy" class="size-3.5" />{/snippet}
					<span class="sr-only">Copy project path</span>
				</Button>
			</div>
		</div>
	</div>

	<div class="flex flex-wrap gap-2 border-t border-line bg-panel/55 px-6 py-4 sm:px-8">
		{#if succeeded}
			<Button onclick={onOpenEditor} loading={busyAction === 'editor'}>
				{#snippet icon()}<Icon name="external" class="size-4" />{/snippet}
				Open in {editorLabel}
			</Button>
			<Button variant="secondary" onclick={onReveal} loading={busyAction === 'reveal'}>
				{#snippet icon()}<Icon name="folder" class="size-4" />{/snippet}
				Reveal in files
			</Button>
		{:else}
			<Button onclick={onRetry}>
				<Icon name="refresh" class="size-4" />
				Retry with same answers
			</Button>
			<Button variant="secondary" onclick={onCopyLog} loading={busyAction === 'copy-log'}>
				{#snippet icon()}<Icon name="copy" class="size-4" />{/snippet}
				Copy full log
			</Button>
		{/if}
		<Button variant="ghost" onclick={onCreateAnother}>
			<Icon name="plus" class="size-4" />
			Create another
		</Button>
	</div>
</section>

{#if !succeeded}
	<section class="mt-6 overflow-hidden rounded-panel border border-line bg-panel/75">
		<header class="flex items-center justify-between border-b border-line px-4 py-3">
			<div>
				<h2 class="text-sm font-semibold text-ink">Last error output</h2>
				<p class="mt-0.5 text-xs text-ink-faint">
					The most recent stderr lines retained in this session.
				</p>
			</div>
			<Badge tone="danger">{errorLines.length} lines</Badge>
		</header>
		<div class="max-h-64 overflow-auto bg-[#07090d] p-4">
			{#if errorLines.length === 0}
				<p class="font-mono text-xs text-ink-faint">No stderr output was captured.</p>
			{:else}
				{#each errorLines as line (line.seq)}
					<p class="font-mono text-xs leading-5 whitespace-pre-wrap text-danger/90">
						<span class="mr-3 text-ink-faint select-none">{line.seq}</span>{line.text}
					</p>
				{/each}
			{/if}
		</div>
	</section>
{/if}
