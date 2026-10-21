import {DragEvent, FormEvent, useState} from 'react';
import {Copy, FileKey2, GripVertical, Plus, Save, Trash2, X} from 'lucide-react';
import type {MessageKey} from './i18n';
import type {EntryTemplate, TemplateField, TemplateFieldType} from './entryTemplates';

type Translate = (key: MessageKey, values?: Record<string, string | number>) => string;

type Props = {
    templates: EntryTemplate[];
    t: Translate;
    onSave: (template: EntryTemplate) => Promise<void>;
    onDelete: (template: EntryTemplate) => void;
    onReorder: (templates: EntryTemplate[]) => Promise<void>;
    onToggleEnabled: (template: EntryTemplate, enabled: boolean) => Promise<void>;
};

const fieldTypes: TemplateFieldType[] = ['text', 'password', 'url', 'date', 'month', 'textarea'];

const newField = (index: number): TemplateField => ({
    id: `field${index}`,
    storageKey: `Custom Field ${index}`,
    label: `Field ${index}`,
    type: 'text',
    section: 'details',
    required: false,
    protected: false,
    searchable: true,
});

const nextField = (fields: TemplateField[]) => {
    let index = fields.length + 1;
    while (fields.some((field) => field.id === `field${index}` || field.storageKey === `Custom Field ${index}`)) index++;
    return newField(index);
};

const editableCopy = (template?: EntryTemplate): EntryTemplate => template ? {
    ...template,
    id: template.builtIn ? '' : template.id,
    name: template.builtIn ? `${template.name} Copy` : template.name,
    builtIn: false,
    enabled: template.builtIn ? true : template.enabled,
    deleted: false,
    fields: template.fields.map((field) => ({...field})),
} : {id: '', version: 1, name: '', builtIn: false, enabled: true, deleted: false, fields: [newField(1)]};

