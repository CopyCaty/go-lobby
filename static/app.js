const $ = (id) => document.getElementById(id);

const storageKey = "go_lobby_token";

const routeMeta = {
  "#/login": {
    kicker: "Player Access",
    title: "玩家登录",
    desc: "创建或登录测试玩家，进入匹配大厅。",
  },
  "#/lobby": {
    kicker: "Lobby",
    title: "玩家大厅",
    desc: "登录玩家账号，进入匹配队列，完成房间准备和战绩结算。",
  },
  "#/match": {
    kicker: "Matchmaking",
    title: "匹配",
    desc: "选择模式并进入队列，等待系统分配房间。",
  },
  "#/room": {
    kicker: "Room",
    title: "我的房间",
    desc: "查看队伍和准备状态，连接房间实时消息。",
  },
  "#/settlement": {
    kicker: "Settlement",
    title: "战绩结算",
    desc: "提交对局结果并触发积分结算。",
  },
  "#/leaderboard": {
    kicker: "Leaderboard",
    title: "排行榜",
    desc: "查看不同模式下的玩家积分榜。",
  },
  "#/map": {
    kicker: "Chunk Pyramid",
    title: "中国地图",
    desc: "拖拽缩放查看中国地图 Chunk 金字塔，放大后自动进入 128x128 最低级 Chunk。",
  },
  "#/debug": {
    kicker: "Debug",
    title: "开发调试",
    desc: "查看 token、接口地址、原始响应和操作日志。",
  },
};

const chunkDemo = {
  region: "cn",
  maxLevel: 6,
  level: 0,
  minGridLevel: 6,
  maxZoom: 512,
  detailLevelZoom: 24,
  zoom: 1,
  centerX: 0.5,
  centerY: 0.5,
  size: 128,
  chunks: [],
  snapshots: new Map(),
  pendingSnapshots: new Set(),
  flags: new Map(),
  hoveredChunk: null,
  hoveredCell: null,
  dragging: false,
  dragStart: null,
  dragMoved: false,
  actionPending: false,
  lastAction: "",
  loadTimer: null,
  lastBBox: null,
  fallback: false,
};

const appState = {
  token: localStorage.getItem(storageKey) || "",
  user: null,
  queue: null,
  room: null,
  match: null,
  leaderboard: null,
  ws: null,
};

function pretty(value) {
  try {
    return JSON.stringify(value, null, 2);
  } catch (error) {
    return String(value);
  }
}

function text(id, value) {
  const el = $(id);
  if (el) el.textContent = value;
}

function setOutput(value) {
  text("output", pretty(value));
}

function setBadge(id, value, kind = "") {
  const el = $(id);
  if (!el) return;
  el.textContent = value;
  el.className = "badge" + (kind ? " " + kind : "");
}

function showNotice(message, kind = "") {
  const el = $("notice");
  if (!el) return;
  el.textContent = message;
  el.className = "notice" + (kind ? " " + kind : "");
  el.hidden = false;
}

function clearNotice() {
  const el = $("notice");
  if (!el) return;
  el.textContent = "";
  el.hidden = true;
}

function payloadOf(data) {
  return data && data.data ? data.data : data;
}

function statusText(status) {
  const map = {
    init: "未入队",
    matching: "匹配中",
    matched: "已匹配",
    cancelled: "已取消",
    waiting: "等待准备",
    playing: "对局中",
    finished: "已结束",
  };
  return map[String(status || "").toLowerCase()] || status || "-";
}

function badgeKind(status) {
  const value = String(status || "").toLowerCase();
  if (["matched", "playing", "finished"].includes(value)) return "ok";
  if (["matching", "waiting"].includes(value)) return "warn";
  if (["cancelled", "failed"].includes(value)) return "bad";
  return "";
}

function addLog(title, detail = "") {
  const line = `${new Date().toLocaleTimeString()} - ${title}${detail ? " - " + detail : ""}`;
  ["log", "dashboard_log"].forEach((id) => {
    const container = $(id);
    if (!container) return;
    const item = document.createElement("div");
    item.className = "event-item";
    item.textContent = line;
    container.prepend(item);
  });
}

function addWSLog(title, detail = "") {
  const container = $("ws_log");
  if (!container) return;
  const item = document.createElement("div");
  item.className = "event-item";
  item.textContent = `${new Date().toLocaleTimeString()} - ${title}${detail ? " - " + detail : ""}`;
  container.prepend(item);
}

function syncToken() {
  const tokenField = $("token");
  const token = tokenField ? tokenField.value.trim() : appState.token;
  appState.token = token;
  if (token) {
    localStorage.setItem(storageKey, token);
  } else {
    localStorage.removeItem(storageKey);
  }
}

function rawToken() {
  syncToken();
  return appState.token.startsWith("Bearer ")
    ? appState.token.slice("Bearer ".length).trim()
    : appState.token;
}

function authHeaders(extra = {}) {
  syncToken();
  const headers = { ...extra };
  if (appState.token) {
    headers.Authorization = appState.token.startsWith("Bearer ")
      ? appState.token
      : "Bearer " + appState.token;
  }
  return headers;
}

async function apiRequest(label, url, options = {}) {
  const resp = await fetch(url, options);
  const body = await resp.text();
  let data;
  try {
    data = JSON.parse(body);
  } catch (error) {
    data = { raw: body };
  }

  setOutput({
    request: { label, url, method: options.method || "GET" },
    response: data,
  });

  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `HTTP ${resp.status}`);
  }
  return data;
}

function setRoute(hash) {
  const route = routeMeta[hash] ? hash : appState.token ? "#/lobby" : "#/login";
  const meta = routeMeta[route];
  clearNotice();

  document.querySelectorAll(".view").forEach((view) => {
    view.classList.toggle("active", view.id === `view_${route.slice(2)}`);
  });
  document.querySelectorAll("[data-route]").forEach((link) => {
    link.classList.toggle("active", link.dataset.route === route);
  });

  text("page_kicker", meta.kicker);
  text("page_title", meta.title);
  text("page_desc", meta.desc);
  document.body.classList.toggle("map-route", route === "#/map");

  if (route === "#/map") {
    startMapView().catch((error) => {
      addLog("Chunk 渲染失败", String(error.message || error));
    });
  }
}

function updatePlayerHeader() {
  const user = appState.user || {};
  const hasToken = Boolean(appState.token);
  const name = user.nickname || user.user_name || (hasToken ? "已保存登录凭证" : "未登录");
  text("session_user", name);
  text("session_hint", hasToken ? `玩家 ID：${user.user_id || "-"}` : "登录后可进入匹配队列。");
  setBadge("account_status", hasToken ? "已登录" : "未登录", hasToken ? "ok" : "");
  text("lobby_welcome", hasToken ? `${name}，准备开始匹配` : "欢迎来到匹配大厅");
  text("lobby_summary", hasToken ? "选择模式进入队列，匹配成功后前往房间准备。" : "请先登录玩家账号，再进入匹配队列。");
  updateProgress();
}

