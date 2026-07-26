/**
 * App-owned dialogs and listbox controls.
 * Keeps browser prompts, confirms, and native selects out of the product UI.
 */

let activeDialog = null;

function getDialog() {
    return document.getElementById('appDialog');
}

function focusable(panel) {
    return Array.from(panel.querySelectorAll('button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])'));
}

function closeActive(value) {
    if (!activeDialog) return;
    const current = activeDialog;
    activeDialog = null;
    current.dialog.classList.remove('is-open');
    document.body.classList.remove('has-wc-dialog');
    window.setTimeout(function () {
        current.dialog.hidden = true;
        if (current.returnFocus && document.contains(current.returnFocus)) current.returnFocus.focus();
        current.resolve(value);
    }, 180);
}

function fieldControl(field) {
    const wrap = document.createElement('label');
    wrap.className = 'wc-dialog__field';
    wrap.textContent = field.label;

    if (field.type === 'choice') {
        wrap.classList.add('wc-dialog__field--choice');
        const input = document.createElement('input');
        input.type = 'hidden';
        input.name = field.name;
        input.value = field.value || '';
        if (field.required) input.dataset.required = 'true';
        wrap.appendChild(input);

        const choices = document.createElement('div');
        choices.className = 'wc-dialog__choices';
        choices.setAttribute('role', 'radiogroup');
        choices.setAttribute('aria-label', field.label);
        (field.options || []).forEach(function (option) {
            const choice = document.createElement('button');
            choice.type = 'button';
            choice.className = 'wc-dialog__choice';
            choice.dataset.value = option.value;
            choice.setAttribute('role', 'radio');
            choice.textContent = option.label;
            choice.addEventListener('click', function () {
                input.value = option.value;
                choices.querySelectorAll('[role="radio"]').forEach(function (item) {
                    const selected = item === choice;
                    item.classList.toggle('is-selected', selected);
                    item.setAttribute('aria-checked', selected ? 'true' : 'false');
                });
            });
            const selected = option.value === input.value;
            choice.classList.toggle('is-selected', selected);
            choice.setAttribute('aria-checked', selected ? 'true' : 'false');
            choices.appendChild(choice);
        });
        wrap.appendChild(choices);
        return wrap;
    }

    const input = document.createElement('input');
    input.name = field.name;
    input.type = field.type || 'text';
    input.value = field.value || '';
    input.placeholder = field.placeholder || '';
    input.autocomplete = 'off';
    input.required = !!field.required;
    if (field.maxLength) input.maxLength = field.maxLength;
    wrap.appendChild(input);
    return wrap;
}

export function openAppDialog(options) {
    const dialog = getDialog();
    if (!dialog) return Promise.resolve(null);
    if (activeDialog) closeActive(null);

    const opts = options || {};
    const panel = dialog.querySelector('[data-app-dialog-panel]');
    const title = dialog.querySelector('[data-app-dialog-title]');
    const eyebrow = dialog.querySelector('[data-app-dialog-eyebrow]');
    const icon = dialog.querySelector('[data-app-dialog-icon]');
    const message = dialog.querySelector('[data-app-dialog-message]');
    const form = dialog.querySelector('[data-app-dialog-form]');
    const fields = dialog.querySelector('[data-app-dialog-fields]');
    const cancel = dialog.querySelector('[data-app-dialog-cancel]');
    const submit = dialog.querySelector('[data-app-dialog-submit]');
    const error = dialog.querySelector('[data-app-dialog-error]');
    const returnFocus = document.activeElement;

    title.textContent = opts.title || 'Confirm action';
    eyebrow.textContent = opts.eyebrow || 'Workspace';
    icon.className = 'bi ' + (opts.icon || 'bi-pencil-square');
    message.textContent = opts.message || '';
    message.hidden = !opts.message;
    cancel.textContent = opts.cancelLabel || 'Cancel';
    submit.textContent = opts.confirmLabel || 'Save';
    submit.classList.toggle('wc-dialog__button--danger', opts.tone === 'danger');
    submit.classList.toggle('wc-dialog__button--primary', opts.tone !== 'danger');
    error.hidden = true;
    error.textContent = '';
    fields.textContent = '';
    (opts.fields || []).forEach(function (field) {
        fields.appendChild(fieldControl(field));
    });

    return new Promise(function (resolve) {
        activeDialog = { dialog: dialog, resolve: resolve, returnFocus: returnFocus };
        dialog.hidden = false;
        document.body.classList.add('has-wc-dialog');
        requestAnimationFrame(function () {
            dialog.classList.add('is-open');
            const first = fields.querySelector('input:not([type="hidden"]), button') || submit;
            first.focus();
        });

        form.onsubmit = function (event) {
            event.preventDefault();
            const missingChoice = Array.from(fields.querySelectorAll('input[data-required="true"]')).find(function (input) {
                return !input.value;
            });
            if (missingChoice) {
                error.textContent = 'Choose a value to continue.';
                error.hidden = false;
                return;
            }
            if (!form.reportValidity()) return;
            const values = {};
            new FormData(form).forEach(function (value, key) { values[key] = String(value).trim(); });
            closeActive(values);
        };
        cancel.onclick = function () { closeActive(null); };
        dialog.querySelector('[data-app-dialog-close]').onclick = function () { closeActive(null); };
        dialog.querySelector('[data-app-dialog-backdrop]').onclick = function () { closeActive(null); };
        panel.onkeydown = function (event) {
            if (event.key === 'Escape') {
                event.preventDefault();
                closeActive(null);
                return;
            }
            if (event.key !== 'Tab') return;
            const items = focusable(panel);
            if (!items.length) return;
            const first = items[0];
            const last = items[items.length - 1];
            if (event.shiftKey && document.activeElement === first) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault();
                first.focus();
            }
        };
    });
}