export default function TemplateManager({templates, t, onSave, onDelete, onReorder, onToggleEnabled}: Props) {
    const [editing, setEditing] = useState<EntryTemplate | null>(null);
    const [pending, setPending] = useState(false);
    const [draggedID, setDraggedID] = useState<string | null>(null);
    const [dropTargetID, setDropTargetID] = useState<string | null>(null);

    const updateField = (index: number, patch: Partial<TemplateField>) => {
        setEditing((current) => current ? {...current, fields: current.fields.map((field, fieldIndex) => fieldIndex === index ? {...field, ...patch} : field)} : current);
    };

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (!editing) return;
        setPending(true);
        try {
            await onSave({
                ...editing,
                fields: editing.fields.map((field) => ({
                    ...field,
                    protected: field.protected || field.type === 'password' || field.storageKey === 'Password',
                })),
            });
            setEditing(null);
        } catch {
            // The parent displays the backend validation error as a toast.
        } finally {
            setPending(false);
        }
    };

    const persistOrder = async (next: EntryTemplate[]) => {
        if (pending) return;
        setPending(true);
        try {
            await onReorder(next);
        } catch {
            // The parent displays persistence errors as a toast.
        } finally {
            setPending(false);
        }
    };

    const reorder = async (targetID: string) => {
        if (!draggedID || draggedID === targetID || pending) return;
        const next = [...templates];
        const sourceIndex = next.findIndex((template) => template.id === draggedID);
        const targetIndex = next.findIndex((template) => template.id === targetID);
        if (sourceIndex < 0 || targetIndex < 0) return;
        const [moved] = next.splice(sourceIndex, 1);
        next.splice(targetIndex, 0, moved);
        setDraggedID(null);
        setDropTargetID(null);
        await persistOrder(next);
    };

    const moveWithKeyboard = async (id: string, direction: -1 | 1) => {
        const sourceIndex = templates.findIndex((template) => template.id === id);
        const targetIndex = sourceIndex + direction;
        if (sourceIndex < 0 || targetIndex < 0 || targetIndex >= templates.length || pending) return;
        const next = [...templates];
        [next[sourceIndex], next[targetIndex]] = [next[targetIndex], next[sourceIndex]];
        await persistOrder(next);
    };

    const startDragging = (event: DragEvent<HTMLSpanElement>, id: string) => {
        event.dataTransfer.effectAllowed = 'move';
        event.dataTransfer.setData('text/plain', id);
        setDraggedID(id);
    };

    const toggleEnabled = async (template: EntryTemplate, enabled: boolean) => {
        if (pending) return;
        setPending(true);
        try {
            await onToggleEnabled(template, enabled);
        } catch {
            // The parent displays persistence errors as a toast.
        } finally {
            setPending(false);
        }
    };

    if (editing) {
        return <form className="template-editor" onSubmit={submit}>
            <div className="template-editor-heading">
                <div><h2>{t(editing.id ? 'template.edit' : 'template.create')}</h2><p>{t('template.editorDescription')}</p></div>
                <button type="button" className="icon-button" onClick={() => setEditing(null)} title={t('common.close')}><X size={17}/></button>
            </div>
            <label className="template-name-field">{t('template.name')}<input value={editing.name} onChange={(event) => setEditing({...editing, name: event.target.value})} required autoFocus/></label>
            <div className="template-field-heading"><strong>{t('template.fields')}</strong><button type="button" className="secondary-button" onClick={() => setEditing({...editing, fields: [...editing.fields, nextField(editing.fields)]})}><Plus size={14}/>{t('template.addField')}</button></div>
            <div className="template-field-list">
                {editing.fields.map((field, index) => <div className="template-field-editor" key={`${field.id}-${index}`}>
                    <input aria-label={t('template.fieldLabel')} value={field.label} onChange={(event) => updateField(index, {label: event.target.value})} required/>
                    <input aria-label={t('template.storageKey')} value={field.storageKey} onChange={(event) => updateField(index, {storageKey: event.target.value})} required/>
                    <select aria-label={t('template.fieldType')} value={field.type} onChange={(event) => updateField(index, {type: event.target.value as TemplateFieldType, protected: event.target.value === 'password' ? true : field.protected})}>
                        {fieldTypes.map((type) => <option key={type} value={type}>{t(`template.type.${type}` as MessageKey)}</option>)}
                    </select>
                    <label><input type="checkbox" checked={field.required} onChange={(event) => updateField(index, {required: event.target.checked})}/>{t('template.required')}</label>
                    <label><input type="checkbox" checked={field.protected || field.type === 'password' || field.storageKey === 'Password'} disabled={field.type === 'password' || field.storageKey === 'Password'} onChange={(event) => updateField(index, {protected: event.target.checked})}/>{t('template.protected')}</label>
                    <button type="button" className="icon-button destructive-icon" onClick={() => setEditing({...editing, fields: editing.fields.filter((_, fieldIndex) => fieldIndex !== index)})} disabled={editing.fields.length === 1} title={t('common.delete')}><Trash2 size={15}/></button>
                </div>)}
            </div>
            <div className="template-editor-actions"><button type="button" className="secondary-button" onClick={() => setEditing(null)}>{t('common.cancel')}</button><button type="submit" className="primary-button" disabled={pending}><Save size={15}/>{t(pending ? 'common.saving' : 'common.saveChanges')}</button></div>
        </form>;
    }

    return <section className="settings-section template-settings">
        <div className="template-settings-heading"><div><h2>{t('template.manage')}</h2><p>{t('template.manageDescription', {count: templates.length})}</p></div><button type="button" className="primary-button" onClick={() => setEditing(editableCopy())}><Plus size={15}/>{t('template.create')}</button></div>
        {templates.length > 0 ? <div className="template-list">
            {templates.map((template) => <div
                className={`template-row ${draggedID === template.id ? 'dragging' : ''} ${dropTargetID === template.id ? 'drop-target' : ''}`}
                key={template.id}
                onDragOver={(event) => { event.preventDefault(); event.dataTransfer.dropEffect = 'move'; setDropTargetID(template.id); }}
                onDragLeave={() => setDropTargetID((current) => current === template.id ? null : current)}
                onDrop={(event) => { event.preventDefault(); void reorder(template.id); }}
            >
                <span
                    className="template-drag-handle"
                    draggable={!pending}
                    role="button"
                    tabIndex={0}
                    title={t('template.dragToReorder')}
                    aria-label={t('template.dragToReorder')}
                    onDragStart={(event) => startDragging(event, template.id)}
                    onDragEnd={() => { setDraggedID(null); setDropTargetID(null); }}
                    onKeyDown={(event) => {
                        if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;
                        event.preventDefault();
                        void moveWithKeyboard(template.id, event.key === 'ArrowUp' ? -1 : 1);
                    }}
                ><GripVertical size={15}/></span>
                <span className="template-row-icon"><FileKey2 size={17}/></span>
                <span className="template-row-summary"><strong>{template.name}</strong><small>{t('template.fieldCount', {count: template.fields.length})} · {template.builtIn ? t('template.builtIn') : t('template.custom')}</small></span>
                <div className="template-row-actions">
                    <label className="template-enabled-toggle" title={t(template.enabled ? 'template.enabled' : 'template.disabled')} onPointerDown={(event) => event.stopPropagation()}>
                        <input type="checkbox" checked={template.enabled} disabled={pending} onChange={(event) => void toggleEnabled(template, event.target.checked)} aria-label={t('template.enabled')}/>
                        <span aria-hidden="true"/>
                        <small>{t(template.enabled ? 'template.enabled' : 'template.disabled')}</small>
                    </label>
                    <button type="button" className="icon-button" onClick={() => setEditing(editableCopy(template))} title={template.builtIn ? t('template.duplicate') : t('common.edit')}>{template.builtIn ? <Copy size={15}/> : <FileKey2 size={15}/>}</button>
                    <button type="button" className="icon-button destructive-icon" onClick={() => onDelete(template)} disabled={pending} title={t('common.delete')}><Trash2 size={15}/></button>
                </div>
            </div>)}
        </div> : <div className="settings-empty"><FileKey2 size={22}/><span>{t('template.empty')}</span></div>}
    </section>;
}
