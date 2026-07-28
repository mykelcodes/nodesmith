import { BrowserOpenURL, ClipboardSetText } from '../wailsjs/runtime/runtime.js';

type RuntimeWindow = Window & {
	runtime?: {
		ClipboardSetText?: (text: string) => Promise<boolean>;
		BrowserOpenURL?: (url: string) => void;
	};
};

export async function copyText(text: string): Promise<void> {
	if (typeof window === 'undefined') {
		throw new Error('Copy text: the desktop clipboard is unavailable outside the application.');
	}

	if (typeof (window as RuntimeWindow).runtime?.ClipboardSetText === 'function') {
		const copied = await ClipboardSetText(text);
		if (!copied) throw new Error('Copy text: the desktop clipboard rejected the content.');
		return;
	}

	if (navigator.clipboard?.writeText) {
		await navigator.clipboard.writeText(text);
		return;
	}

	throw new Error(
		'Copy text: no clipboard provider is available — run this UI through the Wails v2 desktop application.'
	);
}

export function openExternalUrl(rawUrl: string): void {
	let url: URL;
	try {
		url = new URL(rawUrl);
	} catch {
		throw new Error('Open documentation: the recipe provided an invalid URL.');
	}
	if (url.protocol !== 'https:' && url.protocol !== 'http:') {
		throw new Error('Open documentation: only HTTP and HTTPS links are supported.');
	}
	if (typeof window === 'undefined') {
		throw new Error('Open documentation: the desktop browser runtime is unavailable.');
	}

	if (typeof (window as RuntimeWindow).runtime?.BrowserOpenURL === 'function') {
		BrowserOpenURL(url.href);
		return;
	}
	const opened = window.open(url.href, '_blank', 'noopener,noreferrer');
	if (!opened) {
		throw new Error(
			'Open documentation: the link was blocked — run this UI through the Wails v2 desktop application.'
		);
	}
}
