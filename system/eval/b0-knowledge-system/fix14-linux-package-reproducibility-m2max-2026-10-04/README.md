# FIX-14 Linux package reproducibility evidence

2026-10-04. Verified within the current target/toolchain/source fixture; official quality/licensing/operational release remain pending.

`summary.json` defines the scope. Original actual Linux dependency address instability and two unit failure oracles are preserved. Fifteen Python tests pass; their signed fixture suite ran Python only this turn. Actual installed Go verification is covered by both paired archives on both Linux targets. Existing real Darwin binaries retain the previous dependency version/identity metadata; no fresh Darwin package was built in this turn.

`frozen-source-inputs.json` contains 4,624 copy inputs. Each builder compared all bytes/modes before creating its synthetic Git commit. Each actual public packager invocation built all three binaries, and `pair-audit.json` compares both archives, all member paths/bytes/modes/hashes and whole archive SHA. The second package bytes match the first, so metadata/notices are copied once; both archive inventories and both verifier results are preserved. Dependency inspectors repeated on each actual binary match packaged metadata. Builder/runtime images and Go version are pinned in the summary; amd64 is emulated, not native hardware.

Paired builds were allowed to run concurrently as structural/reproducibility checks; this was not a controlled latency experiment. No real model/official quality measurement was performed. Public keys/signatures are temporary test evidence, not operational trust. No private key, executable, archive, database or model is committed. Synthetic installation .log files and raw whitespace are retained unchanged. Later review-document edits do not change the frozen source or tested artifacts.

`evidence-manifest.json` records every retained file except itself. The support/operational/human-review decisions are documented separately and remain required for the full goal.
