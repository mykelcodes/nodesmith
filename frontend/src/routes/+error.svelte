<script lang="ts">
	// Without this file an uncaught error in any route renders SvelteKit's
	// default error page: unstyled, unbranded, and with no way back into the app.
	// Every route calls across the Wails bridge, and ensureBinding throws
	// NodesmithBridgeError when a binding is stale, so this page is reachable in
	// normal use rather than only in development.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Button from '$lib/components/ui/Button.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Icon from '$lib/components/icons/Icon.svelte';

	const status = $derived(page.status);
	const message = $derived(page.error?.message?.trim() || 'An unexpected error occurred.');

	const title = $derived(
		status === 404 ? 'That page does not exist' : 'Something went wrong in Nodesmith'
	);

	// A bridge failure is the most likely cause and is not the user's fault, so
	// the copy points at the actionable next step rather than at the stack.
	const description = $derived(
		status === 404
			? 'The page you tried to open is not part of Nodesmith. Head back to the recipe catalogue to start a project.'
			: 'Nodesmith could not finish rendering this page. Any project already being created keeps running in the background.'
	);
</script>

<svelte:head>
	<title>{title} · Nodesmith</title>
</svelte:head>

<div class="mx-auto w-full max-w-2xl py-6">
	<EmptyState {title} {description}>
		{#snippet icon()}
			<Icon name="triangleAlert" class="size-5" />
		{/snippet}
		{#snippet action()}
			<div class="flex flex-col items-center gap-4">
				<div class="flex flex-wrap justify-center gap-2">
					<Button onclick={() => goto(resolve('/'))}>Back to catalogue</Button>
					<Button variant="secondary" onclick={() => location.reload()}>Reload</Button>
				</div>
				{#if status !== 404}
					<p
						class="max-w-md rounded-control border border-line bg-canvas/65 px-3 py-2 font-mono text-xs leading-5 break-words text-ink-faint"
					>
						{status}: {message}
					</p>
				{/if}
			</div>
		{/snippet}
	</EmptyState>
</div>
