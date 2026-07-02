"use strict";

// cocode frontend: wires a <textarea> to the CRDT and a WebSocket. Local edits
// are diffed into CRDT insert/delete ops and sent; remote ops are applied and
// the textarea is re-rendered from the CRDT's converged value.

(function () {
  const room = (location.hash || "#default").slice(1) || "default";
  const site = (Math.random() * 0xffffffff) >>> 0;
  const doc = new CRDT.Doc(site);

  const editor = document.getElementById("editor");
  const statusEl = document.getElementById("status");
  const presenceEl = document.getElementById("presence");
  document.getElementById("room").textContent = "room: " + room;

  let lastValue = "";
  let applyingRemote = false;

  // --- WebSocket ---
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws?room=${encodeURIComponent(room)}`);

  ws.onopen = () => setStatus("online", "online");
  ws.onclose = () => setStatus("offline", "offline");
  ws.onerror = () => setStatus("offline", "error");

  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data);
    switch (msg.type) {
      case "init":
        (msg.ops || []).forEach((op) => doc.apply(op));
        render();
        break;
      case "op":
        doc.apply(msg.op);
        render();
        break;
      case "presence":
        presenceEl.textContent = msg.count + (msg.count === 1 ? " online" : " online");
        break;
    }
  };

  function setStatus(cls, text) {
    statusEl.textContent = text;
    statusEl.className = "pill status-" + cls;
  }

  function send(op) {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "op", op }));
    }
  }

  // --- Local edits: diff textarea against lastValue into CRDT ops ---
  editor.addEventListener("input", () => {
    if (applyingRemote) return;
    const now = editor.value;
    const old = lastValue;

    // Longest common prefix and suffix (non-overlapping).
    let p = 0;
    while (p < old.length && p < now.length && old[p] === now[p]) p++;
    let s = 0;
    while (
      s < old.length - p &&
      s < now.length - p &&
      old[old.length - 1 - s] === now[now.length - 1 - s]
    ) {
      s++;
    }

    const removed = old.slice(p, old.length - s);
    const added = now.slice(p, now.length - s);

    // Deletes first (all at index p, since each delete shifts left).
    for (let i = 0; i < removed.length; i++) {
      const op = doc.localDelete(p);
      if (op) send(op);
    }
    // Then inserts at p, p+1, ...
    for (let i = 0; i < added.length; i++) {
      const op = doc.localInsert(p + i, added[i]);
      send(op);
    }

    lastValue = now;
  });

  // --- Render the converged CRDT value back into the textarea ---
  function render() {
    const before = editor.value;
    const next = doc.value();
    if (before === next) {
      lastValue = next;
      return;
    }
    // Best-effort caret preservation: keep the caret if the change was after it,
    // otherwise shift it by the length delta.
    const caret = editor.selectionStart;
    let cp = 0;
    while (cp < before.length && cp < next.length && before[cp] === next[cp]) cp++;
    let newCaret = caret <= cp ? caret : caret + (next.length - before.length);
    newCaret = Math.max(0, Math.min(newCaret, next.length));

    applyingRemote = true;
    editor.value = next;
    lastValue = next;
    editor.setSelectionRange(newCaret, newCaret);
    applyingRemote = false;
  }
})();
