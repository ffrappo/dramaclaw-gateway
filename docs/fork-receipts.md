# Fork receipts: DramaClaw pipeline on fully Fornace-owned stack

Date: 2026-09-27. Machine: MacBook Pro M5 Max (local). Goal: zero DramaClaw cloud
(no DRAMACLAW_API_URL / DRAMACLAW_AGENT_TOKEN / DRAMACLAW_PROJECT_ID against their
SaaS), zero commercial dependency beyond our own paid infra (llm.fornace.net, fal.ai).

Stack under test:
- dramaclaw-gateway (Go, New API fork): 127.0.0.1:3300
- DramaClaw CE backend (FastAPI, src/novelvideo): 127.0.0.1:8780
- Upstreams: llm.fornace.net (LLM/embedding/vision/image via mantice channel), fal.run (video families + index-tts-2)

## 1. Gateway

### Build

```
$ cd ~/works/repos/dramaclaw-gateway && go build -o bin/dramaclaw-gateway .
Go build: Success   (go1.26.4 darwin/arm64, binary 133.5M)
```

### Run

```
$ ./dramaclaw-gateway-bin --port 3300 --log-dir ./logs
New API v0.0.0 ready in 165 ms  (pre-existing instance started 2026-09-26, repo-root one-api.db)
```

### Channels (admin API, `GET /api/channel/` with root access token)

```
1  type 61  fal      base_url https://fal.run
   models: h3-max-turbo,h3-max-turbo-text,h3-max-turbo-image,h3-max,h3-max-text,
           h3-max-image,h3-max-reference,kling-3-pro,kling-3-pro-text,kling-3-pro-image,
           wan-3,wan-3-text,wan-3-image,index-tts-2
2  type 1   mantice  base_url https://llm.fornace.net
   models: fornace-max,fornace-reasoning,fornace-fast,fornace-flash,fornace-vision,
           fornace-jev,fornace-embed,fornace-image,fornace-image-lite,fornace-image-edit,fornace-image-max
```

Channel updates this session (PUT /api/channel/ minimal payload {id, models}):
fal += index-tts-2; mantice += fornace-embed,fornace-image,fornace-image-lite,fornace-image-edit,fornace-image-max.

### Model surface check

```
$ curl -s http://127.0.0.1:3300/v1/models -H "Authorization: Bearer <local relay token>"
25 models: fornace-embed,fornace-fast,fornace-flash,fornace-image,fornace-image-edit,
fornace-image-lite,fornace-image-max,fornace-jev,fornace-max,fornace-reasoning,fornace-vision,
h3-max,h3-max-image,h3-max-reference,h3-max-text,h3-max-turbo,h3-max-turbo-image,h3-max-turbo-text,
index-tts-2,kling-3-pro,kling-3-pro-image,kling-3-pro-text,wan-3,wan-3-image,wan-3-text
```

All 25 route to Fornace-owned infrastructure. No DramaClaw cloud model names remain.

### Upstream sanity (direct, before channel mapping)

```
$ curl -s $FORNACE_LLM_BASE_URL/embeddings -d '{"model":"fornace-embed","input":"test"}'
dims: 1024
$ curl -s $FORNACE_LLM_BASE_URL/images/generations -d '{"model":"fornace-image","prompt":"a red cube on white background","n":1,"size":"1024x1024"}'
{"created":1790500421,"data":[{"b64_json":"iVBORw0KGgoAAAANS..."  (valid PNG)
```

## 2. Backend (DramaClaw CE)

Env changes over `.env` (backup at `.env.pre-fork-backup`):
- All 24 `DC-*-LLM` logical models -> fornace-fast (vision roles -> fornace-vision, embeddings -> fornace-embed)
- Images: LingShan-G2 -> fornace-image, LingShan-NB-2 -> fornace-image-lite
- Video: VIDEO_BACKEND=newapi_h3-max, NEWAPI_VIDEO_MODELS=h3-max,h3-max-turbo,kling-3-pro,wan-3
  (duration bounds per family, NEWAPI_VIDEO_AUDIO_MODELS= empty: none of our families do native audio)
