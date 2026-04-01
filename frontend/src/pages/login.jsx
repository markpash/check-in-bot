import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';

const errorMessages = {
  auth_failed: 'Telegram authentication failed. Please try again.',
  not_admin: 'Access denied. Admin privileges required.',
  session_expired: 'Your session has expired. Please log in again.',
};

export default function Login({ error }) {
  const [config, setConfig] = useState(null);

  useEffect(() => {
    fetch('/api/config').then(r => r.json()).then(setConfig).catch(() => {});
  }, []);

  useEffect(() => {
    if (!config || config.dev) return;

    // Load the Telegram Login Widget script.
    const container = document.getElementById('telegram-login');
    if (!container) return;

    const script = document.createElement('script');
    script.src = 'https://telegram.org/js/telegram-widget.js?22';
    script.async = true;
    script.setAttribute('data-telegram-login', config.botUsername || '');
    script.setAttribute('data-size', 'large');
    script.setAttribute('data-auth-url', config.baseURL + '/auth/telegram/callback');
    script.setAttribute('data-request-access', 'write');
    container.innerHTML = '';
    container.appendChild(script);
  }, [config]);

  return h('div', { class: 'login-page' },
    h('h1', null, 'Check-In Bot Admin'),
    error && h('p', { class: 'login-error' }, errorMessages[error] || error),
    config && config.dev
      ? h('div', null,
          h('p', null, 'Dev mode is enabled. Click below to log in as the first admin.'),
          h('button', {
            class: 'primary',
            style: { marginTop: '12px', padding: '10px 24px', fontSize: '16px' },
            onClick: () => { window.location.href = '/auth/dev'; },
          }, 'Dev Login'),
        )
      : h('div', null,
          h('p', null, 'Log in with your Telegram account to continue.'),
          h('div', { id: 'telegram-login' }, config ? null : 'Loading...'),
        ),
  );
}
