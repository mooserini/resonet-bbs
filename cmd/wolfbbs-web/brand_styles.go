package main

// Resonant Mirror brand layer (BRAND_SPEC v1.0). These rules are unlayered,
// so they beat everything in @layer wolfbbs-legacy regardless of specificity.
// That is what keeps every browser, theme mode, and accent mode on the same
// palette: legacy rules can shape layout, but they can no longer paint text
// or surfaces. Only the spec's colors (and alpha blends of them) appear here.
const brandStyleTag = `<style id="wolfbbs-brand">
@font-face{
  font-family:"Web437 ATT PC6300";
  src:url("/assets/fonts/Web437_ATT_PC6300.woff") format("woff");
  font-display:swap;
}
:root{
  color-scheme:dark;
  --rm-ink:#171819;
  --rm-cream:#EDE3D5;
  --rm-chrome-top:#F7EEE2;
  --rm-chrome-mid:#D7C5B2;
  --rm-chrome-bottom:#A98B70;
  --rm-brown:#4A342A;
  --rm-copper:#80624B;
  --rm-sub:#B7A593;
  --rm-pagenum:#595149;
  --rm-green:#00AA00;
  --rm-magenta:#AD4CAC;
  --rm-yellow:#FFFF55;
  --rm-blue:#0000FF;
  --rm-red:#FF0000;
  --rm-surface:rgba(247,238,226,.045);
  --rm-surface-2:rgba(247,238,226,.08);
  --rm-line:rgba(183,165,147,.28);
  --rm-line-strong:rgba(183,165,147,.55);
  --rm-chrome:linear-gradient(180deg,var(--rm-chrome-top) 0%,var(--rm-chrome-mid) 55%,var(--rm-chrome-bottom) 100%);
  --rm-font-display:"Web437 ATT PC6300","Ac437 ATT PC6300",ui-monospace,Menlo,monospace;
  --rm-font-body:"SF Pro Text",-apple-system,BlinkMacSystemFont,"Helvetica Neue",Helvetica,Arial,sans-serif;
  --rm-font-code:Menlo,ui-monospace,"SFMono-Regular",Consolas,monospace;
  /* Legacy tokens, remapped so anything still reading them stays on-brand. */
  --bg:var(--rm-ink);
  --bg-alt:var(--rm-ink);
  --surface:var(--rm-surface);
  --surface-2:var(--rm-surface-2);
  --surface-3:var(--rm-surface-2);
  --text:var(--rm-cream);
  --muted:var(--rm-sub);
  --line:var(--rm-line);
  --line-strong:var(--rm-line-strong);
  --accent:var(--rm-chrome-bottom);
  --accent-strong:var(--rm-chrome-top);
  --accent-soft:var(--rm-surface-2);
  --teal:var(--rm-green);
  --ok:var(--rm-green);
  --warn:var(--rm-yellow);
  --danger:var(--rm-red);
}

/* Reset: legacy rules no longer paint. Every element inherits the brand
   text color, and gradient-clipped or transparent text is made solid. */
html,body{
  background:var(--rm-ink);
  color:var(--rm-cream);
  font-family:var(--rm-font-body);
}
body::before,body::after{display:none}
body *{
  color:inherit;
  -webkit-text-fill-color:currentColor;
  text-shadow:none;
  background-color:transparent;
  background-image:none;
  border-color:var(--rm-line);
  border-image:none;
  box-shadow:none;
}
body *::before,body *::after{
  color:inherit;
  -webkit-text-fill-color:currentColor;
  text-shadow:none;
  background-image:none;
  border-color:var(--rm-line);
  border-image:none;
  box-shadow:none;
}

/* Type */
h1,h2,h3,h4,.wolfbbs-page-hero h1,body > h1{
  font-family:var(--rm-font-display);
  font-weight:400;
  letter-spacing:0;
  text-transform:none;
}
h1,body > h1{color:var(--rm-chrome-top)}
h2,h3,h4{color:var(--rm-cream)}
h2{border-bottom:1px solid var(--rm-copper);padding-bottom:.25em}
p,li,dd,dt,td,label,summary,legend,figcaption{color:var(--rm-cream)}
small,.wolfbbs-muted,.muted,caption,figcaption,.wolfbbs-form-status{color:var(--rm-sub)}
code,kbd,samp,pre,tt{font-family:var(--rm-font-code)}
pre,code{background-color:var(--rm-surface-2)}
em,i,cite,dfn{color:var(--rm-magenta);font-style:normal}
strong,b{color:var(--rm-chrome-top)}
mark{background-color:var(--rm-yellow);color:var(--rm-ink);-webkit-text-fill-color:var(--rm-ink)}
hr{border:0;border-top:1px solid var(--rm-copper)}

/* Links */
a{color:var(--rm-chrome-top);text-decoration-color:var(--rm-copper);text-underline-offset:.18em}
a:hover{color:var(--rm-green);text-decoration-color:var(--rm-green)}
a:focus-visible,button:focus-visible,input:focus-visible,select:focus-visible,textarea:focus-visible,summary:focus-visible{
  outline:2px solid var(--rm-green);
  outline-offset:2px;
}

/* Surfaces */
article,form,table,fieldset,details,dialog,
.wolfbbs-card,.wolfbbs-kpi-card,.wolfbbs-action-card,.wolfbbs-helper-card,
.wolfbbs-guide-strip,.wolfbbs-section-nav,p.wolfbbs-nav-row,body > p:has(> a){
  background-color:var(--rm-surface);
  border-color:var(--rm-line);
}
article article,form form,article form,form table,article table,details details{background-color:transparent}
.wolfbbs-guide-strip{border-left-color:var(--rm-copper)}
th{
  font-family:var(--rm-font-display);
  font-weight:400;
  color:var(--rm-sub);
  background-color:var(--rm-ink);
}
tr,td,th{border-color:var(--rm-line)}
tbody tr:hover td{background-color:var(--rm-surface)}

/* Page header: dark band with the chrome strip along its top edge. */
.wolfbbs-page-hero{
  background:var(--rm-chrome) top/100% 4px no-repeat,var(--rm-ink);
  border-color:var(--rm-line-strong);
}

/* Buttons are chrome. */
button,input[type=submit],input[type=button],input[type=reset],.wolfbbs-button,a.button{
  background-image:var(--rm-chrome);
  background-color:var(--rm-chrome-mid);
  color:var(--rm-ink);
  -webkit-text-fill-color:var(--rm-ink);
  border:1px solid var(--rm-chrome-bottom);
  font-family:var(--rm-font-display);
  font-weight:400;
}
button:hover,input[type=submit]:hover,input[type=button]:hover{background-image:none;background-color:var(--rm-chrome-top)}
button:disabled,input:disabled{opacity:.55}

/* Navigation pills: outlined, lit on hover. */
p.wolfbbs-nav-row a,body > p:has(> a) > a,.wolfbbs-section-nav a,.wolfbbs-section-nav summary,
.wolfbbs-breadcrumbs a,nav a,.wolfbbs-chip,.wolfbbs-filter-chip,.wolfbbs-focus-pill,.wolfbbs-dash-chip,.wolfbbs-session-chip{
  color:var(--rm-cream);
  background-color:transparent;
  border-color:var(--rm-line-strong);
  text-decoration:none;
}
p.wolfbbs-nav-row a:hover,body > p:has(> a) > a:hover,.wolfbbs-section-nav a:hover,nav a:hover,.wolfbbs-chip:hover{
  background-color:var(--rm-surface-2);
  border-color:var(--rm-chrome-bottom);
  color:var(--rm-chrome-top);
}

/* Form fields */
input,select,textarea{
  background-color:var(--rm-ink);
  color:var(--rm-cream);
  border-color:var(--rm-copper);
  font-family:var(--rm-font-body);
}
input::placeholder,textarea::placeholder{color:var(--rm-pagenum);-webkit-text-fill-color:var(--rm-pagenum)}
input[type=checkbox],input[type=radio],input[type=range]{accent-color:var(--rm-green)}

/* Status: the accents carry meaning. */
.wolfbbs-hero-chip,.wolfbbs-status-pill,.wolfbbs-chat-status-pill,.wolfbbs-score-pill,.wolfbbs-incident-badge,.wolfbbs-room-badge{
  font-family:var(--rm-font-display);
  font-weight:400;
  color:var(--rm-sub);
  border:1px solid var(--rm-line-strong);
  background-color:var(--rm-ink);
}
.wolfbbs-status-pill.ok{color:var(--rm-green);border-color:var(--rm-green)}
.wolfbbs-status-pill.warn{color:var(--rm-yellow);border-color:var(--rm-yellow)}
.wolfbbs-status-pill.danger{color:var(--rm-red);border-color:var(--rm-red)}
.wolfbbs-flash-error,.error,[role=alert]{color:var(--rm-red)}
.wolfbbs-flash-notice,.notice,[role=status]{color:var(--rm-green)}
.wolfbbs-callout{
  background-color:var(--rm-surface);
  border:1px solid var(--rm-yellow);
  border-left-width:4px;
}

/* Decorative spatial preview on the start page. */
.wolfbbs-spatial-terminal-face,.wolfbbs-spatial-screen-row{
  background-color:var(--rm-surface);
  border-color:var(--rm-line-strong);
}
.wolfbbs-spatial-stage,.wolfbbs-spatial-shell{background-color:var(--rm-ink)}
.wolfbbs-webgl-canvas{display:none}

/* Overlays and floating controls need solid ground under them. */
[id^="wolfbbs"][id$="Overlay"]{background-color:rgba(23,24,25,.82)}
[id^="wolfbbs"][id$="Overlay"] > *{background-color:var(--rm-ink);border:1px solid var(--rm-line-strong)}
#wolfbbsPaletteHeader,#wolfbbsActionDock,#wolfbbsToastRegion > *,.wolfbbs-form-actions-sticky,
.wolfbbs-table-wrap.wolfbbs-sticky-head table thead th,table.wolfbbs-ux20-freeze-col td:first-child{
  background-color:var(--rm-ink);
}
[id^="wolfbbs"][id$="Button"]:not(button),#wolfbbsBackToTop{
  background-color:var(--rm-ink);
  border:1px solid var(--rm-line-strong);
}
#wolfbbsScrollProgress{background-color:var(--rm-copper)}
.wolfbbs-skip-link{background-color:var(--rm-chrome-top);color:var(--rm-ink);-webkit-text-fill-color:var(--rm-ink)}
::selection{background-color:var(--rm-copper);color:var(--rm-chrome-top)}
</style>`
