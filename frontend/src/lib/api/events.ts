import { EventsOff, EventsOn } from '../wailsjs/runtime/runtime.js';
import {
	parseJobDoneEvent,
	parseJobLogEvent,
	parseJobStartedEvent,
	parseJobStepEvent,
	parseRecipesReloadedEvent,
	parseToolchain
} from './parse';
import type { NodesmithEventMap, NodesmithEventName } from './types';

type EventParserMap = {
	[Name in NodesmithEventName]: (value: unknown, path?: string) => NodesmithEventMap[Name];
};

type EventRuntimeWindow = Window & {
	runtime?: {
		EventsOnMultiple?: unknown;
		EventsOff?: unknown;
	};
};

const parsers: EventParserMap = {
	'nodesmith:job:started': parseJobStartedEvent,
	'nodesmith:job:step': parseJobStepEvent,
	'nodesmith:job:log': parseJobLogEvent,
	'nodesmith:job:done': parseJobDoneEvent,
	'nodesmith:toolchain:changed': parseToolchain,
	'nodesmith:recipes:reloaded': parseRecipesReloadedEvent
};

function ensureEventRuntime(action: string): void {
	const runtime =
		typeof window === 'undefined' ? undefined : (window as EventRuntimeWindow).runtime;
	if (typeof runtime?.EventsOnMultiple !== 'function' || typeof runtime.EventsOff !== 'function') {
		throw new Error(
			`${action}: the Wails v2 event runtime is unavailable — run this UI through the Nodesmith desktop application`
		);
	}
}

export type Unsubscribe = () => void;

export function onNodesmithEvent<Name extends NodesmithEventName>(
	name: Name,
	listener: (payload: NodesmithEventMap[Name]) => void
): Unsubscribe {
	ensureEventRuntime(`Subscribe to ${name}`);
	const unsubscribe = EventsOn(name, (...data: unknown[]) => {
		if (data.length !== 1) {
			throw new TypeError(
				`Invalid Nodesmith event ${name}: expected one payload, received ${data.length}.`
			);
		}
		listener(parsers[name](data[0], name));
	});
	let active = true;
	return () => {
		if (!active) return;
		active = false;
		unsubscribe();
	};
}

export function offNodesmithEvent(
	name: NodesmithEventName,
	...additionalNames: NodesmithEventName[]
): void {
	ensureEventRuntime(`Unsubscribe from ${name}`);
	EventsOff(name, ...additionalNames);
}

export const eventApi = {
	on: onNodesmithEvent,
	off: offNodesmithEvent,
	onJobStarted: (listener: (payload: NodesmithEventMap['nodesmith:job:started']) => void) =>
		onNodesmithEvent('nodesmith:job:started', listener),
	onJobStep: (listener: (payload: NodesmithEventMap['nodesmith:job:step']) => void) =>
		onNodesmithEvent('nodesmith:job:step', listener),
	onJobLog: (listener: (payload: NodesmithEventMap['nodesmith:job:log']) => void) =>
		onNodesmithEvent('nodesmith:job:log', listener),
	onJobDone: (listener: (payload: NodesmithEventMap['nodesmith:job:done']) => void) =>
		onNodesmithEvent('nodesmith:job:done', listener),
	onToolchainChanged: (
		listener: (payload: NodesmithEventMap['nodesmith:toolchain:changed']) => void
	) => onNodesmithEvent('nodesmith:toolchain:changed', listener),
	onRecipesReloaded: (
		listener: (payload: NodesmithEventMap['nodesmith:recipes:reloaded']) => void
	) => onNodesmithEvent('nodesmith:recipes:reloaded', listener)
} as const;
