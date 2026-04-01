import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { api } from '../api.js';
import { formatTimestamp } from '../time.js';
import { displayName } from '../users.js';

export default function Silences() {
  const [silences, setSilences] = useState(null);
  const [err, setErr] = useState(null);

  const load = () => api.getSilences().then(data => { setSilences(data); setErr(null); }).catch(e => setErr(e.message));
  useEffect(() => { load(); }, []);

  async function cancel(id) {
    try { await api.deleteSilence(id); load(); } catch (e) { setErr(e.message); }
  }

  if (err) return h('p', null, 'Error: ', err);
  if (!silences) return h('p', null, 'Loading...');

  if (silences.length === 0) {
    return h('div', null,
      h('h1', null, 'Active Silences'),
      h('div', { class: 'empty' }, 'No active silences.'),
    );
  }

  return h('div', null,
    h('h1', null, 'Active Silences'),
    h('table', null,
      h('thead', null, h('tr', null,
        h('th', null, 'User'),
        h('th', null, 'Days'),
        h('th', null, 'Reason'),
        h('th', null, 'Ends'),
        h('th', null, ''),
      )),
      h('tbody', null, silences.map(s =>
        h('tr', { key: s.id },
          h('td', null, h('a', { href: `#/users/${s.userId}`, class: 'list-link' }, displayName(s))),
          h('td', null, s.days),
          h('td', null, s.reason || h('span', { class: 'time' }, '—')),
          h('td', { class: 'time' }, formatTimestamp(s.endsAt)),
          h('td', null, h('button', { class: 'danger', onClick: () => cancel(s.id) }, 'Cancel')),
        ),
      )),
    ),
  );
}
