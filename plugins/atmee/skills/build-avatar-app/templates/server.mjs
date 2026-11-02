// The backend half of an Atmee avatar app: it holds the API key, starts the
// avatar's conversation and ends it. The browser only ever receives the
// session's LiveKit server URL and user token; avatar-room.js joins with them.
//
//   node --env-file=.env server.mjs     (Node 20.6+; no packages needed)
//
//   ATMEE_API_KEY    an Avatar API key with the `sessions` scope; server-side only
//   AVATAR_ID        the avatar the app talks to (readyToChat, v2)
//   ATMEE_BASE_URL   default https://api.atmanity.us
//   MAX_SECONDS      longest session to ask for, default 300
//   APP_TOOLS_FILE   optional: a JSON file { "instructions": "...", "tools": [...] }, such as app-tools.example.json:
//                    the app tools every session declares (avatar-room.js answers them)
//   PORT             default 3000
//
// It serves this machine only (127.0.0.1, and only to its own pages), since
// the start route spends the key's credits. Port the two /api routes to your
// own backend, behind your own sign-in; everything else here only serves
// try.html for a first live test.
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';

const BASE = process.env.ATMEE_BASE_URL || 'https://api.atmanity.us';
const KEY = process.env.ATMEE_API_KEY;
const AVATAR_ID = process.env.AVATAR_ID;
const MAX_SECONDS = Number(process.env.MAX_SECONDS || 300);
const PORT = Number(process.env.PORT || 3000);
if (!KEY || !AVATAR_ID) throw new Error('Set ATMEE_API_KEY and AVATAR_ID');
// The app's tools, read once; the API checks them on each start (400 invalid_tools).
const APP_TOOLS = process.env.APP_TOOLS_FILE
  ? JSON.parse(await readFile(process.env.APP_TOOLS_FILE, 'utf8'))
  : null;

// Sessions this server started: only these can be ended through it. Each id
// is forgotten once the session can no longer be running.
const started = new Set();
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function avatarApi(method, path, body) {
  const res = await fetch(BASE + path, {
    method,
    headers: { 'X-Api-Key': KEY, 'Content-Type': 'application/json' },
    body: body && JSON.stringify(body),
  });
  const json = await res.json().catch(() => ({}));
  return { status: res.status, json, retryAfter: Number(res.headers.get('retry-after')) || 0 };
}

// POST /api/session. Each call starts a billed session.
async function startSession() {
  const r = await avatarApi('POST', '/v1/session', {
    avatarId: AVATAR_ID,
    maxDurationSeconds: MAX_SECONDS,
    metadata: { app: 'my-avatar-app' },
    // Functions the avatar may call in this session; each call reaches the page.
    ...(APP_TOOLS && { appTools: APP_TOOLS }),
    // visitorId: an unguessable id derived from YOUR user's session, only with
    // a key that has visitor memory; never a value taken from the page.
    // welcome: false starts without the avatar's self-introduction, for an app
    // that has already met the user.
    // checkInAfterS: seconds of silence before it speaks up, 0 for no check-in.
  });
  if (r.status !== 201) {
    // The code is for the page; the message (which limit, which key) is for you.
    console.warn('POST /v1/session -> %d %s %s', r.status, r.json.error || '', r.json.message || '');
    return [r.status, { error: r.json.error || `http_${r.status}`, retryAfter: r.retryAfter }];
  }
  const { sessionId, serverUrl, userToken, actualDurationSeconds } = r.json;
  started.add(sessionId);
  // The join window plus the longest session this server asks for.
  setTimeout(() => started.delete(sessionId), (MAX_SECONDS + 120) * 1000).unref();
  // actualDurationSeconds is absent for an uncapped (metered) session.
  return [201, { sessionId, serverUrl, userToken, maxSeconds: actualDurationSeconds ?? null }];
}

// POST /api/session/<id>/end. Ending twice is safe (alreadyEnded: true), so
// 503 session_unavailable (the room could not be closed yet) and 500
// finalize_failed (not settled yet) are retried.
async function endSession(id) {
  if (!started.has(id)) return [404, { error: 'not_found' }];
  for (let attempt = 1; ; attempt++) {
    const r = await avatarApi('POST', `/v1/session/${id}/end`);
    const retryable = r.status === 503 || r.status === 500;
    if (retryable && attempt < 3) {
      await sleep(Math.min(r.retryAfter || 2, 10) * 1000);
      continue;
    }
    if (r.status === 200) started.delete(id);
    return [r.status, r.json];
  }
}

// Only this machine's own pages: a Host check against DNS rebinding, and an
// Origin check so another site open in the browser can't start sessions.
const OWN = [`localhost:${PORT}`, `127.0.0.1:${PORT}`];
const hostAllowed = (host) => OWN.includes(String(host).toLowerCase());
const originAllowed = (origin) => !origin || OWN.some((own) => origin === `http://${own}`);

const PAGES = {
  '/': ['try.html', 'text/html'],
  '/avatar-room.js': ['avatar-room.js', 'text/javascript'],
  '/floor.mjs': ['floor.mjs', 'text/javascript'],
};

createServer(async (req, res) => {
  const send = (status, body, type = 'application/json') => {
    res.writeHead(status, { 'Content-Type': type });
    res.end(type === 'application/json' ? JSON.stringify(body) : body);
  };
  try {
    if (!hostAllowed(req.headers.host) || !originAllowed(req.headers.origin)) return send(403, { error: 'forbidden' });
    const path = new URL(req.url, 'http://localhost').pathname;
    if (req.method === 'POST' && path === '/api/session') return send(...(await startSession()));
    const end = path.match(/^\/api\/session\/([0-9a-f-]{36})\/end$/);
    if (req.method === 'POST' && end) return send(...(await endSession(end[1])));
    if (req.method === 'GET' && PAGES[path]) {
      const [file, type] = PAGES[path];
      return send(200, await readFile(new URL(file, import.meta.url)), type);
    }
    send(404, { error: 'not_found' });
  } catch (err) {
    console.error(err);
    send(502, { error: 'upstream_failed', message: String(err.message) });
  }
}).listen(PORT, '127.0.0.1', () => console.log(`http://localhost:${PORT}/`));
