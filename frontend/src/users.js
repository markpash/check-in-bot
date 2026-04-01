export function fullName(row) {
  const parts = [row?.firstName, row?.lastName].filter(Boolean);
  if (parts.length > 0) return parts.join(' ');
  if (row?.username) return `@${row.username}`;
  if (row?.id) return `User ${row.id}`;
  return 'Unknown';
}

export function displayName(row) {
  const name = fullName(row);
  const label = row?.nickname ? `${row.nickname} (${name})` : name;
  return row?.username ? `${label} (@${row.username})` : label;
}
