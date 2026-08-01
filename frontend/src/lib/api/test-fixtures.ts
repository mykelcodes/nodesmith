import type { RecipeFieldConstraints } from './types';

/**
 * The constraint fields a recipe field carries when it declares none.
 *
 * The backend always emits these keys — booleans and strings zero-valued,
 * optional bounds as null — so a test fixture that omits them is not shaped like
 * anything the bridge can actually produce. Spreading this keeps fixtures honest
 * without repeating six keys per field.
 */
export const noFieldConstraints: RecipeFieldConstraints = {
	required: false,
	pattern: '',
	minLength: null,
	maxLength: null,
	min: null,
	max: null
};
