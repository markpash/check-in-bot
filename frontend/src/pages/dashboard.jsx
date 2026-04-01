import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { api } from '../api.js';
import { formatTimestamp } from '../time.js';
import { displayName } from '../users.js';

function Section({ title, badge, items, renderRow, emptyText }) {
  return h('div', { class: 'section' },
    h('div', { class: 'section-header' },
      h('h2', null, title),
      h('span', { class: `badge ${badge}` }, items.length),
    ),
    items.length === 0
      ? h('div', { class: 'empty' }, emptyText)
      : h('table', null,
          h('thead', null, h('tr', null,
            h('th', null, 'User'),
            h('th', null, 'Pinged'),
            h('th', null, 'Status'),
          )),
          h('tbody', null, items.map(renderRow)),
        ),
  );
}

export default function Dashboard() {
  const [data, setData] = useState(null);
  const [err, setErr] = useState(null);

  const load = () => api.getDashboard().then(setData).catch(e => setErr(e.message));

  useEffect(() => { load(); const t = setInterval(load, 60000); return () => clearInterval(t); }, []);

  if (err) return h('p', null, 'Error: ', err);
  if (!data) return h('p', null, 'Loading...');

  return h('div', null,
    h('h1', null, 'Dashboard'),

    Section({
      title: 'Missed',
      badge: 'badge-red',
      items: data.missed || [],
      emptyText: 'No missed check-ins.',
      renderRow: (c) => h('tr', { key: c.id },
        h('td', null, h('a', { href: `#/users/${c.userId}`, class: 'list-link' }, displayName(c))),
        h('td', { class: 'time' }, formatTimestamp(c.pingedAt)),
        h('td', null, h('span', { class: 'badge badge-red' }, 'Missed')),
      ),
    }),

    Section({
      title: 'Pending',
      badge: 'badge-yellow',
      items: data.pending || [],
      emptyText: 'No pending check-ins.',
      renderRow: (c) => h('tr', { key: c.id },
        h('td', null, h('a', { href: `#/users/${c.userId}`, class: 'list-link' }, displayName(c))),
        h('td', { class: 'time' }, formatTimestamp(c.pingedAt)),
        h('td', null, h('span', { class: 'badge badge-yellow' }, 'Waiting')),
      ),
    }),

    Section({
      title: 'Checked In',
      badge: 'badge-green',
      items: data.checkedIn || [],
      emptyText: 'No check-ins yet today.',
      renderRow: (c) => h('tr', { key: c.id },
        h('td', null, h('a', { href: `#/users/${c.userId}`, class: 'list-link' }, displayName(c))),
        h('td', { class: 'time' }, formatTimestamp(c.checkedInAt)),
        h('td', null,
          h('span', { class: 'badge badge-green' }, 'OK'),
          c.note ? h('span', null, ` — ${c.note}`) : null,
        ),
      ),
    }),

    Section({
      title: 'Silenced',
      badge: 'badge-gray',
      items: data.silenced || [],
      emptyText: 'No silenced users.',
      renderRow: (s) => h('tr', { key: s.id },
        h('td', null, h('a', { href: `#/users/${s.userId}`, class: 'list-link' }, displayName(s))),
        h('td', { class: 'time' }, `Until ${formatTimestamp(s.endsAt)}`),
        h('td', null, s.reason || h('span', { class: 'time' }, 'No reason given')),
      ),
    }),
  );
}
