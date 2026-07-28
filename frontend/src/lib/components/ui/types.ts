export interface FieldContext {
	controlId: string;
	describedBy: string | undefined;
	invalid: boolean;
}

export interface SelectOption {
	value: string;
	label: string;
	disabled?: boolean;
}

export interface MultiSelectOption {
	value: string;
	label: string;
	description?: string;
	disabled?: boolean;
}
