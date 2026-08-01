import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import CardGrid from './__card-grid.test.svelte';

/**
 * Catalogue cards must line up regardless of how much content each recipe has.
 *
 * Equal height comes from `auto-rows-fr` on the grid plus `h-full` on the card,
 * which is easy to drop by accident while restyling. Measuring the rendered
 * boxes catches that in a way a class-name assertion cannot.
 */
describe('catalogue card layout', () => {
	it('gives every card the same height whatever its content', async () => {
		render(CardGrid);
		await new Promise((resolve) => setTimeout(resolve, 150));

		const cards = [...document.querySelectorAll('[data-testid="grid"] article')];
		expect(cards.length).toBe(4);

		const heights = cards.map((card) => Math.round(card.getBoundingClientRect().height));
		expect(new Set(heights).size).toBe(1);
	});

	// The action row is the card's baseline: if it floats with content length the
	// grid reads as ragged even when the outer boxes match.
	it('keeps the footer pinned to the bottom of every card', async () => {
		render(CardGrid);
		await new Promise((resolve) => setTimeout(resolve, 150));

		const cards = [...document.querySelectorAll('[data-testid="grid"] article')];
		const gaps = cards.map((card) => {
			const footer = card.querySelector('.border-t') as HTMLElement;
			return Math.round(
				card.getBoundingClientRect().bottom - footer.getBoundingClientRect().bottom
			);
		});
		expect(new Set(gaps).size).toBe(1);
	});
});

/**
 * A layered base stylesheet is what lets a component turn the default focus
 * indicator off. While the base rules were unlayered they beat every Tailwind
 * utility, so a rounded control still drew a square outline and no class could
 * stop it.
 */
describe('focus indicator', () => {
	it('lets utilities override the default focus ring', async () => {
		render(CardGrid);
		await new Promise((resolve) => setTimeout(resolve, 150));

		const link = document.getElementById('focus-probe') as HTMLElement;
		link.focus();
		const style = getComputedStyle(link);

		// The base rule draws an outline; a component asking for a ring must win.
		expect(style.outlineStyle).toBe('none');
		// Tailwind's `focus-visible:ring-3` renders as a 3px spread box-shadow.
		// The base fallback would be a 4px spread instead, so this distinguishes
		// which rule actually applied.
		expect(style.boxShadow).toContain('0px 0px 0px 3px');
		expect(style.boxShadow).not.toContain('0px 0px 0px 4px');
	});
});
