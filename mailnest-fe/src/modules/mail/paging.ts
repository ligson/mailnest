// 游标只在当前筛选和写入代次内复用；跳页或旧后端没有游标时保留页码兼容路径。
export class MailCursorHistory {
  private scope = '';
  private pages = new Map<number, string>();

  select(scope: string, page: number): string | undefined {
    if (scope !== this.scope) {
      this.scope = scope;
      this.pages.clear();
    }
    if (page === 1) this.pages.clear();
    return page > 1 ? this.pages.get(page) : undefined;
  }

  remember(scope: string, page: number, cursor?: string) {
    if (scope !== this.scope) return;
    for (const key of this.pages.keys()) if (key > page + 1) this.pages.delete(key);
    if (cursor) this.pages.set(page + 1, cursor);
    else this.pages.delete(page + 1);
    if (this.pages.size > 128) this.pages.delete(this.pages.keys().next().value!);
  }
}
