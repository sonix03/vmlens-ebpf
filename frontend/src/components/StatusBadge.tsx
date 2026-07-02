interface Props { status?: string }

export function StatusBadge({ status = 'unknown' }: Props) {
  return <span className={`status-badge status-${status}`}>{status}</span>
}

