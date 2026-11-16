// Test support: run a report page's own scripts in Node against a small DOM.
//
//   node scripts/accuracy_live_dom.js scenario.json
//
// The scenario holds the documents a server would return, as element trees
// ([tag, attributes, children] and strings), and a list of user actions and
// refreshes. Scripts are taken from the first document and executed as they
// are. What they leave behind is printed as JSON after the load and after
// every refresh, for accuracy_test_support.live_dom to assert on.
//
// The DOM is only what the report scripts use. Two browser behaviours that the
// scripts depend on are modelled deliberately:
//   - the element the address names (:target) is set by navigation only. An
//     element of the same name inserted later does not become the target, and
//     without a connected target the commit report shows its overview;
//   - a box has a height only where it is displayed (not in a closed details,
//     not in a view that is not shown), and its top follows document order.
'use strict';
const fs = require('fs');
const vm = require('vm');

let version = 0; // bumped by every change that can move a box
const state = {scrollY: 0, fetches: 0, reloads: 0, timers: [], nextTimer: 1};

class Text {
  constructor(data) { this.data = data; this.parentNode = null; }
  get textContent() { return this.data; }
}

const kebab = name => name.replace(/[A-Z]/g, letter => '-' + letter.toLowerCase());

class Element {
  constructor(tag, attributes, document) {
    this.localName = tag;
    this.ownerDocument = document;
    this.parentNode = null;
    this.childNodes = [];
    this.attributeMap = new Map(Object.entries(attributes));
    this.scrollLeft = 0;
    this.scrollTop = 0;
    this.style = {};
    this.dataset = new Proxy({}, {
      get: (_, name) => this.attributeMap.get('data-' + kebab(name)),
      set: (_, name, value) => { this.setAttribute('data-' + kebab(name), String(value)); return true; },
    });
  }
  get id() { return this.attributeMap.get('id') || ''; }
  get children() { return this.childNodes.filter(node => node instanceof Element); }
  get lastElementChild() { return this.children.at(-1) || null; }
  get parentElement() { return this.parentNode instanceof Element ? this.parentNode : null; }
  get attributes() { return [...this.attributeMap].map(([name, value]) => ({name, value})); }
  hasAttribute(name) { return this.attributeMap.has(name); }
  getAttribute(name) { return this.attributeMap.has(name) ? this.attributeMap.get(name) : null; }
  setAttribute(name, value) { this.attributeMap.set(name, String(value)); version++; }
  removeAttribute(name) { this.attributeMap.delete(name); version++; }
  get open() { return this.attributeMap.has('open'); }
  set open(value) { if (value) this.setAttribute('open', ''); else this.removeAttribute('open'); }
  get hidden() { return this.attributeMap.has('hidden'); }
  set hidden(value) { if (value) this.setAttribute('hidden', ''); else this.removeAttribute('hidden'); }
  get textContent() { return this.childNodes.map(node => node.textContent).join(''); }
  set textContent(value) { this.replaceChildren(new Text(String(value))); }
  get isConnected() { return this.ownerDocument.documentElement.contains(this); }
  contains(node) {
    for (; node; node = node.parentNode) if (node === this) return true;
    return false;
  }
  closest(selector) {
    for (let node = this; node instanceof Element; node = node.parentNode) if (matches(node, selector)) return node;
    return null;
  }
  querySelectorAll(selector) {
    const found = [];
    (function walk(node) {
      for (const child of node.children) {
        if (matches(child, selector)) found.push(child);
        walk(child);
      }
    })(this);
    return found;
  }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
  replaceChildren(...nodes) {
    for (const child of this.childNodes) child.parentNode = null;
    for (const node of nodes) {
      if (node.parentNode) node.parentNode.childNodes.splice(node.parentNode.childNodes.indexOf(node), 1);
      node.parentNode = this;
      if (node instanceof Element) adopt(node, this.ownerDocument);
    }
    this.childNodes = nodes;
    version++;
  }
  focus() { this.ownerDocument.activeElement = this; }
  getBoundingClientRect() {
    const top = layout(this.ownerDocument).get(this);
    return top === undefined ? {top: 0, bottom: 0, height: 0} : {top: top - state.scrollY, bottom: top - state.scrollY + 10, height: 10};
  }
}

