import {createContext, ReactNode, useContext, useEffect, useMemo, useState} from 'react';
import en, {groupIconLabelsEn} from './locales/en';
import zhCN from './locales/zh-CN';

export type Locale = 'zh-CN' | 'en';
export type MessageKey = keyof typeof zhCN;
type MessageValues = Record<string, string | number>;

const storageKey = 'keynest.locale';
const catalogs = {'zh-CN': zhCN, en};

const detectLocale = (): Locale => {
    const stored = localStorage.getItem(storageKey);
    if (stored === 'zh-CN' || stored === 'en') return stored;
    return navigator.language.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en';
};

const interpolate = (message: string, values: MessageValues = {}) => message.replace(/\{(\w+)}/g, (match, key) => String(values[key] ?? match));

type I18nContextValue = {
    locale: Locale;
    setLocale: (locale: Locale) => void;
    t: (key: MessageKey, values?: MessageValues) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({children}: {children: ReactNode}) {
    const [locale, setLocale] = useState<Locale>(detectLocale);

    useEffect(() => {
        localStorage.setItem(storageKey, locale);
        document.documentElement.lang = locale;
    }, [locale]);

    const value = useMemo<I18nContextValue>(() => ({
        locale,
        setLocale,
        t: (key, values) => interpolate(catalogs[locale][key], values),
    }), [locale]);

    return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
    const value = useContext(I18nContext);
    if (!value) throw new Error('useI18n must be used within I18nProvider');
    return value;
}

export const localizedGroupIconLabel = (id: number, label: string, locale: Locale) => locale === 'en' ? groupIconLabelsEn[id] ?? label : label;
