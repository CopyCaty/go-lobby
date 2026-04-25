const $ = (id) => document.getElementById(id);

const storageKey = "go_lobby_token";

const appState = {
  token: localStorage.getItem(storageKey) || "",
  user: null,
  queue: null,
  room: null,
  match: null,
  ws: null,
};

function pretty(value) {
  try {
    return JSON.stringify(value, null, 2);
  } catch (error) {
    return String(value);
  }
}

function setOutput(value) {
  $("output").textContent = pretty(value);
}

function setBadge(id, text, kind = "") {
  const el = $(id);
  el.textContent = text;
  el.className = "badge" + (kind ? " " + kind : "");
}

function addLog(title, detail = "") {
  const el = document.createElement("div");
  el.className = "log-item";
  el.textContent = `${new Date().toLocaleTimeString()} - ${title}${detail ? " - " + detail : ""}`;
  $("log").prepend(el);
}

function addWSLog(title, detail = "") {
  const el = document.createElement("div");
  el.className = "log-item";
  el.textContent = `${new Date().toLocaleTimeString()} - ${title}${detail ? " - " + detail : ""}`;
  $("ws_log").prepend(el);
}

function syncToken() {
  const token = $("token").value.trim();
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
  const text = await resp.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch (error) {
    data = { raw: text };
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

function payloadOf(data) {
  return data && data.data ? data.data : data;
}

function syncQueueView(data) {
  const payload = payloadOf(data);
  if (!payload) return;

  appState.queue = payload;
  const status = payload.status || payload.queue_status || "-";
  const matchID = payload.match_id || "-";
  const roomID = payload.room_id || "-";

  $("metric_status").textContent = status;
  $("metric_match").textContent = matchID;
  $("metric_room").textContent = roomID;

  if (payload.room_id) $("room_id").value = payload.room_id;
  if (payload.match_id) $("match_id").value = payload.match_id;

  setBadge("queue_status", status, status === "matched" ? "ok" : status === "matching" ? "warn" : "");
}

function renderRoomPlayers(room) {
  const container = $("room_players");
  container.innerHTML = "";

  const players = room.players || room.Players || {};
  Object.values(players).forEach((player) => {
    const card = document.createElement("div");
    card.className = "player";

    const userID = player.user_id ?? player.UserID ?? "-";
    const teamNo = player.team_no ?? player.TeamNo ?? "-";
    const ready = player.ready ?? player.Ready ?? false;
    const online = player.online ?? player.Online ?? false;

    card.innerHTML = `
      <span>玩家 ${userID}</span>
      <strong>队伍 ${teamNo} - ${ready ? "已准备" : "未准备"}</strong>
      <span>${online ? "在线" : "离线"}</span>
    `;
    container.appendChild(card);
  });
}

function syncRoomView(data) {
  const room = payloadOf(data);
  if (!room) return;

  appState.room = room;
  $("metric_room").textContent = room.id || room.room_id || "-";
  $("metric_match").textContent = room.match_id || "-";
  $("metric_room_status").textContent = room.status || "-";

  if (room.id || room.room_id) $("room_id").value = room.id || room.room_id;
  if (room.match_id) $("match_id").value = room.match_id;

  renderRoomPlayers(room);
  setBadge("room_status", room.status || "已加载", room.status === "playing" ? "ok" : "warn");
}

function syncMatchView(data) {
  const match = payloadOf(data);
  if (!match) return;

  appState.match = match;
  $("metric_match").textContent = match.id || match.match_id || "-";
  if (match.room_id) {
    $("metric_room").textContent = match.room_id;
    $("room_id").value = match.room_id;
  }

  const status = String(match.status ?? "-");
  setBadge("match_status", `status=${status}`, status === "2" || match.finished_at ? "ok" : "warn");
}

async function registerUser() {
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
  addLog("注册成功", payload.user_name);
  return data;
}

async function loginUser() {
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
  $("session_user").textContent = user.nickname || user.user_name || payload.user_name;
  $("session_hint").textContent = `user_id=${user.user_id || "-"} - ${user.user_name || payload.user_name}`;
  setBadge("account_status", "登录成功", "ok");
  addLog("登录成功", `user_id=${user.user_id || "-"}`);
}

async function refreshMe() {
  const data = await apiRequest("当前用户", "/api/v1/me", {
    method: "GET",
    headers: authHeaders(),
  });

  const user = data.data || {};
  appState.user = { ...appState.user, ...user };
  $("session_user").textContent = user.nickname || user.user_name || "已登录";
  $("session_hint").textContent = user.user_name ? `user_name=${user.user_name}` : "token 有效";
  addLog("刷新用户", user.user_name || "成功");
}

async function joinQueue() {
  setBadge("queue_status", "入队中", "warn");
  const mode = $("queue_mode").value;
  const data = await apiRequest("加入匹配", "/api/v1/match/queue/join", {
    method: "POST",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify({ mode }),
  });

  syncQueueView(data);
  addLog("加入匹配", mode);
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
  const data = await apiRequest("查询房间", roomURL(roomID), {
    method: "GET",
    headers: authHeaders(),
  });

  syncRoomView(data);
  addLog("查询房间", roomID);
}

async function readyRoom() {
  const roomID = currentRoomID();
  setBadge("room_status", "准备中", "warn");
  const data = await apiRequest("玩家准备", roomURL(roomID, "/ready"), {
    method: "POST",
    headers: authHeaders(),
  });

  addLog("玩家准备", roomID);
  setBadge("room_status", "已准备", "ok");
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

  const data = await apiRequest("提交对局结果", "/api/v1/match/result", {
    method: "POST",
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify({
      match_id: matchID,
      win_team_no: winTeamNo,
    }),
  });

  setBadge("match_status", "已提交", "ok");
  addLog("提交对局结果", `match_id=${matchID}, win_team_no=${winTeamNo}`);
  await getMatchInfo();
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
  addWSLog("发起连接", url);

  ws.onopen = () => {
    setBadge("ws_status", "已连接", "ok");
    addWSLog("连接成功");
  };

  ws.onmessage = (event) => {
    addWSLog("收到消息", event.data);
    setOutput({
      websocket: {
        event: "message",
        data: event.data,
      },
    });
  };

  ws.onerror = () => {
    setBadge("ws_status", "连接错误", "bad");
    addWSLog("连接错误");
  };

  ws.onclose = (event) => {
    setBadge("ws_status", "未连接");
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
  addWSLog("主动断开");
}

function useCurrentRoomForWS() {
  useCurrentRoom();
  addWSLog("已选择房间", $("room_id").value.trim());
}

function bind(id, fn) {
  $(id).addEventListener("click", async () => {
    const button = $(id);
    button.disabled = true;
    try {
      await fn();
    } catch (error) {
      const message = String(error.message || error);
      setOutput({ error: message });
      addLog("操作失败", message);
      if (id.includes("queue")) setBadge("queue_status", "失败", "bad");
      if (id.includes("room")) setBadge("room_status", "失败", "bad");
      if (id.includes("match")) setBadge("match_status", "失败", "bad");
      if (id.includes("login") || id.includes("register")) setBadge("account_status", "失败", "bad");
      if (id.includes("ws")) {
        setBadge("ws_status", "失败", "bad");
        addWSLog("操作失败", message);
      }
    } finally {
      button.disabled = false;
    }
  });
}

$("token").value = appState.token;
if (appState.token) {
  $("session_user").textContent = "已保存 token";
  $("session_hint").textContent = "点击刷新用户验证 token";
}

bind("fill", fillSample);
bind("register_submit", registerUser);
bind("login_submit", loginUser);
bind("me_submit", refreshMe);
bind("join_queue_submit", joinQueue);
bind("status_queue_submit", refreshQueueStatus);
bind("cancel_queue_submit", cancelQueue);
bind("room_submit", getRoom);
bind("room_ready_submit", readyRoom);
bind("use_current_room", useCurrentRoom);
bind("match_submit", getMatchInfo);
bind("match_result_submit", submitMatchResult);
bind("use_current_match", useCurrentMatch);
bind("ws_connect_submit", connectWS);
bind("ws_send_submit", sendWSMessage);
bind("ws_disconnect_submit", disconnectWS);
bind("ws_use_current_room", useCurrentRoomForWS);

$("clear_token").addEventListener("click", () => {
  $("token").value = "";
  syncToken();
  appState.user = null;
  $("session_user").textContent = "未登录";
  $("session_hint").textContent = "请先登录，受保护接口会使用 token。";
  setOutput({});
  addLog("清空 token", "本地 token 已移除");
});
