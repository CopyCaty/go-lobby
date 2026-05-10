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
  "#/debug": {
    kicker: "Debug",
    title: "开发调试",
    desc: "查看 token、接口地址、原始响应和操作日志。",
  },
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

  document.querySelectorAll("[data-mode]").forEach((button) => {
    button.addEventListener("click", () => selectMode(button.dataset.mode));
  });
  document.querySelectorAll("[data-board-mode]").forEach((button) => {
    button.addEventListener("click", () => selectBoardMode(button.dataset.boardMode));
  });

  window.addEventListener("hashchange", () => setRoute(location.hash));
  if (!location.hash) location.hash = appState.token ? "#/lobby" : "#/login";
  setRoute(location.hash);
  setOutput({});
  addLog("玩家大厅已加载", "准备开始演示");
}

bootstrap();
