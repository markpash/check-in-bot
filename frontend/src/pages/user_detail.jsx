import { h } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { api } from '../api.js';
import { formatTimestamp } from '../time.js';
import { displayName, fullName } from '../users.js';

function openRow(path) {
  window.location.hash = path;
}

function LabelValue({ label, value }) {
  return h('div', { class: 'meta-row' },
    h('dt', null, label),
    h('dd', null, value || '—'),
  );
}

function ToggleSwitch({ label, checked, disabled, onChange }) {
  return h('label', { class: `switch-row${disabled ? ' disabled' : ''}` },
    h('span', { class: 'switch-label' }, label),
    h('span', { class: 'switch-control' },
      h('input', {
        type: 'checkbox',
        checked,
        disabled,
        onChange,
      }),
      h('span', { class: 'switch-slider', 'aria-hidden': 'true' }),
    ),
  );
}

function pad2(value) {
  return String(value).padStart(2, '0');
}

function describeSchedule(schedule) {
  if (!schedule) return 'Uses global schedule';

  const daily = schedule.match(/^(\d{1,2}) (\d{1,2}) \* \* \*$/);
  if (daily) return `Every day at ${pad2(daily[2])}:${pad2(daily[1])} UTC`;

  const weekdays = schedule.match(/^(\d{1,2}) (\d{1,2}) \* \* 1-5$/);
  if (weekdays) return `Weekdays at ${pad2(weekdays[2])}:${pad2(weekdays[1])} UTC`;

  const everyHours = schedule.match(/^(\d{1,2}) \*\/(\d{1,2}) \* \* \*$/);
  if (everyHours) return `Every ${everyHours[2]} hours at :${pad2(everyHours[1])} UTC`;

  return `Custom cron: ${schedule}`;
}

function parseSchedule(schedule) {
  if (!schedule) {
    return {
      mode: 'global',
      hour: '09',
      minute: '00',
      intervalHours: '6',
      custom: '',
    };
  }

  const daily = schedule.match(/^(\d{1,2}) (\d{1,2}) \* \* \*$/);
  if (daily) {
    return {
      mode: 'daily',
      hour: pad2(daily[2]),
      minute: pad2(daily[1]),
      intervalHours: '6',
      custom: schedule,
    };
  }

  const weekdays = schedule.match(/^(\d{1,2}) (\d{1,2}) \* \* 1-5$/);
  if (weekdays) {
    return {
      mode: 'weekdays',
      hour: pad2(weekdays[2]),
      minute: pad2(weekdays[1]),
      intervalHours: '6',
      custom: schedule,
    };
  }

  const everyHours = schedule.match(/^(\d{1,2}) \*\/(\d{1,2}) \* \* \*$/);
  if (everyHours) {
    return {
      mode: 'every-hours',
      hour: '09',
      minute: pad2(everyHours[1]),
      intervalHours: everyHours[2],
      custom: schedule,
    };
  }

  return {
    mode: 'custom',
    hour: '09',
    minute: '00',
    intervalHours: '6',
    custom: schedule,
  };
}

function buildSchedule(mode, hour, minute, intervalHours, custom) {
  switch (mode) {
    case 'global':
      return '';
    case 'daily':
      return `${Number(minute)} ${Number(hour)} * * *`;
    case 'weekdays':
      return `${Number(minute)} ${Number(hour)} * * 1-5`;
    case 'every-hours':
      return `${Number(minute)} */${Number(intervalHours)} * * *`;
    case 'custom':
      return custom.trim();
    default:
      return '';
  }
}

