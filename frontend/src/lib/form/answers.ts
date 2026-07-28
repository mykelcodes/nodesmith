import type { Answers, RecipeField } from '../api';
import { evaluateCondition } from './condition';

export type FieldErrors = Record<string, string>;

export function createDefaultAnswers(fields: readonly RecipeField[]): Answers {
	return Object.fromEntries(
		fields.map((field) => [
			field.id,
			Array.isArray(field.default) ? [...field.default] : field.default
		])
	);
}

export function isFieldVisible(
	field: RecipeField,
	answers: Readonly<Record<string, unknown>>
): boolean {
	return field.visibleIf.trim() === '' || evaluateCondition(field.visibleIf, answers);
}

export function visibleFields(
	fields: readonly RecipeField[],
	answers: Readonly<Record<string, unknown>>
): RecipeField[] {
	return fields.filter((field) => isFieldVisible(field, answers));
}

export function visibleAnswers(
	fields: readonly RecipeField[],
	answers: Readonly<Answers>
): Answers {
	const result: Answers = {};
	for (const field of visibleFields(fields, answers)) {
		const value = answers[field.id];
		if (value !== undefined) {
			result[field.id] = Array.isArray(value) ? [...value] : value;
		}
	}
	return result;
}

export function validateRequiredText(
	fields: readonly RecipeField[],
	answers: Readonly<Record<string, unknown>>
): FieldErrors {
	const errors: FieldErrors = {};
	for (const field of fields) {
		if (field.type === 'text') {
			const value = answers[field.id];
			if (typeof value !== 'string' || value.trim() === '') {
				errors[field.id] = `${field.label} is required.`;
			}
		}
	}
	return errors;
}

export function validateVisibleRequiredText(
	fields: readonly RecipeField[],
	answers: Readonly<Record<string, unknown>>
): FieldErrors {
	return validateRequiredText(visibleFields(fields, answers), answers);
}
