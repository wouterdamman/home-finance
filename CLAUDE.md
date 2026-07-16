# CLAUDE.md

See `AGENTS.md` for conventions, architecture decisions, dev setup, and known gotchas — that file is the canonical instructions doc for this repo and applies equally here.

Claude-specific addenda:

- Before considering a feature "done," click through it in the browser against real or realistic multi-item data — single-item test fixtures have repeatedly hidden real bugs (layout wrapping, split-percentage edge cases) that only show up with more than one row/category/pot.
- For any page with a mobile fork (see AGENTS.md), test at multiple viewport widths (375/390/430px) and verify desktop is pixel-unchanged at ≥768px — a fix that only gets eyeballed at one width has repeatedly hidden real bugs (e.g. a table column squeezing a badge unreadable only appeared once the date column wrapped at exactly 390px).
