export function lengthText(seconds: number, lower: boolean): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor(seconds / 60) % 60;
  const length = hours > 0
    ? minutes > 0 ? `${hours} 時間 ${minutes} 分` : `${hours} 時間`
    : minutes > 0 ? `${minutes} 分` : `${Math.max(1, seconds)} 秒`;
  return lower ? `${length}以上` : length;
}

export function clockText(at: number, now = new Date()): string {
  const moment = new Date(at);
  const hh = String(moment.getHours()).padStart(2, '0');
  const mm = String(moment.getMinutes()).padStart(2, '0');
  const clock = `${hh}:${mm}`;
  const today = moment.getFullYear() === now.getFullYear()
    && moment.getMonth() === now.getMonth() && moment.getDate() === now.getDate();
  return today ? clock : `${moment.getMonth() + 1}/${moment.getDate()} ${clock}`;
}

export function sinceText(at: number, lower: boolean): string {
  return `${clockText(at)} ${lower ? 'より前' : 'から'}`;
}
