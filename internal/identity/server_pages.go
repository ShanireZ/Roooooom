package identity

// ServerPageCSS styles the service-rendered activation and expired pages in the
// blueprint system. The activation file inlines it because its CSP forbids any
// fetch; the expired page links the identical web/public/assets/server-pages.css.
const ServerPageCSS = `:root{color-scheme:light dark;--paper:#f6f3ea;--ink:#1d3557;--muted:#1d3557ab;--grid:#1d355714}
@media (prefers-color-scheme:dark){:root{--paper:#0f1b2d;--ink:#d7e3f4;--muted:#d7e3f4ab;--grid:#d7e3f414}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;background-color:var(--paper);background-image:linear-gradient(var(--grid) 1px,transparent 1px),linear-gradient(90deg,var(--grid) 1px,transparent 1px);background-size:16px 16px;color:var(--ink);font:16px/1.7 "Segoe UI",system-ui,sans-serif}
html:lang(zh-TW) body{font-family:"Microsoft JhengHei","PingFang TC","Noto Sans TC",sans-serif}
html:lang(zh-CN) body{font-family:"Microsoft YaHei","PingFang SC","Noto Sans SC",sans-serif}
html:lang(ja) body{font-family:"Yu Gothic UI","Hiragino Sans","Noto Sans JP",sans-serif}
main{max-width:520px;margin:14vh auto;padding:32px 36px;background:var(--paper);border:1.5px solid var(--ink);box-shadow:5px 5px 0 var(--ink)}
.mark{display:block;font:800 13px/1 "Cascadia Mono","JetBrains Mono",Consolas,ui-monospace,monospace;letter-spacing:1px}
h1{font-size:24px;line-height:1.4;margin:14px 0 10px}
p{color:var(--muted);margin:0 0 16px}
button{font:inherit;font-weight:700;background:var(--ink);color:var(--paper);border:1.5px solid var(--ink);border-radius:0;padding:10px 22px;min-height:48px;cursor:pointer}
button:focus-visible{outline:3px solid var(--ink);outline-offset:3px}
small{display:block;color:var(--muted);margin-top:24px;font-size:13px}
@media (max-width:600px){main{margin:8vh 16px;padding:24px}}
`
