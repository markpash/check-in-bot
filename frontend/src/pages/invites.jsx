import { h } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { api } from '../api.js';
import { formatTimestamp } from '../time.js';
import { displayName } from '../users.js';

export default function Invites() {
  const [invites, setInvites] = useState(null);
  const [creating, setCreating] = useState(false);
  const [err, setErr] = useState(null);

  const load = () => api.getInvites()
    .then(data => {
      setInvites(data);
      setErr(null);
    })
    .catch(e => setErr(e.message));

  useEffect(() => { load(); }, []);

  async function createInvite() {
    try {
      setCreating(true);
      await api.createInvite();
      await load();
    } catch (e) {
      setErr(e.message);
    } finally {
      setCreating(false);
    }
  }

  if (err) return h('p', null, 'Error: ', err);
  if (!invites) return h('p', null, 'Loading...');

  return h('div', null,
    h('div', { class: 'page-header' },
      h('div', null,
        h('h1', null, 'Invite Codes'),
        h('p', { class: 'time' }, 'New users must register with `/start <invite_code>` before they can join the bot.'),
      ),
      h('button', { class: 'primary', disabled: creating, onClick: createInvite }, creating ? 'Creating...' : 'Generate invite'),
    ),
    invites.length === 0
      ? h('div', { class: 'empty' }, 'No invite codes yet.')
      : h('div', { class: 'table-shell' },
          h('table', null,
            h('thead', null, h('tr', null,
              h('th', null, 'Code'),
              h('th', null, 'Created'),
              h('th', null, 'Used By'),
              h('th', null, 'Used At'),
            )),
            h('tbody', null, invites.map(invite => {
              const usedBy = invite.usedBy
                ? displayName({
                    id: invite.usedBy,
                    firstName: invite.usedByFirstName,
                    lastName: invite.usedByLastName,
                    nickname: invite.usedByNickname,
                    username: invite.usedByUsername,
                  })
                : 'Unused';

              return h('tr', { key: invite.code },
                h('td', null, h('code', null, invite.code)),
                h('td', { class: 'time' }, formatTimestamp(invite.createdAt)),
                h('td', null, usedBy),
                h('td', { class: 'time' }, formatTimestamp(invite.usedAt)),
              );
            })),
          ),
        ),
  );
}
