import { describe, expect, it } from 'vitest';
import type { Answers, RecipeField } from '../api';
import { noFieldConstraints } from '../api/test-fixtures';
import {
	createDefaultAnswers,
	fieldConstraintError,
	validateVisibleRequiredText,
	visibleAnswers,
	visibleFields
} from './answers';

const fields: RecipeField[] = [
	{
		...noFieldConstraints,
		id: 'template',
		label: 'Template',
		type: 'select',
		default: 'demo',
		help: '',
		options: [
			{ value: 'demo', label: 'Demo' },
			{ value: 'library', label: 'Library' }
		],
		visibleIf: ''
	},
	{
		...noFieldConstraints,
		id: 'description',
		label: 'Description',
		type: 'text',
		default: '',
		help: '',
		options: [],
		visibleIf: 'template != "library"'
	},
	{
		...noFieldConstraints,
		id: 'addons',
		label: 'Add-ons',
		type: 'multiselect',
		default: ['vitest'],
		help: '',
		options: [{ value: 'vitest', label: 'Vitest' }],
		visibleIf: 'template != "library"'
	}
];

describe('dynamic form answers', () => {
	it('constructs cloned defaults for every data-driven field', () => {
		const first = createDefaultAnswers(fields);
		const second = createDefaultAnswers(fields);
		expect(first).toEqual({ template: 'demo', description: '', addons: ['vitest'] });
		expect(first.addons).not.toBe(second.addons);
	});

	it('retains hidden values in state while excluding them from a request', () => {
		const answers: Answers = {
			template: 'library',
			description: 'Keep me',
			addons: ['vitest']
		};

		expect(visibleFields(fields, answers).map((field) => field.id)).toEqual(['template']);
		expect(visibleAnswers(fields, answers)).toEqual({ template: 'library' });
		expect(answers).toEqual({
			template: 'library',
			description: 'Keep me',
			addons: ['vitest']
		});

		answers.template = 'demo';
		expect(visibleAnswers(fields, answers)).toEqual(answers);
	});

	it('validates visible text fields and ignores hidden text fields', () => {
		expect(
			validateVisibleRequiredText(fields, {
				template: 'demo',
				description: '   ',
				addons: []
			})
		).toEqual({ description: 'Description is required.' });
		expect(
			validateVisibleRequiredText(fields, {
				template: 'library',
				description: '',
				addons: []
			})
		).toEqual({});
	});
});

describe('field value constraints', () => {
	function textField(overrides: Partial<RecipeField> = {}): RecipeField {
		return {
			...noFieldConstraints,
			id: 'scope',
			label: 'Scope',
			type: 'text',
			default: '',
			help: '',
			options: [],
			visibleIf: '',
			...overrides
		} as RecipeField;
	}

	function numberField(overrides: Partial<RecipeField> = {}): RecipeField {
		return {
			...noFieldConstraints,
			id: 'port',
			label: 'Port',
			type: 'number',
			default: 3000,
			help: '',
			options: [],
			visibleIf: '',
			...overrides
		} as RecipeField;
	}

	it('rejects an empty text answer', () => {
		expect(fieldConstraintError(textField(), '')).toContain('required');
		expect(fieldConstraintError(textField(), '   ')).toContain('required');
	});

	it('applies declared length bounds', () => {
		const field = textField({ minLength: 3, maxLength: 6 });
		expect(fieldConstraintError(field, 'ab')).toContain('at least 3');
		expect(fieldConstraintError(field, 'abcdefg')).toContain('at most 6');
		expect(fieldConstraintError(field, 'abcd')).toBeUndefined();
	});

	// Length is counted in characters, so an astral-plane emoji costs one, not
	// the two UTF-16 code units String.length would report.
	it('counts characters rather than UTF-16 code units', () => {
		const field = textField({ maxLength: 2 });
		expect(fieldConstraintError(field, '👍👍')).toBeUndefined();
		expect(fieldConstraintError(field, '👍👍👍')).toContain('at most 2');
	});

	it('applies a declared pattern', () => {
		const field = textField({ pattern: '^[a-z][a-z0-9-]*$' });
		expect(fieldConstraintError(field, 'my-app')).toBeUndefined();
		expect(fieldConstraintError(field, 'My App')).toContain('required format');
		// The realistic failure: a value the generator would read as a flag.
		expect(fieldConstraintError(field, '-rf')).toContain('required format');
	});

	// The planner is authoritative, so a pattern JavaScript cannot compile must
	// not block the user here.
	it('defers to the planner when a pattern will not compile in JavaScript', () => {
		expect(fieldConstraintError(textField({ pattern: '(?P<name>x)' }), 'anything')).toBeUndefined();
	});

	it('applies inclusive number bounds', () => {
		const field = numberField({ min: 1024, max: 65535 });
		expect(fieldConstraintError(field, 80)).toContain('at least 1024');
		expect(fieldConstraintError(field, 70000)).toContain('at most 65535');
		expect(fieldConstraintError(field, 1024)).toBeUndefined();
		expect(fieldConstraintError(field, 65535)).toBeUndefined();
		expect(fieldConstraintError(field, Number.NaN)).toContain('must be a number');
	});

	it('leaves unconstrained non-text fields alone', () => {
		const field: RecipeField = {
			...noFieldConstraints,
			id: 'addons',
			label: 'Add-ons',
			type: 'multiselect',
			default: [],
			help: '',
			options: [],
			visibleIf: ''
		};
		expect(fieldConstraintError(field, undefined)).toBeUndefined();
		expect(fieldConstraintError({ ...field, required: true }, undefined)).toContain('required');
		expect(fieldConstraintError({ ...field, required: true }, ['vitest'])).toBeUndefined();
	});
});
