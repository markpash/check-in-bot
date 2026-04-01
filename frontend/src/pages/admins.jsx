import { h } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { api } from '../api.js';
import { fullName } from '../users.js';

function openRow(path) {
  window.location.hash = path;
}

export default function Admins({ me }) {
  const [admins, setAdmins] = useState(null);
  const [adminID, setAdminID] = useState('');
  const [err, setErr] = useState(null);

  const load = () => api.getAdmins()
    .then(data => {
      setAdmins(data);
      setErr(null);
    })
    .catch(e => setErr(e.message));

  useEffect(() => { load(); }, []);

  async function addAdmin() {
    const id = Number(adminID.trim());
    if (!id) {
      setErr('Enter a numeric Telegram ID.');
      return;
    }
    try {
      await api.addAdmin(id);
      setAdminID('');
      await load();
    } catch (e) {
      setErr(e.message);
    }
  }

  async function removeAdmin(id) {
    try {
      await api.removeAdmin(id);
      await load();
    } catch (e) {
      setErr(e.message);
    }
  }

  if (err) return h('p', null, 'Error: ', err);
  if (!admins) return h('p', null, 'Loading...');

  return h('div', null,
    h('h1', null, 'Admins'),
    h('div', { class: 'card section' },
      h('h2', null, 'Add Admin'),
      h('div', { class: 'button-row' },
        h('input', {
          class: 'text-input',
          type: 'text',
          inputMode: 'numeric',
          placeholder: 'Telegram numeric ID',
          value: adminID,
          onInput: (e) => setAdminID(e.target.value),
        }),
        h('button', { class: 'primary', onClick: addAdmin }, 'Add admin'),
      ),
      h('p', { class: 'time', style: { marginTop: '12px' } }, 'New admins are granted admin access and their check-ins are enabled immediately.'),
    ),
    admins.length === 0
      ? h('div', { class: 'empty' }, 'No admins found.')
      : h('div', { class: 'table-shell' },
          h('table', null,
            h('thead', null, h('tr', null,
              h('th', null, 'ID'),
              h('th', null, 'Name'),
              h('th', null, 'Nickname'),
              h('th', null, 'Check-Ins'),
              h('th', null, ''),
            )),
            h('tbody', null, admins.map(admin =>
              h('tr', {
                key: admin.id,
                class: 'clickable-row',
                onClick: () => openRow(`/users/${admin.id}`),
              },
                h('td', null, h('code', null, admin.id)),
                h('td', null, fullName(admin)),
                h('td', null, admin.nickname || h('span', { class: 'time' }, '—')),
                h('td', null, h('button', { disabled: true, class: admin.checkinsEnabled ? 'primary' : '' }, admin.checkinsEnabled ? 'On' : 'Off')),
                h('td', null,
                  admin.id === me.id
                    ? h('span', { class: 'time' }, 'You')
                    : h('button', {
                        class: 'danger',
                        onClick: (e) => {
                          e.stopPropagation();
                          removeAdmin(admin.id);
                        },
                      }, 'Remove'),
                ),
              ),
            )),
          ),
        ),
  );
}