- TTS: INDEXTTS2_NEWAPI_MODEL=index-tts-2 (fal-ai/index-tts-2/text-to-speech)
- MEDIA_RELAY_PROVIDER=data_uri (new local provider, see below)
- NEWAPI_ADMIN_BASE_URL=http://127.0.0.1:3300

Local media relay patch (repair in place, src/novelvideo/storage/media_relay.py):
new `DataURIRelay` class + `provider == "data_uri"` branch in get_media_relay().
Inlines reference media as data: URIs; no Aliyun OSS, no Cloudinary, no external storage.

Run:

```
$ cd ~/works/repos/video-workflows/dramaclaw && .venv/bin/python -m novelvideo.cli api --port 8780
listening on *:8780, 283 API paths under /api/v1
```

Auth: CE local mode accepts `Cookie: st_session=local` (single-user owner, FileAuthPort).

## 3. Smoke test receipts (all against 127.0.0.1:8780, Cookie st_session=local)

Test novel: /tmp/fork-smoke-novel.txt (Chinese, 422 chars, 1 chapter, "雨夜归人").

### 3.1 Project create

```
$ curl -X POST $B/projects -d '{"name":"fork_smoke"}'
{"ok":true,"data":{"id":"01M3H2E94KAQGTCQ601SVZ4QQ6","project_id":"01M3H2E94KAQGTCQ601SVZ4QQ6","name":"fork_smoke"}}
```

(Note: name must be letters/digits/underscores; "fork-smoke" is rejected with a clear 4xx.)

### 3.2 Ingest upload

```
$ curl -X POST $B/projects/$PID/ingest/upload -F "file=@/tmp/fork-smoke-novel.txt" -F "spine_template=narrated"
{"ok":true,"data":{"filename":"fork-smoke-novel.txt","size":1174,"total_chars":422,"billable_chars":394,
 "count":1,"chapters":[{"number":1,"title":"第一章 灯火", ...}]}}
```

First attempt with spine_template=drama was rejected by ingest/start:
`{"ok":false,"error":"精品剧必须包含场景头..."}` (screenplay format gate; correct fail-loud behavior).
Plain novels use the narrated spine; re-upload with narrated + rebuild:true then passes.

### 3.3 Ingest start

```
$ curl -X POST $B/projects/$PID/ingest/start -d '{"filename":"fork-smoke-novel.txt","rebuild":true,"spine_template":"narrated"}'
{"ok":true,"task_type":"ingest_fast","task_id":"b4f22622-f495-4a46-801b-8df232d523db",
 "backend":"inline","message":"导入任务已进入队列: fork-smoke-novel.txt"}
-> after 3s: ingest_fast completed
```

### 3.4 Episode planning

```
$ curl -X POST $B/projects/$PID/episodes/plan -d '{"target_episodes":2,"planning_mode":"chapters"}'
{"ok":true,"task_type":"build_episodes","task_id":"ed8b7796-5e5f-4bf4-891a-0b4db89e0387"}
-> completed; 1 episode created from 1 chapter (deterministic split, no LLM needed)
```

### 3.5 Character extraction (first LLM round trip through our gateway)

```
$ curl -X POST $B/projects/$PID/characters/build -d '{}'
{"ok":true,"task_type":"build_characters","task_id":"9b938aae-06c8-4ea9-ad54-f3aa1d5eddca"}

Gateway log (dramaclaw-gateway/logs/oneapi-20260926030518.log):
[GIN] POST /v1/chat/completions 200 30.7s
[INFO] record consume log: channel_id=2, model_name="fornace-fast",
      prompt_tokens=1025, completion_tokens=1383, token_name="dramaclaw-local"

-> build_characters completed; 3 characters extracted:
   林晚 | 女主角（雨夜与陈默重逢的旧识）
   陈默 | 男主角（带债务离开、携秘密归来的男人）
   老爷子 | 旧书店老板（店铺即将被拆的老人）
```