function adopt(node, document) {
  node.ownerDocument = document;
  for (const child of node.children) adopt(child, document);
}

// One compound selector: tag, #id, .class and [attribute] or [attribute="value"] parts.
function matchesCompound(node, compound) {
  const parts = compound.match(/^[a-z0-9*]+|#[\w-]+|\.[\w-]+|\[[^\]]+\]/gi);
  return parts.every(part => {
    if (part === '*') return true;
    if (part[0] === '#') return node.id === part.slice(1);
    if (part[0] === '.') return (node.getAttribute('class') || '').split(/\s+/).includes(part.slice(1));
    if (part[0] === '[') {
      const [name, value] = part.slice(1, -1).split('=');
      return node.hasAttribute(name) && (value === undefined || node.getAttribute(name) === value.replace(/^["']|["']$/g, ''));
    }
    return node.localName === part;
  });
}

// Comma lists and descendant combinators of compound selectors.
function matches(node, selector) {
  return selector.split(',').some(alternative => {
    const chain = alternative.trim().split(/\s+/);
    if (!matchesCompound(node, chain.at(-1))) return false;
    let ancestor = node.parentNode;
    for (let i = chain.length - 2; i >= 0; i--) {
      while (ancestor instanceof Element && !matchesCompound(ancestor, chain[i])) ancestor = ancestor.parentNode;
      if (!(ancestor instanceof Element)) return false;
      ancestor = ancestor.parentNode;
    }
    return true;
  });
}

class Document {
  constructor(tree) {
    this.listeners = {};
    this.activeElement = null;
    this.target = null; // the element the address names; set by navigation only
    this.documentElement = build(tree, this);
  }
  getElementById(id) { return this.documentElement.querySelectorAll('*').find(node => node.id === id) || null; }
  querySelectorAll(selector) { return this.documentElement.querySelectorAll(selector); }
  querySelector(selector) { return this.documentElement.querySelector(selector); }
  addEventListener(name, listener) { (this.listeners[name] ||= []).push(listener); }
  dispatchEvent(event) { for (const listener of this.listeners[event.type] || []) listener(event); }
}

function build(tree, document) {
  if (typeof tree === 'string') return new Text(tree);
  const element = new Element(tree[0], tree[1], document);
  element.childNodes = tree[2].map(child => build(child, document));
  for (const child of element.childNodes) child.parentNode = element;
  return element;
}

// The view the commit report's style sheet shows: the one that is or contains
// the target, otherwise the overview.
function shownView(document) {
  const target = document.target && document.target.isConnected ? document.target.closest('.view') : null;
  return target || document.getElementById('overview');
}

const layouts = new WeakMap();
function layout(document) {
  const cached = layouts.get(document);
  if (cached && cached.version === version) return cached.tops;
  const tops = new Map(), shown = shownView(document);
  let row = 0;
  (function walk(node, displayed) {
    for (const child of node.children) {
      const visible = displayed && !child.hidden && !['script', 'style', 'title', 'meta'].includes(child.localName)
        && !(matchesCompound(child, '.view') && shown && child !== shown)
        && !(node.localName === 'details' && !node.open && child.localName !== 'summary');
      if (visible) tops.set(child, 10 * row++);
      walk(child, visible);
    }
  })(document.documentElement, true);
  layouts.set(document, {version, tops});
  return tops;
}

async function main() {
  const scenario = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
  const document = new Document(scenario.pages[0]);
  const windowListeners = {};
  const location = {href: 'http://report.test/', hash: '', reload() { state.reloads++; }};
  const sandbox = {
    document, location, console,
    setTimeout(callback, delay) { state.timers.push({id: state.nextTimer, callback, delay}); return state.nextTimer++; },
    clearTimeout(id) { state.timers = state.timers.filter(timer => timer.id !== id); },
    addEventListener(name, listener) { (windowListeners[name] ||= []).push(listener); },
    async fetch() {
      const page = scenario.pages[Math.min(++state.fetches, scenario.pages.length - 1)];
      if (page === null) throw new Error('network unreachable');
      if (page.status) return {ok: false, status: page.status};
      return {ok: true, text: async () => String(state.fetches)};
    },
    DOMParser: class { parseFromString(token) { return new Document(scenario.pages[Math.min(Number(token), scenario.pages.length - 1)]); } },
    Event: class { constructor(type) { this.type = type; } },
    AbortController, innerHeight: 800, innerWidth: 1200, scrollX: 0,
  };
  Object.defineProperty(sandbox, 'scrollY', {get: () => state.scrollY});
  sandbox.window = sandbox;
  sandbox.scrollTo = (x, y) => { state.scrollY = y; };
  vm.createContext(sandbox);
  for (const script of document.querySelectorAll('script')) vm.runInContext(script.textContent, sandbox);

  const tip = document.getElementById('point-tip');
  const watched = [];
  let anchor = null;
  const one = selector => {
    const node = document.querySelector(selector);
    if (!node) throw new Error('no element matches ' + selector);
    return node;
  };
  const observe = () => ({
    fetches: state.fetches, reloads: state.reloads, timers: state.timers.map(timer => timer.delay).sort((a, b) => a - b),
    liveState: document.getElementById('accuracy-report')?.dataset.liveState ?? null,
    targetConnected: document.target ? document.target.isConnected : null,
    shownView: shownView(document)?.id ?? null,
    current: document.querySelectorAll('nav.views a').filter(link => link.hasAttribute('aria-current')).map(link => link.getAttribute('href')),
    open: document.querySelectorAll('details').filter(node => node.open).map(node => node.id || node.querySelector('summary').textContent),
    focus: document.activeElement && document.activeElement.isConnected
      ? (document.activeElement.dataset.pointKey || document.activeElement.id || document.activeElement.localName) : null,
    focusView: document.activeElement && document.activeElement.isConnected ? document.activeElement.closest('.view')?.id ?? null : null,
    tip: tip ? {hidden: tip.hidden, text: tip.textContent, connected: !!(tip.point && tip.point.isConnected),
                view: tip.point && tip.point.isConnected ? tip.point.closest('.view')?.id ?? null : null} : null,
    scrollY: state.scrollY, anchorTop: anchor && anchor().getBoundingClientRect().top,
    text: Object.fromEntries(watched.map(selector => [selector, document.querySelector(selector)?.textContent ?? null])),
    elements: document.querySelectorAll('*').length,
  });

  const observations = [];
  for (const [action, argument] of scenario.steps) {
    if (action === 'navigate') {
      location.hash = argument;
      document.target = argument.length > 1 ? document.getElementById(argument.slice(1)) : null;
      version++;
      for (const listener of windowListeners.hashchange || []) listener({type: 'hashchange'});
    } else if (action === 'open') one(argument).open = true;
    else if (action === 'focus') one(argument).focus();
    else if (action === 'hover') { const point = one(argument); tip.textContent = point.dataset.tip; tip.point = point; tip.hidden = false; }
    else if (action === 'scroll') state.scrollY = argument;
    else if (action === 'anchor') anchor = () => one(argument);
    else if (action === 'watch') watched.push(argument);
    else if (action === 'observe') observations.push(observe());
    else if (action === 'refresh') {
      // The page's own timer: nothing happens unless the page scheduled it.
      const due = state.timers.find(timer => timer.delay === 15000);
      if (due) {
        state.timers = state.timers.filter(timer => timer !== due);
        await due.callback();
      }
      observations.push(observe());
    } else throw new Error('unknown step ' + action);
  }
  process.stdout.write(JSON.stringify(observations));
}

main().catch(error => { console.error(error); process.exit(1); });