export function confirmAppDialog(options) {
    return openAppDialog({
        ...(options || {}),
        confirmLabel: (options && options.confirmLabel) || 'Continue',
        fields: [],
    }).then(function (result) { return result !== null; });
}

export function setAppSelectValue(control, value) {
    if (!control) return;
    const input = control.querySelector('input[type="hidden"]');
    const trigger = control.querySelector('[data-ui-select-trigger]');
    const option = Array.from(control.querySelectorAll('[data-ui-select-option]')).find(function (item) {
        return item.dataset.value === value;
    });
    if (!input || !option || !trigger) return;
    input.value = value;
    trigger.querySelector('[data-ui-select-label]').textContent = option.textContent;
    control.querySelectorAll('[data-ui-select-option]').forEach(function (item) {
        const selected = item === option;
        item.setAttribute('aria-selected', selected ? 'true' : 'false');
        item.classList.toggle('is-selected', selected);
    });
}

export function bootAppSelects(root) {
    const scope = root || document;
    scope.querySelectorAll('[data-ui-select]').forEach(function (control) {
        if (control.dataset.ready) return;
        control.dataset.ready = 'true';
        const trigger = control.querySelector('[data-ui-select-trigger]');
        const list = control.querySelector('[data-ui-select-list]');
        if (!trigger || !list) return;
        const close = function () {
            list.hidden = true;
            trigger.setAttribute('aria-expanded', 'false');
        };
        trigger.addEventListener('click', function () {
            const open = list.hidden;
            list.hidden = !open;
            trigger.setAttribute('aria-expanded', open ? 'true' : 'false');
            if (open) {
                const selected = list.querySelector('[aria-selected="true"]') || list.querySelector('[data-ui-select-option]');
                if (selected) selected.focus();
            }
        });
        list.addEventListener('click', function (event) {
            const option = event.target.closest('[data-ui-select-option]');
            if (!option) return;
            setAppSelectValue(control, option.dataset.value);
            close();
            trigger.focus();
        });
        control.addEventListener('keydown', function (event) {
            const items = Array.from(list.querySelectorAll('[data-ui-select-option]'));
            if (!items.length) return;
            if (event.key === 'Escape') { close(); trigger.focus(); return; }
            if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
            event.preventDefault();
            if (list.hidden) {
                list.hidden = false;
                trigger.setAttribute('aria-expanded', 'true');
            }
            const current = items.indexOf(document.activeElement);
            const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1
                : (current + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length;
            items[next].focus();
        });
        document.addEventListener('pointerdown', function (event) {
            if (!control.contains(event.target)) close();
        });
    });
}
