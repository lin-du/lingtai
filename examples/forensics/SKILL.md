---
name: image-similarity-pipeline
description: >
  Drive the LingTai photo-forensics network over the ImageSimilarity CV pipeline: trigger a matching run
  and supervise it via the imagesim addon's watchdog, then for each verified source↔target image pair use an
  MLLM (Gemini 3.1 Pro) to compare the original photo against the edited magazine version, tag the editing
  techniques applied against a two-layer taxonomy, detect republication, hypothesize editorial intent, and
  synthesize a brief humanities report. Built for the Jinchaji Pictorial (晋察冀画报) Digital Historical
  Forensics study but generic over any "original vs. republished/edited photo" corpus.
version: 0.1.0
tags: [forensics, computer-vision, mllm, image-comparison, digital-humanities, imagesim]
---

# Image-Similarity Forensics Pipeline

You orchestrate a CV pipeline + downstream MLLM forensics over pairs of historical photographs. The CV
pipeline (`JordanLow/ImageSimilarity`) finds candidate matching pairs; your job is to turn each verified
pair into an auditable forensic finding and a readable report.

**This is a draft procedure shipped as a design example.** It assumes the `imagesim` MCP addon
(`lingtai-imagesim`) is wired in. Until the addon exists, treat the `mcp.imagesim(...)` calls below as the
target contract and run the steps manually / via the `bash`-driven `cv_pipeline_keeper` (see
`examples/cv-keeper.init.json`). The full design is in
`discussions/imagesim-forensics-integration.md`.

## Core discipline (read first)

1. **CV and MLLM outputs are evidence leads, not conclusions.** Every finding separates *visible visual
   facts* from *historical interpretive hypotheses*. Never auto-elevate a hypothesis to a research claim;
   that is a human's call. Set `needs_human_review` and carry a `warning` when metadata is missing.
2. **Never force a tag.** `non_match` and `uncertain` are always allowed. Low-resolution print, scan
   degradation, and caption ambiguity are real gray zones — do not flatten them.
3. **Direction of edit comes from chronology, not directory.** The CV "source/target" split is by folder.
   Use `issue_date` to decide which is the original; let the model flag anomalies, but dates win on conflict.
4. **Provenance or it didn't happen.** Cite `pair_id`, `image_id`/`sha256`, `run_id`, and artifact paths —
   never paste large JSON blobs into mail or reports.

## When to use

- A human asks to run image-matching over a magazine corpus and analyze how photos were edited/republished.
- A `job_status.json` `done` notification (or the keeper's "N pairs ready" mail) arrives in your inbox.

## Procedure

### 1. Trigger or ingest
- New run: `mcp.imagesim(submit_job, {source_dir, target_dir, run_label})` → `{job_id, run_id}`.
- The addon watchdog mails you progress and, on `stalled`/`timeout`/`failed`, a diagnosis + recovery
  suggestion. **Act on recovery suggestions** (inspect `log_tail`, resubmit with a smaller batch, verify
  weights) rather than waiting indefinitely.
- Already computed: skip to step 2 with the existing `run_id`.

### 2. Select the work set (curation)
- `mcp.imagesim(list_pairs, {geom_band:"high"})`. Default to **high-band only** (ASpanFormer inliers ≥
  breakpoint); low-band are likely false matches. Apply rank/score gates and dedup. Cost-gate: thousands of
  pairs × 2–4 images is real spend — confirm the corpus size and any human-review sampling before fanning out.

### 3. Shard & fan out
- Group pairs by `source.publication + issue_date`. Spawn **one analyst avatar (分身) per shard** (not per
  pair). Give each a cheap text LLM preset — the heavy vision work is inside `compare_pair`. The avatar's
  mission (`reasoning`): analyze its shard's pairs, write one finding row per pair, mail back one headline
  per pair when done.

### 4. Per-pair analysis (inside each analyst)
For each `pair_id`:
- `mcp.imagesim(get_pair, {pair_id})` → resolved crop + full-page paths + CV metadata.
- `mcp.imagesim(compare_pair, {pair_id, use_full_pages:true})`. With full pages available the model sees
  both full magazine pages (crop-bbox overlaid) **and** both crops — essential for `recaption`,
  `recontextualize`, `re-date`, and layout edits that a crop alone can't reveal.
- Validate the returned finding: every `edit_tags[].code` must be in the Layer-1 closed set (below); merge
  `republished` from the manifest's `republication_groups`; ensure `facts_vs_hypotheses` is populated.
- Append the finding to `mllm/edit_tags.jsonl`. Molt (凝蜕) if context fills, carrying only "shard X: K/M done".

### 5. Report-back & synthesis
- Each analyst emails the orchestrator one headline per pair on shard completion.
- The orchestrator drains its inbox, reads all findings, computes the corpus roll-up (edit-code histogram,
  intent histogram, republication count), and writes `reports/findings.jsonl` + `report.md` — per-pair
  sections + a corpus summary. Phrase everything as evidence leads/hypotheses pending human review.

## Two-layer taxonomy

**Layer 1 — visual-edit codes (closed; the only values allowed in `edit_tags[].code`).** Authoritative list:
`examples/forensics/edit-taxonomy.json`. Buckets: geometric (crop, rotate, mirror, vflip, scale,
partial_overlap); compositional (copy-move, splice, inpaint, add); tonal (color, contrast, brightness, tone,
selective); detail (retouch, sharpen, denoise, blur); editorial (recaption, recontextualize,
textual-overlay, re-date); advanced (face-alter, deepfake); artifacts (recompress, format-convert,
print_degradation, detail_suppression). Plus the escape hatches `non_match` and `uncertain`.

**Layer 2 — DHF interpretive tags (open-ended; for explanation only).** `relationship_type`
(derivative / republication / non_match / uncertain) and `intent_hypothesis.label`
(propaganda / censorship / aesthetic / space_fit / unknown), with a free-text `rationale`. Never validated
against a closed list, never auto-elevated to a claim.

## Output schema (one row per pair in `mllm/edit_tags.jsonl`)

```json
{ "pair_id":"...", "run_id":"...", "model":"gemini-3.1-pro",
  "inputs":{"used_full_pages":true,"images_sent":["src_full","tgt_full","src_crop","tgt_crop"]},
  "relationship":{"is_same_base_image":true,"relationship_type":"derivative","confidence":0.86},
  "edit_tags":[{"code":"crop","category":"geometric","confidence":0.92,"evidence":"target omits left margin + lower foreground","visual_basis":"full_page+crop"}],
  "negative_tags":["mirror"],
  "republished":{"count":2,"members":["...","..."]},
  "intent_hypothesis":{"label":"propaganda","rationale":"cropping secondary figures + adding a unit banner refocuses on a single heroic subject"},
  "facts_vs_hypotheses":{"visible_facts":["..."],"historical_hypotheses":["..."]},
  "needs_human_review":true,
  "warning":"Do not claim publication history unless supported by metadata." }
```
