import { describe, expect, it } from 'vitest';
import type { Answers, RecipeField } from '../api';
import {
	createDefaultAnswers,
	validateVisibleRequiredText,
	visibleAnswers,
	visibleFields
} from './answers';

const fields: RecipeField[] = [
	{
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
		id: 'description',
		label: 'Description',
		type: 'text',
		default: '',
		help: '',
		options: [],
		visibleIf: 'template != "library"'
	},
	{
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
