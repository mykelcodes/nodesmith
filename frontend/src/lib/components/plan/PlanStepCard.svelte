<script lang="ts">
	import type { PlanStep } from '$lib/api';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Badge, Button } from '$lib/components/ui';

	interface Props {
		step: PlanStep;
		index: number;
		onCopy: (display: string) => void;
		copied?: boolean;
	}

	let { step, index, onCopy, copied = false }: Props = $props();
	const isConfig = $derived(step.kind === 'project-config' && step.config !== null);
</script>

<article class="overflow-hidden rounded-panel border border-line bg-panel/80 shadow-sm">
	<header
		class="flex items-center justify-between gap-4 border-b border-line bg-panel-raised/65 px-4 py-3"
	>
		<div class="flex min-w-0 items-center gap-3">
			<span
				class="flex size-7 shrink-0 items-center justify-center rounded-lg border border-brand/25 bg-brand-soft font-mono text-xs font-bold text-brand-strong"
			>
				{index + 1}
			</span>
			<div class="min-w-0">
				<h2 class="truncate text-sm font-semibold text-ink">{step.label}</h2>
				<p class="mt-0.5 truncate font-mono text-[0.6875rem] text-ink-faint">{step.dir}</p>
			</div>
		</div>
		<Badge size="sm">{step.id}</Badge>
	</header>

	<div class="p-4">
		<div class="flex items-start gap-3 rounded-control border border-line bg-canvas/75 p-3">
			<Icon
				name={isConfig ? 'settings' : 'terminal'}
				class="mt-0.5 size-4 shrink-0 text-brand-strong"
			/>
			<code class="min-w-0 flex-1 overflow-x-auto font-mono text-xs leading-5 text-ink">
				{step.display}
			</code>
			<Button
				variant="ghost"
				size="icon"
				class="-m-1 size-7"
				onclick={() => onCopy(step.display)}
				aria-label={`Copy ${isConfig ? 'configuration details' : 'command'} for ${step.label}`}
			>
				<Icon name={copied ? 'check' : 'copy'} class="size-3.5" />
			</Button>
		</div>

		<details class="mt-3 text-xs text-ink-muted">
			<summary
				class="w-fit cursor-pointer rounded-md font-semibold transition-colors hover:text-ink focus-visible:ring-3 focus-visible:ring-brand/25 focus-visible:outline-none"
			>
				{isConfig ? 'Configuration edit' : 'Resolved arguments'}
			</summary>
			<dl class="mt-3 grid gap-2 border-l border-line pl-3">
				{#if step.config}
					<div class="grid grid-cols-[4.5rem_minmax(0,1fr)] gap-3">
						<dt class="text-ink-faint">File</dt>
						<dd class="truncate font-mono text-ink">{step.config.path}</dd>
					</div>
					<div class="grid grid-cols-[4.5rem_minmax(0,1fr)] gap-3">
						<dt class="text-ink-faint">Setting</dt>
						<dd class="font-mono break-all text-ink">
							{step.config.section ? `${step.config.section}.` : ''}{step.config.key} = {step.config
								.value}
						</dd>
					</div>
				{:else}
					<div class="grid grid-cols-[4.5rem_minmax(0,1fr)] gap-3">
						<dt class="text-ink-faint">Binary</dt>
						<dd class="truncate font-mono text-ink" title={step.bin}>{step.bin}</dd>
					</div>
					<div class="grid grid-cols-[4.5rem_minmax(0,1fr)] gap-3">
						<dt class="text-ink-faint">Argv</dt>
						<dd class="font-mono text-ink">
							{#each step.args as argument, argumentIndex (argumentIndex)}
								<div class="break-all">
									<span class="mr-2 text-ink-faint">{argumentIndex}</span>{argument}
								</div>
							{/each}
						</dd>
					</div>
				{/if}
			</dl>
		</details>
	</div>
</article>