function updateProgress() {
  $("step_login")?.classList.toggle("done", Boolean(appState.token));
  $("step_queue")?.classList.toggle("done", Boolean(appState.queue));
  $("step_room")?.classList.toggle("done", Boolean(appState.room || (appState.queue && appState.queue.room_id)));
  $("step_result")?.classList.toggle("done", Boolean(appState.match && (appState.match.finished_at || appState.match.win_team_no !== undefined)));

  if (!appState.token) {
    setBadge("lobby_progress_badge", "等待登录");
  } else if (appState.match && (appState.match.finished_at || appState.match.win_team_no !== undefined)) {
    setBadge("lobby_progress_badge", "已完成", "ok");
  } else if (appState.room || (appState.queue && appState.queue.room_id)) {
    setBadge("lobby_progress_badge", "房间阶段", "warn");
  } else if (appState.queue) {
    setBadge("lobby_progress_badge", "匹配阶段", "warn");
  } else {
    setBadge("lobby_progress_badge", "可匹配", "ok");
  }
}

function syncQueueView(data) {
  const payload = payloadOf(data);
  if (!payload) return;

  appState.queue = payload;
  const status = payload.status || payload.queue_status || "init";
  const roomID = payload.room_id || "";
  const matchID = payload.match_id || "";

  text("metric_status", statusText(status));
  text("metric_room", roomID || "-");
  text("metric_match", matchID || "-");
  setBadge("queue_status", statusText(status), badgeKind(status));

  if (roomID) $("room_id").value = roomID;
  if (matchID) $("match_id").value = matchID;

  renderQueueResult(payload);
  updateProgress();
}

function renderQueueResult(payload) {
  const card = $("queue_result_card");
  if (!card) return;

  const status = payload.status || payload.queue_status || "init";
  const roomID = payload.room_id || "-";
  const matchID = payload.match_id || "-";
  card.innerHTML = `
    <strong>${statusText(status)}</strong>
    <p>模式：${payload.mode || $("queue_mode").value}</p>
    <p>房间：${roomID}</p>
    <p>对局：${matchID}</p>
  `;
}

function normalizePlayers(room) {
  const players = room.players || room.Players || {};
  return Array.isArray(players) ? players : Object.values(players);
}

function renderRoomPlayers(room) {
  const container = $("room_players");
  if (!container) return;
  container.innerHTML = "";

  const players = normalizePlayers(room);
  if (!players.length) {
    container.innerHTML = `<div class="empty-state">暂无房间成员数据</div>`;
    return;
  }

  const teams = new Map();
  players.forEach((player) => {
    const teamNo = player.team_no ?? player.TeamNo ?? 0;
    if (!teams.has(teamNo)) teams.set(teamNo, []);
    teams.get(teamNo).push(player);
  });

  [...teams.entries()].sort(([a], [b]) => Number(a) - Number(b)).forEach(([teamNo, teamPlayers]) => {
    const card = document.createElement("section");
    card.className = "team-card";
    card.innerHTML = `<h3>队伍 ${teamNo}</h3>`;
    const list = document.createElement("div");
    list.className = "player-list";

    teamPlayers.forEach((player) => {
      const userID = player.user_id ?? player.UserID ?? "-";
      const ready = player.ready ?? player.Ready ?? false;
      const online = player.online ?? player.Online ?? false;
      const line = document.createElement("div");
      line.className = "player-line";
      line.innerHTML = `
        <strong>玩家 ${userID}</strong>
        <span>${ready ? "已准备" : "未准备"} / ${online ? "在线" : "离线"}</span>
      `;
      list.appendChild(line);
    });

    card.appendChild(list);
    container.appendChild(card);
  });
}

function syncRoomView(data) {
  const room = payloadOf(data);
  if (!room) return;

  appState.room = room;
  const roomID = room.id || room.room_id || "";
  const status = room.status || "waiting";

  text("metric_room", roomID || "-");
  text("metric_match", room.match_id || "-");
  text("metric_room_status", statusText(status));
  setBadge("room_status", statusText(status), badgeKind(status));
  text("socket_hint", roomID ? `房间 ${roomID}` : "未连接房间");

  if (roomID) $("room_id").value = roomID;
  if (room.match_id) $("match_id").value = room.match_id;

  renderRoomPlayers(room);
  updateProgress();
}

function renderMatch(match) {
  const el = $("match_snapshot");
  if (!el) return;
  const players = match.players || match.Players || [];
  const winTeam = match.win_team_no ?? match.WinTeamNo ?? "-";
  const status = statusText(match.status);
  el.innerHTML = `
    <p><strong>对局：</strong>${match.id || match.match_id || "-"}</p>
    <p><strong>房间：</strong>${match.room_id || "-"}</p>
    <p><strong>模式：</strong>${match.mode || "-"}</p>
    <p><strong>状态：</strong>${status}</p>
    <p><strong>胜方：</strong>${winTeam}</p>
    <p><strong>玩家数：</strong>${players.length || "-"}</p>
  `;
}

function syncMatchView(data) {
  const match = payloadOf(data);
  if (!match) return;

  appState.match = match;
  text("metric_match", match.id || match.match_id || "-");
  if (match.room_id) {
    text("metric_room", match.room_id);
    $("room_id").value = match.room_id;
  }

  const finished = Boolean(match.finished_at || match.win_team_no !== undefined);
  setBadge("match_status", finished ? "已结算" : "待结算", finished ? "ok" : "warn");
  renderMatch(match);
  updateProgress();
}

function renderLeaderboard(items) {
  const container = $("leaderboard_items");
  if (!container) return;
  container.innerHTML = "";

  if (!items.length) {
    container.innerHTML = `<div class="empty-state">暂无排行榜数据</div>`;
    return;
  }

  items.forEach((item, index) => {
    const rank = item.rank ?? index + 1;
    const userID = item.user_id ?? item.userID ?? "-";
    const score = item.score ?? 0;
    const row = document.createElement("div");
    row.className = "leaderboard-row";
    row.innerHTML = `
      <strong>#${rank}</strong>
      <span>玩家 ${userID}</span>
      <b>${score} 分</b>
    `;
    container.appendChild(row);
  });
}

function renderLeaderboardPreview(items) {
  const el = $("lobby_leaderboard_preview");
  if (!el) return;
  if (!items.length) {
    el.textContent = "暂无榜单数据";
    return;
  }
  el.innerHTML = items.slice(0, 3).map((item, index) => {
    const userID = item.user_id ?? item.userID ?? "-";
    const score = item.score ?? 0;
    return `<p>#${item.rank ?? index + 1} 玩家 ${userID} · ${score} 分</p>`;
  }).join("");
}

