export type EntryField = {
    key: string;
    value: string;
    protected: boolean;
};

export type TemplateFieldType = 'text' | 'password' | 'url' | 'date' | 'month' | 'textarea';

export type TemplateField = {
    id: string;
    storageKey: string;
    label: string;
    type: TemplateFieldType;
    section: string;
    required: boolean;
    protected: boolean;
    searchable: boolean;
};

export type EntryTemplate = {
    id: string;
    version: number;
    name: string;
    builtIn: boolean;
    enabled: boolean;
    deleted: boolean;
    fields: TemplateField[];
};

export const builtInTemplates: EntryTemplate[] = [
    {
        id: 'login', version: 1, name: 'Login', builtIn: true, enabled: true, deleted: false,
        fields: [
            {id: 'username', storageKey: 'UserName', label: 'Username', type: 'text', section: 'login', required: false, protected: false, searchable: true},
            {id: 'password', storageKey: 'Password', label: 'Password', type: 'password', section: 'login', required: false, protected: true, searchable: false},
            {id: 'url', storageKey: 'URL', label: 'Website', type: 'url', section: 'website', required: false, protected: false, searchable: true},
            {id: 'notes', storageKey: 'Notes', label: 'Notes', type: 'textarea', section: 'notes', required: false, protected: false, searchable: true},
        ],
    },
    {
        id: 'bank-card', version: 1, name: 'Bank Card', builtIn: true, enabled: true, deleted: false,
        fields: [
            {id: 'cardholder', storageKey: 'UserName', label: 'Cardholder', type: 'text', section: 'card', required: false, protected: false, searchable: true},
            {id: 'cardNumber', storageKey: 'Card Number', label: 'Card Number', type: 'password', section: 'card', required: true, protected: true, searchable: false},
            {id: 'expiryDate', storageKey: 'Expiry Date', label: 'Expiry Date', type: 'month', section: 'card', required: false, protected: false, searchable: false},
            {id: 'cvv', storageKey: 'CVV', label: 'CVV', type: 'password', section: 'security', required: false, protected: true, searchable: false},
            {id: 'pin', storageKey: 'Password', label: 'PIN', type: 'password', section: 'security', required: false, protected: true, searchable: false},
            {id: 'customerService', storageKey: 'Customer Service', label: 'Customer Service', type: 'text', section: 'contact', required: false, protected: false, searchable: true},
            {id: 'notes', storageKey: 'Notes', label: 'Notes', type: 'textarea', section: 'notes', required: false, protected: false, searchable: true},
        ],
    },
    {
        id: 'wifi', version: 1, name: 'Wi-Fi', builtIn: true, enabled: true, deleted: false,
        fields: [
            {id: 'ssid', storageKey: 'UserName', label: 'Network Name', type: 'text', section: 'network', required: true, protected: false, searchable: true},
            {id: 'password', storageKey: 'Password', label: 'Password', type: 'password', section: 'network', required: false, protected: true, searchable: false},
            {id: 'securityType', storageKey: 'Security Type', label: 'Security Type', type: 'text', section: 'network', required: false, protected: false, searchable: true},
            {id: 'routerAddress', storageKey: 'URL', label: 'Router Address', type: 'url', section: 'network', required: false, protected: false, searchable: true},
            {id: 'notes', storageKey: 'Notes', label: 'Notes', type: 'textarea', section: 'notes', required: false, protected: false, searchable: true},
        ],
    },
    {
        id: 'secure-note', version: 1, name: 'Secure Note', builtIn: true, enabled: true, deleted: false,
        fields: [
            {id: 'notes', storageKey: 'Notes', label: 'Notes', type: 'textarea', section: 'notes', required: true, protected: true, searchable: false},
        ],
    },
    {
        id: 'identity', version: 1, name: 'Identity', builtIn: true, enabled: true, deleted: false,
        fields: [
            {id: 'fullName', storageKey: 'UserName', label: 'Full Name', type: 'text', section: 'identity', required: true, protected: false, searchable: true},
            {id: 'documentNumber', storageKey: 'Document Number', label: 'Document Number', type: 'password', section: 'identity', required: false, protected: true, searchable: false},
            {id: 'birthDate', storageKey: 'Birth Date', label: 'Birth Date', type: 'date', section: 'identity', required: false, protected: false, searchable: false},
            {id: 'issuedDate', storageKey: 'Issued Date', label: 'Issued Date', type: 'date', section: 'validity', required: false, protected: false, searchable: false},
            {id: 'expiryDate', storageKey: 'Expiry Date', label: 'Expiry Date', type: 'date', section: 'validity', required: false, protected: false, searchable: false},
            {id: 'notes', storageKey: 'Notes', label: 'Notes', type: 'textarea', section: 'notes', required: false, protected: false, searchable: true},
        ],
    },
];

export const fieldsFromLegacyEntry = (entry: {username: string; password: string; url: string; note: string; fields?: EntryField[]}) => {
    if (entry.fields?.length) return entry.fields;
    return [
        {key: 'UserName', value: entry.username ?? '', protected: false},
        {key: 'Password', value: entry.password ?? '', protected: true},
        {key: 'URL', value: entry.url ?? '', protected: false},
        {key: 'Notes', value: entry.note ?? '', protected: false},
    ];
};

export const fieldValue = (fields: EntryField[], key: string) => fields.find((field) => field.key === key)?.value ?? '';

export const templateFor = (templates: EntryTemplate[], id: string | undefined) =>
    templates.find((template) => template.id === id) ?? templates.find((template) => template.id === 'login') ?? builtInTemplates[0];

export const categoryForTemplate = (id: string) => {
    switch (id) {
        case 'bank-card': return '银行卡';
        case 'wifi': return 'Wi-Fi';
        case 'secure-note': return '安全笔记';
        case 'identity': return '身份';
        default: return '登录信息';
    }
};

export const templateIDForCategory = (category: string) => {
    switch (category.trim().toLowerCase()) {
        case '银行卡': case 'bank card': return 'bank-card';
        case 'wi-fi': case 'wifi': return 'wifi';
        case '安全笔记': case 'secure note': return 'secure-note';
        case '身份': case 'identity': return 'identity';
        default: return 'login';
    }
};
