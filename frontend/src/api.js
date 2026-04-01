async function request(method, path, body) {
  const opts = {
    method,
    credentials: 'same-origin',
    headers: {},
  };
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  if (res.status === 401 || res.status === 403) {
    window.location.hash = '#/login?error=session_expired';
    throw new Error('Unauthorized');
  }
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}`);
  }
  return res.json();
}

async function logoutRequest() {
  await fetch('/auth/logout', {
    method: 'POST',
    credentials: 'same-origin',
  });
}

export const api = {
  getConfig: () => request('GET', '/api/config'),
  getMe: () => request('GET', '/api/me'),
  getDashboard: () => request('GET', '/api/dashboard'),
  getUsers: () => request('GET', '/api/users'),
  getAdmins: () => request('GET', '/api/admins'),
  createUser: (id) => request('POST', '/api/users', { id }),
  getUser: (id) => request('GET', `/api/users/${id}`),
  setUserCheckins: (id, enabled) => request('POST', `/api/users/${id}/checkins`, { enabled }),
  addAdmin: (id) => request('POST', '/api/admins', { id }),
  removeAdmin: (id) => request('DELETE', `/api/admins/${id}`),
  setUserNote: (id, note) => request('POST', `/api/users/${id}/note`, { note }),
  setUserNickname: (id, nickname) => request('POST', `/api/users/${id}/nickname`, { nickname }),
  setUserSchedule: (id, schedule) => request('POST', `/api/users/${id}/schedule`, { schedule }),
  getCheckins: (date) => request('GET', `/api/checkins${date ? `?date=${date}` : ''}`),
  getSilences: () => request('GET', '/api/silences'),
  deleteSilence: (id) => request('DELETE', `/api/silences/${id}`),
  getMessages: (unread) => request('GET', `/api/messages${unread ? '?unread=true' : ''}`),
  getMessage: (id) => request('GET', `/api/messages/${id}`),
  markRead: (id) => request('POST', `/api/messages/${id}/read`),
  markAllRead: () => request('POST', '/api/messages/read-all'),
  getInvites: () => request('GET', '/api/invites'),
  createInvite: () => request('POST', '/api/invites'),
  logout: () => logoutRequest(),
};
