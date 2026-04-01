import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { api } from './api.js';
import { fullName } from './users.js';
import Login from './pages/login.jsx';
import Dashboard from './pages/dashboard.jsx';
import Users from './pages/users.jsx';
import Admins from './pages/admins.jsx';
import Inbox from './pages/inbox.jsx';
import UserDetail from './pages/user_detail.jsx';
import MessageDetail from './pages/message_detail.jsx';
import Silences from './pages/silences.jsx';

function getRoute() {
  const hash = window.location.hash.replace('#', '') || '/';
  const [path, query] = hash.split('?');
  const params = new URLSearchParams(query || '');
  return { path, params };
}

function pathParam(path, prefix) {
  if (!path.startsWith(prefix)) return null;
  const value = path.slice(prefix.length);
  return value && !value.includes('/') ? value : null;
}

export default function App() {
  const [route, setRoute] = useState(getRoute());
  const [me, setMe] = useState(null);
  const [loading, setLoading] = useState(true);

  async function logout(e) {
    e.preventDefault();
    try {
      await api.logout();
    } catch (_) {
      // Even if the request fails, treat the session as ended client-side.
    }
    setMe(null);
    setLoading(false);
    window.location.hash = '#/login';
  }

  useEffect(() => {
    const onHash = () => setRoute(getRoute());
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  useEffect(() => {
    api.getMe()
      .then(u => { setMe(u); setLoading(false); })
      .catch(() => { setMe(null); setLoading(false); });
  }, []);

  if (loading) return h('div', { id: 'app' }, 'Loading...');

  if (!me || route.path === '/login') {
    return h(Login, { error: route.params.get('error') });
  }

  const nav = h('nav', null,
    h('a', { href: '#/', class: route.path === '/' ? 'active' : '' }, 'Dashboard'),
    h('a', { href: '#/users', class: route.path === '/users' ? 'active' : '' }, 'Users'),
    h('a', { href: '#/admins', class: route.path === '/admins' ? 'active' : '' }, 'Admins'),
    h('a', { href: '#/inbox', class: route.path === '/inbox' ? 'active' : '' }, 'Inbox'),
    h('a', { href: '#/silences', class: route.path === '/silences' ? 'active' : '' }, 'Silences'),
    h('span', { class: 'spacer' }),
    h('span', { class: 'user-info' }, `${fullName(me)} `),
    h('a', { href: '#/login', onClick: logout }, 'Logout'),
  );

  let page;
  switch (route.path) {
    case '/users': page = h(Users); break;
    case '/admins': page = h(Admins, { me }); break;
    case '/inbox': page = h(Inbox); break;
    case '/silences': page = h(Silences); break;
    default: {
      const userID = pathParam(route.path, '/users/');
      const messageID = pathParam(route.path, '/inbox/');
      if (userID) page = h(UserDetail, { userID, me });
      else if (messageID) page = h(MessageDetail, { messageID });
      else page = h(Dashboard);
      break;
    }
  }

  return h('div', null, nav, page);
}
