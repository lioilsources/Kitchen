// Překlady webových displejů: 45 jazyků, katalogy v i18n/<jazyk>.json, bez závislostí.
// Jazyk: ?lang=xx > uložená volba > jazyk prohlížeče > čeština (zdrojový jazyk).
"use strict";

const I18N = (() => {
  const NAMES = {
    cs: "Čeština", en: "English", ar: "العربية", bn: "বাংলা", ca: "Català", da: "Dansk", de: "Deutsch",
    el: "Ελληνικά", es: "Español", fi: "Suomi", fr: "Français", gu: "ગુજરાતી", he: "עברית", hi: "हिन्दी",
    hr: "Hrvatski", hu: "Magyar", id: "Bahasa Indonesia", it: "Italiano", ja: "日本語", kn: "ಕನ್ನಡ",
    ko: "한국어", ml: "മലയാളം", mr: "मराठी", ms: "Bahasa Melayu", nb: "Norsk bokmål", nl: "Nederlands",
    or: "ଓଡ଼ିଆ", pa: "ਪੰਜਾਬੀ", pl: "Polski", "pt-BR": "Português (Brasil)", "pt-PT": "Português (Portugal)",
    ro: "Română", ru: "Русский", sk: "Slovenčina", sl: "Slovenščina", sv: "Svenska", ta: "தமிழ்",
    te: "తెలుగు", th: "ไทย", tr: "Türkçe", uk: "Українська", ur: "اردو", vi: "Tiếng Việt",
    "zh-Hans": "简体中文", "zh-Hant": "繁體中文",
  };
  const RTL = new Set(["ar", "he", "ur"]);
  const ALIAS = { iw: "he", no: "nb", nn: "nb", "zh-cn": "zh-Hans", "zh-sg": "zh-Hans", "zh-tw": "zh-Hant", "zh-hk": "zh-Hant", "zh-mo": "zh-Hant", pt: "pt-BR", zh: "zh-Hans" };
  const KEY = "predavka.lang";

  // Jazykový tag prohlížeče -> podporovaný kód ("pt-PT" přesně, "de-AT" na "de", ...).
  function match(tag) {
    const lc = String(tag || "").toLowerCase();
    if (!lc) return null;
    const supported = Object.keys(NAMES);
    return supported.find((c) => c.toLowerCase() === lc)
      || ALIAS[lc]
      || supported.find((c) => c.toLowerCase() === lc.split("-")[0])
      || ALIAS[lc.split("-")[0]]
      || null;
  }

  function choose() {
    const fromUrl = match(new URLSearchParams(location.search).get("lang"));
    if (fromUrl) return fromUrl;
    try {
      const saved = match(localStorage.getItem(KEY));
      if (saved) return saved;
    } catch (_) { /* soukromý režim */ }
    for (const tag of navigator.languages || [navigator.language]) {
      const m = match(tag);
      if (m) return m;
    }
    return "cs";
  }

  const lang = choose();
  let dict = {}, base = {};

  const load = async (l) => (await fetch(`i18n/${l}.json`)).json();

  function t(key, params) {
    let s = dict[key] ?? base[key] ?? key;
    if (params) s = s.replace(/\{(\w+)\}/g, (m, k) => (k in params ? params[k] : m));
    return s;
  }

  // Statické texty: data-i18n, data-i18n-html, data-i18n-title, data-i18n-placeholder.
  function apply(root) {
    root.querySelectorAll("[data-i18n]").forEach((el) => { el.textContent = t(el.dataset.i18n); });
    root.querySelectorAll("[data-i18n-html]").forEach((el) => { el.innerHTML = t(el.dataset.i18nHtml); });
    root.querySelectorAll("[data-i18n-title]").forEach((el) => { el.title = t(el.dataset.i18nTitle); });
    root.querySelectorAll("[data-i18n-placeholder]").forEach((el) => { el.placeholder = t(el.dataset.i18nPlaceholder); });
  }

  const ready = (async () => {
    document.documentElement.lang = lang;
    document.documentElement.dir = RTL.has(lang) ? "rtl" : "ltr";
    try {
      [dict, base] = await Promise.all([load(lang), lang === "cs" ? {} : load("cs")]);
    } catch (_) { /* bez katalogu zůstanou texty ze stránky */ }
    apply(document);
  })();

  // Výběr jazyka (stránka rozcestníku).
  function picker(select) {
    for (const [code, name] of Object.entries(NAMES)) {
      const o = document.createElement("option");
      o.value = code; o.textContent = name; o.lang = code;
      select.appendChild(o);
    }
    select.value = lang;
    select.onchange = () => {
      try { localStorage.setItem(KEY, select.value); } catch (_) { /* nic */ }
      const u = new URL(location.href);
      u.searchParams.delete("lang");
      location.href = u.toString();
    };
  }

  // Čas a čísla podle jazyka (cs-CZ jako dosud).
  const locale = lang === "cs" ? "cs-CZ" : lang;
  return { ready, t, apply, picker, lang, locale, NAMES };
})();

const T = I18N.t;