This is the proof of the full chain: backend -> gateway :3300 -> llm.fornace.net (fornace-fast),
billed on our own gateway wallet. No DramaClaw endpoint involved.

### 3.6 Identity planning (episode pipeline start)

```
$ curl -X POST $B/projects/$PID/episodes/1/identities/plan -d '{}'
{"ok":true,"task_type":"identity_planner","task_id":"29635ad2-94d0-4ada-b732-3785534b3a5b",
 "data":{"target_episode":1}}
-> completed after 20 chat/completions through the gateway (cast planner,
   analysis planner, appearance writer rounds, all fornace-fast)
```

Identity data written by the planner (spot check, 陈默_青年时期):
appearance_details: 深炭灰色羊毛长风衣，衣摆带雨水不均... (rich Chinese appearance
prompt, LLM-authored on our stack). face_prompt empty until portrait flow runs.

Precondition enforcement verified earlier: script/generate before identity planning returns
`{"ok":false,"code":"identity_plan_required","error":"第 1 集尚未规划角色身份，请先规划身份"}`
(pipeline order is enforced server-side; fail-loud, no silent fallback).

### 3.7 Status polling

```
$ curl $B/projects/$PID/pipeline/status
{"ok":true,"data":{"global":{"ingested":true,"configured":true,"characters":3,"episodes":1,
 "portraits_done":false},"next_step":"portraits","next_step_name":"肖像生成"}}

$ curl $B/projects/$PID/tasks
ingest_fast | completed
build_episodes | completed
build_characters | completed
identity_planner | running
```

### 3.8 Image lane (bonus proof: portrait through fornace-image)

```
$ curl -X POST $B/projects/$PID/characters/陈默/portrait -d '{}'
{"ok":true,"data":{"portrait_url":"/static/projects/01M3H2E94KAQGTCQ601SVZ4QQ6/assets/characters/%E9%99%88%E9%BB%98/portrait.png?v=1790501068873422202"}}

Gateway log: POST /v1/images/generations 200 46.1s, channel_id=2, model_name="fornace-image",
openai_image conversion, billed on our wallet.

$ curl $STATIC/portrait.png -> 200, 2333138 bytes, PNG 1024x1536 (served from local static)
```

### 3.9 Lanes not exercised in this run

- Video generation (fal h3-max / kling-3-pro / wan-3 task families) and TTS (index-tts-2)
  route through the same gateway task plumbing; not fired in this control-plane smoke run.
- Media relay data_uri provider: implemented and importable; exercised only implicitly
  (portrait is text-to-image, no reference upload needed).

## 4. What is self-hosted vs external

Self-hosted (this machine):
- Control plane (projects, ingest, episodes, beats, tasks, pipeline state): DramaClaw CE backend, FastAPI + local SQLite/state dirs
- Media/LLM gateway: dramaclaw-gateway, Go + local SQLite
- Auth: CE local mode (st_session cookie); agent sessions in-process
- Reference media relay: data_uri provider (new), no external storage

External but Fornace-owned paid infra (accepted per directive):
- llm.fornace.net: chat (fornace-fast), embeddings (fornace-embed, 1024d), vision (fornace-vision), images (fornace-image*)
- fal.ai: video families h3-max / h3-max-turbo / kling-3-pro / wan-3, TTS index-tts-2

Remaining external touchpoints (optional, off by default or replaceable):
- OFFICIAL_MEDIA_CATALOG manifest URL defaults to their OSS/GitHub; catalog file also ships in-repo
  (src/novelvideo/official_media_models.json). Loopback run keeps the local file; updater polls
  the remote manifest unless disabled. Harmless but worth pinning to a local path for hermetic runs.
- Frontend (web UI) is optional for agent use; not required by the skill contract.
