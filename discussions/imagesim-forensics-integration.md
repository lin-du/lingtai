# ImageSimilarity → LingTai: a staged, schema-first photo-forensics agent network

**Date:** 2026-06-24
**Status:** Design spec (approved plan v3). In-repo scaffold landed alongside this doc; the addon, the
`ImageSimilarity` scripts, and the kernel catalog entry are build specs for sibling repos.
**Repos touched:** this repo (`lin-du/lingtai`, Go TUI + portal) — docs/examples + a gated Go registration
diff; NEW sibling `lingtai-imagesim` (MCP addon); `JordanLow/ImageSimilarity` (CV pipeline, thin additive
scripts); `lingtai-kernel` (`mcp_catalog.json` one-liner).

## TL;DR

[`JordanLow/ImageSimilarity`](https://github.com/JordanLow/ImageSimilarity) (`run.ipynb`) is the CV engine
behind the *Digital Historical Forensics* study of the *Jinchaji Pictorial* (晋察冀画报, 1942–45): it finds
matching/edited photo pairs across Chinese wartime magazines (YOLO crops upstream → `match.py` cosine
top-15 retrieval → `process_images.py` ASpanFormer + RANSAC geometric verification). We want it to become
one stage of a LingTai agent network, with downstream **humanities-forensics** agents that compare the
original photo vs. the edited magazine version with an MLLM (Gemini 3.1 Pro), **tag the editing techniques**
applied, detect **republication**, hypothesize **editorial/propaganda intent**, and write a brief report.

The pipeline is **not** a stable component you can drop into LingTai today — it is a Colab/Drive-coupled
research notebook with real engineering risks. So the plan is **not** "an agent that runs the notebook" but
a **schema-first evidence factory**: harden the I/O + provenance contract first, insert curation/human
review, wrap as MCP **last**. Two independent analyses (a Claude-Code plan and a `minimax_cn` agent report,
the latter checked into `reports/`) were cross-compared; this doc is the merge.

---

## 1. Why this shape (cross-comparison of two analyses)

Two analyses attacked different halves of the problem and are merged here:

| Dimension | `minimax_cn` report | Claude-Code plan (v1) | Merge |
|---|---|---|---|
| Repo engineering read | **Strong** — cloned `71266e0`; found Colab/Drive coupling, CUDA hardcode, missing deps/weights, **RGB↔grayscale train/infer mismatch**, abs-path/basename-collision JSON, lost match metadata, command drift | Rated it "8/10, clean CLIs" — too rosy | Adopt the risk list; treat as working assumption |
| Build sequencing | **"MCP is step two, not step one"** | Led with the MCP addon | Evidence factory first, MCP last |
| Provenance/audit | sha256, model/weights hash + preprocessing version per record, `audit/run.json` | `run_id` only | Adopt provenance discipline |
| Data contract | Schema-first **JSONL** chain (scales) | Monolithic manifest | JSONL chain |
| Curation/human-in-loop | `default_curation.yml` + `review_queue.jsonl` | Defaulted to `geom_band:high` | Add curation stage |
| Epistemics | Separate **visible fact vs historical hypothesis** | `human_review` field only | Adopt fact-vs-hypothesis split + warnings |
| LingTai mechanics | Conceptual, **unverified** | **Verified against this repo** (addon contract, registration points, schema constraint, LICC) | Keep verified mechanics |
| Watchdog/recovery | Polling daemon idea, no recovery behavior | **Active watchdog** (stall/timeout/failure → alert + recovery) | Keep the watchdog |
| `compare_pair` vs `vision` | "MCP or local API" generic | Justified by the **single-image** limit of built-in `vision` | Dedicated `compare_pair` MCP tool |
| MLLM specifics | General | Gemini 3.1 Pro / Claude Opus 4.8, image & token limits | Keep specifics |

Sources: `reports/image_similarity_agentic_network_report.html` (origin/main `3aa2799`) and
`reports/imagesim_plan_cross_comparison.html` (origin/main `08cabd2`). Four refinements from the latter's
review of v1 are folded in: (a) **diagnose, don't unilaterally refactor** Jordan's repo; (b) **two-layer
taxonomy**; (c) **gate TUI registration on a tested addon**; (d) **copyright check as a pre-build gate** +
MVP gate = *fixture → manifest + compare_pair(mock) → report*.

---

## 2. Architecture

Four parts, each mapped to an existing LingTai idiom:

| Part | What | LingTai idiom | Where it lives |
|---|---|---|---|
| **A** | CV job control + watchdog + pair access + MLLM compare | stdio MCP addon `python -m lingtai_imagesim` (clone of `lingtai-imap`) | **NEW** sibling `lingtai-imagesim` |
| **B** | Headless runner + manifest/page-index emitters | thin additive scripts over existing CLIs | `JordanLow/ImageSimilarity` |
| **C** | Forensic edit-tagging procedure + two-layer taxonomy + report templates | declarative `SKILL.md` | this repo (`examples/forensics/`), shipped with the addon |
| **D** | TUI addon registration + example config/orchestrator | Go one-liners + jsonc examples | this repo |

Topology (steady state):

```
器灵 orchestrator (text LLM; capabilities {file,avatar,daemon,email,psyche,codex}; addons:["imagesim"]; forensic SKILL)
 ├─ mcp.imagesim(submit_job)         → launches headless run_pipeline.py on the GPU runner
 ├─ [addon watchdog / LICC listener] polls job_status.json → mails progress / STALL / TIMEOUT / DONE, wakes agent
 ├─ on DONE: mcp.imagesim(list_pairs{geom_band:"high"}) → shard pairs by magazine issue
 ├─ spawn one analyst 分身 per shard (avatar)
 │     └─ per pair: mcp.imagesim(get_pair) → mcp.imagesim(compare_pair, use_full_pages=true)
 │                  → write mllm/edit_tags.jsonl row → mail headline back
 ├─ collect findings from mailbox/inbox
 └─ synthesis → reports/findings.jsonl + report.md → deliver to operator
```

Why the MLLM compare is a dedicated MCP tool, not the built-in `vision` capability: verified in
`discussions/lingtai-vision-capability-fallback-patch.md` — `vision` is **single-image** (`analyze_image`),
and `materialize_active_preset` wholesale-replaces `manifest.capabilities` on every refresh, so per-agent
two-image wiring via the preset silently vanishes. `compare_pair` owns the file paths (from the manifest)
and the model call, configured by the addon's own config (`LINGTAI_IMAGESIM_CONFIG`).

---

## 3. Guiding principle & gates

**Principle:** build a schema-first evidence factory. Harden the I/O + provenance contract first; insert
curation/human review; do MCP/addon wrapping last. Every record is auditable, reproducible, queueable, and
citable; every claim is labeled fact-vs-hypothesis.

**Pre-build gates:**
- **Copyright / data-rights check** *(before any batch MLLM upload — Phase 4):* confirm the *Jinchaji
  Pictorial* scans may be sent to a third-party MLLM API. Front-load this.
- **MVP success gate** *(end of Phase 4):* on a fixed fixture, reproduce the evidence manifest + a
  `compare_pair` (mock) finding + a `report.md`. Passing this — not "TUI registration done" — defines a
  working MVP.

---

## 4. Phased roadmap

**Phase 0 — Align on facts + diagnose (do NOT refactor Jordan's repo).** Pull 20–50 real sample sets
(crop filename, full page, `output/*.json`, `match_log.json`, OCR/caption/issue metadata). Confirm with
Jordan/春水: canonical meaning of `workingsourcecrops`/`working_targetcrops`/`fullsource`; production
checkpoint; ASpanFormer fork/commit/weights hash; how `BREAKPOINT_VALUE` (50/75) was calibrated; whether
`match.py` grayscale inference is intentional. **File issues** for RGB↔grayscale mismatch, CUDA hardcode,
unmanaged deps/weights, basename overwrite, abs-path JSON, scale — these are owner-approved follow-ups,
**not prerequisites**. Keep `train.py`/`match.py`/`process_images.py` untouched.

**Phase 1 — Thin runner + workspace + audit + heartbeat** *(ImageSimilarity).* `run_pipeline.py` chains
`match.py`/`process_images.py`, owns a `run_id` workspace, writes `audit/run.json` and continuously updates
`job_status.json` (the watchdog heartbeat).

**Phase 2 — Evidence chain (JSONL) + provenance + page index** *(ImageSimilarity).* `make_page_index.py`
(crop filenames → `page_index.json`, regex configurable) and `emit_evidence.py` (legacy outputs → JSONL
contracts with sha256, model/weights hash, preprocessing version, `geom_band`, `republication_groups`).

**Phase 3 — Curation + LingTai keeper + watchdog + Skill** *(LingTai).* `default_curation.yml` →
`curated_pairs.jsonl` + `review_queue.jsonl`. `cv_pipeline_keeper` agent (daemon/avatar) drives
`run_pipeline.py` via the `bash` capability (no MCP yet) and runs the watchdog. Ship the
`image-similarity-pipeline` Skill.

**Phase 4 — MLLM edit-tagger + humanities report** *(LingTai).* Prototype on 10–50 pairs.
`compare_pair(pair_id, use_full_pages=true)` → finding schema; `humanities-report-writer` →
`reports/findings.jsonl` + `report.md`, phrased as evidence leads/hypotheses.

**Phase 5 — MCP addon, then TUI registration (LAST, gated).** 5a: build + unit-test a minimal addon
standalone (mock MLLM, fake runner). 5b: **gated on 5a passing** — apply the TUI registration diff in §7.

---

## 5. Data contracts — JSONL evidence chain (provenance-first)

Core rule: stable `pair_id`; `image_id` + `sha256` per image; every record carries `run_id` + model/weights
hash + preprocessing version. One JSON object per line:

- `audit/run.json` — run params, `model_definition`, weights hash, ASpanFormer commit/weights hash, thresholds.
- `manifest/images.jsonl` — `{image_id, path, sha256, role, publication, issue_date, page, full_page_path?, bbox_xywh?}`.
- `retrieval/candidates.jsonl` — `{run_id, pair_id, source_id, target_id, rank, similarity_score, model_definition, weights_hash, preprocessing_version, self_match_suppressed, error?}`.
- `verification/pairs.jsonl` — `{pair_id, aspan_config, aspan_weights_hash, raw_keypoints, inlier_count, ransac_method, reproj_threshold, breakpoint, geom_band, visualization_ref?}`.
- `curation/curated_pairs.jsonl`, `curation/review_queue.jsonl` — selected pairs + review state.
- `mllm/edit_tags.jsonl` — the finding objects.
- `reports/findings.jsonl` — humanities write-ups, citing artifact ids/hashes.

Derived: `republication_groups` (group high-band pairs by `source.image_id`) — the pre-MLLM "republished
multiple times" signal, independent of the model.

`job_status.json` (the watchdog heartbeat):
```json
{ "schema":"lingtai.imagesim/job-status@1", "job_id":"...", "run_id":"...",
  "state":"queued|running|stalled|done|failed", "stage":"match|verify|emit",
  "items_done":740, "items_total":1284, "pid":12345,
  "started_at":"...", "updated_at":"...", "log_path":"...", "error":null }
```

Forensic finding (`mllm/edit_tags.jsonl`):
```json
{ "pair_id":"...", "run_id":"...", "model":"gemini-3.1-pro",
  "inputs":{"used_full_pages":true,"images_sent":["src_full","tgt_full","src_crop","tgt_crop"]},
  "relationship":{"is_same_base_image":true,"relationship_type":"derivative|republication|non_match|uncertain","confidence":0.86},
  "edit_tags":[{"code":"crop","category":"geometric","confidence":0.92,"evidence":"target omits left margin + lower foreground","visual_basis":"full_page+crop"},
               {"code":"recaption","category":"editorial","confidence":0.81,"evidence":"caption differs across contexts","metadata_needed":true}],
  "negative_tags":["mirror"],
  "republished":{"count":2,"members":["...","..."]},
  "intent_hypothesis":{"label":"propaganda|censorship|aesthetic|space_fit|unknown","rationale":"..."},
  "facts_vs_hypotheses":{"visible_facts":["..."],"historical_hypotheses":["..."]},
  "needs_human_review":true,
  "warning":"Do not claim publication history unless supported by metadata." }
```

### Two-layer taxonomy
- **Layer 1 — visual-edit codes (closed, schema-validated):** geometric (crop, rotate, mirror, vflip,
  scale); compositional (copy-move, splice, inpaint, add); tonal (color, contrast, brightness, tone,
  selective); detail (retouch, sharpen, denoise, blur); editorial (recaption, recontextualize,
  textual-overlay, re-date); advanced (face-alter, deepfake); artifacts (recompress, format-convert); plus
  `partial_overlap`, `print_degradation`, `detail_suppression`. `non_match` and `uncertain` always
  permitted — never force a definite tag. Source of truth: `examples/forensics/edit-taxonomy.json`.
- **Layer 2 — DHF interpretive tags (open-ended, humanities):** `republication/circulation`,
  `recontextualization`, propaganda/censorship/aesthetic intent — the `relationship_type` layer +
  `intent_hypothesis`. Used for explanation, never auto-elevated to a research claim.

---

## 6. Build specs (sibling repos)

### 6A. `lingtai-imagesim` MCP addon (NEW sibling repo)
Clone `lingtai-imap`. Package `lingtai-imagesim`, module `lingtai_imagesim`, entry `python -m
lingtai_imagesim` (stdio MCP server). Reads `LINGTAI_IMAGESIM_CONFIG`.

Tools (all schemas MUST be bare `{"type":"object","properties":…,"required":[…]}` — **no top-level
`oneOf`/`anyOf`/`allOf`/`enum`/`not`**, per `discussions/intrinsics-strict-schema-scan.md`; enforce
per-action required fields in the handler): `submit_job`, `job_status`, `cancel_job`, `list_pairs`,
`get_pair`, `compare_pair`, `list_republications`.

Watchdog = the LICC (LingTai Inbox Callback Contract) listener thread: every `poll_interval` (default 30 s)
read the runner status; track last progress change; write a mailbox message + wake the agent on
`started`/stage transitions/`progress`, **`stalled`** (`last_heartbeat_age_s > stall_threshold` or
`items_done` frozen), **`timeout`** (runtime > `max_runtime`), **`failed`** (process dead, state≠done),
**`done`** (emit normalized manifest then announce "N pairs ready"). Reuse the kernel LICC primitives
(`_make_message`/`inbox.put`/`_wake_nap`) — see `docs/superpowers/plans/2026-04-11-wechat-addon-kernel.md`.

Runner transport (config-driven; `job_status.json` is the universal contract): `local_subprocess`, `ssh`,
or `command` (user-supplied submit/status/cancel templates). `vision_client.py`: Gemini 3.1 Pro primary /
Claude Opus 4.8 fallback; sends 2 images (crops) or 4 (crops + full pages when `use_full_pages`).

```
src/lingtai_imagesim/{__init__,__main__,config,runner,watchdog,manifest,tools,vision_client,prompt}.py
src/lingtai_imagesim/schemas/{pair-manifest,forensic-finding,job-status}.schema.json  edit-taxonomy.json
skills/lingtai-forensics/SKILL.md     tests/{test_manifest,test_tools,test_runner,test_watchdog,test_vision_client}.py
pyproject.toml  README.md  mcp_catalog_entry.json
```

### 6B. `JordanLow/ImageSimilarity` thin scripts (additive only)
No change to `train.py`/`match.py`/`process_images.py`.
- `run_pipeline.py` — chains the CLIs; owns `run_id` workspace; writes `audit/run.json` + continuously
  updates `job_status.json`.
- `make_page_index.py` — parse crop filenames (e.g. `North China Magazine 1939-6-12_Page_002__crop07.jpg`)
  → `page_index.json` (`crop_id → {full_page_path, bbox_xywh?}`); filename regex configurable.
- `emit_evidence.py` — `output/{source}.json` + `visualizations/{source}/match_log.json` + `page_index.json`
  → the JSONL contracts; compute `geom_band` from `inlier_count` vs `aspan_breakpoint` (50) and derive
  `republication_groups`.

### 6C. `lingtai-kernel`
One-line `"imagesim" → lingtai_imagesim` entry in `mcp_catalog.json` (the kernel "decompresses each addon
name from `mcp_catalog.json`"). Separate kernel PR per CLAUDE.md's issue→branch→PR rule.

---

## 7. Phase-5b TUI registration diff (apply ONLY after 6A is published & tested)

> ⚠️ **Gated.** Do not apply until `lingtai-imagesim` exists on the runtime venv and 5a passes. Adding
> `imagesim` to `AllAddons` surfaces it in firstrun/`/mcp` for all users; a missing module would make the
> selection fail at launch. This diff is the ready-to-apply step, intentionally **not** landed in this PR.

1. `tui/internal/tui/presets.go` — `var AllAddons = []string{"imap", "telegram", "feishu", "wechat", "imagesim"}`.
2. `tui/internal/preset/preset.go` — in the addon switch (~line 1332), add:
   ```go
   case "imagesim":
       return "lingtai_imagesim", "LINGTAI_IMAGESIM_CONFIG", filepath.Join(".secrets", "imagesim.json"), true
   ```
3. `tui/internal/preset/templates/imagesim.jsonc` — config template (mirror `templates/imap.jsonc`; see
   `examples/imagesim.jsonc` for the field set).
4. `tui/internal/migrate/m028_addons_to_mcp.go` — *(optional, forward-safety)* add
   `"imagesim": {module:"lingtai_imagesim", envVarName:"LINGTAI_IMAGESIM_CONFIG", defaultRel:".secrets/imagesim.json"}`
   to `addonSpecs` (only matters if a user hand-writes the legacy dict form).
5. `tui/i18n/{en,zh,wen}.json` — add `imagesim` label/help in all three locales (CLAUDE.md three-locale rule).
6. Tests — extend `tui/internal/preset/preset_addons_default_test.go` (and the m028 tests) to include
   `imagesim` in the expected addon set, asserting the `addons:["imagesim"]` + `mcp.imagesim` stdio shape.

No migration bump — `AllAddons` and the `preset.go` switch are additive runtime registration, not on-disk
versioned state.

---

## 8. Agent network flow

1. Operator/器灵 → `mcp.imagesim(submit_job, source_dir, target_dir)` → runner launches headless
   `run_pipeline.py`; `job_status.json` begins updating.
2. Addon watchdog polls; mails progress and, on stall/timeout/failure, a diagnosis + recovery suggestion
   that wakes the agent.
3. On `done`, watchdog runs `emit_evidence.py` (or reads the emitted manifest) and mails "N pairs ready".
4. 器灵 `list_pairs{geom_band:"high"}`, loads the forensic SKILL, shards by `publication+issue_date`.
5. One analyst avatar per shard (file + imagesim addon + SKILL; cheap text LLM — the heavy lifting is in
   `compare_pair`). Mission brief is the avatar's `reasoning`.
6. Each analyst: `get_pair` → `compare_pair(use_full_pages=true)` → write `mllm/edit_tags.jsonl` (merge
   `republished` from the manifest) → molt if context fills.
7. On shard completion the analyst emails 器灵 one headline per pair (atomic-rename mailbox delivery).
8. 器灵 drains inbox, reads all findings.
9. Synthesis: histograms + republication counts → `reports/findings.jsonl` + `report.md`.
10. Deliver the report path to the operator (via the wired channel addon, or the human boundary).

---

## 9. Open questions / risks

1. **Pipeline maturity** — Phase 0 resolves the "8/10 vs fragile" divergence empirically + via Jordan/春水.
2. **Canonical inputs** — `fullsource` real meaning; is copying target crops into source crops a workflow or
   a workaround? (gates full-page support + correctness).
3. **RGB vs grayscale** — confirm/fix before trusting any pair.
4. **Crop filename convention** — `make_page_index.py` regex; bbox may be absent.
5. **Scale** — source×target size + topk decides full score matrix vs FAISS/vector index.
6. **Cost** — thousands of pairs × 2–4 images through Gemini; curation gates; rate/concurrency caps.
7. **Metadata** — where do OCR/journal/date/page live today (recaption/republication need them)?
8. **Direction-of-edit** — CV "source/target" is by directory, not chronology; trust `issue_date`.
9. **Licensing** of the scans before batch-sending to a third-party MLLM.
10. **GPU runner transport** (local/SSH/queue) and where weights/ASpanFormer ckpt live.

---

## 10. Verification

- **Pipeline (ImageSimilarity, P0–2):** assert RGB/grayscale parity across train/inference; run
  `emit_evidence.py` on a fixture and validate each JSONL against its schema (sha256 present, no basename
  collisions, `geom_band` split at the breakpoint, republication grouping); confirm CPU fallback exits
  cleanly without a GPU.
- **Keeper + watchdog (P3):** a fake long job that stops updating `job_status.json` → confirm a STALL
  mailbox alert with a recovery suggestion within `stall_threshold`; a `done` transition announces the count.
- **MLLM (P4):** `compare_pair` on 10–50 real pairs (full pages + crops) → tag codes stay in the closed
  vocabulary, `relationship_type`/`negative_tags`/`needs_human_review` populate, report phrases claims as
  hypotheses; one live Gemini 3.1 Pro call to confirm the wire path.
- **Addon + TUI (P5):** `cd tui && make build`; Go test mirroring `preset_addons_default_test.go` for the
  `imagesim` activation shape; assert `tools.py` schemas are bare `type:object`. Per CLAUDE.md, also
  `cd portal && make build` (shared meta.json version space).

### Critical existing files (reuse / conform to)
- `tui/internal/migrate/m028_addons_to_mcp.go` — authoritative `addons:[list]` + `mcp.<name>` stdio contract.
- `tui/internal/tui/presets.go` (`AllAddons`), `tui/internal/preset/preset.go` (addon switch),
  `tui/internal/preset/templates/imap.jsonc` — registration + template to mirror.
- `examples/imap.jsonc`, `examples/init.jsonc` — config/init shapes to mirror.
- `discussions/intrinsics-strict-schema-scan.md` — bare-object schema constraint.
- `discussions/lingtai-vision-capability-fallback-patch.md` — `vision` is single-image → `compare_pair`.
- `docs/superpowers/plans/2026-04-11-wechat-addon-kernel.md` — canonical addon skeleton + LICC primitives.
- `reports/image_similarity_agentic_network_report.html`, `reports/imagesim_plan_cross_comparison.html` —
  the `minimax_cn` analyses merged here.