export default function UserDetail({ userID, me }) {
  const [data, setData] = useState(null);
  const [nickname, setNickname] = useState('');
  const [note, setNote] = useState('');
  const [scheduleMode, setScheduleMode] = useState('global');
  const [scheduleHour, setScheduleHour] = useState('09');
  const [scheduleMinute, setScheduleMinute] = useState('00');
  const [scheduleIntervalHours, setScheduleIntervalHours] = useState('6');
  const [customSchedule, setCustomSchedule] = useState('');
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState(null);

  const load = () => api.getUser(userID)
    .then(res => {
      const parsedSchedule = parseSchedule(res.user.checkinSchedule || '');
      setData(res);
      setNickname(res.user.nickname || '');
      setNote(res.user.note || '');
      setScheduleMode(parsedSchedule.mode);
      setScheduleHour(parsedSchedule.hour);
      setScheduleMinute(parsedSchedule.minute);
      setScheduleIntervalHours(parsedSchedule.intervalHours);
      setCustomSchedule(parsedSchedule.custom);
      setErr(null);
    })
    .catch(e => setErr(e.message));

  useEffect(() => { load(); }, [userID]);

  async function toggleCheckins() {
    try {
      await api.setUserCheckins(userID, !data.user.checkinsEnabled);
      await load();
    } catch (e) { setErr(e.message); }
  }

  async function promoteToAdmin() {
    try {
      await api.addAdmin(Number(userID));
      await load();
    } catch (e) { setErr(e.message); }
  }

  async function demoteAdmin() {
    try {
      await api.removeAdmin(Number(userID));
      await load();
    } catch (e) { setErr(e.message); }
  }

  async function saveDetails() {
    const builtSchedule = buildSchedule(scheduleMode, scheduleHour, scheduleMinute, scheduleIntervalHours, customSchedule);
    try {
      setSaving(true);
      await api.setUserNickname(userID, nickname);
      await api.setUserNote(userID, note);
      await api.setUserSchedule(userID, builtSchedule);
      await load();
    } catch (e) {
      setErr(e.message);
    } finally {
      setSaving(false);
    }
  }

  if (err) return h('p', null, 'Error: ', err);
  if (!data) return h('p', null, 'Loading...');

  const user = data.user;
  const recentCheckins = data.recentCheckins || [];
  const recentMessages = data.recentMessages || [];
  const activeSilences = data.activeSilences || [];
  const builtSchedule = buildSchedule(scheduleMode, scheduleHour, scheduleMinute, scheduleIntervalHours, customSchedule);
  const detailsChanged = note !== (user.note || '') || nickname !== (user.nickname || '') || builtSchedule !== (user.checkinSchedule || '');
  const hourOptions = Array.from({ length: 24 }, (_, hour) => pad2(hour));
  const minuteOptions = Array.from({ length: 60 }, (_, minute) => pad2(minute));
  const intervalOptions = ['2', '3', '4', '6', '8', '12', '24'];

  return h('div', null,
    h('div', { class: 'page-header' },
      h('div', null,
        h('a', { href: user.isAdmin ? '#/admins' : '#/users', class: 'back-link' }, user.isAdmin ? '< Back to admins' : '< Back to users'),
        h('h1', null, fullName(user)),
        h('p', { class: 'time' }, displayName(user)),
      ),
      h('div', { class: 'switch-group' },
        h(ToggleSwitch, {
          label: 'Check-ins enabled',
          checked: user.checkinsEnabled,
          onChange: toggleCheckins,
        }),
        h(ToggleSwitch, {
          label: user.id === me?.id ? 'Admin (you)' : 'Admin',
          checked: user.isAdmin,
          disabled: user.id === me?.id,
          onChange: user.isAdmin ? demoteAdmin : promoteToAdmin,
        }),
      ),
    ),

    h('section', { class: 'hero-card section' },
      h('div', { class: 'hero-card-main' },
        h('p', { class: 'eyebrow' }, 'User overview'),
        h('h2', null, fullName(user)),
        h('p', { class: 'hero-copy' }, user.nickname || 'No nickname set.'),
      ),
      h('div', { class: 'hero-card-stats' },
        h('div', { class: 'stat-chip' }, h('span', null, 'Messages'), h('strong', null, recentMessages.length)),
        h('div', { class: 'stat-chip' }, h('span', null, 'Check-ins'), h('strong', null, recentCheckins.length)),
        h('div', { class: 'stat-chip' }, h('span', null, 'Silences'), h('strong', null, activeSilences.length)),
      ),
    ),

    h('section', { class: 'detail-grid' },
      h('div', { class: 'card' },
        h('h2', null, 'Telegram Profile'),
        h('dl', { class: 'meta-grid' },
          h(LabelValue, { label: 'Telegram ID', value: h('code', null, user.id) }),
          h(LabelValue, { label: 'Username', value: user.username ? `@${user.username}` : '—' }),
          h(LabelValue, { label: 'First name', value: user.firstName }),
          h(LabelValue, { label: 'Last name', value: user.lastName }),
          h(LabelValue, { label: 'Language', value: user.languageCode }),
          h(LabelValue, { label: 'Premium', value: user.isPremium ? 'Yes' : 'No' }),
          h(LabelValue, { label: 'Bot account', value: user.isBot ? 'Yes' : 'No' }),
          h(LabelValue, { label: 'Check-ins enabled', value: user.checkinsEnabled ? 'Yes' : 'No' }),
          h(LabelValue, { label: 'Check-in schedule', value: describeSchedule(user.checkinSchedule || '') }),
          h(LabelValue, { label: 'Admin', value: user.isAdmin ? 'Yes' : 'No' }),
          h(LabelValue, { label: 'Created', value: formatTimestamp(user.createdAt) }),
          h(LabelValue, { label: 'Updated', value: formatTimestamp(user.updatedAt) }),
        ),
      ),
      h('div', { class: 'card' },
        h('h2', null, 'Details'),
        h('label', { class: 'field-label' }, 'Nickname'),
        h('input', {
          class: 'text-input',
          type: 'text',
          value: nickname,
          placeholder: 'Optional nickname',
          onInput: (e) => setNickname(e.target.value),
        }),
        h('label', { class: 'field-label', style: { marginTop: '12px' } }, 'Check-In Schedule Override'),
        h('select', {
          class: 'text-input',
          value: scheduleMode,
          onChange: (e) => setScheduleMode(e.target.value),
        },
          h('option', { value: 'global' }, 'Use global schedule'),
          h('option', { value: 'daily' }, 'Every day at a specific time'),
          h('option', { value: 'weekdays' }, 'Weekdays at a specific time'),
          h('option', { value: 'every-hours' }, 'Every few hours'),
          h('option', { value: 'custom' }, 'Custom cron expression'),
        ),
        (scheduleMode === 'daily' || scheduleMode === 'weekdays') && h('div', { class: 'schedule-builder-row' },
          h('select', {
            class: 'text-input',
            value: scheduleHour,
            onChange: (e) => setScheduleHour(e.target.value),
          }, hourOptions.map(hour => h('option', { key: hour, value: hour }, `${hour} hour`))),
          h('select', {
            class: 'text-input',
            value: scheduleMinute,
            onChange: (e) => setScheduleMinute(e.target.value),
          }, minuteOptions.map(minute => h('option', { key: minute, value: minute }, `${minute} minute`))),
        ),
        scheduleMode === 'every-hours' && h('div', { class: 'schedule-builder-row' },
          h('select', {
            class: 'text-input',
            value: scheduleIntervalHours,
            onChange: (e) => setScheduleIntervalHours(e.target.value),
          }, intervalOptions.map(interval => h('option', { key: interval, value: interval }, `Every ${interval} hours`))),
          h('select', {
            class: 'text-input',
            value: scheduleMinute,
            onChange: (e) => setScheduleMinute(e.target.value),
          }, minuteOptions.map(minute => h('option', { key: minute, value: minute }, `At :${minute} UTC`))),
        ),
        scheduleMode === 'custom' && h('input', {
          class: 'text-input',
          type: 'text',
          value: customSchedule,
          placeholder: 'minute hour day-of-month month day-of-week',
          onInput: (e) => setCustomSchedule(e.target.value),
        }),
        h('p', { class: 'time', style: { marginTop: '8px', marginBottom: '0' } },
          scheduleMode === 'global'
            ? 'This user will follow the global check-in schedule.'
            : scheduleMode === 'custom'
              ? 'Advanced mode. Enter a cron expression in UTC.'
              : `This will save as: ${builtSchedule}`,
        ),
        h('label', { class: 'field-label', style: { marginTop: '12px' } }, 'Admin Note'),
        h('textarea', {
          class: 'detail-textarea',
          rows: 8,
          value: note,
          placeholder: 'Add an internal note for this user',
          onInput: (e) => setNote(e.target.value),
        }),
        h('div', { class: 'button-row', style: { marginTop: '12px' } },
          h('button', {
            class: detailsChanged ? 'primary' : '',
            disabled: !detailsChanged || saving,
            onClick: saveDetails,
          }, saving ? 'Saving...' : 'Save changes'),
        ),
      ),
    ),

    h('section', { class: 'card section' },
      h('h2', null, 'Active Silences'),
      activeSilences.length === 0
        ? h('div', { class: 'empty compact-empty' }, 'No active silences.')
        : h('table', null,
            h('thead', null, h('tr', null,
              h('th', null, 'Days'),
              h('th', null, 'Reason'),
              h('th', null, 'Ends'),
            )),
            h('tbody', null, activeSilences.map(s =>
              h('tr', { key: s.id },
                h('td', null, s.days),
                h('td', null, s.reason || '—'),
                h('td', { class: 'time' }, formatTimestamp(s.endsAt)),
              ),
            )),
          ),
    ),

    h('section', { class: 'card section' },
      h('h2', null, 'Recent Check-Ins'),
      recentCheckins.length === 0
        ? h('div', { class: 'empty compact-empty' }, 'No check-ins recorded yet.')
        : h('table', null,
            h('thead', null, h('tr', null,
              h('th', null, 'Pinged'),
              h('th', null, 'Checked In'),
              h('th', null, 'Note'),
            )),
            h('tbody', null, recentCheckins.map(c =>
              h('tr', { key: c.id },
                h('td', { class: 'time' }, formatTimestamp(c.pingedAt)),
                h('td', { class: 'time' }, formatTimestamp(c.checkedInAt)),
                h('td', { class: 'msg-body' }, c.note || '—'),
              ),
            )),
          ),
    ),

    h('section', { class: 'card section' },
      h('h2', null, 'Recent Inbox Messages'),
      recentMessages.length === 0
        ? h('div', { class: 'empty compact-empty' }, 'No messages sent yet.')
        : h('div', { class: 'table-shell' },
            h('table', null,
              h('thead', null, h('tr', null,
                h('th', null, 'Time'),
                h('th', null, 'Preview'),
                h('th', null, 'Status'),
              )),
              h('tbody', null, recentMessages.map(m =>
                h('tr', {
                  key: m.id,
                  class: 'clickable-row',
                  onClick: () => openRow(`/inbox/${m.id}`),
                },
                  h('td', { class: 'time' }, formatTimestamp(m.createdAt)),
                  h('td', null, m.body.length > 96 ? `${m.body.slice(0, 96)}...` : m.body),
                  h('td', null, m.isRead ? 'Read' : 'Unread'),
                ),
              )),
            )),
    ),
  );
}
