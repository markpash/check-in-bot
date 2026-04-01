function parseUTC(utcStr) {
  if (!utcStr) return null;
  return new Date(`${utcStr}Z`);
}

function pad(value) {
  return String(value).padStart(2, '0');
}

function formatAbsolute(date) {
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`;
}

function formatRelative(date) {
  const diffSeconds = Math.floor((Date.now() - date.getTime()) / 1000);
  const isFuture = diffSeconds < 0;
  const absDiffSeconds = Math.abs(diffSeconds);
  const units = [
    { name: 'month', seconds: 60 * 60 * 24 * 30 },
    { name: 'week', seconds: 60 * 60 * 24 * 7 },
    { name: 'day', seconds: 60 * 60 * 24 },
    { name: 'hour', seconds: 60 * 60 },
    { name: 'minute', seconds: 60 },
    { name: 'second', seconds: 1 },
  ];

  for (const unit of units) {
    if (absDiffSeconds >= unit.seconds || unit.name === 'second') {
      const value = Math.floor(absDiffSeconds / unit.seconds);
      const suffix = value === 1 ? unit.name : `${unit.name}s`;
      return isFuture ? `in ${value} ${suffix}` : `${value} ${suffix} ago`;
    }
  }

  return isFuture ? 'in 0 seconds' : '0 seconds ago';
}

export function formatTimestamp(utcStr) {
  const date = parseUTC(utcStr);
  if (!date || Number.isNaN(date.getTime())) return '—';
  return `${formatAbsolute(date)} (${formatRelative(date)})`;
}
