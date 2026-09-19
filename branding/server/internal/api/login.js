// Brings the saved branding onto OpenCloud's sign-in page, whose React app
// takes only the logo from the theme. React renders the page late and swaps
// screens, so every change to the page is answered by applying it again.
(() => {
  // React owns its footer and would trip over a removed node, so it is hidden
  // and a footer of our own follows it.
  const applyFooter = (b, stock) => {
    if (!stock.hidden) {
      stock.hidden = true
    }
    let own = stock.nextElementSibling
    if (!own || !own.hasAttribute('data-branding')) {
      own = document.createElement('footer')
      own.className = 'oc-footer-message'
      own.setAttribute('data-branding', '')
      stock.after(own)
    }
    if (own.textContent !== (b.name || 'OpenCloud') + (b.slogan ? ` - ${b.slogan}` : '')) {
      const name = document.createElement('strong')
      name.textContent = b.name || 'OpenCloud'
      own.replaceChildren(name)
      if (b.slogan) {
        own.append(` - ${b.slogan}`)
      }
    }
  }

  // Each step checks first, so applying twice changes nothing and the
  // observer below does not wake itself up.
  const apply = (b) => {
    if (b.name && document.title !== `Sign in - ${b.name}`) {
      document.title = `Sign in - ${b.name}`
    }
    if (b.name || b.slogan) {
      document.querySelectorAll('footer.oc-footer-message:not([data-branding])').forEach((stock) => applyFooter(b, stock))
    }
    if (b.background) {
      const bg = document.querySelector('.oc-login-bg')
      const image = `url("${b.background}")`
      if (bg && bg.style.backgroundImage !== image) {
        bg.style.backgroundImage = image
      }
      if (bg && !bg.classList.contains('oc-login-bg-image')) {
        bg.classList.add('oc-login-bg-image')
      }
      const artwork = document.querySelector('img.oc-login-bg-icon')
      if (artwork && !artwork.hidden) {
        artwork.hidden = true
      }
    }
    const icon = b.favicon && document.querySelector('link[rel="icon"]')
    if (icon && icon.getAttribute('href') !== b.favicon) {
      icon.setAttribute('href', b.favicon)
      icon.removeAttribute('type')
    }
  }

  fetch('/brandingsvc/login.json', { cache: 'no-store' })
    .then((response) => (response.ok ? response.json() : null))
    .then((b) => {
      if (!b || !(b.name || b.slogan || b.background || b.favicon)) {
        return
      }
      apply(b)
      new MutationObserver(() => apply(b)).observe(document.body, { childList: true, subtree: true })
    })
    .catch(() => {})
})()
