import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { api } from '../api.js';
import { formatTimestamp } from '../time.js';
import { displayName } from '../users.js';

function isRtlText(text) {
  return /[\u0591-\u07FF\uFB1D-\uFDFD\uFE70-\uFEFC]/.test(text || '');
}

function hasPersianScript(text) {
  return /[\u0600-\u06FF\u0750-\u077F\u08A0-\u08FF\uFB50-\uFDFF\uFE70-\uFEFF]/.test(text || '');
}

function openRow(path) {
  window.location.hash = path;
}

export default function Inbox() {
  const [messages, setMessages] = useState(null);
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [err, setErr] = useState(null);

  const load = () => api.getMessages(unreadOnly).then(data => { setMessages(data); setErr(null); }).catch(e => setErr(e.message));
  useEffect(() => { load(); }, [unreadOnly]);

  async function markRead(id) {
    try { await api.markRead(id); load(); } catch (e) { setErr(e.message); }
  }

  async function markAllRead() {
    try { await api.markAllRead(); load(); } catch (e) { setErr(e.message); }
  }

  if (err) return h('p', null, 'Error: ', err);
  if (!messages) return h('p', null, 'Loading...');

  const unreadCount = messages.filter(m => !m.isRead).length;

  return h('div', null,
    h('h1', null, 'Inbox'),
    h('div', { style: { display: 'flex', gap: '8px', marginBottom: '16px', alignItems: 'center' } },
      h('label', null,
        h('input', {
          type: 'checkbox',
          checked: unreadOnly,
          onChange: (e) => setUnreadOnly(e.target.checked),
        }),
        ' Unread only',
      ),
      unreadCount > 0 && h('button', { onClick: markAllRead }, `Mark all read (${unreadCount})`),
    ),
    messages.length === 0
      ? h('div', { class: 'empty' }, unreadOnly ? 'No unread messages.' : 'No messages yet.')
      : h('div', { class: 'table-shell' },
          h('table', { class: 'inbox-table' },
          h('colgroup', null,
            h('col', { class: 'col-time' }),
            h('col', { class: 'col-from' }),
            h('col', { class: 'col-preview' }),
            h('col', { class: 'col-actions' }),
          ),
          h('thead', null, h('tr', null,
            h('th', null, 'Time'),
            h('th', null, 'From'),
            h('th', null, 'Message Preview'),
            h('th', null, ''),
          )),
          h('tbody', null, messages.map(m =>
            (() => {
              const previewIsRtl = isRtlText(m.body);
              const previewHasPersianScript = hasPersianScript(m.body);
              const previewClass = `message-preview-line${previewHasPersianScript ? ' message-preview-line-persian' : ''}`;

              return h('tr', {
                key: m.id,
                class: `clickable-row ${m.isRead ? '' : 'msg-unread'}`,
                onClick: () => openRow(`/inbox/${m.id}`),
              },
                h('td', { class: 'time inbox-time-cell' }, formatTimestamp(m.createdAt)),
                h('td', { class: 'inbox-from-cell' }, displayName(m)),
                h('td', { class: 'inbox-preview-cell' },
                  h('div', {
                    class: previewClass,
                    dir: 'auto',
                    lang: previewHasPersianScript ? 'fa' : previewIsRtl ? 'ar' : 'en',
                  }, m.body),
                ),
                h('td', { class: 'inbox-actions-cell' },
                  !m.isRead && h('button', {
                    onClick: (e) => {
                      e.stopPropagation();
                      markRead(m.id);
                    },
                  }, 'Mark read'),
                ),
              );
            })(),
          )),
        )),
  );
}
