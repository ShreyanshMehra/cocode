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
  const participantsEl = document.getElementById("participants");
  document.getElementById("room").textContent = "room: " + room;

  const roomQuery = "room=" + encodeURIComponent(room);
  let lastValue = "";
  let applyingRemote = false;
  let selfId = null;

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
      case "welcome":
        selfId = msg.self ? msg.self.id : null;
        break;
      case "op":
        doc.apply(msg.op);
        render();
        break;
      case "presence":
        renderParticipants(msg.participants || []);
        break;
    }
  };

  function setStatus(cls, text) {
    statusEl.textContent = text;
    statusEl.className = "pill status-" + cls;
  }

  function renderParticipants(list) {
    participantsEl.innerHTML = "";
    list.forEach((p) => {
      const chip = document.createElement("span");
      chip.className = "chip";
      const dot = document.createElement("span");
      dot.className = "dot";
      dot.style.background = p.color;
      chip.appendChild(dot);
      const label = p.id === selfId ? p.name + " (you)" : p.name;
      chip.appendChild(document.createTextNode(label));
      participantsEl.appendChild(chip);
    });
  }

  function send(op) {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "op", op }));
    }
  }

  // Throttled cursor reporting so the caret position is shared for presence.
  let cursorTimer = null;
  function sendCursor() {
    if (cursorTimer || ws.readyState !== WebSocket.OPEN) return;
    cursorTimer = setTimeout(() => {
      cursorTimer = null;
      ws.send(JSON.stringify({ type: "cursor", cursor: editor.selectionStart }));
    }, 120);
  }
  editor.addEventListener("keyup", sendCursor);
  editor.addEventListener("click", sendCursor);

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

  // --- Version history + diff panel (uses the HTTP API) ---
  const saveBtn = document.getElementById("save-btn");
  const versionsEl = document.getElementById("versions");
  const diffEl = document.getElementById("diff");
  let versions = [];
  let selected = []; // up to 2 selected version ids, oldest-first for diff

  saveBtn.addEventListener("click", async () => {
    const msg = prompt("Version label:", "checkpoint");
    if (msg === null) return;
    saveBtn.disabled = true;
    try {
      await fetch(`/api/snapshot?${roomQuery}&message=${encodeURIComponent(msg)}`, {
        method: "POST",
      });
      await loadVersions();
    } finally {
      saveBtn.disabled = false;
    }
  });

  async function loadVersions() {
    const res = await fetch(`/api/versions?${roomQuery}`);
    const data = await res.json();
    versions = data.versions || [];
    renderVersions();
  }

  function renderVersions() {
    versionsEl.innerHTML = "";
    if (versions.length === 0) {
      const li = document.createElement("li");
      li.className = "muted";
      li.textContent = "No versions yet — click Save version.";
      versionsEl.appendChild(li);
      return;
    }
    versions.forEach((v) => {
      const li = document.createElement("li");
      if (selected.includes(v.id)) li.classList.add("selected");
      const msg = document.createElement("span");
      msg.className = "vmsg";
      msg.textContent = v.message || "(no label)";
      const time = document.createElement("span");
      time.className = "vtime";
      time.textContent = new Date(v.unixTime * 1000).toLocaleTimeString();
      li.appendChild(msg);
      li.appendChild(time);
      li.addEventListener("click", () => toggleSelect(v.id));
      versionsEl.appendChild(li);
    });
  }

  function toggleSelect(id) {
    const at = selected.indexOf(id);
    if (at >= 0) selected.splice(at, 1);
    else {
      selected.push(id);
      if (selected.length > 2) selected.shift();
    }
    renderVersions();
    if (selected.length === 2) loadDiff();
    else diffEl.innerHTML = '<span class="muted">Select two versions to compare.</span>';
  }

  async function loadDiff() {
    // Order the two ids oldest-first per the versions list.
    const order = versions.map((v) => v.id);
    const pair = [...selected].sort((a, b) => order.indexOf(a) - order.indexOf(b));
    const res = await fetch(`/api/diff?${roomQuery}&a=${pair[0]}&b=${pair[1]}`);
    const data = await res.json();
    renderDiff(data.diff || "");
  }

  function renderDiff(text) {
    diffEl.innerHTML = "";
    if (!text) {
      diffEl.innerHTML = '<span class="muted">No differences.</span>';
      return;
    }
    text.split("\n").forEach((line) => {
      if (line === "") return;
      const span = document.createElement("span");
      const mark = line[0];
      span.className = mark === "+" ? "add" : mark === "-" ? "del" : "ctx";
      span.textContent = line + "\n";
      diffEl.appendChild(span);
    });
  }

  loadVersions();
})();
