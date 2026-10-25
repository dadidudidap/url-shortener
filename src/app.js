(function () {
  "use strict";

  var root = document.documentElement;

  // Resolve the theme before the body paints. The script is loaded in <head>
  // and init() waits for the DOM, so this avoids a flash of the wrong theme.
  var saved = null;
  try { saved = localStorage.getItem("theme"); } catch (error) {}
  var initialTheme = saved === "light" || saved === "dark"
    ? saved
    : (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  root.dataset.theme = initialTheme;

  function init() {
    var themeButton = document.getElementById("theme-toggle");
    var shortenForm = document.getElementById("shorten-form");
    var longUrlInput = document.getElementById("long-url");
    var shortenSubmit = document.getElementById("shorten-submit");
    var shortenStatus = document.getElementById("shorten-status");
    var shortenResult = document.getElementById("shorten-result");
    var shortLink = document.getElementById("short-link");
    var copyButton = document.getElementById("copy-button");
    var copyStatus = document.getElementById("copy-status");
    var shortQR = document.getElementById("short-qr");
    var statsForm = document.getElementById("stats-form");
    var statsCodeInput = document.getElementById("stats-code");
    var statsSubmit = document.getElementById("stats-submit");
    var statsStatus = document.getElementById("stats-status");
    var statsEmpty = document.getElementById("stats-empty");
    var statsResult = document.getElementById("stats-result");
    var statsUrl = document.getElementById("stats-url");
    var statsClicks = document.getElementById("stats-clicks");
    var statsCreated = document.getElementById("stats-created");
    var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

    var copyTimer = null;
    var statsRequest = 0;
    var displayedCode = "";
    var liveSocket = null;
    var liveCode = "";
    var liveRetry = null;

    function applyTheme(theme) {
      root.dataset.theme = theme;
      themeButton.textContent = theme === "dark" ? "Light" : "Dark";
      themeButton.setAttribute("aria-label", theme === "dark" ? "Switch to light theme" : "Switch to dark theme");
    }

    themeButton.addEventListener("click", function () {
      var next = root.dataset.theme === "dark" ? "light" : "dark";
      applyTheme(next);
      try { localStorage.setItem("theme", next); } catch (error) {}
    });

    function setStatus(element, text, isError) {
      element.textContent = text;
      element.classList.toggle("error", Boolean(isError));
    }

    function setBusy(button, busy) {
      button.disabled = busy;
      button.setAttribute("aria-busy", busy ? "true" : "false");
    }

    function isHttpURL(value) {
      try {
        var parsed = new URL(value);
        return parsed.protocol === "http:" || parsed.protocol === "https:";
      } catch (error) {
        return false;
      }
    }

    function normalizeURL(value) {
      var raw = value.trim();
      if (!raw) return "";
      var candidate = raw.indexOf("://") === -1 ? "https://" + raw : raw;
      if (candidate.length > 2048) return "";
      try {
        var parsed = new URL(candidate);
        if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return "";
        return parsed.href;
      } catch (error) {
        return "";
      }
    }

    function shortCodeFrom(value) {
      var raw = value.trim();
      if (!raw) return "";
      try {
        var parts = new URL(raw).pathname.split("/").filter(Boolean);
        if (parts.length) return parts[parts.length - 1];
      } catch (error) {}
      return raw.replace(/^\/+|\/+$/g, "");
    }

    function formatDate(value) {
      var date = new Date(value);
      if (Number.isNaN(date.getTime())) return value;
      return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
    }

    async function request(url, options) {
      var response = await fetch(url, options);
      if (!response.ok) {
        var error = new Error("Request failed with " + response.status);
        error.status = response.status;
        throw error;
      }
      return response.json();
    }

    function openReveal(element) {
      if (!element.hidden && element.classList.contains("is-open")) return;
      element.hidden = false;
      requestAnimationFrame(function () {
        requestAnimationFrame(function () {
          element.classList.add("is-open");
        });
      });
    }

    function closeReveal(element) {
      if (element.hidden) return;
      element.classList.remove("is-open");
      var delay = reduceMotion.matches ? 0 : 300;
      window.setTimeout(function () {
        if (!element.classList.contains("is-open")) element.hidden = true;
      }, delay);
    }

    async function copyText(text) {
      if (navigator.clipboard && window.isSecureContext) {
        try {
          await navigator.clipboard.writeText(text);
          return;
        } catch (error) {
          // Permission can be denied even on a secure origin; use the selection fallback.
        }
      }
      var helper = document.createElement("textarea");
      helper.value = text;
      helper.setAttribute("readonly", "");
      helper.style.position = "fixed";
      helper.style.top = "-1000px";
      document.body.appendChild(helper);
      helper.select();
      var copied = document.execCommand("copy");
      document.body.removeChild(helper);
      if (!copied) throw new Error("Copy failed");
    }

    function closeLive() {
      window.clearTimeout(liveRetry);
      liveRetry = null;
      liveCode = "";
      if (liveSocket) {
        liveSocket.onmessage = null;
        liveSocket.onclose = null;
        liveSocket.close();
        liveSocket = null;
      }
    }

    function openLive() {
      var code = liveCode;
      if (!code) return;

      var socket;
      try {
        socket = new WebSocket(liveURL(code));
      } catch (error) {
        liveRetry = window.setTimeout(openLive, 2000);
        return;
      }
      liveSocket = socket;

      socket.onmessage = function (event) {
        if (liveCode !== code) return;
        var data;
        try { data = JSON.parse(event.data); } catch (error) { return; }
        if (!data || data.short_code !== code) return;
        var clicks = Number(data.clicks);
        // A message already in flight can be older than the HTTP snapshot; never count backwards.
        if (!Number.isFinite(clicks) || clicks < Number(statsClicks.textContent || 0)) return;
        statsClicks.textContent = String(clicks);
      };

      socket.onclose = function () {
        if (liveSocket === socket) liveSocket = null;
        if (liveCode !== code || document.hidden) return;
        liveRetry = window.setTimeout(openLive, 2000);
      };
    }

    function connectLive(code) {
      if (!("WebSocket" in window)) return;
      closeLive();
      liveCode = code;
      openLive();
    }

    function liveURL(code) {
      var scheme = window.location.protocol === "https:" ? "wss://" : "ws://";
      return scheme + window.location.host + "/api/ws/" + encodeURIComponent(code);
    }

    async function loadStats(code, quiet) {
      var requestID = ++statsRequest;
      closeLive();
      statsEmpty.hidden = true;
      statsResult.classList.add("is-loading");
      setBusy(statsSubmit, true);
      setStatus(statsStatus, quiet ? "" : "Checking clicks\u2026", false);

      try {
        var stats = await request("/api/stats/" + encodeURIComponent(code));
        if (requestID !== statsRequest) return;

        statsUrl.textContent = stats.long_url;
        if (isHttpURL(stats.long_url)) {
          statsUrl.href = stats.long_url;
        } else {
          statsUrl.removeAttribute("href");
        }
        statsClicks.textContent = String(stats.clicks);
        statsCreated.textContent = formatDate(stats.created_at);
        statsResult.classList.remove("is-loading");
        openReveal(statsResult);
        setStatus(statsStatus, "", false);
        displayedCode = code;
        connectLive(code);
      } catch (error) {
        if (requestID !== statsRequest) return;
        displayedCode = "";
        statsResult.classList.remove("is-loading");
        closeReveal(statsResult);
        if (error.status === 404) {
          setStatus(statsStatus, "No link found for that code.", true);
        } else {
          setStatus(statsStatus, "Stats could not be loaded. Try again.", true);
        }
      } finally {
        if (requestID === statsRequest) setBusy(statsSubmit, false);
      }
    }

    shortenForm.addEventListener("submit", async function (event) {
      event.preventDefault();
      var longURL = normalizeURL(longUrlInput.value);

      if (!longURL) {
        setStatus(shortenStatus, "Enter a web address, for example example.com/page.", true);
        longUrlInput.focus();
        return;
      }

      setStatus(shortenStatus, "Shortening link\u2026", false);
      setBusy(shortenSubmit, true);
      try {
        var data = await request("/api/shorten", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ long_url: longURL })
        });
        var code = shortCodeFrom(data.short_url);

        shortLink.href = data.short_url;
        shortLink.textContent = data.short_url;
        copyButton.textContent = "Copy";
        setStatus(copyStatus, "", false);
        openReveal(shortenResult);
        setStatus(shortenStatus, "Short link ready.", false);

        if (code) {
          shortQR.src = "/api/qr/" + encodeURIComponent(code);
          shortQR.alt = "QR code for " + data.short_url;
          statsCodeInput.value = code;
          loadStats(code, true);
        }
      } catch (error) {
        if (error.status === 429) {
          setStatus(shortenStatus, "Too many links in a minute. Wait a minute, then try again.", true);
        } else if (error.status === 400) {
          setStatus(shortenStatus, "That address was rejected. Check it and try again.", true);
        } else {
          setStatus(shortenStatus, "The link could not be saved. Check your connection and try again.", true);
        }
      } finally {
        setBusy(shortenSubmit, false);
      }
    });

    statsForm.addEventListener("submit", function (event) {
      event.preventDefault();
      var code = shortCodeFrom(statsCodeInput.value);
      if (!code) {
        setStatus(statsStatus, "Enter a short code or link first.", true);
        statsCodeInput.focus();
        return;
      }
      statsCodeInput.value = code;
      loadStats(code, false);
    });

    copyButton.addEventListener("click", async function () {
      try {
        await copyText(shortLink.textContent);
        copyButton.textContent = "Copied";
        setStatus(copyStatus, "Copied to clipboard.", false);
        window.clearTimeout(copyTimer);
        copyTimer = window.setTimeout(function () {
          copyButton.textContent = "Copy";
          setStatus(copyStatus, "", false);
        }, 2000);
      } catch (error) {
        setStatus(copyStatus, "Copy failed. Select the link and copy it manually.", true);
      }
    });

    applyTheme(root.dataset.theme === "dark" ? "dark" : "light");

    document.addEventListener("visibilitychange", function () {
      if (document.hidden) {
        closeLive();
      } else if (displayedCode) {
        connectLive(displayedCode);
      }
    });
    window.addEventListener("pagehide", closeLive);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
