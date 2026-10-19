// 写信页只依赖轻量桥接类型，编辑内核在打开写信时才加载。
export interface ComposeEditorBridge {
  getHTML(): string;
  getText(): string;
  setContent(html: string): void;
  insertHTML(html: string): void;
}
