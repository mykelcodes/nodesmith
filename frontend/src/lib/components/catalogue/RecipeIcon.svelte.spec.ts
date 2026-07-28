import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import RecipeIcon from './RecipeIcon.svelte';

describe('RecipeIcon', () => {
	it('renders a registered manifest icon', async () => {
		render(RecipeIcon, { icon: 'react', label: 'React recipe' });

		await expect
			.element(page.getByRole('img', { name: 'React recipe' }))
			.toHaveAttribute('data-recipe-icon', 'react');
	});

	it('uses the local fallback for an unknown user recipe icon', async () => {
		render(RecipeIcon, { icon: 'private-stack', label: 'Private stack recipe' });

		await expect
			.element(page.getByRole('img', { name: 'Private stack recipe' }))
			.toHaveAttribute('data-recipe-icon', 'fallback');
	});
});
