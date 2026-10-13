# Host preview package installation

This archive contains ckg, ckv and cks for the operating system and architecture recorded in manifest.json, plus installation instructions, the project license, linked-module notices and policies/sanitization_rules.yaml. It is a host preview, not a signed public release or a certification of other platforms and native embedding libraries.

## Before execution

Obtain the archive and its SHA-256 through a trusted channel and compare the recorded hash before extraction. A hash distributed with the archive alone does not establish independent trust. Public-key/fingerprint and organizational release acceptance remain on hold. Preserve manifest.json, modules.txt, third-party-licenses.json and their notice files. Do not substitute a binary built for another architecture.

Extract into a directory you control. Keep ckg, ckv and cks beside one another so setup can find its companion binaries. Git and the appropriate Go toolchain are needed to index Go projects and run project tests. Real semantic retrieval requires a running Ollama endpoint with the exact selected model; model bytes are not included. ONNX additionally needs matching native libraries and a tagged build. Mock checks wiring only.

## Build and serve a dataset

From the extracted package directory, use absolute source/dataset/config paths outside this archive and keep the dataset outside the source tree. The following example uses BGE-M3 through Ollama:

```sh
./cks init --src /absolute/source --dataset /absolute/dataset \
  --config-out /absolute/setup.yaml --embedder ollama --model-name bge-m3
./cks setup --config /absolute/setup.yaml --version preview-001
./cks mcp gen-config --dataset-dir /absolute/dataset --source-root /absolute/source \
  --out /absolute/cks.yaml --sanitize-rules ./policies/sanitization_rules.yaml
./cks mcp --config /absolute/cks.yaml
```

The init command creates a stable project ID once and refuses to overwrite existing config. Retain that setup config for later versions. Check command help for required model/endpoint, source mode, capture limits and project inputs. The model identity used to serve must match the built index. The generated config serves HTTP on loopback by default; use `cks mcp client-config --config /absolute/cks.yaml` for the client entry. Have the client call cks.ops.health and the appropriate context tool. A configured real backend failure must not be interpreted as successful mock retrieval.

## Upgrade and recovery boundaries

Keep the old package and dataset versions. Build a new version name with the same project config; failed gates leave current unchanged. Existing servers keep their pinned version until restart. Review retained versions before applying garbage collection. A backup must preserve the selected dataset, retained source blobs/history, configs, knowledge inputs and exact model/binary identity. Test actual restore and query behavior before relying on it.

Ontology retrieval, knowledge attachment and type-neighbor insertion retain their explicit/default-off boundaries. Installation success does not approve content, independent answer quality, performance, organization trust or public deployment.

The current repository references are [setup](https://github.com/0xmhha/knowledge-system/blob/main/system/docs/SETUP.md), [release policy](https://github.com/0xmhha/knowledge-system/blob/main/docs/spec-driven/RELEASE-POLICY.md) and [remaining work](https://github.com/0xmhha/knowledge-system/blob/main/docs/REMAINING-WORK.md). For reproducibility, resolve these at the package's recorded commit rather than assuming later main describes the original build.
