<script lang="ts">
	import Icon from '$lib/components/icons/Icon.svelte';

	type WizardStep = 'configure' | 'review' | 'run' | 'result';

	interface Props {
		current: WizardStep;
	}

	const steps: readonly { id: WizardStep; label: string }[] = [
		{ id: 'configure', label: 'Configure' },
		{ id: 'review', label: 'Review' },
		{ id: 'run', label: 'Run' },
		{ id: 'result', label: 'Result' }
	];

	let { current }: Props = $props();
	const currentIndex = $derived(steps.findIndex((step) => step.id === current));
</script>

<nav aria-label="Project creation progress" class="overflow-x-auto pb-1">
	<ol class="flex min-w-[29rem] items-center">
		{#each steps as step, index (step.id)}
			<li
				class="flex min-w-0 flex-1 items-center"
				aria-current={step.id === current ? 'step' : undefined}
			>
				<div class="flex items-center gap-2.5">
					<span
						class={[
							'flex size-7 shrink-0 items-center justify-center rounded-full border text-xs font-bold transition-colors',
							index < currentIndex && 'border-success/40 bg-success-soft text-success',
							index === currentIndex && 'border-brand/55 bg-brand-soft text-brand-strong',
							index > currentIndex && 'border-line-strong bg-panel-raised text-ink-faint'
						]}
					>
						{#if index < currentIndex}
							<Icon name="check" class="size-3.5" />
							<span class="sr-only">Completed</span>
						{:else}
							{index + 1}
						{/if}
					</span>
					<span
						class={[
							'text-xs font-semibold whitespace-nowrap',
							index === currentIndex ? 'text-ink' : 'text-ink-faint'
						]}
					>
						{step.label}
					</span>
				</div>
				{#if index < steps.length - 1}
					<span
						class={[
							'mx-3 h-px min-w-4 flex-1',
							index < currentIndex ? 'bg-success/45' : 'bg-line-strong'
						]}
						aria-hidden="true"
					></span>
				{/if}
			</li>
		{/each}
	</ol>
</nav>
