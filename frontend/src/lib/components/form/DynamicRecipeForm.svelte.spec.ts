import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Recipe, ScaffoldRequest } from '$lib/api';
import DynamicRecipeForm from './DynamicRecipeForm.svelte';

const recipe: Recipe = {
	schemaVersion: 1,
	id: 'test',
	name: 'Test recipe',
	category: 'test',
	description: 'A test recipe.',
	docsUrl: 'https://example.com',
	tags: [],
	icon: 'test',
	verifiedAt: '2026-07-28',
	installPolicy: 'optional',
	minimumReleaseAge: null,
	requires: {
		node: '>=20',
		packageManagers: ['pnpm', 'npm'],
		tools: []
	},
	fields: [
		{
			id: 'typescript',
			label: 'Use TypeScript',
			type: 'boolean',
			default: false,
			help: '',
			options: [],
			visibleIf: ''
		},
		{
			id: 'scope',
			label: 'Organisation scope',
			type: 'text',
			default: '',
			help: 'Required when TypeScript is selected.',
			options: [],
			visibleIf: 'typescript'
		}
	],
	steps: [],
	available: true,
	unavailableReasons: []
};

const request: ScaffoldRequest = {
	recipeId: 'test',
	projectName: '',
	parentDir: '/projects',
	packageManager: 'pnpm',
	installDeps: true,
	gitInit: true,
	minimumReleaseAge: null,
	answers: {
		typescript: false,
		scope: ''
	}
};

describe('DynamicRecipeForm', () => {
	it('renders only detected package managers and exposes real required controls', async () => {
		render(DynamicRecipeForm, {
			recipe,
			request,
			packageManagers: ['pnpm'],
			errors: { packageManager: 'Choose an available package manager.' },
			onPickDirectory: vi.fn()
		});

		await expect.element(page.getByLabelText('Project name')).toHaveAttribute('required');
		await expect
			.element(page.getByLabelText('Project name'))
			.toHaveAttribute('aria-required', 'true');
		await expect.element(page.getByPlaceholder('/path/to/projects')).toHaveAttribute('required');
		await expect
			.element(page.getByRole('combobox', { name: /^Package manager/ }))
			.toHaveAttribute('required');
		await expect.element(page.getByRole('option', { name: /^pnpm$/ })).toBeInTheDocument();
		await expect.element(page.getByRole('option', { name: /^npm$/ })).not.toBeInTheDocument();
		await expect
			.element(page.getByText('Choose an available package manager.'))
			.toBeInTheDocument();
	});

	it('reactively reveals a conditional required manifest field', async () => {
		render(DynamicRecipeForm, {
			recipe,
			request,
			packageManagers: ['pnpm'],
			onPickDirectory: vi.fn()
		});

		await expect.element(page.getByLabelText('Organisation scope')).not.toBeInTheDocument();
		await page.getByRole('switch', { name: 'Use TypeScript' }).click();
		await expect.element(page.getByLabelText('Organisation scope')).toBeInTheDocument();
		await expect.element(page.getByLabelText('Organisation scope')).toHaveAttribute('required');
		await expect
			.element(page.getByLabelText('Organisation scope'))
			.toHaveAttribute('aria-required', 'true');
	});
});
