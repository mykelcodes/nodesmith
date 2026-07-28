<script lang="ts">
	import { tick } from 'svelte';
	import type { LogLine } from '$lib/api';
	import AnsiLine from './AnsiLine.svelte';

	interface Props {
		lines: readonly LogLine[];
		autoscroll?: boolean;
		onAutoscrollChange?: (enabled: boolean) => void;
	}

	const rowHeight = 22;
	const overscan = 14;

	let { lines, autoscroll = true, onAutoscrollChange }: Props = $props();
	let viewport: HTMLDivElement;
	let scrollTop = $state(0);
	let viewportHeight = $state(440);

	const startIndex = $derived(Math.max(0, Math.floor(scrollTop / rowHeight) - overscan));
	const visibleCount = $derived(Math.ceil(viewportHeight / rowHeight) + overscan * 2);
	const endIndex = $derived(Math.min(lines.length, startIndex + visibleCount));
	const visibleLines = $derived(lines.slice(startIndex, endIndex));

	function handleScroll() {
		scrollTop = viewport.scrollTop;
		viewportHeight = viewport.clientHeight;
		const distanceFromBottom = viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
		const nextAutoscroll = distanceFromBottom < rowHeight * 3;
		if (nextAutoscroll !== autoscroll) onAutoscrollChange?.(nextAutoscroll);
	}

	$effect(() => {
		const hasLines = lines.length > 0;
		if (!hasLines || !autoscroll || !viewport) return;
		void tick().then(() => {
			viewport.scrollTop = viewport.scrollHeight;
		});
	});
</script>

<div
	bind:this={viewport}
	class="h-full min-h-80 overflow-auto overscroll-contain bg-[#07090d]"
	role="log"
	aria-label="Scaffold output"
	aria-live="off"
	onscroll={handleScroll}
>
	{#if lines.length === 0}
		<div class="flex h-full min-h-80 items-center justify-center px-6 text-center">
			<p class="font-mono text-xs text-ink-faint">Waiting for process output…</p>
		</div>
	{:else}
		<div class="relative min-w-max" style:height={`${lines.length * rowHeight}px`}>
			<div
				class="absolute right-0 left-0"
				style:transform={`translateY(${startIndex * rowHeight}px)`}
			>
				{#each visibleLines as line (line.seq)}
					<AnsiLine {line} />
				{/each}
			</div>
		</div>
	{/if}
</div>
