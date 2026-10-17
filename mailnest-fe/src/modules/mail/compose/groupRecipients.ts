import type { GroupMember } from '../../contacts/types';

function emailKey(value: string): string {
  const address = value.match(/<([^<>]+)>\s*$/)?.[1] || value;
  return address.trim().toLowerCase();
}

// 只对当前地址栏去重，绝不将密送成员转入其他栏；保留手工输入和原有排序。
export function mergeGroupRecipients(existing: string[], members: GroupMember[]): string[] {
  const result = [...existing];
  const seen = new Set(existing.map(emailKey));
  for (const member of members) {
    const email = member.email.trim();
    const key = emailKey(email);
    if (!key || seen.has(key)) continue;
    const name = member.nickname || member.displayName || member.name;
    const escapedName = (name || '').replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/[\r\n]/g, ' ');
    result.push(name && name !== email ? `"${escapedName}" <${email}>` : email);
    seen.add(key);
  }
  return result;
}