function syncLeaderboardView(data) {
  const payload = payloadOf(data);
  if (!payload) return;

  appState.leaderboard = payload;
  const items = payload.items || [];
  renderLeaderboard(items);
  renderLeaderboardPreview(items);
  setBadge("leaderboard_status", items.length ? `${items.length} 名玩家` : "空榜", items.length ? "ok" : "warn");
}

function cellIndex(x, y) {
  return y * chunkDemo.size + x;
}

function chunkColorFor(state) {
  const colors = {
    normal: "rgba(45, 92, 76, 0.58)",
    opening: "rgba(216, 179, 75, 0.72)",
    hot: "rgba(223, 124, 63, 0.76)",
    hidden: "rgba(20, 38, 32, 0.64)",
    opened: "#a7c7b9",
    flagged: "#f4c95d",
    closed: "rgba(127, 29, 29, 0.82)",
  };
  return colors[state] || colors.normal;
}

function chunkStateText(state) {
  const map = {
    normal: "普通",
    opening: "探索中",
    hot: "活跃",
    hidden: "未探索",
    opened: "已打开",
    flagged: "已标记",
    closed: "封闭",
  };
  return map[state] || state || "-";
}

function chunkGridSize(level) {
  return 2 ** level;
}

function mapLevelForZoom() {
  if (chunkDemo.zoom >= chunkDemo.detailLevelZoom) return chunkDemo.maxLevel;
  return Math.max(0, Math.min(chunkDemo.maxLevel - 1, Math.floor(Math.log2(chunkDemo.zoom))));
}

function resizeMapCanvas() {
  const canvas = $("chunk_canvas");
  if (!canvas) return null;
  const rect = canvas.getBoundingClientRect();
  const ratio = window.devicePixelRatio || 1;
  const width = Math.max(640, Math.floor(rect.width * ratio));
  const height = Math.max(420, Math.floor(rect.height * ratio));
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width;
    canvas.height = height;
  }
  return { canvas, ratio, cssWidth: rect.width, cssHeight: rect.height };
}

function mapScale(canvas) {
  return Math.min(canvas.width, canvas.height) * 0.82 * chunkDemo.zoom;
}

function worldToScreen(x, y, canvas) {
  const scale = mapScale(canvas);
  return {
    x: canvas.width / 2 + (x - chunkDemo.centerX) * scale,
    y: canvas.height / 2 + (y - chunkDemo.centerY) * scale,
  };
}

function screenToWorld(clientX, clientY) {
  const canvas = $("chunk_canvas");
  if (!canvas) return null;
  const rect = canvas.getBoundingClientRect();
  const ratio = window.devicePixelRatio || 1;
  const scale = mapScale(canvas);
  return {
    x: chunkDemo.centerX + ((clientX - rect.left) * ratio - canvas.width / 2) / scale,
    y: chunkDemo.centerY + ((clientY - rect.top) * ratio - canvas.height / 2) / scale,
  };
}

function currentMapBBox(canvas) {
  const scale = mapScale(canvas);
  const halfW = canvas.width / 2 / scale;
  const halfH = canvas.height / 2 / scale;
  return {
    min_x: Math.max(0, chunkDemo.centerX - halfW),
    min_y: Math.max(0, chunkDemo.centerY - halfH),
    max_x: Math.min(1, chunkDemo.centerX + halfW),
    max_y: Math.min(1, chunkDemo.centerY + halfH),
  };
}

function clampMapCenter() {
  chunkDemo.centerX = Math.max(0, Math.min(1, chunkDemo.centerX));
  chunkDemo.centerY = Math.max(0, Math.min(1, chunkDemo.centerY));
}

function formatBBox(bbox) {
  return `${bbox.min_x.toFixed(3)},${bbox.min_y.toFixed(3)} - ${bbox.max_x.toFixed(3)},${bbox.max_y.toFixed(3)}`;
}

function fallbackChunks(level, bbox) {
  const grid = chunkGridSize(level);
  const minX = Math.max(0, Math.min(grid - 1, Math.floor(bbox.min_x * grid)));
  const minY = Math.max(0, Math.min(grid - 1, Math.floor(bbox.min_y * grid)));
  const maxX = Math.max(0, Math.min(grid - 1, Math.floor((bbox.max_x - 0.000000001) * grid)));
  const maxY = Math.max(0, Math.min(grid - 1, Math.floor((bbox.max_y - 0.000000001) * grid)));
  const chunks = [];
  for (let y = minY; y <= maxY; y += 1) {
    for (let x = minX; x <= maxX; x += 1) {
      const score = (x * 31 + y * 17 + level * 13) % 23;
      const state = score === 0 || score === 7 ? "closed" : score === 3 || score === 11 ? "hot" : score === 5 || score === 19 ? "opening" : "normal";
      chunks.push({
        chunk_id: `cn:${level}:${x}:${y}`,
        region: "cn",
        level,
        z: level,
        x,
        y,
        state,
        closed: state === "closed",
        opened_count: 80 + ((x * 43 + y * 29 + level * 97) % 600),
        bounds: {
          min_x: x / grid,
          min_y: y / grid,
          max_x: (x + 1) / grid,
          max_y: (y + 1) / grid,
        },
      });
    }
  }
  return chunks;
}

async function mapFetchJSON(url) {
  const resp = await fetch(url);
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok || data.code !== 0) {
    throw new Error(data.message || `HTTP ${resp.status}`);
  }
  return payloadOf(data);
}

async function mapActionRequest(label, url, body) {
  const data = await apiRequest(label, url, {
    method: "POST",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
  });
  return payloadOf(data);
}

async function loadVisibleChunks() {
  const canvasInfo = resizeMapCanvas();
  if (!canvasInfo) return;
  chunkDemo.level = mapLevelForZoom();
  const bbox = currentMapBBox(canvasInfo.canvas);
  chunkDemo.lastBBox = bbox;
  const params = new URLSearchParams({
    level: String(chunkDemo.level),
    min_x: String(bbox.min_x),
    min_y: String(bbox.min_y),
    max_x: String(bbox.max_x),
    max_y: String(bbox.max_y),
  });

  setBadge("chunk_status", "加载中", "warn");
  try {
    const payload = await mapFetchJSON(`/api/v1/map/chunks?${params.toString()}`);
    chunkDemo.chunks = payload.chunks || [];
    pruneInvisibleFlags();
    chunkDemo.fallback = false;
    setBadge("chunk_status", chunkDemo.level === chunkDemo.maxLevel ? "最低级 Chunk" : "聚合层", "ok");
  } catch (error) {
    chunkDemo.chunks = fallbackChunks(chunkDemo.level, bbox);
    pruneInvisibleFlags();
    chunkDemo.fallback = true;
    setBadge("chunk_status", "本地 fallback", "warn");
    addLog("地图 Chunk 接口不可用", String(error.message || error));
  }

  if (chunkDemo.level === chunkDemo.maxLevel) {
    await loadVisibleSnapshots();
  }
  drawChunkCanvas();
}

