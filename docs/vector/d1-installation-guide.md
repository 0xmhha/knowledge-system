# Optional ONNX runtime preparation

The default is Ollama/BGE-M3. This guide is for the optional in-process ONNX adapter; it is not a native installation requirement for the default backend or a claim of production acceptance.

## Dependency and model contract

- Enable CGO and the bgeonnx build tag. The Go dependency pins are in [go.mod](../../go.mod); the native tokenizer must satisfy the C ABI required by that pinned Go wrapper.
- Prepare libonnxruntime and libtokenizers for the target architecture. CGO_LDFLAGS supplies the tokenizer library search path; CKV_ONNXRUNTIME_LIB supplies the ONNX shared-library path. Mere file presence does not prove compatible symbols or successful inference.
- Use a model registered with a nonempty ONNX export in [the model registry](../../internal/vector/embed/registry). Prepare its exact ONNX and tokenizer files as required by [model_config.go](../../internal/vector/embed/bgeonnx/model_config.go). A registry entry without an ONNX export is rejected.
- Record library/model hashes, architecture, Go/compiler versions and inference results. Match the same embedding identity in every query and index build. Old BGE-large timings are historical measurements and are not current acceptance criteria.

## Build and wiring smoke

From the repository root, after native dependencies and model files are installed:

```bash
CGO_LDFLAGS="-L$HOME/lib" go build -tags bgeonnx -o ./bin/ckv ./cmd/ckv
./bin/ckv build --src=./testdata/sample --out=/tmp/ckv-bge --embedder=bgeonnx
./bin/ckv query "tokenizer encoding pipeline" --out=/tmp/ckv-bge --embedder=bgeonnx
```

The default native model name and file layout come from the registry; supply --model-name/--model-dir when selecting a different supported export. Avoid mixing this temporary index with a default Ollama or mock index.

## Failure interpretation

| Failure | Check |
|---|---|
| tokenizer link or symbol error | Go wrapper/native C ABI, architecture and link search path |
| ONNX shared library load error | CKV_ONNXRUNTIME_LIB, platform library extension and loader dependencies |
| missing model or tokenizer file | registered layout and explicit model directory |
| missing token_type_ids or unexpected output shape | model signature, pooling and selected registry configuration |
| embedding identity mismatch | exact model bytes, dimensions and prefix policy in build and query |
| ErrNotImplemented | real factory was not enabled by the bgeonnx build tag |

The separate direct CoreML adapter requires darwin/tokenizers and is not part of ordinary CLI dispatch. The native-install-contract CI job exercises packaged **mock** installation and external-consumer/recovery behavior; its success does not certify these ONNX libraries or a real model. See [integration](./embedder-integration.md), [evaluation status](../spec-driven/EVALUATION-STATUS.md) and [remaining work](../REMAINING-WORK.md).
