import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';

const errorMessages = {
  auth_failed: 'Telegram authentication failed. Please try again.',
  not_admin: 'Access denied. Admin privileges required.',
  session_expired: 'Your session has expired. Please log in again.',
};

export default function Login({ error }) {
  const [config, setConfig] = useState(null);
  const message = error ? (errorMessages[error] || error) : null;

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
    h('div', { class: 'login-shell' },
      h('section', { class: 'login-panel card' },
        h('p', { class: 'eyebrow' }, config && config.dev ? 'Development' : 'Sign In'),
        h('h2', null, 'Check-In Bot Admin'),
        h('p', { class: 'login-copy' }, config && config.dev
          ? 'Sign in as the first admin user in the database.'
          : 'Log in with Telegram to continue.'),
        message && h('p', { class: 'login-error' }, message),
        config && config.dev
          ? h('button', {
              class: 'primary login-dev-button',
              onClick: () => { window.location.href = '/auth/dev'; },
            }, 'Dev Login')
          : h('div', { class: 'login-widget-wrap' },
              h('div', { id: 'telegram-login' }, config ? null : 'Loading...'),
            ),
      ),
    ),
  );
}
