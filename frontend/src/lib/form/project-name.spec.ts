import { describe, expect, it } from 'vitest';
import { validatePortableProjectName } from './project-name';

describe('portable project-name validation', () => {
	it.each([
		['app', true],
		['a b', true],
		['café-项目', true],
		['package.name', true],
		['.hidden', false],
		['..', false],
		['.', false],
		['con', false],
		['CON.txt', false],
		['CONOUT$', false],
		['lpt9.log', false],
		['COM¹', false],
		['com0', true],
		['a/b', false],
		['a\\b', false],
		['a:b', false],
		['a"b', false],
		[' leading', false],
		['trailing ', false],
		['trailing.', false],
		['line\nbreak', false],
		['a'.repeat(300), false]
	])('mirrors the backend rule for %j', (name, valid) => {
		expect(validatePortableProjectName(name) === '').toBe(valid);
	});

	it('measures the portable limit in UTF-8 bytes rather than JavaScript characters', () => {
		expect(validatePortableProjectName('é'.repeat(127))).toBe('');
		expect(validatePortableProjectName('é'.repeat(128))).toMatch(/255 UTF-8 bytes/);
	});
});
