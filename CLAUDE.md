# CLAUDE.md

See `AGENTS.md` for conventions, architecture decisions, dev setup, and known gotchas — that file is the canonical instructions doc for this repo and applies equally here.

Claude-specific addenda:

- Before considering a feature "done," click through it in the browser against real or realistic multi-item data — single-item test fixtures have repeatedly hidden real bugs (layout wrapping, split-percentage edge cases) that only show up with more than one row/category/pot.
- For any page with a mobile fork (see AGENTS.md), test at multiple viewport widths (375/390/430px) and verify desktop is pixel-unchanged at ≥768px — a fix that only gets eyeballed at one width has repeatedly hidden real bugs (e.g. a table column squeezing a badge unreadable only appeared once the date column wrapped at exactly 390px).
- When a feature writes data and a *different* feature reads it back (export/import, any serialize-then-deserialize pair), test the actual round trip, not each direction in isolation — checking export's totals/headers looked right missed a scan bug that silently dropped every transaction row for a whole session; only feeding the export back into import surfaced it.
- Before trusting "it's broken" during manual browser testing against local dev servers, verify which process is actually answering the port — a stale docker container or a browser's cached response for an earlier bad server state can produce a confusing "bug" that isn't in the code at all. Check with `lsof`/`curl -D-` and a hard cache-bypass (`fetch(..., {cache:'reload'})` or a fresh incognito-style context) before debugging the app itself.
