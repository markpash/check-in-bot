import { h } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { api } from '../api.js';
import { formatTimestamp } from '../time.js';
import { displayName, fullName } from '../users.js';

function isRtlText(text) {
  return /[\u0591-\u07FF\uFB1D-\uFDFD\uFE70-\uFEFC]/.test(text || '');
}

function hasPersianScript(text) {
  return /[\u0600-\u06FF\u0750-\u077F\u08A0-\u08FF\uFB50-\uFDFF\uFE70-\uFEFF]/.test(text || '');
}

export default function MessageDetail({ messageID }) {
  const [data, setData] = useState(null);
  const [err, setErr] = useState(null);

  const load = () => api.getMessage(messageID)
    .then(res => {
      setData(res);
      setErr(null);
    })
    .catch(e => setErr(e.message));

  useEffect(() => { load(); }, [messageID]);

  async function markRead() {
    try {
      await api.markRead(messageID);
      await load();
    } catch (e) { setErr(e.message); }
  }

  if (err) return h('p', null, 'Error: ', err);
  if (!data) return h('p', null, 'Loading...');

  const { message, user } = data;
  const bodyIsRtl = isRtlText(message.body);
  const bodyHasPersianScript = hasPersianScript(message.body);
  const messageBodyClass = `message-page-body${bodyHasPersianScript ? ' message-page-body-persian' : ''}`;

  return h('div', null,
    h('div', { class: 'page-header' },
      h('div', null,
        h('a', { href: '#/inbox', class: 'back-link' }, '< Back to inbox'),
        h('h1', null, 'Message'),
        h('p', { class: 'time' }, `From ${displayName(message)}`),
      ),
      !message.isRead && h('button', { class: 'primary', onClick: markRead }, 'Mark read'),
    ),

    h('section', { class: 'hero-card section' },
      h('div', { class: 'hero-card-main' },
        h('p', { class: 'eyebrow' }, 'Inbox message'),
        h('h2', null, 'Message contents'),
        h('p', { class: 'hero-copy' }, `From ${displayName(message)} on ${formatTimestamp(message.createdAt)}.`),
      ),
      h('div', { class: 'hero-card-stats' },
        h('div', { class: 'stat-chip' }, h('span', null, 'Status'), h('strong', null, message.isRead ? 'Read' : 'Unread')),
        h('div', { class: 'stat-chip' }, h('span', null, 'Message ID'), h('strong', null, message.id)),
      ),
    ),

    h('section', { class: 'detail-grid' },
      h('div', { class: 'card' },
        h('h2', null, 'Message Details'),
        h('dl', { class: 'meta-grid' },
          h('div', { class: 'meta-row' }, h('dt', null, 'Message ID'), h('dd', null, h('code', null, message.id))),
          h('div', { class: 'meta-row' }, h('dt', null, 'Sent'), h('dd', null, formatTimestamp(message.createdAt))),
          h('div', { class: 'meta-row' }, h('dt', null, 'Status'), h('dd', null, message.isRead ? 'Read' : 'Unread')),
          h('div', { class: 'meta-row' }, h('dt', null, 'User'), h('dd', null, h('a', { href: `#/users/${user.id}`, class: 'list-link' }, fullName(user)))),
        ),
      ),
      h('div', { class: 'card' },
        h('h2', null, 'Sender'),
        h('dl', { class: 'meta-grid' },
          h('div', { class: 'meta-row' }, h('dt', null, 'Telegram ID'), h('dd', null, h('code', null, user.id))),
          h('div', { class: 'meta-row' }, h('dt', null, 'Username'), h('dd', null, user.username ? `@${user.username}` : '—')),
          h('div', { class: 'meta-row' }, h('dt', null, 'Language'), h('dd', null, user.languageCode || '—')),
          h('div', { class: 'meta-row' }, h('dt', null, 'Admin note'), h('dd', null, user.note || '—')),
        ),
      ),
    ),

    h('section', { class: 'card section' },
      h('h2', null, 'Message Body'),
      h('div', {
        class: messageBodyClass,
        dir: 'auto',
        lang: bodyHasPersianScript ? 'fa' : bodyIsRtl ? 'ar' : 'en',
      }, message.body),
    ),

    h('section', { class: 'detail-grid' },
      h('div', { class: 'card' },
        h('h2', null, 'Admin Context'),
        h('p', { class: 'time', style: { marginBottom: '10px' } }, 'Internal note for the sender'),
        h('div', { class: 'message-page-body compact-message-body' }, user.note || 'No admin note saved for this user.'),
      ),
      h('div', { class: 'card' },
        h('h2', null, 'Quick Links'),
        h('div', { class: 'button-row' },
          h('a', { href: `#/users/${user.id}`, class: 'button-link' }, 'Open user profile'),
          h('a', { href: '#/inbox', class: 'button-link subtle-link' }, 'Back to inbox'),
        ),
      ),
    ),
  );
}
