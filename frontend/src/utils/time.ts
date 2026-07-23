export function formatUTCClock(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  const parts = [
    date.getUTCHours(),
    date.getUTCMinutes(),
    date.getUTCSeconds(),
  ].map((item) => String(item).padStart(2, '0'))
  return parts.join(':')
}
