/** 邮件正文独立于应用样式；保留表格的自然列宽，由阅读容器横向滚动。 */
export function buildMailBodySrcdoc(html: string) {
  return `<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <base target="_blank" />
    <style>
      html, body { margin: 0 !important; padding: 0 !important; background: transparent !important; }
      body {
        color: #1f2937;
        font-family: Arial, "Microsoft YaHei", sans-serif;
        font-size: 14px;
        line-height: 1.7;
        overflow-wrap: break-word;
        word-break: normal;
      }
      img, video, canvas, svg { max-width: 100%; height: auto; }
      table { border-collapse: collapse; }
      td, th { overflow-wrap: normal; word-break: normal; }
      pre { white-space: pre-wrap; overflow-wrap: break-word; }
    </style>
  </head>
  <body>${html}</body>
</html>`;
}
