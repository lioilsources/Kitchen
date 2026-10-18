// Společný klient pro webové displeje: WebSocket se snapshoty, HTTP akce, titulek hlášek.
"use strict";

const P = {
  params: new URLSearchParams(location.search),

  // connect(onSnapshot, onAnnounce) — automatické znovupřipojení, indikátor spojení, overlay při výpadku.
  connect(onSnapshot, onAnnounce) {
    let ws, lastMsg = 0, backoff = 500;
    const dot = document.querySelector(".dot");
    const overlay = document.querySelector(".offline");
    const open = () => {
      const proto = location.protocol === "https:" ? "wss" : "ws";
      ws = new WebSocket(`${proto}://${location.host}/ws`);
      ws.onopen = () => { backoff = 500; dot && dot.classList.add("on"); };
      ws.onmessage = (ev) => {
        lastMsg = Date.now();
        const m = JSON.parse(ev.data);
        if (m.type === "snapshot") onSnapshot(m.data);
        else if (m.type === "announce" && onAnnounce) onAnnounce(m.data);
      };
      ws.onclose = () => {
        dot && dot.classList.remove("on");
        setTimeout(open, backoff);
        backoff = Math.min(backoff * 2, 8000);
      };
    };
    open();
    setInterval(() => {
      const stale = Date.now() - lastMsg > 12000;
      overlay && overlay.classList.toggle("show", stale);
    }, 1000);
  },

  async post(path, body) {
    const r = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(data.error || r.statusText);
    return data;
  },

  async get(path) {
    const r = await fetch(path);
    if (!r.ok) throw new Error(r.statusText);
    return r.json();
  },

  caption(text) {
    const el = document.querySelector(".caption");
    if (!el) return;
    el.textContent = text;
    el.classList.add("show");
    clearTimeout(el._t);
    el._t = setTimeout(() => el.classList.remove("show"), 8000);
  },

  toast(text) {
    let el = document.querySelector(".toast");
    if (!el) {
      el = document.createElement("div");
      el.className = "toast";
      document.body.appendChild(el);
    }
    el.textContent = text;
    el.classList.add("show");
    clearTimeout(el._t);
    el._t = setTimeout(() => el.classList.remove("show"), 2200);
  },

  mins(s) { return Math.floor(s / 60); },

  esc(s) {
    return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  },

  statusCs: {
    HOLD: "čeká na chod", NEW: "nové", CLAIMED: "vzato", COOKING: "vaří se", READY: "hotovo",
  },
};
