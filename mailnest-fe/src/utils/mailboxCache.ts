// 仅内存保存短期列表，不写 localStorage；切换登录态或成功写操作后立即清空。
let owner = '';
let epoch = 0;
const entries = new Map<string, { value: unknown; expires: number }>();

export function clearMailboxCache() { entries.clear(); owner = ''; epoch += 1; }
export function mailboxCacheEpoch() { return epoch; }
export function getMailboxCache<T>(scope: string, key: string): T | undefined {
  if (owner !== scope) { clearMailboxCache(); owner = scope; }
  const entry = entries.get(key);
  if (!entry || entry.expires <= Date.now()) { entries.delete(key); return undefined; }
  return structuredClone(entry.value) as T;
}
export function setMailboxCache<T>(scope: string, key: string, value: T, expectedEpoch: number) {
  if (owner !== scope || epoch !== expectedEpoch) return;
  if (entries.size >= 24 && !entries.has(key)) entries.delete(entries.keys().next().value!);
  entries.set(key, { value: structuredClone(value), expires: Date.now() + 15000 });
}