function pruneInvisibleFlags() {
  const visibleIDs = new Set(chunkDemo.chunks.map((chunk) => chunk.chunk_id));
  [...chunkDemo.flags.keys()].forEach((chunkID) => {
    if (!visibleIDs.has(chunkID)) chunkDemo.flags.delete(chunkID);
  });
}

async function loadVisibleSnapshots() {
  const tasks = chunkDemo.chunks.map((chunk) => {
    if (chunkDemo.snapshots.has(chunk.chunk_id) || chunkDemo.pendingSnapshots.has(chunk.chunk_id)) {
      return Promise.resolve();
    }
    chunkDemo.pendingSnapshots.add(chunk.chunk_id);
    return mapFetchJSON(`/api/v1/map/chunks/${encodeURIComponent(chunk.chunk_id)}/snapshot`)
      .then((snapshot) => {
        chunkDemo.snapshots.set(chunk.chunk_id, snapshot);
      })
      .catch((error) => {
        addLog("Chunk 快照不可用", `${chunk.chunk_id}: ${String(error.message || error)}`);
      })
      .finally(() => {
        chunkDemo.pendingSnapshots.delete(chunk.chunk_id);
      });
  });
  await Promise.all(tasks);
}

function openedCellInSnapshot(chunkID, x, y) {
  const snapshot = chunkDemo.snapshots.get(chunkID);
  return snapshot?.opened_cells?.find((cell) => Number(cell.x) === x && Number(cell.y) === y) || null;
}

function flaggedCellsFor(chunkID) {
  if (!chunkDemo.flags.has(chunkID)) {
    chunkDemo.flags.set(chunkID, new Map());
  }
  return chunkDemo.flags.get(chunkID);
}

function isCellFlagged(chunkID, index) {
  return Boolean(chunkDemo.flags.get(chunkID)?.get(index));
}

function setCellFlag(chunkID, index, flagged) {
  const flags = flaggedCellsFor(chunkID);
  if (flagged) flags.set(index, true);
  else flags.delete(index);
  if (flags.size === 0) chunkDemo.flags.delete(chunkID);
}

function drawChinaShape(ctx, canvas) {
  const points = [
    [0.20, 0.20], [0.33, 0.13], [0.52, 0.16], [0.66, 0.23], [0.76, 0.35],
    [0.84, 0.49], [0.75, 0.62], [0.66, 0.76], [0.50, 0.84], [0.35, 0.78],
    [0.24, 0.66], [0.16, 0.50], [0.12, 0.34],
  ];
  ctx.beginPath();
  points.forEach(([x, y], index) => {
    const p = worldToScreen(x, y, canvas);
    if (index === 0) ctx.moveTo(p.x, p.y);
    else ctx.lineTo(p.x, p.y);
  });
  ctx.closePath();
  ctx.fillStyle = "rgba(26, 55, 46, 0.54)";
  ctx.fill();
  ctx.strokeStyle = "rgba(167, 199, 185, 0.56)";
  ctx.lineWidth = 2;
  ctx.stroke();
}

function drawChunkRect(ctx, canvas, chunk) {
  const bounds = chunk.bounds;
  const start = worldToScreen(bounds.min_x, bounds.min_y, canvas);
  const end = worldToScreen(bounds.max_x, bounds.max_y, canvas);
  const width = end.x - start.x;
  const height = end.y - start.y;
  ctx.fillStyle = chunkColorFor(chunk.state);
  ctx.fillRect(start.x, start.y, width, height);
  ctx.strokeStyle = chunk.closed ? "rgba(248, 113, 113, 0.95)" : "rgba(236, 244, 239, 0.20)";
  ctx.lineWidth = chunk.closed ? 2 : 1;
  ctx.strokeRect(start.x + 0.5, start.y + 0.5, width, height);
  if (chunk.level < chunkDemo.maxLevel) {
    ctx.fillStyle = "rgba(236, 244, 239, 0.74)";
    ctx.font = "12px sans-serif";
    ctx.fillText(chunk.chunk_id, start.x + 8, start.y + 18);
  }
}

function drawSnapshotCells(ctx, canvas, chunk) {
  const bounds = chunk.bounds;
  const start = worldToScreen(bounds.min_x, bounds.min_y, canvas);
  const end = worldToScreen(bounds.max_x, bounds.max_y, canvas);
  const width = end.x - start.x;
  const height = end.y - start.y;
  const cellW = width / chunkDemo.size;
  const cellH = height / chunkDemo.size;
  ctx.fillStyle = chunk.closed ? chunkColorFor("closed") : "rgba(20, 38, 32, 0.76)";
  ctx.fillRect(start.x, start.y, width, height);

  const snapshot = chunkDemo.snapshots.get(chunk.chunk_id);
  const openedCells = snapshot?.opened_cells || [];
  ctx.fillStyle = chunkColorFor("opened");
  openedCells.forEach((cell) => {
    ctx.fillRect(start.x + cell.x * cellW, start.y + cell.y * cellH, Math.max(1, cellW), Math.max(1, cellH));
  });

  const flags = chunkDemo.flags.get(chunk.chunk_id);
  if (flags && flags.size > 0) {
    ctx.fillStyle = chunkColorFor("flagged");
    flags.forEach((_, index) => {
      const x = index % chunkDemo.size;
      const y = Math.floor(index / chunkDemo.size);
      const left = start.x + x * cellW;
      const top = start.y + y * cellH;
      ctx.beginPath();
      ctx.moveTo(left + cellW * 0.28, top + cellH * 0.22);
      ctx.lineTo(left + cellW * 0.76, top + cellH * 0.44);
      ctx.lineTo(left + cellW * 0.28, top + cellH * 0.66);
      ctx.closePath();
      ctx.fill();
      if (cellW >= 8) {
        ctx.strokeStyle = "rgba(255, 255, 255, 0.72)";
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(left + cellW * 0.28, top + cellH * 0.22);
        ctx.lineTo(left + cellW * 0.28, top + cellH * 0.82);
        ctx.stroke();
      }
    });
  }

  if (cellW >= 12) {
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.font = `${Math.max(10, Math.min(18, cellW * 0.42))}px sans-serif`;
    ctx.fillStyle = "#12352a";
    openedCells.forEach((cell) => {
      const adjacentMines = Number(cell.adjacent_mines || 0);
      if (adjacentMines <= 0) return;
      ctx.fillText(String(adjacentMines), start.x + (cell.x + 0.5) * cellW, start.y + (cell.y + 0.5) * cellH);
    });
    ctx.textAlign = "start";
    ctx.textBaseline = "alphabetic";
  }

  ctx.strokeStyle = chunk.closed ? "rgba(248, 113, 113, 0.95)" : "rgba(236, 244, 239, 0.24)";
  ctx.lineWidth = chunk.closed ? 2 : 1;
  ctx.strokeRect(start.x + 0.5, start.y + 0.5, width, height);

  if (cellW >= 12) {
    ctx.strokeStyle = "rgba(236, 244, 239, 0.12)";
    ctx.lineWidth = 1;
    for (let i = 0; i <= chunkDemo.size; i += 1) {
      const x = Math.round(start.x + i * cellW) + 0.5;
      const y = Math.round(start.y + i * cellH) + 0.5;
      ctx.beginPath();
      ctx.moveTo(x, start.y);
      ctx.lineTo(x, end.y);
      ctx.moveTo(start.x, y);
      ctx.lineTo(end.x, y);
      ctx.stroke();
    }
  } else if (cellW >= 3) {
    ctx.strokeStyle = "rgba(236, 244, 239, 0.065)";
    ctx.lineWidth = 1;
    for (let i = 0; i <= chunkDemo.size; i += 8) {
      const x = Math.round(start.x + i * cellW) + 0.5;
      const y = Math.round(start.y + i * cellH) + 0.5;
      ctx.beginPath();
      ctx.moveTo(x, start.y);
      ctx.lineTo(x, end.y);
      ctx.moveTo(start.x, y);
      ctx.lineTo(end.x, y);
      ctx.stroke();
    }
  }
}

