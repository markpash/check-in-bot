import { h } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { api } from '../api.js';
import { fullName } from '../users.js';

function openRow(path) {
  window.location.hash = path;
}

function statusButton(label, active) {
  return h('button', { disabled: true, class: active ? 'primary' : '' }, active ? label : 'No');
}

export default function Users() {
  const [users, setUsers] = useState(null);
  const [invites, setInvites] = useState(null);
  const [manualID, setManualID] = useState('');
  const [creatingInvite, setCreatingInvite] = useState(false);
  const [addingUser, setAddingUser] = useState(false);
  const [err, setErr] = useState(null);

  const load = () => Promise.all([api.getUsers(), api.getInvites()])
    .then(([userData, inviteData]) => {
      setUsers(userData);
      setInvites(inviteData);
      setErr(null);
    })
    .catch(e => setErr(e.message));

  useEffect(() => { load(); }, []);

  async function createInvite() {
    try {
      setCreatingInvite(true);
      await api.createInvite();
      await load();
    } catch (e) {
      setErr(e.message);
    } finally {
      setCreatingInvite(false);
    }
  }

  async function addManualUser() {
    const id = Number(manualID.trim());
    if (!id) {
      setErr('Enter a numeric Telegram ID.');
      return;
    }
    try {
      setAddingUser(true);
      await api.createUser(id);
      setManualID('');
      await load();
    } catch (e) {
      setErr(e.message);
    } finally {
      setAddingUser(false);
    }
  }

  if (err) return h('p', null, 'Error: ', err);
  if (!users || !invites) return h('p', null, 'Loading...');

  const latestInvite = invites.find(invite => !invite.usedBy) || invites[0] || null;

  return h('div', null,
    h('h1', null, 'Users'),
    h('div', { class: 'detail-grid section' },
      h('div', { class: 'card' },
        h('h2', null, 'Add User by Invite Code'),
        h('button', { class: 'primary', disabled: creatingInvite, onClick: createInvite }, creatingInvite ? 'Generating...' : 'Generate invite code'),
        latestInvite && h('p', { style: { marginTop: '12px' } }, h('code', null, latestInvite.code)),
        h('p', { class: 'time', style: { marginTop: '12px' } }, 'Give the user this invite code and tell them to open Telegram, start the bot, and send `/start <invite_code>`. Once they do, they will be added automatically and check-ins will be enabled.'),
      ),
      h('div', { class: 'card' },
        h('h2', null, 'Add User by Telegram ID'),
        h('div', { class: 'button-row' },
          h('input', {
            class: 'text-input',
            type: 'text',
            inputMode: 'numeric',
            placeholder: 'Telegram numeric ID',
            value: manualID,
            onInput: (e) => setManualID(e.target.value),
          }),
          h('button', { class: 'primary', disabled: addingUser, onClick: addManualUser }, addingUser ? 'Adding...' : 'Add user'),
        ),
        h('p', { class: 'time', style: { marginTop: '12px' } }, 'Use this when you already know the user’s Telegram numeric ID. This creates the user immediately and enables check-ins, but prompts will only start once the user actually opens the bot and sends `/start`.'),
      ),
    ),
    h('p', { class: 'time', style: { marginBottom: '16px' } }, 'Open a user to edit their nickname, admin note, check-in access, or promote them to admin.'),
    users.length === 0
      ? h('div', { class: 'empty' }, 'No users yet. Use one of the add-user methods above.')
      : h('div', { class: 'table-shell' },
          h('table', null,
            h('thead', null, h('tr', null,
              h('th', null, 'ID'),
              h('th', null, 'Name'),
              h('th', null, 'Nickname'),
              h('th', null, 'Check-Ins'),
              h('th', null, 'Admin'),
            )),
            h('tbody', null, users.map(u =>
              h('tr', {
                key: u.id,
                class: 'clickable-row',
                onClick: () => openRow(`/users/${u.id}`),
              },
                h('td', null, h('code', null, u.id)),
                h('td', null, fullName(u)),
                h('td', null, u.nickname || h('span', { class: 'time' }, '—')),
                h('td', null, statusButton('On', u.checkinsEnabled)),
                h('td', null, statusButton('Yes', u.isAdmin)),
              ),
            )),
          ),
        ),
  );
}
