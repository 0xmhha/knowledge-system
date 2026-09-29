# Knowledge System 설치 시범 절차

현재 산출물은 **빌드한 호스트에서 검증하는 개발 프리뷰**다. 운영 배포 전에 대상 OS/CPU에서 네이티브 라이브러리, 모델, 라이선스와 전체 WBS Gate 5를 확인한다. `mock` 임베더는 구조 스모크 전용이며 자연어 검색 품질을 검증하지 않는다.

## 1. 호스트 패키지

저장소에서 다음을 실행한다.

```sh
python3 scripts/package-host.py --out-dir /tmp/knowledge-system-dist
```

출력 JSON의 `sha256`으로 압축 파일을 확인하고 해제한다. 압축 파일에는 `ckg`, `ckv`, `cks`, `LICENSE`, `INSTALLATION.md`, 소독 규칙, `modules.txt`, `manifest.json`이 들어 있다. `manifest.json`에는 바이너리별 SHA-256·크기, 빌드 커밋, 호스트, Go 버전, 감지한 네이티브 의존성이 기록된다. 작업 트리가 더러울 때 프리뷰가 필요하면 `--allow-dirty`를 명시하며, 매니페스트가 `dirty: true`로 표시된다. 배포 가능 판정은 아니다.

## 2. 프로젝트별 초기화

압축을 푼 디렉터리에서 **다른 Git 저장소**에도 동일한 절차를 적용한다. 데이터셋은 소스 트리 밖에 둔다.

```sh
./cks doctor --src /path/to/project
./cks init --src /path/to/project \
  --dataset /path/to/project-dataset \
  --config-out /path/to/project-setup.yaml \
  --embedder ollama --model-name YOUR_EMBEDDING_MODEL
./cks setup --config /path/to/project-setup.yaml --version auto
./cks doctor --src /path/to/project --dataset /path/to/project-dataset --strict
```

`init`은 기존 설정을 덮어쓰지 않는다. 실제 임베더를 명시해야 하며 Ollama 모델은 별도로 준비해야 한다. `doctor`는 Go, TypeScript, JavaScript, Solidity, Markdown 파일 수와 graph-only Proto 수를 미리 보고한다. 심볼릭 링크와 의심스러운 비밀 경로를 개수로만 보고하며 내용을 읽거나 출력하지 않는다. 이 진단은 프로젝트의 `.ckvignore`와 파일 목록 정책이 제대로 적용됐는지 대체하지 않는다.

`--version auto`는 커밋 기반 버전 이름을 사용한다. 추적·미추적 파일이 남아 있거나 Git이 무시하지만 CKV/CKG가 읽을 수 있는 소스 파일이 있으면 같은 HEAD의 서로 다른 바이트가 같은 버전 이름을 갖지 않도록 거부한다. 작업 트리 스냅샷 모드는 아직 구현 중이다. 설정 파일을 소스 저장소 안에 만들었다면 커밋하거나 무시 정책을 정한 뒤 빌드한다.

## 3. 서비스와 롤백

데이터셋이 승격된 다음 CKS 런타임 설정을 생성한다. 배포용 소독 규칙 경로를 명시한다.

```sh
./cks mcp gen-config --dataset-dir /path/to/project-dataset/current \
  --source-root /path/to/project \
  --sanitize-rules /path/to/unpacked/policies/sanitization_rules.yaml \
  --embed-model YOUR_EMBEDDING_MODEL --out /path/to/cks.yaml
./cks mcp --config /path/to/cks.yaml
```

롤백은 이전에 검증된 버전으로 `current`를 되돌린다. 실행 중인 MCP 프로세스는 이미 연 데이터셋 핸들을 유지하므로 롤백·승격 후 재시작해야 한다.

```sh
./cks setup --config /path/to/project-setup.yaml --rollback PREVIOUS_VERSION
```

이 절차는 프로젝트별 수동 스모크의 출발점이다. Windows/Linux 빌드·실행, 깨끗한 호스트의 네이티브 의존성, 실제 임베딩 모델의 품질·지연, 여러 프로젝트의 격리, 작업 트리 스냅샷, 재시작 후 인용 동일성은 Gate 5에 남아 있다.
