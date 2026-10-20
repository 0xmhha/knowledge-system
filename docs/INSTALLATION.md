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


## 선택 CoreML native 프로필

기본 빌드와 Ollama 기준선은 그대로다. CoreML은 macOS·CGO와 `tokenizers` 빌드 태그가 모두 필요하다. 로컬 macOS SDK와 Go 의존성에 고정된 `github.com/daulet/tokenizers` v1.27.0의 native 라이브러리를 준비하고, SDK 위치와 라이브러리 폴더를 `CGO_CFLAGS`/`CGO_LDFLAGS`에 지정한 환경에서 선택 빌드한다. 전역 설치나 모델 다운로드를 자동 수행하지 않는다.

```sh
CGO_ENABLED=1 go build -tags tokenizers -o bin/ckv ./cmd/vector
ckv build --src <repo> --out <new-output> --embedder coreml --model-name bge-m3 --model-dir <coreml-model-directory> --coreml-max-seq 512
```

`--model-dir`은 `.mlmodelc`/`.mlpackage` 디렉터리 자체 또는 그중 하나만 `model.mlmodelc`/`model.mlpackage` 이름으로 가진 부모 폴더다. tokenizer.json은 기본적으로 모델 옆에서 찾고 `--coreml-tokenizer`로 명시할 수 있다. `--model-name`은 명시해야 하며, 현재 CLI는 등록된 hidden-state/CLS 모델에 한정한다. `--embed-dim`을 사용하면 등록된 native 차원과 같아야 하며 CoreML MRL 축소는 제공하지 않는다.

첫 native 출력 계약은 batch1의 Int32 input_ids/attention_mask 두 입력과 `[1,sequence,hidden]` 출력이다. 호출자가 여러 문장을 전달하면 순서대로 batch1 추론하고 CLS를 L2 정규화한다. 실제 출력의 차원·자료형·stride를 검증하며 Float32/Double/Float16을 변환한다. 이미 pooling된2D 출력이나 추가 필수 입력을 가진 export는 지원한다고 추측하지 않는다. 모델 파일·tokenizer 바이트와 CLS/정규화/문맥 길이는 버전2 embedding identity에 반영한다. 모델 디렉터리는 열린 동안 변경하지 않는다.

현재 발견된 Qwen3 임베딩0.6b/4b는 Ollama GGUF이며, 레지스트리에서도 Ollama 전용이다. 파일 확장자나 차원이 같다는 이유로 CoreML로 사용할 수 없다. 실제 CoreML 패키지·호환 tokenizer·배포 대상과 reference embedding 검증 없이 Qwen3 CoreML 지원이나 운영 데이터셋 수용을 주장하지 않는다. 모델 없는 native 자료형·feature-provider fixture의 성공과 실제 모델 추론·품질 검증을 구분한다.
