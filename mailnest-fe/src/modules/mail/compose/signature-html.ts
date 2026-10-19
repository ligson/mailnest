/** 标记完整签名，避免编辑器把布局表格和品牌样式转换成普通正文。 */
export function wrapMailSignature(html: string) {
  return `<section data-mailnest-signature="true">${html}</section>`;
}
