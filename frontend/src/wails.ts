import {Browser} from '@wailsio/runtime';

type WailsWindow = Window & {
    _wails?: {
        environment?: unknown;
    };
};

export const hasWailsBridge = () => Boolean((window as WailsWindow)._wails?.environment);

export const openExternalURL = (url: string) => {
    if (hasWailsBridge()) return Browser.OpenURL(url);
    window.open(url, '_blank', 'noopener,noreferrer');
    return Promise.resolve();
};
