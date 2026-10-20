import {useEffect, useMemo, useState} from 'react';
import {Check, Copy, RefreshCw, ShieldCheck, X, Zap} from 'lucide-react';
import {useI18n, type MessageKey} from './i18n';

type Mode = 'random' | 'readable' | 'pin';
type CharacterOptions = {upper: boolean; lower: boolean; numbers: boolean; symbols: boolean};
export type PasswordStrength = {entropy: number; level: number};

type PasswordGeneratorProps = {
    onUse: (password: string) => void;
    onClose: () => void;
};

const characterSets = {
    upper: 'ABCDEFGHJKLMNPQRSTUVWXYZ',
    lower: 'abcdefghijkmnopqrstuvwxyz',
    numbers: '23456789',
    symbols: '!@#$%^&*+-_=?.',
};

export function passwordStrengthFromEntropy(entropy: number): PasswordStrength {
    if (entropy >= 100) return {entropy, level: 4};
    if (entropy >= 70) return {entropy, level: 3};
    if (entropy >= 45) return {entropy, level: 2};
    return {entropy, level: entropy > 0 ? 1 : 0};
}

export const passwordStrengthMessageKey = (level: number): MessageKey => (
    level >= 4 ? 'password.veryStrong'
        : level === 3 ? 'password.strong'
            : level === 2 ? 'password.medium'
                : level === 1 ? 'password.weak' : 'password.empty'
);

export function calculatePasswordStrength(password: string): PasswordStrength {
    let poolSize = 0;
    if (/[A-Z]/.test(password)) poolSize += characterSets.upper.length;
    if (/[a-z]/.test(password)) poolSize += characterSets.lower.length;
    if (/\d/.test(password)) poolSize += 10;
    if (/[^A-Za-z0-9]/.test(password)) poolSize += characterSets.symbols.length;
    const length = Array.from(password).length;
    const entropy = poolSize > 0 ? Math.round(length * Math.log2(poolSize)) : 0;
    return passwordStrengthFromEntropy(entropy);
}

function secureIndex(length: number) {
    if (length < 1) return 0;
    const limit = 256 - (256 % length);
    const random = new Uint8Array(1);
    do crypto.getRandomValues(random); while (random[0] >= limit);
    return random[0] % length;
}

function securePick(pool: string) {
    return pool[secureIndex(pool.length)];
}

function secureShuffle(values: string[]) {
    for (let index = values.length - 1; index > 0; index -= 1) {
        const swap = secureIndex(index + 1);
        [values[index], values[swap]] = [values[swap], values[index]];
    }
    return values;
}

export function generatePassword(length: number, mode: Mode, options: CharacterOptions) {
    if (mode === 'pin') {
        return Array.from({length}, () => securePick('0123456789')).join('');
    }

    const enabled = Object.entries(options)
        .filter(([, active]) => active)
        .map(([name]) => characterSets[name as keyof CharacterOptions]);
    const pools = enabled.length > 0 ? enabled : [characterSets.lower];
    const pool = pools.join('');
    const result = pools.map(securePick);
    while (result.length < length) result.push(securePick(pool));
    return secureShuffle(result).join('');
}

export function generateDefaultPassword() {
    return generatePassword(16, 'random', {upper: true, lower: true, numbers: true, symbols: true});
}

function PasswordGenerator({onUse, onClose}: PasswordGeneratorProps) {
    const {t} = useI18n();
    const [mode, setMode] = useState<Mode>('random');
    const [length, setLength] = useState(16);
    const [options, setOptions] = useState<CharacterOptions>({upper: true, lower: true, numbers: true, symbols: true});
    const [revision, setRevision] = useState(0);
    const [copied, setCopied] = useState(false);

    const password = useMemo(() => generatePassword(length, mode, options), [length, mode, options, revision]);
    const poolSize = mode === 'pin' ? 10 : Object.entries(options).reduce((total, [name, active]) => total + (active ? characterSets[name as keyof CharacterOptions].length : 0), 0);
    const entropy = Math.round(length * Math.log2(Math.max(poolSize, 1)));
    const strength = passwordStrengthFromEntropy(entropy);

    useEffect(() => {
        setCopied(false);
    }, [password]);

    const changeMode = (next: Mode) => {
        setMode(next);
        setLength(next === 'pin' ? 6 : 16);
        if (next === 'readable') setOptions({upper: true, lower: true, numbers: true, symbols: false});
    };

    const toggleOption = (key: keyof CharacterOptions) => {
        setOptions((current) => {
            if (current[key] && Object.values(current).filter(Boolean).length === 1) return current;
            return {...current, [key]: !current[key]};
        });
    };

    const copyPassword = async () => {
        await navigator.clipboard.writeText(password);
        setCopied(true);
    };

    return (
        <section className="password-generator" aria-label={t('password.generator')} onClick={(event) => event.stopPropagation()}>
            <header>
                <div><span className="generator-icon"><Zap size={15}/></span><strong>{t('password.generate')}</strong></div>
                <button type="button" onClick={onClose} title={t('password.closeGenerator')}><X size={16}/></button>
            </header>

            <div className="generator-output">
                <code>{password}</code>
                <button type="button" onClick={() => setRevision((value) => value + 1)} title={t('password.regenerate')}><RefreshCw size={16}/></button>
                <button type="button" onClick={copyPassword} title={t('password.copyGenerated')}>{copied ? <Check size={16}/> : <Copy size={16}/>}</button>
            </div>

            <div className="generator-segments" aria-label={t('password.type')}>
                <button type="button" className={mode === 'random' ? 'active' : ''} onClick={() => changeMode('random')}>{t('password.random')}</button>
                <button type="button" className={mode === 'readable' ? 'active' : ''} onClick={() => changeMode('readable')}>{t('password.readable')}</button>
                <button type="button" className={mode === 'pin' ? 'active' : ''} onClick={() => changeMode('pin')}>{t('password.pin')}</button>
            </div>

            <div className="length-control">
                <label htmlFor="password-length">{t('password.length')} <strong>{length}</strong></label>
                <input id="password-length" type="range" min={mode === 'pin' ? 4 : 8} max={mode === 'pin' ? 12 : 64} value={length} onChange={(event) => setLength(Number(event.target.value))}/>
            </div>

            {mode !== 'pin' && (
                <div className="character-options">
                    <label><input type="checkbox" checked={options.upper} onChange={() => toggleOption('upper')}/><span>ABC</span></label>
                    <label><input type="checkbox" checked={options.lower} onChange={() => toggleOption('lower')}/><span>abc</span></label>
                    <label><input type="checkbox" checked={options.numbers} onChange={() => toggleOption('numbers')}/><span>123</span></label>
                    <label><input type="checkbox" checked={options.symbols} onChange={() => toggleOption('symbols')}/><span>!@#</span></label>
                </div>
            )}

            <div className="generator-strength"><ShieldCheck size={14}/><span>{t(passwordStrengthMessageKey(strength.level))}</span><em>{t('password.bits', {count: entropy})}</em></div>
            <button type="button" className="use-password-button" onClick={() => onUse(password)}><Check size={15}/> {t('password.use')}</button>
        </section>
    );
}

export default PasswordGenerator;
