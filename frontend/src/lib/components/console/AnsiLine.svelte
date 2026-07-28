<script lang="ts">
	import type { LogLine } from '$lib/api';
	import { tokenizeAnsi } from './ansi';

	interface Props {
		line: LogLine;
	}

	let { line }: Props = $props();
	const tokens = $derived(tokenizeAnsi(line.text));
</script>

<div
	class={[
		'grid min-w-max grid-cols-[4.25rem_5.5rem_minmax(0,1fr)] px-3 font-mono text-xs leading-[1.375rem]',
		line.stream === 'stderr' ? 'bg-danger/5' : ''
	]}
>
	<span class="pr-3 text-right text-ink-faint select-none">{line.seq}</span>
	<span class={line.stream === 'stderr' ? 'text-danger/80' : 'text-info/80'}>{line.stepId}</span>
	<span class="whitespace-pre text-ink-muted">
		{#each tokens as token, index (`${index}-${token.classes}`)}
			<span class={token.classes}>{token.text}</span>
		{/each}
	</span>
</div>
