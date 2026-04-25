const $ = (id) => document.getElementById(id);

const storageKey = "go_lobby_token";

const appState = {
  token: localStorage.getItem(storageKey) || "",
  user: null,
  queue: null,
  room: null,
  match: null,
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
  el.textContent = `${new Date().toLocaleTimeString()} · ${title}${detail ? " · " + detail : ""}`;
  $("log").prepend(el);
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

function syncQueueView(data) {
  const payload = data && data.data ? data.data : data;
  if (!payload) return;

  appState.queue = payload;

  const status = payload.status || payload.queue_status || "-";
  const matchID = payload.match_id || "-";
  const roomID = payload.room_id || "-";

  $("metric_status").textContent = status;
  $("metric_match").textContent = matchID;
  $("metric_room").textContent = roomID;

  if (payload.room_id) {
    $("room_id").value = payload.room_id;
  }
  if (payload.match_id) {
    $("match_id").value = payload.match_id;
  }

  const kind = status === "matched" ? "ok" : status === "matching" ? "warn" : "";
  setBadge("queue_status", status, kind);
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
      <strong>队伍 ${teamNo} · ${ready ? "已准备" : "未准备"}</strong>
      <span>${online ? "在线" : "离线"}</span>
    `;
    container.appendChild(card);
  });
}

function syncRoomView(data) {
  const room = data && data.data ? data.data : data;
  if (!room) return;

  appState.room = room;

  $("metric_room").textContent = room.id || room.room_id || "-";
  $("metric_match").textContent = room.match_id || "-";
  $("metric_room_status").textContent = room.status || "-";
  if (room.match_id) {
    $("match_id").value = room.match_id;
  }
  renderRoomPlayers(room);

  const kind = room.status === "playing" ? "ok" : room.status === "waiting" ? "warn" : "";
  setBadge("room_status", room.status || "已获取", kind);
}

function syncMatchView(data) {
  const match = data && data.data ? data.data : data;
  if (!match) return;

  appState.match = match;
  $("metric_match").textContent = match.id || match.match_id || "-";
  if (match.room_id) {
    $("metric_room").textContent = match.room_id;
    $("room_id").value = match.room_id;
  }

  const status = String(match.status ?? "-");
  const kind = status === "2" || match.finished_at ? "ok" : "warn";
  setBadge("match_status", `status=${status}`, kind);
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
  $("session_hint").textContent = `user_id=${user.user_id || "-"} · ${user.user_name || payload.user_name}`;
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
  $("session_user").textContent = user.nickname || user.user_name || "已登录玩家";
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

function currentMatchID() {
  const raw = $("match_id").value.trim();
  const matchID = Number(raw);
  if (!raw || !Number.isInteger(matchID) || matchID <= 0) {
    throw new Error("请先填写有效 match_id，或匹配成功后使用当前对局");
  }
  return matchID;
}

async function getRoom() {
  const roomID = $("room_id").value.trim();
  if (!roomID) {
    throw new Error("请先填写 room_id，或匹配成功后再查询房间");
  }

  setBadge("room_status", "查询中", "warn");
  const data = await apiRequest("房间详情", roomURL(roomID), {
    method: "GET",
    headers: authHeaders(),
  });

  syncRoomView(data);
  addLog("查询房间", roomID);
}

async function readyRoom() {
  const roomID = $("room_id").value.trim();
  if (!roomID) {
    throw new Error("请先填写 room_id，或匹配成功后再准备");
  }

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
  const data = await apiRequest("对局详情", `/api/v1/match/${encodeURIComponent(matchID)}`, {
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
  const roomID = appState.queue && appState.queue.room_id ? appState.queue.room_id : "";
  if (roomID) {
    $("room_id").value = roomID;
    addLog("已填入当前房间", roomID);
    return;
  }

  setOutput({ error: "当前没有 room_id，请先匹配成功或手动填写" });
}

function useCurrentMatch() {
  const matchID =
    (appState.queue && appState.queue.match_id) ||
    (appState.room && appState.room.match_id) ||
    (appState.match && (appState.match.id || appState.match.match_id)) ||
    "";

  if (matchID) {
    $("match_id").value = matchID;
    addLog("已填入当前对局", `match_id=${matchID}`);
    return;
  }

  setOutput({ error: "当前没有 match_id，请先匹配成功或手动填写" });
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
      if (id.includes("login") || id.includes("register")) setBadge("account_status", "失败", "bad");
    } finally {
      button.disabled = false;
    }
  });
}

$("token").value = appState.token;
if (appState.token) {
  $("session_user").textContent = "已保存 token";
  $("session_hint").textContent = "可点击刷新用户验证 token";
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

$("clear_token").addEventListener("click", () => {
  $("token").value = "";
  syncToken();
  appState.user = null;
  $("session_user").textContent = "未登录";
  $("session_hint").textContent = "请先登录玩家账号";
  setOutput({});
  addLog("退出登录", "本地 token 已清空");
});
