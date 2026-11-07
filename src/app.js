(function () {
  "use strict";

  var THEME_KEY = "theme";
  var TOKEN_KEY = "token";
  var EMAIL_KEY = "email";

  var root = document.documentElement;
  var sessionExpired = false;
  var session = readSession();
  var currentView = "";
  var pendingAuthMessage = "";
  var authMode = "login";
  var homeCode = "";
  var homeShortURL = "";
  var statsCode = "";
  var statsLinkData = null;
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
      shortExpiry: byId("short-expiry"),
      copyButton: byId("copy-button"),
      statsButton: byId("stats-button"),
      copyStatus: byId("copy-status"),
      statsLink: byId("stats-link"),
      statsCopy: byId("stats-copy"),
      statsEdit: byId("stats-edit"),
      statsUrl: byId("stats-url"),
      statsClicks: byId("stats-clicks"),
      statsCreated: byId("stats-created"),
      statsExpires: byId("stats-expires"),
      statsQr: byId("stats-qr"),
      statsStatus: byId("stats-status"),
      editPanel: byId("edit-panel"),
      editForm: byId("edit-form"),
      editCode: byId("edit-code"),
      editExpires: byId("edit-expires"),
      editLifetime: byId("edit-lifetime"),
      editSave: byId("edit-save"),
      editCancel: byId("edit-cancel"),
      editDelete: byId("edit-delete"),
      editStatus: byId("edit-status"),
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
    elements.statsEdit.addEventListener("click", openEdit);
    elements.editForm.addEventListener("submit", onEditSubmit);
    elements.editCancel.addEventListener("click", closeEdit);
    elements.editDelete.addEventListener("click", onEditDelete);
    elements.editLifetime.addEventListener("change", syncLifetime);

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

    if (sessionExpired) {
      pendingAuthMessage = "Session expired. Log in again.";
      sessionExpired = false;
    }

    if (parts[0] === "stats" && parts[1]) {
      if (!session) {
        window.location.hash = "#/";
        return;
      }
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
        pendingAuthMessage = pendingAuthMessage || "Log in to see your links.";
        window.location.hash = "#/account";
        return;
      }
      showView("links");
      loadLinks();
      return;
    }

    if (pendingAuthMessage) {
      setStatus(elements.shortenStatus, pendingAuthMessage, true);
      pendingAuthMessage = "";
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
      elements.statsButton.hidden = !session;
      var expiry = expiryLabel(data.expires_at);
      elements.shortExpiry.textContent = expiry;
      elements.shortExpiry.hidden = !expiry;
      elements.result.hidden = false;
      setStatus(elements.shortenStatus, "", false);
      setStatus(elements.copyStatus, "", false);
      updateHeader();
    } catch (error) {
      if (error.status === 401 && session) {
        expireSession("Session expired. Log in again.");
        setStatus(elements.shortenStatus, "Session expired. Log in again.", true);
        return;
      }
      setStatus(elements.shortenStatus, error.message, true);
    } finally {
      setFormLoading(elements.shortenForm, elements.shortenSubmit, false);
    }
  }

  async function shortenLink(longURL) {
    var headers = { "Content-Type": "application/json" };
    if (session) headers.Authorization = "Bearer " + session.token;

    return request("/api/shorten", {
      method: "POST",
      headers: headers,
      body: JSON.stringify({ long_url: longURL }),
    });
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
    statsLinkData = null;
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
    elements.statsExpires.textContent = "—";
    elements.statsExpires.classList.remove("is-expired");
    elements.statsEdit.hidden = true;
    elements.editPanel.hidden = true;
    setStatus(elements.statsStatus, "Loading…", false);

    authRequest("/api/stats/" + encodeURIComponent(code))
      .then(function (data) {
        if (statsCode !== code || currentView !== "stats") return;
        elements.statsUrl.textContent = displayURL(data.long_url);
        elements.statsUrl.href = data.long_url;
        elements.statsUrl.target = "_blank";
        elements.statsUrl.rel = "noopener noreferrer";
        elements.statsClicks.textContent = String(data.clicks);
        elements.statsCreated.textContent = formatDate(data.created_at);
        elements.statsExpires.textContent = expiryText(data.expires_at, data.expired);
        elements.statsExpires.classList.toggle("is-expired", Boolean(data.expired));
        statsLinkData = data;
        elements.statsEdit.hidden = false;
        setStatus(elements.statsStatus, "", false);
        connectLive(code);
      })
      .catch(function (error) {
        if (statsCode !== code || currentView !== "stats" || !session) return;
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

  function openEdit() {
    if (!statsLinkData) return;

    elements.editCode.value = statsLinkData.short_code || statsCode;
    elements.editExpires.value = statsLinkData.expires_at
      ? toLocalDatetime(statsLinkData.expires_at)
      : "";
    elements.editLifetime.checked = !statsLinkData.expires_at;
    syncLifetime();
    setStatus(elements.editStatus, "", false);
    elements.editPanel.hidden = false;
    elements.editCode.focus();
  }

  function closeEdit() {
    elements.editPanel.hidden = true;
    setStatus(elements.editStatus, "", false);
  }

  function syncLifetime() {
    elements.editExpires.disabled = elements.editLifetime.checked;
  }

  async function onEditSubmit(event) {
    event.preventDefault();
    if (!statsLinkData) return;

    var oldCode = statsCode;
    var code = elements.editCode.value.trim();
    var lifetime = elements.editLifetime.checked;
    var expiresAt = "";

    if (!code) {
      setStatus(elements.editStatus, "Enter a short code.", true);
      return;
    }
    if (!lifetime && elements.editExpires.value) {
      expiresAt = fromLocalDatetime(elements.editExpires.value);
      if (!expiresAt) {
        setStatus(elements.editStatus, "Enter a valid expiry date and time.", true);
        return;
      }
    }

    setFormLoading(elements.editForm, elements.editSave, true);
    setStatus(elements.editStatus, "Saving…", false);

    try {
      var data = await authRequest("/api/links/" + encodeURIComponent(oldCode), {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          short_code: code,
          expires_at: expiresAt,
          lifetime: lifetime,
        }),
      });
      if (currentView !== "stats" || statsCode !== oldCode) return;

      elements.editPanel.hidden = true;
      if (data && data.short_code && data.short_code !== oldCode) {
        window.location.hash = "#/stats/" + encodeURIComponent(data.short_code);
      } else {
        showStats(oldCode);
      }
    } catch (error) {
      if (error.status === 401) return;
      setStatus(
        elements.editStatus,
        error.status === 409 ? "That short code is already taken." : error.message,
        true
      );
    } finally {
      setFormLoading(elements.editForm, elements.editSave, false);
    }
  }

  async function onEditDelete() {
    if (!statsLinkData) return;
    if (!window.confirm("Delete this link permanently?")) return;

    var code = statsCode;
    elements.editDelete.disabled = true;
    setStatus(elements.editStatus, "Deleting…", false);

    try {
      await authRequest("/api/links/" + encodeURIComponent(code), { method: "DELETE" });
      if (currentView !== "stats" || statsCode !== code) return;
      window.location.hash = "#/links";
    } catch (error) {
      if (error.status === 401) return;
      setStatus(elements.editStatus, error.message, true);
    } finally {
      elements.editDelete.disabled = false;
    }
  }

  function connectLive(code) {
    closeLive();
    if (!("WebSocket" in window) || !session) return;

    var protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    try {
      socket = new WebSocket(
        protocol + "//" + window.location.host + "/api/ws/" + encodeURIComponent(code),
        ["bearer", session.token]
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
    if (currentView === "links" || currentView === "stats") {
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
        setStatus(elements.linksFormStatus, "Session expired. Log in again.", true);
        expireSession("Session expired. Log in again.");
        return;
      }
      elements.linksInput.value = "";
      setStatus(elements.linksFormStatus, "Short link ready. It is first in the list below.", false);
      loadLinks();
    } catch (error) {
      if (error.status === 401) {
        setStatus(elements.linksFormStatus, "Session expired. Log in again.", true);
        expireSession("Session expired. Log in again.");
        return;
      }
      setStatus(elements.linksFormStatus, error.message, true);
    } finally {
      setFormLoading(elements.linksForm, elements.linksSubmit, false);
    }
  }

  async function loadLinks() {
    if (!session) return;
    setStatus(elements.linksStatus, "Loading…", false);

    try {
      var data = await authRequest("/api/links");
      if (currentView !== "links") return;
      renderLinks(data.links || []);
      setStatus(elements.linksStatus, "", false);
    } catch (error) {
      if (error.status === 401) return;
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
      meta.append(clickLabel(link.clicks) + " · " + formatDate(link.created_at) + " · ");

      var expiry = document.createElement("span");
      expiry.className = "link-expiry";
      expiry.textContent = expiryText(link.expires_at, link.expired);
      if (link.expired) expiry.classList.add("is-expired");
      meta.append(expiry);

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

  async function authRequest(url, options) {
    if (!session) throw httpError("Session expired. Log in again.", 401);

    var headers = Object.assign({}, (options && options.headers) || {}, {
      Authorization: "Bearer " + session.token,
    });
    var requestOptions = Object.assign({}, options || {}, { headers: headers });

    try {
      return await request(url, requestOptions);
    } catch (error) {
      if (error.status === 401) expireSession("Session expired. Log in again.");
      throw error;
    }
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

  function formatDateTime(value) {
    var date = new Date(value);
    if (isNaN(date.getTime())) return "—";
    return new Intl.DateTimeFormat(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
    }).format(date);
  }

  function expiryText(value, expired) {
    if (!value) return "Lifetime";
    if (expired) return "Expired";
    return "Expires " + formatDateTime(value);
  }

  function toLocalDatetime(value) {
    var date = new Date(value);
    if (isNaN(date.getTime())) return "";
    var local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
    return local.toISOString().slice(0, 16);
  }

  function fromLocalDatetime(value) {
    var date = new Date(value);
    if (isNaN(date.getTime())) return "";
    return date.toISOString();
  }

  function expiryLabel(value) {
    var date = new Date(value);
    if (isNaN(date.getTime())) return "";

    var remaining = date.getTime() - Date.now();
    if (remaining <= 0) return "Expired";
    var days = Math.ceil(remaining / 86400000);
    return days === 1 ? "Expires in 1 day" : "Expires in " + days + " days";
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
      if (!token || !email) return null;

      var expiresAt = tokenExpiry(token);
      if (!expiresAt || expiresAt <= Date.now()) {
        sessionExpired = true;
        localStorage.removeItem(TOKEN_KEY);
        localStorage.removeItem(EMAIL_KEY);
        return null;
      }
      return { token: token, email: email, expiresAt: expiresAt };
    } catch (error) {
      /* storage disabled */
    }
    return null;
  }

  function tokenExpiry(token) {
    try {
      var parts = token.split(".");
      if (parts.length < 2 || !parts[1]) return 0;

      var payload = parts[1].replace(/-/g, "+").replace(/_/g, "/");
      while (payload.length % 4) payload += "=";
      var claims = JSON.parse(atob(payload));
      var exp = Number(claims.exp);
      return Number.isFinite(exp) && exp > 0 ? exp * 1000 : 0;
    } catch (error) {
      return 0;
    }
  }

  function byId(id) {
    return document.getElementById(id);
  }
})();