function drawChunkCanvas() {
  const canvasInfo = resizeMapCanvas();
  if (!canvasInfo) return;
  const { canvas } = canvasInfo;
  const ctx = canvas.getContext("2d");

  ctx.clearRect(0, 0, canvas.width, canvas.height);
  const gradient = ctx.createLinearGradient(0, 0, canvas.width, canvas.height);
  gradient.addColorStop(0, "#081512");
  gradient.addColorStop(1, "#102019");
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  drawChinaShape(ctx, canvas);

  chunkDemo.chunks.forEach((chunk) => {
    if (chunkDemo.level === chunkDemo.maxLevel) drawSnapshotCells(ctx, canvas, chunk);
    else drawChunkRect(ctx, canvas, chunk);
  });

  updateMapHUD();
}

function updateChunkHover(event) {
  if (chunkDemo.dragging) return;
  const hit = mapCellHit(event.clientX, event.clientY);
  chunkDemo.hoveredChunk = hit?.chunk || null;
  chunkDemo.hoveredCell = hit || null;
  updateMapHUD();
}

function mapCellHit(clientX, clientY) {
  const world = screenToWorld(clientX, clientY);
  if (!world) return null;
  const chunk = chunkDemo.chunks.find((item) => {
    const b = item.bounds;
    return world.x >= b.min_x && world.x < b.max_x && world.y >= b.min_y && world.y < b.max_y;
  });
  if (!chunk) return null;
  if (chunkDemo.level !== chunkDemo.maxLevel) {
    return { chunk };
  }
  const b = chunk.bounds;
  const x = Math.max(0, Math.min(chunkDemo.size - 1, Math.floor(((world.x - b.min_x) / (b.max_x - b.min_x)) * chunkDemo.size)));
  const y = Math.max(0, Math.min(chunkDemo.size - 1, Math.floor(((world.y - b.min_y) / (b.max_y - b.min_y)) * chunkDemo.size)));
  const index = cellIndex(x, y);
  const opened = openedCellInSnapshot(chunk.chunk_id, x, y);
  const flagged = isCellFlagged(chunk.chunk_id, index);
  return { chunk, x, y, index, opened, flagged };
}

function canActOnHit(hit, action) {
  if (!hit?.chunk || hit.x === undefined || hit.y === undefined) return "请先放大到最低级 Chunk";
  if (!appState.token) return "请先登录后再操作地图";
  if (chunkDemo.fallback) return "当前使用本地 fallback，不能提交操作";
  if (chunkDemo.actionPending) return "上一次地图操作尚未完成";
  if (hit.chunk.closed) return "chunk 已封闭";
  if (hit.opened) return action === "flag" ? "已打开格子不能标记" : "";
  if (action === "open" && hit.flagged) return "已标记格子需要先取消标记";
  return "";
}

function ensureSnapshot(chunkID) {
  const snapshot = chunkDemo.snapshots.get(chunkID) || {
    chunk_id: chunkID,
    width: chunkDemo.size,
    height: chunkDemo.size,
    closed: false,
    version: 1,
    opened_cells: [],
  };
  if (!Array.isArray(snapshot.opened_cells)) snapshot.opened_cells = [];
  chunkDemo.snapshots.set(chunkID, snapshot);
  return snapshot;
}

function upsertOpenedCell(chunkID, result) {
  const snapshot = ensureSnapshot(chunkID);
  const x = Number(result.x);
  const y = Number(result.y);
  const index = Number(result.index ?? cellIndex(x, y));
  const nextCell = {
    x,
    y,
    index,
    adjacent_mines: Number(result.adjacent_mines || 0),
    opened_by: appState.user ? { user_id: appState.user.user_id || appState.user.id || 0, nickname: appState.user.nickname || "" } : { user_id: 0, nickname: "" },
    opened_at: result.opened_at || new Date().toISOString(),
  };
  const existingIndex = snapshot.opened_cells.findIndex((cell) => Number(cell.index) === index);
  if (existingIndex >= 0) snapshot.opened_cells[existingIndex] = nextCell;
  else snapshot.opened_cells.push(nextCell);
  snapshot.version = result.version || snapshot.version;
  setCellFlag(chunkID, index, false);
}

function upsertOpenedCells(chunkID, result) {
  const cells = Array.isArray(result.opened_cells) && result.opened_cells.length > 0
    ? result.opened_cells
    : [result];
  cells.forEach((cell) => upsertOpenedCell(chunkID, { ...cell, version: result.version }));
}

function updateChunkAfterAction(chunkID, result) {
  const chunk = chunkDemo.chunks.find((item) => item.chunk_id === chunkID);
  if (!chunk) return;
  if (result.closed) {
    chunk.closed = true;
    chunk.state = "closed";
  } else if ((chunk.opened_count || 0) <= 0) {
    chunk.state = "opening";
  }
  if (result.version) chunk.version = result.version;
  if (!result.mine && result.index !== undefined) {
    chunk.opened_count = Math.max(Number(chunk.opened_count || 0), (chunkDemo.snapshots.get(chunkID)?.opened_cells || []).length);
  }
}

