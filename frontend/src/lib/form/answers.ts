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

/**
 * Checks one answer against the field's declared value constraints.
 *
 * This mirrors `recipe.CheckFieldConstraints` on the Go side. The planner
 * re-checks everything, so a divergence here costs a worse error message rather
 * than an unvalidated answer reaching argv.
 */
export function fieldConstraintError(field: RecipeField, value: unknown): string | undefined {
	if (field.type === 'text') {
		if (typeof value !== 'string') return `${field.label} is required.`;
		// Text fields are non-empty by default: an empty string reaching a
		// generator arrives where it expects a token.
		if (value.trim() === '') return `${field.label} is required.`;
		// Count characters, not UTF-16 code units, so an emoji or a CJK
		// character costs what the recipe author expects it to.
		const length = [...value].length;
		if (field.minLength !== null && length < field.minLength) {
			return `${field.label} must be at least ${field.minLength} characters.`;
		}
		if (field.maxLength !== null && length > field.maxLength) {
			return `${field.label} must be at most ${field.maxLength} characters.`;
		}
		if (field.pattern !== '') {
			let expression: RegExp;
			try {
				expression = new RegExp(field.pattern);
			} catch {
				// A pattern the backend compiled but JavaScript cannot is not the
				// user's problem: let the planner be the judge.
				return undefined;
			}
			if (!expression.test(value)) {
				return `${field.label} is not in the required format.`;
			}
		}
		return undefined;
	}

	if (field.type === 'number') {
		if (typeof value !== 'number' || !Number.isFinite(value)) {
			return `${field.label} must be a number.`;
		}
		if (field.min !== null && value < field.min) {
			return `${field.label} must be at least ${field.min}.`;
		}
		if (field.max !== null && value > field.max) {
			return `${field.label} must be at most ${field.max}.`;
		}
		return undefined;
	}

	if (field.required && value === undefined) {
		return `${field.label} is required.`;
	}
	return undefined;
}

export function validateRequiredText(
	fields: readonly RecipeField[],
	answers: Readonly<Record<string, unknown>>
): FieldErrors {
	const errors: FieldErrors = {};
	for (const field of fields) {
		const message = fieldConstraintError(field, answers[field.id]);
		if (message !== undefined) errors[field.id] = message;
	}
	return errors;
}

export function validateVisibleRequiredText(
	fields: readonly RecipeField[],
	answers: Readonly<Record<string, unknown>>
): FieldErrors {
	return validateRequiredText(visibleFields(fields, answers), answers);
}
