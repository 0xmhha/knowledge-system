# Embedding model configuration

The ordinary CLI defaults to Ollama/BGE-M3. Model bytes are external inputs, not bundled repository assets. Exact tag/digest, dimension and query-prefix policy must match the index. Changing these requires a new build identity; an old model experiment does not change the default.

| Backend | Selection | Current boundary |
|---|---|---|
| Ollama | --embedder=ollama --model-name=bge-m3 | Running endpoint and pulled model; public adapter pins identity and probes dimension |
| Mock | --embedder=mock | Deterministic wiring tests, no semantic-quality guarantee |
| ONNX | --embedder=bgeonnx | Optional build tag, native libraries and registered ONNX export; see [preparation](../../docs/vector/d1-installation-guide.md) |
| Direct CoreML adapter | Internal darwin/tokenizers package | Not wired into ordinary CLI dispatch; no --embedder=coreml contract |

The [registry](../../internal/vector/embed/registry) is authoritative for defaults, dimensions, prefixes and export layouts. Registration alone does not mean model files or a working ONNX export exist. See [consumer integration](../../docs/vector/embedder-integration.md) and [remaining work](../../docs/REMAINING-WORK.md).

Use `ckv model --help` for the registered model-management command surface. A command that downloads or converts a model is not a compatibility or retrieval-quality verdict. Preserve actual model/native hashes and measurements in evaluation data.
