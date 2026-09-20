// Brings the saved branding onto OpenCloud's sign-in page, whose React app
// takes only the logo from the theme. React renders the page late and swaps
// screens, so every change to the page is answered by applying it again.
(() => {
  // React owns its footer and would trip over a removed node, so it is hidden
  // and a footer of our own follows it.
  const applyFooter = (b, stock) => {
    stock.hidden = true
    let own = stock.nextElementSibling
    if (!own || !own.hasAttribute('data-branding')) {
      own = document.createElement('footer')
      own.className = 'oc-footer-message'
      own.setAttribute('data-branding', '')
      stock.after(own)
    }
    // Rewriting the footer is itself a change to the page, so it only happens
    // when the text differs.
    if (own.textContent !== (b.name || 'OpenCloud') + (b.slogan ? ` - ${b.slogan}` : '')) {
      const name = document.createElement('strong')
      name.textContent = b.name || 'OpenCloud'
      own.replaceChildren(name)
      if (b.slogan) {
        own.append(` - ${b.slogan}`)
      }
    }
  }

  // The sign-in page is light only, so a dark card is a sheet of our own in
  // the colours the web UI uses for its dark theme. It goes last in the head,
  // after the page's own stylesheet.
  const darkCard = `
.oc-card { background: #1d2021; color: #e1e3e4; color-scheme: dark; }
.oc-card a { color: #5cd5fb; }
.oc-card .oc-input { background: #111415; border-color: #8a9296; color: #e1e3e4; }
.oc-card .oc-input::placeholder { color: #bfc8cc; }
.oc-card .oc-input:focus { border-color: #5cd5fb; }
.oc-card .oc-input.error { border-color: #ffb4ab; }
.oc-card .oc-input:-webkit-autofill { -webkit-text-fill-color: #e1e3e4; box-shadow: 0 0 0 1000px #111415 inset; caret-color: #e1e3e4; }
.oc-card .MuiTypography-colorError { color: #ffb4ab !important; }
.oc-card .oc-button-secondary { background: #323537 !important; color: #e1e3e4 !important; }
`

  const applyCard = (mode) => {
    if (mode !== 'dark' && mode !== 'auto') {
      return
    }
    const sheet = document.createElement('style')
    sheet.setAttribute('data-branding', '')
    sheet.textContent = mode === 'auto' ? `@media (prefers-color-scheme: dark) {${darkCard}}` : darkCard
    document.head.append(sheet)
  }

  const apply = (b) => {
    if (b.name) {
      document.title = `Sign in - ${b.name}`
    }
    if (b.name || b.slogan) {
      document.querySelectorAll('footer.oc-footer-message:not([data-branding])').forEach((stock) => applyFooter(b, stock))
    }
    if (b.background) {
      const bg = document.querySelector('.oc-login-bg')
      if (bg) {
        bg.style.backgroundImage = `url("${b.background}")`
        bg.classList.add('oc-login-bg-image')
      }
      const artwork = document.querySelector('img.oc-login-bg-icon')
      if (artwork) {
        artwork.hidden = true
      }
    }
    // Setting the same href again can make the browser fetch the icon again.
    const icon = b.favicon && document.querySelector('link[rel="icon"]')
    if (icon && icon.getAttribute('href') !== b.favicon) {
      icon.setAttribute('href', b.favicon)
      icon.removeAttribute('type')
    }
  }

  fetch('/brandingsvc/login.json', { cache: 'no-store' })
    .then((response) => (response.ok ? response.json() : null))
    .then((b) => {
      if (!b) {
        return
      }
      applyCard(b.theme)
      if (!(b.name || b.slogan || b.background || b.favicon)) {
        return
      }
      apply(b)
      new MutationObserver(() => apply(b)).observe(document.body, { childList: true, subtree: true })
    })
    .catch(() => {})
})()
