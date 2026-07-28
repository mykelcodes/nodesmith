import { describe, expect, it } from 'vitest';
import { evaluateCondition, parseCondition } from './condition';

describe('condition grammar', () => {
	it('parses every supported expression form', () => {
		expect(parseCondition('enabled')).toEqual({
			identifier: 'enabled',
			operator: '',
			negated: false
		});
		expect(parseCondition(' ! enabled ')).toEqual({
			identifier: 'enabled',
			operator: '',
			negated: true
		});
		expect(parseCondition('template!="library"')).toMatchObject({
			identifier: 'template',
			operator: '!=',
			literal: 'library'
		});
		expect(parseCondition('count == -1.25e2')).toMatchObject({
			identifier: 'count',
			operator: '==',
			literal: -125
		});
		expect(parseCondition('addons includes "vitest"')).toMatchObject({
			identifier: 'addons',
			operator: 'includes',
			literal: 'vitest'
		});
	});

	it.each([
		'',
		'!',
		'1field',
		'enabled nope true',
		'!enabled == true',
		'enabled ==',
		'enabled == "unterminated',
		'enabled == null',
		'enabled == []',
		'enabled == true false',
		'addons includesThing true',
		'addonsincludes "vitest"'
	])('rejects malformed expression %j', (expression) => {
		expect(() => parseCondition(expression)).toThrow(SyntaxError);
	});
});

describe('condition evaluation', () => {
	it('matches truthiness, equality and negation semantics', () => {
		expect(evaluateCondition('enabled', { enabled: true })).toBe(true);
		expect(evaluateCondition('!name', { name: '' })).toBe(true);
		expect(evaluateCondition('count == 2', { count: 2 })).toBe(true);
		expect(evaluateCondition('count == "2"', { count: 2 })).toBe(false);
		expect(evaluateCondition('template != "library"', { template: 'demo' })).toBe(true);
	});

	it('uses element equality for includes rather than string substring matching', () => {
		expect(evaluateCondition('addons includes "vitest"', { addons: ['eslint', 'vitest'] })).toBe(
			true
		);
		expect(evaluateCondition('addons includes "test"', { addons: ['vitest'] })).toBe(false);
		expect(evaluateCondition('choices includes true', { choices: [false, true] })).toBe(true);
		expect(() => evaluateCondition('addons includes "x"', { addons: 'x' })).toThrow(
			/requires .* multiselect/
		);
	});

	it('rejects unknown identifiers rather than silently hiding a field', () => {
		expect(() => evaluateCondition('missing', {})).toThrow(/Unknown condition identifier/);
	});
});