async function openMapCell(hit) {
  const blocked = canActOnHit(hit, "open");
  if (blocked) {
    if (blocked) showNotice(blocked, "warn");
    return;
  }
  chunkDemo.actionPending = true;
  setBadge("chunk_status", "开格中", "warn");
  try {
    const result = await mapActionRequest("地图开格", `/api/v1/map/chunks/${encodeURIComponent(hit.chunk.chunk_id)}/open`, { x: hit.x, y: hit.y });
    if (!result.mine) upsertOpenedCells(hit.chunk.chunk_id, result);
    updateChunkAfterAction(hit.chunk.chunk_id, result);
    const openedCount = Array.isArray(result.opened_cells) ? result.opened_cells.length : 1;
    chunkDemo.lastAction = result.mine
      ? `触雷：${hit.chunk.chunk_id} (${hit.x},${hit.y})`
      : `开格：${hit.chunk.chunk_id} (${hit.x},${hit.y})，邻雷 ${result.adjacent_mines || 0}，打开 ${openedCount} 格`;
    text("chunk_last_action", chunkDemo.lastAction);
    setBadge("chunk_status", result.mine ? "触雷封闭" : "已开格", result.mine ? "bad" : "ok");
    addLog(result.mine ? "地图触雷" : "地图开格", chunkDemo.lastAction);
  } catch (error) {
    const message = String(error.message || error);
    setBadge("chunk_status", "操作失败", "bad");
    addLog("地图开格失败", message);
    showNotice(message, "bad");
  } finally {
    chunkDemo.actionPending = false;
    drawChunkCanvas();
  }
}

async function toggleMapFlag(hit) {
  const blocked = canActOnHit(hit, "flag");
  if (blocked) {
    if (blocked) showNotice(blocked, "warn");
    return;
  }
  const nextFlagged = !hit.flagged;
  chunkDemo.actionPending = true;
  setBadge("chunk_status", nextFlagged ? "标记中" : "取消标记中", "warn");
  try {
    const result = await mapActionRequest("地图标记", `/api/v1/map/chunks/${encodeURIComponent(hit.chunk.chunk_id)}/flag`, {
      x: hit.x,
      y: hit.y,
      flagged: nextFlagged,
    });
    setCellFlag(hit.chunk.chunk_id, hit.index, Boolean(result.flagged));
    updateChunkAfterAction(hit.chunk.chunk_id, result);
    chunkDemo.lastAction = `${result.flagged ? "标记" : "取消标记"}：${hit.chunk.chunk_id} (${hit.x},${hit.y})`;
    text("chunk_last_action", chunkDemo.lastAction);
    setBadge("chunk_status", result.flagged ? "已标记" : "已取消标记", "ok");
    addLog("地图标记", chunkDemo.lastAction);
  } catch (error) {
    const message = String(error.message || error);
    setBadge("chunk_status", "操作失败", "bad");
    addLog("地图标记失败", message);
    showNotice(message, "bad");
  } finally {
    chunkDemo.actionPending = false;
    drawChunkCanvas();
  }
}

function resetChunkHover() {
  chunkDemo.hoveredChunk = null;
  chunkDemo.hoveredCell = null;
  updateMapHUD();
}

function updateMapHUD() {
  const bbox = chunkDemo.lastBBox || { min_x: 0, min_y: 0, max_x: 1, max_y: 1 };
  const canvas = $("chunk_canvas");
  const gridSize = chunkGridSize(chunkDemo.maxLevel);
  const cellPixels = canvas ? mapScale(canvas) / gridSize / chunkDemo.size : 0;
  text("chunk_level", `${chunkDemo.level} / ${chunkDemo.maxLevel}`);
  text("chunk_zoom", `${chunkDemo.zoom.toFixed(2)}x / ${cellPixels.toFixed(1)}px 每格`);
  text("chunk_bbox", formatBBox(bbox));
  text("chunk_count", String(chunkDemo.chunks.length));
  text("chunk_id", chunkDemo.hoveredChunk ? chunkDemo.hoveredChunk.chunk_id : "-");
  text("chunk_hover", "cell_x=-, cell_y=-, index=-");
  text("chunk_last_action", chunkDemo.lastAction || "-");
  if (chunkDemo.hoveredCell) {
    const cell = chunkDemo.hoveredCell;
    if (cell.x === undefined) {
      text("chunk_cell_state", `${chunkStateText(cell.chunk.state)} (${cell.chunk.state})`);
      return;
    }
    text("chunk_hover", `cell_x=${cell.x}, cell_y=${cell.y}, index=${cell.index}`);
    if (cell.chunk.closed) {
      text("chunk_cell_state", "Chunk 已封闭 (closed)");
    } else if (cell.opened) {
      text("chunk_cell_state", `已打开，邻雷 ${cell.opened.adjacent_mines || 0} (opened)`);
    } else if (cell.flagged) {
      text("chunk_cell_state", "已标记 (flagged)");
    } else {
      text("chunk_cell_state", "未探索 (hidden)");
    }
    return;
  }
  text("chunk_cell_state", chunkDemo.hoveredChunk ? `${chunkStateText(chunkDemo.hoveredChunk.state)} (${chunkDemo.hoveredChunk.state})` : "-");
}

function scheduleMapLoad() {
  window.clearTimeout(chunkDemo.loadTimer);
  chunkDemo.loadTimer = window.setTimeout(() => {
    loadVisibleChunks().catch((error) => addLog("地图加载失败", String(error.message || error)));
  }, 160);
  drawChunkCanvas();
}

async function startMapView() {
  resizeMapCanvas();
  await loadVisibleChunks();
}

function zoomMapBy(factor, anchorWorld = null) {
  const oldZoom = chunkDemo.zoom;
  const nextZoom = Math.max(1, Math.min(chunkDemo.maxZoom, chunkDemo.zoom * factor));
  if (nextZoom === oldZoom) return;
  if (anchorWorld) {
    chunkDemo.centerX = anchorWorld.x - (anchorWorld.x - chunkDemo.centerX) * (oldZoom / nextZoom);
    chunkDemo.centerY = anchorWorld.y - (anchorWorld.y - chunkDemo.centerY) * (oldZoom / nextZoom);
  }
  chunkDemo.zoom = nextZoom;
  clampMapCenter();
  scheduleMapLoad();
}

function resetMapView() {
  chunkDemo.zoom = 1;
  chunkDemo.centerX = 0.5;
  chunkDemo.centerY = 0.5;
  scheduleMapLoad();
}

