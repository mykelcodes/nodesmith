import { serviceApi } from './client';
import { eventApi } from './events';

export const api = { ...serviceApi, events: eventApi } as const;

export { copyText, openExternalUrl } from './clipboard';
export { NodesmithBridgeError, toErrorMessage } from './client';
export { eventApi, offNodesmithEvent, onNodesmithEvent } from './events';
export type { Unsubscribe } from './events';
export type * from './types';
