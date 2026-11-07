(function () {
  "use strict";

  var THEME_KEY = "theme";
  var TOKEN_KEY = "token";
  var EMAIL_KEY = "email";

  var root = document.documentElement;
  var session = readSession();
  var currentView = "";
  var pendingAuthMessage = "";
  var authMode = "login";
  var homeCode = "";
  var homeShortURL = "";
  var statsCode = "";
  var socket = null;
  var elements = {};

  var savedTheme = null;
  try {
    savedTheme = localStorage.getItem(THEME_KEY);
  } catch (error) {
    savedTheme = null;
  }
  if (savedTheme !== "light" && savedTheme !== "dark") {
    savedTheme =
      window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
  }
  root.dataset.theme = savedTheme;

  document.addEventListener("DOMContentLoaded", init);

  function init() {
    elements = {
      views: {
        home: byId("view-home"),
        stats: byId("view-stats"),
        account: byId("view-account"),
        links: byId("view-links"),
      },
      headerLogin: byId("header-login"),
      headerLinks: byId("header-links"),
      headerShorten: byId("header-shorten"),
      backHome: byId("back-home"),
      signOut: byId("sign-out"),
      themeToggle: byId("theme-toggle"),
      shortenForm: byId("shorten-form"),
      shortenInput: byId("long-url"),
      shortenSubmit: byId("shorten-submit"),
      shortenStatus: byId("shorten-status"),
      result: byId("shorten-result"),
      shortLink: byId("short-link"),
      shortQr: byId("short-qr"),
      copyButton: byId("copy-button"),
      statsButton: byId("stats-button"),
      copyStatus: byId("copy-status"),
      statsLink: byId("stats-link"),
      statsCopy: byId("stats-copy"),
      statsUrl: byId("stats-url"),
      statsClicks: byId("stats-clicks"),
      statsCreated: byId("stats-created"),
      statsQr: byId("stats-qr"),
      statsStatus: byId("stats-status"),
      loginTab: byId("login-tab"),
      registerTab: byId("register-tab"),
      authForm: byId("auth-form"),
      authEmail: byId("auth-email"),
      authPassword: byId("auth-password"),
      authHint: byId("auth-hint"),
      authSubmit: byId("auth-submit"),
      authSubmitLabel: byId("auth-submit-label"),
      authStatus: byId("auth-status"),
      linksWho: byId("links-who"),
      linksForm: byId("links-form"),
      linksInput: byId("links-url"),
      linksSubmit: byId("links-submit"),
      linksFormStatus: byId("links-form-status"),
      linksList: byId("links-list"),
      linksEmpty: byId("links-empty"),
      linksStatus: byId("links-status"),
    };

    elements.shortenForm.addEventListener("submit", onHomeShorten);
    elements.copyButton.addEventListener("click", onCopyHomeLink);
    elements.statsButton.addEventListener("click", function () {
      if (homeCode) window.location.hash = "#/stats/" + encodeURIComponent(homeCode);
    });

    elements.statsCopy.addEventListener("click", onCopyStatsLink);

    elements.loginTab.addEventListener("click", function () {
      setAuthMode("login");
    });
    elements.registerTab.addEventListener("click", function () {
      setAuthMode("register");
    });
    elements.authForm.addEventListener("submit", onAuthSubmit);

    elements.linksForm.addEventListener("submit", onLinksShorten);
    elements.signOut.addEventListener("click", signOut);
    elements.themeToggle.addEventListener("click", toggleTheme);

    applyTheme(root.dataset.theme);
    setAuthMode("login");
    setStatus(elements.shortenStatus, "Ready when you are.", false);

    if (window.matchMedia) {
      var media = window.matchMedia("(prefers-color-scheme: dark)");
      if (media.addEventListener) {
        media.addEventListener("change", function (event) {
          var stored = null;
          try {
            stored = localStorage.getItem(THEME_KEY);
          } catch (error) {
            stored = null;
          }
          if (!stored) applyTheme(event.matches ? "dark" : "light");
        });
      }
    }

    window.addEventListener("hashchange", route);
    route();
  }

  /* Theme */

  function toggleTheme() {
    applyTheme(root.dataset.theme === "dark" ? "light" : "dark");
  }

  function applyTheme(theme) {
    root.dataset.theme = theme;
    try {
      localStorage.setItem(THEME_KEY, theme);
    } catch (error) {
      /* storage disabled: theme still applies for this page */
    }
    if (elements.themeToggle) {
      elements.themeToggle.setAttribute(
        "aria-label",
        theme === "dark" ? "Switch to light theme" : "Switch to dark theme"
      );
    }
  }

  /* Routing */

  function route() {
    var raw = window.location.hash.replace(/^#/, "");
    var parts = raw.split("/").filter(Boolean);

    if (parts[0] === "stats" && parts[1]) {
      showStats(decodeURIComponent(parts[1]));
      return;
    }

    if (parts[0] === "account") {
      if (session) {
        window.location.hash = "#/links";
        return;
      }
      showView("account");
      if (authMode !== "login") setAuthMode("login");
      setStatus(elements.authStatus, "", false);
      if (pendingAuthMessage) {
        setStatus(elements.authStatus, pendingAuthMessage, true);
        pendingAuthMessage = "";
      }
      return;
    }

    if (parts[0] === "links") {
      if (!session) {
        pendingAuthMessage = "Log in to see your links.";
        window.location.hash = "#/account";
        return;
      }
      showView("links");
      loadLinks();
      return;
    }

    showView("home");
  }

  function showView(name) {
    currentView = name;
    Object.keys(elements.views).forEach(function (key) {
      elements.views[key].hidden = key !== name;
    });
    if (name !== "stats") {
      closeLive();
      statsCode = "";
    }
    updateHeader();
    document.title =
      name === "home"
        ? "URL shortener"
        : name === "stats"
          ? "Link stats · URL shortener"
          : name === "account"
            ? "Your account · URL shortener"
            : "Your links · URL shortener";
    window.scrollTo(0, 0);
  }

  function updateHeader() {
    elements.headerLogin.hidden = !(
      (currentView === "home" || currentView === "stats") &&
      !session
    );
    elements.headerLinks.hidden = !(currentView === "home" && !!session);
    elements.headerShorten.hidden = currentView !== "account";
    var backToLinks = currentView === "stats" && !!session;
    elements.backHome.hidden =
      currentView !== "links" && currentView !== "stats";
    elements.backHome.href = backToLinks ? "#/links" : "#/";
    elements.backHome.setAttribute(
      "aria-label",
      backToLinks ? "Back to your links" : "Back to home"
    );
    elements.signOut.hidden = !(
      currentView === "links" ||
      (currentView === "stats" && !!session)
    );
  }

  /* Home shortening */

  async function onHomeShorten(event) {
    event.preventDefault();
    var longURL = normalizeURL(elements.shortenInput.value);
    if (!longURL) {
      setStatus(elements.shortenStatus, "Enter a valid http or https URL.", true);
      return;
    }

    setFormLoading(elements.shortenForm, elements.shortenSubmit, true);
    setStatus(elements.shortenStatus, "Shortening…", false);

    try {
      var data = await shortenLink(longURL);
      var code = data.short_code || codeFromShortURL(data.short_url);
      if (!code) {
        setStatus(elements.shortenStatus, "The server returned an unexpected response.", true);
        return;
      }
      homeCode = code;
      homeShortURL = data.short_url || absoluteLink(code);
      elements.shortLink.textContent = homeShortURL;
      elements.shortLink.href = homeShortURL;
      elements.shortQr.src = "/api/qr/" + encodeURIComponent(code);
      elements.shortQr.alt = "QR code for " + code;
      elements.result.hidden = false;
      setStatus(elements.shortenStatus, "", false);
      setStatus(elements.copyStatus, "", false);
      updateHeader();
    } catch (error) {
      setStatus(elements.shortenStatus, error.message, true);
    } finally {
      setFormLoading(elements.shortenForm, elements.shortenSubmit, false);
    }
  }

  async function shortenLink(longURL) {
    var token = session ? session.token : "";
    var headers = { "Content-Type": "application/json" };
    if (token) headers.Authorization = "Bearer " + token;

    try {
      return await request("/api/shorten", {
        method: "POST",
        headers: headers,
        body: JSON.stringify({ long_url: longURL }),
      });
    } catch (error) {
      if (error.status !== 401 || !token) throw error;
      clearSession();
      updateHeader();
      return request("/api/shorten", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ long_url: longURL }),
      });
    }
  }

  async function onCopyHomeLink() {
    if (!homeCode) return;
    var ok = await copyText(homeShortURL || absoluteLink(homeCode));
    flashLabel(elements.copyButton, ok ? "Copied" : "Copy failed");
    setStatus(
      elements.copyStatus,
      ok ? "Copied." : "Copy failed. Select the link and copy it manually.",
      !ok
    );
  }

  /* Stats page */

  function showStats(code) {
    statsCode = code;
    showView("stats");

    elements.statsLink.textContent = shortLinkLabel(code);
    elements.statsLink.href = "/" + encodeURIComponent(code);
    elements.statsQr.src = "/api/qr/" + encodeURIComponent(code);
    elements.statsQr.alt = "QR code for " + code;
    elements.statsUrl.textContent = "—";
    elements.statsUrl.removeAttribute("href");
    elements.statsUrl.removeAttribute("target");
    elements.statsUrl.removeAttribute("rel");
    elements.statsClicks.textContent = "—";
    elements.statsCreated.textContent = "—";
    setStatus(elements.statsStatus, "Loading…", false);

    request("/api/stats/" + encodeURIComponent(code))
      .then(function (data) {
        if (statsCode !== code || currentView !== "stats") return;
        elements.statsUrl.textContent = displayURL(data.long_url);
        elements.statsUrl.href = data.long_url;
        elements.statsUrl.target = "_blank";
        elements.statsUrl.rel = "noopener noreferrer";
        elements.statsClicks.textContent = String(data.clicks);
        elements.statsCreated.textContent = formatDate(data.created_at);
        setStatus(elements.statsStatus, "", false);
        connectLive(code);
      })
      .catch(function (error) {
        if (statsCode !== code || currentView !== "stats") return;
        setStatus(
          elements.statsStatus,
          error.status === 404 ? "No link found for that code." : error.message,
          true
        );
      });
  }

  async function onCopyStatsLink() {
    if (!statsCode) return;
    var ok = await copyText(absoluteLink(statsCode));
    flashLabel(elements.statsCopy, ok ? "Copied" : "Copy failed");
    if (!ok) {
      setStatus(elements.statsStatus, "Copy failed. Select the link and copy it manually.", true);
    }
  }

  function connectLive(code) {
    closeLive();
    if (!("WebSocket" in window)) return;

    var protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    try {
      socket = new WebSocket(
        protocol + "//" + window.location.host + "/api/ws/" + encodeURIComponent(code)
      );
    } catch (error) {
      socket = null;
      return;
    }

    socket.addEventListener("message", function (event) {
      if (statsCode !== code || currentView !== "stats") return;
      var data;
      try {
        data = JSON.parse(event.data);
      } catch (error) {
        return;
      }
      if (!data || data.short_code !== code) return;
      var clicks = Number(data.clicks);
      if (!Number.isFinite(clicks)) return;
      var current = Number(elements.statsClicks.textContent);
      if (!Number.isFinite(current) || clicks >= current) {
        elements.statsClicks.textContent = String(clicks);
      }
    });

    socket.addEventListener("error", function () {
      closeLive();
    });
  }

  function closeLive() {
    if (!socket) return;
    var closing = socket;
    socket = null;
    closing.close();
  }

  /* Account */

  function setAuthMode(mode) {
    authMode = mode;
    var isLogin = mode === "login";
    elements.loginTab.classList.toggle("is-on", isLogin);
    elements.loginTab.setAttribute("aria-pressed", String(isLogin));
    elements.registerTab.classList.toggle("is-on", !isLogin);
    elements.registerTab.setAttribute("aria-pressed", String(!isLogin));
    elements.authSubmitLabel.textContent = isLogin ? "Log in" : "Create account";
    elements.authHint.hidden = isLogin;
    elements.authPassword.autocomplete = isLogin ? "current-password" : "new-password";
    setStatus(elements.authStatus, "", false);
  }

  async function onAuthSubmit(event) {
    event.preventDefault();
    var email = elements.authEmail.value.trim();
    var password = elements.authPassword.value;

    if (!email || !password) {
      setStatus(elements.authStatus, "Enter your email and password.", true);
      return;
    }
    if (authMode === "register" && password.length < 8) {
      setStatus(elements.authStatus, "Use at least 8 characters.", true);
      return;
    }

    setFormLoading(elements.authForm, elements.authSubmit, true);
    setStatus(
      elements.authStatus,
      authMode === "login" ? "Signing in…" : "Creating account…",
      false
    );

    try {
      if (authMode === "register") {
        await request("/api/register", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email: email, password: password }),
        });
        setStatus(elements.authStatus, "Account created. Signing in…", false);
      }

      var data = await request("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: email, password: password }),
      });

      saveSession(data.token, email);
      elements.authForm.reset();
      pendingAuthMessage = "";
      updateHeader();
      window.location.hash = "#/links";
    } catch (error) {
      var message = error.message;
      if (error.status === 401) message = "That email and password do not match.";
      if (error.status === 409) message = "That email is already registered. Log in instead.";
      setStatus(elements.authStatus, message, true);
    } finally {
      setFormLoading(elements.authForm, elements.authSubmit, false);
    }
  }

  function signOut() {
    clearSession();
    if (currentView === "links") {
      window.location.hash = "#/";
      return;
    }
    updateHeader();
  }

  function expireSession(message) {
    clearSession();
    pendingAuthMessage = message || "";
    if (currentView === "links") {
      window.location.hash = "#/account";
    } else {
      updateHeader();
    }
  }

  /* Dashboard */

  async function onLinksShorten(event) {
    event.preventDefault();
    var longURL = normalizeURL(elements.linksInput.value);
    if (!longURL) {
      setStatus(elements.linksFormStatus, "Enter a valid http or https URL.", true);
      return;
    }

    setFormLoading(elements.linksForm, elements.linksSubmit, true);
    setStatus(elements.linksFormStatus, "Shortening…", false);

    try {
      await shortenLink(longURL);
      if (!session) {
        setStatus(elements.linksFormStatus, "Your session expired. Log in again.", true);
        expireSession("Your session expired. Log in again.");
        return;
      }
      elements.linksInput.value = "";
      setStatus(elements.linksFormStatus, "Short link ready. It is first in the list below.", false);
      loadLinks();
    } catch (error) {
      setStatus(elements.linksFormStatus, error.message, true);
    } finally {
      setFormLoading(elements.linksForm, elements.linksSubmit, false);
    }
  }

  async function loadLinks() {
    if (!session) return;
    setStatus(elements.linksStatus, "Loading…", false);

    try {
      var data = await request("/api/links", {
        headers: { Authorization: "Bearer " + session.token },
      });
      if (currentView !== "links") return;
      renderLinks(data.links || []);
      setStatus(elements.linksStatus, "", false);
    } catch (error) {
      if (error.status === 401) {
        expireSession("Your session expired. Log in again.");
        return;
      }
      setStatus(elements.linksStatus, error.message, true);
    }
  }

  function renderLinks(links) {
    elements.linksList.textContent = "";
    elements.linksList.hidden = links.length === 0;
    elements.linksEmpty.hidden = links.length > 0;

    var who = session ? "Signed in as " + session.email : "Signed in.";
    elements.linksWho.textContent =
      who + " · " + links.length + (links.length === 1 ? " link." : " links.");

    links.forEach(function (link) {
      var code = link.short_code;
      var item = document.createElement("li");
      item.className = "link-row-item";

      var main = document.createElement("div");
      main.className = "link-main";

      var shortLink = document.createElement("a");
      shortLink.href = "/" + encodeURIComponent(code);
      shortLink.target = "_blank";
      shortLink.rel = "noopener noreferrer";
      shortLink.textContent = shortLinkLabel(code);

      var destination = document.createElement("p");
      destination.className = "link-destination";
      destination.textContent = displayURL(link.long_url);

      var meta = document.createElement("p");
      meta.className = "link-meta";
      meta.textContent =
        clickLabel(link.clicks) + " · " + formatDate(link.created_at);

      main.append(shortLink, destination, meta);

      var actions = document.createElement("div");
      actions.className = "link-actions";

      var stats = document.createElement("button");
      stats.type = "button";
      stats.className = "ghost";
      stats.textContent = "Stats";
      stats.addEventListener("click", function () {
        window.location.hash = "#/stats/" + encodeURIComponent(code);
      });

      var copy = document.createElement("button");
      copy.type = "button";
      copy.className = "ghost";
      copy.textContent = "Copy";
      copy.addEventListener("click", async function () {
        var ok = await copyText(absoluteLink(code));
        flashLabel(copy, ok ? "Copied" : "Copy failed");
        if (!ok) {
          setStatus(elements.linksStatus, "Copy failed. Select the link and copy it manually.", true);
        }
      });

      actions.append(stats, copy);
      item.append(main, actions);
      elements.linksList.append(item);
    });
  }

  /* Helpers */

  async function request(url, options) {
    var response;
    try {
      response = await fetch(url, options || {});
    } catch (error) {
      throw httpError("Can't reach the server. Check your connection and try again.", 0);
    }

    var text = await response.text();
    var body = null;
    if (text) {
      try {
        body = JSON.parse(text);
      } catch (error) {
        body = null;
      }
      if (!body && text.length < 200) {
        body = { error: text.trim() };
      }
    }

    if (!response.ok) {
      throw httpError(messageForStatus(response.status, body), response.status);
    }
    return body;
  }

  function messageForStatus(status, body) {
    if (status === 401) {
      return (body && body.error && sentenceCase(body.error)) || "Your session expired. Log in again.";
    }
    if (status === 429) return "Too many attempts. Wait a minute and try again.";
    if (status === 502) return "Account service is unavailable. Try again shortly.";
    if (body && typeof body.error === "string" && body.error && status < 500) {
      return sentenceCase(body.error);
    }
    if (status === 404) return "No link found for that code.";
    if (status >= 500) return "Server error. Try again shortly.";
    if (status === 400) return "That input was not accepted.";
    return "Request failed (" + status + ").";
  }

  function httpError(message, status) {
    var error = new Error(message);
    error.status = status;
    return error;
  }

  function normalizeURL(raw) {
    var value = (raw || "").trim();
    if (!value) return "";

    if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(value)) {
      var local = /^(localhost|127\.\d+\.\d+\.\d+|0\.0\.0\.0|\[::1\])(:\d+)?(\/|$)/i.test(value);
      value = (local ? "http://" : "https://") + value;
    }

    try {
      var parsed = new URL(value);
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return "";
      var host = parsed.hostname;
      if (!host.includes(".") && host !== "localhost" && !host.startsWith("[") && host !== "::1") {
        return "";
      }
      return parsed.toString();
    } catch (error) {
      return "";
    }
  }

  function shortLinkLabel(code) {
    return window.location.host + "/" + code;
  }

  function displayURL(value) {
    return (value || "").replace(/^https?:\/\//i, "");
  }

  function codeFromShortURL(value) {
    if (!value) return "";
    try {
      var path = new URL(value, window.location.origin).pathname;
      var parts = path.split("/").filter(Boolean);
      return parts.length ? decodeURIComponent(parts[parts.length - 1]) : "";
    } catch (error) {
      return "";
    }
  }

  function absoluteLink(code) {
    return window.location.origin + "/" + code;
  }

  function clickLabel(clicks) {
    return clicks === 1 ? "1 click" : clicks + " clicks";
  }

  function formatDate(value) {
    var date = new Date(value);
    if (isNaN(date.getTime())) return "—";
    return new Intl.DateTimeFormat(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }).format(date);
  }

  function sentenceCase(text) {
    return text.charAt(0).toUpperCase() + text.slice(1);
  }

  function setStatus(element, message, isError) {
    element.textContent = message;
    element.classList.toggle("error", Boolean(isError));
  }

  function setFormLoading(form, submit, loading) {
    form.classList.toggle("is-loading", loading);
    submit.disabled = loading;
  }

  function flashLabel(button, text) {
    if (button.dataset.original === undefined) {
      button.dataset.original = button.textContent;
    }
    button.textContent = text;
    clearTimeout(button.flashTimer);
    button.flashTimer = setTimeout(function () {
      button.textContent = button.dataset.original;
      delete button.dataset.original;
    }, 1500);
  }

  async function copyText(text) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (error) {
      var area = document.createElement("textarea");
      area.value = text;
      area.setAttribute("readonly", "");
      area.style.position = "fixed";
      area.style.opacity = "0";
      document.body.append(area);
      area.select();
      var ok = false;
      try {
        ok = document.execCommand("copy");
      } catch (copyError) {
        ok = false;
      }
      area.remove();
      return ok;
    }
  }

  function saveSession(token, email) {
    session = { token: token, email: email };
    try {
      localStorage.setItem(TOKEN_KEY, token);
      localStorage.setItem(EMAIL_KEY, email);
    } catch (error) {
      /* storage disabled: session lives for this page only */
    }
  }

  function clearSession() {
    session = null;
    try {
      localStorage.removeItem(TOKEN_KEY);
      localStorage.removeItem(EMAIL_KEY);
    } catch (error) {
      /* storage disabled */
    }
  }

  function readSession() {
    try {
      var token = localStorage.getItem(TOKEN_KEY);
      var email = localStorage.getItem(EMAIL_KEY);
      if (token && email) return { token: token, email: email };
    } catch (error) {
      /* storage disabled */
    }
    return null;
  }

  function byId(id) {
    return document.getElementById(id);
  }
})();