async function registerUser() {
  clearNotice();
  setBadge("account_status", "注册中", "warn");
  const payload = {
    user_name: $("user_name").value.trim(),
    nickname: $("nickname").value.trim(),
    password: $("password").value,
  };

  const data = await apiRequest("注册", "/api/v1/users/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  $("login_password").value = $("password").value;
  setBadge("account_status", "注册成功", "ok");
  showNotice("注册成功，可以直接登录大厅。", "ok");
  addLog("注册成功", payload.user_name);
  return data;
}

async function loginUser() {
  clearNotice();
  setBadge("account_status", "登录中", "warn");
  const payload = {
    user_name: $("user_name").value.trim(),
    password: $("login_password").value || $("password").value,
  };

  const data = await apiRequest("登录", "/api/v1/users/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  const user = data.data || {};
  $("token").value = user.token || "";
  syncToken();
  appState.user = user;
  updatePlayerHeader();
  setBadge("account_status", "已登录", "ok");
  showNotice("登录成功，已进入玩家大厅。", "ok");
  addLog("登录成功", `user_id=${user.user_id || "-"}`);
  location.hash = "#/lobby";
}

async function refreshMe() {
  const data = await apiRequest("当前玩家", "/api/v1/me", {
    method: "GET",
    headers: authHeaders(),
  });

  const user = data.data || {};
  appState.user = { ...appState.user, ...user };
  updatePlayerHeader();
  addLog("刷新玩家状态", user.user_name || "成功");
}

async function joinQueue() {
  setBadge("queue_status", "匹配中", "warn");
  const mode = $("queue_mode").value;
  const data = await apiRequest("进入匹配", "/api/v1/match/queue/join", {
    method: "POST",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify({ mode }),
  });

  syncQueueView(data);
  addLog("进入匹配", mode);
  if ((payloadOf(data) || {}).room_id) {
    addLog("匹配成功", "可以进入我的房间");
  }
}

async function refreshQueueStatus() {
  const endpoint = $("status_endpoint").value.trim() || "/api/v1/match/queue/status";
  const data = await apiRequest("匹配状态", endpoint, {
    method: "GET",
    headers: authHeaders(),
  });

  syncQueueView(data);
  addLog("刷新匹配状态", data.message || "成功");
}

async function cancelQueue() {
  const data = await apiRequest("取消匹配", "/api/v1/match/queue/cancel", {
    method: "POST",
    headers: authHeaders(),
  });

  syncQueueView(data);
  addLog("取消匹配", data.message || "成功");
}

function roomBaseURL() {
  return $("room_endpoint").value.trim() || "/api/v1/room/";
}

function roomURL(roomID, suffix = "") {
  const base = roomBaseURL();
  const path = base.endsWith("/")
    ? base + encodeURIComponent(roomID)
    : base + "/" + encodeURIComponent(roomID);
  return path + suffix;
}

function currentRoomID() {
  const roomID = $("room_id").value.trim();
  if (!roomID) throw new Error("请先填写 room_id");
  return roomID;
}

function currentMatchID() {
  const raw = $("match_id").value.trim();
  const matchID = Number(raw);
  if (!raw || !Number.isInteger(matchID) || matchID <= 0) {
    throw new Error("请先填写有效的 match_id");
  }
  return matchID;
}

async function getRoom() {
  const roomID = currentRoomID();
  setBadge("room_status", "查询中", "warn");
  const data = await apiRequest("查看房间", roomURL(roomID), {
    method: "GET",
    headers: authHeaders(),
  });

  syncRoomView(data);
  addLog("查看房间", roomID);
}

async function readyRoom() {
  const roomID = currentRoomID();
  setBadge("room_status", "准备中", "warn");
  const data = await apiRequest("玩家准备", roomURL(roomID, "/ready"), {
    method: "POST",
    headers: authHeaders(),
  });

  setBadge("room_status", "已准备", "ok");
  addLog("玩家准备", roomID);
  await getRoom();
  return data;
}

async function getMatchInfo() {
  const matchID = currentMatchID();
  setBadge("match_status", "查询中", "warn");
  const data = await apiRequest("查询对局", `/api/v1/match/${encodeURIComponent(matchID)}`, {
    method: "GET",
    headers: authHeaders(),
  });

  syncMatchView(data);
  addLog("查询对局", `match_id=${matchID}`);
}

async function submitMatchResult() {
  const matchID = currentMatchID();
  const winTeamNo = Number($("winner_team_no").value);
  setBadge("match_status", "提交中", "warn");

  const data = await apiRequest("提交战绩", "/api/v1/match/result", {
    method: "POST",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify({
      match_id: matchID,
      win_team_no: winTeamNo,
    }),
  });

  setBadge("match_status", "已提交", "ok");
  addLog("提交战绩", `match_id=${matchID}, win_team_no=${winTeamNo}`);
  await getMatchInfo();
  return data;
}

async function getLeaderboard() {
  const mode = $("leaderboard_mode").value;
  const limit = Number($("leaderboard_limit").value || 20);
  setBadge("leaderboard_status", "查询中", "warn");

  const params = new URLSearchParams({ mode, limit: String(limit) });
  const data = await apiRequest("排行榜", `/api/v1/leaderboard?${params.toString()}`, {
    method: "GET",
    headers: authHeaders(),
  });

  syncLeaderboardView(data);
  addLog("刷新排行榜", `${mode}, limit=${limit}`);
  return data;
}

function fillSample() {
  const suffix = Math.random().toString(16).slice(2, 8);
  $("user_name").value = "player_" + suffix;
  $("nickname").value = "玩家_" + suffix;
  $("password").value = "password_" + suffix;
  $("login_password").value = "password_" + suffix;
  setBadge("account_status", "已生成");
}

function useCurrentRoom() {
  const roomID =
    (appState.queue && appState.queue.room_id) ||
    (appState.room && (appState.room.id || appState.room.room_id)) ||
    "";
  if (!roomID) throw new Error("当前没有 room_id");
  $("room_id").value = roomID;
  text("socket_hint", `房间 ${roomID}`);
  addLog("使用当前房间", roomID);
}

function useCurrentMatch() {
  const matchID =
    (appState.queue && appState.queue.match_id) ||
    (appState.room && appState.room.match_id) ||
    (appState.match && (appState.match.id || appState.match.match_id)) ||
    "";
  if (!matchID) throw new Error("当前没有 match_id");
  $("match_id").value = matchID;
  addLog("使用当前对局", `match_id=${matchID}`);
}

function wsURLForRoom(roomID) {
  const base = $("ws_url").value.trim() || "/api/v1/ws/room/";
  const path = base.endsWith("/")
    ? base + encodeURIComponent(roomID)
    : base + "/" + encodeURIComponent(roomID);
  const url = new URL(path, window.location.href);
  url.protocol = window.location.protocol === "https:" ? "wss:" : "ws:";

  const token = rawToken();
  if ($("ws_append_token").checked && token) {
    url.searchParams.set("token", token);
  }
  return url.toString();
}

function connectWS() {
  const roomID = currentRoomID();
  if (appState.ws && appState.ws.readyState === WebSocket.OPEN) {
    throw new Error("WebSocket 已连接");
  }

  const url = wsURLForRoom(roomID);
  const ws = new WebSocket(url);
  appState.ws = ws;
  setBadge("ws_status", "连接中", "warn");
  text("socket_hint", `正在连接房间 ${roomID}`);
  addWSLog("发起连接", roomID);

  ws.onopen = () => {
    setBadge("ws_status", "已连接", "ok");
    text("socket_hint", `已连接房间 ${roomID}`);
    addWSLog("连接成功");
  };

  ws.onmessage = (event) => {
    addWSLog("收到消息", event.data);
    setOutput({ websocket: { event: "message", data: event.data } });
  };

  ws.onerror = () => {
    setBadge("ws_status", "连接错误", "bad");
    addWSLog("连接错误");
  };

  ws.onclose = (event) => {
    setBadge("ws_status", "未连接");
    text("socket_hint", "未连接房间");
    addWSLog("连接关闭", `code=${event.code}`);
    if (appState.ws === ws) appState.ws = null;
  };
}

function sendWSMessage() {
  const ws = appState.ws;
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    throw new Error("WebSocket 未连接");
  }
  const message = $("ws_message").value;
  ws.send(message);
  addWSLog("发送消息", message);
}

function disconnectWS() {
  const ws = appState.ws;
  if (!ws) return;
  ws.close(1000, "client disconnect");
  appState.ws = null;
  setBadge("ws_status", "未连接");
  text("socket_hint", "未连接房间");
  addWSLog("主动断开");
}

function useCurrentRoomForWS() {
  useCurrentRoom();
  addWSLog("同步房间", $("room_id").value.trim());
}

function logout() {
  $("token").value = "";
  syncToken();
  appState.user = null;
  appState.queue = null;
  appState.room = null;
  appState.match = null;
  if (appState.ws) disconnectWS();
  text("metric_status", "未入队");
  text("metric_room_status", "未进入");
  text("metric_match", "-");
  text("metric_room", "-");
  updatePlayerHeader();
  setOutput({});
  addLog("退出登录", "本地登录凭证已清空");
  location.hash = "#/login";
}

function selectMode(mode) {
  $("queue_mode").value = mode;
  text("selected_mode_badge", mode);
  document.querySelectorAll("[data-mode]").forEach((button) => {
    button.classList.toggle("active", button.dataset.mode === mode);
  });
}

function selectBoardMode(mode) {
  $("leaderboard_mode").value = mode;
  document.querySelectorAll("[data-board-mode]").forEach((button) => {
    button.classList.toggle("active", button.dataset.boardMode === mode);
  });
}

function bind(id, fn) {
  const el = $(id);
  if (!el) return;
  el.addEventListener("click", async () => {
    el.disabled = true;
    try {
      clearNotice();
      await fn();
    } catch (error) {
      const message = String(error.message || error);
      setOutput({ error: message });
      showNotice(message, "bad");
      addLog("操作失败", message);
      if (id.includes("queue")) setBadge("queue_status", "失败", "bad");
      if (id.includes("room")) setBadge("room_status", "失败", "bad");
      if (id.includes("match")) setBadge("match_status", "失败", "bad");
      if (id.includes("leaderboard")) setBadge("leaderboard_status", "失败", "bad");
      if (id.includes("login") || id.includes("register")) setBadge("account_status", "失败", "bad");
      if (id.includes("ws")) {
        setBadge("ws_status", "失败", "bad");
        addWSLog("操作失败", message);
      }
    } finally {
      el.disabled = false;
    }
  });
}

function bootstrap() {
  $("token").value = appState.token;
  updatePlayerHeader();

  bind("fill", fillSample);
  bind("register_submit", registerUser);
  bind("login_submit", loginUser);
  bind("me_submit", refreshMe);
  bind("logout_submit", logout);
  bind("join_queue_submit", joinQueue);
  bind("status_queue_submit", refreshQueueStatus);
  bind("cancel_queue_submit", cancelQueue);
  bind("room_submit", getRoom);
  bind("room_ready_submit", readyRoom);
  bind("use_current_room", useCurrentRoom);
  bind("match_submit", getMatchInfo);
  bind("match_result_submit", submitMatchResult);
  bind("use_current_match", useCurrentMatch);
  bind("leaderboard_submit", getLeaderboard);
  bind("leaderboard_quick_submit", getLeaderboard);
  bind("ws_connect_submit", connectWS);
  bind("ws_send_submit", sendWSMessage);
  bind("ws_disconnect_submit", disconnectWS);
  bind("ws_use_current_room", useCurrentRoomForWS);
  bind("map_zoom_in", async () => zoomMapBy(2));
  bind("map_zoom_out", async () => zoomMapBy(1 / 2));
  bind("map_reset", async () => resetMapView());

  document.querySelectorAll("[data-mode]").forEach((button) => {
    button.addEventListener("click", () => selectMode(button.dataset.mode));
  });
  document.querySelectorAll("[data-board-mode]").forEach((button) => {
    button.addEventListener("click", () => selectBoardMode(button.dataset.boardMode));
  });

  const mapCanvas = $("chunk_canvas");
  mapCanvas?.addEventListener("mousemove", (event) => {
    if (chunkDemo.dragStart) {
      const canvas = $("chunk_canvas");
      const ratio = window.devicePixelRatio || 1;
      const scale = mapScale(canvas);
      const dx = event.clientX - chunkDemo.dragStart.clientX;
      const dy = event.clientY - chunkDemo.dragStart.clientY;
      if (Math.hypot(dx, dy) > 4) {
        chunkDemo.dragging = true;
        chunkDemo.dragMoved = true;
        mapCanvas.classList.add("dragging");
        chunkDemo.centerX = chunkDemo.dragStart.centerX - (dx * ratio) / scale;
        chunkDemo.centerY = chunkDemo.dragStart.centerY - (dy * ratio) / scale;
        clampMapCenter();
        scheduleMapLoad();
        return;
      }
    }
    updateChunkHover(event);
  });
  mapCanvas?.addEventListener("mouseleave", resetChunkHover);
  mapCanvas?.addEventListener("mousedown", (event) => {
    if (event.button !== 0) return;
    chunkDemo.dragging = false;
    chunkDemo.dragMoved = false;
    chunkDemo.dragStart = {
      clientX: event.clientX,
      clientY: event.clientY,
      centerX: chunkDemo.centerX,
      centerY: chunkDemo.centerY,
    };
  });
  window.addEventListener("mouseup", (event) => {
    if (!chunkDemo.dragStart) return;
    if (!chunkDemo.dragMoved && event.button === 0) {
      const hit = mapCellHit(event.clientX, event.clientY);
      if (hit?.x !== undefined) {
        openMapCell(hit);
      }
    }
    chunkDemo.dragging = false;
    chunkDemo.dragMoved = false;
    chunkDemo.dragStart = null;
    mapCanvas?.classList.remove("dragging");
  });
  mapCanvas?.addEventListener("contextmenu", (event) => {
    const hit = mapCellHit(event.clientX, event.clientY);
    if (!hit || hit.x === undefined) return;
    event.preventDefault();
    toggleMapFlag(hit);
  });
  mapCanvas?.addEventListener("wheel", (event) => {
    event.preventDefault();
    const anchor = screenToWorld(event.clientX, event.clientY);
    zoomMapBy(event.deltaY < 0 ? 1.5 : 1 / 1.5, anchor);
  }, { passive: false });
  window.addEventListener("resize", () => {
    if (location.hash === "#/map") scheduleMapLoad();
  });

  window.addEventListener("hashchange", () => setRoute(location.hash));
  if (!location.hash) location.hash = appState.token ? "#/lobby" : "#/login";
  setRoute(location.hash);
  setOutput({});
  addLog("玩家大厅已加载", "准备开始演示");
}

bootstrap();
