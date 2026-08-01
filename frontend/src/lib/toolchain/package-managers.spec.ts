import { describe, expect, it } from 'vitest';
import type { Tool, Toolchain } from '$lib/api';
import {
	availablePackageManagers,
	isUsableTool,
	repairPackageManagerSelection
} from './package-managers';

function tool(name: string, overrides: Partial<Tool> = {}): Tool {
	return {
		name,
		path: `/bin/${name}`,
		version: '1.0.0',
		present: true,
		error: '',
		...overrides
	};
}

function toolchain(tools: Tool[]): Toolchain {
	return {
		path: '/bin',
		detectedAt: '2026-07-28T12:00:00Z',
		tools,
		pathWarning: ''
	};
}

describe('package-manager availability', () => {
	it.each([
		['usable', tool('pnpm'), true],
		['not present', tool('pnpm', { present: false }), false],
		['has a detection error', tool('pnpm', { error: 'failed to execute' }), false],
		['has no version', tool('pnpm', { version: '' }), false],
		['was not detected', undefined, false]
	])('%s tool usability', (_label, candidate, expected) => {
		expect(isUsableTool(candidate)).toBe(expected);
	});

	it('intersects recipe order with detected usable tools', () => {
		const detected = toolchain([
			tool('npm'),
			tool('pnpm', { present: false }),
			tool('yarn'),
			tool('bun')
		]);

		expect(availablePackageManagers(['pnpm', 'yarn', 'npm'], detected)).toEqual(['yarn', 'npm']);
	});

	it('keeps an available selection and otherwise repairs to the first available option', () => {
		const detected = toolchain([tool('npm'), tool('pnpm')]);

		expect(repairPackageManagerSelection(['pnpm', 'npm'], detected, 'npm')).toBe('npm');
		expect(repairPackageManagerSelection(['pnpm', 'npm'], detected, 'yarn')).toBe('pnpm');
		expect(
			repairPackageManagerSelection(['pnpm'], toolchain([tool('pnpm', { version: '' })]), 'pnpm')
		).toBe('');
	});
});
